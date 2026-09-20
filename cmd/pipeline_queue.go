package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/pipelinequeue"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

const pipelineQueueScanCap = 50

var pipelineQueueShortcut = &cobra.Command{
	Use:   "+queue",
	Short: "Shortcut: list RUNNING/WAITING runs across pipelines",
	Long: `Risk: read

Scan pipelines (cap 50 unless --pipeline-id) and list RUNNING / WAITING runs with
wait duration. When flow YAML is readable, attach runsOn.group values.

OpenAPI does not expose private runner queue depth or online executor counts;
this is a client-side cross-pipeline view for triage.

  yunxiao pipeline +queue
  yunxiao pipeline +queue --pipeline-id 5230549
  yunxiao pipeline +queue --group private/UY4U0xvrIoD6MHo7`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		groupFilter, _ := cmd.Flags().GetString("group")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		entries, truncated, err := collectQueueEntries(cmd.Context(), c, pid, groupFilter)
		if err != nil {
			handleErr(err)
			return
		}
		meta := map[string]any{"risk": risk.Read, "count": len(entries), "truncated": truncated}
		if groupFilter != "" {
			meta["group_filter"] = groupFilter
		}
		handleErr(output.Success(map[string]any{"queue": entries}, meta))
	},
}

var pipelineRunnerGroupsCmd = &cobra.Command{
	Use:     "runner-groups",
	Aliases: []string{"rg"},
	Short:   "Private runner groups (client-side discovery)",
}

var pipelineRGListCmd = &cobra.Command{
	Use:   "list",
	Short: "List runner groups discovered from pipeline YAML runsOn.group",
	Long: `Risk: read

Scans up to 50 pipelines, GETs each definition, and extracts unique runsOn.group
values. There is no public OpenAPI to list private runner groups directly.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		groups, nPipelines, truncated, err := discoverRunnerGroups(cmd.Context(), c)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(map[string]any{
			"groups":            groups,
			"pipelines_scanned": nPipelines,
			"note":              "discovered from pipeline YAML runsOn.group; not a server inventory API",
		}, map[string]any{"risk": risk.Read, "truncated": truncated}))
	},
}

var pipelineRGStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Queue summary for a runner group (client-side)",
	Long: `Risk: read

Filters +queue-style scan to runs whose pipeline YAML references --group.
Reports waiting/running counts. Online executor count is unavailable via OpenAPI.

  yunxiao pipeline runner-groups status --group private/UY4U0xvrIoD6MHo7`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		group, _ := cmd.Flags().GetString("group")
		if err := requireFlags("group", group); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		entries, truncated, err := collectQueueEntries(cmd.Context(), c, "", group)
		if err != nil {
			handleErr(err)
			return
		}
		var waiting, running int
		for _, e := range entries {
			switch strings.ToUpper(e.Status) {
			case "WAITING":
				waiting++
			case "RUNNING":
				running++
			}
		}
		handleErr(output.Success(map[string]any{
			"group":                 group,
			"waiting_runs":          waiting,
			"running_runs":          running,
			"queue":                 entries,
			"online_executors":      nil,
			"online_executors_note": "not available via public Flow OpenAPI; use the Flow web UI for agent online count",
		}, map[string]any{"risk": risk.Read, "truncated": truncated}))
	},
}

func collectQueueEntries(ctx context.Context, c *client.Client, onlyPipelineID, groupFilter string) ([]pipelinequeue.QueueEntry, bool, error) {
	type pipe struct{ id, name string }
	var pipes []pipe
	truncated := false
	if onlyPipelineID != "" {
		pipes = []pipe{{id: onlyPipelineID}}
	} else {
		ids, tr, err := resolvePendingPipelineIDs(ctx, c, "", true, 20)
		if err != nil {
			return nil, false, err
		}
		truncated = tr
		for _, id := range ids {
			pipes = append(pipes, pipe{id: id})
		}
	}

	now := time.Now()
	var out []pipelinequeue.QueueEntry
	for _, p := range pipes {
		groups := []string{}
		if flow, err := fetchPipelineFlowYAML(ctx, c, p.id); err == nil {
			groups = pipelinequeue.ExtractRunnerGroups(flow)
		}
		if groupFilter != "" {
			if !containsFold(groups, groupFilter) {
				continue
			}
		}
		for _, status := range []string{"WAITING", "RUNNING"} {
			runs, err := fetchRunsByStatus(ctx, c, p.id, status, 1, 20)
			if err != nil {
				continue
			}
			for _, r := range runs {
				rm, _ := r.(map[string]any)
				if rm == nil {
					continue
				}
				runID := fmt.Sprint(rm["pipelineRunId"])
				if runID == "" || runID == "<nil>" {
					runID = fmt.Sprint(rm["id"])
				}
				start := pipelinequeue.AsInt64(rm["startTime"])
				if start == 0 {
					start = pipelinequeue.AsInt64(rm["createTime"])
				}
				st := strings.ToUpper(fmt.Sprint(rm["status"]))
				entry := pipelinequeue.QueueEntry{
					PipelineID:   p.id,
					PipelineName: p.name,
					RunID:        runID,
					Status:       st,
					StartTimeMs:  start,
					WaitSeconds:  pipelinequeue.WaitSecondsSince(start, now),
					RunnerGroups: groups,
				}
				if u, ok := rm["url"].(string); ok {
					entry.URL = u
				} else if u := zhiyi.PipelineURL(p.id); u != "" {
					entry.URL = u
				}
				out = append(out, entry)
			}
		}
	}
	return out, truncated, nil
}

func discoverRunnerGroups(ctx context.Context, c *client.Client) ([]string, int, bool, error) {
	ids, truncated, err := resolvePendingPipelineIDs(ctx, c, "", true, 20)
	if err != nil {
		return nil, 0, false, err
	}
	seen := map[string]struct{}{}
	var groups []string
	for _, id := range ids {
		flow, err := fetchPipelineFlowYAML(ctx, c, id)
		if err != nil {
			continue
		}
		for _, g := range pipelinequeue.ExtractRunnerGroups(flow) {
			if _, ok := seen[g]; ok {
				continue
			}
			seen[g] = struct{}{}
			groups = append(groups, g)
		}
	}
	return groups, len(ids), truncated, nil
}

func containsFold(list []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, s := range list {
		if strings.EqualFold(strings.TrimSpace(s), want) {
			return true
		}
	}
	return false
}

func enrichRunQueueMeta(ctx context.Context, c *client.Client, pipelineID string, run map[string]any, meta map[string]any) {
	if run == nil || meta == nil {
		return
	}
	st := strings.ToUpper(fmt.Sprint(run["status"]))
	waitingJobs := pipelinequeue.ExtractWaitingJobs(run)
	var groups []string
	if flow, err := fetchPipelineFlowYAML(ctx, c, pipelineID); err == nil {
		groups = pipelinequeue.ExtractRunnerGroups(flow)
	}
	if st != "WAITING" && st != "RUNNING" && len(waitingJobs) == 0 {
		return
	}
	start := pipelinequeue.AsInt64(run["startTime"])
	if start == 0 {
		start = pipelinequeue.AsInt64(run["createTime"])
	}
	meta["queue"] = map[string]any{
		"run_status":    st,
		"wait_seconds":  pipelinequeue.WaitSecondsSince(start, time.Now()),
		"runner_groups": groups,
		"waiting_jobs":  waitingJobs,
		"note":          "runner queue depth / occupant not exposed by public OpenAPI; use pipeline +queue --group for cross-pipeline WAITING/RUNNING",
	}
}

func init() {
	pipelineQueueShortcut.Flags().String("pipeline-id", "", "limit to one pipeline")
	pipelineQueueShortcut.Flags().String("group", "", "filter by runsOn.group (e.g. private/xxx)")
	pipelineRGStatusCmd.Flags().String("group", "", "runner group id/name (required), e.g. private/UY4U0xvrIoD6MHo7")
	pipelineRunnerGroupsCmd.AddCommand(pipelineRGListCmd, pipelineRGStatusCmd)
}
