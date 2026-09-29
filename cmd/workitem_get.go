package cmd

import (
	"fmt"
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
                       sprint, priority (from customFieldValues fieldId "priority"),
                       gmtModified, and description_summary
                       "(description: <n> chars, use --full or --fields description)"
                       instead of the description text. meta.projection = "brief".
  --full               the raw GetWorkitem object, identical to 0.16.33 and earlier.
  --fields a,b,c       only these top-level keys, raw values (e.g. description,
                       customFieldValues, workitemType); "priority" is derived as above.
                       Case-sensitive; an unknown name fails with
                       error.subtype=unknown_fields and details.available.
                       meta.projection = "fields".
All views keep meta.url / serial_number / resolved_id and the {ok,data,meta} envelope.
--jq runs on the projected envelope: scripts reading .data.description or
.data.customFieldValues need --full (or --fields). --dry-run sends nothing and shows
the chosen view under request.projection.

  yunxiao workitem get ZYPT-5916
  yunxiao workitem get ZYPT-5916 --fields subject,description
  yunxiao workitem get ZYPT-5916 --full --jq '.data.customFieldValues'`,
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
		// Same as runRead, but a projection error (unknown --fields) must fail before
		// anything is written to stdout.
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

// workitemGetView is the output view picked by --full / --brief / --fields (#98).
type workitemGetView struct {
	mode   string // "brief" (default), "full" or "fields"
	fields []string
}

// workitemGetViewFromFlags validates the view flags before any request, so conflicts
// and malformed --fields fail the same way with or without --dry-run.
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
	full, _ := cmd.Flags().GetBool("full")
	switch {
	case cmd.Flags().Changed("fields"):
		raw, _ := cmd.Flags().GetString("fields")
		fields, err := zhiyi.ParseWorkItemFieldList(raw)
		if err != nil {
			return workitemGetView{}, err
		}
		return workitemGetView{mode: "fields", fields: fields}, nil
	case full:
		return workitemGetView{mode: "full"}, nil
	}
	return workitemGetView{mode: "brief"}, nil
}

// previewProjection is request.projection for --dry-run (nil for --full, whose
// dry-run output stays as before).
func (v workitemGetView) previewProjection() map[string]any {
	switch v.mode {
	case "brief":
		return map[string]any{"mode": "brief"}
	case "fields":
		return map[string]any{"mode": "fields", "fields": v.fields}
	}
	return nil
}

// apply projects the GET result. meta is already enriched from the full item, so
// meta.url etc. survive every view. Non-object payloads pass through unchanged.
func (v workitemGetView) apply(out any, item map[string]any, meta map[string]any) (any, error) {
	if v.mode == "full" || item == nil {
		return out, nil
	}
	if v.mode == "brief" {
		meta["projection"] = "brief"
		return zhiyi.WorkItemGetBrief(item), nil
	}
	data, unknown := zhiyi.ProjectWorkItem(item, v.fields)
	if len(unknown) > 0 {
		avail := zhiyi.WorkItemAvailableFields(item)
		details := map[string]any{"unknown": unknown, "available": avail}
		if sug := zhiyi.WorkItemFieldSuggestions(unknown, avail); len(sug) > 0 {
			details["suggestions"] = sug
		}
		return nil, &detailedError{
			Subtype: "unknown_fields",
			Message: fmt.Sprintf("workitem get --fields: unknown field(s) %s (names are case-sensitive top-level keys)", strings.Join(unknown, ", ")),
			Hint:    "see error.details.available (or run with --full); nested values: --full --jq '.data.<key>...'",
			Details: details,
		}
	}
	meta["projection"] = "fields"
	return data, nil
}

// requestPreviewWithProjection adds the chosen output view to a dry-run preview.
type requestPreviewWithProjection struct {
	client.RequestPreview
	Projection map[string]any `json:"projection,omitempty"`
}
