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

Safety (#412): a real run is allowed only in a YXCLI sandbox, identified from the active
profile as serial_prefix=YXCLI (the play example) or a profile named play with an empty
prefix. serial_prefix=ZYPT (zhiyi / 智衣 / production) is never treated as a sandbox.
Recommended path: --profile play --type-id <id> --cleanup --yes (optionally --write-profile).
Creating probes without --cleanup is refused so leftover "[cli-explore] workflow probe …"
items cannot pollute a space. --id reuses an existing work item, performs real status
PUTs, and may move it into a terminal status with no outbound edge; afterwards CLI/API
cannot be reverted — this is refused unless you also pass --i-know-this-mutates-real-item.
Writes against a non-sandbox space (including --write-profile) also require the explicit
escape hatch --allow-non-sandbox; do not use that on production ZYPT.

  yunxiao workitem +explore-workflow --profile play \
    --type-id <type-id> --dry-run

  yunxiao workitem +explore-workflow --profile play \
    --type-id <type-id> --cleanup --yes

  yunxiao workitem +explore-workflow --profile play \
    --type-id <type-id> --cleanup --write-profile --yes

--space-id defaults to active profile.space_id.
--id reuses an existing work item of that type (never deleted). Requires
--i-know-this-mutates-real-item because status PUTs are real and a terminal status
cannot be reverted via CLI/API.
--cleanup deletes a temp item we created (high-risk; needs --yes). Required when the
command creates the probe.
--allow-non-sandbox is the explicit, hard-to-typo escape hatch for a non-YXCLI space
(still requires --cleanup / --i-know-this-mutates-real-item as applicable).
--write-profile writes into profile.workflows[<type-id>] (project-scoped profile via space_id;
workflows are per type_id). For Bug category when type-id matches profile.bug_type_id (or
bug_type_id is empty), also merges legacy bug_edges / bug_statuses for +bug-transition.
--write-profile requires an active --profile / YUNXIAO_PROFILE; this is validated before any
request is sent (exit 1, zero API calls) (#110). On a non-sandbox profile it also needs
--allow-non-sandbox (#412).
--dry-run is fully offline (#110): zero API requests (plan.network_reads is always 0).
Category is resolved from the active profile only — pass --category explicitly when the
profile has none for the type-id. Workflow statuses are not fetched and assignedTo "self"
is not resolved; the real run performs the reads listed in plan.steps before any write.
The dry-run plan records sandbox / real_run_blocked so a blocked real run is visible
without sending requests.

--category defaults to Bug for backward compatibility, but the command resolves the real
category from --type-id (profile workitem_defaults/workflows, else types list API) and
auto-overrides a mismatched default. Explicit --category that disagrees with the type
errors clearly. If category cannot be resolved, pass --category explicitly (avoids
injecting Bug fields into Req/Task create and mixed HTTP 400s; issue 60).

MVP (#61): needs_fields outcomes are hinted_edges (not verified edges). Pass
--custom-fields on probe create (plus profile workitem_defaults) and optional --fields on
each PUT. Optional --from <status> repositions before probing. Per-status required-field
tables (generalized bug_transition_required) are deferred.

Do not use --allow-non-sandbox on production ZYPT.`,
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

		// #110: validate --write-profile flag coherence BEFORE anything else
		// (client creation, dry-run preview, reads, writes). A missing profile
		// must fail fast with zero API calls, never after probes were created,
		// transitioned, or cleaned up.
		if writeProfile && pf == nil {
			handleErr(fmt.Errorf("--write-profile requires active --profile / YUNXIAO_PROFILE"))
			return
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

		allowNonSandbox, _ := cmd.Flags().GetBool("allow-non-sandbox")
		mutateRealItem, _ := cmd.Flags().GetBool("i-know-this-mutates-real-item")
		safetyDenial := checkExploreWorkflowSafety(exploreWorkflowSafetyInput{
			Profile:         pf,
			ExistingID:      existingID,
			Cleanup:         cleanup,
			WriteProfile:    writeProfile,
			AllowNonSandbox: allowNonSandbox,
			MutateRealItem:  mutateRealItem,
		})
		// #412: block real runs before mustClient / any API request. --dry-run
		// stays fully offline and only records the same decision in the plan.
		if !globalDryRun && safetyDenial != nil {
			handleErr(safetyDenial.err())
			return
		}

		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}

		categoryChanged := cmd.Flags().Changed("category")

		// Dry-run: plan only, fully offline (#110) — zero API requests.
		if globalDryRun {
			// Category comes from the active profile only; the types list API
			// is a real-run read (pass --category when the profile has none).
			resolvedCat := categoryFromProfile(pf, typeID)
			var lookupErr error
			if resolvedCat == "" {
				lookupErr = fmt.Errorf("dry-run is offline; category resolved from the active profile only (the real run queries the types list API)")
			}
			category, categoryNote, err := resolveExploreCategory(category, categoryChanged, resolvedCat, lookupErr)
			if err != nil {
				handleErr(err)
				return
			}

			wfSuffix := "/projects/" + spaceID + "/workitemTypes/" + typeID + "/workflows"
			wfPreviewPath, wfOK := c.ProjexPathOffline(wfSuffix)
			if !wfOK {
				wfPreviewPath = "/oapi/v1/projex/organizations/<org-id>" + wfSuffix
			}
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
				"network_reads": 0,
				"steps": []string{
					"GET type workflow statuses",
					"obtain probe work item (reuse --id or POST create temp)",
					"for each status pair (from,to): ensure at from; PUT status=to; record edge/hint",
					"optional DELETE temp probe if --cleanup",
					"optional merge workflows[type_id] (+ legacy bug_* for Bug) if --write-profile",
				},
				"get_workflow": c.Preview("GET", wfPreviewPath, nil, nil),
			}
			sandbox, sandboxReason := isExploreWorkflowSandbox(pf)
			plan["sandbox"] = sandbox
			plan["sandbox_reason"] = sandboxReason
			if safetyDenial != nil {
				plan["real_run_blocked"] = true
				plan["safety_error"] = safetyDenial.Message
				plan["safety_hint"] = safetyDenial.Hint
			} else {
				plan["real_run_blocked"] = false
			}
			if categoryNote != "" {
				plan["category_note"] = categoryNote
			}
			if existingID == "" {
				createBody := buildExploreCreateBodyOffline(pf, spaceID, typeID, category, customFields)
				createPreviewPath, createOK := c.ProjexPathOffline("/workitems")
				if !createOK {
					createPreviewPath = "/oapi/v1/projex/organizations/<org-id>/workitems"
				}
				plan["create_probe"] = c.Preview("POST", createPreviewPath, nil, createBody)
				if assigned, _ := createBody["assignedTo"].(string); assigned == "self" {
					plan["create_probe_note"] = `assignedTo "self" is resolved to the current user id only in the real run`
				}
			} else {
				plan["reuse_id"] = existingID
			}
			handleErr(output.DryRunResult(string(risk.Write), plan))
			return
		}

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
		cleanupCreatedProbes := func() (bool, []string) {
			if !cleanup || len(createdIDs) == 0 {
				return false, nil
			}
			errs := []string{}
			cleanedAll := true
			for _, id := range createdIDs {
				delPath, err := c.ProjexPath(cmd.Context(), "/workitems/"+id)
				if err != nil {
					errs = append(errs, err.Error())
					cleanedAll = false
					continue
				}
				var delOut any
				if err := c.Delete(cmd.Context(), delPath, nil, &delOut); err != nil {
					errs = append(errs, err.Error())
					cleanedAll = false
					continue
				}
			}
			return cleanedAll, errs
		}
		failAfterProbes := func(err error) {
			if err == nil {
				return
			}
			cleaned, cleanupErrs := cleanupCreatedProbes()
			hint := exploreIrreversibleHint
			if len(createdIDs) > 0 && !cleaned {
				hint = hint + "; leftover probes are listed in error.details.probe_created_ids — delete them with workitem delete"
			}
			handleErr(&detailedError{
				Subtype: "explore_workflow_failed",
				Message: err.Error(),
				Hint:    hint,
				Details: map[string]any{
					"probe_created_ids": createdIDs,
					"probe_item_id":     probeID,
					"probe_cleaned":     cleaned,
					"cleanup_errors":    cleanupErrs,
				},
			})
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
					failAfterProbes(fmt.Errorf("--from %s: could not move probe to status %s: %w (pass --fields for required transition fields)", fromStatusFlag, fromID, err))
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
			failAfterProbes(err)
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
			// pf != nil is guaranteed by the pre-flight check at the top (#110).
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

const (
	exploreSandboxSerialPrefix    = "YXCLI"
	exploreProductionSerialPrefix = "ZYPT"
)

type exploreWorkflowSafetyInput struct {
	Profile         *profile.Profile
	ExistingID      string
	Cleanup         bool
	WriteProfile    bool
	AllowNonSandbox bool
	MutateRealItem  bool
}

type exploreWorkflowSafetyDenial struct {
	Message string
	Hint    string
	Details map[string]any
}

func (d *exploreWorkflowSafetyDenial) err() error {
	if d == nil {
		return nil
	}
	return &detailedError{
		Subtype: "explore_workflow_safety",
		Message: d.Message,
		Hint:    d.Hint,
		Details: d.Details,
	}
}

// isExploreWorkflowSandbox reports whether the active profile is the documented
// YXCLI / play sandbox. Identification follows repo conventions:
// profiles/play.example.json sets serial_prefix=YXCLI; zhiyi.example.json sets
// ZYPT for 智衣/production. A custom profile is a sandbox only when its
// serial_prefix is YXCLI. Profile name "play" with an empty prefix is also
// accepted (installed play example whose prefix was left unset). Name "play"
// with serial_prefix=ZYPT is not a sandbox.
func isExploreWorkflowSandbox(pf *profile.Profile) (bool, string) {
	if pf == nil {
		return false, "no active profile; set --profile play or a profile with serial_prefix=YXCLI"
	}
	prefix := strings.ToUpper(strings.TrimSpace(pf.SerialPrefix))
	name := strings.TrimSpace(pf.Name)
	if prefix == exploreSandboxSerialPrefix {
		return true, "serial_prefix=YXCLI"
	}
	if prefix == exploreProductionSerialPrefix {
		return false, fmt.Sprintf("profile %q serial_prefix=ZYPT (zhiyi / 智衣 / production)", name)
	}
	if strings.EqualFold(name, "play") && prefix == "" {
		return true, "profile name play with empty serial_prefix"
	}
	if name == "" {
		return false, fmt.Sprintf("serial_prefix=%q is not a YXCLI sandbox", pf.SerialPrefix)
	}
	return false, fmt.Sprintf("profile %q serial_prefix=%q is not a YXCLI sandbox", name, pf.SerialPrefix)
}

func exploreSafetyDetails(in exploreWorkflowSafetyInput, extra map[string]any) map[string]any {
	details := map[string]any{
		"cleanup":           in.Cleanup,
		"write_profile":     in.WriteProfile,
		"allow_non_sandbox": in.AllowNonSandbox,
		"mutate_real_item":  in.MutateRealItem,
		"existing_id":       strings.TrimSpace(in.ExistingID),
	}
	if in.Profile != nil {
		details["profile"] = in.Profile.Name
		details["serial_prefix"] = in.Profile.SerialPrefix
		details["space_id"] = in.Profile.SpaceID
	}
	sandbox, reason := isExploreWorkflowSandbox(in.Profile)
	details["sandbox"] = sandbox
	details["sandbox_reason"] = reason
	for k, v := range extra {
		details[k] = v
	}
	return details
}

const exploreRecommendedPath = "recommended: yunxiao workitem +explore-workflow --profile play --type-id <type-id> --cleanup --yes  (YXCLI sandbox / play profile; space 「yunxiao-cli 沙箱」)"

const exploreIrreversibleHint = "real status PUTs may move the work item into a terminal status with no outbound edge; afterwards CLI/API cannot be reverted (a workflow change in the web UI may be required)"

func checkExploreWorkflowSafety(in exploreWorkflowSafetyInput) *exploreWorkflowSafetyDenial {
	if strings.TrimSpace(in.ExistingID) != "" && !in.MutateRealItem {
		return &exploreWorkflowSafetyDenial{
			Message: "--id reuses a real work item as the probe and will perform real status PUTs; refusing without --i-know-this-mutates-real-item (CLI/API cannot be reverted if the item reaches a terminal status)",
			Hint:    exploreIrreversibleHint + ". Pass --i-know-this-mutates-real-item only if you accept that risk. Prefer a self-created sandbox probe with --cleanup. " + exploreRecommendedPath,
			Details: exploreSafetyDetails(in, map[string]any{"reason": "real_item_id"}),
		}
	}
	if strings.TrimSpace(in.ExistingID) == "" && !in.Cleanup {
		return &exploreWorkflowSafetyDenial{
			Message: "refusing to create probe work items without --cleanup (leftover titles like \"[cli-explore] workflow probe …\" pollute the space)",
			Hint:    "pass --cleanup so temp probes are deleted after the run. " + exploreRecommendedPath,
			Details: exploreSafetyDetails(in, map[string]any{"reason": "cleanup_required"}),
		}
	}
	sandbox, reason := isExploreWorkflowSandbox(in.Profile)
	if !sandbox && !in.AllowNonSandbox {
		return &exploreWorkflowSafetyDenial{
			Message: "refusing +explore-workflow writes (probe create/PUT, --id, --write-profile) against a non-sandbox space: " + reason,
			Hint:    "identify a sandbox by profile serial_prefix=YXCLI (play example) or profile name play with an empty prefix. Do not run this on production ZYPT / 智衣. To override, pass --allow-non-sandbox (hard to typo; still requires --cleanup for creates and --i-know-this-mutates-real-item for --id). " + exploreRecommendedPath,
			Details: exploreSafetyDetails(in, map[string]any{"reason": "non_sandbox"}),
		}
	}
	return nil
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

// exploreDefaultAssignee returns the probe assignee before "self" resolution.
func exploreDefaultAssignee(pf *profile.Profile) string {
	assigned := ""
	if pf != nil {
		assigned = pf.DefaultAssignedTo
	}
	if assigned == "" {
		assigned = "self"
	}
	return assigned
}

func buildExploreCreateBody(ctx context.Context, pf *profile.Profile, c *client.Client, spaceID, typeID, category string, customFields map[string]any) map[string]any {
	assigned := exploreDefaultAssignee(pf)
	if resolved, err := resolveSelfID(ctx, c, assigned); err == nil && resolved != "" {
		assigned = resolved
	}
	return buildExploreCreateBodyFor(pf, spaceID, typeID, category, customFields, assigned)
}

// buildExploreCreateBodyOffline builds the dry-run probe-create preview without
// any API request (#110): assignedTo stays "self" (the real run resolves it to
// the current user id via GET /platform/user).
func buildExploreCreateBodyOffline(pf *profile.Profile, spaceID, typeID, category string, customFields map[string]any) map[string]any {
	return buildExploreCreateBodyFor(pf, spaceID, typeID, category, customFields, exploreDefaultAssignee(pf))
}

func buildExploreCreateBodyFor(pf *profile.Profile, spaceID, typeID, category string, customFields map[string]any, assigned string) map[string]any {
	subject := fmt.Sprintf("[cli-explore] workflow probe %s", time.Now().Format("20060102-150405"))
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
	workitemExploreWorkflowCmd.Flags().String("id", "", "reuse existing work item as probe (never deleted; requires --i-know-this-mutates-real-item, #412)")
	workitemExploreWorkflowCmd.Flags().String("category", "Bug", "work item category Req|Bug|Task|… (default Bug; auto-overridden from type-id when mismatched)")
	workitemExploreWorkflowCmd.Flags().String("type-name", "", "optional type display name stored in workflows[type_id].name")
	workitemExploreWorkflowCmd.Flags().Bool("cleanup", false, "delete temp probe item if we created it (high-risk; required when the command creates the probe, #412)")
	workitemExploreWorkflowCmd.Flags().Bool("allow-non-sandbox", false, "explicit escape hatch to run writes against a non-YXCLI space (do not use on production ZYPT / 智衣; #412)")
	workitemExploreWorkflowCmd.Flags().Bool("i-know-this-mutates-real-item", false, "required with --id: confirm real status PUTs; a terminal status cannot be reverted via CLI/API (#412)")
	workitemExploreWorkflowCmd.Flags().Bool("write-profile", false, "merge into profile.workflows[type_id]; also bug_edges/bug_statuses when Bug matches bug_type_id (non-sandbox also needs --allow-non-sandbox, #412)")
	workitemExploreWorkflowCmd.Flags().String("custom-fields", "", "JSON object merged into probe create customFieldValues (after Bug helpers; before profile workitem_defaults)")
	workitemExploreWorkflowCmd.Flags().String("fields", "", "JSON object merged into every probe PUT body alongside status (transition required fields)")
	workitemExploreWorkflowCmd.Flags().String("from", "", "optional status id/name: move probe here (using --fields) before probing")
	workitemCmd.AddCommand(workitemExploreWorkflowCmd)
}
