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

func wiUpdateFieldDryRunSetup(t *testing.T, tokenOrg string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-upd-"+tokenOrg+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-upd-"+tokenOrg)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, workitemUpdateCmd,
		"id", "subject", "status", "assigned-to", "priority", "description",
		"labels", "sprint", "verifier", "participants", "trackers", "versions",
		"custom-fields", "cancel-reason",
	)
	return stdout, hits
}

func wiUpdateFieldAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantBody map[string]any) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.Write) && env.Risk != "write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/workitems/wi-upd-batch") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil req=%v", req)
	}
	for k, want := range wantBody {
		got := body[k]
		switch w := want.(type) {
		case []string:
			gotRaw, _ := json.Marshal(got)
			s := string(gotRaw)
			for _, item := range w {
				if !strings.Contains(s, item) {
					t.Fatalf("body[%s] missing %q got=%v body=%v", k, item, got, body)
				}
			}
		default:
			if got != want {
				t.Fatalf("body[%s]=%v want=%v body=%v", k, got, want, body)
			}
		}
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestWorkitemUpdateStatusFieldDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "status")
	rootCmd.SetArgs([]string{"workitem", "update", "--id", "wi-upd-batch", "--status", "100005", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{"status": "100005"})
}

func TestWorkitemUpdateAssignedToFieldDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "assigned")
	rootCmd.SetArgs([]string{"workitem", "update", "--id", "wi-upd-batch", "--assigned-to", "user-42", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{"assignedTo": "user-42"})
}

func TestWorkitemUpdatePriorityFieldDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "priority")
	rootCmd.SetArgs([]string{"workitem", "update", "--id", "wi-upd-batch", "--priority", "87", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{"priority": "87"})
}

func TestWorkitemUpdateDescriptionFieldDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "desc")
	rootCmd.SetArgs([]string{"workitem", "update", "--id", "wi-upd-batch", "--description", "batch-desc", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{"description": "batch-desc"})
}

func TestWorkitemUpdateLabelsFieldDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "labels")
	rootCmd.SetArgs([]string{"workitem", "update", "--id", "wi-upd-batch", "--labels", "lab-a,lab-b", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{"labels": []string{"lab-a", "lab-b"}})
}

func TestWorkitemUpdateSprintFieldDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "sprint")
	rootCmd.SetArgs([]string{"workitem", "update", "--id", "wi-upd-batch", "--sprint", "sp-9", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{"sprint": "sp-9"})
}

func TestWorkitemUpdateVerifierFieldDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "verifier")
	rootCmd.SetArgs([]string{"workitem", "update", "--id", "wi-upd-batch", "--verifier", "user-v1", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{"verifier": "user-v1"})
}

func TestWorkitemUpdateParticipantsTrackersVersionsDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "ptv")
	rootCmd.SetArgs([]string{
		"workitem", "update",
		"--id", "wi-upd-batch",
		"--participants", "u-p1,u-p2",
		"--trackers", "u-t1",
		"--versions", "ver-1,ver-2",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{
		"participants": []string{"u-p1", "u-p2"},
		"trackers":     []string{"u-t1"},
		"versions":     []string{"ver-1", "ver-2"},
	})
}

func TestWorkitemUpdateCustomFieldsDryRun(t *testing.T) {
	stdout, hits := wiUpdateFieldDryRunSetup(t, "cf")
	rootCmd.SetArgs([]string{
		"workitem", "update",
		"--id", "wi-upd-batch",
		"--custom-fields", `{"80":"2026-10-01","cfX":"yes"}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiUpdateFieldAssert(t, stdout, hits, map[string]any{"80": "2026-10-01", "cfX": "yes"})
}