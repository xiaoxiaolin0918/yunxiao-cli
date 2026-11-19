package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func TestTesthubCasesGetMetaURLUsesCustomCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","customCode":"HEGL-1","subject":"s"}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-get-url-415-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-get-url-415")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	resetStringFlags(t, testhubCasesGetCmd, "repo-id", "id")
	rootCmd.SetArgs([]string{"testhub", "cases", "get", "--repo-id", "r1", "--id", "c1"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	want := "https://devops.aliyun.com/testhub/case/HEGL-1"
	if env.Meta["url"] != want {
		t.Fatalf("meta.url=%v want %s raw=%s", env.Meta["url"], want, stdout.String())
	}
	if env.Meta["webPathHint"] != nil {
		t.Fatalf("webPathHint should be absent: %v", env.Meta)
	}
	raw, _ := json.Marshal(env.Data)
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	if data["url"] != want {
		t.Fatalf("data.url=%v", data["url"])
	}
	if strings.Contains(stdout.String(), "/projex/") {
		t.Fatalf("must not emit Projex testcase URL: %s", stdout.String())
	}
}

func TestTesthubCasesGetMetaURLFallbackWithoutCustomCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","subject":"s"}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-get-url-fb-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-get-url-fb")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	resetStringFlags(t, testhubCasesGetCmd, "repo-id", "id")
	rootCmd.SetArgs([]string{"testhub", "cases", "get", "--repo-id", "r1", "--id", "c1"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	want := "https://devops.aliyun.com/testhub/repo/r1/case"
	if env.Meta["url"] != want {
		t.Fatalf("meta.url=%v want %s raw=%s", env.Meta["url"], want, stdout.String())
	}
}

func TestTesthubDirectoriesListURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"e82550461056cea35168705b02","name":"dir"}]`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-dirs-url-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-dirs-url")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	resetStringFlags(t, testhubDirectoriesListCmd, "repo-id")
	rootCmd.SetArgs([]string{"testhub", "directories", "list", "--repo-id", "627d7f45b45be0ece02f93d246"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	raw, _ := json.Marshal(env.Data)
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("data=%s", raw)
	}
	wantItem := "https://devops.aliyun.com/testhub/repo/627d7f45b45be0ece02f93d246/case#selectedTreeIdentifier=e82550461056cea35168705b02"
	if len(items) != 1 || items[0]["url"] != wantItem {
		t.Fatalf("items=%v", items)
	}
	wantMeta := "https://devops.aliyun.com/testhub/repo/627d7f45b45be0ece02f93d246/case"
	if env.Meta["url"] != wantMeta {
		t.Fatalf("meta.url=%v", env.Meta["url"])
	}
	if strings.Contains(stdout.String(), "/projex/") {
		t.Fatalf("must not emit Projex testcase URL: %s", stdout.String())
	}
}

func TestTesthubReposListURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"testRepoIdentifier":"repo-abc","name":"lib"}]`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-repos-url-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-repos-url")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	resetStringFlags(t, testhubReposListCmd, "name")
	rootCmd.SetArgs([]string{"testhub", "repos", "list"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	raw, _ := json.Marshal(env.Data)
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("data=%s", raw)
	}
	want := "https://devops.aliyun.com/testhub/repo/repo-abc/case"
	if len(items) != 1 || items[0]["url"] != want || items[0]["id"] != "repo-abc" {
		t.Fatalf("items=%v", items)
	}
	if strings.Contains(stdout.String(), "/projex/") {
		t.Fatalf("must not emit Projex testcase URL: %s", stdout.String())
	}
}

func TestTesthubCasesCreateRefreshURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/testcases"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"case-new-1"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/testcases/case-new-1"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"case-new-1","customCode":"HEGL-9999","subject":"refreshed"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-create-url-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-create-url")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	globalYes = true
	resetStringFlags(t, testhubCasesCreateCmd, "repo-id", "subject", "directory-id", "assigned-to", "data", "data-file")
	rootCmd.SetArgs([]string{
		"testhub", "cases", "create",
		"--repo-id", "repo-r",
		"--subject", "title",
		"--assigned-to", "user-1",
		"--yes",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	want := "https://devops.aliyun.com/testhub/case/HEGL-9999"
	if env.Meta["url"] != want {
		t.Fatalf("meta.url=%v meta=%v raw=%s", env.Meta["url"], env.Meta, stdout.String())
	}
	raw, _ := json.Marshal(env.Data)
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	if data["url"] != want {
		t.Fatalf("data.url=%v", data["url"])
	}
}
