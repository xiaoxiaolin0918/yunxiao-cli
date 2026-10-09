package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

// Asserts --labels lands in conditions as tag/multiList CONTAINS (#141).
func TestWorkitemSearchLabelsDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-search-labels-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-search-labels")
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
		"workitem", "search",
		"--space-id", "space-labels-1",
		"--category", "Req",
		"--labels", "lbl-a,lbl-b",
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
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/workitems:search") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil || body["spaceId"] != "space-labels-1" || body["category"] != "Req" {
		t.Fatalf("body=%v", body)
	}
	cond, _ := body["conditions"].(string)
	if !strings.Contains(cond, `"fieldIdentifier":"tag"`) && !strings.Contains(cond, `"fieldIdentifier": "tag"`) {
		// marshaled without spaces
		if !strings.Contains(cond, "fieldIdentifier") || !strings.Contains(cond, "tag") {
			t.Fatalf("conditions missing tag filter: %q", cond)
		}
	}
	if !strings.Contains(cond, "lbl-a") || !strings.Contains(cond, "lbl-b") || !strings.Contains(cond, "CONTAINS") {
		t.Fatalf("conditions missing label ids: %q", cond)
	}
	if !strings.Contains(cond, "multiList") {
		t.Fatalf("conditions want multiList: %q", cond)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}