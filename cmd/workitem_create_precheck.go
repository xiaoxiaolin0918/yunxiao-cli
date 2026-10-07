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

// requestPreviewWithPrecheck adds the create precheck outcome to a dry-run preview.
type requestPreviewWithPrecheck struct {
	client.RequestPreview
	Precheck map[string]any `json:"precheck,omitempty"`
}

// precheckWorkitemCreate (#95) reads the type's field config once and compares it with
// the final create body (flags, *-file inputs and profile defaults already applied).
// Returns meta.precheck (dry-run: request.precheck):
//   - status "ok", source "fields": every checked required field present;
//     required_checked, plus skipped_default (ids skipped for a server defaultValue)
//   - some missing → *detailedError (subtype missing_required_fields) listing every one
//     with field id, name and options; the caller must not POST
//   - 401 on the GET → that error (auth problem: no degrade)
//   - config unreadable (other HTTP error, network, odd payload) → status "skipped",
//     or read but empty → status "empty"; with reason, hint and warning. If the profile
//     has workitem_defaults[type].create_required, those ids are checked instead
//     (source "profile_fallback", profile_missing[]), warn-only; else source "none".
//     Create proceeds either way and the server validates as before.
//
// A degraded result's warning goes to meta.precheck.warning (dry-run:
// request.precheck.warning) and, as elsewhere in the CLI, one "warning: ..." line on
// stderr (printed by the caller, see printPrecheckWarning).
// Read-only; also runs under --dry-run. Never mutates body.
func precheckWorkitemCreate(ctx context.Context, c *client.Client, spaceID, typeID string, body map[string]any, profileRequired []string) (map[string]any, error) {
	inspect := fmt.Sprintf("yunxiao workitem fields --space-id %s --type-id %s", spaceID, typeID)
	pctx, cancel := context.WithTimeout(client.WithRetryPolicy(ctx, precheckMaxAttempts, precheckMaxDelay), precheckTimeout)
	defer cancel()
	fields, err := fetchWorkitemFieldConfig(pctx, c, spaceID, typeID)
	var ae *client.APIError
	if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
		return nil, err
	}
	if err == nil && len(fields) > 0 {
		return precheckAgainstFields(fields, body, spaceID, typeID, inspect)
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
	return res, nil
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
	case *apiDetailsError:
		// Enriched API error (#99): same envelope as handleErr's branch, with the
		// precheck-skip reason appended to the hint.
		_ = output.Fail(e.body(hint), 1)
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
