package cmd

import "testing"

func TestRelationMatchesMRByBusinessIdLocalID(t *testing.T) {
	id := mrExtIdentity{LocalID: "125", ProjectID: "4951320"}
	rel := map[string]any{"businessId": "125", "projectId": "4951320"}
	if !relationMatchesMR(rel, id) {
		t.Fatal("expected match on businessId == localId")
	}
}

func TestRelationMatchesMRNumericProjectId(t *testing.T) {
	id := mrExtIdentity{LocalID: "125", ProjectID: "4951320"}
	// JSON numbers decode as float64; fmt.Sprint would become 4.95132e+06.
	rel := map[string]any{"businessId": "125", "projectId": float64(4951320)}
	if !relationMatchesMR(rel, id) {
		t.Fatal("expected match when projectId is a JSON number")
	}
}

func TestRelationMatchesMRByBizId(t *testing.T) {
	id := mrExtIdentity{
		LocalID:   "3141",
		ProjectID: "4951320",
		BizIDs:    []string{"9857ed39f0f64411acd9c29f06d5f91a"},
	}
	rel := map[string]any{
		"businessId": "9857ed39f0f64411acd9c29f06d5f91a",
		"projectId":  "4951320",
	}
	if !relationMatchesMR(rel, id) {
		t.Fatal("expected match on MR bizId stored as businessId")
	}
}

func TestRelationMatchesMRByURL(t *testing.T) {
	id := mrExtIdentity{
		LocalID:   "3141",
		ProjectID: "4951320",
		URL:       "https://codeup.aliyun.com/sanzhi/zhiyi/mgy/iipmes_gy/change/3141",
	}
	rel := map[string]any{
		"businessId": "not-the-local-id",
		"projectId":  float64(4951320),
		"url":        "https://codeup.aliyun.com/sanzhi/zhiyi/mgy/iipmes_gy/change/3141",
	}
	if !relationMatchesMR(rel, id) {
		t.Fatal("expected match on extRelation URL")
	}
}

func TestRelationMatchesMRDifferentLocalID(t *testing.T) {
	id := mrExtIdentity{
		LocalID:   "3141",
		ProjectID: "4951320",
		URL:       "https://codeup.aliyun.com/sanzhi/zhiyi/mgy/iipmes_gy/change/3141",
	}
	rel := map[string]any{
		"businessId": "3142",
		"projectId":  "4951320",
		"url":        "https://codeup.aliyun.com/sanzhi/zhiyi/mgy/iipmes_gy/change/3142",
	}
	if relationMatchesMR(rel, id) {
		t.Fatal("must not match a different MR")
	}
}

func TestRelationMatchesMRProjectMismatchOnLocalID(t *testing.T) {
	id := mrExtIdentity{LocalID: "125", ProjectID: "111"}
	rel := map[string]any{"businessId": "125", "projectId": "222"}
	if relationMatchesMR(rel, id) {
		t.Fatal("localId-only match must honor projectId")
	}
}

func TestRelationMatchesMRBangForm(t *testing.T) {
	id := mrExtIdentity{LocalID: "3141", ProjectID: "4951320"}
	rel := map[string]any{"businessId": "iipmes_gy!3141", "projectId": "4951320"}
	if !relationMatchesMR(rel, id) {
		t.Fatal("expected match on repo!localId")
	}
}

func TestMrURLHasLocalIDDoesNotPrefixMatch(t *testing.T) {
	if mrURLHasLocalID("https://codeup.aliyun.com/org/repo/change/31410", "3141") {
		t.Fatal("3141 must not match /change/31410")
	}
	if !mrURLHasLocalID("https://codeup.aliyun.com/org/repo/change/3141?x=1", "3141") {
		t.Fatal("expected /change/3141 match")
	}
}
