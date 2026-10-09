package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func asTagsSearchSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-as-tags-q-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-as-tags-q-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, appstackTagsSearchCmd, "search", "order-by", "sort")
	for _, name := range []string{"current", "page-size"} {
		f := appstackTagsSearchCmd.Flags().Lookup(name)
		if f != nil {
			_ = appstackTagsSearchCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	return stdout, hits
}

func asTagsSearchAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any, q url.Values, rawURL string)) {
	t.Helper()
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
	rawURL, _ := req["url"].(string)
	if !strings.Contains(rawURL, "/appTags:search") {
		t.Fatalf("url=%q", rawURL)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if check != nil {
		check(t, body, u.Query(), rawURL)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestAppstackTagsSearchOrderByDryRun(t *testing.T) {
	stdout, hits := asTagsSearchSetup(t, "order")
	rootCmd.SetArgs([]string{
		"appstack", "tags", "search",
		"--order-by", "name",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asTagsSearchAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ url.Values, _ string) {
		if body == nil || body["orderBy"] != "name" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["search"]; has {
			t.Fatalf("search must be omitted: %#v", body)
		}
		if _, has := body["sort"]; has {
			t.Fatalf("sort must be omitted: %#v", body)
		}
	})
}

func TestAppstackTagsSearchSortAscDryRun(t *testing.T) {
	stdout, hits := asTagsSearchSetup(t, "sort-asc")
	rootCmd.SetArgs([]string{
		"appstack", "tags", "search",
		"--sort", "asc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asTagsSearchAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ url.Values, _ string) {
		if body == nil || body["sort"] != "asc" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackTagsSearchSortDescDryRun(t *testing.T) {
	stdout, hits := asTagsSearchSetup(t, "sort-desc")
	rootCmd.SetArgs([]string{
		"appstack", "tags", "search",
		"--sort", "desc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asTagsSearchAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ url.Values, _ string) {
		if body == nil || body["sort"] != "desc" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackTagsSearchPageQueryDryRun(t *testing.T) {
	stdout, hits := asTagsSearchSetup(t, "page")
	rootCmd.SetArgs([]string{
		"appstack", "tags", "search",
		"--current", "2",
		"--page-size", "30",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asTagsSearchAssert(t, stdout, hits, func(t *testing.T, body map[string]any, q url.Values, rawURL string) {
		if q.Get("current") != "2" || q.Get("pageSize") != "30" {
			t.Fatalf("q=%v url=%q", q, rawURL)
		}
		if body != nil && len(body) > 0 {
			t.Fatalf("body should be empty/nil: %#v", body)
		}
	})
}

func TestAppstackTagsSearchEmptyBodyDryRun(t *testing.T) {
	stdout, hits := asTagsSearchSetup(t, "empty")
	rootCmd.SetArgs([]string{
		"appstack", "tags", "search",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asTagsSearchAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ url.Values, _ string) {
		if body != nil && len(body) > 0 {
			t.Fatalf("expected empty body, got %#v", body)
		}
	})
}

func TestAppstackTagsSearchKeywordAndOrderDryRun(t *testing.T) {
	stdout, hits := asTagsSearchSetup(t, "kw-order")
	rootCmd.SetArgs([]string{
		"appstack", "tags", "search",
		"--search", "staging",
		"--order-by", "gmtCreate",
		"--sort", "desc",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asTagsSearchAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ url.Values, _ string) {
		if body["search"] != "staging" || body["orderBy"] != "gmtCreate" || body["sort"] != "desc" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackTagsSearchEmptySearchOmitsDryRun(t *testing.T) {
	stdout, hits := asTagsSearchSetup(t, "empty-search")
	rootCmd.SetArgs([]string{
		"appstack", "tags", "search",
		"--search", "",
		"--order-by", "name",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asTagsSearchAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ url.Values, _ string) {
		if _, has := body["search"]; has {
			t.Fatalf("empty search must omit: %#v", body)
		}
		if body["orderBy"] != "name" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAppstackTagsSearchFullQueryDryRun(t *testing.T) {
	stdout, hits := asTagsSearchSetup(t, "full")
	rootCmd.SetArgs([]string{
		"appstack", "tags", "search",
		"--search", "batch",
		"--order-by", "name",
		"--sort", "asc",
		"--current", "4",
		"--page-size", "12",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	asTagsSearchAssert(t, stdout, hits, func(t *testing.T, body map[string]any, q url.Values, rawURL string) {
		if body["search"] != "batch" || body["orderBy"] != "name" || body["sort"] != "asc" {
			t.Fatalf("body=%v", body)
		}
		if q.Get("current") != "4" || q.Get("pageSize") != "12" {
			t.Fatalf("q=%v url=%q", q, rawURL)
		}
	})
}