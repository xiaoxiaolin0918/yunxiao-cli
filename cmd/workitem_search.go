package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var workitemSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search work items",
	Long: `Risk: read (no --yes required)
HTTP: POST .../workitems:search

Date window flags map into conditions.conditionGroups using the official
BETWEEN / dateTime / input shape (same as docs for gmtCreate):

  --created-after/--created-before → gmtCreate
  --updated-after/--updated-before → gmtModified
  --finish-after/--finish-before   → finishTime

Status filters (official SearchWorkitems filterObject shapes):

  --status       → fieldIdentifier status, className status, CONTAINS, CSV of status ids
  --status-stage → fieldIdentifier statusStage, className statusStage, CONTAINS, CSV
  --labels       → fieldIdentifier tag, className tag, multiList, CONTAINS, CSV of label ids (#141)

Datetime format: "YYYY-MM-DD HH:MM:SS" (e.g. "2026-09-01 00:00:00").
Bounds are treated as inclusive (OpenAPI BETWEEN value[0]..toValue) unless the
API documents otherwise. Use the same timezone convention as the Yunxiao UI /
OpenAPI examples (typically Asia/Shanghai wall time for China tenants).

Server-side date conditions may still return out-of-window rows. Scripts should
client-filter on gmtCreate / gmtModified / customFieldValues (and finishTime when
present) as needed. Inspect meta.request.conditions to see what was sent.

Subject filters:
  --subject       single substring CONTAINS (backward compatible)
  --subject-all   CSV of terms; each becomes a subject CONTAINS in the same
                  condition group (AND). Accepts ASCII or Chinese commas.
                  Combine with --subject: all terms AND together.

Time window shortcut (#368):
  --recent-days N  maps to --updated-after = (today-N) 00:00:00 Asia/Shanghai
                   (gmtModified). Mutually exclusive with --updated-after.

Combine freely with --assigned-to, --subject, --status / --status-stage, --labels, etc.

Pagination:
  Response meta includes page, perPage, total, totalPages, has_more from x-* headers
  (also nested meta.pagination and raw meta.pagination_headers).
  OpenAPI perPage max is 200 — len(data)==200 is not total; use meta.total / has_more.
  --all follows pages via ListAll (cap 50 pages), dedupes by workitem id.

Stable script shape (opt-in):
  --as-items wraps data as { "items": [...], "pagination": {...} } inside the
  usual { ok, data, meta } envelope. Default remains a raw array in data for
  compatibility. Prefer --as-items for weekly-report scripts.

Schema: yunxiao schema workitem.search

Examples:
  yunxiao workitem search --category Req --created-after "2026-09-01 00:00:00" --created-before "2026-09-07 23:59:59"
  yunxiao workitem search --category Bug --status 100005,100010 --status-stage 1,2
  yunxiao workitem search --category Bug --subject-all "清空,筛选" --recent-days 30
  yunxiao workitem search --category Req --labels <label-id> --all
  yunxiao workitem search --category Req --finish-after "2026-09-01 00:00:00" --finish-before "2026-09-07 23:59:59" --all --as-items

finishTime notes (CLI vs MCP):
  Filtering by finishTime via conditions may work (same BETWEEN pattern; verified live).
  Official oapi SearchWorkitems / GetWorkitem response schemas list gmtCreate,
  gmtModified, and updateStatusAt — not finishTime. Live oapi search/get/activities
  also omit finishTime (custom fields like 测试完成时间 are not finishTime).
  This CLI does not invent finishTime from updateStatusAt and does not enrich it.
  Prefer meta.total + --all for weekly pipelines; MCP as chat fallback only.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		category, _ := cmd.Flags().GetString("category")
		spaceID, _ := cmd.Flags().GetString("space-id")
		spaceID, err := resolveSpaceIDFlag(spaceID)
		if err != nil {
			handleErr(err)
			return
		}
		assignedTo, _ := cmd.Flags().GetString("assigned-to")
		creator, _ := cmd.Flags().GetString("creator")
		subject, _ := cmd.Flags().GetString("subject")
		subjectAll, _ := cmd.Flags().GetString("subject-all")
		recentDays, _ := cmd.Flags().GetInt("recent-days")
		status, _ := cmd.Flags().GetString("status")
		statusStage, _ := cmd.Flags().GetString("status-stage")
		workitemType, _ := cmd.Flags().GetString("type")
		priority, _ := cmd.Flags().GetString("priority")
		orderBy, _ := cmd.Flags().GetString("order-by")
		sort, _ := cmd.Flags().GetString("sort")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		allPages, _ := cmd.Flags().GetBool("all")
		asItems, _ := cmd.Flags().GetBool("as-items")
		createdAfter, _ := cmd.Flags().GetString("created-after")
		createdBefore, _ := cmd.Flags().GetString("created-before")
		updatedAfter, _ := cmd.Flags().GetString("updated-after")
		updatedBefore, _ := cmd.Flags().GetString("updated-before")
		var errRecent error
		updatedAfter, errRecent = resolveWorkitemSearchUpdatedAfter(updatedAfter, recentDays, time.Now())
		if errRecent != nil {
			handleErr(errRecent)
			return
		}
		finishAfter, _ := cmd.Flags().GetString("finish-after")
		finishBefore, _ := cmd.Flags().GetString("finish-before")
		labels, _ := cmd.Flags().GetString("labels")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		assignedTo, err = resolveSelfID(cmd.Context(), c, assignedTo)
		if err != nil {
			handleErr(err)
			return
		}
		creator, err = resolveSelfID(cmd.Context(), c, creator)
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/workitems:search")
		if err != nil {
			handleErr(err)
			return
		}
		filters := buildWorkitemSearchFilters(workitemSearchFilterInput{
			AssignedTo:    assignedTo,
			Creator:       creator,
			Subject:       subject,
			SubjectAll:    subjectAll,
			Status:        status,
			StatusStage:   statusStage,
			WorkitemType:  workitemType,
			Priority:      priority,
			CreatedAfter:  createdAfter,
			CreatedBefore: createdBefore,
			UpdatedAfter:  updatedAfter,
			UpdatedBefore: updatedBefore,
			FinishAfter:   finishAfter,
			FinishBefore:  finishBefore,
			Labels:        labels,
		})
		body := map[string]any{
			"category": category,
			"orderBy":  orderBy,
			"sort":     sort,
			"page":     page,
			"perPage":  perPage,
			"spaceId":  spaceID,
		}
		var conditionsStr string
		if len(filters) > 0 {
			conds := map[string]any{"conditionGroups": []any{filters}}
			cb, _ := json.Marshal(conds)
			conditionsStr = string(cb)
			body["conditions"] = conditionsStr
		}
		reqMeta := workitemSearchRequestMeta(body, conditionsStr)
		baseMeta := map[string]any{"risk": risk.Read, "request": reqMeta}
		if asItems {
			baseMeta["envelope"] = "items"
		}

		if allPages {
			handleErr(runWorkitemSearchAll(cmd.Context(), c, path, body, perPage, baseMeta, asItems))
			return
		}
		handleErr(runRead(cmd.Context(), c, "POST", path, nil, body, baseMeta, func(out any, meta map[string]any) (any, map[string]any) {
			items := client.ExtractListItems(out)
			n := 0
			if items != nil {
				n = len(items)
			} else if s, ok := out.([]any); ok {
				n = len(s)
			}
			client.ApplyFullPageHasMoreHeuristic(meta, n, perPage)
			if asItems {
				out = wrapDataAsItems(out, meta)
			}
			return out, meta
		}))
	},
}

type workitemSearchFilterInput struct {
	AssignedTo, Creator, Subject, SubjectAll, Status, StatusStage, WorkitemType, Priority, Labels string
	CreatedAfter, CreatedBefore, UpdatedAfter, UpdatedBefore, FinishAfter, FinishBefore           string
}

func buildWorkitemSearchFilters(in workitemSearchFilterInput) []any {
	var filters []any
	filters = appendUserFilter(filters, "assignedTo", in.AssignedTo)
	filters = appendUserFilter(filters, "creator", in.Creator)
	for _, term := range subjectSearchTerms(in.Subject, in.SubjectAll) {
		filters = append(filters, map[string]any{
			"className": "string", "fieldIdentifier": "subject", "format": "input",
			"operator": "CONTAINS", "value": []string{term},
		})
	}
	filters = appendStatusFilter(filters, in.Status)
	filters = appendStatusStageFilter(filters, in.StatusStage)
	if in.WorkitemType != "" {
		filters = append(filters, map[string]any{
			"className": "workitemType", "fieldIdentifier": "workitemType", "format": "list",
			"operator": "CONTAINS", "value": splitCSV(in.WorkitemType),
		})
	}
	if in.Priority != "" {
		filters = append(filters, map[string]any{
			"className": "option", "fieldIdentifier": "priority", "format": "list",
			"operator": "CONTAINS", "value": splitCSV(in.Priority),
		})
	}
	filters = appendDateRangeFilter(filters, "gmtCreate", in.CreatedAfter, in.CreatedBefore)
	filters = appendDateRangeFilter(filters, "gmtModified", in.UpdatedAfter, in.UpdatedBefore)
	filters = appendDateRangeFilter(filters, "finishTime", in.FinishAfter, in.FinishBefore)
	filters = appendLabelsFilter(filters, in.Labels)
	return filters
}

// appendLabelsFilter mirrors official SearchWorkitems tag filter (#141):
// fieldIdentifier tag / className tag / multiList / CONTAINS / CSV label ids.
func appendLabelsFilter(filters []any, labelsCSV string) []any {
	if labelsCSV == "" {
		return filters
	}
	return append(filters, map[string]any{
		"className": "tag", "fieldIdentifier": "tag", "format": "multiList",
		"operator": "CONTAINS", "value": splitCSV(labelsCSV),
	})
}

// appendStatusFilter mirrors official docs: status / status / list / CONTAINS / CSV ids.
func appendStatusFilter(filters []any, statusCSV string) []any {
	if statusCSV == "" {
		return filters
	}
	return append(filters, map[string]any{
		"className": "status", "fieldIdentifier": "status", "format": "list",
		"operator": "CONTAINS", "value": splitCSV(statusCSV),
	})
}

// appendStatusStageFilter mirrors --status-stage wiring into conditions.
func appendStatusStageFilter(filters []any, statusStageCSV string) []any {
	if statusStageCSV == "" {
		return filters
	}
	return append(filters, map[string]any{
		"className": "statusStage", "fieldIdentifier": "statusStage", "format": "list",
		"operator": "CONTAINS", "value": splitCSV(statusStageCSV),
	})
}

func workitemSearchRequestMeta(body map[string]any, conditionsStr string) map[string]any {
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	req := map[string]any{
		"body_keys":              keys,
		"unknown_fields_dropped": false, // CLI sends only known SearchWorkitems body fields
		"category":               body["category"],
		"spaceId":                body["spaceId"],
		"orderBy":                body["orderBy"],
		"sort":                   body["sort"],
		"page":                   body["page"],
		"perPage":                body["perPage"],
	}
	if conditionsStr != "" {
		req["conditions"] = conditionsStr
	}
	return req
}

func runWorkitemSearchAll(ctx context.Context, c *client.Client, path string, baseBody map[string]any, perPage int, baseMeta map[string]any, asItems bool) error {
	if baseMeta == nil {
		baseMeta = map[string]any{}
	}
	if globalDryRun {
		previewBody := map[string]any{}
		for k, v := range baseBody {
			previewBody[k] = v
		}
		previewBody["page"] = 1
		previewBody["perPage"] = perPage
		return output.DryRunResult(string(risk.Read), c.Preview("POST", path, nil, previewBody))
	}
	fetch := func(ctx context.Context, page, pp int) (any, http.Header, error) {
		b := map[string]any{}
		for k, v := range baseBody {
			b[k] = v
		}
		b["page"] = page
		b["perPage"] = pp
		var out any
		hdr, err := c.Do(ctx, "POST", path, nil, b, &out)
		return out, hdr, err
	}
	res, err := client.ListAllPages(ctx, 1, perPage, client.DefaultListAllMaxPages, fetch)
	if err != nil {
		return err
	}
	items := client.DedupItemsByID(res.Items)
	meta := map[string]any{}
	for k, v := range baseMeta {
		meta[k] = v
	}
	for k, v := range res.Meta {
		meta[k] = v
	}
	meta["deduped"] = len(res.Items) != len(items)
	meta["result_count"] = len(items)
	var out any = items
	if asItems {
		out = wrapDataAsItems(items, meta)
	}
	return output.Success(out, meta)
}

// subjectSearchTerms builds the AND list for subject CONTAINS filters (#368).
// --subject (single) plus --subject-all CSV (ASCII or Chinese commas) are
// concatenated; empty tokens are dropped. Order is preserved; duplicates kept
// (OpenAPI AND of identical CONTAINS is harmless).
func subjectSearchTerms(subject, subjectAll string) []string {
	var terms []string
	if subject != "" {
		terms = append(terms, subject)
	}
	if subjectAll != "" {
		normalized := strings.ReplaceAll(subjectAll, "，", ",")
		terms = append(terms, splitCSV(normalized)...)
	}
	return terms
}

// recentDaysUpdatedAfter returns gmtModified lower bound as
// "YYYY-MM-DD 00:00:00" in Asia/Shanghai for (calendar today - days).
func recentDaysUpdatedAfter(days int, now time.Time) (string, error) {
	if days <= 0 {
		return "", fmt.Errorf("--recent-days must be > 0 (got %d)", days)
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return "", err
	}
	local := now.In(loc)
	y, m, d := local.Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, loc).AddDate(0, 0, -days)
	return start.Format("2006-01-02 15:04:05"), nil
}

// resolveWorkitemSearchUpdatedAfter applies --recent-days onto updated-after.
// Explicit --updated-after and --recent-days together is an error.
func resolveWorkitemSearchUpdatedAfter(updatedAfter string, recentDays int, now time.Time) (string, error) {
	if recentDays == 0 {
		return updatedAfter, nil
	}
	if updatedAfter != "" {
		return "", fmt.Errorf("--recent-days and --updated-after are mutually exclusive; pick one")
	}
	return recentDaysUpdatedAfter(recentDays, now)
}
