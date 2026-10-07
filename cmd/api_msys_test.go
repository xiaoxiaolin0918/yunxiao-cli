package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

func falseProbe(string) bool { return false }

// TestRestoreMSYSMangledAPIPath locks the restore rules (#117): only
// "<drive>:/<msys-root>/oapi/…" shapes, only with an MSYS env signal or a
// probe-validated MSYS root; everything else (plain paths, URLs, drive paths
// without /oapi/, unsure cases) is left untouched.
func TestRestoreMSYSMangledAPIPath(t *testing.T) {
	probeGitRoot := func(root string) bool { return root == "C:/Program Files/Git" }
	probeMsysUnderOapi := func(root string) bool { return root == "D:/oapi/msys64" }

	cases := []struct {
		name     string
		arg      string
		envSig   bool
		probe    func(string) bool
		want     string
		wantBack bool
	}{
		{
			name:     "real MSYS prefix with env signal",
			arg:      "C:/Program Files/Git/oapi/v1/platform/user",
			envSig:   true,
			probe:    falseProbe,
			want:     "/oapi/v1/platform/user",
			wantBack: true,
		},
		{
			name:     "machine-specific git root (drive D)",
			arg:      "D:/tools/Git/oapi/v1/platform/user",
			envSig:   true,
			probe:    falseProbe,
			want:     "/oapi/v1/platform/user",
			wantBack: true,
		},
		{
			name:     "query string survives",
			arg:      "C:/Program Files/Git/oapi/v1/x?organizationId=1",
			envSig:   true,
			probe:    falseProbe,
			want:     "/oapi/v1/x?organizationId=1",
			wantBack: true,
		},
		{
			name:     "colon-style action path",
			arg:      "C:/Program Files/Git/oapi/v1/organizations/123/workitems:search",
			envSig:   true,
			probe:    falseProbe,
			want:     "/oapi/v1/organizations/123/workitems:search",
			wantBack: true,
		},
		{
			name:     "lowercase drive",
			arg:      "c:/git/oapi/v1/platform/user",
			envSig:   true,
			probe:    falseProbe,
			want:     "/oapi/v1/platform/user",
			wantBack: true,
		},
		{
			name:     "mid-path /oapi/ keeps leftmost restore",
			arg:      "C:/Program Files/Git/oapi/v1/x/oapi/y",
			envSig:   true,
			probe:    falseProbe,
			want:     "/oapi/v1/x/oapi/y",
			wantBack: true,
		},
		{
			name:     "probe-validated root without env signal",
			arg:      "C:/Program Files/Git/oapi/v1/platform/user",
			envSig:   false,
			probe:    probeGitRoot,
			want:     "/oapi/v1/platform/user",
			wantBack: true,
		},
		{
			name: "probe picks the real root when it contains an oapi dir",
			// MSYS installed at D:/oapi/msys64: the leftmost candidate root
			// (D:/oapi) must not win; the probe-validated one restores correctly.
			arg:      "D:/oapi/msys64/oapi/v1/platform/user",
			envSig:   false,
			probe:    probeMsysUnderOapi,
			want:     "/oapi/v1/platform/user",
			wantBack: true,
		},
		{
			name:     "drive-shaped but no signal and no valid probe: unchanged",
			arg:      "C:/Program Files/Git/oapi/v1/platform/user",
			envSig:   false,
			probe:    falseProbe,
			wantBack: false,
		},
		{
			name:     "plain API path unchanged",
			arg:      "/oapi/v1/platform/user",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
		{
			name:     "relative API path unchanged",
			arg:      "oapi/v1/platform/user",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
		{
			name:     "full URL unchanged (not drive-shaped)",
			arg:      "https://openapi-rdc.aliyuncs.com/oapi/v1/platform/user",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
		{
			name:     "drive path without /oapi/ segment unchanged",
			arg:      "C:/Program Files/Git/usr/bin/bash.exe",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
		{
			name:     "git-bash double-slash escape left to the collapse path",
			arg:      "//oapi/v1/platform/user",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
		{
			name:     "empty unchanged",
			arg:      "",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := restoreMSYSMangledAPIPath(tc.arg, tc.envSig, tc.probe)
			if ok != tc.wantBack {
				t.Fatalf("ok=%v want %v (arg %q got %q)", ok, tc.wantBack, tc.arg, got)
			}
			if ok && got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if !ok && got != "" {
				t.Fatalf("not-restored must return empty, got %q", got)
			}
		})
	}
}

func TestUnmangleAPIPathArgDoubleSlashCollapse(t *testing.T) {
	got, note := unmangleAPIPathArg("//oapi/v1/platform/user")
	if got != "/oapi/v1/platform/user" {
		t.Fatalf("got %q", got)
	}
	if note == "" {
		t.Fatal("expected a note for the collapse")
	}
	// Non-API double-slash prefixes are not touched.
	got, note = unmangleAPIPathArg("//custom/prefix")
	if got != "//custom/prefix" || note != "" {
		t.Fatalf("got %q note %q", got, note)
	}
}

func TestUnmangleAPIPathArgNoteAndKillSwitch(t *testing.T) {
	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("MSYS", "")
	mangled := "C:/Program Files/Git/oapi/v1/platform/user"

	got, note := unmangleAPIPathArg(mangled)
	if got != "/oapi/v1/platform/user" {
		t.Fatalf("got %q", got)
	}
	for _, want := range []string{"note:", mangled, "/oapi/v1/platform/user", "MSYS_NO_PATHCONV", envAPIUnmangleOff} {
		if !strings.Contains(note, want) {
			t.Fatalf("note missing %q: %s", want, note)
		}
	}

	// YUNXIAO_API_NO_UNMANGLE=1 disables restore and collapse.
	t.Setenv(envAPIUnmangleOff, "1")
	if got, note := unmangleAPIPathArg(mangled); got != mangled || note != "" {
		t.Fatalf("kill switch: got %q note %q", got, note)
	}
	if got, note := unmangleAPIPathArg("//oapi/v1/x"); got != "//oapi/v1/x" || note != "" {
		t.Fatalf("kill switch collapse: got %q note %q", got, note)
	}

	// "0" keeps the restore enabled.
	t.Setenv(envAPIUnmangleOff, "0")
	if got, _ := unmangleAPIPathArg(mangled); got != "/oapi/v1/platform/user" {
		t.Fatalf("env=0 must keep restore, got %q", got)
	}
}

func TestUnmangleAPIPathArgWithoutSignalLeavesUnchanged(t *testing.T) {
	// No MSYS env signal and a root that is not on disk: conservative no-op.
	t.Setenv("MSYSTEM", "")
	t.Setenv("MSYS", "")
	mangled := "C:/Definitely/Not/A/Root/oapi/v1/platform/user"
	got, note := unmangleAPIPathArg(mangled)
	if got != mangled || note != "" {
		t.Fatalf("got %q note %q", got, note)
	}
}

func TestProbeMSYSRootOnDisk(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "usr", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !probeMSYSRootOnDisk(root) {
		t.Fatal("usr/bin dir must probe as an MSYS root")
	}
	if probeMSYSRootOnDisk(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("missing dir must not probe as an MSYS root")
	}
	if probeMSYSRootOnDisk("") {
		t.Fatal("empty root must not probe")
	}
}

// TestUnmangleAPIPathArgProbeRealFS: a temp dir that looks like an MSYS install
// restores even without the env signal (Windows-only: needs a drive-shaped path).
func TestUnmangleAPIPathArgProbeRealFS(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	if !isWindowsDrivePath(root) {
		t.Skip("temp dir is not drive-shaped on this OS")
	}
	if err := os.MkdirAll(filepath.Join(root, "usr", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "bash.exe"), []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MSYSTEM", "")
	t.Setenv("MSYS", "")
	arg := root + "/oapi/v1/platform/user"
	got, note := unmangleAPIPathArg(arg)
	if got != "/oapi/v1/platform/user" || note == "" {
		t.Fatalf("got %q note %q", got, note)
	}
}

func TestHasWindowsDriveInURLPath(t *testing.T) {
	cases := []struct {
		u    string
		want bool
	}{
		{"https://openapi-rdc.aliyuncs.com/D:/tools/Git/oapi/v1/platform/user", true},
		{"https://g/C:/Program Files/Git/oapi/v1/x", true},
		{"https://g/oapi/v1/platform/user", false},
		{"https://g/oapi/v1/organizations/1/workitems:search", false},
		{"https://g/oapi/v1/x?redirect=/D:/evil", false}, // query stripped
		{"", false},
	}
	for _, tc := range cases {
		if got := hasWindowsDriveInURLPath(tc.u); got != tc.want {
			t.Errorf("hasWindowsDriveInURLPath(%q)=%v want %v", tc.u, got, tc.want)
		}
	}
}

// --- error rendering (#117) ---

func TestHandleErrDecodeErrorMsysHint(t *testing.T) {
	de := &client.DecodeError{
		Method:      "GET",
		URL:         "https://openapi-rdc.aliyuncs.com/D:/tools/Git/oapi/v1/platform/user",
		Status:      200,
		ContentType: "text/html",
		Body:        "<!doctype html><html lang=\"zh\">…(landing page)",
		Err:         fmt.Errorf("invalid character '<' looking for beginning of value"),
	}
	eb, code := handleErrBody(t, de)
	if code != 1 || eb.Type != "api" || eb.Subtype != "non_json_response" {
		t.Fatalf("code=%d body=%+v", code, eb)
	}
	if eb.Code != 200 {
		t.Fatalf("code=%d", eb.Code)
	}
	if eb.Hint != msysHint || !strings.Contains(eb.Hint, "MSYS_NO_PATHCONV") {
		t.Fatalf("hint=%q", eb.Hint)
	}
	for _, want := range []string{de.URL, "HTTP 200", "text/html", "decode response"} {
		if !strings.Contains(eb.Message, want) {
			t.Fatalf("message missing %q: %s", want, eb.Message)
		}
	}
	details := eb.Details
	if details["url"] != de.URL || details["method"] != "GET" || details["content_type"] != "text/html" {
		t.Fatalf("details=%v", details)
	}
}

func TestHandleErrDecodeErrorPlainHTMLHint(t *testing.T) {
	de := &client.DecodeError{
		Method: "GET", URL: "https://g/oapi/v1/nowhere", Status: 200,
		ContentType: "text/html", Body: "<html>", Err: fmt.Errorf("bad"),
	}
	eb, _ := handleErrBody(t, de)
	if eb.Hint != htmlResponseHint {
		t.Fatalf("hint=%q", eb.Hint)
	}
}

func TestHandleErrContextErrorWrapsDecodeError(t *testing.T) {
	de := &client.DecodeError{
		Method: "GET", URL: "https://g/C:/Git/oapi/v1/x", Status: 200,
		ContentType: "text/html", Body: "<html>", Err: fmt.Errorf("bad"),
	}
	eb, code := handleErrBody(t, &contextError{Context: "fetch thing", Hint: "check the id", Err: de})
	if code != 1 || eb.Type != "api" || eb.Subtype != "non_json_response" {
		t.Fatalf("code=%d body=%+v", code, eb)
	}
	if !strings.HasPrefix(eb.Message, "fetch thing: yunxiao API GET") {
		t.Fatalf("message=%q", eb.Message)
	}
	if !strings.Contains(eb.Hint, msysHint) || !strings.HasSuffix(eb.Hint, "; check the id") {
		t.Fatalf("hint=%q", eb.Hint)
	}
}

func TestAPIErrorHintMsysDriveURL(t *testing.T) {
	ae := &client.APIError{Status: 404, Method: "GET", URL: "https://g/D:/tools/Git/oapi/v1/x", Body: "{}"}
	if got := apiErrorHint(ae); got != msysHint {
		t.Fatalf("hint=%q", got)
	}
}

// --- end-to-end: mangled argv → restored request hits the right path ---

// setupAPICmdTestEnv isolates config/auth and points the CLI at srvURL.
func setupAPICmdTestEnv(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv(config.EnvAccessToken, "test-token-117-not-real")
	t.Setenv(config.EnvAPIBaseURL, srvURL)
}

func runAPICmdAgainstServer(t *testing.T, srvURL, mangledArg string) (output.Envelope, string) {
	t.Helper()
	setupAPICmdTestEnv(t, srvURL)
	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("MSYS", "")

	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	globalYes = false
	t.Cleanup(func() { globalDryRun = false; globalYes = false })

	var noteBuf bytes.Buffer
	prevNote := msysNoteOut
	msysNoteOut = &noteBuf
	t.Cleanup(func() { msysNoteOut = prevNote })

	prevExit := processExit
	processExit = func(code int) { panic(exitPanic{code: code}) }
	t.Cleanup(func() { processExit = prevExit })

	rootCmd.SetArgs([]string{"api", "GET", mangledArg})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var execErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		execErr = rootCmd.Execute()
	}()
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout JSON: %v / %s", err, stdout.String())
	}
	return env, noteBuf.String()
}

func TestAPICmdUnmanglesMSYSPathEndToEnd(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":"yes"}`))
	}))
	t.Cleanup(srv.Close)

	env, note := runAPICmdAgainstServer(t, srv.URL, "C:/Program Files/Git/oapi/v1/platform/user")
	if gotPath != "/oapi/v1/platform/user" {
		t.Fatalf("server saw path %q, want /oapi/v1/platform/user", gotPath)
	}
	if !env.OK {
		t.Fatalf("envelope: %+v", env)
	}
	if env.Meta["path"] != "/oapi/v1/platform/user" {
		t.Fatalf("meta.path=%v", env.Meta["path"])
	}
	if !strings.Contains(note, "note:") || !strings.Contains(note, "MSYS_NO_PATHCONV") {
		t.Fatalf("stderr note missing: %q", note)
	}
}

// When the restore is disabled (kill switch) and the gateway answers 2xx HTML,
// the error envelope must be diagnosable: URL + status + content-type + hint.
func TestAPICmdMangledPathHTMLFailureEndToEnd(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><html lang=\"zh\">" + strings.Repeat("x", 3000)))
	}))
	t.Cleanup(srv.Close)

	setupAPICmdTestEnv(t, srv.URL)
	t.Setenv(envAPIUnmangleOff, "1")
	t.Setenv("MSYSTEM", "")
	t.Setenv("MSYS", "")

	// The kill switch must win even with an MSYS env signal present: restore
	// skipped, mangled path sent verbatim, and the HTML failure readable.
	stdout := withCmdJSONCapture(t)
	globalDryRun = false
	t.Cleanup(func() { globalDryRun = false })

	var stderrBuf bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderrBuf
	t.Cleanup(func() { output.Stderr = prevErr })

	prevExit := processExit
	var gotCode int
	processExit = func(code int) { gotCode = code; panic(exitPanic{code: code}) }
	t.Cleanup(func() { processExit = prevExit })

	rootCmd.SetArgs([]string{"api", "GET", "C:/Program Files/Git/oapi/v1/platform/user"})
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
		t.Fatalf("exit code=%d stderr=%s stdout=%s", gotCode, stderrBuf.String(), stdout.String())
	}
	if want := "/C:/Program Files/Git/oapi/v1/platform/user"; gotPath != want {
		t.Fatalf("kill switch must send the mangled path verbatim, server saw %q want %q", gotPath, want)
	}
	var envOut output.Envelope
	if err := json.Unmarshal(stderrBuf.Bytes(), &envOut); err != nil {
		t.Fatalf("stderr JSON: %v / %s", err, stderrBuf.String())
	}
	if envOut.OK || envOut.Error == nil {
		t.Fatalf("envelope: %+v / %s", envOut, stderrBuf.String())
	}
	eb := envOut.Error
	if eb.Type != "api" || eb.Subtype != "non_json_response" {
		t.Fatalf("error body: %+v", eb)
	}
	if eb.Code != 200 || !strings.Contains(eb.Hint, "MSYS_NO_PATHCONV") {
		t.Fatalf("error body: %+v", eb)
	}
	if !strings.Contains(eb.Message, gotPath) || !strings.Contains(eb.Message, "HTTP 200") || !strings.Contains(eb.Message, "text/html") {
		t.Fatalf("message lacks URL/status/content-type: %s", eb.Message)
	}
	if len(eb.Message) > 3000 {
		t.Fatalf("message must not embed the full HTML page: len=%d", len(eb.Message))
	}
}
