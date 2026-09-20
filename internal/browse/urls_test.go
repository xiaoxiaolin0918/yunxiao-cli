package browse

import (
	"strings"
	"testing"
)

func TestPipeline(t *testing.T) {
	got, err := Pipeline("5272454", "")
	if err != nil || got.URL != "https://flow.aliyun.com/pipelines/5272454" {
		t.Fatalf("pipeline: %+v %v", got, err)
	}
	got, err = Pipeline("5272454", "4")
	if err != nil || got.URL != "https://flow.aliyun.com/pipelines/5272454/builds/4" {
		t.Fatalf("run: %+v %v", got, err)
	}
	if _, err := Pipeline("", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestWorkItem(t *testing.T) {
	got, err := WorkItem("space1", "wid", "", "Bug")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.URL, "space1") || !strings.Contains(got.URL, "openWorkitemIdentifier=wid") {
		t.Fatalf("url=%s", got.URL)
	}
	got, err = WorkItem("space1", "", "ZYPT-1", "Req")
	if err != nil || !strings.Contains(got.URL, "serialNumber=ZYPT-1") {
		t.Fatalf("serial: %s %v", got.URL, err)
	}
}

func TestMergeRequest(t *testing.T) {
	got, err := MergeRequest("https://codeup.aliyun.com/o/r", "3", "")
	if err != nil || got.URL != "https://codeup.aliyun.com/o/r/change/3" {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = MergeRequest("", "", "https://codeup.aliyun.com/o/r/change/9")
	if err != nil || got.URL != "https://codeup.aliyun.com/o/r/change/9" {
		t.Fatalf("detail %+v %v", got, err)
	}
}

func TestAssertHTTPURL(t *testing.T) {
	if err := AssertHTTPURL("https://example.com/a"); err != nil {
		t.Fatal(err)
	}
	if err := AssertHTTPURL("javascript:alert(1)"); err == nil {
		t.Fatal("expected reject")
	}
}
