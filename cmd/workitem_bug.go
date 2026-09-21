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

Ports zhiyi domain.ts TransitionSteps + bug.ts required-field union.`,
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
		steps, err := zhiyi.TransitionSteps(current, target, pf.StatusGraph(), pf.AllStatusIDs())
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

		metaBase := map[string]any{"risk": risk.Write, "profile": pf.Name}
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
			handleErr(output.DryRunResult(string(risk.Write), map[string]any{
				"work_item":       id,
				"resolved_id":     resolvedID,
				"serial_number":   serial,
				"current":         current,
				"target":          target,
				"steps":           steps,
				"provided_fields": provided,
				"required_fields": requiredIDs,
				"planned_puts":    planned,
			}))
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
	workitemBugTransitionCmd.Flags().String("id", "", "work item id or serial (ZYPT-xxxx)")
	workitemBugTransitionCmd.Flags().String("to", "", "target status alias or status id (required)")
	workitemBugTransitionCmd.Flags().String("plan-due-date", "", "YYYY-MM-DD (required en route to processing/testing)")
	workitemBugTransitionCmd.Flags().String("developer", "", "developer userId(s), comma-separated")
	workitemBugTransitionCmd.Flags().String("responsible-person", "", "responsible person userId (deploy-test)")
	workitemBugTransitionCmd.Flags().String("bug-reason", "", "free-text bug reason (not an enum id; paste the business description)")
	workitemBugTransitionCmd.Flags().String("bug-impact-scope", "", "free-text impact scope (not an enum id)")
	workitemCmd.AddCommand(workitemBugTransitionCmd)
}
