package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func runMrsDiffs(t *testing.T) (output.Envelope, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = false
	t.Cleanup(func() { globalDryRun = prevDry })
	resetStringFlags(t, codeupMrsDiffsCmd, "repo", "local-id")
	rootCmd.SetArgs([]string{"codeup", "mrs", "diffs", "--repo", "4951320", "--local-id", "125"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.String())
	}
	return env, stdout.String(), 0
}

// #94: one call locates the latest patchset: per-item latest + meta.latest_patchset_biz_id.
func TestMrsDiffsMarksLatestPatchset(t *testing.T) {
	s := newMrsCommentServer(t, mrsPatchesFixture)
	env, raw, _ := runMrsDiffs(t)
	if !env.OK {
		t.Fatalf("envelope: %s", raw)
	}
	if env.Meta["latest_patchset_biz_id"] != "ps-v3" || env.Meta["latest_version_no"] != float64(3) {
		t.Fatalf("meta=%#v", env.Meta)
	}
	items, _ := env.Data.([]any)
	if len(items) != 4 {
		t.Fatalf("data=%#v", env.Data)
	}
	latest := 0
	for i, it := range items {
		m := it.(map[string]any)
		if m["latest"] == true {
			latest++
			if m["patchSetBizId"] != "ps-v3" {
				t.Fatalf("wrong latest item %d: %#v", i, m)
			}
		} else if m["latest"] != false {
			t.Fatalf("item %d missing latest bool: %#v", i, m)
		}
		for _, k := range []string{"patchSetBizId", "versionNo", "relatedMergeItemType", "createTime"} {
			if _, ok := m[k]; !ok {
				t.Fatalf("item %d lost field %s", i, k)
			}
		}
	}
	if latest != 1 {
		t.Fatalf("expected exactly one latest item, got %d", latest)
	}
	// order preserved (API order, not sorted)
	if items[0].(map[string]any)["patchSetBizId"] != "ps-target" {
		t.Fatalf("items reordered: %#v", items[0])
	}
	if s.patchGETs != 1 {
		t.Fatalf("patchGETs=%d", s.patchGETs)
	}
}

func TestMrsDiffsNoSourcePatchsetStillOK(t *testing.T) {
	newMrsCommentServer(t, `[{"patchSetBizId":"t","versionNo":1,"relatedMergeItemType":"MERGE_TARGET"}]`)
	env, raw, _ := runMrsDiffs(t)
	if !env.OK {
		t.Fatalf("envelope: %s", raw)
	}
	if _, ok := env.Meta["latest_patchset_biz_id"]; ok {
		t.Fatalf("no source patchset → no meta.latest_patchset_biz_id: %#v", env.Meta)
	}
	items, _ := env.Data.([]any)
	if len(items) != 1 || items[0].(map[string]any)["latest"] != false {
		t.Fatalf("data=%#v", env.Data)
	}
}

func TestMrsDiffsHelpDocumentsLatest(t *testing.T) {
	h := codeupMrsDiffsCmd.Long
	for _, want := range []string{"latest", "latest_patchset_biz_id", "MERGE_SOURCE"} {
		if !strings.Contains(h, want) {
			t.Fatalf("diffs help missing %q: %s", want, h)
		}
	}
}

// latest_version_no is omitted (not 0) when the latest patch set has no versionNo.
func TestMrsDiffsOmitsMissingLatestVersion(t *testing.T) {
	newMrsCommentServer(t, `[{"patchSetBizId":"ps-only","relatedMergeItemType":"MERGE_SOURCE"}]`)
	env, raw, _ := runMrsDiffs(t)
	if !env.OK || env.Meta["latest_patchset_biz_id"] != "ps-only" {
		t.Fatalf("envelope: %s", raw)
	}
	if _, ok := env.Meta["latest_version_no"]; ok {
		t.Fatalf("latest_version_no must be omitted when missing: %#v", env.Meta)
	}
}
