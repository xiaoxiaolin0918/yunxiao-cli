package cmd

import (
	"strings"
	"testing"
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
