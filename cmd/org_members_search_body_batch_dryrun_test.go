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

func orgMSBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-org-ms-body-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-ms-body-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, orgMembersSearchCmd, "query", "page", "per-page", "include-aliyun-uid")
	return stdout, hits
}

func orgMSBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any, req map[string]any)) {
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
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/members:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	if check != nil {
		check(t, body, req)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func orgMSNum(t *testing.T, v any) float64 {
	t.Helper()
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("not number: %#v", v)
	}
	return n
}

func TestOrgMembersSearchDefaultsDryRun(t *testing.T) {
	stdout, hits := orgMSBodySetup(t, "defaults")
	rootCmd.SetArgs([]string{"organization", "members", "search", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	orgMSBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if orgMSNum(t, body["page"]) != 1 || orgMSNum(t, body["perPage"]) != 100 {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["query"]; has {
			t.Fatalf("query must be omitted: %#v", body)
		}
	})
}

func TestOrgMembersSearchQueryDryRun(t *testing.T) {
	stdout, hits := orgMSBodySetup(t, "query")
	rootCmd.SetArgs([]string{"organization", "members", "search", "--query", "alice", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	orgMSBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["query"] != "alice" {
			t.Fatalf("body=%v", body)
		}
		if orgMSNum(t, body["page"]) != 1 || orgMSNum(t, body["perPage"]) != 100 {
			t.Fatalf("defaults changed: body=%v", body)
		}
	})
}

func TestOrgMembersSearchPageDryRun(t *testing.T) {
	stdout, hits := orgMSBodySetup(t, "page")
	rootCmd.SetArgs([]string{"organization", "members", "search", "--page", "4", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	orgMSBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if orgMSNum(t, body["page"]) != 4 || orgMSNum(t, body["perPage"]) != 100 {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["query"]; has {
			t.Fatalf("query must be omitted: %#v", body)
		}
	})
}

func TestOrgMembersSearchPerPageDryRun(t *testing.T) {
	stdout, hits := orgMSBodySetup(t, "perpage")
	rootCmd.SetArgs([]string{"organization", "members", "search", "--per-page", "25", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	orgMSBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if orgMSNum(t, body["page"]) != 1 || orgMSNum(t, body["perPage"]) != 25 {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestOrgMembersSearchQueryAndPageDryRun(t *testing.T) {
	stdout, hits := orgMSBodySetup(t, "q-page")
	rootCmd.SetArgs([]string{"organization", "members", "search", "--query", "bob", "--page", "2", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	orgMSBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["query"] != "bob" || orgMSNum(t, body["page"]) != 2 || orgMSNum(t, body["perPage"]) != 100 {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestOrgMembersSearchQueryAndPerPageDryRun(t *testing.T) {
	stdout, hits := orgMSBodySetup(t, "q-pp")
	rootCmd.SetArgs([]string{"organization", "members", "search", "--query", "carol", "--per-page", "50", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	orgMSBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["query"] != "carol" || orgMSNum(t, body["page"]) != 1 || orgMSNum(t, body["perPage"]) != 50 {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestOrgMembersSearchPageAndPerPageDryRun(t *testing.T) {
	stdout, hits := orgMSBodySetup(t, "page-pp")
	rootCmd.SetArgs([]string{"organization", "members", "search", "--page", "3", "--per-page", "40", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	orgMSBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if orgMSNum(t, body["page"]) != 3 || orgMSNum(t, body["perPage"]) != 40 {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["query"]; has {
			t.Fatalf("query must be omitted: %#v", body)
		}
	})
}

func TestOrgMembersSearchAllFieldsDryRun(t *testing.T) {
	stdout, hits := orgMSBodySetup(t, "all")
	rootCmd.SetArgs([]string{
		"organization", "members", "search",
		"--query", "dave",
		"--page", "5",
		"--per-page", "15",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	orgMSBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["query"] != "dave" || orgMSNum(t, body["page"]) != 5 || orgMSNum(t, body["perPage"]) != 15 {
			t.Fatalf("body=%v", body)
		}
	})
}
