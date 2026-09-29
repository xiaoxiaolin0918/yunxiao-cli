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

// fetchMRPatchSets lists and parses an MR's patch sets (unordered, no "latest" marker).
// Shared by comments create (#93); mrs diffs latest marking (#94) can reuse it.
func fetchMRPatchSets(ctx context.Context, c *client.Client, repoID, localID string) ([]mrpatchset.PatchSet, error) {
	path, err := mrPatchSetsPath(ctx, c, repoID, localID)
	if err != nil {
		return nil, err
	}
	var out any
	if _, err := c.Do(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, err
	}
	return mrpatchset.Parse(out)
}

// resolveLatestMRPatchSet returns the MR's latest MERGE_SOURCE patch set, or an
// actionable error when none exists. API errors are returned unchanged.
// repoArg is the user's --repo value (id or alias), echoed in the hint.
func resolveLatestMRPatchSet(ctx context.Context, c *client.Client, repoArg, repoID, localID string) (mrpatchset.PatchSet, error) {
	sets, err := fetchMRPatchSets(ctx, c, repoID, localID)
	if err != nil {
		return mrpatchset.PatchSet{}, err
	}
	ps, err := mrpatchset.Latest(sets)
	if err != nil {
		return ps, fmt.Errorf("cannot default --patchset-biz-id for MR %s: %v (inspect with: yunxiao codeup mrs diffs --repo %s --local-id %s; or pass --patchset-biz-id explicitly)", localID, err, shellArg(repoArg), shellArg(localID))
	}
	return ps, nil
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
