package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"fmt"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func wiCreateBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-create-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-create-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, workitemCreateCmd,
		"space-id", "type-id", "subject", "subject-file", "assigned-to",
		"description", "description-file", "format-type", "parent-id", "sprint",
		"labels", "participants", "trackers", "verifier", "versions",
		"priority", "custom-fields", "custom-fields-file",
		"no-defaults", "no-precheck", "full",
	)
	return stdout, hits
}

func wiCreateBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any)) {
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
	if !strings.Contains(url, "/workitems") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	if body["spaceId"] != "space-wi-create" || body["workitemTypeId"] != "type-req" {
		t.Fatalf("base body=%v", body)
	}
	if check != nil {
		check(t, body)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func wiCreateCSV(t *testing.T, v any) []string {
	t.Helper()
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("not array: %#v", v)
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

func TestWorkitemCreateSubjectAssignedDryRun(t *testing.T) {
	stdout, hits := wiCreateBodySetup(t, "min")
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-wi-create",
		"--type-id", "type-req",
		"--subject", "Batch create min",
		"--assigned-to", "user-1",
		"--no-precheck", "--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["subject"] != "Batch create min" || body["assignedTo"] != "user-1" {
			t.Fatalf("body=%v", body)
		}
		for _, k := range []string{"description", "parentId", "sprint", "labels", "participants", "trackers", "verifier", "versions", "customFieldValues"} {
			if _, has := body[k]; has {
				t.Fatalf("%s must be omitted: %#v", k, body)
			}
		}
	})
}

func TestWorkitemCreateDescriptionFormatDryRun(t *testing.T) {
	stdout, hits := wiCreateBodySetup(t, "desc")
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-wi-create",
		"--type-id", "type-req",
		"--subject", "With desc",
		"--assigned-to", "user-1",
		"--description", "hello **md**",
		"--format-type", "MARKDOWN",
		"--no-precheck", "--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["description"] != "hello **md**" || body["formatType"] != "MARKDOWN" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemCreateParentIdDryRun(t *testing.T) {
	stdout, hits := wiCreateBodySetup(t, "parent")
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-wi-create",
		"--type-id", "type-req",
		"--subject", "Child",
		"--assigned-to", "user-2",
		"--parent-id", "wi-parent-9",
		"--no-precheck", "--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["parentId"] != "wi-parent-9" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemCreateSprintLabelsDryRun(t *testing.T) {
	stdout, hits := wiCreateBodySetup(t, "sprint-labels")
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-wi-create",
		"--type-id", "type-req",
		"--subject", "Sprint item",
		"--assigned-to", "user-1",
		"--sprint", "sp-42",
		"--labels", "lab-a,lab-b",
		"--no-precheck", "--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["sprint"] != "sp-42" {
			t.Fatalf("body=%v", body)
		}
		labs := wiCreateCSV(t, body["labels"])
		if len(labs) != 2 || labs[0] != "lab-a" || labs[1] != "lab-b" {
			t.Fatalf("labels=%v", body["labels"])
		}
	})
}

func TestWorkitemCreateParticipantsTrackersDryRun(t *testing.T) {
	stdout, hits := wiCreateBodySetup(t, "people")
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-wi-create",
		"--type-id", "type-req",
		"--subject", "People",
		"--assigned-to", "user-1",
		"--participants", "u-a,u-b",
		"--trackers", "u-c",
		"--no-precheck", "--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		p := wiCreateCSV(t, body["participants"])
		tr := wiCreateCSV(t, body["trackers"])
		if len(p) != 2 || p[0] != "u-a" || p[1] != "u-b" {
			t.Fatalf("participants=%v", body["participants"])
		}
		if len(tr) != 1 || tr[0] != "u-c" {
			t.Fatalf("trackers=%v", body["trackers"])
		}
	})
}

func TestWorkitemCreateVerifierVersionsDryRun(t *testing.T) {
	stdout, hits := wiCreateBodySetup(t, "verifier")
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-wi-create",
		"--type-id", "type-req",
		"--subject", "Verifier",
		"--assigned-to", "user-1",
		"--verifier", "user-qa",
		"--versions", "ver-1,ver-2",
		"--no-precheck", "--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["verifier"] != "user-qa" {
			t.Fatalf("body=%v", body)
		}
		vers := wiCreateCSV(t, body["versions"])
		if len(vers) != 2 || vers[0] != "ver-1" || vers[1] != "ver-2" {
			t.Fatalf("versions=%v", body["versions"])
		}
	})
}

func TestWorkitemCreateSubjectFileDryRun(t *testing.T) {
	stdout, hits := wiCreateBodySetup(t, "subj-file")
	dir := t.TempDir()
	fp := filepath.Join(dir, "subject.txt")
	if err := os.WriteFile(fp, []byte("\ufeffSubject from file"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-wi-create",
		"--type-id", "type-req",
		"--subject-file", fp,
		"--assigned-to", "user-1",
		"--no-precheck", "--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["subject"] != "Subject from file" {
			t.Fatalf("subject=%v", body["subject"])
		}
	})
}

func TestWorkitemCreateDescriptionFileDryRun(t *testing.T) {
	stdout, hits := wiCreateBodySetup(t, "desc-file")
	dir := t.TempDir()
	fp := filepath.Join(dir, "desc.md")
	if err := os.WriteFile(fp, []byte("\ufeffDesc from file"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-wi-create",
		"--type-id", "type-req",
		"--subject", "With file desc",
		"--assigned-to", "user-1",
		"--description-file", fp,
		"--no-precheck", "--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any) {
		if body["description"] != "Desc from file" {
			t.Fatalf("description=%v", body["description"])
		}
	})
}
