package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/workitemfields"
)

// transitionRequiredFieldsSubtype marks a transition PUT that the server rejected for
// missing status-entry required fields whose Chinese names the CLI mapped back to
// field ids via the type's field config (#113).
const transitionRequiredFieldsSubtype = "transition_required_fields"

// transitionRequiredFieldsNote is attached to +transition dry-run requests whenever
// no required-field ids are known (#113): status-entry required fields have no
// OpenAPI config (the fields endpoint only marks type-level required), so dry-run
// cannot predict them; the 400 fallback below is the discovery path.
const transitionRequiredFieldsNote = "状态入场必填字段无 OpenAPI 配置（fields 端点只标注类型级 required，dry-run 无法预判）；真实 PUT 若 400「xx必填」，CLI 会把中文字段名映射回 fieldId（error.subtype=transition_required_fields，补齐草稿见 error.details.fields / fields_draft）"

// transitionPutError wraps a failed transition step PUT (#113). Non-400 errors (and
// 400s without a 必填 field list) pass through with today's message. A 400 that lists
// required fields by Chinese name is enriched: the names are matched against the
// type's field config (bounded GET, same budget as the #95 create precheck) and the
// result is a *contextError with error.subtype=transition_required_fields and
// error.details.fields[] (field id, name, current value, enum options, copy-paste
// draft) plus fields_draft for a --fields retry. Unmatched names and mapping
// failures degrade to verbatim passthrough — the server text always stays in
// error.message. item is the pre-PUT GET payload (current values).
func transitionPutError(ctx context.Context, c *client.Client, cmdName string, stepIdx, stepTotal int, applied []string, err error,
	item map[string]any, spaceID, typeID, idArg, toArg string) error {
	if err == nil {
		return nil
	}
	wrapped := fmt.Errorf("流转在第 %d/%d 步失败；已成功：%v；%w", stepIdx, stepTotal, applied, err)
	var ae *client.APIError
	if !errors.As(err, &ae) || ae.Status != http.StatusBadRequest {
		return wrapped
	}
	serverMsg := workitemfields.RequiredFieldMessage(ae.Body)
	names := workitemfields.RequiredFieldNames(serverMsg)
	if len(names) == 0 {
		return wrapped
	}

	details := map[string]any{
		"server_message": serverMsg,
		"step":           fmt.Sprintf("%d/%d", stepIdx, stepTotal),
		"applied":        applied,
	}
	spaceID, typeID = strings.TrimSpace(spaceID), strings.TrimSpace(typeID)
	if spaceID == "" || typeID == "" {
		details["unmapped_names"] = names
		details["mapping"] = map[string]any{"source": "none", "reason": "space_id / type_id unavailable"}
		return &contextError{
			Context: cmdName,
			Hint:    transitionUnmappedHint(spaceID, typeID),
			Err:     wrapped,
			Subtype: transitionRequiredFieldsSubtype,
			Details: details,
		}
	}
	details["inspect"] = fmt.Sprintf("yunxiao workitem fields --space-id %s --type-id %s", spaceID, typeID)

	fields, ferr := fetchWorkitemFieldConfigForError(ctx, c, spaceID, typeID)
	if ferr != nil || len(fields) == 0 {
		reason, source := "field config returned no fields", "fields_endpoint_empty"
		if ferr != nil {
			reason, source = precheckSkipReason(ferr), "fields_endpoint_error"
		}
		details["unmapped_names"] = names
		details["mapping"] = map[string]any{"source": source, "reason": reason}
		return &contextError{
			Context: cmdName,
			Hint:    transitionUnmappedHint(spaceID, typeID),
			Err:     wrapped,
			Subtype: transitionRequiredFieldsSubtype,
			Details: details,
		}
	}

	hits, unmapped := workitemfields.MapRequiredFields(fields, names)
	if len(hits) > 0 {
		details["fields"] = workitemfields.RequiredDetails(hits, item)
		details["fields_draft"] = workitemfields.FieldsDraft(hits)
	}
	if len(unmapped) > 0 {
		details["unmapped_names"] = unmapped
	}
	details["mapping"] = map[string]any{"source": "fields_endpoint", "matched": len(hits), "unmapped": len(unmapped)}
	return &contextError{
		Context: cmdName,
		Hint:    transitionRetryHint(cmdName, idArg, toArg, hits, spaceID, typeID),
		Err:     wrapped,
		Subtype: transitionRequiredFieldsSubtype,
		Details: details,
	}
}

// transitionRetryHint is the copy-paste retry suggestion when at least one name
// mapped: rerun the same command with --fields carrying the draft skeleton
// (root keys included, so sprint ≠ customFields).
func transitionRetryHint(cmdName, idArg, toArg string, hits []workitemfields.RequiredHit, spaceID, typeID string) string {
	if len(hits) == 0 {
		return transitionUnmappedHint(spaceID, typeID)
	}
	draft, err := json.Marshal(workitemfields.FieldsDraft(hits))
	if err != nil {
		return transitionUnmappedHint(spaceID, typeID)
	}
	idPart, toPart := idArg, toArg
	if idPart == "" {
		idPart = "<id>"
	}
	if toPart == "" {
		toPart = "<to>"
	}
	return fmt.Sprintf("补齐必填后重试：%s --id %s --to %s --fields '%s' --yes（sprint 用迭代 id，枚举字段用 error.details.fields[].options 里的 option id，日期用具体时间；系统字段也可走 workitem update 的对应 flag，见 error.details.fields[].draft）",
		cmdName, idPart, toPart, string(draft))
}

// transitionUnmappedHint is the degraded hint: nothing could be mapped, so point at
// the field config and keep the server text as the source of truth.
func transitionUnmappedHint(spaceID, typeID string) string {
	if spaceID == "" || typeID == "" {
		return "服务器按中文字段名报必填，但 CLI 缺少 space/type 信息无法映射 fieldId；原文见 error.details.server_message"
	}
	return fmt.Sprintf("服务器报必填的中文字段名未能在字段配置中匹配（原文见 error.details.server_message / unmapped_names，不猜测）；对照：yunxiao workitem fields --space-id %s --type-id %s",
		spaceID, typeID)
}

// fetchWorkitemFieldConfigForError reads the field config on the failure path with
// the precheck budget (2 attempts, ≤1s backoff, 10s total) so a degraded fields
// endpoint cannot stall an already-failed command (#113; same bounds as #95).
func fetchWorkitemFieldConfigForError(ctx context.Context, c *client.Client, spaceID, typeID string) ([]workitemfields.Field, error) {
	pctx, cancel := context.WithTimeout(client.WithRetryPolicy(ctx, precheckMaxAttempts, precheckMaxDelay), precheckTimeout)
	defer cancel()
	return fetchWorkitemFieldConfig(pctx, c, spaceID, typeID)
}
