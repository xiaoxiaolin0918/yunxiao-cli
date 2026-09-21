package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
)

func TestMrsListSearchHint(t *testing.T) {
	got := mrsListSearchHint("iipmes_gy", `fix "x"`)
	if !strings.Contains(got, "--repo iipmes_gy") || !strings.Contains(got, "--search") {
		t.Fatal(got)
	}
}

func TestWithWriteDedupeHintPassthrough(t *testing.T) {
	err := withWriteDedupeHint(nil, "x")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithWriteDedupeHint_SingleLine(t *testing.T) {
	base := fmt.Errorf("request failed: connection reset by peer")
	fromClient := client.AnnotateWriteNetworkError(base, "POST", "")
	got := withWriteDedupeHint(fromClient, mrsListSearchHint("repo1", "title1"))
	msg := got.Error()
	if strings.Count(msg, "check before retry:") != 1 {
		t.Fatalf("stacked hints: %q", msg)
	}
	if !strings.Contains(msg, "mrs list --repo repo1") {
		t.Fatalf("%q", msg)
	}
}
