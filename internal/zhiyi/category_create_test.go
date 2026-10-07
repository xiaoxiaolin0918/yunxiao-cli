package zhiyi

import (
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func catItemProfile() *profile.Profile {
	return &profile.Profile{
		Name:    "cat",
		SpaceID: "space-1",
		BugCreateFields: profile.BugCreateFields{
			Priority: map[string]string{"medium": "prio-medium"},
		},
	}
}

func TestBuildCreateCategoryItemArgs(t *testing.T) {
	pf := catItemProfile()
	body, err := BuildCreateCategoryItemArgs(CreateCategoryItemInput{
		Category:    "Risk",
		TypeID:      " type-risk ",
		Title:       "风险标题",
		Description: "风险说明",
		Priority:    "medium",
		AssignedTo:  "user-1",
	}, pf)
	if err != nil {
		t.Fatal(err)
	}
	if body["spaceId"] != "space-1" || body["workitemTypeId"] != "type-risk" {
		t.Fatalf("body=%#v", body)
	}
	if body["subject"] != "风险标题" || body["description"] != "风险说明" {
		t.Fatalf("body=%#v", body)
	}
	if body["formatType"] != "MARKDOWN" {
		t.Fatalf("formatType=%v", body["formatType"])
	}
	if body["assignedTo"] != "user-1" {
		t.Fatalf("assignedTo=%v", body["assignedTo"])
	}
	// #128: sprint is NOT sent unless explicitly provided.
	if _, ok := body["sprint"]; ok {
		t.Fatalf("sprint must be omitted by default: %#v", body)
	}
	cf, _ := body["customFieldValues"].(map[string]any)
	if cf["priority"] != "prio-medium" {
		t.Fatalf("customFieldValues=%#v", cf)
	}
}

func TestBuildCreateCategoryItemArgsOptionalKeys(t *testing.T) {
	pf := catItemProfile()
	body, err := BuildCreateCategoryItemArgs(CreateCategoryItemInput{
		Category:     "Req",
		TypeID:       "type-req",
		Title:        "t",
		Description:  "d",
		Priority:     "", // omit priority entirely
		Sprint:       " sprint-7 ",
		AssignedTo:   "user-1",
		Participants: []string{"u2", "u3"},
	}, pf)
	if err != nil {
		t.Fatal(err)
	}
	if body["sprint"] != "sprint-7" {
		t.Fatalf("sprint=%#v", body["sprint"])
	}
	if _, ok := body["customFieldValues"]; ok {
		t.Fatalf("priority omitted must drop customFieldValues: %#v", body)
	}
	parts, _ := body["participants"].([]string)
	if len(parts) != 2 || parts[0] != "u2" {
		t.Fatalf("participants=%#v", body["participants"])
	}
}

func TestBuildCreateCategoryItemArgsErrors(t *testing.T) {
	cases := []struct {
		name    string
		input   CreateCategoryItemInput
		pf      *profile.Profile
		wantErr string
	}{
		{"nil profile", CreateCategoryItemInput{TypeID: "t"}, nil, "profile required"},
		{"missing space", CreateCategoryItemInput{TypeID: "t"}, &profile.Profile{Name: "x"}, "missing space_id"},
		{"missing type", CreateCategoryItemInput{}, catItemProfile(), "type id required"},
		{
			"unmapped priority alias", CreateCategoryItemInput{TypeID: "type-x", Priority: "urgent"},
			catItemProfile(), "not mapped",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildCreateCategoryItemArgs(tc.input, tc.pf)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v want %q", err, tc.wantErr)
			}
		})
	}
}
