package cmd

import (
	"context"
	"regexp"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// #97: `codeup mrs update --wip/--unwip` title-prefix toggles.
//
// A WIP prefix is one leading "WIP" (case-insensitive) followed by either a
// colon (with optional surrounding whitespace) or whitespace: "WIP: x", "wip x",
// "Wip:x". "WIPfix" is not a prefix (no separator). --wip prefixes "WIP: "
// idempotently; --unwip strips one leading prefix idempotently (no prefix →
// no write at all).

var wipTitlePrefixRe = regexp.MustCompile(`(?i)^\s*wip(?:\s*:\s*|\s+)`)

// hasWipTitlePrefix reports whether title starts with one WIP prefix.
func hasWipTitlePrefix(title string) bool {
	return wipTitlePrefixRe.MatchString(title)
}

// stripWipTitlePrefix removes one leading WIP prefix ("WIP:" / "WIP" + space,
// case-insensitive). A title that is only the prefix (result would be empty)
// is returned unchanged — the API would reject an empty title.
func stripWipTitlePrefix(title string) string {
	loc := wipTitlePrefixRe.FindStringIndex(title)
	if loc == nil {
		return title
	}
	rest := strings.TrimSpace(title[loc[1]:])
	if rest == "" {
		return title
	}
	return rest
}

// applyWipToggle resolves the title after --wip/--unwip (exactly one set) and
// reports whether it changed.
func applyWipToggle(title string, wip bool) (string, bool) {
	if wip {
		if hasWipTitlePrefix(title) {
			return title, false
		}
		return "WIP: " + strings.TrimSpace(title), true
	}
	stripped := stripWipTitlePrefix(title)
	return stripped, stripped != title
}

// fetchMRForWipToggle GETs the current MR (title needed to apply --wip/--unwip).
// Runs also under --dry-run, same as #93 patchset resolution.
func fetchMRForWipToggle(ctx context.Context, c *client.Client, repoID, localID string) (map[string]any, error) {
	path, err := c.CodeupPath(ctx, "/repositories/"+repoID+"/changeRequests/"+localID)
	if err != nil {
		return nil, err
	}
	var out any
	if _, err := c.Do(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, &contextError{
			Context: "fetch MR " + localID + " title for --wip/--unwip",
			Hint:    "pass --title with the full new title instead to skip the fetch",
			Err:     err,
		}
	}
	return zhiyi.StabilizeMergeRequest(zhiyi.UnwrapMergeRequestPayload(asStringMap(out))), nil
}

// wipToggleMeta returns the flat meta keys describing a resolved --wip/--unwip.
func wipToggleMeta(info map[string]any, meta map[string]any) map[string]any {
	if info == nil {
		return meta
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["wip_action"] = info["action"]
	meta["wip_changed"] = info["changed"]
	return meta
}

// wipToggleTitle reads the current title off a fetched MR object.
func wipToggleTitle(mr map[string]any) string {
	return mrStringField(mr, "title")
}
