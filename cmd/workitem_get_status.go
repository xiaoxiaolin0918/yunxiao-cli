package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// refreshWorkItemStatusViaSearch looks up the same work item through
// SearchWorkitems (serialNumber CONTAINS) so brief get can fill a null status
// that GetWorkitem sometimes omits (#369). Best-effort: returns an error when
// space/category/serial cannot be resolved or no matching item is found.
func refreshWorkItemStatusViaSearch(ctx context.Context, c *client.Client, item map[string]any) (map[string]any, error) {
	if c == nil || item == nil {
		return nil, fmt.Errorf("status refresh: missing client or item")
	}
	serial := zhiyi.SerialNumber(item)
	id := zhiyi.InternalID(item)
	if serial == "" && id == "" {
		return nil, fmt.Errorf("status refresh: item has no serialNumber or id")
	}
	spaceID := zhiyi.ResolveSpaceID(item, profileSpaceID(), "")
	if spaceID == "" {
		return nil, fmt.Errorf("status refresh: could not resolve space id")
	}
	category := workitemCategoryForSearch(item)
	if category == "" {
		return nil, fmt.Errorf("status refresh: could not resolve category")
	}
	path, err := c.ProjexPath(ctx, "/workitems:search")
	if err != nil {
		return nil, err
	}
	filters := []any{}
	if serial != "" {
		filters = append(filters, map[string]any{
			"className": "string", "fieldIdentifier": "serialNumber", "format": "input",
			"operator": "CONTAINS", "value": []string{serial},
		})
	}
	body := map[string]any{
		"category": category,
		"spaceId":  spaceID,
		"page":     1,
		"perPage":  20,
		"orderBy":  "gmtModified",
		"sort":     "desc",
	}
	if len(filters) > 0 {
		conds := map[string]any{"conditionGroups": []any{filters}}
		cb, _ := json.Marshal(conds)
		body["conditions"] = string(cb)
	}
	var out any
	if err := c.Post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	items := client.ExtractListItems(out)
	for _, raw := range items {
		m, _ := raw.(map[string]any)
		if m == nil {
			continue
		}
		if id != "" && zhiyi.InternalID(m) == id {
			return m, nil
		}
		if serial != "" && zhiyi.SerialNumber(m) == serial {
			return m, nil
		}
	}
	return nil, fmt.Errorf("status refresh: no search hit for serial=%q id=%q", serial, id)
}

// workitemCategoryForSearch picks the SearchWorkitems category body field from a
// GetWorkitem object (categoryId, else workitemType.category).
func workitemCategoryForSearch(item map[string]any) string {
	if item == nil {
		return ""
	}
	if c, _ := item["categoryId"].(string); strings.TrimSpace(c) != "" {
		return strings.TrimSpace(c)
	}
	wt, _ := item["workitemType"].(map[string]any)
	if wt != nil {
		for _, k := range []string{"category", "categoryId"} {
			if c, _ := wt[k].(string); strings.TrimSpace(c) != "" {
				return strings.TrimSpace(c)
			}
		}
	}
	return ""
}