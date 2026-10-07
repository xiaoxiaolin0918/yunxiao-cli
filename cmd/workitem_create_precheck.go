package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/workitemfields"
)

// Precheck GET retry bound: 1 initial try + at most 1 retry, each backoff capped at 1s
// (also caps Retry-After), so a degraded precheck never stalls like the default GET
// policy (4 attempts, Retry-After up to 30s each ≈ 90s).
const (
	precheckMaxAttempts = 2
	precheckMaxDelay    = time.Second
)

// precheckTimeout bounds the whole field-config read (both attempts and the backoff),
// so a server that accepts the connection but never answers can't stall the create
// for the HTTP client timeout per attempt (~2 min) before degrading. Var for tests.
var precheckTimeout = 10 * time.Second

// workitemFieldsPath is GET .../projects/{space}/workitemTypes/{typeId}/fields (same as `workitem fields`).
func workitemFieldsPath(ctx context.Context, c *client.Client, spaceID, typeID string) (string, error) {
	return c.ProjexPath(ctx, "/projects/"+spaceID+"/workitemTypes/"+typeID+"/fields")
}

// requestPreviewWithPrecheck adds the create prepare outcomes (#95 precheck, #126
// option display-value resolution) to a dry-run preview.
type requestPreviewWithPrecheck struct {
	client.RequestPreview
	Precheck         map[string]any `json:"precheck,omitempty"`
	OptionResolution map[string]any `json:"option_resolution,omitempty"`
}

// prepareWorkitemCreate reads the type's field config once (same GET as `workitem
// fields`) and prepares the create with it:
//
//  1. #126 option display-value resolution: list/multiList customFieldValues entries
//     given as a display value (e.g. {"priority":"高"}) are rewritten in body to
//     their option id. Unresolvable values (no option id / display value match, or
//     an ambiguous display value) → *detailedError (subtype invalid_option_values)
//     listing every bad value with the field's valid options; the caller must not
//     POST. Returns meta.option_resolution (dry-run: request.option_resolution):
//     {status:"ok", resolved:[{field_id,field_name,from,to},…]} — resolved is
//     omitted when nothing needed mapping.
//  2. #95 required-field precheck against the (already resolved) body; returns
//     meta.precheck as documented below.
//
// meta.option_resolution is nil (absent from output) when the body has no
// customFieldValues, so creates without custom fields see no new meta.
//
// precheck meta:
//   - status "ok", source "fields": every checked required field present;
//     required_checked, plus skipped_default (ids skipped for a server defaultValue)
//   - some missing → *detailedError (subtype missing_required_fields) listing every one
//     with field id, name and options; the caller must not POST
//   - 401 on the GET → that error (auth problem: no degrade)
//   - config unreadable (other HTTP error, network, odd payload) → status "skipped",
//     or read but empty → status "empty"; with reason, hint and warning. If the profile
//     has workitem_defaults[type].create_required, those ids are checked instead
//     (source "profile_fallback", profile_missing[]), warn-only; else source "none".
//     Create proceeds either way and the server validates as before. No resolution
//     happened in this case: customFieldValues are sent as-is and
//     meta.option_resolution is {status:"skipped"|"empty", reason, hint}.
//
// A degraded result's warning goes to meta.precheck.warning (dry-run:
// request.precheck.warning) and, as elsewhere in the CLI, one "warning: ..." line on
// stderr (printed by the caller, see printPrecheckWarning).
// Also runs under --dry-run (the preview then shows the resolved ids). Apart from
// the #126 resolution it never mutates body.
func prepareWorkitemCreate(ctx context.Context, c *client.Client, spaceID, typeID string, body map[string]any, profileRequired []string) (precheck, optionResolution map[string]any, err error) {
	inspect := fmt.Sprintf("yunxiao workitem fields --space-id %s --type-id %s", spaceID, typeID)
	pctx, cancel := context.WithTimeout(client.WithRetryPolicy(ctx, precheckMaxAttempts, precheckMaxDelay), precheckTimeout)
	defer cancel()
	fields, err := fetchWorkitemFieldConfig(pctx, c, spaceID, typeID)
	var ae *client.APIError
	if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
		return nil, nil, err
	}
	cf, _ := body["customFieldValues"].(map[string]any)
	if err == nil && len(fields) > 0 {
		if len(cf) > 0 {
			resolved, invalid := workitemfields.ResolveOptionValues(fields, cf)
			if len(invalid) > 0 {
				return nil, nil, optionResolveError(invalid, spaceID, typeID, inspect)
			}
			res := map[string]any{"status": "ok"}
			if len(resolved) > 0 {
				res["resolved"] = resolved
			}
			optionResolution = res
		}
		precheck, err = precheckAgainstFields(fields, body, spaceID, typeID, inspect)
		return precheck, optionResolution, err
	}
	res := map[string]any{"hint": "check required fields with: " + inspect}
	if err != nil {
		res["status"] = "skipped"
		res["reason"] = precheckSkipReason(err)
	} else {
		res["status"] = "empty"
		res["reason"] = "field config returned no fields"
	}
	warning := fmt.Sprintf("required-field precheck %s (%s); server-side validation still applies", res["status"], res["reason"])
	if res["status"] == "skipped" {
		warning = fmt.Sprintf("required-field precheck skipped (%s); server-side validation still applies", res["reason"])
	}
	if len(profileRequired) > 0 {
		res["source"] = "profile_fallback"
		pseudo := make([]workitemfields.Field, 0, len(profileRequired))
		for _, id := range profileRequired {
			pseudo = append(pseudo, workitemfields.Field{ID: id, Name: id, Required: true})
		}
		missing, checked := workitemfields.MissingRequired(pseudo, body)
		res["required_checked"] = checked
		if len(missing) > 0 {
			ids := make([]string, 0, len(missing))
			for _, m := range missing {
				ids = append(ids, m.FieldID)
			}
			res["profile_missing"] = ids
			warning += fmt.Sprintf("; profile workitem_defaults[%s].create_required fields not provided: %s (not blocking)", typeID, strings.Join(ids, ", "))
		} else {
			warning += fmt.Sprintf("; all profile workitem_defaults[%s].create_required fields provided", typeID)
		}
	} else {
		res["source"] = "none"
	}
	res["warning"] = warning
	if len(cf) > 0 {
		optionResolution = map[string]any{
			"status": res["status"],
			"reason": res["reason"],
			"hint":   "custom field values are sent as-is; option display values were not resolved to option ids",
		}
	}
	return res, optionResolution, nil
}

// optionResolveError builds the #126 structured failure for unresolvable option
// display values: every bad value at once, each with the field's valid options.
func optionResolveError(invalid []workitemfields.InvalidValue, spaceID, typeID, inspect string) error {
	parts := make([]string, 0, len(invalid))
	for _, inv := range invalid {
		if inv.Reason == "ambiguous" {
			ids := make([]string, 0, len(inv.Options))
			for _, o := range inv.Options {
				ids = append(ids, o.ID)
			}
			parts = append(parts, fmt.Sprintf("%s (%s): %q is an ambiguous display value (matches option ids %s)", inv.Name, inv.FieldID, inv.Value, strings.Join(ids, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%s (%s): %q matches no option id or display value", inv.Name, inv.FieldID, inv.Value))
		}
	}
	return &detailedError{
		Subtype: "invalid_option_values",
		Message: fmt.Sprintf("workitem create: %d invalid option value(s) for type %s: %s", len(invalid), typeID, strings.Join(parts, "; ")),
		Hint: fmt.Sprintf(`pass an option id or display value from error.details.values[].options via --custom-fields / --custom-fields-file as {"<field_id>":"<value>"}; full config: %s; display values are only resolved when the field config is readable (--no-precheck sends values as-is)`,
			inspect),
		Details: map[string]any{
			"space_id": spaceID,
			"type_id":  typeID,
			"values":   invalid,
		},
	}
}

func precheckAgainstFields(fields []workitemfields.Field, body map[string]any, spaceID, typeID, inspect string) (map[string]any, error) {
	missing, checked := workitemfields.MissingRequired(fields, body)
	skippedDefault := workitemfields.DefaultSkipped(fields)
	if len(missing) == 0 {
		res := map[string]any{"status": "ok", "source": "fields", "required_checked": checked}
		if len(skippedDefault) > 0 {
			res["skipped_default"] = skippedDefault
		}
		return res, nil
	}
	names := make([]string, 0, len(missing))
	for _, m := range missing {
		names = append(names, fmt.Sprintf("%s (%s)", m.Name, m.FieldID))
	}
	details := map[string]any{
		"space_id":         spaceID,
		"type_id":          typeID,
		"required_checked": checked,
		"missing":          missing,
	}
	if len(skippedDefault) > 0 {
		details["skipped_default"] = skippedDefault
	}
	return nil, &detailedError{
		Subtype: "missing_required_fields",
		Message: fmt.Sprintf("workitem create precheck: %d required field(s) missing for type %s: %s", len(missing), typeID, strings.Join(names, ", ")),
		Hint: fmt.Sprintf(`pass custom fields via --custom-fields / --custom-fields-file as {"<field_id>":"<option id or value>"} (options in error.details.missing), system fields via the flag in pass_via; full config: %s; skip this check with --no-precheck`,
			inspect),
		Details: details,
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

// precheckSkipReason is a short, token-free reason for meta.precheck.
func precheckSkipReason(err error) string {
	var ae *client.APIError
	if errors.As(err, &ae) {
		return fmt.Sprintf("GET fields -> HTTP %d", ae.Status)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("GET fields timed out after %s", precheckTimeout)
	}
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return msg
}

// handleCreateErr is handleErr for the create POST. meta.precheck only exists on
// success, so when the precheck was skipped / empty and the POST then fails, the
// reason is appended to error.hint as "precheck skipped: <reason>" (or "precheck
// empty: <reason>"). The envelope is otherwise the same as handleErr's (type "api" +
// status for API errors). Other errors, and runs with an ok precheck, go to handleErr.
// Kept out of handleErr so helpers.go doesn't conflict with #101's contextError.
func handleCreateErr(err error, precheck map[string]any) {
	hint := precheckErrorHint(precheck)
	if err == nil || hint == "" {
		handleErr(err)
		return
	}
	switch e := err.(type) {
	case *client.APIError:
		_ = output.Fail(output.ErrorBody{
			Type:    "api",
			Message: e.Error(),
			Hint:    joinPrecheckHint(apiErrorHint(e), hint),
			Code:    e.Status,
		}, 1)
	case risk.GateResult, output.ExitError, *detailedError:
		handleErr(err)
		return
	default:
		_ = output.Fail(output.ErrorBody{Type: "cli", Message: err.Error(), Hint: hint}, 1)
	}
	processExit(1)
}

// precheckErrorHint is "precheck <status>: <reason>" for a skipped / empty precheck.
func precheckErrorHint(precheck map[string]any) string {
	status, _ := precheck["status"].(string)
	if status != "skipped" && status != "empty" {
		return ""
	}
	reason, _ := precheck["reason"].(string)
	return fmt.Sprintf("precheck %s: %s", status, reason)
}

func joinPrecheckHint(apiHint, hint string) string {
	if apiHint == "" {
		return hint
	}
	return apiHint + "; " + hint
}

// printPrecheckWarning writes the degraded-precheck warning as one "warning: ..." line
// on stderr (repo convention, cf. browse / mrs +create link warnings); the same text
// stays in meta.precheck.warning or request.precheck.warning.
func printPrecheckWarning(precheck map[string]any) {
	if w, _ := precheck["warning"].(string); w != "" {
		fmt.Fprintf(output.Stderr, "warning: %s\n", w)
	}
}
