package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var workitemBugTransitionCmd = &cobra.Command{
	Use:   "+bug-transition",
	Short: "Shortcut: multi-step bug status transition (BFS + required fields)",
	Long: `Risk: write (multi-step PUT; requires --yes for real run; prefer --dry-run first)

Needs active profile with bug_statuses (e.g. --profile zhiyi / YUNXIAO_PROFILE=zhiyi).

  yunxiao workitem +bug-transition --id ZYPT-5768 --to processing \
    --plan-due-date 2026-09-20 --developer <uid> --dry-run
  yunxiao workitem +bug-transition --id ZYPT-5768 --to testing \
    --plan-due-date 2026-09-20 --developer <uid> \
    --responsible-person <uid> --bug-reason "代码缺陷：简述根因" --bug-impact-scope "影响模块/范围简述" --yes

Ports zhiyi domain.ts TransitionSteps + bug.ts required-field union.

Profile bug_edges are UNVERIFIED template assumptions unless written by
+explore-workflow --write-profile (OpenAPI exposes statuses only; edges need real
PUT probes). When BFS over those edges finds no route, this command falls back to
ONE direct PUT of the target status (meta.transition_mode=bfs_no_path_direct) so
"platform allows but the profile has no edge" still works; --direct forces that
single-step attempt without consulting the graph (also skips status-machine
membership checks — the platform is the arbiter).

Failures distinguish the two causes: platform rejected a graph-planned step
(platform_rejected_transition) vs profile edges had no route AND the direct
fallback hop was rejected (no path in profile edges + platform rejected
transition — either truly disallowed, or the real route needs intermediate
statuses the profile has not verified). Fix the graph in a sandbox:
yunxiao workitem +explore-workflow --type-id <bug_type_id> --category Bug --write-profile --yes`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pf, err := requireProfile()
		if err != nil {
			handleErr(err)
			return
		}
		if len(pf.BugStatuses) == 0 {
			handleErr(fmt.Errorf("profile %q missing bug_statuses", pf.Name))
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

		current := zhiyi.CurrentStatusID(item)
		if current == "" {
			handleErr(fmt.Errorf("无法解析当前状态: %s", id))
			return
		}
		target := zhiyi.ResolveBugStatusId(to, pf.BugStatuses)
		direct, _ := cmd.Flags().GetBool("direct")
		steps, transitionMode, err := zhiyi.BugTransitionPlan(current, target, pf.StatusGraph(), pf.AllStatusIDs(), direct)
		if err != nil {
			handleErr(err)
			return
		}

		planDueDate, _ := cmd.Flags().GetString("plan-due-date")
		developer, _ := cmd.Flags().GetString("developer")
		responsiblePerson, _ := cmd.Flags().GetString("responsible-person")
		bugReason, _ := cmd.Flags().GetString("bug-reason")
		bugImpactScope, _ := cmd.Flags().GetString("bug-impact-scope")

		provided := map[string]any{}
		fidPlan := pf.FieldID("plan_due_date")
		fidDev := pf.FieldID("developer")
		fidResp := pf.FieldID("responsible_person")
		fidReason := pf.FieldID("bug_reason")
		fidImpact := pf.FieldID("bug_impact_scope")

		if strings.TrimSpace(planDueDate) != "" {
			if fidPlan == "" {
				handleErr(fmt.Errorf("profile missing bug_fields.plan_due_date"))
				return
			}
			provided[fidPlan] = zhiyi.PlanDueDateWire(planDueDate)
		}
		if ids := zhiyi.SplitUserIDs(developer); len(ids) > 0 {
			if fidDev == "" {
				handleErr(fmt.Errorf("profile missing bug_fields.developer"))
				return
			}
			provided[fidDev] = ids
		}
		if strings.TrimSpace(responsiblePerson) != "" {
			if fidResp == "" {
				handleErr(fmt.Errorf("profile missing bug_fields.responsible_person"))
				return
			}
			provided[fidResp] = strings.TrimSpace(responsiblePerson)
		}
		if strings.TrimSpace(bugReason) != "" {
			if fidReason == "" {
				handleErr(fmt.Errorf("profile missing bug_fields.bug_reason"))
				return
			}
			provided[fidReason] = strings.TrimSpace(bugReason)
		}
		if strings.TrimSpace(bugImpactScope) != "" {
			if fidImpact == "" {
				handleErr(fmt.Errorf("profile missing bug_fields.bug_impact_scope"))
				return
			}
			provided[fidImpact] = strings.TrimSpace(bugImpactScope)
		}

		requiredIDs := zhiyi.RequiredFieldIDs(steps, pf.BugTransitionRequired)
		flagNameFor := func(fid string) string {
			switch fid {
			case fidPlan:
				return "--plan-due-date"
			case fidDev:
				return "--developer"
			case fidResp:
				return "--responsible-person"
			case fidReason:
				return "--bug-reason"
			case fidImpact:
				return "--bug-impact-scope"
			default:
				return fid
			}
		}

		planned := make([]map[string]any, 0, len(steps))
		for _, st := range steps {
			body := map[string]any{"status": st}
			for k, v := range provided {
				body[k] = v
			}
			planned = append(planned, map[string]any{
				"method": "PUT",
				"path":   putPath,
				"body":   body,
			})
		}

		metaBase := map[string]any{"risk": risk.Write, "profile": pf.Name, "transition_mode": transitionMode}
		if resolvedID != "" {
			metaBase["resolved_id"] = resolvedID
		}
		if serial != "" {
			metaBase["serial_number"] = serial
		}
		if u := zhiyi.WorkItemURL(item, zhiyi.ResolveSpaceID(item, pf.SpaceID, "")); u != "" {
			metaBase["url"] = u
		}

		if globalDryRun {
			req := map[string]any{
				"work_item":       id,
				"resolved_id":     resolvedID,
				"serial_number":   serial,
				"current":         current,
				"target":          target,
				"steps":           steps,
				"transition_mode": transitionMode,
				"provided_fields": provided,
				"required_fields": requiredIDs,
				"planned_puts":    planned,
			}
			if w := bugTransitionModeWarning(transitionMode); w != "" {
				req["warning"] = w
			}
			handleErr(output.DryRunResult(string(risk.Write), req))
			return
		}

		if len(steps) > 0 && len(requiredIDs) > 0 {
			var missing []string
			for _, fid := range requiredIDs {
				if _, ok := provided[fid]; !ok {
					missing = append(missing, flagNameFor(fid))
				}
			}
			if len(missing) > 0 {
				handleErr(fmt.Errorf("途经 %s 云效要求必填：%s（按每步并集，非只校验终点）",
					strings.Join(steps, "→"), strings.Join(missing, "、")))
				return
			}
		}

		if err := risk.CheckConfirmed("workitem +bug-transition", risk.Write, globalYes); err != nil {
			handleErr(err)
			return
		}

		applied := []string{}
		for i, st := range steps {
			body := map[string]any{"status": st}
			for k, v := range provided {
				body[k] = v
			}
			var out any
			if err := c.Put(cmd.Context(), putPath, body, &out); err != nil {
				handleErr(bugTransitionPutError(transitionMode, i+1, len(steps), applied, err))
				return
			}
			applied = append(applied, st)
		}

		refreshed, refreshOK := refreshAfterTransition(cmd.Context(), c, putPath, id)
		result := map[string]any{
			"work_item":        id,
			"resolved_id":      resolvedID,
			"serial_number":    serial,
			"current":          current,
			"target":           target,
			"steps":            steps,
			"transition_mode":  transitionMode,
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
	workitemBugTransitionCmd.Flags().String("id", "", "work item id or serial (ZYPT-xxxx)")
	workitemBugTransitionCmd.Flags().String("to", "", "target status alias or status id (required)")
	workitemBugTransitionCmd.Flags().String("plan-due-date", "", "YYYY-MM-DD (required en route to processing/testing)")
	workitemBugTransitionCmd.Flags().String("developer", "", "developer userId(s), comma-separated")
	workitemBugTransitionCmd.Flags().String("responsible-person", "", "responsible person userId (deploy-test)")
	workitemBugTransitionCmd.Flags().String("bug-reason", "", "free-text bug reason (not an enum id; paste the business description)")
	workitemBugTransitionCmd.Flags().String("bug-impact-scope", "", "free-text impact scope (not an enum id)")
	workitemBugTransitionCmd.Flags().Bool("direct", false, "skip BFS over profile bug_edges: single-step PUT of the target status (platform decides legality; profile edges are unverified template guesses until +explore-workflow --write-profile)")
	workitemCmd.AddCommand(workitemBugTransitionCmd)
}

// bugTransitionModeWarning explains non-graph-backed plans in dry-run output (#123):
// the cached profile edges hold no route (or were skipped), so the single direct PUT
// is arbitrated by the platform and may still fail with HTTP 400.
func bugTransitionModeWarning(mode string) string {
	switch mode {
	case zhiyi.TransitionModeBFSNoPathDirect:
		return "profile bug_edges 无实证路径（no path in profile edges），回退单步直试目标态：真实 PUT 由平台裁决，可能 HTTP 400「不能流转到目标状态」。profile 边是未实证模板；沙箱跑 workitem +explore-workflow --write-profile 可固化实证边。"
	case zhiyi.TransitionModeDirectForced:
		return "--direct 单步直试：跳过 profile bug_edges 与状态机校验，真实 PUT 由平台裁决，可能 HTTP 400「不能流转到目标状态」。"
	}
	return ""
}

// bugTransitionPutError renders a failed +bug-transition status PUT with the #123
// cause split:
//   - profile_bfs: every planned step came from the cached graph, so the failure is
//     a platform rejection of a graph-backed step (platform_rejected_transition);
//   - bfs_no_path_direct: the profile edges held no route and the fallback direct
//     hop was rejected too (no_path_in_profile_edges + platform_rejected_transition
//     — either truly disallowed, or the real route needs unverified intermediates);
//   - direct_forced: --direct skipped the graph and the platform rejected the hop.
//
// The cause is wrapped in a contextError so a *client.APIError underneath keeps the
// api envelope (type "api", status code, API hint) with the cause-prefixed message.
func bugTransitionPutError(mode string, step, total int, applied []string, err error) error {
	switch mode {
	case zhiyi.TransitionModeBFSNoPathDirect:
		return &contextError{
			Context: fmt.Sprintf("no path in profile edges（bug_edges 无实证路径），回退单步直试目标态也被平台拒绝（platform rejected transition）；已成功：%v", applied),
			Hint:    "两种可能：平台确实不允许该单步流转；或需经中间状态多步流转但 profile 边未实证。沙箱跑 yunxiao workitem +explore-workflow --type-id <bug_type_id> --category Bug --write-profile --yes 固化真实图后重试。",
			Err:     err,
		}
	case zhiyi.TransitionModeDirectForced:
		return &contextError{
			Context: fmt.Sprintf("--direct 单步直试被平台拒绝（platform rejected transition）；已成功：%v", applied),
			Hint:    "平台不允许当前态单步流转到该目标（或目标状态 id 无效）；经中间状态的路径可用 workitem +explore-workflow 探测。",
			Err:     err,
		}
	default:
		return &contextError{
			Context: fmt.Sprintf("流转在第 %d/%d 步被平台拒绝（platform rejected transition）；已成功：%v", step, total, applied),
			Hint:    "profile 边为未实证模板或已过期；沙箱重跑 yunxiao workitem +explore-workflow --write-profile 刷新实证边。",
			Err:     err,
		}
	}
}
