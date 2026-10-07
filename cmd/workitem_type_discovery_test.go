package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// wiTypeServer serves the #99/#118 discovery surface: GET .../workitemTypes
// (per-category fixtures, optional per-category failures), GET .../workitemTypes/{t}/fields
// (create precheck), GET .../workitemTypes/{t}/workflows (#118) and POST .../workitems
// (configurable failure bodies). Every request is recorded for assertions.
type wiTypeServer struct {
	mu sync.Mutex
	// categories maps category -> JSON body of the types list response.
	categories map[string]string
	// failingCategories answers HTTP 404 with this body instead of the fixture.
	failingCategories map[string]bool
	fieldsBody        string
	workflowsBody     string
	workflowsStatus   int // non-zero: workflows GET answers with this status
	postStatus        int
	postBody          string

	typesGETs     []string // categories queried (types list shape)
	fieldsGETs    int
	workflowsGETs int
	posts         int
}

func newWiTypeServer(t *testing.T) *wiTypeServer {
	t.Helper()
	s := &wiTypeServer{
		failingCategories: map[string]bool{},
		categories: map[string]string{
			"Req":   `[{"id":"req-enabled","name":"产品类需求","nameEn":"Req"}]`,
			"Bug":   `[{"id":"bug-enabled","name":"缺陷","nameEn":"Bug"}]`,
			"Task":  `[]`,
			"Risk":  `[]`,
			"Topic": `[]`,
		},
		fieldsBody: `[
		 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
		 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true}
		]`,
		workflowsBody: `{"id":"wf-1","name":"缺陷流程","defaultStatusId":"s-open","statuses":[
			{"id":"s-open","name":"待处理","displayName":"待处理","nameEn":"Open"},
			{"id":"s-done","displayName":"已完成"}
		]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(p, "/projects/space-1/workitemTypes"):
			cat := r.URL.Query().Get("category")
			s.typesGETs = append(s.typesGETs, cat)
			if s.failingCategories[cat] {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"errorCode":"NotFound","errorMessage":"no such category"}`)
				return
			}
			_, _ = io.WriteString(w, s.categories[cat])
		case r.Method == http.MethodGet && strings.Contains(p, "/workitemTypes/") && strings.HasSuffix(p, "/fields"):
			s.fieldsGETs++
			_, _ = io.WriteString(w, s.fieldsBody)
		case r.Method == http.MethodGet && strings.Contains(p, "/workitemTypes/") && strings.HasSuffix(p, "/workflows"):
			s.workflowsGETs++
			if s.workflowsStatus != 0 {
				w.WriteHeader(s.workflowsStatus)
				_, _ = io.WriteString(w, s.workflowsBody)
				return
			}
			_, _ = io.WriteString(w, s.workflowsBody)
		case r.Method == http.MethodPost && strings.HasSuffix(p, "/workitems"):
			s.posts++
			if s.postStatus != 0 {
				w.WriteHeader(s.postStatus)
				_, _ = io.WriteString(w, s.postBody)
				return
			}
			_, _ = io.WriteString(w, `{"id":"wi-1","serialNumber":"ZYPT-1","subject":"需求"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"errorCode":"NotFound","errorMessage":"unexpected path `+p+`"}`)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-wi-types-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-types")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return s
}

func (s *wiTypeServer) sortedTypesGETs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]string(nil), s.typesGETs...)
	sort.Strings(out)
	return out
}

func (s *wiTypeServer) countTypesGETs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.typesGETs)
}

type yxRun struct {
	stdout, stderr string
	code           int
}

// runYxCmd executes rootCmd with args (processExit captured as exitPanic).
func runYxCmd(t *testing.T, dryRun bool, args ...string) yxRun {
	t.Helper()
	stdout := withCmdJSONCapture(t) // NOTE: defaults globalDryRun=true
	globalDryRun = dryRun
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		_ = rootCmd.Execute()
	}()
	return yxRun{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func yxEnvelope(t *testing.T, stdout string) output.Envelope {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout)
	}
	return env
}

// yxErrorBody decodes the stderr JSON error envelope, skipping "warning: ..." lines.
func yxErrorBody(t *testing.T, stderr string) output.ErrorBody {
	t.Helper()
	var keep []string
	for _, l := range strings.SplitAfter(stderr, "\n") {
		if !strings.HasPrefix(l, "warning: ") {
			keep = append(keep, l)
		}
	}
	var env output.Envelope
	if err := json.Unmarshal([]byte(strings.Join(keep, "")), &env); err != nil || env.Error == nil {
		t.Fatalf("stderr JSON: %v / %s", err, stderr)
	}
	return *env.Error
}

func allCategoryList() []string {
	return []string{"Bug", "Req", "Risk", "Task", "Topic"}
}

// --- #99a: workitem types list -------------------------------------------------

// Table: without --category every known category is fetched (query param checked)
// and merged so Bug types are visible; an explicit category keeps one GET.
func TestWorkitemTypesListAllCategories(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// wantGETs is the sorted set of category query values the server must see.
		wantGETs []string
		// wantIDs is the expected merged data type id set (sorted comparison).
		wantIDs []string
	}{
		{name: "default-no-category", args: []string{"workitem", "types", "list", "--space-id", "space-1"},
			wantGETs: allCategoryList(), wantIDs: []string{"bug-enabled", "req-enabled"}},
		{name: "explicit-all", args: []string{"workitem", "types", "list", "--space-id", "space-1", "--category", "all"},
			wantGETs: allCategoryList(), wantIDs: []string{"bug-enabled", "req-enabled"}},
		{name: "explicit-bug", args: []string{"workitem", "types", "list", "--space-id", "space-1", "--category", "Bug"},
			wantGETs: []string{"Bug"}, wantIDs: []string{"bug-enabled"}},
		{name: "explicit-req", args: []string{"workitem", "types", "list", "--space-id", "space-1", "--category", "Req"},
			wantGETs: []string{"Req"}, wantIDs: []string{"req-enabled"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newWiTypeServer(t)
			resetStringFlags(t, workitemTypesListCmd, "space-id", "category")
			r := runYxCmd(t, false, tc.args...)
			if r.code != 0 {
				t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
			}
			if got := s.sortedTypesGETs(); !equalStrings(got, tc.wantGETs) {
				t.Fatalf("category GETs=%v want %v", got, tc.wantGETs)
			}
			env := yxEnvelope(t, r.stdout)
			if !env.OK {
				t.Fatalf("envelope: %s", r.stdout)
			}
			assertTypeIDs(t, env.Data, tc.wantIDs, tc.name)
			if len(tc.wantGETs) == 1 {
				// single-category query: no merged meta
				if _, ok := env.Meta["categories"]; ok {
					t.Fatalf("single-category must not add meta.categories: %#v", env.Meta)
				}
				return
			}
			// merged: categories injected, deduped by id
			assertItemCategory(t, env.Data, "bug-enabled", "Bug")
			assertItemCategory(t, env.Data, "req-enabled", "Req")
			if cats, _ := env.Meta["categories"].([]any); len(cats) != 5 || env.Meta["category"] != "all" {
				t.Fatalf("meta=%#v", env.Meta)
			}
			if _, ok := env.Meta["categories_failed"]; ok {
				t.Fatalf("unexpected categories_failed: %#v", env.Meta)
			}
		})
	}
}

// A category GET failure (404, not retried) is skipped with meta.categories_failed
// plus one stderr warning; the remaining categories still print.
func TestWorkitemTypesListPartialCategoryFailure(t *testing.T) {
	s := newWiTypeServer(t)
	s.failingCategories = map[string]bool{"Topic": true, "Risk": true}
	resetStringFlags(t, workitemTypesListCmd, "space-id", "category")
	r := runYxCmd(t, false, "workitem", "types", "list", "--space-id", "space-1")
	if r.code != 0 {
		t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
	}
	if got := s.countTypesGETs(); got != 5 {
		t.Fatalf("category GETs=%d want 5", got)
	}
	env := yxEnvelope(t, r.stdout)
	if !env.OK {
		t.Fatalf("envelope: %s", r.stdout)
	}
	assertTypeIDs(t, env.Data, []string{"req-enabled", "bug-enabled"}, "partial failure")
	failed, _ := env.Meta["categories_failed"].(map[string]any)
	if len(failed) != 2 {
		t.Fatalf("categories_failed=%#v", env.Meta)
	}
	for cat := range failed {
		if cat != "Topic" && cat != "Risk" {
			t.Fatalf("categories_failed keys=%v", failed)
		}
	}
	for _, want := range []string{"warning: types list category Topic failed", "warning: types list category Risk failed"} {
		if !strings.Contains(r.stderr, want) {
			t.Fatalf("stderr missing %q: %q", want, r.stderr)
		}
	}
}

// Table: payload-shape tolerance. A "result"-wrapped list still merges; a payload
// we cannot extract becomes an explicit categories_failed entry, never a silent
// empty category (#99 anti-pattern); a JSON null counts as an empty category.
func TestWorkitemTypesListPayloadShapes(t *testing.T) {
	cases := []struct {
		name        string
		category    string
		body        string
		wantFailed  bool
		wantReason  string
		wantItemsOf []string // ids expected from THIS category when merged
	}{
		{name: "result-wrapped", category: "Task",
			body:        `{"success":true,"result":[{"id":"task-enabled","name":"任务"}]}`,
			wantItemsOf: []string{"task-enabled"}},
		{name: "unrecognized-shape-fails-category", category: "Risk",
			body:       `{"weird": 1}`,
			wantFailed: true, wantReason: "unexpected types payload"},
		{name: "json-null-is-empty-category", category: "Topic", body: `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newWiTypeServer(t)
			s.categories[tc.category] = tc.body
			resetStringFlags(t, workitemTypesListCmd, "space-id", "category")
			r := runYxCmd(t, false, "workitem", "types", "list", "--space-id", "space-1")
			if r.code != 0 {
				t.Fatalf("exit=%d stderr=%s", r.code, r.stderr)
			}
			env := yxEnvelope(t, r.stdout)
			if !env.OK {
				t.Fatalf("envelope: %s", r.stdout)
			}
			assertTypeIDs(t, env.Data, append([]string{"req-enabled", "bug-enabled"}, tc.wantItemsOf...), tc.name)
			failed, _ := env.Meta["categories_failed"].(map[string]any)
			if tc.wantFailed {
				reason, _ := failed[tc.category].(string)
				if !strings.Contains(reason, tc.wantReason) {
					t.Fatalf("categories_failed=%#v", failed)
				}
				if !strings.Contains(r.stderr, "warning: types list category "+tc.category+" failed") {
					t.Fatalf("stderr=%q", r.stderr)
				}
			} else if len(failed) != 0 {
				t.Fatalf("unexpected categories_failed=%#v", failed)
			}
			if tc.wantItemsOf != nil {
				assertItemCategory(t, env.Data, tc.wantItemsOf[0], tc.category)
			}
		})
	}
}

// Every category failing is an error, not an empty success.
func TestWorkitemTypesListAllCategoriesFail(t *testing.T) {
	s := newWiTypeServer(t)
	for cat := range s.categories {
		s.failingCategories[cat] = true
	}
	resetStringFlags(t, workitemTypesListCmd, "space-id", "category")
	r := runYxCmd(t, false, "workitem", "types", "list", "--space-id", "space-1")
	if r.code != 1 || r.stdout != "" {
		t.Fatalf("exit=%d stdout=%s", r.code, r.stdout)
	}
	eb := yxErrorBody(t, r.stderr)
	if eb.Type != "cli" || !strings.Contains(eb.Message, "every category GET failed") {
		t.Fatalf("error=%+v", eb)
	}
}

// Dry-run without --category previews the five category GETs and hits no API.
func TestWorkitemTypesListAllCategoriesDryRun(t *testing.T) {
	s := newWiTypeServer(t)
	resetStringFlags(t, workitemTypesListCmd, "space-id", "category")
	r := runYxCmd(t, true, "workitem", "types", "list", "--space-id", "space-1")
	if r.code != 0 || s.countTypesGETs() != 0 {
		t.Fatalf("exit=%d GETs=%d stderr=%s", r.code, s.countTypesGETs(), r.stderr)
	}
	env := yxEnvelope(t, r.stdout)
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %s", r.stdout)
	}
	previews, ok := env.Request.([]any)
	if !ok || len(previews) != 5 {
		t.Fatalf("request previews=%#v", env.Request)
	}
	for _, p := range previews {
		m, _ := p.(map[string]any)
		if m["method"] != "GET" || !strings.Contains(anyString(m["url"]), "category=") {
			t.Fatalf("preview=%#v", m)
		}
	}
}

// --- #99b: type-not-enabled error enrichment -----------------------------------

// Table: create / +bug-create against 工作项类型未启用 get the enabled-types list
// attached; other API errors pass through unchanged (no types GETs).
func TestWorkitemCreateTypeNotEnabledError(t *testing.T) {
	const notEnabledBody = `{"errorCode":"InvalidParam","errorMessage":"工作项类型未启用！"}`
	const otherBadRequest = `{"errorCode":"InvalidParam","errorMessage":"【所属模块】必填"}`

	t.Run("create-not-enabled", func(t *testing.T) {
		s := newWiTypeServer(t)
		s.postStatus = http.StatusBadRequest
		s.postBody = notEnabledBody
		resetStringFlags(t, workitemCreateCmd,
			"space-id", "type-id", "subject", "subject-file", "assigned-to",
			"description", "description-file", "custom-fields", "custom-fields-file",
			"format-type", "parent-id", "sprint", "labels", "participants", "trackers", "verifier", "versions",
			"no-defaults", "no-precheck", "full")
		r := runYxCmd(t, false, "workitem", "create", "--space-id", "space-1", "--type-id", "type-disabled",
			"--subject", "需求", "--assigned-to", "u1", "--no-defaults")
		assertTypeNotEnabledEnvelope(t, r, s)
	})

	t.Run("create-other-error-passes-through", func(t *testing.T) {
		s := newWiTypeServer(t)
		s.postStatus = http.StatusBadRequest
		s.postBody = otherBadRequest
		resetStringFlags(t, workitemCreateCmd,
			"space-id", "type-id", "subject", "subject-file", "assigned-to",
			"description", "description-file", "custom-fields", "custom-fields-file",
			"format-type", "parent-id", "sprint", "labels", "participants", "trackers", "verifier", "versions",
			"no-defaults", "no-precheck", "full")
		r := runYxCmd(t, false, "workitem", "create", "--space-id", "space-1", "--type-id", "type-req",
			"--subject", "需求", "--assigned-to", "u1", "--no-defaults")
		if r.code != 1 || r.stdout != "" {
			t.Fatalf("exit=%d stdout=%s stderr=%s", r.code, r.stdout, r.stderr)
		}
		eb := yxErrorBody(t, r.stderr)
		if eb.Type != "api" || eb.Code != http.StatusBadRequest || eb.Subtype != "" || !strings.Contains(eb.Message, "所属模块") {
			t.Fatalf("error=%+v", eb)
		}
		if n := s.countTypesGETs(); n != 0 {
			t.Fatalf("no types GET expected on unrelated errors, got %d", n)
		}
	})

	t.Run("bug-create-not-enabled", func(t *testing.T) {
		s := newWiTypeServer(t)
		s.postStatus = http.StatusBadRequest
		s.postBody = notEnabledBody
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		writeTransitionTestProfile(t, xdg, &profile.Profile{
			Name:              "t99bug",
			OrganizationID:    "org-wi-types",
			SpaceID:           "space-1",
			BugTypeID:         "bug-disabled",
			DefaultAssignedTo: "user-assignee",
			DefaultVerifier:   "user-verifier",
			BugCreateFields: profile.BugCreateFields{
				Priority:     map[string]string{"high": "prio-high"},
				SeriousLevel: map[string]string{"normal": "sev-normal"},
			},
		})
		t.Setenv("YUNXIAO_PROFILE", "t99bug")
		globalProfile = "t99bug"
		resetStringFlags(t, workitemBugCreateCmd, "title", "title-file", "description", "description-file",
			"environment", "module", "priority", "serious-level", "expected-completion", "sprint", "assigned-to", "verifier")
		r := runYxCmd(t, false, "workitem", "+bug-create",
			"--title", "t", "--description", "d", "--sprint", "sprint-1", "--minimal", "--yes")
		assertTypeNotEnabledEnvelope(t, r, s)
	})
}

func assertTypeNotEnabledEnvelope(t *testing.T, r yxRun, s *wiTypeServer) {
	t.Helper()
	if r.code != 1 || r.stdout != "" {
		t.Fatalf("exit=%d stdout=%s stderr=%s", r.code, r.stdout, r.stderr)
	}
	if got := s.sortedTypesGETs(); !equalStrings(got, allCategoryList()) {
		t.Fatalf("enrichment must fetch every category, got %v", got)
	}
	eb := yxErrorBody(t, r.stderr)
	if eb.Type != "api" || eb.Code != http.StatusBadRequest {
		t.Fatalf("error=%+v", eb)
	}
	if eb.Subtype != "workitem_type_not_enabled" {
		t.Fatalf("subtype=%q error=%+v", eb.Subtype, eb)
	}
	if !strings.Contains(eb.Message, "工作项类型未启用") {
		t.Fatalf("message=%q", eb.Message)
	}
	for _, want := range []string{"not enabled", "error.details.available_types", "yunxiao workitem types list --space-id space-1"} {
		if !strings.Contains(eb.Hint, want) {
			t.Fatalf("hint missing %q: %q", want, eb.Hint)
		}
	}
	avail, _ := eb.Details["available_types"].([]any)
	if len(avail) != 2 {
		t.Fatalf("available_types=%#v", eb.Details)
	}
	byID := map[string]map[string]string{}
	for _, a := range avail {
		m, _ := a.(map[string]any)
		byID[anyString(m["id"])] = map[string]string{
			"name":     anyString(m["name"]),
			"category": anyString(m["category"]),
		}
	}
	if e := byID["bug-enabled"]; e["name"] != "缺陷" || e["category"] != "Bug" {
		t.Fatalf("bug-enabled=%#v", byID)
	}
	if e := byID["req-enabled"]; e["name"] != "产品类需求" || e["category"] != "Req" {
		t.Fatalf("req-enabled=%#v", byID)
	}
}

// The types lookup on the error path is bounded: a full outage degrades to a hint
// instead of failing the error reporting.
func TestWorkitemCreateTypeNotEnabledLookupFails(t *testing.T) {
	s := newWiTypeServer(t)
	s.postStatus = http.StatusBadRequest
	s.postBody = `{"errorCode":"InvalidParam","errorMessage":"工作项类型未启用！"}`
	for cat := range s.categories {
		s.failingCategories[cat] = true
	}
	resetStringFlags(t, workitemCreateCmd,
		"space-id", "type-id", "subject", "subject-file", "assigned-to",
		"description", "description-file", "custom-fields", "custom-fields-file",
		"format-type", "parent-id", "sprint", "labels", "participants", "trackers", "verifier", "versions",
		"no-defaults", "no-precheck", "full")
	r := runYxCmd(t, false, "workitem", "create", "--space-id", "space-1", "--type-id", "type-disabled",
		"--subject", "需求", "--assigned-to", "u1", "--no-defaults")
	if r.code != 1 || r.stdout != "" {
		t.Fatalf("exit=%d stdout=%s", r.code, r.stdout)
	}
	eb := yxErrorBody(t, r.stderr)
	if eb.Subtype != "workitem_type_not_enabled" {
		t.Fatalf("error=%+v", eb)
	}
	if _, ok := eb.Details["available_types"]; ok {
		t.Fatalf("available_types must be absent on lookup failure: %#v", eb.Details)
	}
	if !strings.Contains(eb.Hint, "could not list enabled types") || !strings.Contains(eb.Hint, "yunxiao workitem types list --space-id space-1") {
		t.Fatalf("hint=%q", eb.Hint)
	}
}

// --- #118: workitem statuses ---------------------------------------------------

// Help and flag docs stay in sync with the merged-default behavior (#99).
func TestWorkitemTypesListHelpDocs(t *testing.T) {
	if !strings.Contains(workitemTypesListCmd.Long, "#99") || !strings.Contains(workitemTypesListCmd.Long, "workitem fields") {
		t.Fatalf("types list help: %s", workitemTypesListCmd.Long)
	}
	if f := workitemTypesListCmd.Flags().Lookup("category"); f == nil || f.DefValue != "" {
		t.Fatalf("category flag default must be empty (all categories): %+v", f)
	}
	if !strings.Contains(workitemCreateCmd.Long, "available_types") || !strings.Contains(workitemBugCreateCmd.Long, "available_types") {
		t.Fatal("create helps must document the type-not-enabled enrichment")
	}
}

// --- helpers -------------------------------------------------------------------

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func assertTypeIDs(t *testing.T, data any, want []string, ctx string) {
	t.Helper()
	items, _ := data.([]any)
	if len(items) != len(want) {
		t.Fatalf("%s: data=%#v want ids %v", ctx, data, want)
	}
	got := make([]string, 0, len(items))
	for _, it := range items {
		m, _ := it.(map[string]any)
		got = append(got, anyString(m["id"]))
	}
	sort.Strings(got)
	sort.Strings(want)
	if !equalStrings(got, want) {
		t.Fatalf("%s: ids=%v want %v", ctx, got, want)
	}
}

func assertItemCategory(t *testing.T, data any, id, category string) {
	t.Helper()
	items, _ := data.([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if anyString(m["id"]) == id {
			if m["category"] != category {
				t.Fatalf("type %s category=%#v want %q", id, m["category"], category)
			}
			return
		}
	}
	t.Fatalf("type %s not found in %#v", id, data)
}
