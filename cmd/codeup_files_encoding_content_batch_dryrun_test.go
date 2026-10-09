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
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func filesEncSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-files-enc-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-files-enc-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, codeupFilesCreateCmd, "repo", "path", "branch", "message", "content", "content-file", "encoding")
	resetStringFlags(t, codeupFilesUpdateCmd, "repo", "path", "branch", "message", "content", "content-file", "encoding")
	resetStringFlags(t, codeupFilesDeleteCmd, "repo", "path", "branch", "message")
	return stdout, hits
}

func filesEncAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantMethod, pathSub string, check func(t *testing.T, body map[string]any, req map[string]any)) {
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
	if !strings.Contains(url, pathSub) {
		t.Fatalf("url=%q want %q", url, pathSub)
	}
	if req["method"] != wantMethod {
		t.Fatalf("method=%v want %s", req["method"], wantMethod)
	}
	if check != nil {
		body, _ := req["body"].(map[string]any)
		check(t, body, req)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestCodeupFilesCreateEncodingBase64DryRun(t *testing.T) {
	stdout, hits := filesEncSetup(t, "create-b64")
	rootCmd.SetArgs([]string{
		"codeup", "files", "create",
		"--repo", "4952001",
		"--path", "bin/a.dat",
		"--branch", "main",
		"--message", "add bin",
		"--content", "aGVsbG8=",
		"--encoding", "base64",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	filesEncAssert(t, stdout, hits, "POST", "/repositories/4952001/files", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body == nil || body["encoding"] != "base64" || body["content"] != "aGVsbG8=" {
			t.Fatalf("body=%v", body)
		}
		if body["filePath"] != "bin/a.dat" || body["commitMessage"] != "add bin" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestCodeupFilesCreateDefaultEncodingTextDryRun(t *testing.T) {
	stdout, hits := filesEncSetup(t, "create-def-enc")
	rootCmd.SetArgs([]string{
		"codeup", "files", "create",
		"--repo", "4952001",
		"--path", "a.txt",
		"--branch", "main",
		"--message", "add a",
		"--content", "plain",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	filesEncAssert(t, stdout, hits, "POST", "/repositories/4952001/files", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["encoding"] != "text" || body["content"] != "plain" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestCodeupFilesCreateContentFileDryRun(t *testing.T) {
	stdout, hits := filesEncSetup(t, "create-file")
	dir := t.TempDir()
	fp := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(fp, []byte("from-file-body"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{
		"codeup", "files", "create",
		"--repo", "4952001",
		"--path", "from/file.txt",
		"--branch", "feat/x",
		"--message", "from file",
		"--content-file", fp,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	filesEncAssert(t, stdout, hits, "POST", "/repositories/4952001/files", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "from-file-body" || body["branch"] != "feat/x" {
			t.Fatalf("body=%v", body)
		}
		if body["encoding"] != "text" {
			t.Fatalf("encoding=%v", body["encoding"])
		}
	})
}

func TestCodeupFilesUpdateEncodingBase64DryRun(t *testing.T) {
	stdout, hits := filesEncSetup(t, "update-b64")
	rootCmd.SetArgs([]string{
		"codeup", "files", "update",
		"--repo", "4952002",
		"--path", "bin/a.dat",
		"--branch", "main",
		"--message", "upd bin",
		"--content", "d29ybGQ=",
		"--encoding", "base64",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	filesEncAssert(t, stdout, hits, "PUT", "/repositories/4952002/files/", func(t *testing.T, body map[string]any, req map[string]any) {
		url, _ := req["url"].(string)
		if !strings.Contains(url, "/files/") {
			t.Fatalf("url=%q", url)
		}
		if body["encoding"] != "base64" || body["content"] != "d29ybGQ=" || body["commitMessage"] != "upd bin" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["filePath"]; has {
			t.Fatalf("update must not send filePath in body: %#v", body)
		}
	})
}

func TestCodeupFilesUpdateContentFileDryRun(t *testing.T) {
	stdout, hits := filesEncSetup(t, "update-file")
	dir := t.TempDir()
	fp := filepath.Join(dir, "upd.txt")
	if err := os.WriteFile(fp, []byte("updated-from-file"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{
		"codeup", "files", "update",
		"--repo", "4952002",
		"--path", "docs/x.md",
		"--branch", "main",
		"--message", "upd from file",
		"--content-file", fp,
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	filesEncAssert(t, stdout, hits, "PUT", "/repositories/4952002/files/", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "updated-from-file" || body["commitMessage"] != "upd from file" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestCodeupFilesUpdateCommitMessageEncodingDryRun(t *testing.T) {
	stdout, hits := filesEncSetup(t, "update-msg-enc")
	rootCmd.SetArgs([]string{
		"codeup", "files", "update",
		"--repo", "4952002",
		"--path", "a.txt",
		"--branch", "develop",
		"--message", "chore: encoding text",
		"--content", "v2",
		"--encoding", "text",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	filesEncAssert(t, stdout, hits, "PUT", "/repositories/4952002/files/", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["branch"] != "develop" || body["commitMessage"] != "chore: encoding text" || body["encoding"] != "text" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestCodeupFilesCreateNestedPathDryRun(t *testing.T) {
	stdout, hits := filesEncSetup(t, "create-nested")
	rootCmd.SetArgs([]string{
		"codeup", "files", "create",
		"--repo", "4952001",
		"--path", "deep/nested/path/f.txt",
		"--branch", "main",
		"--message", "nested",
		"--content", "n",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	filesEncAssert(t, stdout, hits, "POST", "/repositories/4952001/files", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["filePath"] != "deep/nested/path/f.txt" {
			t.Fatalf("filePath=%v", body["filePath"])
		}
	})
}

func TestCodeupFilesCreateContentFileBase64DryRun(t *testing.T) {
	stdout, hits := filesEncSetup(t, "create-file-b64")
	dir := t.TempDir()
	fp := filepath.Join(dir, "b64.txt")
	if err := os.WriteFile(fp, []byte("YmluYXJ5"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{
		"codeup", "files", "create",
		"--repo", "4952001",
		"--path", "b.bin",
		"--branch", "main",
		"--message", "b64 file",
		"--content-file", fp,
		"--encoding", "base64",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	filesEncAssert(t, stdout, hits, "POST", "/repositories/4952001/files", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["content"] != "YmluYXJ5" || body["encoding"] != "base64" {
			t.Fatalf("body=%v", body)
		}
	})
}