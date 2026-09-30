package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var workitemGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get a work item by ID or serial (e.g. ZYPT-5768); brief by default, --full for raw",
	Long: `Risk: read
HTTP: GET .../workitems/{id}
Accepts --id or positional; Yunxiao accepts serial numbers like ZYPT-xxxx.

Output views (0.16.34+, #98; exactly one of):
  (default) / --brief  id, serialNumber, subject, status {id, displayName}, assignedTo,
                       sprint, priority, workitemType {id, name}, categoryId,
                       gmtModified, and description_summary
                       "(description: <n> chars, use --full or --fields description)"
                       instead of the description text (RICHTEXT/HTML: "<n> chars of
                       text excluding HTML tags"). Null / "" values are omitted.
                       meta.projection = "brief".
  --full               the raw GetWorkitem object, identical to 0.16.33 and earlier.
  --fields a,b,c       only these top-level keys, raw values (e.g. description,
                       customFieldValues, workitemType). Case-sensitive. A GetWorkitem
                       schema key this item lacks (e.g. no sprint) is null and listed
                       in meta.absent_fields; any other unknown name fails with
                       error.subtype=unknown_fields and details.available.
                       meta.projection = "fields".
priority: a top-level priority if the API returns a non-empty one, else the first
non-empty value of customFieldValues whose fieldId is "priority" or whose fieldName
is 优先级 / Priority, as {id, displayValue}. If none, brief omits it and
--fields priority prints null with meta.hint.

Compatibility switch: YUNXIAO_WORKITEM_GET_VIEW=full|brief picks the view when no
view flag is given (flag > env > default brief). CLIs before 0.16.34 ignore it, so
scripts that must work on old and new CLIs can set YUNXIAO_WORKITEM_GET_VIEW=full
instead of passing --full. Other values fail before any request.

All views keep meta.url / serial_number / resolved_id and the {ok,data,meta} envelope.
--jq runs on the projected envelope: scripts reading .data.description or
.data.customFieldValues need --full (or --fields). --fields on a non-object response
(array / null) fails with error.subtype=non_object_response. --dry-run sends nothing
and shows the chosen view under request.projection.

  yunxiao workitem get ZYPT-5916
  yunxiao workitem get ZYPT-5916 --fields subject,description
  yunxiao workitem get ZYPT-5916 --full --jq '.data.customFieldValues'
  YUNXIAO_WORKITEM_GET_VIEW=full yunxiao workitem get ZYPT-5916`,

	Run: func(cmd *cobra.Command, args []string) {
		view, err := workitemGetViewFromFlags(cmd)
		if err != nil {
			handleErr(err)
			return
		}
		flagOrg(globalOrg)
		if _, err := applyActiveProfileOrg(); err != nil {
			handleErr(err)
			return
		}
		id, err := workitemIDFromFlagOrArg(cmd, args)
		if err != nil {
			handleErr(err)
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
		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.Read), requestPreviewWithProjection{
				RequestPreview: c.Preview("GET", path, nil, nil),
				Projection:     view.previewProjection(),
			}))
			return
		}
		// Same steps as runRead, inlined because runRead's after-hook cannot return an
		// error (unknown --fields / non-object response must exit 1 via handleErr).
		var out any
		hdr, err := c.Do(cmd.Context(), "GET", path, nil, nil, &out)
		if err != nil {
			handleErr(err)
			return
		}
		meta := client.MetaWithPagination(map[string]any{"risk": risk.Read}, hdr)
		item := asStringMap(out)
		zhiyi.EnrichWorkItemMeta(meta, item, profileSpaceID(), "")
		data, err := view.apply(out, item, meta)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(data, meta))
	},
}

// envWorkitemGetView is the compatibility switch for scripts that must run on CLIs
// before and after 0.16.34 (old CLIs ignore it): full | brief. Flag > env > default.
const envWorkitemGetView = "YUNXIAO_WORKITEM_GET_VIEW"

// workitemGetView is the output view picked by --full / --brief / --fields or
// YUNXIAO_WORKITEM_GET_VIEW (#98).
type workitemGetView struct {
	mode   string // "brief" (default), "full" or "fields"
	fields []string
	source string // "flag", "env" or "default"
}

// workitemGetViewFromFlags validates the view flags and env before any request, so
// conflicts and malformed values fail the same way with or without --dry-run.
func workitemGetViewFromFlags(cmd *cobra.Command) (workitemGetView, error) {
	// --full=false / --brief=false count as not given.
	var set []string
	for _, f := range []string{"full", "brief"} {
		if on, _ := cmd.Flags().GetBool(f); on && cmd.Flags().Changed(f) {
			set = append(set, "--"+f)
		}
	}
	if cmd.Flags().Changed("fields") {
		set = append(set, "--fields")
	}
	if len(set) > 1 {
		return workitemGetView{}, fmt.Errorf("--full, --brief and --fields are mutually exclusive (got %s)", strings.Join(set, " and "))
	}
	if len(set) == 1 {
		switch set[0] {
		case "--fields":
			raw, _ := cmd.Flags().GetString("fields")
			fields, err := zhiyi.ParseWorkItemFieldList(raw)
			if err != nil {
				return workitemGetView{}, err
			}
			return workitemGetView{mode: "fields", fields: fields, source: "flag"}, nil
		case "--full":
			return workitemGetView{mode: "full", source: "flag"}, nil
		}
		return workitemGetView{mode: "brief", source: "flag"}, nil
	}
	raw, ok := os.LookupEnv(envWorkitemGetView)
	switch v := strings.ToLower(strings.TrimSpace(raw)); {
	case !ok || v == "":
		return workitemGetView{mode: "brief", source: "default"}, nil
	case v == "full" || v == "brief":
		return workitemGetView{mode: v, source: "env"}, nil
	default:
		return workitemGetView{}, &detailedError{
			Subtype: "invalid_env",
			Message: fmt.Sprintf("%s=%q: expected full or brief", envWorkitemGetView, raw),
			Hint:    fmt.Sprintf("unset %s or set it to full / brief; a --full / --brief / --fields flag overrides it", envWorkitemGetView),
			Details: map[string]any{"env": envWorkitemGetView, "value": raw, "allowed": []string{"full", "brief"}},
		}
	}
}

// previewProjection is request.projection for --dry-run. nil for --full given as a
// flag, whose dry-run output stays as before.
func (v workitemGetView) previewProjection() map[string]any {
	switch {
	case v.mode == "fields":
		return map[string]any{"mode": "fields", "fields": v.fields, "source": v.source}
	case v.mode == "full" && v.source == "flag":
		return nil
	}
	return map[string]any{"mode": v.mode, "source": v.source}
}

// apply projects the GET result. meta is already enriched from the full item, so
// meta.url etc. survive every view. --full returns out unchanged; brief passes a
// non-object payload through unchanged (as before); --fields on a non-object payload
// is an error, since there are no keys to project.
func (v workitemGetView) apply(out any, item map[string]any, meta map[string]any) (any, error) {
	if v.mode == "full" {
		return out, nil
	}
	if item == nil {
		if v.mode == "brief" {
			return out, nil
		}
		kind := zhiyi.JSONTypeName(out)
		return nil, &detailedError{
			Subtype: "non_object_response",
			Message: fmt.Sprintf("workitem get --fields: the API returned %s, not a work item object; nothing to project", kind),
			Hint:    "rerun with --full to see the raw response",
			Details: map[string]any{"response_type": kind},
		}
	}
	if v.mode == "brief" {
		meta["projection"] = "brief"
		return zhiyi.WorkItemGetBrief(item), nil
	}
	res := zhiyi.ProjectWorkItem(item, v.fields)
	if len(res.Unknown) > 0 {
		avail := zhiyi.WorkItemAvailableFields(item)
		details := map[string]any{"unknown": res.Unknown, "available": avail}
		if sug := zhiyi.WorkItemFieldSuggestions(res.Unknown, avail); len(sug) > 0 {
			details["suggestions"] = sug
		}
		return nil, &detailedError{
			Subtype: "unknown_fields",
			Message: fmt.Sprintf("workitem get --fields: unknown field(s) %s (names are case-sensitive top-level keys)", strings.Join(res.Unknown, ", ")),
			Hint:    "see error.details.available (or run with --full); nested values: --full --jq '.data.<key>...'",
			Details: details,
		}
	}
	meta["projection"] = "fields"
	if len(res.Absent) > 0 {
		meta["absent_fields"] = res.Absent
	}
	if res.PriorityUnresolved {
		meta["hint"] = "priority: no non-empty top-level priority and no customFieldValues entry with fieldId \"priority\" or fieldName 优先级/Priority; check with --full --jq '.data.customFieldValues' or `yunxiao workitem fields --space-id {space.id} --type-id {workitemType.id}`"
	}
	return res.Data, nil
}

// requestPreviewWithProjection adds the chosen output view to a dry-run preview.
type requestPreviewWithProjection struct {
	client.RequestPreview
	Projection map[string]any `json:"projection,omitempty"`
}
