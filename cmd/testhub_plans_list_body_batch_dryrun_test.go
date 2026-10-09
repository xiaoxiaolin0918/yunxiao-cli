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

func thPlansBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-th-plans-body-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-th-plans-body-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, testhubPlansListCmd, "project-id", "sprint-id", "name", "status", "page", "per-page")
	return stdout, hits
}

func thPlansBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any)) {
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
	if !strings.Contains(url, "/testPlan/list") {
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
		check(t, body)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func thPlansNum(t *testing.T, v any) float64 {
	t.Helper()
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("not number: %#v", v)
	}
	return n
}

func TestTesthubPlansListDefaultsBodyDryRun(t *testing.T) {
	stdout, hits := thPlansBodySetup(t, "defaults")
	rootCmd.SetArgs([]string{"testhub", "plans", "list", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlansBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if thPlansNum(t, body["page"]) != 1 || thPlansNum(t, body["perPage"]) != 100 {
			t.Fatalf("body=%v", body)
		}
		for _, k := range []string{"projectIdentifier", "sprintIdentifier", "name", "status"} {
			if _, has := body[k]; has {
				t.Fatalf("%s must be omitted: %#v", k, body)
			}
		}
	})
}

func TestTesthubPlansListProjectIdBodyDryRun(t *testing.T) {
	stdout, hits := thPlansBodySetup(t, "project")
	rootCmd.SetArgs([]string{"testhub", "plans", "list", "--project-id", "proj-batch", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlansBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["projectIdentifier"] != "proj-batch" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["sprintIdentifier"]; has {
			t.Fatalf("sprint must be omitted: %#v", body)
		}
	})
}

func TestTesthubPlansListSprintIdBodyDryRun(t *testing.T) {
	stdout, hits := thPlansBodySetup(t, "sprint")
	rootCmd.SetArgs([]string{"testhub", "plans", "list", "--sprint-id", "sp-7", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlansBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["sprintIdentifier"] != "sp-7" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubPlansListNameBodyDryRun(t *testing.T) {
	stdout, hits := thPlansBodySetup(t, "name")
	rootCmd.SetArgs([]string{"testhub", "plans", "list", "--name", "smoke", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlansBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["name"] != "smoke" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubPlansListStatusBodyDryRun(t *testing.T) {
	stdout, hits := thPlansBodySetup(t, "status")
	rootCmd.SetArgs([]string{"testhub", "plans", "list", "--status", "DOING", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlansBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["status"] != "DOING" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubPlansListPageBodyDryRun(t *testing.T) {
	stdout, hits := thPlansBodySetup(t, "page")
	rootCmd.SetArgs([]string{"testhub", "plans", "list", "--page", "3", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlansBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if thPlansNum(t, body["page"]) != 3 || thPlansNum(t, body["perPage"]) != 100 {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubPlansListPerPageBodyDryRun(t *testing.T) {
	stdout, hits := thPlansBodySetup(t, "perpage")
	rootCmd.SetArgs([]string{"testhub", "plans", "list", "--per-page", "40", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlansBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if thPlansNum(t, body["page"]) != 1 || thPlansNum(t, body["perPage"]) != 40 {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestTesthubPlansListAllFiltersBodyDryRun(t *testing.T) {
	stdout, hits := thPlansBodySetup(t, "all")
	rootCmd.SetArgs([]string{
		"testhub", "plans", "list",
		"--project-id", "proj-all",
		"--sprint-id", "sp-all",
		"--name", "full",
		"--status", "TODO,DONE",
		"--page", "2",
		"--per-page", "25",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	thPlansBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["projectIdentifier"] != "proj-all" || body["sprintIdentifier"] != "sp-all" ||
			body["name"] != "full" || body["status"] != "TODO,DONE" {
			t.Fatalf("body=%v", body)
		}
		if thPlansNum(t, body["page"]) != 2 || thPlansNum(t, body["perPage"]) != 25 {
			t.Fatalf("body=%v", body)
		}
	})
}
