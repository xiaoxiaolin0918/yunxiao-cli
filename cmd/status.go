package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/pipelinegate"
	"github.com/yunxiao-cli/yunxiao/internal/pipelinescan"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

// statusCmd aggregates the morning-check surfaces that used to require three
// separate commands (+my-open-items / +open-mrs / +pending), closing the
// yx status gap documented in docs/wiki/00-process/gh-yx-migration.md.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Dashboard: my open work items, open MRs, optional pending gates",
	Long: `Risk: read

Aggregates common morning checks that used to require three commands
(project +my-open-items, codeup +open-mrs, pipeline +pending) — closes the
yx status gap (docs/wiki/00-process/gh-yx-migration.md).

Sections (soft-fail per section; one failure does not abort the rest):
  workitems       assigned-to-me open items (default category=Req, statusStage=1,2)
  mrs             opened merge requests (same url/status/wip enrich as +open-mrs)
  pending_gates   only when --pipeline-id or --all-pipelines is set; otherwise
                  skipped with a reason (scanning every pipeline is expensive)

Flags:
  --category / --space-id / --status-stage   workitem section (same semantics as +my-open-items)
  --repo                                     optional MR repo filter (id|alias|org/repo|bare name)
  --pipeline-id / --all-pipelines / --include-running
  --skip-workitems / --skip-mrs
  --page / --per-page                        shared list pagination (default 1 / 20)
  --dry-run                                  preview each enabled section's first request (no API calls)

Output data shape:
  {
    "workitems":     {"ok": true, "count": N, "items": [...]},
    "mrs":           {"ok": true, "count": N, "items": [...]},
    "pending_gates": {"ok": true, "skipped": true, "reason": "..."} | {"ok": true, "count": N, "items": [...], ...}
  }
`,
	Run: runStatus,
}

func runStatus(cmd *cobra.Command, _ []string) {
	flagOrg(globalOrg)
	skipWI, _ := cmd.Flags().GetBool("skip-workitems")
	skipMrs, _ := cmd.Flags().GetBool("skip-mrs")
	category, _ := cmd.Flags().GetString("category")
	spaceIDFlag, _ := cmd.Flags().GetString("space-id")
	statusStage, _ := cmd.Flags().GetString("status-stage")
	repo, _ := cmd.Flags().GetString("repo")
	pipelineID, _ := cmd.Flags().GetString("pipeline-id")
	allPipelines, _ := cmd.Flags().GetBool("all-pipelines")
	includeRunning, _ := cmd.Flags().GetBool("include-running")
	page, _ := cmd.Flags().GetInt("page")
	perPage, _ := cmd.Flags().GetInt("per-page")
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	pipelineID = strings.TrimSpace(pipelineID)
	wantPending := pipelineID != "" || allPipelines

	c, _, err := mustClient()
	if err != nil {
		handleErr(err)
		return
	}

	if globalDryRun {
		previews, perr := statusDryRunPreviews(cmd.Context(), c, statusDryRunOpts{
			skipWI: skipWI, skipMrs: skipMrs, wantPending: wantPending,
			category: category, spaceIDFlag: spaceIDFlag, statusStage: statusStage,
			repo: repo, pipelineID: pipelineID, allPipelines: allPipelines,
			page: page, perPage: perPage,
		})
		if perr != nil {
			handleErr(perr)
			return
		}
		handleErr(output.DryRunResult(string(risk.Read), map[string]any{
			"action":   "status",
			"previews": previews,
		}))
		return
	}

	out := map[string]any{}
	meta := map[string]any{"risk": risk.Read}
	sectionsOK := true

	if skipWI {
		out["workitems"] = map[string]any{"ok": true, "skipped": true, "reason": "--skip-workitems"}
	} else {
		sec, ok := statusFetchWorkitems(cmd.Context(), c, category, spaceIDFlag, statusStage, page, perPage)
		out["workitems"] = sec
		if !ok {
			sectionsOK = false
		}
	}

	if skipMrs {
		out["mrs"] = map[string]any{"ok": true, "skipped": true, "reason": "--skip-mrs"}
	} else {
		sec, ok := statusFetchMrs(cmd.Context(), c, repo, page, perPage)
		out["mrs"] = sec
		if !ok {
			sectionsOK = false
		}
	}

	if !wantPending {
		out["pending_gates"] = map[string]any{
			"ok":      true,
			"skipped": true,
			"reason":  "pass --pipeline-id or --all-pipelines to include pending gates",
		}
	} else {
		sec, ok := statusFetchPending(cmd.Context(), c, pipelineID, allPipelines, includeRunning, page, perPage)
		out["pending_gates"] = sec
		if !ok {
			sectionsOK = false
		}
	}

	meta["sections_ok"] = sectionsOK
	handleErr(output.Success(out, meta))
	if !sectionsOK {
		handleErr(output.ExitError{Code: 1, Msg: "status: one or more sections failed"})
	}
}

type statusDryRunOpts struct {
	skipWI, skipMrs, wantPending       bool
	category, spaceIDFlag, statusStage string
	repo, pipelineID                   string
	allPipelines                       bool
	page, perPage                      int
}

type statusSectionPreview struct {
	Section string `json:"section"`
	client.RequestPreview
}

func statusDryRunPreviews(ctx context.Context, c *client.Client, opt statusDryRunOpts) ([]statusSectionPreview, error) {
	previews := []statusSectionPreview{}
	if !opt.skipWI {
		spaceID, err := resolveSpaceIDFlag(opt.spaceIDFlag)
		if err != nil {
			return nil, err
		}
		path, err := c.ProjexPath(ctx, "/workitems:search")
		if err != nil {
			return nil, err
		}
		body := map[string]any{
			"category": opt.category,
			"spaceId":  spaceID,
			"page":     opt.page,
			"perPage":  opt.perPage,
			"orderBy":  "gmtCreate",
			"sort":     "desc",
		}
		previews = append(previews, statusSectionPreview{Section: "workitems", RequestPreview: c.Preview("POST", path, nil, body)})
	}
	if !opt.skipMrs {
		path, err := c.CodeupPath(ctx, "/changeRequests")
		if err != nil {
			return nil, err
		}
		q := map[string]string{
			"page":    strconv.Itoa(opt.page),
			"perPage": strconv.Itoa(opt.perPage),
			"state":   "opened",
		}
		if opt.repo != "" {
			resolved, err := resolveCodeupRepo(opt.repo)
			if err != nil {
				return nil, err
			}
			q["projectIds"] = resolved
		}
		previews = append(previews, statusSectionPreview{Section: "mrs", RequestPreview: c.Preview("GET", path, q, nil)})
	}
	if opt.wantPending {
		var path string
		var err error
		q := client.PageQuery(1, opt.perPage)
		if opt.pipelineID != "" {
			path, err = c.FlowPath(ctx, "/pipelines/"+opt.pipelineID+"/runs")
			q["status"] = "WAITING"
		} else {
			path, err = c.FlowPath(ctx, "/pipelines")
		}
		if err != nil {
			return nil, err
		}
		previews = append(previews, statusSectionPreview{Section: "pending_gates", RequestPreview: c.Preview("GET", path, q, nil)})
	}
	return previews, nil
}

func statusSectionError(err error) map[string]any {
	return map[string]any{
		"ok":      false,
		"count":   0,
		"items":   []any{},
		"error":   err.Error(),
		"skipped": false,
	}
}

func statusFetchWorkitems(ctx context.Context, c *client.Client, category, spaceIDFlag, statusStage string, page, perPage int) (map[string]any, bool) {
	spaceID, err := resolveSpaceIDFlag(spaceIDFlag)
	if err != nil {
		return statusSectionError(err), false
	}
	var user map[string]any
	if err := c.Get(ctx, "/oapi/v1/platform/user", nil, &user); err != nil {
		return statusSectionError(err), false
	}
	uid, _ := user["id"].(string)
	if uid == "" {
		return statusSectionError(fmt.Errorf("platform user response missing id")), false
	}
	path, err := c.ProjexPath(ctx, "/workitems:search")
	if err != nil {
		return statusSectionError(err), false
	}
	conds := map[string]any{
		"conditionGroups": []any{
			[]any{
				map[string]any{
					"className":       "user",
					"fieldIdentifier": "assignedTo",
					"format":          "list",
					"operator":        "CONTAINS",
					"value":           []string{uid},
				},
				map[string]any{
					"className":       "statusStage",
					"fieldIdentifier": "statusStage",
					"format":          "list",
					"operator":        "CONTAINS",
					"value":           splitCSV(statusStage),
				},
			},
		},
	}
	cb, _ := json.Marshal(conds)
	body := map[string]any{
		"category":   category,
		"conditions": string(cb),
		"orderBy":    "gmtCreate",
		"sort":       "desc",
		"page":       page,
		"perPage":    perPage,
		"spaceId":    spaceID,
	}
	var raw any
	if _, err := c.Do(ctx, "POST", path, nil, body, &raw); err != nil {
		return statusSectionError(err), false
	}
	items := client.ExtractListItems(raw)
	if items == nil {
		items = []any{}
	}
	return map[string]any{
		"ok":          true,
		"count":       len(items),
		"items":       items,
		"assigned_to": uid,
		"category":    category,
		"space_id":    spaceID,
	}, true
}

func statusFetchMrs(ctx context.Context, c *client.Client, repo string, page, perPage int) (map[string]any, bool) {
	path, err := c.CodeupPath(ctx, "/changeRequests")
	if err != nil {
		return statusSectionError(err), false
	}
	q := map[string]string{
		"page":    strconv.Itoa(page),
		"perPage": strconv.Itoa(perPage),
		"state":   "opened",
	}
	if repo != "" {
		resolved, err := resolveCodeupRepo(repo)
		if err != nil {
			return statusSectionError(err), false
		}
		q["projectIds"] = resolved
	}
	var raw any
	if _, err := c.Do(ctx, "GET", path, q, nil, &raw); err != nil {
		return statusSectionError(err), false
	}
	enriched, _ := enrichMrsList("")(raw, map[string]any{})
	items := client.ExtractListItems(enriched)
	if items == nil {
		if s, ok := enriched.([]any); ok {
			items = s
		} else {
			items = []any{}
		}
	}
	sec := map[string]any{
		"ok":    true,
		"count": len(items),
		"items": items,
		"state": "opened",
	}
	if repo != "" {
		sec["repo"] = repo
	}
	return sec, true
}

func statusFetchPending(ctx context.Context, c *client.Client, pipelineID string, allPipelines, includeRunning bool, page, perPage int) (map[string]any, bool) {
	pipelineIDs, truncated, err := resolvePendingPipelineIDsNoDryRun(ctx, c, pipelineID, allPipelines, perPage)
	if err != nil {
		return statusSectionError(err), false
	}
	statuses := []string{"WAITING"}
	if includeRunning {
		statuses = append(statuses, "RUNNING")
	}
	var pending []pipelinegate.PendingJob
	scannedRuns := 0
	softFail := allPipelines || len(pipelineIDs) > 1
	var scanRep pipelinescan.Report
	for _, pid := range pipelineIDs {
		scanRep.NoteScannedPipeline(pid)
		for _, st := range statuses {
			runs, err := fetchRunsByStatus(ctx, c, pid, st, page, perPage)
			if err != nil {
				if softFail {
					scanRep.Record(pid, "runs/"+st, err)
					continue
				}
				return statusSectionError(err), false
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
				scannedRuns++
				detail, err := fetchRunDetail(ctx, c, pid, runID)
				if err != nil {
					if softFail {
						scanRep.Record(pid, "run/"+runID, err)
						continue
					}
					return statusSectionError(err), false
				}
				pending = append(pending, pipelinegate.ExtractPendingJobs(detail, pid)...)
			}
		}
	}
	sec := map[string]any{
		"ok":              true,
		"count":           len(pending),
		"items":           pending,
		"pipeline_ids":    pipelineIDs,
		"statuses":        statuses,
		"scanned_runs":    scannedRuns,
		"include_running": includeRunning,
		"truncated":       truncated,
	}
	for k, v := range scanRep.Meta() {
		sec[k] = v
	}
	if n, ok := sec["error_count"].(int); ok && n > 0 {
		sec["degraded"] = true
	}
	return sec, true
}

// resolvePendingPipelineIDsNoDryRun mirrors resolvePendingPipelineIDs without the
// dry-run side channel (status handles dry-run at the top level).
func resolvePendingPipelineIDsNoDryRun(ctx context.Context, c *client.Client, pid string, allPipelines bool, perPage int) ([]string, bool, error) {
	pid = strings.TrimSpace(pid)
	if pid != "" {
		return []string{pid}, false, nil
	}
	if !allPipelines {
		return nil, false, fmt.Errorf("missing --pipeline-id (or pass --all-pipelines)")
	}
	path, err := c.FlowPath(ctx, "/pipelines")
	if err != nil {
		return nil, false, err
	}
	fetch := func(ctx context.Context, q map[string]string) (any, http.Header, error) {
		var body any
		hdr, err := c.Do(ctx, "GET", path, q, nil, &body)
		return body, hdr, err
	}
	res, err := client.ListAll(ctx, 1, perPage, client.DefaultListAllMaxPages, map[string]string{}, fetch)
	if err != nil {
		return nil, false, err
	}
	ids := pipelineIDsFromItems(res.Items)
	const maxPendingPipelines = 50
	truncated := res.Truncated
	if len(ids) > maxPendingPipelines {
		ids = ids[:maxPendingPipelines]
		truncated = true
	}
	return ids, truncated, nil
}

func init() {
	statusCmd.Flags().Bool("skip-workitems", false, "omit the workitems section")
	statusCmd.Flags().Bool("skip-mrs", false, "omit the mrs section")
	statusCmd.Flags().String("category", "Req", "work item category: Req|Task|Bug|Risk")
	statusCmd.Flags().String("space-id", "", "project/space id (default: profile.space_id)")
	statusCmd.Flags().String("status-stage", "1,2", "status stage IDs (default open: 1,2)")
	statusCmd.Flags().String("repo", "", "filter open MRs by repository id, alias, org/repo path, or bare name")
	statusCmd.Flags().String("pipeline-id", "", "include pending gates for this pipeline")
	statusCmd.Flags().Bool("all-pipelines", false, "include pending gates across pipelines (page budget; hard-cap 50)")
	statusCmd.Flags().Bool("include-running", false, "also scan RUNNING runs when pending gates are enabled")
	statusCmd.Flags().Int("page", 1, "list page (shared by sections)")
	statusCmd.Flags().Int("per-page", 20, "list per page (shared by sections)")
	rootCmd.AddCommand(statusCmd)
}
