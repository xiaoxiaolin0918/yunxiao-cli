package workitemfields

import (
	"reflect"
	"testing"
)

// resolveTestFields extends the #95 config fixture (#126): priority 高/低, mod-1 MES,
// note text, plus a multiList field, an ambiguous list field (same display value for
// two ids), a list field whose options have no displayValue, and a user field that
// carries options but is not list-like (must never be resolved).
func resolveTestFields(t *testing.T) []Field {
	t.Helper()
	return append(parse(t, config),
		Field{ID: "cats", Name: "分类", Format: "multiList", Type: "CustomField", Options: []Option{
			{ID: "c-1", Value: "甲", DisplayValue: "甲"},
			{ID: "c-2", Value: "乙", DisplayValue: "乙"},
		}},
		Field{ID: "impact", Name: "影响程度", Format: "list", Type: "CustomField", Options: []Option{
			{ID: "imp-a", Value: "高", DisplayValue: "高"},
			{ID: "imp-b", Value: "高", DisplayValue: "高"},
		}},
		Field{ID: "legacy", Name: "旧字段", Format: "list", Type: "CustomField", Options: []Option{
			{ID: "lg-1", Value: "旧值", DisplayValue: "旧值"}, // displayValue absent: Parse falls back to value
			{ID: "lg-2", Value: "又旧值", DisplayValue: "又旧值"},
		}},
		Field{ID: "members", Name: "成员", Format: "user", Type: "CustomField", Options: []Option{
			{ID: "u-1", Value: "张三", DisplayValue: "张三"},
		}},
	)
}

func TestResolveOptionValues(t *testing.T) {
	cases := []struct {
		name        string
		cf          map[string]any
		wantCF      map[string]any
		wantResolve []ResolvedValue
		wantInvalid []InvalidValue
	}{
		{
			name:        "display value resolves to option id",
			cf:          map[string]any{"priority": "高", "mod-1": "MES"},
			wantCF:      map[string]any{"priority": "prio-high", "mod-1": "m-a"},
			wantResolve: []ResolvedValue{{FieldID: "priority", FieldName: "优先级", From: "高", To: "prio-high"}, {FieldID: "mod-1", FieldName: "所属模块", From: "MES", To: "m-a"}},
		},
		{
			name:   "exact option ids pass through with no record",
			cf:     map[string]any{"priority": "prio-low", "mod-1": "m-a", "note": "x"},
			wantCF: map[string]any{"priority": "prio-low", "mod-1": "m-a", "note": "x"},
		},
		{
			name:   "id with surrounding blanks is normalized to the canonical id",
			cf:     map[string]any{"priority": " prio-low "},
			wantCF: map[string]any{"priority": "prio-low"},
		},
		{
			name:   "multiList array resolves element by element",
			cf:     map[string]any{"cats": []any{"甲", "c-2", 3}},
			wantCF: map[string]any{"cats": []any{"c-1", "c-2", 3}},
			wantResolve: []ResolvedValue{
				{FieldID: "cats", FieldName: "分类", From: "甲", To: "c-1"},
			},
		},
		{
			name:   "[]string multiList resolves too",
			cf:     map[string]any{"cats": []string{"乙"}},
			wantCF: map[string]any{"cats": []string{"c-2"}},
			wantResolve: []ResolvedValue{
				{FieldID: "cats", FieldName: "分类", From: "乙", To: "c-2"},
			},
		},
		{
			name:   "option without displayValue matches on value",
			cf:     map[string]any{"legacy": "旧值"},
			wantCF: map[string]any{"legacy": "lg-1"},
			wantResolve: []ResolvedValue{
				{FieldID: "legacy", FieldName: "旧字段", From: "旧值", To: "lg-1"},
			},
		},
		{
			name:   "blank stays blank (precheck reports it as missing)",
			cf:     map[string]any{"priority": " "},
			wantCF: map[string]any{"priority": " "},
		},
		{
			name:   "non-string values pass through",
			cf:     map[string]any{"priority": 3, "note": true},
			wantCF: map[string]any{"priority": 3, "note": true},
		},
		{
			name:   "fields without options and unknown field ids pass through",
			cf:     map[string]any{"note": "自由文本", "not-in-config": "高"},
			wantCF: map[string]any{"note": "自由文本", "not-in-config": "高"},
		},
		{
			name:   "non-list format with options is never resolved",
			cf:     map[string]any{"members": "张三"},
			wantCF: map[string]any{"members": "张三"},
		},
		{
			name:   "unknown value reports the valid options",
			cf:     map[string]any{"priority": "最高"},
			wantCF: map[string]any{"priority": "最高"},
			wantInvalid: []InvalidValue{{
				FieldID: "priority", Name: "优先级", Format: "list", Value: "最高", Reason: "not_found",
				Options:      []MissingOption{{ID: "prio-high", DisplayValue: "高"}, {ID: "prio-low", DisplayValue: "低"}},
				OptionsTotal: 2,
			}},
		},
		{
			name:   "ambiguous display value reports the colliding options only",
			cf:     map[string]any{"impact": "高"},
			wantCF: map[string]any{"impact": "高"},
			wantInvalid: []InvalidValue{{
				FieldID: "impact", Name: "影响程度", Format: "list", Value: "高", Reason: "ambiguous",
				Options:      []MissingOption{{ID: "imp-a", DisplayValue: "高"}, {ID: "imp-b", DisplayValue: "高"}},
				OptionsTotal: 2,
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := resolveTestFields(t)
			cf := cloneCF(tc.cf)
			resolved, invalid := ResolveOptionValues(fields, cf)
			if !reflect.DeepEqual(cf, tc.wantCF) {
				t.Fatalf("cf=%#v want %#v", cf, tc.wantCF)
			}
			if !reflect.DeepEqual(invalid, tc.wantInvalid) {
				t.Fatalf("invalid=%#v want %#v", invalid, tc.wantInvalid)
			}
			if !reflect.DeepEqual(resolved, tc.wantResolve) {
				t.Fatalf("resolved=%#v want %#v", resolved, tc.wantResolve)
			}
		})
	}
}

// Invalid values for several fields are reported together in field-config order;
// a multiList with one invalid element keeps its whole array untouched, while other
// fields still resolve (the caller fails the create before POSTing anything).
func TestResolveOptionValuesReportsAllInvalidInConfigOrder(t *testing.T) {
	cf := map[string]any{"impact": "高", "cats": []any{"甲", "丙"}, "priority": "高"}
	resolved, invalid := ResolveOptionValues(resolveTestFields(t), cf)
	if len(invalid) != 2 || invalid[0].FieldID != "cats" || invalid[1].FieldID != "impact" {
		t.Fatalf("invalid=%#v", invalid)
	}
	if invalid[0].Value != "丙" || invalid[0].Reason != "not_found" || invalid[1].Reason != "ambiguous" {
		t.Fatalf("invalid=%#v", invalid)
	}
	if cats, _ := cf["cats"].([]any); !reflect.DeepEqual(cats, []any{"甲", "丙"}) {
		t.Fatalf("cats=%#v", cf["cats"])
	}
	if cf["priority"] != "prio-high" || cf["impact"] != "高" {
		t.Fatalf("cf=%#v", cf)
	}
	if len(resolved) != 2 || resolved[0].FieldID != "priority" || resolved[1].FieldID != "cats" {
		t.Fatalf("resolved=%#v", resolved)
	}
}

// Several display matches that all point at one id still resolve (dedup by id).
func TestResolveOptionValuesDuplicateOptionsSameID(t *testing.T) {
	fields := []Field{{ID: "dup", Name: "重复", Format: "list", Options: []Option{
		{ID: "d-1", Value: "同", DisplayValue: "同"},
		{ID: "d-1", Value: "同", DisplayValue: "同"},
	}}}
	cf := map[string]any{"dup": "同"}
	resolved, invalid := ResolveOptionValues(fields, cf)
	if len(invalid) != 0 || cf["dup"] != "d-1" || len(resolved) != 1 || resolved[0].To != "d-1" {
		t.Fatalf("cf=%#v resolved=%#v invalid=%#v", cf, resolved, invalid)
	}
}

// The options list in an InvalidValue is capped at MaxOptions; OptionsTotal keeps
// the real count (same contract as missing-field options in #95).
func TestResolveOptionValuesCapsOptions(t *testing.T) {
	opts := make([]Option, MaxOptions+5)
	for i := range opts {
		id := "o-" + string(rune('a'+i))
		opts[i] = Option{ID: id, Value: id, DisplayValue: id}
	}
	fields := []Field{{ID: "big", Name: "大", Format: "list", Options: opts}}
	_, invalid := ResolveOptionValues(fields, map[string]any{"big": "zzz"})
	if len(invalid) != 1 || len(invalid[0].Options) != MaxOptions || invalid[0].OptionsTotal != MaxOptions+5 {
		t.Fatalf("invalid=%#v", invalid[0])
	}
}

func cloneCF(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
