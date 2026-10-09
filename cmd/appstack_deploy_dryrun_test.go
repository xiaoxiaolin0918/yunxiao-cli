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

func TestAppstackDeployHelpSurface(t *testing.T) {
	h := appstackDeployCmd.Long
	if !strings.Contains(h, "machine-log") {
		t.Fatalf("deploy Long should mention machine-log: %s", h)
	}
	if !strings.Contains(h, "high-risk-write") {
		t.Fatalf("deploy Long should mention high-risk-write: %s", h)
	}
	var hasLog, hasAdd bool
	for _, c := range appstackDeployCmd.Commands() {
		switch c.Name() {
		case "machine-log":
			hasLog = true
		case "add-hosts":
			hasAdd = true
		}
	}
	if !hasLog || !hasAdd {
		t.Fatalf("expected machine-log+add-hosts, got log=%v add=%v", hasLog, hasAdd)
	}
}

func TestAppstackDeployMachineLogDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-machine-log-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-machine-log")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackDeployMachineLogCmd, "machine-sn")
	_ = appstackDeployMachineLogCmd.Flags().Set("tunnel-id", "0")
	f := appstackDeployMachineLogCmd.Flags().Lookup("tunnel-id")
	if f != nil {
		f.Changed = false
	}
	rootCmd.SetArgs([]string{
		"appstack", "deploy", "machine-log",
		"--tunnel-id", "42",
		"--machine-sn", "i-abc",
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
	if !strings.Contains(url, "/host/deployLog") {
		t.Fatalf("url=%q", url)
	}
	if !strings.Contains(url, "tunnelId=42") || !strings.Contains(url, "machineSn=i-abc") {
		t.Fatalf("query missing url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestAppstackDeployAddHostsDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-add-hosts-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-add-hosts")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackDeployAddHostsCmd, "instance", "host-sns")
	rootCmd.SetArgs([]string{
		"appstack", "deploy", "add-hosts",
		"--instance", "pool-1",
		"--host-sns", "sn-1,sn-2",
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
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/pools/instances/pool-1/addHostList") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	hosts, _ := body["hostSns"].([]any)
	if len(hosts) != 2 || hosts[0] != "sn-1" || hosts[1] != "sn-2" {
		t.Fatalf("body=%v", body)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestAppstackDeployAddHostsToGroupDryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-appstack-add-hosts-group-not-real")
	t.Setenv(config.EnvOrganizationID, "org-appstack-add-hosts-group")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, appstackDeployAddHostsGroupCmd, "instance", "group", "host-sns")
	rootCmd.SetArgs([]string{
		"appstack", "deploy", "add-hosts-to-group",
		"--instance", "pool-2",
		"--group", "g-a",
		"--host-sns", "sn-9",
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
	if env.Risk != string(risk.HighRiskWrite) && env.Risk != "high-risk-write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/pools/instances/pool-2/deployGroup/g-a/addHostList") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}