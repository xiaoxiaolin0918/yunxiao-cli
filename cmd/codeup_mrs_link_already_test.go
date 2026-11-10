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

func mrsLinkTestServer(t *testing.T, relations any, onMutate func(method, path string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onMutate != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete) {
			onMutate(r.Method, r.URL.Path)
		}
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "extRelationRecords"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wi-internal-1", "serialNumber": "ZYPT-1"})
		case strings.Contains(r.URL.Path, "/changeRequests/") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"localId":      125,
				"title":        "feat: x",
				"sourceBranch": "feat/x",
				"targetBranch": "master",
				"detailUrl":    "https://codeup.aliyun.com/sanzhi/zhiyi/mgy/iipmes_gy/change/125",
				"projectId":    "4951320",
				"bizId":        "mr-biz-125",
			})
		case strings.Contains(r.URL.Path, "extRelationRecords") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(relations)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"unexpected ` + r.Method + ` ` + r.URL.Path + `"}`))
		}
	}))
}

func executeMrsLink(t *testing.T, extraArgs ...string) *output.Envelope {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsLinkCmd, "repo", "local-id", "work-item")
	args := append([]string{
		"codeup", "mrs", "link",
		"--repo", "4951320",
		"--local-id", "125",
		"--work-item", "ZYPT-1",
	}, extraArgs...)
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.Bytes())
	}
	if !env.OK {
		t.Fatalf("envelope: %+v", env)
	}
	return &env
}

func TestMrsLinkDryRunAlreadyLinkedViaBusinessId(t *testing.T) {
	var mutated []string
	srv := mrsLinkTestServer(t, []any{
		map[string]any{
			"businessId":       "125",
			"projectId":        "4951320",
			"category":         "codeupMergeRequest",
			"relationRecordId": "rr-1",
		},
	}, func(method, path string) { mutated = append(mutated, method+" "+path) })
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-link-already-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-link-already")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	env := executeMrsLink(t, "--dry-run")
	if !env.DryRun {
		t.Fatalf("expected dry-run: %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if toLink, ok := req["to_link"]; ok {
		t.Fatalf("already-linked WI must not appear in to_link: %v", toLink)
	}
	body, _ := req["body"].(map[string]any)
	if body["alreadyLinked"] != true {
		t.Fatalf("expected alreadyLinked in body, req=%v", req)
	}
	if len(mutated) != 0 {
		t.Fatalf("mutating calls during dry-run: %v", mutated)
	}
}

func TestMrsLinkDryRunAlreadyLinkedViaBizIdAndURL(t *testing.T) {
	srv := mrsLinkTestServer(t, []any{
		map[string]any{
			"businessId": "mr-biz-125",
			"projectId":  float64(4951320),
			"url":        "https://codeup.aliyun.com/sanzhi/zhiyi/mgy/iipmes_gy/change/125",
			"category":   "codeupMergeRequest",
		},
	}, nil)
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-link-bizurl-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-link-bizurl")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	env := executeMrsLink(t, "--dry-run")
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if toLink, ok := req["to_link"]; ok {
		t.Fatalf("already-linked WI must not appear in to_link: %v", toLink)
	}
	body, _ := req["body"].(map[string]any)
	if body["alreadyLinked"] != true {
		t.Fatalf("expected alreadyLinked, req=%v", req)
	}
}

func TestMrsLinkDryRunUnlinkedStillToLink(t *testing.T) {
	srv := mrsLinkTestServer(t, []any{
		map[string]any{
			"businessId": "3142",
			"projectId":  "4951320",
			"url":        "https://codeup.aliyun.com/sanzhi/zhiyi/mgy/iipmes_gy/change/3142",
			"category":   "codeupMergeRequest",
		},
	}, nil)
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-link-other-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-link-other")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	env := executeMrsLink(t, "--dry-run")
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	toLink, _ := req["to_link"].([]any)
	if len(toLink) != 1 || toLink[0] != "wi-internal-1" {
		t.Fatalf("expected unlinked WI in to_link, req=%v", req)
	}
}

func TestMrsLinkAlreadyLinkedDoesNotCreate(t *testing.T) {
	var mutated []string
	srv := mrsLinkTestServer(t, []any{
		map[string]any{
			"businessId": "mr-biz-125",
			"url":        "https://codeup.aliyun.com/sanzhi/zhiyi/mgy/iipmes_gy/change/125",
			"projectId":  float64(4951320),
			"category":   "codeupMergeRequest",
		},
	}, func(method, path string) { mutated = append(mutated, method+" "+path) })
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-link-nopost-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-link-nopost")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevDry := globalDryRun
	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	t.Cleanup(func() { globalDryRun = prevDry })
	resetStringFlags(t, codeupMrsLinkCmd, "repo", "local-id", "work-item")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "link",
		"--repo", "4951320",
		"--local-id", "125",
		"--work-item", "ZYPT-1",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	raw, _ := json.Marshal(env.Data)
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	if data["alreadyLinked"] != true {
		t.Fatalf("data=%v", data)
	}
	if len(mutated) != 0 {
		t.Fatalf("already-linked link must not POST: %v", mutated)
	}
}
