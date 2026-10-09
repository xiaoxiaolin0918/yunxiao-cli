package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
)

// Profile configures module/environment + allowed_* snapshots; empty CLI defaults
// must skip send + allowed_* gates (tenant-neutral +bug-create).
func TestBugCreateEmptyModuleEnvDefaultsSkipGate(t *testing.T) {
	var postBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/fields"):
			_, _ = io.WriteString(w, `[
 {"id":"subject","name":"标题","type":"NativeField","format":"string","required":true,"showWhenCreate":true},
 {"id":"assignedTo","name":"负责人","type":"NativeField","format":"user","required":true,"showWhenCreate":true},
 {"id":"sprint","name":"迭代","type":"NativeField","format":"sprint","required":true,"showWhenCreate":true},
 {"id":"priority","name":"优先级","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"prio-high","value":"高","displayValue":"高"}]},
 {"id":"seriousLevel","name":"严重程度","type":"SystemCustomField","format":"list","required":true,"showWhenCreate":true,
  "options":[{"id":"sev-normal","value":"一般","displayValue":"一般"}]},
 {"id":"mod-1","name":"所属模块","type":"CustomField","format":"list","required":false,"showWhenCreate":true,
  "options":[{"id":"m-a","value":"MES","displayValue":"MES"}]},
 {"id":"env-1","name":"环境","type":"CustomField","format":"list","required":false,"showWhenCreate":true,
  "options":[{"id":"e-1","value":"测试环境","displayValue":"测试环境"}]}
]`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/workitems"):
			b, _ := io.ReadAll(r.Body)
			postBody = string(b)
			_, _ = io.WriteString(w, `{"id":"bug-1","serialNumber":"YXCLI-1","subject":"t"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:              "p-debias",
		OrganizationID:    "org-debias",
		SpaceID:           "space-1",
		BugTypeID:         "bug-type-1",
		DefaultAssignedTo: "user-assignee",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
			Module:       "mod-1",
			Environment:  "env-1",
		},
		AllowedModules:      []string{"MES", "OMS"},
		AllowedEnvironments: []string{"生产环境", "测试环境"},
	})
	t.Setenv(config.EnvAccessToken, "test-token-debias-empty-defaults-not-real")
	t.Setenv(config.EnvOrganizationID, "org-debias")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "p-debias")

	prevProfile := globalProfile
	globalProfile = "p-debias"
	t.Cleanup(func() { globalProfile = prevProfile })

	modFlag := workitemBugCreateCmd.Flags().Lookup("module")
	envFlag := workitemBugCreateCmd.Flags().Lookup("environment")
	if modFlag == nil || modFlag.DefValue != "" {
		t.Fatalf("module DefValue want empty, got %#v", modFlag)
	}
	if envFlag == nil || envFlag.DefValue != "" {
		t.Fatalf("environment DefValue want empty, got %#v", envFlag)
	}

	r := runBugCreateDebias(t, true)
	if r.code != 0 {
		t.Fatalf("empty module/env should not gate-fail: exit=%d stdout=%s stderr=%s", r.code, r.stdout, r.stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &env); err != nil {
		t.Fatalf("json: %v\n%s", err, r.stdout)
	}
	req, _ := env["data"].(map[string]any)
	if req == nil {
		// dry-run envelope: data.request.body or similar
		if d, ok := env["data"].(map[string]any); ok {
			req = d
		}
	}
	raw, _ := json.Marshal(env)
	s := string(raw)
	if strings.Contains(s, `"mod-1"`) || strings.Contains(s, "MES") {
		t.Fatalf("dry-run must omit empty module: %s", s)
	}
	if strings.Contains(s, `"env-1"`) || strings.Contains(s, "测试环境") {
		t.Fatalf("dry-run must omit empty environment: %s", s)
	}
	_ = postBody
}

func TestBugCreateExplicitModuleStillGated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeTransitionTestProfile(t, xdg, &profile.Profile{
		Name:           "p-debias2",
		OrganizationID: "org-debias2",
		SpaceID:        "space-1",
		BugTypeID:      "bug-type-1",
		BugCreateFields: profile.BugCreateFields{
			Priority:     map[string]string{"high": "prio-high"},
			SeriousLevel: map[string]string{"normal": "sev-normal"},
			Module:       "mod-1",
			Environment:  "env-1",
		},
		AllowedModules:      []string{"MES", "OMS"},
		AllowedEnvironments: []string{"生产环境", "测试环境"},
	})
	t.Setenv(config.EnvAccessToken, "test-token-debias-gate-not-real")
	t.Setenv(config.EnvOrganizationID, "org-debias2")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "p-debias2")

	prevProfile := globalProfile
	globalProfile = "p-debias2"
	t.Cleanup(func() { globalProfile = prevProfile })

	r := runBugCreateDebias(t, true, "--module", "NOPE")
	if r.code == 0 {
		t.Fatalf("expected allowed_modules gate to reject NOPE: stdout=%s stderr=%s", r.stdout, r.stderr)
	}
	combined := r.stdout + r.stderr
	if !strings.Contains(combined, "MES") && !strings.Contains(combined, "--module") && !strings.Contains(combined, "module") {
		t.Fatalf("error should mention module allow-list: exit=%d out=%q err=%q", r.code, r.stdout, r.stderr)
	}
}

func runBugCreateDebias(t *testing.T, dryRun bool, extra ...string) bugCreateRun {
	t.Helper()
	stdout := withCmdJSONCapture(t)
	prevDry := globalDryRun
	globalDryRun = dryRun
	t.Cleanup(func() { globalDryRun = prevDry })
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	prevYes := globalYes
	if !dryRun {
		globalYes = true
	}
	t.Cleanup(func() { globalYes = prevYes })

	resetStringFlags(t, workitemBugCreateCmd,
		"title", "title-file", "description", "description-file",
		"environment", "module", "priority", "serious-level", "expected-completion",
		"sprint", "assigned-to", "verifier", "minimal", "no-defaults", "no-precheck")

	args := []string{
		"workitem", "+bug-create",
		"--title", "debias title",
		"--description", "debias desc",
		"--sprint", "sprint-1",
		"--no-defaults",
	}
	if dryRun {
		args = append(args, "--dry-run")
	} else {
		args = append(args, "--yes")
	}
	args = append(args, extra...)
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	}()
	return bugCreateRun{stdout: stdout.String(), stderr: stderr.String(), code: code}
}
