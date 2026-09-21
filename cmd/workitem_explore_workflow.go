package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/workflow"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var workitemExploreWorkflowCmd = &cobra.Command{
	Use:   "+explore-workflow",
	Short: "Shortcut: probe workitem type status transition graph",
	Long: `Risk: write (multi-step create/PUT/delete; requires --yes for real run; prefer --dry-run first)

OpenAPI type/workflows returns statuses only (no edges). This command creates or reuses a
probe work item and attempts PUT {"status": to} for each ordered pair to discover edges.

  yunxiao workitem +explore-workflow --profile play \
    --type-id <type-id> --dry-run

  yunxiao workitem +explore-workflow --profile play \
    --type-id <type-id> --cleanup --yes

  yunxiao workitem +explore-workflow --profile play \
    --type-id <type-id> --cleanup --write-profile --yes

--space-id defaults to active profile.space_id.
--id reuses an existing work item of that type (never deleted).
--cleanup deletes a temp item we created (high-risk; needs --yes).
--write-profile writes into profile.workflows[<type-id>] (project-scoped profile via space_id;
workflows are per type_id). For Bug category when type-id matches profile.bug_type_id (or
bug_type_id is empty), also merges legacy bug_edges / bug_statuses for +bug-transition.

Limitations: required-field failures are recorded as edges with required_hints; some
false negatives possible when errors are ambiguous. Do not run against production ZYPT
without an explicit probe item and review.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pf, err := applyActiveProfileOrg()
		if err != nil {
			handleErr(err)
			return
		}

		typeID, _ := cmd.Flags().GetString("type-id")
		spaceID, _ := cmd.Flags().GetString("space-id")
		existingID, _ := cmd.Flags().GetString("id")
		category, _ := cmd.Flags().GetString("category")
		cleanup, _ := cmd.Flags().GetBool("cleanup")
		writeProfile, _ := cmd.Flags().GetBool("write-profile")

		if strings.TrimSpace(typeID) == "" && pf != nil && strings.EqualFold(category, "Bug") {
			typeID = pf.BugTypeID
		}
		if err := requireFlags("type-id", typeID); err != nil {
			handleErr(err)
			return
		}
		if spaceID == "" && pf != nil {
			spaceID = pf.SpaceID
		}
		if spaceID == "" {
			handleErr(fmt.Errorf("missing --space-id (or profile.space_id)"))
			return
		}

		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}

		wfPath, err := c.ProjexPath(cmd.Context(), "/projects/"+spaceID+"/workitemTypes/"+typeID+"/workflows")
		if err != nil {
			handleErr(err)
			return
		}

		// Dry-run: plan only (still may GET workflow to enrich the plan).
		if globalDryRun {
			plan := map[string]any{
				"action":        "workitem +explore-workflow",
				"space_id":      spaceID,
				"type_id":       typeID,
				"category":      category,
				"existing_id":   existingID,
				"cleanup":       cleanup,
				"write_profile": writeProfile,
				"steps": []string{
					"GET type workflow statuses",
					"obtain probe work item (reuse --id or POST create temp)",
					"for each status pair (from,to): ensure at from; PUT status=to; record edge/hint",
					"optional DELETE temp probe if --cleanup",
					"optional merge workflows[type_id] (+ legacy bug_* for Bug) if --write-profile",
				},
				"get_workflow": c.Preview("GET", wfPath, nil, nil),
			}
			if existingID == "" {
				createBody := buildExploreCreateBody(cmd.Context(), pf, c, spaceID, typeID, category)
				createPath, _ := c.ProjexPath(cmd.Context(), "/workitems")
				plan["create_probe"] = c.Preview("POST", createPath, nil, createBody)
			} else {
				plan["reuse_id"] = existingID
			}
			var wfRaw any
			if err := c.Get(cmd.Context(), wfPath, nil, &wfRaw); err == nil {
				_, _, def, statuses, perr := workflow.ParseWorkflowResponse(wfRaw)
				if perr == nil {
					ids := workflow.StatusIDs(statuses)
					n := len(ids)
					plan["default_status_id"] = def
					plan["status_count"] = n
					plan["max_pair_attempts"] = n * (n - 1)
					plan["statuses"] = statuses
				}
			}
			handleErr(output.DryRunResult(string(risk.Write), plan))
			return
		}

		if err := risk.CheckConfirmed("workitem +explore-workflow", risk.Write, globalYes); err != nil {
			handleErr(err)
			return
		}

		var wfRaw any
		if err := c.Get(cmd.Context(), wfPath, nil, &wfRaw); err != nil {
			handleErr(err)
			return
		}
		wfID, wfName, defaultStatusID, statuses, err := workflow.ParseWorkflowResponse(wfRaw)
		if err != nil {
			handleErr(err)
			return
		}
		statusIDs := workflow.StatusIDs(statuses)

		userSupplied := strings.TrimSpace(existingID) != ""
		var createdIDs []string
		probeID := strings.TrimSpace(existingID)
		var probeSerial string
		var startStatus string

		createPath, err := c.ProjexPath(cmd.Context(), "/workitems")
		if err != nil {
			handleErr(err)
			return
		}

		createProbe := func() (string, string, string, error) {
			body := buildExploreCreateBody(cmd.Context(), pf, c, spaceID, typeID, category)
			var createdOut map[string]any
			if err := c.Post(cmd.Context(), createPath, body, &createdOut); err != nil {
				return "", "", "", fmt.Errorf("create probe work item: %w", err)
			}
			id := zhiyi.InternalID(createdOut)
			if id == "" {
				return "", "", "", fmt.Errorf("create probe succeeded but no id in response: %v", createdOut)
			}
			serial := ""
			st := defaultStatusID
			if item, err := fetchWorkItem(cmd.Context(), c, id); err == nil {
				serial = zhiyi.SerialNumber(item)
				if cur := zhiyi.CurrentStatusID(item); cur != "" {
					st = cur
				}
			}
			return id, serial, st, nil
		}

		if probeID != "" {
			item, err := fetchWorkItem(cmd.Context(), c, probeID)
			if err != nil {
				handleErr(err)
				return
			}
			if rid := zhiyi.InternalID(item); rid != "" {
				probeID = rid
			}
			probeSerial = zhiyi.SerialNumber(item)
			startStatus = zhiyi.CurrentStatusID(item)
			gotType := workitemTypeID(item)
			if gotType != "" && gotType != typeID {
				handleErr(fmt.Errorf("probe item type %s does not match --type-id %s", gotType, typeID))
				return
			}
		} else {
			id, serial, st, err := createProbe()
			if err != nil {
				handleErr(err)
				return
			}
			probeID = id
			probeSerial = serial
			startStatus = st
			createdIDs = append(createdIDs, id)
		}

		probeRes, err := workflow.ExploreTransitions(workflow.ProbeOptions{
			StatusIDs:     statusIDs,
			StartStatus:   startStatus,
			DefaultStatus: defaultStatusID,
			Get: func() (string, error) {
				item, err := fetchWorkItem(cmd.Context(), c, probeID)
				if err != nil {
					return "", err
				}
				return zhiyi.CurrentStatusID(item), nil
			},
			Put: func(to string) error {
				putPath, err := c.ProjexPath(cmd.Context(), "/workitems/"+probeID)
				if err != nil {
					return err
				}
				var out any
				return c.Put(cmd.Context(), putPath, map[string]any{"status": to}, &out)
			},
			Reset: func() (string, error) {
				// Only auto-recreate when we own the probe items (not user --id).
				if userSupplied {
					return "", fmt.Errorf("cannot reset user-supplied probe item")
				}
				id, serial, st, err := createProbe()
				if err != nil {
					return "", err
				}
				probeID = id
				probeSerial = serial
				createdIDs = append(createdIDs, id)
				return st, nil
			},
		})
		if err != nil {
			handleErr(err)
			return
		}

		cleaned := false
		cleanupErrors := []string{}
		if cleanup && len(createdIDs) > 0 {
			if err := risk.CheckHighRisk("workitem +explore-workflow --cleanup delete", globalYes); err != nil {
				handleErr(err)
				return
			}
			cleanedAll := true
			for _, id := range createdIDs {
				delPath, err := c.ProjexPath(cmd.Context(), "/workitems/"+id)
				if err != nil {
					cleanupErrors = append(cleanupErrors, err.Error())
					cleanedAll = false
					continue
				}
				var delOut any
				if err := c.Delete(cmd.Context(), delPath, nil, &delOut); err != nil {
					cleanupErrors = append(cleanupErrors, err.Error())
					cleanedAll = false
					continue
				}
			}
			cleaned = cleanedAll
		}

		typeName, _ := cmd.Flags().GetString("type-name")
		snippet := workflow.BuildProfileSnippet(workflow.SnippetInput{
			TypeID:          typeID,
			TypeName:        typeName,
			Category:        category,
			WorkflowID:      wfID,
			WorkflowName:    wfName,
			DefaultStatusID: defaultStatusID,
			Statuses:        statuses,
			Edges:           probeRes.Edges,
		})
		profilePathWritten := ""
		if writeProfile {
			if pf == nil {
				handleErr(fmt.Errorf("--write-profile requires active --profile / YUNXIAO_PROFILE"))
				return
			}
			sw := snippet.Workflow
			pf.MergeWorkflow(typeID, profile.WorkitemWorkflow{
				TypeID:          sw.TypeID,
				Name:            sw.Name,
				Category:        sw.Category,
				WorkflowID:      sw.WorkflowID,
				WorkflowName:    sw.WorkflowName,
				DefaultStatusID: sw.DefaultStatusID,
				Statuses:        sw.Statuses,
				Edges:           sw.Edges,
			})
			// Keep legacy bug_* for +bug-transition when this is the profile bug type.
			if strings.EqualFold(category, "Bug") && (pf.BugTypeID == "" || pf.BugTypeID == typeID) {
				pf.MergeBugWorkflow(snippet.BugStatuses, snippet.BugEdges)
			}
			path, err := pf.Save()
			if err != nil {
				handleErr(err)
				return
			}
			profilePathWritten = path
		}

		statusOut := make([]map[string]any, 0, len(statuses))
		for _, s := range statuses {
			statusOut = append(statusOut, map[string]any{
				"id":          s.ID,
				"name":        s.Name,
				"displayName": s.DisplayName,
			})
		}

		data := map[string]any{
			"space_id":          spaceID,
			"type_id":           typeID,
			"workflow_id":       wfID,
			"workflow_name":     wfName,
			"default_status_id": defaultStatusID,
			"statuses":          statusOut,
			"edges":             probeRes.Edges,
			"required_hints":    probeRes.RequiredHints,
			"probe_item_id":     probeID,
			"probe_serial":      probeSerial,
			"probe_created":     len(createdIDs) > 0,
			"probe_created_ids": createdIDs,
			"probe_cleaned":     cleaned,
			"cleanup_errors":    cleanupErrors,
			"hard_to_reach":     probeRes.HardToReach,
			"final_status_id":   probeRes.FinalStatus,
			"attempts":          probeRes.Attempts,
			"edge_count":        workflow.CountEdges(probeRes.Edges),
			"profile_snippet":   snippet,
		}
		if profilePathWritten != "" {
			data["profile_written"] = profilePathWritten
		}
		meta := map[string]any{"risk": risk.Write}
		if pf != nil {
			meta["profile"] = pf.Name
		}
		handleErr(output.Success(data, meta))
	},
}

func fetchWorkItem(ctx context.Context, c *client.Client, id string) (map[string]any, error) {
	path, err := c.ProjexPath(ctx, "/workitems/"+id)
	if err != nil {
		return nil, err
	}
	var item map[string]any
	if err := c.Get(ctx, path, nil, &item); err != nil {
		return nil, err
	}
	return item, nil
}

func workitemTypeID(item map[string]any) string {
	if item == nil {
		return ""
	}
	if v, ok := item["workitemTypeId"]; ok {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return fmt.Sprintf("%.0f", t)
		}
	}
	if wt, ok := item["workitemType"].(map[string]any); ok {
		switch t := wt["id"].(type) {
		case string:
			return t
		case float64:
			return fmt.Sprintf("%.0f", t)
		}
	}
	return ""
}

func buildExploreCreateBody(ctx context.Context, pf *profile.Profile, c *client.Client, spaceID, typeID, category string) map[string]any {
	subject := fmt.Sprintf("[cli-explore] workflow probe %s", time.Now().Format("20060102-150405"))
	assigned := ""
	if pf != nil {
		assigned = pf.DefaultAssignedTo
	}
	if assigned == "" {
		assigned = "self"
	}
	if resolved, err := resolveSelfID(ctx, c, assigned); err == nil && resolved != "" {
		assigned = resolved
	}
	body := map[string]any{
		"spaceId":        spaceID,
		"workitemTypeId": typeID,
		"subject":        subject,
		"assignedTo":     assigned,
	}
	if strings.EqualFold(category, "Bug") && pf != nil {
		cf := map[string]any{}
		if id, err := pf.ResolvePriorityID("high"); err == nil && id != "" && id != "high" {
			cf["priority"] = id
		} else if id, err := pf.ResolvePriorityID("medium"); err == nil && id != "" && id != "medium" {
			cf["priority"] = id
		}
		if id, err := pf.ResolveSeriousLevelID("normal"); err == nil && id != "" && id != "normal" {
			cf["seriousLevel"] = id
		}
		if len(cf) > 0 {
			body["customFieldValues"] = cf
		}
	}
	if pf != nil {
		profile.ApplyWorkitemDefaults(body, typeID, pf)
	}
	return body
}

func init() {
	workitemExploreWorkflowCmd.Flags().String("type-id", "", "work item type id (required; or profile.bug_type_id when --category Bug)")
	workitemExploreWorkflowCmd.Flags().String("space-id", "", "project/space id (default: profile.space_id)")
	workitemExploreWorkflowCmd.Flags().String("id", "", "reuse existing work item as probe (never deleted)")
	workitemExploreWorkflowCmd.Flags().String("category", "Bug", "work item category Req|Bug|Task (create defaults + workflows entry)")
	workitemExploreWorkflowCmd.Flags().String("type-name", "", "optional type display name stored in workflows[type_id].name")
	workitemExploreWorkflowCmd.Flags().Bool("cleanup", false, "delete temp probe item if we created it (high-risk)")
	workitemExploreWorkflowCmd.Flags().Bool("write-profile", false, "merge into profile.workflows[type_id]; also bug_edges/bug_statuses when Bug matches bug_type_id")
	workitemCmd.AddCommand(workitemExploreWorkflowCmd)
}
