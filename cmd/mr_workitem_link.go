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
// Prefer ensureMRWorkItemLinks for create paths that require links.
func warnMRWorkItemLinks(meta map[string]any, wanted []mrlink.ResolvedWorkItem, mr map[string]any) {
	if len(wanted) == 0 {
		return
	}
	missing := mrlink.MissingWorkItemIDs(wanted, mrlink.AttachedWorkItemIDs(mr))
	if len(missing) == 0 {
		return
	}
	msg := mrlink.FormatMissingLinkWarning(missing)
	fmt.Fprintf(os.Stderr, "warning: %s\\n", msg)
	if meta == nil {
		return
	}
	existing, _ := meta["warnings"].([]string)
	meta["warnings"] = append(existing, msg)
	meta["work_item_link_missing"] = missing
}

func mrLocalID(mr map[string]any) string {
	if mr == nil {
		return ""
	}
	for _, key := range []string{"localId", "iid", "local_id"} {
		if s := strings.TrimSpace(fmt.Sprint(mr[key])); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func mrURLFrom(meta map[string]any, mr map[string]any) string {
	if meta != nil {
		if u, ok := meta["url"].(string); ok && u != "" {
			return u
		}
	}
	if mr != nil {
		for _, key := range []string{"detailUrl", "webUrl", "url"} {
			if u, ok := mr[key].(string); ok && u != "" {
				return u
			}
		}
	}
	return ""
}

// listWorkItemMRRelations GETs projex extRelationRecords for codeupMergeRequest.
func listWorkItemMRRelations(ctx context.Context, c *client.Client, workItemID string) ([]map[string]any, error) {
	path, err := c.ProjexPath(ctx, "/workitems/"+workItemID+"/extRelationRecords")
	if err != nil {
		return nil, err
	}
	q := map[string]string{"category": "codeupMergeRequest"}
	var body any
	if _, err := c.Do(ctx, "GET", path, q, nil, &body); err != nil {
		return nil, err
	}
	switch t := body.(type) {
	case []any:
		var out []map[string]any
		for _, e := range t {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out, nil
	case map[string]any:
		if items := client.ExtractListItems(t); items != nil {
			var out []map[string]any
			for _, e := range items {
				if m, ok := e.(map[string]any); ok {
					out = append(out, m)
				}
			}
			return out, nil
		}
	}
	return nil, nil
}

func relationMatchesMR(rel map[string]any, projectID, localID string) bool {
	biz := strings.TrimSpace(fmt.Sprint(rel["businessId"]))
	proj := strings.TrimSpace(fmt.Sprint(rel["projectId"]))
	if biz == "<nil>" {
		biz = ""
	}
	if proj == "<nil>" {
		proj = ""
	}
	if localID != "" && biz != "" && biz != localID {
		return false
	}
	if projectID != "" && proj != "" && proj != projectID {
		return false
	}
	return biz == localID || (localID != "" && biz == localID)
}

// missingWorkItemLinksViaExtRelations returns wanted internal ids not linked to this MR.
func missingWorkItemLinksViaExtRelations(ctx context.Context, c *client.Client, projectID, localID string, wanted []mrlink.ResolvedWorkItem) ([]string, error) {
	if len(wanted) == 0 || localID == "" {
		return nil, nil
	}
	var missing []string
	for _, w := range wanted {
		rels, err := listWorkItemMRRelations(ctx, c, w.InternalID)
		if err != nil {
			return nil, fmt.Errorf("list extRelationRecords for %s: %w", w.InternalID, err)
		}
		ok := false
		for _, rel := range rels {
			if relationMatchesMR(rel, projectID, localID) {
				ok = true
				break
			}
		}
		if !ok {
			missing = append(missing, w.InternalID)
		}
	}
	return missing, nil
}

func createWorkItemMRRelation(ctx context.Context, c *client.Client, workItemID, projectID, localID, title, url, source, target string) error {
	path, err := c.ProjexPath(ctx, "/workitems/"+workItemID+"/extRelationRecords")
	if err != nil {
		return err
	}
	body := map[string]any{
		"category":       "codeupMergeRequest",
		"mergeRequestId": localID,
		"projectId":      projectID,
	}
	if title != "" {
		body["title"] = title
	}
	if url != "" {
		body["url"] = url
	}
	if source != "" {
		body["sourceBranch"] = source
	}
	if target != "" {
		body["targetBranch"] = target
	}
	return c.Post(ctx, path, body, nil)
}

// ensureMRWorkItemLinks verifies (and if needed repairs) work-item ↔ MR links after create.
// When --work-item was provided and links remain missing, returns an error so the CLI exits non-zero.
func ensureMRWorkItemLinks(ctx context.Context, c *client.Client, projectID string, wanted []mrlink.ResolvedWorkItem, mr map[string]any, meta map[string]any) error {
	if len(wanted) == 0 {
		return nil
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["work_item_ids_sent"] = mrlink.WorkItemIDsCSV(mrlink.InternalIDs(wanted))

	// First: response payload fields (if any).
	missing := mrlink.MissingWorkItemIDs(wanted, mrlink.AttachedWorkItemIDs(mr))
	localID := mrLocalID(mr)
	url := mrURLFrom(meta, mr)
	title, _ := mr["title"].(string)
	source, _ := mr["sourceBranch"].(string)
	target, _ := mr["targetBranch"].(string)

	// Prefer extRelationRecords — GetChangeRequest often omits work item fields.
	if localID != "" {
		extMissing, err := missingWorkItemLinksViaExtRelations(ctx, c, projectID, localID, wanted)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not verify work item links via extRelationRecords: %v\\n", err)
		} else {
			missing = extMissing
		}
	}

	if len(missing) == 0 {
		meta["work_item_linked"] = true
		return nil
	}

	// Attempt OpenAPI repair: CreateWorkitemExtRelationRecord per missing id.
	if localID != "" {
		var still []string
		for _, id := range missing {
			if err := createWorkItemMRRelation(ctx, c, id, projectID, localID, title, url, source, target); err != nil {
				fmt.Fprintf(os.Stderr, "warning: CreateWorkitemExtRelationRecord for %s failed: %v\\n", id, err)
				still = append(still, id)
				continue
			}
		}
		if recheck, err := missingWorkItemLinksViaExtRelations(ctx, c, projectID, localID, wanted); err == nil {
			missing = recheck
		} else {
			missing = still
		}
	}

	if len(missing) == 0 {
		meta["work_item_linked"] = true
		meta["work_item_link_repaired"] = true
		return nil
	}

	meta["work_item_link_missing"] = missing
	meta["work_item_linked"] = false
	return fmt.Errorf("%s", mrlink.FormatMissingLinkError(missing, url))
}
