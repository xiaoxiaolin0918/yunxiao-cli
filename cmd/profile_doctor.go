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
	Long: `Risk: read (--write: write — merges suggested status ids and allowed_* enum drift into the local profile file only; never writes to the API)

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
  yunxiao profile doctor --fix-suggest=false
  yunxiao profile doctor --write             # backfill status ids + allowed_* enum drift
  yunxiao profile doctor --write --dry-run   # preview the backfill, no file write

Report categories per id:
  ok                 — present on live type / workflow
  missing_on_type    — profile field id not found in live fields (similar_fields hints same-name live fields)
  unknown_in_profile — live status id not listed in profile status maps (informational)
  status_not_in_workflow — profile status / edge target not in live workflow statuses
  enum_stale_in_profile   — allowed_* snapshot value no longer in the live field options
  enum_missing_in_profile — live option the allowed_* snapshot would reject
  enum_unverified         — field options unavailable, snapshot not checked (informational)

Findings carry live metadata (#120): status findings add display_name / name_en
from the live workflow; field findings add field_name from the live field config;
missing_on_type adds similar_fields (live fields whose name matches the profile key).

--fix-suggest (default true) attaches suggestions to unknown_in_profile findings:
an alias guessed from nameEn/displayName plus the exact profile keys to backfill.
--write applies those suggestions: bug_statuses[alias] for the bug type and
workflows[type_id].statuses[displayName] for every checked type. For the bug type it
also applies allowed_modules / allowed_environments drift: remove enum_stale_in_profile
values and append enum_missing_in_profile labels. Edges and other probe-discovered
fields are never touched. Status keys that already map to another id are reported as
suggest_conflict and skipped. Prefer --dry-run first.
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
		fixSuggest, _ := cmd.Flags().GetBool("fix-suggest")
		writeFlag, _ := cmd.Flags().GetBool("write")
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
		allSuggestions := make([]doctorTypeSuggestions, 0, len(typeIDs))
		summaryOK := true
		for _, typeID := range typeIDs {
			rep, sugg, ok := doctorType(cmd.Context(), c, pf, typeID, fixSuggest)
			if !ok {
				summaryOK = false
			}
			reports = append(reports, rep)
			if len(sugg.suggestions) > 0 || len(sugg.enums) > 0 {
				allSuggestions = append(allSuggestions, sugg)
			}
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
		meta := map[string]any{"risk": risk.Read, "healthy": summaryOK, "fix_suggest": fixSuggest}

		if writeFlag {
			applied, skipped := doctorWritePlan(allSuggestions)
			if globalDryRun {
				preview := map[string]any{
					"action":  "profile doctor --write",
					"profile": pf.Name,
					"applied": applied,
					"skipped": skipped,
					"report":  out,
				}
				if dst, perr := profile.Path(pf.Name); perr == nil {
					preview["to"] = dst
				}
				handleErr(output.DryRunResult(string(risk.Write), preview))
				if !summaryOK {
					handleErr(output.ExitError{Code: 1, Msg: "profile doctor found mismatches"})
				}
				return
			}
			meta["risk"] = risk.Write
			writeOut := map[string]any{
				"applied": applied,
				"skipped": skipped,
				"saved":   false,
			}
			if len(applied) > 0 {
				doctorApplyWrites(pf, applied)
				path, serr := pf.Save()
				if serr != nil {
					handleErr(serr)
					return
				}
				writeOut["saved"] = true
				writeOut["path"] = path
			}
			out["write"] = writeOut
		}

		handleErr(output.Success(out, meta))
		if !summaryOK {
			handleErr(output.ExitError{Code: 1, Msg: "profile doctor found mismatches"})
		}
	},
}

// doctorSuggestTarget is one profile key a suggested status id can be backfilled into.
type doctorSuggestTarget struct {
	Map      string // "bug_statuses" | "workflows.statuses"
	Key      string // alias (bug_statuses) or displayName/name (workflows.statuses)
	Value    string // live status id
	Conflict string // non-empty: the key already maps to a different id (write skips it)
}

// doctorStatusSuggestion is the backfill plan for one unknown_in_profile status (#120).
type doctorStatusSuggestion struct {
	StatusID    string
	DisplayName string
	NameEn      string
	Alias       string // canonical profile alias ("" when ambiguous/unknown)
	Targets     []doctorSuggestTarget
}

// doctorTypeSuggestions groups one type's suggestions with its type metadata.
type doctorTypeSuggestions struct {
	typeID      string
	isBugType   bool
	suggestions []doctorStatusSuggestion
	enums       []doctorEnumSuggestion
}

// doctorEnumSuggestion is one allowed_* add/remove planned by --write.
type doctorEnumSuggestion struct {
	Source string // allowed_modules | allowed_environments
	Value  string
	Op     string // "add" | "remove"
}

// doctorWriteEntry is one applied (or previewed) profile backfill (#120).
type doctorWriteEntry struct {
	TypeID string `json:"type_id,omitempty"`
	Map    string `json:"map"` // "bug_statuses" | "workflows.statuses" | "allowed_modules" | "allowed_environments"
	Key    string `json:"key,omitempty"`
	Value  string `json:"value"`
	Op     string `json:"op,omitempty"` // ""|"set" for statuses; "add"|"remove" for allowed_*
}

// doctorSkipEntry is a backfill blocked by an existing mapping.
type doctorSkipEntry struct {
	doctorWriteEntry
	Reason string `json:"reason"`
}

// doctorWritePlan splits suggestions into applyable entries and conflict skips.
// Pure: never touches the profile, so --dry-run previews the exact same plan.
func doctorWritePlan(suggs []doctorTypeSuggestions) ([]doctorWriteEntry, []doctorSkipEntry) {
	applied := []doctorWriteEntry{}
	skipped := []doctorSkipEntry{}
	for _, ts := range suggs {
		for _, s := range ts.suggestions {
			for _, tgt := range s.Targets {
				e := doctorWriteEntry{TypeID: ts.typeID, Map: tgt.Map, Key: tgt.Key, Value: tgt.Value, Op: "set"}
				if tgt.Conflict != "" {
					skipped = append(skipped, doctorSkipEntry{doctorWriteEntry: e, Reason: tgt.Conflict})
					continue
				}
				applied = append(applied, e)
			}
		}
		for _, e := range ts.enums {
			applied = append(applied, doctorWriteEntry{
				TypeID: ts.typeID,
				Map:    e.Source,
				Value:  e.Value,
				Op:     e.Op,
			})
		}
	}
	return applied, skipped
}

// doctorApplyWrites merges the planned entries into the in-memory profile.
// Statuses only: edges and hinted_edges are never touched (#120).
func doctorApplyWrites(pf *profile.Profile, applied []doctorWriteEntry) {
	if pf == nil {
		return
	}
	for _, e := range applied {
		switch e.Map {
		case "bug_statuses":
			if pf.BugStatuses == nil {
				pf.BugStatuses = map[string]string{}
			}
			pf.BugStatuses[e.Key] = e.Value
		case "workflows.statuses":
			pf.MergeWorkflow(e.TypeID, profile.WorkitemWorkflow{
				TypeID:   e.TypeID,
				Statuses: map[string]string{e.Key: e.Value},
			})
		case "allowed_modules":
			pf.AllowedModules = doctorApplyEnumList(pf.AllowedModules, e.Value, e.Op)
		case "allowed_environments":
			pf.AllowedEnvironments = doctorApplyEnumList(pf.AllowedEnvironments, e.Value, e.Op)
		}
	}
}

// doctorApplyEnumList applies an add/remove op to an allowed_* snapshot (order preserved).
func doctorApplyEnumList(list []string, value, op string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return list
	}
	switch op {
	case "remove":
		out := make([]string, 0, len(list))
		for _, v := range list {
			if v != value {
				out = append(out, v)
			}
		}
		return out
	default: // add
		for _, v := range list {
			if v == value {
				return list
			}
		}
		return append(append([]string{}, list...), value)
	}
}

// buildDoctorStatusSuggestion computes backfill targets for one live status that the
// profile does not reference (#120): a canonical alias guessed from nameEn/displayName
// (workflow.AliasForStatus; empty when ambiguous) plus the workflows[type].statuses
// displayName key. Keys already mapping to a different id are recorded as conflicts.
func buildDoctorStatusSuggestion(pf *profile.Profile, typeID string, isBugType bool, st workflow.StatusInfo) doctorStatusSuggestion {
	s := doctorStatusSuggestion{
		StatusID:    st.ID,
		DisplayName: st.DisplayName,
		NameEn:      st.NameEn,
		Alias:       workflow.AliasForStatus(st),
	}
	if isBugType && s.Alias != "" {
		t := doctorSuggestTarget{Map: "bug_statuses", Key: s.Alias, Value: st.ID}
		if cur, ok := pf.BugStatuses[s.Alias]; ok && cur != st.ID {
			t.Conflict = fmt.Sprintf("bug_statuses[%q] already maps to %q", s.Alias, cur)
		}
		s.Targets = append(s.Targets, t)
	}
	label := st.DisplayName
	if label == "" {
		label = st.Name
	}
	if label != "" {
		t := doctorSuggestTarget{Map: "workflows.statuses", Key: label, Value: st.ID}
		if cur, ok := pf.Workflows[typeID].Statuses[label]; ok && cur != st.ID {
			t.Conflict = fmt.Sprintf("workflows[%q].statuses[%q] already maps to %q", typeID, label, cur)
		}
		s.Targets = append(s.Targets, t)
	}
	return s
}

// suggestionLine renders the backfill targets as copy-paste assignment lines.
func (s doctorStatusSuggestion) suggestionLine(typeID string) string {
	parts := make([]string, 0, len(s.Targets))
	for _, t := range s.Targets {
		if t.Map == "bug_statuses" {
			parts = append(parts, fmt.Sprintf("bug_statuses[%q] = %q", t.Key, t.Value))
		} else {
			parts = append(parts, fmt.Sprintf("workflows[%q].statuses[%q] = %q", typeID, t.Key, t.Value))
		}
	}
	return strings.Join(parts, "; ")
}

// conflictLine joins the targets' conflict reasons ("" when none).
func (s doctorStatusSuggestion) conflictLine() string {
	parts := make([]string, 0, len(s.Targets))
	for _, t := range s.Targets {
		if t.Conflict != "" {
			parts = append(parts, t.Conflict)
		}
	}
	return strings.Join(parts, "; ")
}

// doctorFieldKeyFragments maps profile field keys (bug_fields.* /
// bug_create_fields.*) to live fieldName fragments (zh + en) used to hint
// same-purpose live fields on missing_on_type findings (#120).
var doctorFieldKeyFragments = map[string][]string{
	"module":             {"模块", "module"},
	"environment":        {"环境", "environment"},
	"expcompletiontime":  {"期望完成时间", "完成时间", "completion"},
	"plan_due_date":      {"计划完成", "完成时间", "due"},
	"developer":          {"开发", "developer"},
	"responsible_person": {"负责人", "责任", "owner"},
	"bug_reason":         {"原因", "reason"},
	"bug_impact_scope":   {"影响", "impact"},
	"priority":           {"优先级", "priority"},
	"serious_level":      {"严重", "serious", "severity"},
}

// doctorProfileFieldKey extracts the semantic key from a finding source label
// (bug_fields.<key> / bug_create_fields.<key>); "" for unsourced refs.
func doctorProfileFieldKey(source string) string {
	for _, prefix := range []string{"bug_fields.", "bug_create_fields."} {
		if strings.HasPrefix(source, prefix) {
			return strings.ToLower(strings.TrimPrefix(source, prefix))
		}
	}
	return ""
}

// doctorSimilarFields lists up to 5 live fields whose fieldName matches the profile
// key's fragments (case-insensitive), sorted by field id for stable output (#120).
func doctorSimilarFields(key string, fields []workitemfields.Field) []map[string]any {
	if key == "" || len(fields) == 0 {
		return nil
	}
	frags := doctorFieldKeyFragments[key]
	if len(frags) == 0 {
		frags = []string{key}
	}
	var out []map[string]any
	for _, f := range fields {
		if f.ID == "" || f.Name == "" {
			continue
		}
		name := strings.ToLower(f.Name)
		for _, frag := range frags {
			if strings.Contains(name, strings.ToLower(frag)) {
				out = append(out, map[string]any{"id": f.ID, "field_name": f.Name})
				break
			}
		}
		if len(out) >= 5 {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := out[i]["id"].(string)
		b, _ := out[j]["id"].(string)
		return a < b
	})
	return out
}

// attachLiveStatusMeta copies live displayName / nameEn onto a finding entry (#120).
func attachLiveStatusMeta(entry map[string]any, st workflow.StatusInfo) {
	if st.DisplayName != "" {
		entry["display_name"] = st.DisplayName
	}
	if st.NameEn != "" {
		entry["name_en"] = st.NameEn
	}
}

func doctorType(ctx context.Context, c *client.Client, pf *profile.Profile, typeID string, fixSuggest bool) (map[string]any, doctorTypeSuggestions, bool) {
	suggOut := doctorTypeSuggestions{typeID: typeID, isBugType: typeID == pf.BugTypeID}
	fieldsPath, err := c.ProjexPath(ctx, "/projects/"+pf.SpaceID+"/workitemTypes/"+typeID+"/fields")
	if err != nil {
		return map[string]any{"type_id": typeID, "error": err.Error()}, suggOut, false
	}
	wfPath, err := c.ProjexPath(ctx, "/projects/"+pf.SpaceID+"/workitemTypes/"+typeID+"/workflows")
	if err != nil {
		return map[string]any{"type_id": typeID, "error": err.Error()}, suggOut, false
	}

	var fieldsRaw any
	if err := c.Get(ctx, fieldsPath, nil, &fieldsRaw); err != nil {
		return map[string]any{"type_id": typeID, "error": err.Error()}, suggOut, false
	}
	var wfRaw any
	if err := c.Get(ctx, wfPath, nil, &wfRaw); err != nil {
		return map[string]any{"type_id": typeID, "error": err.Error()}, suggOut, false
	}

	// Live field ids + names (#120); fall back to the legacy id-only extraction
	// for payload shapes workitemfields.Parse rejects.
	// #121: keep parse error so allowed_* diffs can emit enum_unverified.
	parsedFields, fieldsParseErr := workitemfields.Parse(fieldsRaw)
	liveFields := map[string]bool{}
	liveFieldNames := map[string]string{}
	for _, f := range parsedFields {
		if f.ID == "" {
			continue
		}
		liveFields[f.ID] = true
		if f.Name != "" {
			liveFieldNames[f.ID] = f.Name
		}
	}
	if len(liveFields) == 0 {
		liveFields = extractFieldIDs(fieldsRaw)
	}

	_, _, _, statuses, wfErr := workflow.ParseWorkflowResponse(wfRaw)
	liveStatus := map[string]bool{}
	liveStatusByID := map[string]workflow.StatusInfo{}
	for _, s := range statuses {
		liveStatus[s.ID] = true
		liveStatusByID[s.ID] = s
	}

	isBugType := typeID == pf.BugTypeID
	suggOut.isBugType = isBugType
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
			if name := liveFieldNames[fid]; name != "" {
				entry["field_name"] = name
			}
		} else {
			entry["status"] = "missing_on_type"
			if sim := doctorSimilarFields(doctorProfileFieldKey(src), parsedFields); len(sim) > 0 {
				entry["similar_fields"] = sim
			}
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
		if wfErr != nil {
			entry["status"] = "workflow_parse_error"
			entry["error"] = wfErr.Error()
			ok = false
		} else if liveStatus[sid] {
			entry["status"] = "ok"
			attachLiveStatusMeta(entry, liveStatusByID[sid])
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
		st := liveStatusByID[sid]
		entry := map[string]any{
			"id":     sid,
			"source": "live_workflow",
			"status": "unknown_in_profile",
		}
		attachLiveStatusMeta(entry, st)
		if fixSuggest {
			sugg := buildDoctorStatusSuggestion(pf, typeID, isBugType, st)
			if sugg.Alias != "" {
				entry["suggest_alias"] = sugg.Alias
			}
			if line := sugg.suggestionLine(typeID); line != "" {
				entry["suggestion"] = line
			}
			if conflict := sugg.conflictLine(); conflict != "" {
				entry["suggest_conflict"] = conflict
			}
			suggOut.suggestions = append(suggOut.suggestions, sugg)
		}
		findings = append(findings, entry)
	}

	// #121: allowed_* snapshots are tenant data snapshots; diff them against the live
	// field options of the bug type so drift is reported instead of silently gating
	// +bug-create with stale values.
	if isBugType {
		enumFindings, enumSuggs, enumDrift := doctorAllowedEnumFindings(pf, parsedFields, fieldsParseErr, fixSuggest)
		findings = append(findings, enumFindings...)
		suggOut.enums = append(suggOut.enums, enumSuggs...)
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
	}, suggOut, ok
}

func doctorAllowedEnumFindings(pf *profile.Profile, fields []workitemfields.Field, fieldsParseErr error, fixSuggest bool) (findings []map[string]any, suggs []doctorEnumSuggestion, drift bool) {
	if pf == nil {
		return nil, nil, false
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
			entry := map[string]any{
				"id":           v,
				"source":       ck.source,
				"field_id":     ck.fieldID,
				"status":       "enum_stale_in_profile",
				"live_options": labels,
			}
			if fixSuggest {
				entry["suggestion"] = fmt.Sprintf("remove %s[%q]", ck.source, v)
				suggs = append(suggs, doctorEnumSuggestion{Source: ck.source, Value: v, Op: "remove"})
			}
			findings = append(findings, entry)
			drift = true
		}
		for _, v := range unlisted {
			entry := map[string]any{
				"id":       v,
				"source":   ck.source,
				"field_id": ck.fieldID,
				"status":   "enum_missing_in_profile",
				"allowed":  ck.allowed,
			}
			if fixSuggest {
				entry["suggestion"] = fmt.Sprintf("add %s[%q]", ck.source, v)
				suggs = append(suggs, doctorEnumSuggestion{Source: ck.source, Value: v, Op: "add"})
			}
			findings = append(findings, entry)
			drift = true
		}
	}
	return findings, suggs, drift
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
	profileDoctorCmd.Flags().Bool("fix-suggest", true, "attach alias suggestions to unknown_in_profile findings")
	profileDoctorCmd.Flags().Bool("write", false, "backfill suggested status ids and allowed_* enum drift into the profile (bug_statuses + workflows[type].statuses + allowed_modules/environments; edges untouched; use --dry-run to preview)")
	profileCmd.AddCommand(profileDoctorCmd)
}
