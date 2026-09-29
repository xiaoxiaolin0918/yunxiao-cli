package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/mrpatchset"
)

// mrPatchSetsPath builds GET .../changeRequests/{localId}/diffs/patches for an encoded repo id.
func mrPatchSetsPath(ctx context.Context, c *client.Client, repoID, localID string) (string, error) {
	return c.CodeupPath(ctx, "/repositories/"+repoID+"/changeRequests/"+localID+"/diffs/patches")
}

// fetchMRPatchSets lists and parses an MR's patch sets (unordered, no "latest" marker)
// and returns the GET path it used. Used by comments create (#93) via
// resolveLatestMRPatchSet; mrs diffs (#94) does not call it — it marks the raw GET
// response in place via markLatestPatchSet (same mrpatchset.Latest rule).
func fetchMRPatchSets(ctx context.Context, c *client.Client, repoID, localID string) ([]mrpatchset.PatchSet, string, error) {
	path, err := mrPatchSetsPath(ctx, c, repoID, localID)
	if err != nil {
		return nil, "", err
	}
	var out any
	if _, err := c.Do(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, path, err
	}
	sets, err := mrpatchset.Parse(out)
	return sets, path, err
}

// resolveLatestMRPatchSet returns the MR's latest MERGE_SOURCE patch set and how it was
// resolved ("GET <path>"). Failures are wrapped as "resolve latest patchset for MR <n>: …"
// (contextError keeps *client.APIError reporting) with a hint to pass --patchset-biz-id.
// repoArg is the user's --repo value (id or alias), echoed in the hint.
func resolveLatestMRPatchSet(ctx context.Context, c *client.Client, repoArg, repoID, localID string) (mrpatchset.PatchSet, string, error) {
	wrap := func(err error, hint string) error {
		return &contextError{Context: "resolve latest patchset for MR " + localID, Hint: hint, Err: err}
	}
	inspect := fmt.Sprintf("inspect with: yunxiao codeup mrs diffs --repo %s --local-id %s", shellArg(repoArg), shellArg(localID))
	sets, path, err := fetchMRPatchSets(ctx, c, repoID, localID)
	if err != nil {
		return mrpatchset.PatchSet{}, "", wrap(err, "pass --patchset-biz-id explicitly to skip patchset resolution")
	}
	ps, err := mrpatchset.Latest(sets)
	if err != nil {
		return ps, "", wrap(err, inspect+"; or pass --patchset-biz-id explicitly")
	}
	return ps, "GET " + path, nil
}

// requestPreviewWithResolved adds CLI-resolved values (e.g. defaulted patchset) to a dry-run preview.
type requestPreviewWithResolved struct {
	client.RequestPreview
	Resolved map[string]any `json:"resolved,omitempty"`
}

// shellArg quotes a value for copy-paste hints when it contains whitespace or quotes.
func shellArg(v string) string {
	if v == "" || strings.ContainsAny(v, " \t\"'") {
		return fmt.Sprintf("%q", v)
	}
	return v
}

// markLatestPatchSet is the mrs diffs after-hook (#94): per-item "latest" plus
// meta.latest_patchset_biz_id (+ latest_version_no when known) when a latest patch set exists.
func markLatestPatchSet(out any, meta map[string]any) (any, map[string]any) {
	ps, ok := mrpatchset.MarkLatest(out)
	if ok {
		if meta == nil {
			meta = map[string]any{}
		}
		meta["latest_patchset_biz_id"] = ps.BizID
		if ps.VersionNo != 0 {
			meta["latest_version_no"] = ps.VersionNo
		}
	}
	return out, meta
}
