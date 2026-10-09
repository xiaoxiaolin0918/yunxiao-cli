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

func mrsCreateBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-mrs-create-body-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-create-body-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, codeupMrsCreateCmd,
		"repo", "source", "target", "title", "description",
		"reviewer", "source-project-id", "target-project-id", "create-from", "work-item",
	)
	// DefValue for create-from is WEB; resetStringFlags may leave empty if DefValue blank in some builds.
	if f := codeupMrsCreateCmd.Flags().Lookup("create-from"); f != nil && f.DefValue == "" {
		_ = codeupMrsCreateCmd.Flags().Set("create-from", "WEB")
		f.Changed = false
	}
	return stdout, hits
}

func mrsCreateBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any, req map[string]any)) {
	t.Helper()
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
	if !strings.Contains(url, "/repositories/") || !strings.Contains(url, "/changeRequests") {
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

func TestCodeupMrsCreateDescriptionDryRun(t *testing.T) {
	stdout, hits := mrsCreateBodySetup(t, "desc")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", "4951320",
		"--source", "feat/x",
		"--target", "master",
		"--title", "feat: x",
		"--description", "batch desc body",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	mrsCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["description"] != "batch desc body" {
			t.Fatalf("description=%v", body["description"])
		}
	})
}

func TestCodeupMrsCreateFromCommandLineDryRun(t *testing.T) {
	stdout, hits := mrsCreateBodySetup(t, "create-from")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", "4951320",
		"--source", "feat/x",
		"--target", "master",
		"--title", "feat: x",
		"--create-from", "COMMAND_LINE",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	mrsCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["createFrom"] != "COMMAND_LINE" {
			t.Fatalf("createFrom=%v", body["createFrom"])
		}
	})
}

func TestCodeupMrsCreateProjectIdsDryRun(t *testing.T) {
	stdout, hits := mrsCreateBodySetup(t, "proj-ids")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", "4951320",
		"--source", "feat/x",
		"--target", "master",
		"--title", "feat: x",
		"--source-project-id", "111",
		"--target-project-id", "222",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	mrsCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["sourceProjectId"] != "111" || body["targetProjectId"] != "222" {
			t.Fatalf("source=%v target=%v", body["sourceProjectId"], body["targetProjectId"])
		}
	})
}

func TestCodeupMrsCreateDefaultCreateFromWEBDryRun(t *testing.T) {
	stdout, hits := mrsCreateBodySetup(t, "default-web")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", "4951320",
		"--source", "feat/x",
		"--target", "master",
		"--title", "feat: x",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	mrsCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["createFrom"] != "WEB" {
			t.Fatalf("createFrom=%v want WEB", body["createFrom"])
		}
	})
}

func TestCodeupMrsCreateNoDescriptionOmitsFieldDryRun(t *testing.T) {
	stdout, hits := mrsCreateBodySetup(t, "no-desc")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", "4951320",
		"--source", "feat/x",
		"--target", "master",
		"--title", "feat: x",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	mrsCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if _, has := body["description"]; has {
			t.Fatalf("empty description must omit field: %#v", body["description"])
		}
	})
}

func TestCodeupMrsCreateSingleReviewerDryRun(t *testing.T) {
	stdout, hits := mrsCreateBodySetup(t, "one-rev")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", "4951320",
		"--source", "feat/x",
		"--target", "master",
		"--title", "feat: x",
		"--reviewer", "uid-only",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	mrsCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		ids, ok := body["reviewerUserIds"].([]any)
		if !ok || len(ids) != 1 || ids[0] != "uid-only" {
			t.Fatalf("reviewerUserIds=%#v", body["reviewerUserIds"])
		}
		if _, hasWrong := body["reviewerIds"]; hasWrong {
			t.Fatalf("must not send legacy reviewerIds: %#v", body)
		}
	})
}

func TestCodeupMrsCreateDescriptionAndCreateFromDryRun(t *testing.T) {
	stdout, hits := mrsCreateBodySetup(t, "desc-from")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", "4951320",
		"--source", "feat/x",
		"--target", "master",
		"--title", "feat: x",
		"--description", "combo desc",
		"--create-from", "COMMAND_LINE",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	mrsCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["description"] != "combo desc" || body["createFrom"] != "COMMAND_LINE" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestCodeupMrsCreateBranchesTitleBodyDryRun(t *testing.T) {
	stdout, hits := mrsCreateBodySetup(t, "branches-title")
	rootCmd.SetArgs([]string{
		"codeup", "mrs", "create",
		"--repo", "4951320",
		"--source", "feat/batch-src",
		"--target", "develop",
		"--title", "feat: batch title",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	mrsCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["sourceBranch"] != "feat/batch-src" || body["targetBranch"] != "develop" || body["title"] != "feat: batch title" {
			t.Fatalf("body=%v", body)
		}
		// numeric --repo fills both project ids when flags omitted
		if body["sourceProjectId"] != "4951320" || body["targetProjectId"] != "4951320" {
			t.Fatalf("projectIds source=%v target=%v", body["sourceProjectId"], body["targetProjectId"])
		}
	})
}