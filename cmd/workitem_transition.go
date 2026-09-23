package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/workflow"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var workitemTransitionCmd = &cobra.Command{
	Use:   "+transition",
	Short: "Shortcut: multi-step status transition for any workitem type",
	Long: `Risk: write (multi-step PUT; requires --yes for real run; prefer --dry-run first)

Uses profile.workflows[<type_id>] edges/statuses. When type is profile.bug_type_id and
workflows entry is missing, falls back to legacy bug_edges/bug_statuses.
Does not inject Zhiyi bug required-field flags; pass optional --fields JSON instead.
When category is Bug and profile.bug_transition_required is set, those field ids must
appear in --fields (same union as +bug-transition). Prefer +bug-transition for Zhiyi
named flags (--plan-due-date, --developer, …).

  yunxiao workitem +transition --id YXCLI-6 --to 处理中 --dry-run
  yunxiao workitem +transition --id <id> --to <alias|statusId> \
    --fields '{"80":"2026-09-20T00:00:00+08:00"}' --yes

Needs active profile. Discover graphs with +explore-workflow --write-profile.
When profile edges are missing, falls back to a single-step status PUT if --to matches a
unique status from GET workitem workflow (meta.transition_mode=direct_status). For one-off
sets you can also use: workitem update --status <id> [--cancel-reason …].

--dry-run: with profile.workflows[<type>] edges, validates current→target locally
(edge_validation=validated|hinted|illegal; illegal → ok:false). Hinted-only edges
(hinted_edges / needs_fields) are not "validated". Without cached edges, dry-run still
resolves the target status id but sets edge_validation=skipped and a warning — do not
treat that as "transition will succeed".`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pf, err := requireProfile()
		if err != nil {
			handleErr(err)
			return
		}

		id, err := workitemIDFromFlagOrArg(cmd, args)
		if err != nil {
			handleErr(err)
			return
		}
		to, _ := cmd.Flags().GetString("to")
		if strings.TrimSpace(to) == "" {
			handleErr(fmt.Errorf("missing required flag --to"))
			return
		}
		typeIDFlag, _ := cmd.Flags().GetString("type-id")
		fieldsJSON, _ := cmd.Flags().GetString("fields")
		extraFields, err := parseJSONMap(fieldsJSON)
		if err != nil {
			handleErr(err)
			return
		}
		if extraFields == nil {
			extraFields = map[string]any{}
		}

		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/workitems/"+id)
		if err != nil {
			handleErr(err)
			return
		}

		var item map[string]any
		if err := c.Get(cmd.Context(), path, nil, &item); err != nil {
			handleErr(err)
			return
		}
		resolvedID := zhiyi.InternalID(item)
		serial := zhiyi.SerialNumber(item)
		putID := id
		if resolvedID != "" {
			putID = resolvedID
		}
		putPath, err := c.ProjexPath(cmd.Context(), "/workitems/"+putID)
		if err != nil {
			handleErr(err)
			return
		}

		typeID := strings.TrimSpace(typeIDFlag)
		if typeID == "" {
			typeID = workitemTypeID(item)
		}
		if typeID == "" {
			handleErr(fmt.Errorf("无法解析工作项类型 type_id；请传 --type-id"))
			return
		}

		current := zhiyi.CurrentStatusID(item)
		if current == "" {
			handleErr(fmt.Errorf("无法解析当前状态: %s", id))
			return
		}

		var (
			wf             profileWorkflowView
			steps          []string
			target         string
			transitionMode = "profile_graph"
			resolveErr     error
		)
		wfResolved, resolveErr := pf.ResolveWorkflow(typeID)
		if resolveErr == nil {
			wf = profileWorkflowView{
				Category:    wfResolved.Category,
				Name:        wfResolved.Name,
				Edges:       wfResolved.Edges,
				HintedEdges: wfResolved.HintedEdges,
				Statuses:    wfResolved.Statuses,
				Source:      wfResolved.Source,
			}
			target = zhiyi.ResolveBugStatusId(to, wf.Statuses)
			steps, err = zhiyi.TransitionSteps(current, target, wf.Edges, wfResolved.AllStatusIDs())
			if err != nil {
				handleErr(err)
				return
			}
		} else {
			// Optional: when graph missing, try API workflow statuses → single-step status PUT.
			direct, derr := tryDirectStatusFromAPI(cmd.Context(), c, pf.SpaceID, typeID, to)
			if derr != nil {
				handleErr(fmt.Errorf("%v; also tried API workflow fallback: %w", resolveErr, derr))
				return
			}
			target = direct
			if current == target {
				steps = nil
			} else {
				steps = []string{target}
			}
			transitionMode = "direct_status"
			wf = profileWorkflowView{
				Source:   "api_workflow_statuses",
				Statuses: map[string]string{to: target},
			}
			if v, ok := item["categoryId"]; ok && v != nil {
				wf.Category = fmt.Sprint(v)
			}
		}

		category := wf.Category
		if category == "" {
			if v, ok := item["categoryId"]; ok && v != nil {
				category = fmt.Sprint(v)
			}
		}

		var requiredIDs []string
		if strings.EqualFold(category, "Bug") && len(pf.BugTransitionRequired) > 0 {
			requiredIDs = zhiyi.RequiredFieldIDs(steps, pf.BugTransitionRequired)
		}

		planned := make([]map[string]any, 0, len(steps))
		for _, st := range steps {
			body := map[string]any{"status": st}
			for k, v := range extraFields {
				body[k] = v
			}
			planned = append(planned, map[string]any{
				"method": "PUT",
				"path":   putPath,
				"body":   body,
			})
		}

		metaBase := map[string]any{
			"risk":            risk.Write,
			"profile":         pf.Name,
			"type_id":         typeID,
			"source":          wf.Source,
			"category":        category,
			"transition_mode": transitionMode,
		}
		if resolvedID != "" {
			metaBase["resolved_id"] = resolvedID
		}
		if serial != "" {
			metaBase["serial_number"] = serial
		}
		if wf.Name != "" {
			metaBase["type_name"] = wf.Name
		}
		if u := zhiyi.WorkItemURL(item, zhiyi.ResolveSpaceID(item, pf.SpaceID, "")); u != "" {
			metaBase["url"] = u
		}

		if globalDryRun {
			req := map[string]any{
				"work_item":       id,
				"resolved_id":     resolvedID,
				"serial_number":   serial,
				"type_id":         typeID,
				"category":        category,
				"source":          wf.Source,
				"transition_mode": transitionMode,
				"current":         current,
				"target":          target,
				"steps":           steps,
				"provided_fields": extraFields,
				"required_fields": requiredIDs,
				"planned_puts":    planned,
			}
			ev, warn, illegal := transitionDryRunEdgeValidationFull(transitionMode, current, target, wf.Edges, wf.HintedEdges)
			req["edge_validation"] = ev
			if warn != "" {
				req["warning"] = warn
			}
			if illegal {
				handleErr(fmt.Errorf("非法流转（dry-run）：%s → %s 不在 profile edges∪hinted_edges 中；真实 PUT 会 HTTP 400。先 workitem +explore-workflow 核实边，或改用 workitem update --status", current, target))
				return
			}
			handleErr(output.DryRunResult(string(risk.Write), req))
			return
		}

		if len(steps) > 0 && len(requiredIDs) > 0 {
			var missing []string
			for _, fid := range requiredIDs {
				if _, ok := extraFields[fid]; !ok {
					missing = append(missing, fid)
				}
			}
			if len(missing) > 0 {
				handleErr(fmt.Errorf("途经 %s 云效要求必填字段（--fields 键）：%s；或改用 workitem +bug-transition",
					strings.Join(steps, "→"), strings.Join(missing, "、")))
				return
			}
		}

		if err := risk.CheckConfirmed("workitem +transition", risk.Write, globalYes); err != nil {
			handleErr(err)
			return
		}

		applied := []string{}
		for i, st := range steps {
			body := map[string]any{"status": st}
			for k, v := range extraFields {
				body[k] = v
			}
			var out any
			if err := c.Put(cmd.Context(), putPath, body, &out); err != nil {
				handleErr(fmt.Errorf("流转在第 %d/%d 步失败；已成功：%v；%w", i+1, len(steps), applied, err))
				return
			}
			applied = append(applied, st)
		}

		refreshed, refreshOK := refreshAfterTransition(cmd.Context(), c, putPath, id)
		result := map[string]any{
			"work_item":        id,
			"resolved_id":      resolvedID,
			"serial_number":    serial,
			"type_id":          typeID,
			"category":         category,
			"source":           wf.Source,
			"transition_mode":  transitionMode,
			"current":          current,
			"target":           target,
			"steps":            steps,
			"applied":          applied,
			"refresh_ok":       refreshOK,
			"refreshed_status": zhiyi.CurrentStatusID(refreshed),
			"item":             refreshed,
		}
		urlItem := refreshed
		if urlItem == nil {
			urlItem = item
		}
		if u := zhiyi.WorkItemURL(urlItem, zhiyi.ResolveSpaceID(urlItem, pf.SpaceID, "")); u != "" {
			result["url"] = u
			metaBase["url"] = u
		}
		handleErr(output.Success(result, metaBase))
	},
}

func init() {
	workitemTransitionCmd.Flags().String("id", "", "work item id or serial (YXCLI-x / ZYPT-xxxx)")
	workitemTransitionCmd.Flags().String("to", "", "target status alias/displayName or status id (required)")
	workitemTransitionCmd.Flags().String("type-id", "", "override workitem type id (default: from item)")
	workitemTransitionCmd.Flags().String("fields", "", `optional JSON object merged into each PUT body, e.g. '{"80":"2026-09-20T00:00:00+08:00"}'`)
	workitemCmd.AddCommand(workitemTransitionCmd)
}

// profileWorkflowView is a slim view used by +transition (profile graph or API fallback).
type profileWorkflowView struct {
	Category    string
	Name        string
	Edges       map[string][]string
	HintedEdges map[string][]string
	Statuses    map[string]string
	Source      string
}


// transitionDryRunEdgeValidation reports whether --dry-run validated current→target
// against profile workflow edges (issue #59). Kept for older unit tests.
func transitionDryRunEdgeValidation(transitionMode string, edges map[string][]string) (status, warning string) {
	st, warn, _ := transitionDryRunEdgeValidationFull(transitionMode, "", "", edges, nil)
	return st, warn
}

// transitionDryRunEdgeValidationFull classifies current→target against edges / hinted_edges (#75).
//
//   - skipped: no cached edges (direct_status / empty)
//   - validated: path exists in verified edges
//   - hinted: only reachable via hinted_edges (needs_fields / unverified)
//   - illegal: known graph but target not in edges∪hinted_edges (including source-not-on-graph passthrough)
func transitionDryRunEdgeValidationFull(transitionMode, current, target string, edges, hinted map[string][]string) (status, warning string, illegal bool) {
	if transitionMode == "direct_status" || len(edges) == 0 {
		return "skipped", "未校验流转边：无 profile.workflows 缓存边（仅解析目标状态 id / api_workflow_statuses）；真实 PUT 仍可能 HTTP 400「不能流转到目标状态」。可先 workitem +explore-workflow --write-profile 缓存边后再 --dry-run。", false
	}
	// Backward-compat helper call without current/target: presence of edges ⇒ validated.
	if current == "" && target == "" {
		return "validated", "", false
	}
	if current == target {
		return "validated", "", false
	}
	if edgePathExists(edges, current, target) {
		return "validated", "", false
	}
	if edgePathExists(hinted, current, target) || edgePathExists(mergeEdgeMaps(edges, hinted), current, target) {
		return "hinted", "目标边仅在 hinted_edges（needs_fields / 未实证）；dry-run 不保证真实 PUT 成功。", false
	}
	return "illegal", "当前状态→目标不在 edges∪hinted_edges；真实 PUT 会失败。", true
}

func mergeEdgeMaps(a, b map[string][]string) map[string][]string {
	out := map[string][]string{}
	for _, src := range []map[string][]string{a, b} {
		for from, tos := range src {
			seen := map[string]bool{}
			for _, t := range out[from] {
				seen[t] = true
			}
			for _, t := range tos {
				if t == "" || seen[t] {
					continue
				}
				seen[t] = true
				out[from] = append(out[from], t)
			}
		}
	}
	return out
}

// edgePathExists is BFS on adjacency; also true for direct membership.
func edgePathExists(edges map[string][]string, from, to string) bool {
	if len(edges) == 0 || from == "" || to == "" {
		return false
	}
	if from == to {
		return true
	}
	queue := []string{from}
	seen := map[string]bool{from: true}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, next := range edges[n] {
			if next == to {
				return true
			}
			if seen[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return false
}

// tryDirectStatusFromAPI loads GET .../workitemTypes/{type}/workflows statuses and
// resolves --to uniquely. Used only when profile edges are missing (no multi-hop).
func tryDirectStatusFromAPI(ctx context.Context, c interface {
	ProjexPath(context.Context, string) (string, error)
	Get(context.Context, string, map[string]string, any) error
}, spaceID, typeID, to string) (string, error) {
	spaceID = strings.TrimSpace(spaceID)
	typeID = strings.TrimSpace(typeID)
	if spaceID == "" || typeID == "" {
		return "", fmt.Errorf("need space_id and type_id for workflow status fallback")
	}
	path, err := c.ProjexPath(ctx, "/projects/"+spaceID+"/workitemTypes/"+typeID+"/workflows")
	if err != nil {
		return "", err
	}
	var raw any
	if err := c.Get(ctx, path, nil, &raw); err != nil {
		return "", err
	}
	_, _, _, statuses, err := workflow.ParseWorkflowResponse(raw)
	if err != nil {
		return "", err
	}
	return workflow.ResolveUniqueStatus(to, statuses)
}
