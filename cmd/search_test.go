package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func TestResolveUnifiedSearchRouteAliases(t *testing.T) {
	cases := []struct {
		in, wantCanonical, wantPath, wantQuery string
	}{
		{"workitem", "workitem", "workitem search", "subject"},
		{"ISSUE", "workitem", "workitem search", "subject"},
		{"mr", "mr", "codeup mrs list", "search"},
		{"mrs", "mr", "codeup mrs list", "search"},
		{"repo", "repo", "codeup repos list", "search"},
		{"repos", "repo", "codeup repos list", "search"},
		{"case", "case", "testhub cases search", ""},
		{"cases", "case", "testhub cases search", ""},
		{"app-tag", "app-tag", "appstack tags search", "search"},
		{"pipeline", "pipeline", "pipeline list", "name"},
	}
	for _, tc := range cases {
		got, err := resolveUnifiedSearchRoute(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if got.Canonical != tc.wantCanonical || got.TargetPath != tc.wantPath || got.QueryFlag != tc.wantQuery {
			t.Fatalf("%s: got %+v", tc.in, got)
		}
		if got.Target == nil || got.Target.Run == nil {
			t.Fatalf("%s: missing target Run", tc.in)
		}
	}
}

func TestResolveUnifiedSearchRouteUnknown(t *testing.T) {
	_, err := resolveUnifiedSearchRoute("packages")
	if err == nil || !strings.Contains(err.Error(), "unknown search type") {
		t.Fatalf("err=%v", err)
	}
	_, err = resolveUnifiedSearchRoute("")
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseUnifiedSearchArgs(t *testing.T) {
	typ, q, rest, err := parseUnifiedSearchArgs([]string{"workitem", "--subject", "x", "--dry-run"})
	if err != nil || typ != "workitem" || q != "" {
		t.Fatalf("pos: typ=%q q=%q err=%v", typ, q, err)
	}
	if strings.Join(rest, " ") != "--subject x --dry-run" {
		t.Fatalf("rest=%v", rest)
	}

	typ, q, rest, err = parseUnifiedSearchArgs([]string{"--type", "mr", "--state", "opened", "--q", "fix"})
	if err != nil || typ != "mr" || q != "fix" {
		t.Fatalf("type+q: typ=%q q=%q err=%v", typ, q, err)
	}
	if strings.Join(rest, " ") != "--state opened" {
		t.Fatalf("rest=%v", rest)
	}

	// First --type is router; second --type is for workitem.
	typ, q, rest, err = parseUnifiedSearchArgs([]string{"--type", "workitem", "--type", "tid-1", "--subject", "a"})
	if err != nil || typ != "workitem" || q != "" {
		t.Fatalf("double type: typ=%q q=%q err=%v", typ, q, err)
	}
	if strings.Join(rest, " ") != "--type tid-1 --subject a" {
		t.Fatalf("rest=%v", rest)
	}

	_, _, _, err = parseUnifiedSearchArgs([]string{"--q", "a", "--query", "b"})
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflict err=%v", err)
	}
}

func TestInjectMappedQueryFlag(t *testing.T) {
	got, err := injectMappedQueryFlag([]string{"--category", "Bug"}, "subject", "login")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, " ") != "--category Bug --subject login" {
		t.Fatalf("%v", got)
	}
	got, err = injectMappedQueryFlag([]string{"--subject", "login"}, "subject", "login")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, " ") != "--subject login" {
		t.Fatalf("idempotent %v", got)
	}
	_, err = injectMappedQueryFlag([]string{"--subject", "other"}, "subject", "login")
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("err=%v", err)
	}
}

func TestUnifiedSearchWorkitemQueryDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-unified-search-wi-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-wi")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemSearchCmd,
		"category", "space-id", "assigned-to", "creator", "subject", "status",
		"status-stage", "type", "priority", "labels", "order-by", "sort",
		"created-after", "created-before", "updated-after", "updated-before",
		"finish-after", "finish-before",
	)
	_ = workitemSearchCmd.Flags().Set("page", "1")
	_ = workitemSearchCmd.Flags().Set("per-page", "20")
	_ = workitemSearchCmd.Flags().Set("all", "false")
	_ = workitemSearchCmd.Flags().Set("as-items", "false")

	rootCmd.SetArgs([]string{
		"search", "issue",
		"--space-id", "space-us-1",
		"--category", "Bug",
		"--q", "login-fail",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.Read) && env.Risk != "read" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	body, _ := req["body"].(map[string]any)
	if body == nil || body["spaceId"] != "space-us-1" || body["category"] != "Bug" {
		t.Fatalf("body=%v", body)
	}
	cond, _ := body["conditions"].(string)
	if !strings.Contains(cond, "subject") || !strings.Contains(cond, "login-fail") {
		t.Fatalf("conditions=%q", cond)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestUnifiedSearchReposQueryDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-unified-search-repo-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-repo")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupReposListCmd, "search")
	for _, name := range []string{"page", "per-page"} {
		f := codeupReposListCmd.Flags().Lookup(name)
		if f != nil {
			_ = codeupReposListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs([]string{"search", "--type", "repos", "--query", "yunxiao-cli", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories") || !strings.Contains(url, "search=yunxiao-cli") {
		t.Fatalf("url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestUnifiedSearchUnknownTypeExits(t *testing.T) {
	t.Setenv(config.EnvAccessToken, "test-token-unified-search-unk-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-unk")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv("YUNXIAO_PROFILE", "")

	var stderr bytes.Buffer
	prevErr := output.Stderr
	prevOut := output.Stdout
	output.Stderr = &stderr
	output.Stdout = &bytes.Buffer{}
	t.Cleanup(func() {
		output.Stderr = prevErr
		output.Stdout = prevOut
	})

	prevExit := processExit
	var gotCode int
	processExit = func(code int) {
		gotCode = code
		panic(exitPanic{code: code})
	}
	t.Cleanup(func() { processExit = prevExit })

	rootCmd.SetArgs([]string{"search", "packages"})
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
	if gotCode != 1 {
		t.Fatalf("exit=%d stderr=%s", gotCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "packages") {
		t.Fatalf("stderr=%s", stderr.String())
	}
}

func TestUnifiedSearchCasesQueryRejected(t *testing.T) {
	t.Setenv(config.EnvAccessToken, "test-token-unified-search-case-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-case")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv("YUNXIAO_PROFILE", "")

	var stderr bytes.Buffer
	prevErr := output.Stderr
	prevOut := output.Stdout
	output.Stderr = &stderr
	output.Stdout = &bytes.Buffer{}
	t.Cleanup(func() {
		output.Stderr = prevErr
		output.Stdout = prevOut
	})

	prevExit := processExit
	var gotCode int
	processExit = func(code int) {
		gotCode = code
		panic(exitPanic{code: code})
	}
	t.Cleanup(func() { processExit = prevExit })

	rootCmd.SetArgs([]string{"search", "cases", "--q", "login", "--repo-id", "r1"})
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
	if gotCode != 1 {
		t.Fatalf("exit=%d stderr=%s", gotCode, stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "--q") && !strings.Contains(out, "query") {
		t.Fatalf("stderr=%s", out)
	}
}


func TestUnifiedSearchTypeHelpTableMatchesRoutes(t *testing.T) {
	table := unifiedSearchTypeHelpTable()
	for _, r := range unifiedSearchRoutes() {
		if !strings.Contains(table, r.Canonical) {
			t.Fatalf("help missing canonical %q:\n%s", r.Canonical, table)
		}
		if !strings.Contains(table, r.TargetPath) {
			t.Fatalf("help missing path %q:\n%s", r.TargetPath, table)
		}
		for _, a := range r.Aliases {
			if !strings.Contains(table, a) {
				t.Fatalf("help missing alias %q:\n%s", a, table)
			}
		}
		if r.QueryFlag != "" {
			want := "--" + r.QueryFlag
			if !strings.Contains(table, want) {
				t.Fatalf("help missing query flag %q for %s:\n%s", want, r.Canonical, table)
			}
		} else if !strings.Contains(table, "no keyword") {
			t.Fatalf("help missing no-keyword note:\n%s", table)
		}
	}
}

func TestUnifiedSearchFormatJQApplied(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-unified-search-fmt-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-fmt")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevFmtGlobal := globalFormat
	prevJQGlobal := globalJQ
	globalFormat = "json"
	globalJQ = ""
	t.Cleanup(func() {
		globalFormat = prevFmtGlobal
		globalJQ = prevJQGlobal
	})

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupReposListCmd, "search")
	for _, name := range []string{"page", "per-page"} {
		f := codeupReposListCmd.Flags().Lookup(name)
		if f != nil {
			_ = codeupReposListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}

	// Flags after `search` must apply (Critical regression: previously only
	// updated global* while output.* stayed at PersistentPreRun snapshot).
	rootCmd.SetArgs([]string{
		"search", "repo",
		"--q", "demo",
		"--dry-run",
		"--format", "pretty",
		"--jq", ".ok",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	out := strings.TrimSpace(stdout.String())
	if out != "true" {
		t.Fatalf("jq .ok want true, got %q (format/jq not applied?)", out)
	}
	if output.Format != "pretty" {
		t.Fatalf("output.Format=%q want pretty", output.Format)
	}
	if output.JQ != ".ok" {
		t.Fatalf("output.JQ=%q want .ok", output.JQ)
	}
	if globalFormat != "pretty" || globalJQ != ".ok" {
		t.Fatalf("globals format=%q jq=%q", globalFormat, globalJQ)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}

	// Second pass: pretty without jq — indented envelope proves --format alone.
	stdout2 := withCmdJSONCapture(t)
	resetStringFlags(t, codeupReposListCmd, "search")
	globalFormat = "json"
	globalJQ = ""
	rootCmd.SetArgs([]string{"search", "repo", "--q", "demo", "--dry-run", "--format", "pretty"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("pretty execute: %v\n%s", err, stdout2.String())
	}
	pretty := stdout2.String()
	if !strings.Contains(pretty, "\n  \"ok\": true") && !strings.Contains(pretty, "\n  \"ok\":true") {
		// json.Encoder with indent uses "  \"ok\": true"
		if !strings.Contains(pretty, "{\n") || !strings.Contains(pretty, "\n}") {
			t.Fatalf("expected pretty indented JSON, got %q", pretty)
		}
		if !strings.Contains(pretty, "\"ok\"") {
			t.Fatalf("expected ok field in pretty output: %q", pretty)
		}
	}
	if output.Format != "pretty" {
		t.Fatalf("pretty pass output.Format=%q", output.Format)
	}
	if output.JQ != "" {
		t.Fatalf("pretty pass output.JQ=%q want empty", output.JQ)
	}
}

func TestUnifiedSearchMRQueryDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-unified-search-mr-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-mr")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsListCmd, "repo", "search", "state", "source", "target", "sort", "status", "all", "page", "per-page")
	rootCmd.SetArgs([]string{"search", "mr", "--q", "fix-title", "--state", "opened", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/changeRequests") || !strings.Contains(url, "search=fix-title") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "state=opened") {
		t.Fatalf("state missing url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestUnifiedSearchPipelineQueryDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-unified-search-pipe-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-pipe")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, pipelineListCmd, "name", "status-list")
	for _, name := range []string{"page", "per-page"} {
		f := pipelineListCmd.Flags().Lookup(name)
		if f != nil {
			_ = pipelineListCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	if f := pipelineListCmd.Flags().Lookup("all"); f != nil {
		_ = pipelineListCmd.Flags().Set("all", "false")
		f.Changed = false
	}
	rootCmd.SetArgs([]string{"search", "pipeline", "--q", "deploy-bot", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/pipelines") || !strings.Contains(url, "pipelineName=deploy-bot") {
		t.Fatalf("url=%q", url)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestUnifiedSearchAppTagQueryDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-unified-search-tag-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-tag")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackTagsSearchCmd, "search", "order-by", "sort")
	for _, name := range []string{"current", "page-size"} {
		f := appstackTagsSearchCmd.Flags().Lookup(name)
		if f != nil {
			_ = appstackTagsSearchCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs([]string{"search", "app-tag", "--q", "prod-tag", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/appTags:search") {
		t.Fatalf("url=%q", url)
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["search"] != "prod-tag" {
		t.Fatalf("body=%v", body)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestUnifiedSearchUnknownDomainFlagExits(t *testing.T) {
	t.Setenv(config.EnvAccessToken, "test-token-unified-search-badflag-not-real")
	t.Setenv(config.EnvOrganizationID, "org-unified-search-badflag")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv("YUNXIAO_PROFILE", "")

	var stderr bytes.Buffer
	prevErr := output.Stderr
	prevOut := output.Stdout
	output.Stderr = &stderr
	output.Stdout = &bytes.Buffer{}
	t.Cleanup(func() {
		output.Stderr = prevErr
		output.Stdout = prevOut
	})

	prevExit := processExit
	var gotCode int
	processExit = func(code int) {
		gotCode = code
		panic(exitPanic{code: code})
	}
	t.Cleanup(func() { processExit = prevExit })

	rootCmd.SetArgs([]string{"search", "repo", "--not-a-real-flag", "x"})
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
	if gotCode != 1 {
		t.Fatalf("exit=%d stderr=%s", gotCode, stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "not-a-real-flag") {
		t.Fatalf("stderr missing flag name: %s", out)
	}
}
