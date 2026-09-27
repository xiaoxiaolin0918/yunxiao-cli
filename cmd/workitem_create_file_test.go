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
)

func TestWorkitemCreateFileFlagsHelpAndSchemaHints(t *testing.T) {
	for _, name := range []string{"custom-fields-file", "description-file", "subject-file"} {
		if workitemCreateCmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
	long := workitemCreateCmd.Long
	for _, needle := range []string{"custom-fields-file", "description-file", "subject-file", "Windows"} {
		if !strings.Contains(long, needle) {
			t.Fatalf("create Long should mention %q: %s", needle, long)
		}
	}
	cf := workitemCreateCmd.Flags().Lookup("custom-fields")
	if cf == nil || !strings.Contains(cf.Usage, "custom-fields-file") {
		t.Fatalf("--custom-fields usage should point at --custom-fields-file: %q", cf.Usage)
	}
	cff := workitemCreateCmd.Flags().Lookup("custom-fields-file")
	if cff == nil || !strings.Contains(strings.ToLower(cff.Usage), "windows") {
		t.Fatalf("--custom-fields-file usage should mention Windows: %q", cff.Usage)
	}
}

func TestReadFlagOrFile(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("note.txt", []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readFlagOrFile("", "note.txt", "description", false)
	if err != nil || got != "hello" {
		t.Fatalf("file: %q %v", got, err)
	}
	got, err = readFlagOrFile("inline", "", "description", false)
	if err != nil || got != "inline" {
		t.Fatalf("inline: %q %v", got, err)
	}
	if _, err := readFlagOrFile("x", "note.txt", "description", false); err == nil || !strings.Contains(err.Error(), "description-file") {
		t.Fatalf("mutex: %v", err)
	}
	got, err = readFlagOrFile("", "", "description", false)
	if err != nil || got != "" {
		t.Fatalf("optional empty: %q %v", got, err)
	}
	if _, err := readFlagOrFile("", "", "subject", true); err == nil || !strings.Contains(err.Error(), "subject-file") {
		t.Fatalf("required: %v", err)
	}
	abs := filepath.Join(dir, "note.txt")
	got, err = readFlagOrFile("", abs, "subject", true)
	if err != nil || got != "hello" {
		t.Fatalf("abs: %q %v", got, err)
	}
	if _, err := readFlagOrFile("", "../note.txt", "subject", true); err == nil {
		t.Fatal("parent escape should fail")
	}
	// empty / whitespace required file content must fail (not return "")
	if err := os.WriteFile("empty.txt", []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("blank.txt", []byte("  \n\t  "), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFlagOrFile("", "empty.txt", "subject", true); err == nil || !strings.Contains(err.Error(), "subject-file") {
		t.Fatalf("empty required file: %v", err)
	}
	if _, err := readFlagOrFile("", "blank.txt", "subject", true); err == nil || !strings.Contains(err.Error(), "subject-file") {
		t.Fatalf("whitespace required file: %v", err)
	}
	if _, err := readFlagOrFile("   \t  ", "", "subject", true); err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("whitespace required inline: %v", err)
	}
	// optional empty/whitespace file still ok
	got, err = readFlagOrFile("", "empty.txt", "description", false)
	if err != nil || got != "" {
		t.Fatalf("optional empty file: %q %v", got, err)
	}
	got, err = readFlagOrFile("", "blank.txt", "description", false)
	if err != nil || strings.TrimSpace(got) != "" {
		t.Fatalf("optional blank file: %q %v", got, err)
	}
}

func TestReadFlagOrFileStripsBOM(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	payload := append([]byte{0xEF, 0xBB, 0xBF}, []byte("中文标题")...)
	if err := os.WriteFile("sub.txt", payload, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readFlagOrFile("", "sub.txt", "subject", true)
	if err != nil || got != "中文标题" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestReadJSONMapFlagOrFile(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("cf.json", []byte(`{"fid":"生产环境"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"a":1}`)...)
	if err := os.WriteFile("bom.json", bom, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("bad.json", []byte(`{not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("arr.json", []byte(`[1]`), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := readJSONMapFlagOrFile("", "cf.json", "custom-fields")
	if err != nil || m["fid"] != "生产环境" {
		t.Fatalf("file: %#v %v", m, err)
	}
	m, err = readJSONMapFlagOrFile(`{"x":true}`, "", "custom-fields")
	if err != nil || m["x"] != true {
		t.Fatalf("inline: %#v %v", m, err)
	}
	if _, err := readJSONMapFlagOrFile(`{}`, "cf.json", "custom-fields"); err == nil || !strings.Contains(err.Error(), "custom-fields-file") {
		t.Fatalf("mutex: %v", err)
	}
	m, err = readJSONMapFlagOrFile("", "", "custom-fields")
	if err != nil || m != nil {
		t.Fatalf("empty: %#v %v", m, err)
	}
	m, err = readJSONMapFlagOrFile("", "bom.json", "custom-fields")
	if err != nil || m["a"].(float64) != 1 {
		t.Fatalf("bom: %#v %v", m, err)
	}
	if _, err := readJSONMapFlagOrFile("", "bad.json", "custom-fields"); err == nil {
		t.Fatal("bad JSON")
	}
	if _, err := readJSONMapFlagOrFile("", "arr.json", "custom-fields"); err == nil {
		t.Fatal("array JSON")
	}
}

func TestWorkitemCreateFileInputsDryRun(t *testing.T) {
	var gotMethods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethods = append(gotMethods, r.Method)
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	sub := append([]byte{0xEF, 0xBB, 0xBF}, []byte("缺陷标题")...)
	desc := append([]byte{0xEF, 0xBB, 0xBF}, []byte("详细说明：已复现")...)
	cf := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"79eb":"生产环境"}`)...)
	if err := os.WriteFile("subject.txt", sub, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("desc.md", desc, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("cf.json", cf, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(config.EnvAccessToken, "test-token-wi-create-file-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-create-file-test")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemCreateCmd,
		"space-id", "type-id", "subject", "subject-file", "assigned-to",
		"description", "description-file", "custom-fields", "custom-fields-file",
		"format-type", "parent-id", "sprint", "labels", "participants", "trackers", "verifier", "versions")
	_ = workitemCreateCmd.Flags().Set("no-defaults", "true")
	t.Cleanup(func() { _ = workitemCreateCmd.Flags().Set("no-defaults", "false") })

	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "space-1",
		"--type-id", "bug-type-1",
		"--assigned-to", "user-1",
		"--subject-file", "subject.txt",
		"--description-file", "desc.md",
		"--custom-fields-file", "cf.json",
		"--no-defaults",
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
	_ = json.Unmarshal(raw, &req)
	body, _ := req["body"].(map[string]any)
	if body["subject"] != "缺陷标题" {
		t.Fatalf("subject=%v", body["subject"])
	}
	if body["description"] != "详细说明：已复现" {
		t.Fatalf("description=%v", body["description"])
	}
	cfv, _ := body["customFieldValues"].(map[string]any)
	if cfv == nil || cfv["79eb"] != "生产环境" {
		t.Fatalf("customFieldValues=%#v", body["customFieldValues"])
	}
	for _, m := range gotMethods {
		if m == http.MethodPost || m == http.MethodPut || m == http.MethodDelete {
			t.Fatalf("mutating during dry-run: %v", gotMethods)
		}
	}
}

func TestWorkitemCreateCustomFieldsMutexCLI(t *testing.T) {
	prevExit := processExit
	var code int
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemCreateCmd,
		"space-id", "type-id", "subject", "subject-file", "assigned-to",
		"description", "description-file", "custom-fields", "custom-fields-file")
	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "s",
		"--type-id", "t",
		"--subject", "title",
		"--assigned-to", "u1",
		"--custom-fields", "{}",
		"--custom-fields-file", "x.json",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected processExit panic")
		}
		if _, ok := r.(exitPanic); !ok {
			panic(r)
		}
		if code != 1 {
			t.Fatalf("exit code=%d", code)
		}
		var env output.Envelope
		if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
			t.Fatalf("stderr: %v / %s", err, stderr.Bytes())
		}
		if env.OK || env.Error == nil {
			t.Fatalf("want ok=false error: %+v", env)
		}
		if !strings.Contains(env.Error.Message, "custom-fields-file") {
			t.Fatalf("message=%q", env.Error.Message)
		}
		_ = stdout // keep capture installed for consistency
	}()
	_ = rootCmd.Execute()
}

func TestWorkitemCreateEmptySubjectFileFails(t *testing.T) {
	prevExit := processExit
	var code int
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("empty-subject.txt", []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(config.EnvAccessToken, "test-token-wi-create-empty-subject-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-create-empty-subject")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, "http://127.0.0.1:9")
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout := withCmdJSONCapture(t)
	resetStringFlags(t, workitemCreateCmd,
		"space-id", "type-id", "subject", "subject-file", "assigned-to",
		"description", "description-file", "custom-fields", "custom-fields-file")
	_ = workitemCreateCmd.Flags().Set("no-defaults", "true")
	t.Cleanup(func() { _ = workitemCreateCmd.Flags().Set("no-defaults", "false") })

	rootCmd.SetArgs([]string{
		"workitem", "create",
		"--space-id", "s",
		"--type-id", "t",
		"--assigned-to", "u1",
		"--subject-file", "empty-subject.txt",
		"--no-defaults",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected processExit panic")
		}
		if _, ok := r.(exitPanic); !ok {
			panic(r)
		}
		if code != 1 {
			t.Fatalf("exit code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
		}
		var env output.Envelope
		if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
			t.Fatalf("stderr: %v / %s", err, stderr.Bytes())
		}
		if env.OK || env.Error == nil {
			t.Fatalf("want ok=false error: %+v", env)
		}
		if !strings.Contains(env.Error.Message, "subject-file") {
			t.Fatalf("message=%q", env.Error.Message)
		}
	}()
	_ = rootCmd.Execute()
}
