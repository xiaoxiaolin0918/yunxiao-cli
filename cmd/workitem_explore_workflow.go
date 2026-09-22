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

--category defaults to Bug for backward compatibility, but the command resolves the real
category from --type-id (profile workitem_defaults/workflows, else types list API) and
auto-overrides a mismatched default. Explicit --category that disagrees with the type
errors clearly. If category cannot be resolved, pass --category explicitly (avoids
injecting Bug fields into Req/Task create and mixed HTTP 400s; issue 60).

MVP (#61): needs_fields outcomes are hinted_edges (not verified edges). Pass
--custom-fields on probe create (plus profile workitem_defaults) and optional --fields on
each PUT. Optional --from <status> repositions before probing. Per-status required-field
tables (generalized bug_transition_required) are deferred.

Do not run against production ZYPT without an explicit probe item and review.`,
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
		customFieldsJSON, _ := cmd.Flags().GetString("custom-fields")
		putFieldsJSON, _ := cmd.Flags().GetString("fields")
		fromStatusFlag, _ := cmd.Flags().GetString("from")
		customFields, err := parseJSONMap(customFieldsJSON)
		if err != nil {
			handleErr(fmt.Errorf("--custom-fields: %w", err))
			return
		}
		putFields, err := parseJSONMap(putFieldsJSON)
		if err != nil {
			handleErr(fmt.Errorf("--fields: %w", err))
			return
		}
		if putFields == nil {
			putFields = map[string]any{}
		}

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

		categoryChanged := cmd.Flags().Changed("category")
		resolvedCat, lookupErr := lookupExploreCategory(cmd.Context(), c, pf, spaceID, typeID)
		category, categoryNote, err := resolveExploreCategory(category, categoryChanged, resolvedCat, lookupErr)
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
				"from_status":   fromStatusFlag,
				"custom_fields": customFields,
				"put_fields":    putFields,
				"steps": []string{
					"GET type workflow statuses",
					"obtain probe work item (reuse --id or POST create temp)",
					"for each status pair (from,to): ensure at from; PUT status=to; record edge/hint",
					"optional DELETE temp probe if --cleanup",
					"optional merge workflows[type_id] (+ legacy bug_* for Bug) if --write-profile",
				},
				"get_workflow": c.Preview("GET", wfPath, nil, nil),
			}
			if categoryNote != "" {
				plan["category_note"] = categoryNote
			}
			if existingID == "" {
				createBody := buildExploreCreateBody(cmd.Context(), pf, c, spaceID, typeID, category, customFields)
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
			body := buildExploreCreateBody(cmd.Context(), pf, c, spaceID, typeID, category, customFields)
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


		if strings.TrimSpace(fromStatusFlag) != "" {
			fromID, ferr := workflow.ResolveUniqueStatus(fromStatusFlag, statuses)
			if ferr != nil {
				handleErr(fmt.Errorf("--from: %w", ferr))
				return
			}
			if startStatus != fromID {
				putPath, perr := c.ProjexPath(cmd.Context(), "/workitems/"+probeID)
				if perr != nil {
					handleErr(perr)
					return
				}
				body := map[string]any{"status": fromID}
				for k, v := range putFields {
					body[k] = v
				}
				var out any
				if err := c.Put(cmd.Context(), putPath, body, &out); err != nil {
					handleErr(fmt.Errorf("--from %s: could not move probe to status %s: %w (pass --fields for required transition fields)", fromStatusFlag, fromID, err))
					return
				}
				if item, gerr := fetchWorkItem(cmd.Context(), c, probeID); gerr == nil {
					if cur := zhiyi.CurrentStatusID(item); cur != "" {
						startStatus = cur
					} else {
						startStatus = fromID
					}
				} else {
					startStatus = fromID
				}
			}
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
				body := map[string]any{"status": to}
				for k, v := range putFields {
					body[k] = v
				}
				return c.Put(cmd.Context(), putPath, body, &out)
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
			HintedEdges:     probeRes.HintedEdges,
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
				HintedEdges:     sw.HintedEdges,
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
			"category":          category,
			"workflow_id":       wfID,
			"workflow_name":     wfName,
			"default_status_id": defaultStatusID,
			"statuses":          statusOut,
			"edges":             probeRes.Edges, // verified only (#61)
			"verified_edges":    probeRes.Edges,
			"hinted_edges":      probeRes.HintedEdges,
			"required_hints":    probeRes.RequiredHints,
			"missing_fields":    probeRes.MissingFields,
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
			"hinted_edge_count": workflow.CountEdges(probeRes.HintedEdges),
			"profile_snippet":   snippet,
		}
		if categoryNote != "" {
			data["category_note"] = categoryNote
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

// exploreTypeCategories is the Projex category set probed when resolving type-id.
var exploreTypeCategories = []string{"Req", "Bug", "Task", "Risk", "Topic"}

func categoryFromProfile(pf *profile.Profile, typeID string) string {
	typeID = strings.TrimSpace(typeID)
	if pf == nil || typeID == "" {
		return ""
	}
	if pf.WorkitemDefaults != nil {
		if d, ok := pf.WorkitemDefaults[typeID]; ok {
			if c := strings.TrimSpace(d.Category); c != "" {
				return c
			}
		}
	}
	if pf.Workflows != nil {
		if wf, ok := pf.Workflows[typeID]; ok {
			if c := strings.TrimSpace(wf.Category); c != "" {
				return c
			}
		}
	}
	if strings.TrimSpace(pf.BugTypeID) != "" && pf.BugTypeID == typeID {
		return "Bug"
	}
	return ""
}

func typeIDInWorkitemTypesList(raw any, typeID string) bool {
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return false
	}
	items := client.ExtractListItems(raw)
	if items == nil {
		if s, ok := raw.([]any); ok {
			items = s
		}
	}
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok || m == nil {
			continue
		}
		id := ""
		switch v := m["id"].(type) {
		case string:
			id = v
		case float64:
			id = fmt.Sprintf("%.0f", v)
		default:
			if v != nil {
				id = strings.TrimSpace(fmt.Sprint(v))
			}
		}
		if id == typeID {
			return true
		}
	}
	return false
}

func exploreCategoriesPrefer(preferred string) []string {
	preferred = strings.TrimSpace(preferred)
	out := make([]string, 0, len(exploreTypeCategories)+1)
	seen := map[string]struct{}{}
	add := func(c string) {
		c = strings.TrimSpace(c)
		if c == "" {
			return
		}
		key := strings.ToLower(c)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	add(preferred)
	for _, c := range exploreTypeCategories {
		add(c)
	}
	return out
}

func lookupTypeCategoryAPI(ctx context.Context, c *client.Client, spaceID, typeID string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("nil client")
	}
	path, err := c.ProjexPath(ctx, "/projects/"+spaceID+"/workitemTypes")
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, cat := range exploreCategoriesPrefer("") {
		var raw any
		if err := c.Get(ctx, path, map[string]string{"category": cat}, &raw); err != nil {
			lastErr = err
			continue
		}
		if typeIDInWorkitemTypesList(raw, typeID) {
			return cat, nil
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("type-id %s not found in categories %v (last list error: %w)", typeID, exploreTypeCategories, lastErr)
	}
	return "", fmt.Errorf("type-id %s not found in categories %v", typeID, exploreTypeCategories)
}

func lookupExploreCategory(ctx context.Context, c *client.Client, pf *profile.Profile, spaceID, typeID string) (string, error) {
	if cat := categoryFromProfile(pf, typeID); cat != "" {
		return cat, nil
	}
	if c == nil || strings.TrimSpace(spaceID) == "" || strings.TrimSpace(typeID) == "" {
		return "", fmt.Errorf("no profile category for type-id %s and cannot query types list", typeID)
	}
	return lookupTypeCategoryAPI(ctx, c, spaceID, typeID)
}

// resolveExploreCategory applies issue 60 rules: auto-override default --category when
// type lookup succeeds; error on explicit mismatch; error when default cannot be resolved.
func resolveExploreCategory(flagCategory string, categoryChanged bool, resolved string, lookupErr error) (category string, note string, err error) {
	flagCategory = strings.TrimSpace(flagCategory)
	if flagCategory == "" {
		flagCategory = "Bug"
	}
	resolved = strings.TrimSpace(resolved)
	if resolved != "" {
		if strings.EqualFold(flagCategory, resolved) {
			return resolved, "", nil
		}
		if categoryChanged {
			return "", "", fmt.Errorf("--category %s does not match type-id category %s; omit --category to auto-select, or pass --category %s", flagCategory, resolved, resolved)
		}
		return resolved, fmt.Sprintf("auto-overrode --category from %s to %s (resolved from type-id)", flagCategory, resolved), nil
	}
	if categoryChanged {
		note = "could not resolve category from type-id; using explicit --category"
		if lookupErr != nil {
			note = note + ": " + lookupErr.Error()
		}
		return flagCategory, note, nil
	}
	msg := "cannot resolve category for type-id; pass --category explicitly (Req|Bug|Task|…). Default --category Bug would inject Bug create fields and may cause mixed HTTP 400s on non-Bug types"
	if lookupErr != nil {
		msg = msg + ": " + lookupErr.Error()
	}
	return "", "", fmt.Errorf("%s", msg)
}

func buildExploreCreateBody(ctx context.Context, pf *profile.Profile, c *client.Client, spaceID, typeID, category string, customFields map[string]any) map[string]any {
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

	if len(customFields) > 0 {
		cf, _ := body["customFieldValues"].(map[string]any)
		if cf == nil {
			cf = map[string]any{}
		}
		for k, v := range customFields {
			cf[k] = v
		}
		body["customFieldValues"] = cf
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
	workitemExploreWorkflowCmd.Flags().String("category", "Bug", "work item category Req|Bug|Task|… (default Bug; auto-overridden from type-id when mismatched)")
	workitemExploreWorkflowCmd.Flags().String("type-name", "", "optional type display name stored in workflows[type_id].name")
	workitemExploreWorkflowCmd.Flags().Bool("cleanup", false, "delete temp probe item if we created it (high-risk)")
	workitemExploreWorkflowCmd.Flags().Bool("write-profile", false, "merge into profile.workflows[type_id]; also bug_edges/bug_statuses when Bug matches bug_type_id")
	workitemExploreWorkflowCmd.Flags().String("custom-fields", "", "JSON object merged into probe create customFieldValues (after Bug helpers; before profile workitem_defaults)")
	workitemExploreWorkflowCmd.Flags().String("fields", "", "JSON object merged into every probe PUT body alongside status (transition required fields)")
	workitemExploreWorkflowCmd.Flags().String("from", "", "optional status id/name: move probe here (using --fields) before probing")
	workitemCmd.AddCommand(workitemExploreWorkflowCmd)
}
