package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var pipelineCmd = &cobra.Command{
	Use:     "pipeline",
	Aliases: []string{"flow"},
	Short:   "Flow pipelines and run summaries",
	Long: `Pipeline / Flow domain.

+shortcuts:
  yunxiao pipeline +failed --pipeline-id <id>
  yunxiao pipeline +status --pipeline-id <id>
  yunxiao pipeline +pending [--pipeline-id <id>|--all-pipelines]
  yunxiao pipeline +approve|--refuse --pipeline-id <id> --run-id <id> --job-id <id> --yes

Typed:
  yunxiao pipeline list
  yunxiao pipeline run list|latest|get|failed|trigger|cancel|watch
  yunxiao pipeline job log|stop|retry|skip|rerun|pass|refuse --pipeline-id <id> --run-id <id> --job-id <id>

Risk: reads are read; trigger/cancel/job mutations/gates are high-risk-write.
Typed pipeline lifecycle:
  yunxiao pipeline get --id <id>
  yunxiao pipeline create --name <n> --file pipeline.yaml|--content '...'
  yunxiao pipeline update --id <id> --name <n> --file pipeline.yaml|--content '...'

Risk: get/list=read; create/update=high-risk-write (YAML replaces pipeline definition).`,
}

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pipelines",
	Long:  "Risk: read\nHTTP: GET .../pipelines\n\nUse --all to follow pages via client.ListAll (cap 50).",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		name, _ := cmd.Flags().GetString("name")
		statusList, _ := cmd.Flags().GetString("status-list")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		allPages, _ := cmd.Flags().GetBool("all")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines")
		if err != nil {
			handleErr(err)
			return
		}
		q := client.PageQuery(page, perPage)
		if name != "" {
			q["pipelineName"] = name
		}
		if statusList != "" {
			q["statusList"] = statusList
		}
		after := func(out any, meta map[string]any) (any, map[string]any) {
			return zhiyi.AttachPipelineURLs(out), meta
		}
		if allPages {
			handleErr(runReadAll(cmd.Context(), c, "GET", path, q, perPage, map[string]any{"risk": risk.Read}, after))
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, after))
	},
}
var pipelineRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Pipeline run queries",
}

var pipelineJobCmd = &cobra.Command{
	Use:   "job",
	Short: "Pipeline job queries",
}

var pipelineRunListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pipeline runs",
	Long:  "Risk: read\nHTTP: GET .../pipelines/{id}/runs\n\nDefault order: newest first by update/create time. Client-side --sort applies within the current page. Use --sort asc for oldest first.",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		status, _ := cmd.Flags().GetString("status")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		if err := requireFlags("pipeline-id", pid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs")
		if err != nil {
			handleErr(err)
			return
		}
		q := client.PageQuery(page, perPage)
		if status != "" {
			q["status"] = status
		}
		sortFlag, _ := cmd.Flags().GetString("sort")
		after, err := afterSortByTime(sortFlag, func(out any, meta map[string]any) (any, map[string]any) {
			return zhiyi.AttachPipelineRunURLs(out, pid), meta
		})
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, after))
	},
}

var pipelineRunLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Get latest pipeline run",
	Long:  "Risk: read\nHTTP: GET .../pipelines/{id}/runs/latestPipelineRun",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		if err := requireFlags("pipeline-id", pid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs/latestPipelineRun")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			zhiyi.EnrichPipelineRunMeta(meta, asStringMap(out), pid)
			return out, meta
		}))
	},
}

var pipelineRunGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a pipeline run by ID",
	Long:  "Risk: read\nHTTP: GET .../pipelines/{id}/runs/{runId}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		rid, _ := cmd.Flags().GetString("run-id")
		if err := requireFlags("pipeline-id", pid, "run-id", rid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs/"+rid)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			zhiyi.EnrichPipelineRunMeta(meta, asStringMap(out), pid)
			return out, meta
		}))
	},
}

var pipelineRunFailedCmd = &cobra.Command{
	Use:   "failed",
	Short: "Summarize latest failed pipeline run(s)",
	Long:  "Risk: read\nHTTP: GET .../pipelines/{id}/runs?status=FAIL",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		perPage, _ := cmd.Flags().GetInt("per-page")
		if err := requireFlags("pipeline-id", pid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"status": "FAIL", "page": "1", "perPage": strconv.Itoa(perPage)}
		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.Read), c.Preview("GET", path, q, nil)))
			return
		}
		var runs any
		if err := c.Get(cmd.Context(), path, q, &runs); err != nil {
			handleErr(err)
			return
		}
		runs = zhiyi.AttachPipelineRunURLs(runs, pid)
		summary := map[string]any{"pipeline_id": pid, "status": "FAIL", "runs": runs}
		lpath, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs/latestPipelineRun")
		if err == nil {
			var latest any
			if err := c.Get(cmd.Context(), lpath, nil, &latest); err == nil {
				summary["latest_run"] = zhiyi.AttachPipelineRunURLs(latest, pid)
			}
		}
		meta := map[string]any{"risk": risk.Read}
		if lm, ok := summary["latest_run"].(map[string]any); ok {
			zhiyi.EnrichPipelineRunMeta(meta, lm, pid)
		} else if u := zhiyi.PipelineURL(pid); u != "" {
			meta["url"] = u
		}
		handleErr(output.Success(summary, meta))
	},
}

var pipelineJobLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Get job run log",
	Long:  "Risk: read\nHTTP: GET .../pipelines/{id}/runs/{runId}/job/{jobId}/log",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		rid, _ := cmd.Flags().GetString("run-id")
		jid, _ := cmd.Flags().GetString("job-id")
		if err := requireFlags("pipeline-id", pid, "run-id", rid, "job-id", jid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs/"+rid+"/job/"+jid+"/log")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var pipelineRunTriggerCmd = &cobra.Command{
	Use:   "trigger",
	Short: "Trigger a pipeline run (high-risk-write)",
	Long: `Risk: high-risk-write

HTTP: POST .../pipelines/{id}/runs

Optional --params is a JSON object (or string) passed as API params.
Optional --branch sets branchModeBranchs via params shorthand.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		branch, _ := cmd.Flags().GetString("branch")
		paramsJSON, _ := cmd.Flags().GetString("params")
		comment, _ := cmd.Flags().GetString("comment")
		if err := requireFlags("pipeline-id", pid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{}
		paramsObj := map[string]any{}
		if paramsJSON != "" {
			var raw any
			if err := json.Unmarshal([]byte(paramsJSON), &raw); err != nil {
				handleErr(fmt.Errorf("--params must be JSON: %w", err))
				return
			}
			switch v := raw.(type) {
			case string:
				body["params"] = v
			case map[string]any:
				paramsObj = v
			default:
				handleErr(fmt.Errorf("--params must be a JSON object or string"))
				return
			}
		}
		if branch != "" {
			paramsObj["branchModeBranchs"] = []string{branch}
		}
		if comment != "" {
			paramsObj["comment"] = comment
		}
		if _, ok := body["params"]; !ok && len(paramsObj) > 0 {
			b, _ := json.Marshal(paramsObj)
			body["params"] = string(b)
		}
		handleErr(runJSONMutating(cmd.Context(), c, "pipeline run trigger", risk.HighRiskWrite, "POST", path, nil, body, func(out any, meta map[string]any) (any, map[string]any) {
			meta["pipeline_id"] = pid
			zhiyi.EnrichPipelineRunMeta(meta, asStringMap(out), pid)
			return out, meta
		}))
	},
}

var pipelineJobStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a pipeline job run (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: PUT .../pipelineRuns/{runId}/jobs/{jobId}/stop",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		rid, _ := cmd.Flags().GetString("run-id")
		jid, _ := cmd.Flags().GetString("job-id")
		if err := requireFlags("pipeline-id", pid, "run-id", rid, "job-id", jid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/pipelineRuns/"+rid+"/jobs/"+jid+"/stop")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "pipeline job stop", risk.HighRiskWrite, "PUT", path, nil, nil, nil))
	},
}

var pipelineRunCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel / stop an entire pipeline run (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: PUT .../pipelines/{id}/runs/{runId}  (UpdatePipelineRun / 终止流水线运行)",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		rid, _ := cmd.Flags().GetString("run-id")
		if err := requireFlags("pipeline-id", pid, "run-id", rid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs/"+rid)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "pipeline run cancel", risk.HighRiskWrite, "PUT", path, nil, nil, func(out any, meta map[string]any) (any, map[string]any) {
			meta["pipeline_id"] = pid
			meta["run_id"] = rid
			m := asStringMap(out)
			if m == nil {
				m = map[string]any{"pipelineId": pid, "pipelineRunId": rid}
			}
			zhiyi.EnrichPipelineRunMeta(meta, m, pid)
			return out, meta
		}))
	},
}

var pipelineJobRetryCmd = &cobra.Command{
	Use: "retry", Short: "Retry a pipeline job (high-risk-write)",
	Long: "Risk: high-risk-write\nHTTP: PUT .../jobs/{jobId}/retry",
	Run:  pipelineJobAction("pipeline job retry", "retry", "PUT", risk.HighRiskWrite),
}
var pipelineJobSkipCmd = &cobra.Command{
	Use: "skip", Short: "Skip a pipeline job (high-risk-write)",
	Long: "Risk: high-risk-write\nHTTP: PUT .../jobs/{jobId}/skip",
	Run:  pipelineJobAction("pipeline job skip", "skip", "PUT", risk.HighRiskWrite),
}
var pipelineJobRerunCmd = &cobra.Command{
	Use: "rerun", Short: "Rerun a pipeline job (deploy jobs; high-risk-write)",
	Long: "Risk: high-risk-write\nHTTP: PUT .../jobs/{jobId}/rerun",
	Run:  pipelineJobAction("pipeline job rerun", "rerun", "PUT", risk.HighRiskWrite),
}

var pipelineJobPassCmd = &cobra.Command{
	Use: "pass", Short: "Pass a manual validation gate (high-risk-write)",
	Long: "Risk: high-risk-write\nHTTP: POST .../jobs/{jobId}/pass",
	Run:  pipelineJobAction("pipeline job pass", "pass", "POST", risk.HighRiskWrite),
}
var pipelineJobRefuseCmd = &cobra.Command{
	Use: "refuse", Short: "Refuse a manual validation gate (high-risk-write)",
	Long: "Risk: high-risk-write\nHTTP: POST .../jobs/{jobId}/refuse",
	Run:  pipelineJobAction("pipeline job refuse", "refuse", "POST", risk.HighRiskWrite),
}

var pipelineFailedShortcut = &cobra.Command{
	Use:   "+failed",
	Short: "Shortcut: latest failed run summary",
	Long:  "Risk: read",
	Run: func(cmd *cobra.Command, args []string) {
		pipelineRunFailedCmd.Run(cmd, args)
	},
}

var pipelineStatusShortcut = &cobra.Command{
	Use:   "+status",
	Short: "Shortcut: latest run status summary for a pipeline",
	Long:  "Risk: read",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		if err := requireFlags("pipeline-id", pid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs/latestPipelineRun")
		if err != nil {
			handleErr(err)
			return
		}
		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.Read), c.Preview("GET", path, nil, nil)))
			return
		}
		var latest map[string]any
		if err := c.Get(cmd.Context(), path, nil, &latest); err != nil {
			handleErr(err)
			return
		}
		latestAny := zhiyi.AttachPipelineRunURLs(latest, pid)
		summary := map[string]any{
			"pipeline_id": pid,
			"latest_run":  latestAny,
		}
		// extract common status fields if present
		for _, k := range []string{"status", "resultStatus", "pipelineRunId", "startTime", "endTime"} {
			if v, ok := latest[k]; ok {
				summary[k] = v
			}
		}
		meta := map[string]any{"risk": risk.Read}
		zhiyi.EnrichPipelineRunMeta(meta, latest, pid)
		handleErr(output.Success(summary, meta))
	},
}

var pipelineGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get pipeline detail (includes pipelineConfig)",
	Long:  "Risk: read\nHTTP: GET .../pipelines/{id}\nSource: operations/flow/pipeline.ts getPipelineFunc",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			zhiyi.EnrichPipelineMeta(meta, asStringMap(out))
			if _, ok := meta["url"]; !ok {
				if u := zhiyi.PipelineURL(id); u != "" {
					meta["url"] = u
				}
			}
			return out, meta
		}))
	},
}

var pipelineCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a pipeline from YAML (high-risk-write)",
	Long: `Risk: high-risk-write
HTTP: POST .../pipelines  body {name, content}
Source: operations/flow/pipeline.ts createPipelineFunc / CreatePipelineSchema
Provide YAML via --file (relative) or --content.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		name, _ := cmd.Flags().GetString("name")
		contentFlag, _ := cmd.Flags().GetString("content")
		file, _ := cmd.Flags().GetString("file")
		if err := requireFlags("name", name); err != nil {
			handleErr(err)
			return
		}
		content, err := readContentOrFile(contentFlag, file)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"name": name, "content": content}
		preview := c.Preview("POST", path, nil, map[string]any{
			"name": name, "content_bytes": len(content), "content_preview": truncateStr(content, 200),
		})
		handleErr(runMutating("pipeline create", risk.HighRiskWrite, globalDryRun, globalYes, preview, func() error {
			var out any
			if err := c.Post(cmd.Context(), path, body, &out); err != nil {
				return err
			}
			meta := map[string]any{"risk": risk.HighRiskWrite}
			zhiyi.EnrichPipelineMeta(meta, asStringMap(out))
			return output.Success(out, meta)
		}))
	},
}

var pipelineUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update pipeline name + YAML content (high-risk-write)",
	Long: `Risk: high-risk-write
HTTP: PUT .../pipelines/{id}  body {name, content}
Source: operations/flow/pipeline.ts updatePipelineFunc / UpdatePipelineSchema
Both --name and YAML (--file|--content) are required by the OpenAPI.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")
		contentFlag, _ := cmd.Flags().GetString("content")
		file, _ := cmd.Flags().GetString("file")
		if err := requireFlags("id", id, "name", name); err != nil {
			handleErr(err)
			return
		}
		content, err := readContentOrFile(contentFlag, file)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"name": name, "content": content}
		preview := c.Preview("PUT", path, nil, map[string]any{
			"name": name, "content_bytes": len(content), "content_preview": truncateStr(content, 200),
		})
		handleErr(runMutating("pipeline update", risk.HighRiskWrite, globalDryRun, globalYes, preview, func() error {
			var out any
			if err := c.Put(cmd.Context(), path, body, &out); err != nil {
				return err
			}
			meta := map[string]any{"risk": risk.HighRiskWrite}
			zhiyi.EnrichPipelineMeta(meta, asStringMap(out))
			if _, ok := meta["url"]; !ok {
				if u := zhiyi.PipelineURL(id); u != "" {
					meta["url"] = u
				}
			}
			return output.Success(out, meta)
		}))
	},
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

var pipelineSCCmd = &cobra.Command{Use: "service-connections", Aliases: []string{"sc"}, Short: "Flow service connections"}
var pipelineHGCmd = &cobra.Command{Use: "host-groups", Short: "Flow host groups"}
var pipelineFlowVGCmd = &cobra.Command{Use: "flow-variable-groups", Aliases: []string{"flow-vg"}, Short: "Flow org-level variable groups (distinct from AppStack VG)"}
var pipelineRMCmd = &cobra.Command{Use: "resource-members", Short: "Flow resource members"}

var pipelineSCListCmd = &cobra.Command{
	Use:   "list",
	Short: "List service connections",
	Long:  "Risk: read\nHTTP: GET .../flow/.../serviceConnections?sericeConnectionType=\nNote: query key spelling matches OpenAPI typo (sericeConnectionType).\nSource: operations/flow/serviceConnection.ts",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		typ, _ := cmd.Flags().GetString("type")
		if err := requireFlags("type", typ); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/serviceConnections")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"sericeConnectionType": typ}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var pipelineHGListCmd = &cobra.Command{
	Use:   "list",
	Short: "List host groups",
	Long:  "Risk: read\nHTTP: GET .../flow/.../hostGroups\nSource: operations/flow/hostGroup.ts",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		name, _ := cmd.Flags().GetString("name")
		ids, _ := cmd.Flags().GetString("ids")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/hostGroups")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{}
		if name != "" {
			q["name"] = name
		}
		if ids != "" {
			q["ids"] = ids
		}
		if page > 0 {
			q["page"] = strconv.Itoa(page)
		}
		if perPage > 0 {
			q["perPage"] = strconv.Itoa(perPage)
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var pipelineFlowVGListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Flow variable groups",
	Long:  "Risk: read\nHTTP: GET .../flow/.../variableGroups\nSource: operations/flow/variableGroups.ts",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/variableGroups")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{}
		if page > 0 {
			q["page"] = strconv.Itoa(page)
		}
		if perPage > 0 {
			q["perPage"] = strconv.Itoa(perPage)
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var pipelineFlowVGGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a Flow variable group",
	Long:  "Risk: read\nHTTP: GET .../variableGroups/{id}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/variableGroups/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var pipelineFlowVGCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Flow variable group (high-risk-write)",
	Long: `Risk: high-risk-write
HTTP: POST .../variableGroups with query params name, variables (JSON string), optional description
Source: createFlowVariableGroupFunc — OpenAPI uses query, not JSON body.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		name, _ := cmd.Flags().GetString("name")
		varsJSON, _ := cmd.Flags().GetString("variables")
		desc, _ := cmd.Flags().GetString("description")
		if err := requireFlags("name", name, "variables", varsJSON); err != nil {
			handleErr(err)
			return
		}
		// validate JSON array
		var raw any
		if err := json.Unmarshal([]byte(varsJSON), &raw); err != nil {
			handleErr(fmt.Errorf("invalid --variables JSON: %w", err))
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/variableGroups")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"name": name, "variables": varsJSON}
		if desc != "" {
			q["description"] = desc
		}
		handleErr(runJSONMutating(cmd.Context(), c, "pipeline flow-variable-groups create", risk.HighRiskWrite, "POST", path, q, nil, nil))
	},
}

var pipelineFlowVGUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Flow variable group (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: PUT .../variableGroups/{id} with query name,variables[,description]",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")
		varsJSON, _ := cmd.Flags().GetString("variables")
		desc, _ := cmd.Flags().GetString("description")
		if err := requireFlags("id", id, "name", name, "variables", varsJSON); err != nil {
			handleErr(err)
			return
		}
		var raw any
		if err := json.Unmarshal([]byte(varsJSON), &raw); err != nil {
			handleErr(fmt.Errorf("invalid --variables JSON: %w", err))
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/variableGroups/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"name": name, "variables": varsJSON}
		if desc != "" {
			q["description"] = desc
		}
		handleErr(runJSONMutating(cmd.Context(), c, "pipeline flow-variable-groups update", risk.HighRiskWrite, "PUT", path, q, nil, nil))
	},
}

var pipelineFlowVGDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete Flow variable group (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: DELETE .../variableGroups/{id}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/variableGroups/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "pipeline flow-variable-groups delete", risk.HighRiskWrite, "DELETE", path, nil, nil, nil))
	},
}

var pipelineRMListCmd = &cobra.Command{
	Use:   "list",
	Short: "List members of a Flow resource",
	Long:  "Risk: read\nHTTP: GET .../resourceMembers/resourceTypes/{type}/resourceIds/{id}\nSource: listResourceMembersFunc",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		rtype, _ := cmd.Flags().GetString("resource-type")
		rid, _ := cmd.Flags().GetString("resource-id")
		if err := requireFlags("resource-type", rtype, "resource-id", rid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/resourceMembers/resourceTypes/"+rtype+"/resourceIds/"+rid)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

func pipelineJobAction(action, suffix, method string, level risk.Level) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pid, _ := cmd.Flags().GetString("pipeline-id")
		rid, _ := cmd.Flags().GetString("run-id")
		jid, _ := cmd.Flags().GetString("job-id")
		if err := requireFlags("pipeline-id", pid, "run-id", rid, "job-id", jid); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/pipelineRuns/"+rid+"/jobs/"+jid+"/"+suffix)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, action, level, method, path, nil, nil, nil))
	}
}

func init() {
	pipelineGetCmd.Flags().String("id", "", "pipeline id (required)")
	pipelineCreateCmd.Flags().String("name", "", "pipeline name (required, max 60)")
	pipelineCreateCmd.Flags().String("content", "", "pipeline YAML content")
	pipelineCreateCmd.Flags().String("file", "", "relative path to YAML file")
	pipelineUpdateCmd.Flags().String("id", "", "pipeline id (required)")
	pipelineUpdateCmd.Flags().String("name", "", "pipeline name (required)")
	pipelineUpdateCmd.Flags().String("content", "", "pipeline YAML content")
	pipelineUpdateCmd.Flags().String("file", "", "relative path to YAML file")
	pipelineListCmd.Flags().String("name", "", "pipeline name filter")
	pipelineListCmd.Flags().String("status-list", "", "comma-separated status list")
	pipelineListCmd.Flags().Int("page", 1, "page")
	pipelineListCmd.Flags().Int("per-page", 20, "per page")
	pipelineListCmd.Flags().Bool("all", false, "follow all pages (ListAll, max 50)")
	pipelineRunListCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineRunListCmd.Flags().String("status", "", "run status filter e.g. FAIL|SUCCESS|RUNNING")
	pipelineRunListCmd.Flags().Int("page", 1, "page")
	pipelineRunListCmd.Flags().Int("per-page", 20, "per page")
	addSortFlag(pipelineRunListCmd)
	pipelineRunLatestCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineRunGetCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineRunGetCmd.Flags().String("run-id", "", "pipeline run id (required)")
	pipelineRunFailedCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineRunFailedCmd.Flags().Int("per-page", 5, "how many failed runs to return")
	pipelineJobLogCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineJobLogCmd.Flags().String("run-id", "", "pipeline run id (required)")
	pipelineJobLogCmd.Flags().String("job-id", "", "job id (required)")
	pipelineFailedShortcut.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineFailedShortcut.Flags().Int("per-page", 5, "how many failed runs")
	pipelineStatusShortcut.Flags().String("pipeline-id", "", "pipeline id (required)")

	pipelinePendingShortcut.Flags().String("pipeline-id", "", "pipeline id (optional with --all-pipelines)")
	pipelinePendingShortcut.Flags().Bool("all-pipelines", false, "scan pipelines via ListAll when --pipeline-id is empty")
	pipelinePendingShortcut.Flags().Bool("include-running", false, "also scan RUNNING runs (default WAITING only)")
	pipelinePendingShortcut.Flags().Int("page", 1, "run list page")
	pipelinePendingShortcut.Flags().Int("per-page", 20, "run list per page")

	pipelineRunWatchCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineRunWatchCmd.Flags().String("run-id", "", "pipeline run id (required)")
	pipelineRunWatchCmd.Flags().Duration("interval", 0, "poll interval (default 5s)")
	pipelineRunWatchCmd.Flags().Duration("timeout", 0, "overall timeout (default 30m)")

	for _, sc := range []*cobra.Command{pipelineApproveShortcut, pipelineRefuseShortcut} {
		sc.Flags().String("pipeline-id", "", "pipeline id (required)")
		sc.Flags().String("run-id", "", "pipeline run id (required)")
		sc.Flags().String("job-id", "", "job id (required)")
	}

	pipelineRunTriggerCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineRunTriggerCmd.Flags().String("branch", "", "optional branch (branchModeBranchs)")
	pipelineRunTriggerCmd.Flags().String("params", "", "JSON params object or string")
	pipelineRunTriggerCmd.Flags().String("comment", "", "run comment")
	pipelineJobStopCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineJobStopCmd.Flags().String("run-id", "", "pipeline run id (required)")
	pipelineJobStopCmd.Flags().String("job-id", "", "job id (required)")
	for _, jc := range []*cobra.Command{pipelineJobRetryCmd, pipelineJobSkipCmd, pipelineJobRerunCmd, pipelineJobPassCmd, pipelineJobRefuseCmd} {
		jc.Flags().String("pipeline-id", "", "pipeline id (required)")
		jc.Flags().String("run-id", "", "pipeline run id (required)")
		jc.Flags().String("job-id", "", "job id (required)")
	}
	pipelineJobCmd.AddCommand(pipelineJobLogCmd, pipelineJobStopCmd, pipelineJobRetryCmd, pipelineJobSkipCmd, pipelineJobRerunCmd, pipelineJobPassCmd, pipelineJobRefuseCmd)
	pipelineRunCancelCmd.Flags().String("pipeline-id", "", "pipeline id (required)")
	pipelineRunCancelCmd.Flags().String("run-id", "", "pipeline run id (required)")
	pipelineRunCmd.AddCommand(pipelineRunListCmd, pipelineRunLatestCmd, pipelineRunGetCmd, pipelineRunFailedCmd, pipelineRunTriggerCmd, pipelineRunCancelCmd, pipelineRunWatchCmd)
	pipelineSCListCmd.Flags().String("type", "", "service connection type (required; query key sericeConnectionType)")
	pipelineHGListCmd.Flags().String("name", "", "host group name filter")
	pipelineHGListCmd.Flags().String("ids", "", "comma-separated ids")
	pipelineHGListCmd.Flags().Int("page", 1, "page")
	pipelineHGListCmd.Flags().Int("per-page", 20, "per page")
	pipelineFlowVGListCmd.Flags().Int("page", 1, "page")
	pipelineFlowVGListCmd.Flags().Int("per-page", 10, "per page (max 30)")
	pipelineFlowVGGetCmd.Flags().String("id", "", "variable group id (required)")
	pipelineFlowVGCreateCmd.Flags().String("name", "", "name (required)")
	pipelineFlowVGCreateCmd.Flags().String("variables", "", "JSON array [{name,value,isEncrypted}] (required)")
	pipelineFlowVGCreateCmd.Flags().String("description", "", "description")
	pipelineFlowVGUpdateCmd.Flags().String("id", "", "id (required)")
	pipelineFlowVGUpdateCmd.Flags().String("name", "", "name (required)")
	pipelineFlowVGUpdateCmd.Flags().String("variables", "", "JSON array (required)")
	pipelineFlowVGUpdateCmd.Flags().String("description", "", "description")
	pipelineFlowVGDeleteCmd.Flags().String("id", "", "id (required)")
	pipelineRMListCmd.Flags().String("resource-type", "", "resource type (required)")
	pipelineRMListCmd.Flags().String("resource-id", "", "resource id (required)")
	pipelineSCCmd.AddCommand(pipelineSCListCmd)
	pipelineHGCmd.AddCommand(pipelineHGListCmd)
	pipelineFlowVGCmd.AddCommand(pipelineFlowVGListCmd, pipelineFlowVGGetCmd, pipelineFlowVGCreateCmd, pipelineFlowVGUpdateCmd, pipelineFlowVGDeleteCmd)
	pipelineRMCmd.AddCommand(pipelineRMListCmd)
	pipelineCmd.AddCommand(pipelineListCmd, pipelineGetCmd, pipelineCreateCmd, pipelineUpdateCmd, pipelineRunCmd, pipelineJobCmd, pipelineSCCmd, pipelineHGCmd, pipelineFlowVGCmd, pipelineRMCmd, pipelineFailedShortcut, pipelineStatusShortcut, pipelinePendingShortcut, pipelineApproveShortcut, pipelineRefuseShortcut)
}
