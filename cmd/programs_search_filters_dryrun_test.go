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

func TestProgramsSearchFiltersDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-programs-filters-not-real")
	t.Setenv(config.EnvOrganizationID, "org-programs-filters")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, programsSearchCmd,
		"name", "status", "gmt-create-start", "gmt-create-end",
		"creator", "users", "order-by", "sort")
	for _, name := range []string{"page", "per-page"} {
		f := programsSearchCmd.Flags().Lookup(name)
		if f != nil {
			_ = programsSearchCmd.Flags().Set(name, f.DefValue)
			f.Changed = false
		}
	}
	rootCmd.SetArgs([]string{
		"programs", "search",
		"--name", "projex",
		"--status", "1,2",
		"--creator", "u-1",
		"--users", "u-2,u-3",
		"--order-by", "name",
		"--sort", "asc",
		"--page", "2",
		"--per-page", "50",
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
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/programs:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	if body["name"] != "projex" || body["status"] != "1,2" || body["creator"] != "u-1" || body["users"] != "u-2,u-3" {
		t.Fatalf("filters=%v", body)
	}
	if body["orderBy"] != "name" || body["sort"] != "asc" {
		t.Fatalf("order=%v", body)
	}
	// JSON numbers may decode as float64.
	page, _ := body["page"].(float64)
	perPage, _ := body["perPage"].(float64)
	if page != 2 || perPage != 50 {
		t.Fatalf("page=%v perPage=%v", body["page"], body["perPage"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}