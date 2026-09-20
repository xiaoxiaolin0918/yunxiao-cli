package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/pipelinegate"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var pipelinePendingShortcut = &cobra.Command{
	Use:   "+pending",
	Short: "List WAITING runs with unhandled manual gates",
	Long: `Risk: read

Scan pipeline runs (status WAITING, optionally RUNNING), fetch run detail, and
extract jobs whose actions include pass/refuse (or ManualValidate / 人工 / Manual).

Flags:
  --pipeline-id       scan one pipeline (recommended)
  --all-pipelines     when --pipeline-id is empty, ListAll pipelines (cap 50)
  --include-running   also scan RUNNING runs
  --page / --per-page run-list pagination (default 1 / 20)

Dry-run previews the first GET (pipeline list or run list).`,
	Run: runPipelinePending,
}

func runPipelinePending(cmd *cobra.Command, _ []string) {
	flagOrg(globalOrg)
	pid, _ := cmd.Flags().GetString("pipeline-id")
	allPipelines, _ := cmd.Flags().GetBool("all-pipelines")
	includeRunning, _ := cmd.Flags().GetBool("include-running")
	page, _ := cmd.Flags().GetInt("page")
	perPage, _ := cmd.Flags().GetInt("per-page")
	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}

	c, _, err := mustClient()
	if err != nil {
		handleErr(err)
		return
	}

	pipelineIDs, err := resolvePendingPipelineIDs(cmd.Context(), c, pid, allPipelines, perPage)
	if err != nil {
		handleErr(err)
		return
	}
	if globalDryRun {
		return
	}

	statuses := []string{"WAITING"}
	if includeRunning {
		statuses = append(statuses, "RUNNING")
	}

	var pending []pipelinegate.PendingJob
	scanned := 0
	for _, pipelineID := range pipelineIDs {
		for _, st := range statuses {
			runs, err := fetchRunsByStatus(cmd.Context(), c, pipelineID, st, page, perPage)
			if err != nil {
				handleErr(err)
				return
			}
			for _, item := range runs {
				rm := asStringMap(item)
				if rm == nil {
					continue
				}
				runID := firstNonEmpty(
					stringifyAny(rm["pipelineRunId"]),
					stringifyAny(rm["runId"]),
					stringifyAny(rm["id"]),
					stringifyAny(rm["buildId"]),
				)
				if runID == "" {
					continue
				}
				scanned++
				detail, err := fetchRunDetail(cmd.Context(), c, pipelineID, runID)
				if err != nil {
					handleErr(err)
					return
				}
				pending = append(pending, pipelinegate.ExtractPendingJobs(detail, pipelineID)...)
			}
		}
	}

	// Attach URLs on each pending row (ExtractPendingJobs already sets them; re-enrich list meta).
	_ = zhiyi.AttachPipelineRunURLs
	data := map[string]any{
		"pending": pending,
		"count":   len(pending),
	}
	meta := map[string]any{
		"risk":            risk.Read,
		"pipeline_ids":    pipelineIDs,
		"statuses":        statuses,
		"scanned_runs":    scanned,
		"include_running": includeRunning,
	}
	handleErr(output.Success(data, meta))
}

func resolvePendingPipelineIDs(ctx context.Context, c *client.Client, pid string, allPipelines bool, perPage int) ([]string, error) {
	pid = strings.TrimSpace(pid)
	if pid != "" {
		if globalDryRun {
			path, err := c.FlowPath(ctx, "/pipelines/"+pid+"/runs")
			if err != nil {
				return nil, err
			}
			q := client.PageQuery(1, perPage)
			q["status"] = "WAITING"
			return nil, output.DryRunResult(string(risk.Read), c.Preview("GET", path, q, nil))
		}
		return []string{pid}, nil
	}
	if !allPipelines {
		return nil, fmt.Errorf("missing --pipeline-id (or pass --all-pipelines)")
	}
	path, err := c.FlowPath(ctx, "/pipelines")
	if err != nil {
		return nil, err
	}
	if globalDryRun {
		q := client.PageQuery(1, perPage)
		return nil, output.DryRunResult(string(risk.Read), c.Preview("GET", path, q, nil))
	}
	fetch := func(ctx context.Context, q map[string]string) (any, http.Header, error) {
		var body any
		hdr, err := c.Do(ctx, "GET", path, q, nil, &body)
		return body, hdr, err
	}
	res, err := client.ListAll(ctx, 1, perPage, client.DefaultListAllMaxPages, map[string]string{}, fetch)
	if err != nil {
		return nil, err
	}
	return pipelineIDsFromItems(res.Items), nil
}

func fetchRunsByStatus(ctx context.Context, c *client.Client, pipelineID, status string, page, perPage int) ([]any, error) {
	path, err := c.FlowPath(ctx, "/pipelines/"+pipelineID+"/runs")
	if err != nil {
		return nil, err
	}
	q := client.PageQuery(page, perPage)
	q["status"] = status
	var body any
	_, err = c.Do(ctx, "GET", path, q, nil, &body)
	if err != nil {
		return nil, err
	}
	items := client.ExtractListItems(body)
	if items == nil {
		return []any{}, nil
	}
	return items, nil
}

func fetchRunDetail(ctx context.Context, c *client.Client, pipelineID, runID string) (map[string]any, error) {
	path, err := c.FlowPath(ctx, "/pipelines/"+pipelineID+"/runs/"+runID)
	if err != nil {
		return nil, err
	}
	var body any
	_, err = c.Do(ctx, "GET", path, nil, nil, &body)
	if err != nil {
		return nil, err
	}
	if m := asStringMap(body); m != nil {
		return m, nil
	}
	return map[string]any{}, nil
}

func pipelineIDsFromItems(items []any) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, it := range items {
		m := asStringMap(it)
		if m == nil {
			continue
		}
		id := firstNonEmpty(
			stringifyAny(m["pipelineId"]),
			stringifyAny(m["id"]),
		)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func stringifyAny(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return fmt.Sprintf("%.0f", t)
	case float32:
		return fmt.Sprintf("%.0f", t)
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

// +approve / +refuse are shortcuts for job pass/refuse; high-risk-write requires --yes.
var pipelineApproveShortcut = &cobra.Command{
	Use:   "+approve",
	Short: "Shortcut: pass a manual gate (requires --yes)",
	Long:  "Risk: high-risk-write\nEquivalent to: pipeline job pass … --yes",
	Run: func(cmd *cobra.Command, args []string) {
		pipelineJobPassCmd.Run(cmd, args)
	},
}

var pipelineRefuseShortcut = &cobra.Command{
	Use:   "+refuse",
	Short: "Shortcut: refuse a manual gate (requires --yes)",
	Long:  "Risk: high-risk-write\nEquivalent to: pipeline job refuse … --yes",
	Run: func(cmd *cobra.Command, args []string) {
		pipelineJobRefuseCmd.Run(cmd, args)
	},
}
