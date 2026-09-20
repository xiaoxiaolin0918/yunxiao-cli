package pipelinescan

import (
	"errors"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
)

func TestIsNoPermission(t *testing.T) {
	if !IsNoPermission(&client.APIError{Status: 403, Body: `{"errorCode":"InvalidPipeline.NotHavePipelinePermission"}`}) {
		t.Fatal("403 should be no-permission")
	}
	if IsNoPermission(&client.APIError{Status: 500, Body: "boom"}) {
		t.Fatal("500 should not")
	}
	if IsNoPermission(errors.New("plain")) {
		t.Fatal("plain should not")
	}
}

func TestReportRecord_softCollect(t *testing.T) {
	r := &Report{}
	r.Record("5083035", "runs/WAITING", &client.APIError{Status: 403, Body: "no perm"})
	r.Record("1", "run/9", &client.APIError{Status: 500, Body: "oops"})
	r.NoteScannedPipeline("5083035")
	r.NoteScannedPipeline("1")
	r.NoteScannedPipeline("2")
	if r.Scanned != 3 {
		t.Fatalf("scanned=%d", r.Scanned)
	}
	if len(r.SkippedNoPermission) != 1 || r.SkippedNoPermission[0] != "5083035" {
		t.Fatalf("skipped=%v", r.SkippedNoPermission)
	}
	if len(r.Errors) != 2 {
		t.Fatalf("errors=%v", r.Errors)
	}
	meta := r.Meta()
	if meta["scanned"] != 3 {
		t.Fatalf("meta scanned %#v", meta["scanned"])
	}
	if meta["skipped_no_permission_count"] != 1 {
		t.Fatalf("meta skip count %#v", meta)
	}
	if meta["error_count"] != 2 {
		t.Fatalf("meta error_count %#v", meta)
	}
}
