package profile

import (
	"strings"
	"testing"
)

func catTestProfile() *Profile {
	return &Profile{
		Name:    "cat",
		SpaceID: "space-1",
		WorkitemDefaults: map[string]WorkitemTypeDefaults{
			"type-risk-a": {Name: "风险", Category: "Risk"},
			"type-req-a":  {Name: "产品类需求", Category: "Req"},
			"type-req-b":  {Name: "技术类需求", Category: "Req"},
			"type-task":   {Name: "任务", Category: "Task"},
		},
		Workflows: map[string]WorkitemWorkflow{
			"type-risk-a": {Category: "Risk", Name: "风险"},
		},
	}
}

func TestCategoryTypeCandidatesExplicitKeyFirst(t *testing.T) {
	pf := catTestProfile()
	pf.RiskTypeID = "type-risk-a"
	cands := pf.CategoryTypeCandidates("Risk")
	if len(cands) != 1 || cands[0].TypeID != "type-risk-a" || cands[0].Source != "profile" {
		t.Fatalf("candidates=%#v", cands)
	}
	// Same type id in defaults/workflows is deduped into the explicit entry.
	pf2 := catTestProfile()
	cands2 := pf2.CategoryTypeCandidates("risk") // case-insensitive category
	if len(cands2) != 1 || cands2[0].TypeID != "type-risk-a" || cands2[0].Name != "风险" {
		t.Fatalf("candidates=%#v", cands2)
	}
}

func TestResolveCategoryTypeID(t *testing.T) {
	cases := []struct {
		name        string
		category    string
		profile     func(*Profile)
		wantID      string
		wantErrPart string
	}{
		{"explicit risk key", "Risk", func(p *Profile) { p.RiskTypeID = "type-risk-a" }, "type-risk-a", ""},
		{"explicit req key wins over two defaults", "Req", func(p *Profile) { p.ReqTypeID = "type-req-b" }, "type-req-b", ""},
		{"single discovered candidate", "Risk", func(p *Profile) {}, "type-risk-a", ""},
		{"bug falls back to bug_type_id", "Bug", func(p *Profile) { p.BugTypeID = "bug-1" }, "bug-1", ""},
		{
			"ambiguous lists candidates", "Req", func(p *Profile) {},
			"", "multiple Req workitem types: type-req-a (产品类需求, workitem_defaults), type-req-b (技术类需求, workitem_defaults) — set profile req_type_id",
		},
		{
			"none errors with hint", "Topic", func(p *Profile) {},
			"", "yunxiao workitem types list --space-id space-1 --category Topic",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pf := catTestProfile()
			tc.profile(pf)
			id, err := pf.ResolveCategoryTypeID(tc.category)
			if tc.wantErrPart != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Fatalf("err=%v want containing %q", err, tc.wantErrPart)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if id != tc.wantID {
				t.Fatalf("id=%q want %q", id, tc.wantID)
			}
		})
	}
}

func typedPriorityProfile() *Profile {
	return &Profile{
		Name:    "prio",
		SpaceID: "space-1",
		BugCreateFields: BugCreateFields{
			Priority: map[string]string{"high": "prio-high", "medium": "prio-medium"},
		},
		WorkitemDefaults: map[string]WorkitemTypeDefaults{
			"type-risk": {Category: "Risk", Fields: map[string]WorkitemDefaultField{
				"priority": {Value: "prio-mid", Display: "中", FieldName: "优先级"},
			}},
		},
	}
}

func TestResolveTypedCreatePriority(t *testing.T) {
	cases := []struct {
		name        string
		typeID      string
		in          string
		want        string
		wantErrPart string
	}{
		{name: "alias via bug map", typeID: "type-risk", in: "high", want: "prio-high"},
		{name: "alias case-insensitive", typeID: "type-risk", in: " High ", want: "prio-high"},
		{name: "display value from defaults", typeID: "type-risk", in: "中", want: "prio-mid"},
		{name: "raw option id passes through", typeID: "type-risk", in: "opt-99", want: "opt-99"},
		{name: "empty omits", typeID: "type-risk", in: "", want: ""},
		{
			name: "unmapped known alias errors", typeID: "type-other", in: "urgent",
			wantErrPart: `priority alias "urgent" is not mapped`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pf := typedPriorityProfile()
			got, err := pf.ResolveTypedCreatePriority(tc.typeID, tc.in)
			if tc.wantErrPart != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Fatalf("err=%v want %q", err, tc.wantErrPart)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

// medium without a bug-map entry falls back to the per-type default value.
func TestResolveTypedCreatePriorityMediumFallback(t *testing.T) {
	pf := &Profile{
		Name:    "prio2",
		SpaceID: "space-1",
		WorkitemDefaults: map[string]WorkitemTypeDefaults{
			"type-risk": {Fields: map[string]WorkitemDefaultField{
				"priority": {Value: "prio-mid", Display: "中"},
			}},
		},
	}
	got, err := pf.ResolveTypedCreatePriority("type-risk", "medium")
	if err != nil || got != "prio-mid" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	// Other unmapped aliases do not silently take the default.
	if _, err := pf.ResolveTypedCreatePriority("type-risk", "high"); err == nil {
		t.Fatal("high without map must error")
	}
}
