package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/mrlink"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// resolveWorkItemsForMR GETs each ref (serial or internal id).
// If expectSpace is non-empty, rejects items whose spaceId differs.
func resolveWorkItemsForMR(ctx context.Context, c *client.Client, refs []string, expectSpace string) ([]mrlink.ResolvedWorkItem, error) {
	var out []mrlink.ResolvedWorkItem
	seen := map[string]struct{}{}
	for _, raw := range refs {
		ref := strings.TrimSpace(raw)
		if ref == "" {
			continue
		}
		wpath, err := c.ProjexPath(ctx, "/workitems/"+ref)
		if err != nil {
			return nil, err
		}
		var item map[string]any
		if err := c.Get(ctx, wpath, nil, &item); err != nil {
			return nil, fmt.Errorf("work item %q not found or inaccessible: %w", ref, err)
		}
		internal := zhiyi.InternalID(item)
		if internal == "" {
			return nil, fmt.Errorf("work item %q: could not resolve internal id", ref)
		}
		if expectSpace != "" {
			gotSpace := mrlink.SpaceIDFromWorkItem(item)
			if gotSpace != "" && gotSpace != expectSpace {
				return nil, fmt.Errorf("work item %q (id %s) is in space %s, expected %s", ref, internal, gotSpace, expectSpace)
			}
		}
		if _, ok := seen[internal]; ok {
			continue
		}
		seen[internal] = struct{}{}
		out = append(out, mrlink.ResolvedWorkItem{
			InternalID: internal,
			MatchKeys:  mrlink.MatchKeysFromWorkItem(item, ref),
		})
	}
	return out, nil
}

func splitWorkItemRefs(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// warnMRWorkItemLinks compares wanted resolved items to MR payload; sets meta.warnings and stderr.
func warnMRWorkItemLinks(meta map[string]any, wanted []mrlink.ResolvedWorkItem, mr map[string]any) {
	if len(wanted) == 0 {
		return
	}
	missing := mrlink.MissingWorkItemIDs(wanted, mrlink.AttachedWorkItemIDs(mr))
	if len(missing) == 0 {
		return
	}
	msg := mrlink.FormatMissingLinkWarning(missing)
	fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	if meta == nil {
		return
	}
	existing, _ := meta["warnings"].([]string)
	meta["warnings"] = append(existing, msg)
	meta["work_item_link_missing"] = missing
}
