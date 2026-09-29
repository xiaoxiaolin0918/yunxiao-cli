package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// runMrsDiffs runs `codeup mrs diffs` with processExit intercepted. On success it
// returns the stdout envelope; when the command exits it returns the stderr envelope,
// the raw stderr and the exit code.
func runMrsDiffs(t *testing.T) (output.Envelope, string, int) {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = false
	t.Cleanup(func() { globalDryRun = prevDry })
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) {
		code = c
		panic(exitPanic{code: c})
	}
	t.Cleanup(func() { processExit = prevExit })
	resetStringFlags(t, codeupMrsDiffsCmd, "repo", "local-id")
	rootCmd.SetArgs([]string{"codeup", "mrs", "diffs", "--repo", "4951320", "--local-id", "125"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	}()
	raw := stdout.Bytes()
	if code != 0 {
		raw = stderr.Bytes()
	}
	var env output.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("output JSON (exit %d): %v / stdout=%s stderr=%s", code, err, stdout.String(), stderr.String())
	}
	return env, string(raw), code
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
	for _, want := range []string{"latest", "latest_patchset_biz_id", "MERGE_SOURCE", "overwritten", "assumes data is an array"} {
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

// Command layer: an empty patch set list is still ok:true with no latest meta.
func TestMrsDiffsEmptyList(t *testing.T) {
	newMrsCommentServer(t, `[]`)
	env, raw, code := runMrsDiffs(t)
	if code != 0 || !env.OK {
		t.Fatalf("exit=%d envelope: %s", code, raw)
	}
	if items, ok := env.Data.([]any); !ok || len(items) != 0 {
		t.Fatalf("data=%#v", env.Data)
	}
	for _, k := range []string{"latest_patchset_biz_id", "latest_version_no"} {
		if _, ok := env.Meta[k]; ok {
			t.Fatalf("empty list must not set meta.%s: %#v", k, env.Meta)
		}
	}
}

// "latest" is injected by the CLI and overwrites a same-named API field.
func TestMrsDiffsOverwritesSameNamedLatestField(t *testing.T) {
	newMrsCommentServer(t, `[
 {"patchSetBizId":"a","versionNo":1,"relatedMergeItemType":"MERGE_SOURCE","latest":"yes"},
 {"patchSetBizId":"b","versionNo":2,"relatedMergeItemType":"MERGE_SOURCE","latest":false}
]`)
	env, raw, code := runMrsDiffs(t)
	if code != 0 || !env.OK {
		t.Fatalf("exit=%d envelope: %s", code, raw)
	}
	items := env.Data.([]any)
	if items[0].(map[string]any)["latest"] != false || items[1].(map[string]any)["latest"] != true {
		t.Fatalf("API latest fields must be overwritten: %#v", items)
	}
}

// Typed entries without MERGE_SOURCE: untyped items are not candidates (same as #93).
func TestMrsDiffsTargetWithUntypedNoCandidate(t *testing.T) {
	newMrsCommentServer(t, `[{"patchSetBizId":"t","versionNo":2,"relatedMergeItemType":"MERGE_TARGET"},{"patchSetBizId":"u","versionNo":1}]`)
	env, raw, code := runMrsDiffs(t)
	if code != 0 || !env.OK {
		t.Fatalf("exit=%d envelope: %s", code, raw)
	}
	if _, ok := env.Meta["latest_patchset_biz_id"]; ok {
		t.Fatalf("meta=%#v", env.Meta)
	}
	for i, it := range env.Data.([]any) {
		if it.(map[string]any)["latest"] != false {
			t.Fatalf("item %d must be latest:false: %#v", i, it)
		}
	}
}

// The patch set marked latest by diffs is exactly the one comments create picks by default.
func TestMrsDiffsLatestMatchesCommentsCreateDefault(t *testing.T) {
	s := newMrsCommentServer(t, mrsPatchesFixture)
	env, raw, code := runMrsDiffs(t)
	if code != 0 || !env.OK {
		t.Fatalf("exit=%d envelope: %s", code, raw)
	}
	diffsLatest := env.Meta["latest_patchset_biz_id"]
	var flagged any
	for _, it := range env.Data.([]any) {
		if m := it.(map[string]any); m["latest"] == true {
			flagged = m["patchSetBizId"]
		}
	}
	if _, stderr, code := runMrsCommentsCreate(t, false); code != 0 {
		t.Fatalf("comments create exit=%d stderr=%s", code, stderr)
	}
	if len(s.posts) != 1 {
		t.Fatalf("posts=%#v", s.posts)
	}
	picked := s.posts[0]["patchset_biz_id"]
	if diffsLatest == nil || diffsLatest != picked || flagged != picked {
		t.Fatalf("diffs meta=%v flagged=%v, comments create picked=%v", diffsLatest, flagged, picked)
	}
}

// A failing GET exits 1 with the API error on stderr (no partial stdout).
func TestMrsDiffsHTTPErrorExits(t *testing.T) {
	s := newMrsCommentServer(t, mrsPatchesFixture)
	s.patchStatus = 403
	env, raw, code := runMrsDiffs(t)
	if code != 1 || env.OK || env.Error == nil || env.Error.Type != "api" || env.Error.Code != 403 {
		t.Fatalf("exit=%d envelope: %s", code, raw)
	}
}
