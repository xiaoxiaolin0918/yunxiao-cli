package zhiyi

import (
	"strings"
	"testing"
)

func TestUserRefFromValue(t *testing.T) {
	got := UserRefFromValue(map[string]any{"id": "674fb074259532263f38b332", "name": "符宁"})
	if got.ID != "674fb074259532263f38b332" || got.Name != "符宁" {
		t.Fatalf("%+v", got)
	}
	if UserRefFromValue("65cc38447d3e6485f086914e").ID != "65cc38447d3e6485f086914e" {
		t.Fatal("string id")
	}
	if !UserRefFromValue(nil).Empty() || !UserRefFromValue("").Empty() {
		t.Fatal("empty")
	}
}

func TestUserRefDisplay(t *testing.T) {
	if d := (UserRef{ID: "u1", Name: "符宁"}).Display(); d != "符宁 (u1)" {
		t.Fatalf("%q", d)
	}
	if d := (UserRef{ID: "u1"}).Display(); d != "u1" {
		t.Fatalf("%q", d)
	}
	if d := (UserRef{}).Display(); d != "（空）" {
		t.Fatalf("%q", d)
	}
}

func TestDiffWorkItemPersonnelAssignedToChanged(t *testing.T) {
	before := map[string]any{
		"assignedTo": map[string]any{"id": "674fb074259532263f38b332", "name": "符宁"},
		"verifier":   map[string]any{"id": "65cc38447d3e6485f086914e", "name": "高扬"},
	}
	after := map[string]any{
		"assignedTo": map[string]any{"id": "65cc38447d3e6485f086914e", "name": "高扬"},
		"verifier":   map[string]any{"id": "65cc38447d3e6485f086914e", "name": "高扬"},
	}
	changes := DiffWorkItemPersonnel(before, after, PersonnelDiffOpts{ID: "ZYPT-6598"})
	if len(changes) != 1 || changes[0].Field != "assignedTo" {
		t.Fatalf("%+v", changes)
	}
	ch := changes[0]
	if ch.From.ID != "674fb074259532263f38b332" || ch.To.ID != "65cc38447d3e6485f086914e" {
		t.Fatalf("%+v", ch)
	}
	if !strings.Contains(ch.Warning, "assignedTo 已由 符宁 (674fb074259532263f38b332) 变为 高扬 (65cc38447d3e6485f086914e)（平台或字段联动）") {
		t.Fatalf("warning=%q", ch.Warning)
	}
	wantRestore := "yunxiao workitem update --id ZYPT-6598 --assigned-to 674fb074259532263f38b332"
	if ch.Restore != wantRestore || !strings.Contains(ch.Warning, wantRestore) {
		t.Fatalf("restore=%q warning=%q", ch.Restore, ch.Warning)
	}
}

func TestDiffWorkItemPersonnelUnchanged(t *testing.T) {
	item := map[string]any{
		"assignedTo": map[string]any{"id": "u-owner", "name": "符宁"},
		"verifier":   map[string]any{"id": "u-qa", "name": "高扬"},
	}
	after := map[string]any{
		"assignedTo": "u-owner",
		"verifier":   map[string]any{"id": "u-qa", "name": "高扬"},
	}
	if changes := DiffWorkItemPersonnel(item, after, PersonnelDiffOpts{ID: "ZYPT-1"}); len(changes) != 0 {
		t.Fatalf("unexpected changes %+v", changes)
	}
}

func TestDiffWorkItemPersonnelDeveloperAndVerifier(t *testing.T) {
	before := map[string]any{
		"assignedTo": "u-owner",
		"verifier":   map[string]any{"id": "u-qa", "name": "高扬"},
		"customFieldValues": []any{
			map[string]any{"fieldId": "dev-fid", "values": []any{map[string]any{"identifier": "u-dev", "displayValue": "肖晓霖"}}},
			map[string]any{"fieldId": "resp-fid", "values": []any{map[string]any{"identifier": "u-resp", "displayValue": "负责人"}}},
		},
	}
	after := map[string]any{
		"assignedTo": "u-owner",
		"verifier":   map[string]any{"id": "u-qa2", "name": "他人"},
		"customFieldValues": []any{
			map[string]any{"fieldId": "dev-fid", "values": []any{map[string]any{"identifier": "u-dev2", "displayValue": "另一人"}}},
			map[string]any{"fieldId": "resp-fid", "values": []any{map[string]any{"identifier": "u-resp", "displayValue": "负责人"}}},
		},
	}
	changes := DiffWorkItemPersonnel(before, after, PersonnelDiffOpts{
		ID:                 "ZYPT-1",
		DeveloperFieldID:   "dev-fid",
		ResponsibleFieldID: "resp-fid",
	})
	if len(changes) != 2 {
		t.Fatalf("len=%d %+v", len(changes), changes)
	}
	by := map[string]PersonnelChange{}
	for _, ch := range changes {
		by[ch.Field] = ch
	}
	if _, ok := by["verifier"]; !ok {
		t.Fatalf("missing verifier: %+v", changes)
	}
	if by["developer"].To.ID != "u-dev2" {
		t.Fatalf("developer %+v", by["developer"])
	}
	if _, ok := by["responsiblePerson"]; ok {
		t.Fatalf("responsiblePerson should be unchanged: %+v", changes)
	}
}

func TestAttachPersonnelBriefOmitsEmptyVerifier(t *testing.T) {
	brief := map[string]any{"id": "wi-1", "subject": "x"}
	src := map[string]any{
		"assignedTo": map[string]any{"id": "u-owner", "name": "符宁"},
		"verifier":   nil,
	}
	AttachPersonnelBrief(brief, src)
	if brief["assignedTo"] == nil {
		t.Fatal("assignedTo missing")
	}
	if _, ok := brief["verifier"]; ok {
		t.Fatalf("empty verifier must be omitted, got %v", brief["verifier"])
	}
	src["verifier"] = map[string]any{"id": "u-qa", "name": "高扬"}
	AttachPersonnelBrief(brief, src)
	v, _ := brief["verifier"].(map[string]any)
	if v["id"] != "u-qa" {
		t.Fatalf("verifier=%v", brief["verifier"])
	}
}
