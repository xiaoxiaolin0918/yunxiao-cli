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
	// Prefer id deep link even when serial is also provided.
	got, err = WorkItem("space1", "wid", "ZYPT-1", "Bug")
	if err != nil || !strings.Contains(got.URL, "openWorkitemIdentifier=wid") || strings.Contains(got.URL, "serialNumber=") {
		t.Fatalf("id+serial: %s %v", got.URL, err)
	}
	// Serial-only: short Projex serial_url (#400), not ?serialNumber=.
	got, err = WorkItem("space1", "", "ZYPT-1", "Req")
	if err != nil || got.URL != "https://devops.aliyun.com/projex/req/ZYPT-1#" {
		t.Fatalf("serial Req: %s %v", got.URL, err)
	}
	got, err = WorkItem("space1", "", "ZYPT-2", "Task")
	if err != nil || got.URL != "https://devops.aliyun.com/projex/task/ZYPT-2#" {
		t.Fatalf("serial Task: %s %v", got.URL, err)
	}
	got, err = WorkItem("space1", "", "ZYPT-3", "Risk")
	if err != nil || got.URL != "https://devops.aliyun.com/projex/risk/ZYPT-3#" {
		t.Fatalf("serial Risk: %s %v", got.URL, err)
	}
	got, err = WorkItem("space1", "", "ZYPT-4", "")
	if err != nil || got.URL != "https://devops.aliyun.com/projex/req/ZYPT-4#" {
		t.Fatalf("serial default category: %s %v", got.URL, err)
	}
	if strings.Contains(got.URL, "serialNumber=") || strings.Contains(got.URL, "/project/") {
		t.Fatalf("serial-only must not use project/?serialNumber=: %s", got.URL)
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

func TestShouldOpenBrowser(t *testing.T) {
	if ShouldOpenBrowser(false, false) != true {
		t.Fatal("default should open")
	}
	if ShouldOpenBrowser(true, false) {
		t.Fatal("print-only must not open")
	}
	if ShouldOpenBrowser(false, true) {
		t.Fatal("dry-run must not open")
	}
	if ShouldOpenBrowser(true, true) {
		t.Fatal("both must not open")
	}
}