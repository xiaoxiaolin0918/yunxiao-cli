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

func protCreateBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-prot-create-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-prot-create-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, codeupProtectedCreateCmd, "repo", "branch", "allow-push-roles", "allow-merge-roles", "body")
	return stdout, hits
}

func protCreateBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, check func(t *testing.T, body map[string]any, req map[string]any)) {
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
	if !strings.Contains(url, "/repositories/4952001/protectedBranches") {
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

func protRolesAsInts(t *testing.T, v any) []int {
	t.Helper()
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("roles not array: %#v", v)
	}
	out := make([]int, 0, len(arr))
	for _, x := range arr {
		switch n := x.(type) {
		case float64:
			out = append(out, int(n))
		case json.Number:
			i, _ := n.Int64()
			out = append(out, int(i))
		default:
			t.Fatalf("role elem %#v", x)
		}
	}
	return out
}

func TestCodeupProtectedCreateBranchOnlyDryRun(t *testing.T) {
	stdout, hits := protCreateBodySetup(t, "branch-only")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4952001",
		"--branch", "release",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	protCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["branch"] != "release" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["allowPushRoles"]; has {
			t.Fatalf("allowPushRoles must be omitted: %#v", body)
		}
		if _, has := body["allowMergeRoles"]; has {
			t.Fatalf("allowMergeRoles must be omitted: %#v", body)
		}
	})
}

func TestCodeupProtectedCreatePushRolesDryRun(t *testing.T) {
	stdout, hits := protCreateBodySetup(t, "push-roles")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4952001",
		"--branch", "main",
		"--allow-push-roles", "40,30",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	protCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["branch"] != "main" {
			t.Fatalf("body=%v", body)
		}
		roles := protRolesAsInts(t, body["allowPushRoles"])
		if len(roles) != 2 || roles[0] != 40 || roles[1] != 30 {
			t.Fatalf("allowPushRoles=%v", body["allowPushRoles"])
		}
		if _, has := body["allowMergeRoles"]; has {
			t.Fatalf("allowMergeRoles must be omitted: %#v", body)
		}
	})
}

func TestCodeupProtectedCreateMergeRolesDryRun(t *testing.T) {
	stdout, hits := protCreateBodySetup(t, "merge-roles")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4952001",
		"--branch", "main",
		"--allow-merge-roles", "40",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	protCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		roles := protRolesAsInts(t, body["allowMergeRoles"])
		if len(roles) != 1 || roles[0] != 40 {
			t.Fatalf("allowMergeRoles=%v", body["allowMergeRoles"])
		}
		if _, has := body["allowPushRoles"]; has {
			t.Fatalf("allowPushRoles must be omitted: %#v", body)
		}
	})
}

func TestCodeupProtectedCreateBothRolesDryRun(t *testing.T) {
	stdout, hits := protCreateBodySetup(t, "both-roles")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4952001",
		"--branch", "develop",
		"--allow-push-roles", "30",
		"--allow-merge-roles", "40,30",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	protCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["branch"] != "develop" {
			t.Fatalf("body=%v", body)
		}
		push := protRolesAsInts(t, body["allowPushRoles"])
		merge := protRolesAsInts(t, body["allowMergeRoles"])
		if len(push) != 1 || push[0] != 30 {
			t.Fatalf("allowPushRoles=%v", body["allowPushRoles"])
		}
		if len(merge) != 2 || merge[0] != 40 || merge[1] != 30 {
			t.Fatalf("allowMergeRoles=%v", body["allowMergeRoles"])
		}
	})
}

func TestCodeupProtectedCreateBodyJSONMergeDryRun(t *testing.T) {
	stdout, hits := protCreateBodySetup(t, "body-merge")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4952001",
		"--branch", "main",
		"--allow-push-roles", "40",
		"--body", `{"testSetting":true,"note":"batch"}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	protCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["branch"] != "main" || body["note"] != "batch" || body["testSetting"] != true {
			t.Fatalf("body=%v", body)
		}
		push := protRolesAsInts(t, body["allowPushRoles"])
		if len(push) != 1 || push[0] != 40 {
			t.Fatalf("allowPushRoles=%v", body["allowPushRoles"])
		}
	})
}

func TestCodeupProtectedCreateBodyBranchOverrideDryRun(t *testing.T) {
	stdout, hits := protCreateBodySetup(t, "body-branch")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4952001",
		"--branch", "main",
		"--body", `{"branch":"hotfix"}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	protCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["branch"] != "hotfix" {
			t.Fatalf("body.branch should win: body=%v", body)
		}
	})
}

func TestCodeupProtectedCreateBodyOnlyBranchDryRun(t *testing.T) {
	stdout, hits := protCreateBodySetup(t, "body-only")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4952001",
		"--body", `{"branch":"from-body","allowPushRoles":[20]}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	protCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["branch"] != "from-body" {
			t.Fatalf("body=%v", body)
		}
		push := protRolesAsInts(t, body["allowPushRoles"])
		if len(push) != 1 || push[0] != 20 {
			t.Fatalf("allowPushRoles=%v", body["allowPushRoles"])
		}
	})
}

func TestCodeupProtectedCreateBodyOverridesRolesDryRun(t *testing.T) {
	stdout, hits := protCreateBodySetup(t, "body-roles")
	rootCmd.SetArgs([]string{
		"codeup", "protected-branches", "create",
		"--repo", "4952001",
		"--branch", "main",
		"--allow-push-roles", "40,30",
		"--body", `{"allowPushRoles":[10]}`,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	protCreateBodyAssert(t, stdout, hits, func(t *testing.T, body map[string]any, _ map[string]any) {
		push := protRolesAsInts(t, body["allowPushRoles"])
		if len(push) != 1 || push[0] != 10 {
			t.Fatalf("body allowPushRoles should win: %v", body["allowPushRoles"])
		}
	})
}