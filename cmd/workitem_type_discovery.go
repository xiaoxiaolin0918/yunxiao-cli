package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

// workitemTypesFromRaw extracts the type list from a GET .../workitemTypes
// response (bare array, or wrapped under a list key — same rule as
// typeIDInWorkitemTypesList / explore-workflow category probing, plus "result" /
// "types" tolerance). Returns nil when raw is null or an unrecognized shape;
// callers treat nil-with-non-nil-raw as a failed category, never a silent empty.
func workitemTypesFromRaw(raw any) []any {
	items := client.ExtractListItems(raw)
	if items == nil {
		if s, ok := raw.([]any); ok {
			items = s
		}
	}
	if items == nil {
		if m, ok := raw.(map[string]any); ok {
			for _, key := range []string{"result", "types"} {
				if inner, ok := m[key]; ok {
					if s := workitemTypesFromRaw(inner); s != nil {
						return s
					}
				}
			}
		}
	}
	return items
}

// typesCategoryAll is the --category value that explicitly selects the merged
// all-categories fetch (same as omitting --category).
const typesCategoryAll = "all"

// typesListAllCategories runs the #99 default: when types list runs without
// --category, fetch every known Projex category (exploreTypeCategories)
// concurrently and merge the results so no category — Bug included — is silently
// hidden behind an implicit default. Each merged item gets its queried category
// injected (only when the API payload lacks it); duplicates by id are dropped.
// Categories whose GET fails are skipped and reported via meta.categories_failed
// (plus one stderr warning each, repo convention); failing every category is an error.
func typesListAllCategories(ctx context.Context, c *client.Client, spaceID string) error {
	categories := exploreTypeCategories
	path, err := c.ProjexPath(ctx, "/projects/"+spaceID+"/workitemTypes")
	if err != nil {
		return err
	}
	if globalDryRun {
		previews := make([]client.RequestPreview, 0, len(categories))
		for _, cat := range categories {
			previews = append(previews, c.Preview("GET", path, map[string]string{"category": cat}, nil))
		}
		return output.DryRunResult(string(risk.Read), previews)
	}
	items, okCats, failed := fetchWorkitemTypesAllCategories(ctx, c, path, categories)
	if len(okCats) == 0 {
		names := make([]string, 0, len(failed))
		for cat := range failed {
			names = append(names, cat)
		}
		sort.Strings(names)
		var last string
		for _, cat := range names {
			last = failed[cat]
		}
		return fmt.Errorf("workitem types list: every category GET failed (%s): %s", strings.Join(names, ", "), last)
	}
	for cat, reason := range failed {
		fmt.Fprintf(output.Stderr, "warning: types list category %s failed (%s); merged the remaining categories\n", cat, reason)
	}
	meta := map[string]any{
		"risk":       risk.Read,
		"category":   typesCategoryAll,
		"categories": okCats,
	}
	if len(failed) > 0 {
		meta["categories_failed"] = failed
	}
	return output.Success(items, meta)
}

// fetchWorkitemTypesAllCategories GETs one category query per entry concurrently
// (fixed category order is preserved in the merged result; per-category errors
// land in failed keyed by category, not the returned error).
func fetchWorkitemTypesAllCategories(ctx context.Context, c *client.Client, path string, categories []string) (items []any, okCategories []string, failed map[string]string) {
	type result struct {
		items []any
		raw   any
		err   error
	}
	results := make([]result, len(categories))
	var wg sync.WaitGroup
	for i, cat := range categories {
		wg.Add(1)
		go func(i int, cat string) {
			defer wg.Done()
			var raw any
			if _, err := c.Do(ctx, "GET", path, map[string]string{"category": cat}, nil, &raw); err != nil {
				results[i] = result{err: err}
				return
			}
			results[i] = result{items: workitemTypesFromRaw(raw), raw: raw}
		}(i, cat)
	}
	wg.Wait()

	failed = map[string]string{}
	for i, cat := range categories {
		if results[i].err != nil {
			failed[cat] = typesFetchFailReason(results[i].err)
			continue
		}
		// A non-null payload we cannot extract is a failed category (explicit in
		// meta.categories_failed), never a silently empty one (#99 anti-pattern).
		if results[i].items == nil && results[i].raw != nil {
			failed[cat] = fmt.Sprintf("unexpected types payload (%T)", results[i].raw)
			continue
		}
		okCategories = append(okCategories, cat)
		for _, it := range results[i].items {
			m, ok := it.(map[string]any)
			if !ok || m == nil {
				items = append(items, it)
				continue
			}
			// Self-describing merged output: keep an API-provided category, else
			// inject the queried one (CLI-injected, cf. mrs diffs "latest" #94).
			if cur, _ := m["category"].(string); strings.TrimSpace(cur) == "" {
				m["category"] = cat
			}
			items = append(items, m)
		}
	}
	return client.DedupItemsByID(items), okCategories, failed
}

// availableTypeSummaries projects merged type items into {id, name, category}
// entries for error.details.available_types (#99). name prefers name, then
// nameEn, then displayName; items without an id are skipped.
func availableTypeSummaries(items []any) []map[string]string {
	out := make([]map[string]string, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok || m == nil {
			continue
		}
		id := anyString(m["id"])
		if id == "" {
			continue
		}
		name := anyString(m["name"])
		if name == "" {
			name = anyString(m["nameEn"])
		}
		if name == "" {
			name = anyString(m["displayName"])
		}
		out = append(out, map[string]string{
			"id":       id,
			"name":     name,
			"category": anyString(m["category"]),
		})
	}
	return out
}

func anyString(v any) string {
	s, _ := v.(string)
	return s
}

// apiDetailsError renders like the wrapped *client.APIError (type "api", status
// code, API hint) plus a machine-readable subtype and details — the API-error
// counterpart of detailedError (#95). Built by enrichTypeNotEnabledError (#99).
type apiDetailsError struct {
	ae      *client.APIError
	Subtype string
	Hint    string
	Details map[string]any
}

func (e *apiDetailsError) Error() string { return e.ae.Error() }

// body is the stderr envelope: apiErrorBody for the wrapped error, then subtype
// / hint / details layered on top; extraHint (e.g. the create precheck-skip
// reason) is appended last.
func (e *apiDetailsError) body(extraHint string) output.ErrorBody {
	b := apiErrorBody(e.ae)
	b.Subtype = e.Subtype
	joinHint := func(h string) {
		if h == "" {
			return
		}
		if b.Hint == "" {
			b.Hint = h
		} else {
			b.Hint += "; " + h
		}
	}
	joinHint(e.Hint)
	joinHint(extraHint)
	if e.Details != nil {
		if b.Details == nil {
			b.Details = e.Details
		} else {
			for k, v := range e.Details {
				b.Details[k] = v
			}
		}
	}
	return b
}

// enrichTypeNotEnabledError (#99): when a workitem create fails because the type
// is not enabled in the project (server text 工作项类型未启用), attach the space's
// enabled types as error.details.available_types (id/name/category, merged across
// the known categories) so the failure is actionable without another round trip.
// Lookup is best-effort with the #95 precheck budget (2 attempts, <=1s backoff,
// 10s overall): if it fails, the original error only gains a hint pointing at
// `workitem types list`. Any other error passes through unchanged.
func enrichTypeNotEnabledError(ctx context.Context, c *client.Client, spaceID, typeID string, err error) error {
	ae, ok := err.(*client.APIError)
	if !ok || ae == nil {
		return err
	}
	if !strings.Contains(ae.Body+" "+ae.Error(), "工作项类型未启用") {
		return err
	}
	details := map[string]any{"space_id": spaceID, "type_id": typeID}
	hint := "work item type is not enabled in this project; types can only be enabled in the Projex project settings UI (no OpenAPI)"
	if c != nil && strings.TrimSpace(spaceID) != "" {
		ectx, cancel := context.WithTimeout(client.WithRetryPolicy(ctx, precheckMaxAttempts, precheckMaxDelay), precheckTimeout)
		defer cancel()
		path, perr := c.ProjexPath(ectx, "/projects/"+spaceID+"/workitemTypes")
		if perr == nil {
			items, _, failed := fetchWorkitemTypesAllCategories(ectx, c, path, exploreTypeCategories)
			if avail := availableTypeSummaries(items); len(avail) > 0 {
				details["available_types"] = avail
				hint += fmt.Sprintf("; pick an enabled type id from error.details.available_types, or list them with: yunxiao workitem types list --space-id %s", spaceID)
			} else {
				hint += fmt.Sprintf("; could not list enabled types%s — run: yunxiao workitem types list --space-id %s", lookupFailSuffix(failed), spaceID)
			}
			if len(failed) > 0 {
				details["categories_failed"] = failed
			}
		}
	}
	return &apiDetailsError{ae: ae, Subtype: "workitem_type_not_enabled", Hint: hint, Details: details}
}

func lookupFailSuffix(failed map[string]string) string {
	if len(failed) == 0 {
		return " (no enabled types found)"
	}
	return fmt.Sprintf(" (categories failed: %d)", len(failed))
}

// typesFetchFailReason is a short, token-free reason for a failed category GET.
func typesFetchFailReason(err error) string {
	var ae *client.APIError
	if errors.As(err, &ae) {
		return fmt.Sprintf("GET workitemTypes -> HTTP %d", ae.Status)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "GET workitemTypes timed out"
	}
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return msg
}
