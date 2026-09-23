package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

func TestTransitionDryRunEdgeValidationHelper(t *testing.T) {
	st, warn := transitionDryRunEdgeValidation("direct_status", nil)
	if st != "skipped" {
		t.Fatalf("direct_status status=%q", st)
	}
	if warn == "" || !strings.Contains(warn, "未校验") {
		t.Fatalf("warning=%q", warn)
	}

	st, warn = transitionDryRunEdgeValidation("profile_graph", map[string][]string{})
	if st != "skipped" || warn == "" {
		t.Fatalf("empty edges: %q %q", st, warn)
	}

	st, warn = transitionDryRunEdgeValidation("profile_graph", map[string][]string{"a": {"b"}})
	if st != "validated" || warn != "" {
		t.Fatalf("with edges: %q %q", st, warn)
	}
}

func writeTransitionTestProfile(t *testing.T, dir string, pf *profile.Profile) {
	t.Helper()
	profDir := filepath.Join(dir, "yunxiao", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(profDir, pf.Name+".json")
	if err := profile.SaveFile(path, pf); err != nil {
		t.Fatal(err)
	}
}

func TestTransitionDryRunSkipsEdgeValidationWithoutCachedEdges(t *testing.T) {
	// Repro shape for #59: no workflows edges → API statuses fallback → dry-run must
	// still note that current→target edge was NOT validated.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "wi-5470",
				"serialNumber": "ZYPT-5470",
				"spaceId":      "space-1",
				"categoryId":   "Req",
				"workitemTypeId": "type-req-1",
				"status":       map[string]any{"id": "st-deploy-prod", "displayName": "待部署生产"},
			})
		case strings.Contains(r.URL.Path, "/workflows") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "wf-1",
				"name": "req",
				"statuses": []any{
					map[string]any{"id": "st-deploy-prod", "displayName": "待部署生产", "name": "待部署生产"},
					map[string]any{"id": "st-done", "displayName": "已完成", "name": "已完成"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"unexpected ` + r.Method + ` ` + r.URL.Path + `"}`))
		}
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "t59",
		OrganizationID: "org-59",
		SpaceID:        "space-1",
		// no Workflows / bug edges → ResolveWorkflow fails → direct_status
	})

	t.Setenv(config.EnvAccessToken, "test-token-transition-59-not-real")
	t.Setenv(config.EnvOrganizationID, "org-59")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t59")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t59"
	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields")
	rootCmd.SetArgs([]string{
		"workitem", "+transition",
		"--id", "ZYPT-5470",
		"--to", "已完成",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}

	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("envelope: %+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req["transition_mode"] != "direct_status" {
		t.Fatalf("transition_mode=%v req=%s", req["transition_mode"], raw)
	}
	if req["edge_validation"] != "skipped" {
		t.Fatalf("edge_validation=%v (want skipped); req=%s", req["edge_validation"], raw)
	}
	warn, _ := req["warning"].(string)
	if warn == "" || !strings.Contains(warn, "未校验") {
		t.Fatalf("warning=%q req=%s", warn, raw)
	}
	if req["target"] != "st-done" {
		t.Fatalf("target=%v", req["target"])
	}
}

func TestTransitionDryRunFailsIllegalEdgeWithCachedWorkflow(t *testing.T) {
	// With profile.workflows edges, illegal current→target must fail dry-run (ok:false).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "wi-5470",
				"serialNumber":   "ZYPT-5470",
				"spaceId":        "space-1",
				"categoryId":     "Req",
				"workitemTypeId": "type-req-1",
				"status":         map[string]any{"id": "st-deploy-prod", "displayName": "待部署生产"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"unexpected"}`))
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "t59g",
		OrganizationID: "org-59",
		SpaceID:        "space-1",
		Workflows: map[string]profile.WorkitemWorkflow{
			"type-req-1": {
				Name:     "产品类需求",
				Category: "Req",
				Statuses: map[string]string{
					"待部署生产": "st-deploy-prod",
					"已完成":   "st-done",
					"处理中":   "st-doing",
				},
				// No path deploy-prod → done (direct or multi-hop).
				Edges: map[string][]string{
					"st-deploy-prod": {"st-doing"},
					"st-doing":       {"st-deploy-prod"},
					"st-done":        {},
				},
			},
		},
	})

	t.Setenv(config.EnvAccessToken, "test-token-transition-59-not-real")
	t.Setenv(config.EnvOrganizationID, "org-59")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t59g")

	var stderr bytes.Buffer
	prevErr := output.Stderr
	prevOut := output.Stdout
	prevJQ := output.JQ
	prevFmt := output.Format
	output.Stderr = &stderr
	output.Stdout = &bytes.Buffer{}
	output.JQ = ""
	output.Format = "json"
	t.Cleanup(func() {
		output.Stderr = prevErr
		output.Stdout = prevOut
		output.JQ = prevJQ
		output.Format = prevFmt
	})

	prevYes := globalYes
	prevDry := globalDryRun
	prevProfile := globalProfile
	prevOrg := globalOrg
	globalYes = false
	globalDryRun = true
	globalProfile = "t59g"
	globalOrg = ""
	t.Cleanup(func() {
		globalYes = prevYes
		globalDryRun = prevDry
		globalProfile = prevProfile
		globalOrg = prevOrg
	})

	prevExit := processExit
	var gotCode int
	processExit = func(code int) {
		gotCode = code
		panic(exitPanic{code: code})
	}
	t.Cleanup(func() { processExit = prevExit })

	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields")
	rootCmd.SetArgs([]string{
		"workitem", "+transition",
		"--profile", "t59g",
		"--id", "ZYPT-5470",
		"--to", "已完成",
		"--dry-run",
	})
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
		t.Fatalf("exit code=%d stderr=%s", gotCode, stderr.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
		t.Fatalf("stderr JSON: %v / %s", err, stderr.Bytes())
	}
	if env.OK || env.Error == nil {
		t.Fatalf("want ok:false error envelope, got %+v", env)
	}
	if !strings.Contains(env.Error.Message, "无法流转") && !strings.Contains(env.Error.Message, "无实证边") {
		t.Fatalf("message=%q", env.Error.Message)
	}
}

func TestTransitionDryRunValidatedWhenEdgesPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "wi-1",
				"serialNumber":   "ZYPT-1",
				"spaceId":        "space-1",
				"categoryId":     "Req",
				"workitemTypeId": "type-req-1",
				"status":         map[string]any{"id": "st-doing", "displayName": "处理中"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "t59ok",
		OrganizationID: "org-59",
		SpaceID:        "space-1",
		Workflows: map[string]profile.WorkitemWorkflow{
			"type-req-1": {
				Category: "Req",
				Statuses: map[string]string{"处理中": "st-doing", "已完成": "st-done"},
				Edges:    map[string][]string{"st-doing": {"st-done"}, "st-done": {}},
			},
		},
	})

	t.Setenv(config.EnvAccessToken, "test-token-transition-59-not-real")
	t.Setenv(config.EnvOrganizationID, "org-59")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t59ok")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t59ok"
	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields")
	rootCmd.SetArgs([]string{
		"workitem", "+transition",
		"--id", "ZYPT-1",
		"--to", "已完成",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if req["edge_validation"] != "validated" {
		t.Fatalf("edge_validation=%v req=%s", req["edge_validation"], raw)
	}
	if _, ok := req["warning"]; ok {
		t.Fatalf("unexpected warning: %v", req["warning"])
	}
	if req["transition_mode"] != "profile_graph" {
		t.Fatalf("mode=%v", req["transition_mode"])
	}
}

func TestTransitionDryRunEdgeValidation_IllegalAndHinted(t *testing.T) {
	edges := map[string][]string{
		"st-pending": {"st-test"},
		"st-test":    {"st-dev-done"},
		"st-deploy":  {"st-regress"},
	}
	hinted := map[string][]string{
		"st-pending": {"st-cancel", "st-design", "st-test"},
		"st-dev-done": {"st-test"},
	}

	// Legal edge in edges → validated
	st, warn, illegal := transitionDryRunEdgeValidationFull("profile_graph", "st-pending", "st-test", edges, hinted)
	if st != "validated" || warn != "" || illegal {
		t.Fatalf("legal: %q %q illegal=%v", st, warn, illegal)
	}

	// Only in hinted → hinted/unverified, not validated, not illegal
	st, warn, illegal = transitionDryRunEdgeValidationFull("profile_graph", "st-pending", "st-design", edges, hinted)
	if st != "hinted" || illegal {
		t.Fatalf("hinted-only: %q warn=%q illegal=%v", st, warn, illegal)
	}
	if warn == "" {
		t.Fatal("hinted-only should warn")
	}

	// In neither → illegal
	st, warn, illegal = transitionDryRunEdgeValidationFull("profile_graph", "st-pending", "st-done", edges, hinted)
	if !illegal || st == "validated" {
		t.Fatalf("illegal: %q warn=%q illegal=%v", st, warn, illegal)
	}

	// Source not in edges keys (passthrough bug) + target unknown → illegal
	st, warn, illegal = transitionDryRunEdgeValidationFull("profile_graph", "st-regress", "st-done", edges, hinted)
	if !illegal {
		t.Fatalf("source-side-branch: %q warn=%q illegal=%v", st, warn, illegal)
	}
}

func TestTransitionDryRunIllegalEdgeReturnsOkFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "wi-75",
				"serialNumber":   "ZYPT-5866",
				"spaceId":        "space-1",
				"categoryId":     "Req",
				"workitemTypeId": "type-req-1",
				"status":         map[string]any{"id": "st-pending", "displayName": "待处理"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "t75",
		OrganizationID: "org-75",
		SpaceID:        "space-1",
		Workflows: map[string]profile.WorkitemWorkflow{
			"type-req-1": {
				Category: "Req",
				Statuses: map[string]string{
					"待处理": "st-pending", "待测试": "st-test", "设计中": "st-design", "已完成": "st-done",
				},
				Edges: map[string][]string{
					"st-pending": {"st-test"},
					"st-test":    {"st-dev-done"},
				},
				HintedEdges: map[string][]string{
					"st-pending": {"st-design", "st-test"},
				},
			},
		},
	})

	t.Setenv(config.EnvAccessToken, "test-token-transition-75-not-real")
	t.Setenv(config.EnvOrganizationID, "org-75")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t75")

	stderr := &bytes.Buffer{}
	prevErr := output.Stderr
	output.Stderr = stderr
	t.Cleanup(func() { output.Stderr = prevErr })

	prevExit := processExit
	var gotCode int
	processExit = func(code int) {
		gotCode = code
		panic(exitPanic{code: code})
	}
	t.Cleanup(func() { processExit = prevExit })

	globalProfile = "t75"
	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields")
	rootCmd.SetArgs([]string{
		"workitem", "+transition",
		"--id", "ZYPT-5866",
		"--to", "已完成",
		"--dry-run",
	})
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
	var env output.Envelope
	if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
		t.Fatalf("stderr JSON: %v / %s", err, stderr.Bytes())
	}
	if env.OK {
		t.Fatalf("want ok:false for illegal edge, got %+v", env)
	}
}

func TestTransitionDryRunHintedEdgeNotValidated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/workitems/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "wi-75h",
				"serialNumber":   "ZYPT-5866",
				"spaceId":        "space-1",
				"categoryId":     "Req",
				"workitemTypeId": "type-req-1",
				"status":         map[string]any{"id": "st-pending", "displayName": "待处理"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "t75h",
		OrganizationID: "org-75",
		SpaceID:        "space-1",
		Workflows: map[string]profile.WorkitemWorkflow{
			"type-req-1": {
				Category: "Req",
				Statuses: map[string]string{
					"待处理": "st-pending", "设计中": "st-design", "待测试": "st-test",
				},
				Edges: map[string][]string{
					"st-pending": {"st-test"},
				},
				HintedEdges: map[string][]string{
					"st-pending": {"st-design", "st-test"},
				},
			},
		},
	})

	t.Setenv(config.EnvAccessToken, "test-token-transition-75-not-real")
	t.Setenv(config.EnvOrganizationID, "org-75")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "t75h")

	stdout := withCmdJSONCapture(t)
	globalProfile = "t75h"
	resetStringFlags(t, workitemTransitionCmd, "id", "to", "type-id", "fields")
	rootCmd.SetArgs([]string{
		"workitem", "+transition",
		"--id", "ZYPT-5866",
		"--to", "设计中",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, stdout.String())
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if req["edge_validation"] != "hinted" {
		t.Fatalf("edge_validation=%v req=%s", req["edge_validation"], raw)
	}
	if _, ok := req["warning"]; !ok {
		t.Fatalf("expected warning for hinted edge; req=%s", raw)
	}
}
