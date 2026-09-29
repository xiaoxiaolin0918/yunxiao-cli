package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/workitemfields"
)

// precheckWarnOut receives the degrade warning when field config is unavailable (tests redirect).
var precheckWarnOut io.Writer = os.Stderr

// workitemFieldsPath is GET .../projects/{space}/workitemTypes/{typeId}/fields (same as `workitem fields`).
func workitemFieldsPath(ctx context.Context, c *client.Client, spaceID, typeID string) (string, error) {
	return c.ProjexPath(ctx, "/projects/"+spaceID+"/workitemTypes/"+typeID+"/fields")
}

// requestPreviewWithPrecheck adds the create precheck outcome to a dry-run preview.
type requestPreviewWithPrecheck struct {
	client.RequestPreview
	Precheck map[string]any `json:"precheck,omitempty"`
}

// precheckWorkitemCreate (#95) reads the type's field config once and compares it with
// the final create body (flags, *-file inputs and profile defaults already applied).
//   - all required fields present → {"status":"ok","required_checked":n}
//   - some missing → *detailedError (subtype missing_required_fields) listing every one
//     with field id, name and options; the caller must not POST
//   - config unavailable (HTTP error, network, odd payload) → degrade: warning on
//     stderr + {"status":"skipped",…}; create proceeds and the server validates as before
//
// Read-only; also runs under --dry-run. Never mutates body.
func precheckWorkitemCreate(ctx context.Context, c *client.Client, spaceID, typeID string, body map[string]any) (map[string]any, error) {
	inspect := fmt.Sprintf("yunxiao workitem fields --space-id %s --type-id %s", spaceID, typeID)
	fields, err := fetchWorkitemFieldConfig(ctx, c, spaceID, typeID)
	if err != nil {
		reason := precheckSkipReason(err)
		fmt.Fprintf(precheckWarnOut, "warning: required-field precheck skipped (%s); server-side validation still applies; check with: %s\n", reason, inspect)
		return map[string]any{"status": "skipped", "reason": reason, "hint": "check required fields with: " + inspect}, nil
	}
	missing, checked := workitemfields.MissingRequired(fields, body)
	if len(missing) == 0 {
		return map[string]any{"status": "ok", "required_checked": checked}, nil
	}
	names := make([]string, 0, len(missing))
	for _, m := range missing {
		names = append(names, fmt.Sprintf("%s (%s)", m.Name, m.FieldID))
	}
	return nil, &detailedError{
		Subtype: "missing_required_fields",
		Message: fmt.Sprintf("workitem create precheck: %d required field(s) missing for type %s: %s", len(missing), typeID, strings.Join(names, ", ")),
		Hint: fmt.Sprintf(`pass custom fields via --custom-fields / --custom-fields-file as {"<field_id>":"<option id or value>"} (options in error.details.missing), system fields via the flag in pass_via; full config: %s; skip this check with --no-precheck`,
			inspect),
		Details: map[string]any{
			"space_id":         spaceID,
			"type_id":          typeID,
			"required_checked": checked,
			"missing":          missing,
		},
	}
}

func fetchWorkitemFieldConfig(ctx context.Context, c *client.Client, spaceID, typeID string) ([]workitemfields.Field, error) {
	path, err := workitemFieldsPath(ctx, c, spaceID, typeID)
	if err != nil {
		return nil, err
	}
	var raw any
	if _, err := c.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	return workitemfields.Parse(raw)
}

// precheckSkipReason is a short, token-free reason for the degrade warning / meta.
func precheckSkipReason(err error) string {
	var ae *client.APIError
	if errors.As(err, &ae) {
		return fmt.Sprintf("GET fields -> HTTP %d", ae.Status)
	}
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return msg
}
