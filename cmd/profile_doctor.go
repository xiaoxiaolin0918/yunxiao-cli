package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/workflow"
	"github.com/yunxiao-cli/yunxiao/internal/workitemfields"
)

var profileDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diff profile field/status ids against live workitem fields + workflow",
	Long: `Risk: read

Fetches live workitem type fields and workflow for the profile bug_type_id
(and optionally each workflows[*].type_id) and diffs against profile
bug_fields / bug_create_fields / bug_statuses / bug_transition_required /
workflows edges / workitem_defaults field ids.

For the bug type it also diffs the allowed_environments / allowed_modules
snapshots (the client-side gate for +bug-create --environment / --module)
against the live options of bug_create_fields.environment / .module (#121);
drift fails the check.

  yunxiao profile doctor
  yunxiao profile doctor play
  yunxiao profile doctor --profile play
  yunxiao profile doctor --all-workflows

Report categories per id:
  ok                 — present on live type / workflow
  missing_on_type    — profile field id not found in live fields
  unknown_in_profile — live status id not listed in profile status maps (informational)
  status_not_in_workflow — profile status / edge target not in live workflow statuses
  enum_stale_in_profile   — allowed_* snapshot value no longer in the live field options
  enum_missing_in_profile — live option the allowed_* snapshot would reject
  enum_unverified         — field options unavailable, snapshot not checked (informational)
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		name := ""
		if len(args) > 0 {
			name = args[0]
		} else {
			name = activeProfileName()
		}
		if name == "" {
			handleErr(profile.HintMissing())
			return
		}
		pf, err := profile.Load(name)
		if err != nil {
			handleErr(err)
			return
		}
		if pf.SpaceID == "" {
			handleErr(fmt.Errorf("profile %q missing space_id", pf.Name))
			return
		}
		allWF, _ := cmd.Flags().GetBool("all-workflows")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}

		typeIDs := []string{}
		seen := map[string]bool{}
		if pf.BugTypeID != "" {
			typeIDs = append(typeIDs, pf.BugTypeID)
			seen[pf.BugTypeID] = true
		}
		if allWF {
			for tid := range pf.Workflows {
				if tid == "" || seen[tid] {
					continue
				}
				typeIDs = append(typeIDs, tid)
				seen[tid] = true
			}
			for tid := range pf.WorkitemDefaults {
				if tid == "" || seen[tid] {
					continue
				}
				typeIDs = append(typeIDs, tid)
				seen[tid] = true
			}
			sort.Strings(typeIDs)
		}

		if len(typeIDs) == 0 {
			handleErr(fmt.Errorf("profile %q has no bug_type_id / workflows to check", pf.Name))
			return
		}

		reports := make([]map[string]any, 0, len(typeIDs))
		summaryOK := true
		for _, typeID := range typeIDs {
			rep, ok := doctorType(cmd.Context(), c, pf, typeID)
			if !ok {
				summaryOK = false
			}
			reports = append(reports, rep)
		}

		defaultTypeIDs := make([]string, 0, len(pf.WorkitemDefaults))
		for tid := range pf.WorkitemDefaults {
			defaultTypeIDs = append(defaultTypeIDs, tid)
		}
		sort.Strings(defaultTypeIDs)
		defaultsSummary := make([]map[string]any, 0, len(defaultTypeIDs))
		for _, tid := range defaultTypeIDs {
			d := pf.WorkitemDefaults[tid]
			defaultsSummary = append(defaultsSummary, map[string]any{
				"type_id":         tid,
				"name":            d.Name,
				"category":        d.Category,
				"field_defaults":  len(d.Fields),
				"create_required": len(d.CreateRequired),
			})
		}

		out := map[string]any{
			"profile":           pf.Name,
			"space_id":          pf.SpaceID,
			"types":             reports,
			"workitem_defaults": defaultsSummary,
			"ok":                summaryOK,
		}
		handleErr(output.Success(out, map[string]any{"risk": risk.Read, "healthy": summaryOK}))
		if !summaryOK {
			handleErr(output.ExitError{Code: 1, Msg: "profile doctor found mismatches"})
		}
	},
}

func doctorType(ctx context.Context, c *client.Client, pf *profile.Profile, typeID string) (map[string]any, bool) {
	fieldsPath, err := c.ProjexPath(ctx, "/projects/"+pf.SpaceID+"/workitemTypes/"+typeID+"/fields")
	if err != nil {
		return map[string]any{"type_id": typeID, "error": err.Error()}, false
	}
	wfPath, err := c.ProjexPath(ctx, "/projects/"+pf.SpaceID+"/workitemTypes/"+typeID+"/workflows")
	if err != nil {
		return map[string]any{"type_id": typeID, "error": err.Error()}, false
	}

	var fieldsRaw any
	if err := c.Get(ctx, fieldsPath, nil, &fieldsRaw); err != nil {
		return map[string]any{"type_id": typeID, "error": err.Error()}, false
	}
	var wfRaw any
	if err := c.Get(ctx, wfPath, nil, &wfRaw); err != nil {
		return map[string]any{"type_id": typeID, "error": err.Error()}, false
	}

	liveFields := extractFieldIDs(fieldsRaw)
	// #121: full field configs (with options) for the allowed_* snapshot diff.
	parsedFields, fieldsParseErr := workitemfields.Parse(fieldsRaw)
	_, _, _, statuses, err := workflow.ParseWorkflowResponse(wfRaw)
	liveStatus := map[string]bool{}
	if err == nil {
		for _, s := range statuses {
			liveStatus[s.ID] = true
		}
	}

	isBugType := typeID == pf.BugTypeID
	findings := []map[string]any{}
	ok := true

	// Profile field ids to check against live fields.
	fieldRefs := map[string]string{} // id → source label
	if isBugType {
		for k, v := range pf.BugFields {
			if v != "" {
				fieldRefs[v] = "bug_fields." + k
			}
		}
		if pf.BugCreateFields.Module != "" {
			fieldRefs[pf.BugCreateFields.Module] = "bug_create_fields.module"
		}
		if pf.BugCreateFields.Environment != "" {
			fieldRefs[pf.BugCreateFields.Environment] = "bug_create_fields.environment"
		}
		if pf.BugCreateFields.ExpCompletionTime != "" {
			fieldRefs[pf.BugCreateFields.ExpCompletionTime] = "bug_create_fields.ExpCompletionTime"
		}
		for statusID, reqs := range pf.BugTransitionRequired {
			for _, fid := range reqs {
				if fid != "" {
					fieldRefs[fid] = "bug_transition_required." + statusID
				}
			}
		}
	}

	if wd, okWD := pf.WorkitemDefaults[typeID]; okWD {
		for fid := range wd.Fields {
			if fid != "" {
				if _, exists := fieldRefs[fid]; !exists {
					fieldRefs[fid] = "workitem_defaults.fields"
				}
			}
		}
		for _, fid := range wd.CreateRequired {
			if fid != "" {
				if _, exists := fieldRefs[fid]; !exists {
					fieldRefs[fid] = "workitem_defaults.create_required"
				}
			}
		}
	}

	for fid, src := range fieldRefs {
		entry := map[string]any{"id": fid, "source": src}
		if liveFields[fid] {
			entry["status"] = "ok"
		} else {
			entry["status"] = "missing_on_type"
			ok = false
		}
		findings = append(findings, entry)
	}

	// Status ids from profile maps / edges.
	statusRefs := map[string]string{}
	if isBugType {
		for alias, id := range pf.BugStatuses {
			if id != "" {
				statusRefs[id] = "bug_statuses." + alias
			}
		}
		for from, tos := range pf.BugEdges {
			if from != "" {
				statusRefs[from] = "bug_edges.from"
			}
			for _, to := range tos {
				if to != "" {
					statusRefs[to] = "bug_edges.to"
				}
			}
		}
		for statusID := range pf.BugTransitionRequired {
			if statusID != "" {
				statusRefs[statusID] = "bug_transition_required.key"
			}
		}
	}
	if wf, okWF := pf.Workflows[typeID]; okWF {
		for alias, id := range wf.Statuses {
			if id != "" {
				statusRefs[id] = "workflows.statuses." + alias
			}
		}
		for from, tos := range wf.Edges {
			if from != "" {
				statusRefs[from] = "workflows.edges.from"
			}
			for _, to := range tos {
				if to != "" {
					statusRefs[to] = "workflows.edges.to"
				}
			}
		}
	}

	for sid, src := range statusRefs {
		entry := map[string]any{"id": sid, "source": src}
		if err != nil {
			entry["status"] = "workflow_parse_error"
			entry["error"] = err.Error()
			ok = false
		} else if liveStatus[sid] {
			entry["status"] = "ok"
		} else {
			entry["status"] = "status_not_in_workflow"
			ok = false
		}
		findings = append(findings, entry)
	}

	// Informational: live statuses not mentioned in profile for this type.
	profileStatusSet := map[string]bool{}
	for sid := range statusRefs {
		profileStatusSet[sid] = true
	}
	unknown := []string{}
	for sid := range liveStatus {
		if !profileStatusSet[sid] {
			unknown = append(unknown, sid)
		}
	}
	sort.Strings(unknown)
	for _, sid := range unknown {
		findings = append(findings, map[string]any{
			"id":     sid,
			"source": "live_workflow",
			"status": "unknown_in_profile",
		})
	}

	// #121: allowed_* snapshots are tenant data snapshots; diff them against the live
	// field options of the bug type so drift is reported instead of silently gating
	// +bug-create with stale values.
	if isBugType {
		enumFindings, enumDrift := doctorAllowedEnumFindings(pf, parsedFields, fieldsParseErr)
		findings = append(findings, enumFindings...)
		if enumDrift {
			ok = false
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		a, _ := findings[i]["id"].(string)
		b, _ := findings[j]["id"].(string)
		sa, _ := findings[i]["status"].(string)
		sb, _ := findings[j]["status"].(string)
		if sa != sb {
			return sa < sb
		}
		return a < b
	})

	name := typeID
	if wf, okWF := pf.Workflows[typeID]; okWF && wf.Name != "" {
		name = wf.Name
	}
	counts := map[string]int{}
	for _, f := range findings {
		s, _ := f["status"].(string)
		counts[s]++
	}
	hasDefaults := false
	defaultFieldN := 0
	if wd, okWD := pf.WorkitemDefaults[typeID]; okWD {
		hasDefaults = true
		defaultFieldN = len(wd.Fields)
		if wd.Name != "" {
			name = wd.Name
		}
	}
	return map[string]any{
		"type_id":                 typeID,
		"name":                    name,
		"is_bug_type":             isBugType,
		"has_workitem_defaults":   hasDefaults,
		"workitem_default_fields": defaultFieldN,
		"live_fields":             len(liveFields),
		"live_statuses":           len(liveStatus),
		"counts":                  counts,
		"findings":                findings,
		"ok":                      ok,
	}, ok
}

// doctorAllowedEnumFindings diffs profile allowed_environments / allowed_modules
// snapshots against the live options of the matching bug-create fields (#121).
// Skips enums the profile does not gate (no field id or empty snapshot list);
// emits informational enum_unverified when the field exists but options are
// unavailable. Returns drift=true when the snapshot and live options disagree.
func doctorAllowedEnumFindings(pf *profile.Profile, fields []workitemfields.Field, fieldsParseErr error) (findings []map[string]any, drift bool) {
	if pf == nil {
		return nil, false
	}
	byID := make(map[string]workitemfields.Field, len(fields))
	for _, f := range fields {
		byID[f.ID] = f
	}
	checks := []struct {
		source  string   // profile key, also the finding source
		fieldID string   // live field whose options are the truth
		allowed []string // profile snapshot (empty = gate disabled)
	}{
		{"allowed_modules", pf.ModuleFieldID(), pf.AllowedModules},
		{"allowed_environments", pf.EnvironmentFieldID(), pf.AllowedEnvironments},
	}
	for _, ck := range checks {
		if ck.fieldID == "" || len(ck.allowed) == 0 {
			continue // profile does not gate this enum; nothing to verify
		}
		f, found := byID[ck.fieldID]
		if !found {
			if fieldsParseErr != nil {
				// Field ids could not be parsed at all — say so instead of silence.
				findings = append(findings, map[string]any{
					"id":     ck.fieldID,
					"source": ck.source,
					"status": "enum_unverified",
					"reason": fieldsParseErr.Error(),
				})
			}
			// Otherwise the field id itself is already reported missing_on_type.
			continue
		}
		var accepted, labels []string
		for _, o := range f.Options {
			for _, tok := range []string{o.ID, o.Value, o.DisplayValue} {
				if tok = strings.TrimSpace(tok); tok != "" {
					accepted = append(accepted, tok)
				}
			}
			lbl := o.DisplayValue
			if lbl = strings.TrimSpace(lbl); lbl == "" {
				lbl = strings.TrimSpace(o.Value)
			}
			if lbl != "" {
				labels = append(labels, lbl)
			}
		}
		if len(labels) == 0 {
			findings = append(findings, map[string]any{
				"id":     ck.fieldID,
				"source": ck.source,
				"status": "enum_unverified",
				"reason": "field has no options",
			})
			continue
		}
		stale, unlisted := profile.DiffAllowedEnum(ck.allowed, accepted, labels)
		for _, v := range stale {
			findings = append(findings, map[string]any{
				"id":           v,
				"source":       ck.source,
				"field_id":     ck.fieldID,
				"status":       "enum_stale_in_profile",
				"live_options": labels,
			})
			drift = true
		}
		for _, v := range unlisted {
			findings = append(findings, map[string]any{
				"id":       v,
				"source":   ck.source,
				"field_id": ck.fieldID,
				"status":   "enum_missing_in_profile",
				"allowed":  ck.allowed,
			})
			drift = true
		}
	}
	return findings, drift
}

func extractFieldIDs(raw any) map[string]bool {
	out := map[string]bool{}
	var list []any
	switch t := raw.(type) {
	case []any:
		list = t
	case map[string]any:
		if arr, ok := t["fields"].([]any); ok {
			list = arr
		} else if arr, ok := t["data"].([]any); ok {
			list = arr
		} else if arr, ok := t["result"].([]any); ok {
			list = arr
		}
	}
	for _, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id := ""
		for _, k := range []string{"id", "fieldIdentifier", "identifier"} {
			if v, ok := m[k]; ok && v != nil {
				id = fmt.Sprint(v)
				if id != "" && id != "<nil>" {
					break
				}
			}
		}
		if id != "" {
			out[id] = true
		}
	}
	return out
}

func init() {
	profileDoctorCmd.Flags().Bool("all-workflows", false, "also check each workflows[*].type_id (default: bug_type_id only)")
	profileCmd.AddCommand(profileDoctorCmd)
}
