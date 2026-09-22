package workflow

import (
	"strings"
)

// Outcome classifies a PUT status=to result.
type Outcome int

const (
	// OutcomeOK: transition succeeded; edge exists; item is at `to`.
	OutcomeOK Outcome = iota
	// OutcomeDenied: invalid / not allowed transition; no edge.
	OutcomeDenied
	// OutcomeNeedsFields: transition is valid but required fields missing
	// (or field validation after accepting the transition intent).
	OutcomeNeedsFields
	// OutcomeOther: unclassified error (treat conservatively as no edge).
	OutcomeOther
)

func (o Outcome) String() string {
	switch o {
	case OutcomeOK:
		return "ok"
	case OutcomeDenied:
		return "denied"
	case OutcomeNeedsFields:
		return "needs_fields"
	default:
		return "other"
	}
}

// ClassifyTransitionError inspects API error text (Chinese/English) seen in Yunxiao verifies.
// msg should include errorMessage / errorMsg body text when available.
func ClassifyTransitionError(msg string) Outcome {
	m := strings.ToLower(msg)
	if m == "" {
		return OutcomeOther
	}

	// Explicit "cannot transition" / invalid status move.
	deniedSnippets := []string{
		"不能流转到目标状态",
		"不支持修改",
		"不支持流转",
		"无法流转",
		"不能流转",
		"invalid transition",
		"not allowed to transition",
		"cannot transition",
		"unsupported status",
	}
	for _, s := range deniedSnippets {
		if strings.Contains(m, strings.ToLower(s)) {
			return OutcomeDenied
		}
	}

	// Field validation: edge likely exists but needs fields / wrong field ids.
	fieldSnippets := []string{
		"不能为空",
		"必填",
		"未启用此字段",
		"does not contains field",
		"does not contain field",
		"required field",
		"missing required",
		"字段",
	}
	// Prefer field-hint when message clearly about fields and not a hard deny.
	for _, s := range fieldSnippets {
		if strings.Contains(m, strings.ToLower(s)) {
			// "字段【x】不能为空" etc.
			if strings.Contains(m, "不能流转") {
				return OutcomeDenied
			}
			return OutcomeNeedsFields
		}
	}

	return OutcomeOther
}

// ExtractAPIErrorBody pulls a shorter snippet from client.APIError-style messages.
func ExtractAPIErrorBody(errMsg string) string {
	if errMsg == "" {
		return ""
	}
	// Prefer JSON errorMessage / errorMsg when present.
	for _, key := range []string{`"errorMessage":"`, `"errorMsg":"`} {
		if i := strings.Index(errMsg, key); i >= 0 {
			rest := errMsg[i+len(key):]
			if j := strings.Index(rest, `"`); j >= 0 {
				return unescapeJSONString(rest[:j])
			}
		}
	}
	if len(errMsg) > 300 {
		return errMsg[:300] + "…"
	}
	return errMsg
}

func unescapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

// ExtractMissingFieldNames pulls Chinese 【field】 markers and common required-field phrases
// from API error text (e.g. 字段【计划完成时间】不能为空). Dedupes, preserves order.
func ExtractMissingFieldNames(msg string) []string {
	if msg == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	rest := msg
	for {
		i := strings.Index(rest, "【")
		if i < 0 {
			break
		}
		rest = rest[i+len("【"):]
		j := strings.Index(rest, "】")
		if j < 0 {
			break
		}
		add(rest[:j])
		rest = rest[j+len("】"):]
	}
	return out
}

