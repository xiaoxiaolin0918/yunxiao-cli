package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func pkgUploadSetup(t *testing.T, tag string, handler http.HandlerFunc) (stdout *bytes.Buffer, hits *int, protocolBase string) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if handler != nil {
			handler(w, r)
			return
		}
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-pkg-upload-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-pkg-upload-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv(envPackagesUsername, "pkg-user")
	t.Setenv(envPackagesPassword, "pkg-pass")
	t.Setenv(envPackagesProtocolBase, srv.URL)

	stdout = withCmdJSONCapture(t)
	// withCmdJSONCapture defaults dry-run=true; reset for optional live paths.
	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })
	resetStringFlags(t, packagesArtifactsUploadCmd,
		"repo-id", "repo-type", "file", "path", "version", "file-name", "description",
		"group-id", "artifact-id", "classifier", "username", "password", "protocol-base")
	return stdout, hits, srv.URL
}

func TestPackagesArtifactsUploadGenericDryRun(t *testing.T) {
	stdout, hits, base := pkgUploadSetup(t, "generic-dry", nil)
	dir := t.TempDir()
	rel := "artifact.bin"
	if err := os.WriteFile(filepath.Join(dir, rel), []byte("hello-generic"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	rootCmd.SetArgs([]string{
		"packages", "artifacts", "upload",
		"--repo-id", "repo-g",
		"--repo-type", "GENERIC",
		"--path", "myapp/build",
		"--version", "1.0.0",
		"--file", rel,
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
	if !env.OK || !env.DryRun || env.Risk != string(risk.Write) {
		t.Fatalf("%+v", env)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	if req["method"] != "POST" {
		t.Fatalf("method=%v", req["method"])
	}
	url, _ := req["url"].(string)
	if !strings.HasPrefix(url, base+"/") || !strings.Contains(url, "/generic/repo-g/files/myapp/build") || !strings.Contains(url, "version=1.0.0") {
		t.Fatalf("url=%q base=%q", url, base)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestPackagesArtifactsUploadMavenDryRun(t *testing.T) {
	stdout, hits, base := pkgUploadSetup(t, "maven-dry", nil)
	dir := t.TempDir()
	rel := "lib.jar"
	if err := os.WriteFile(filepath.Join(dir, rel), []byte("jar-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	rootCmd.SetArgs([]string{
		"packages", "artifacts", "upload",
		"--repo-id", "repo-m",
		"--repo-type", "MAVEN",
		"--group-id", "com.example",
		"--artifact-id", "demo",
		"--version", "2.0.0",
		"--file", rel,
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
	if req["method"] != "PUT" {
		t.Fatalf("method=%v", req["method"])
	}
	url, _ := req["url"].(string)
	want := base + "/org-pkg-upload-maven-dry/maven/repo-m/com/example/demo/2.0.0/lib.jar"
	if url != want {
		t.Fatalf("url=%q want %q", url, want)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestPackagesArtifactsUploadGenericLive(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody []byte
	stdout, hits, _ := pkgUploadSetup(t, "generic-live", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = r.ParseMultipartForm(1 << 20)
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		defer f.Close()
		gotBody, _ = io.ReadAll(f)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"successful":true,"object":{"ok":true}}`))
	})
	dir := t.TempDir()
	rel := "payload.bin"
	if err := os.WriteFile(filepath.Join(dir, rel), []byte("payload-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	rootCmd.SetArgs([]string{
		"packages", "artifacts", "upload",
		"--repo-id", "repo-g",
		"--repo-type", "GENERIC",
		"--path", "p/a",
		"--version", "9.9.9",
		"--file", rel,
		"--yes",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	if *hits != 1 || gotMethod != "POST" || !strings.Contains(gotPath, "/generic/repo-g/files/p/a") {
		t.Fatalf("hits=%d method=%s path=%s stdout=%s", *hits, gotMethod, gotPath, stdout.String())
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("auth=%q", gotAuth)
	}
	if string(gotBody) != "payload-data" {
		t.Fatalf("body=%q", gotBody)
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil || !env.OK {
		t.Fatalf("%v %+v %s", err, env, stdout.String())
	}
}

func TestPackagesArtifactsUploadMissingGenericPath(t *testing.T) {
	stdout, hits, _ := pkgUploadSetup(t, "miss-path", nil)
	dir := t.TempDir()
	rel := "f.bin"
	_ = os.WriteFile(filepath.Join(dir, rel), []byte("x"), 0o644)
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	processExit = func(c int) { panic(fmt.Sprintf("exit:%d", c)) }
	t.Cleanup(func() { processExit = prevExit })

	rootCmd.SetArgs([]string{
		"packages", "artifacts", "upload",
		"--repo-id", "r", "--repo-type", "GENERIC", "--version", "1", "--file", rel, "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() { _ = recover() }()
		_ = rootCmd.Execute()
	}()
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "path") && !strings.Contains(out, "required") {
		t.Fatalf("expected path error: %s", out)
	}
}

func TestPackagesArtifactsUploadUnsupportedNPM(t *testing.T) {
	stdout, hits, _ := pkgUploadSetup(t, "npm", nil)
	dir := t.TempDir()
	rel := "pkg.tgz"
	_ = os.WriteFile(filepath.Join(dir, rel), []byte("x"), 0o644)
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	processExit = func(c int) { panic(fmt.Sprintf("exit:%d", c)) }
	t.Cleanup(func() { processExit = prevExit })

	rootCmd.SetArgs([]string{
		"packages", "artifacts", "upload",
		"--repo-id", "r", "--repo-type", "NPM", "--file", rel, "--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() { _ = recover() }()
		_ = rootCmd.Execute()
	}()
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "NPM") || !strings.Contains(out, "npm publish") {
		t.Fatalf("stdout+stderr=%s", out)
	}
}

func TestPackagesGenericUploadURLHelpers(t *testing.T) {
	u, err := packagesGenericUploadURL("https://packages.example/api/protocol", "org1", "repo1", "/a/b", "1.2.3", "f.bin", "desc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "/org1/generic/repo1/files/a/b?") || !strings.Contains(u, "version=1.2.3") || !strings.Contains(u, "fileName=f.bin") {
		t.Fatalf("%s", u)
	}
	mu, name, err := packagesMavenUploadURL("https://packages.example/api/protocol", "org1", "repo1", "com.foo", "bar", "0.1.0", "", "bar-0.1.0.jar")
	if err != nil || name != "bar-0.1.0.jar" {
		t.Fatalf("%v %s %s", err, mu, name)
	}
	if !strings.HasSuffix(mu, "/org1/maven/repo1/com/foo/bar/0.1.0/bar-0.1.0.jar") {
		t.Fatalf("%s", mu)
	}
}
