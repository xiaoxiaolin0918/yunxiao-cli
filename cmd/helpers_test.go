package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func TestRequireFlags(t *testing.T) {
	if err := requireFlags("id", "x", "content", "y"); err != nil {
		t.Fatal(err)
	}
	if err := requireFlags("id", "", "content", "y"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("1, 2,3")
	if len(got) != 3 || got[0] != "1" || got[1] != "2" || got[2] != "3" {
		t.Fatalf("%v", got)
	}
	if len(splitCSV("")) != 0 {
		t.Fatal()
	}
}

func TestReadContentInput(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("note.txt", []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readContentInput("", "note.txt")
	if err != nil || got != "hello" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := readContentInput("x", "note.txt"); err == nil {
		t.Fatal("both")
	}
	abs := filepath.Join(dir, "note.txt")
	gotAbs, err := readContentInput("", abs)
	if err != nil || gotAbs != "hello" {
		t.Fatalf("abs should work: %q %v", gotAbs, err)
	}
	if _, err := readContentInput("", "../note.txt"); err == nil {
		t.Fatal(".. should fail")
	}
}

func TestResolveContentFilePath(t *testing.T) {
	if _, err := resolveContentFilePath("../x"); err == nil {
		t.Fatal("..")
	}
	got, err := resolveContentFilePath("ok.txt")
	if err != nil || got != "ok.txt" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = resolveContentFilePath("/tmp/abs.txt")
	if err != nil || got != "/tmp/abs.txt" {
		t.Fatalf("abs: %q %v", got, err)
	}
	got, err = resolveContentFilePath("C:/Users/a/b.txt")
	if err != nil || got == "" {
		t.Fatalf("windows path: %q %v", got, err)
	}
}

func TestAPIErrorHint(t *testing.T) {
	h := apiErrorHint(&client.APIError{Body: `{"errorMsg":"未启用此字段【迭代】"}`})
	if !strings.Contains(h, "omit --sprint") {
		t.Fatalf("hint=%q", h)
	}
	h2 := apiErrorHint(&client.APIError{Body: "未启用此字段【所属模块,所属环境】"})
	if !strings.Contains(h2, "--minimal") {
		t.Fatalf("hint2=%q", h2)
	}
	h3 := apiErrorHint(&client.APIError{Body: "取消原因必填"})
	if !strings.Contains(h3, "--cancel-reason") {
		t.Fatalf("hint3=%q", h3)
	}
	h4 := apiErrorHint(&client.APIError{Status: 403, Body: `{"errorMessage":"非高级版组织，不支持此功能"}`})
	if !strings.Contains(h4, "Advanced-edition") || !strings.Contains(h4, "project list") {
		t.Fatalf("programs hint=%q", h4)
	}
}

func TestRunMutatingHighRiskGate(t *testing.T) {
	err := runMutating("codeup files create", risk.HighRiskWrite, false, false, nil, func() error {
		t.Fatal("should not exec")
		return nil
	})
	if !risk.IsConfirmation(err) {
		t.Fatalf("%v", err)
	}
	ran := false
	err = runMutating("codeup files create", risk.HighRiskWrite, true, false, map[string]any{"ok": true}, func() error {
		ran = true
		return nil
	})
	if err != nil || ran {
		t.Fatalf("dry-run should not exec: %v ran=%v", err, ran)
	}
}

func TestParseJSONMap(t *testing.T) {
	m, err := parseJSONMap(`{"a":"1","b":2}`)
	if err != nil || m["a"] != "1" {
		t.Fatalf("%v %v", m, err)
	}
	if _, err := parseJSONMap(`[]`); err == nil {
		t.Fatal("array should fail")
	}
	m, err = parseJSONMap("")
	if err != nil || m != nil {
		t.Fatal()
	}
}

func TestAssertRelativePath(t *testing.T) {
	if err := assertRelativePath("ok.txt"); err != nil {
		t.Fatal(err)
	}
	if err := assertRelativePath("/etc/passwd"); err == nil {
		t.Fatal("abs")
	}
	if err := assertRelativePath("../x"); err == nil {
		t.Fatal("..")
	}
}

func TestReadContentOrFile(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("p.yaml", []byte("a: 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readContentOrFile("", "p.yaml")
	if err != nil || got != "a: 1" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := readContentOrFile("x", "p.yaml"); err == nil {
		t.Fatal("both")
	}
}

func TestLoadJSONBodyFromFlags(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("body.json", []byte(`{"x":1,"y":"z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("bad.json", []byte(`{not-json`), 0o600); err != nil {
		t.Fatal(err)
	}

	// both empty
	got, err := loadJSONBodyFromFlags("", "")
	if err != nil || got != nil {
		t.Fatalf("empty: %v %v", got, err)
	}

	// inline JSON
	got, err = loadJSONBodyFromFlags(`{"a":true}`, "")
	if err != nil {
		t.Fatal(err)
	}
	m := asStringMap(got)
	if m == nil || m["a"] != true {
		t.Fatalf("inline: %#v", got)
	}

	// @file
	got, err = loadJSONBodyFromFlags("@body.json", "")
	if err != nil {
		t.Fatal(err)
	}
	m = asStringMap(got)
	if m == nil || m["x"].(float64) != 1 || m["y"] != "z" {
		t.Fatalf("@file: %#v", got)
	}

	// --data-file
	got, err = loadJSONBodyFromFlags("", "body.json")
	if err != nil {
		t.Fatal(err)
	}
	m = asStringMap(got)
	if m == nil || m["y"] != "z" {
		t.Fatalf("data-file: %#v", got)
	}

	// both set → mutual exclusion
	if _, err := loadJSONBodyFromFlags(`{}`, "body.json"); err == nil {
		t.Fatal("expected mutual exclusion error")
	}

	// bad JSON
	if _, err := loadJSONBodyFromFlags(`{bad`, ""); err == nil {
		t.Fatal("expected bad JSON error")
	}
	if _, err := loadJSONBodyFromFlags("", "bad.json"); err == nil {
		t.Fatal("expected bad data-file JSON error")
	}

	// parent-path rejected (relative escape)
	if _, err := loadJSONBodyFromFlags("", "../body.json"); err == nil {
		t.Fatal("expected parent-path reject for data-file")
	}
	if _, err := loadJSONBodyFromFlags("@../body.json", ""); err == nil {
		t.Fatal("expected parent-path reject for @file")
	}
}

func TestCoalesceSpaceID(t *testing.T) {
	got, err := coalesceSpaceID("flag-1", "profile-1")
	if err != nil || got != "flag-1" {
		t.Fatalf("flag wins: %q %v", got, err)
	}
	got, err = coalesceSpaceID("  ", "profile-1")
	if err != nil || got != "profile-1" {
		t.Fatalf("profile fallback: %q %v", got, err)
	}
	_, err = coalesceSpaceID("", "")
	if err == nil || !strings.Contains(err.Error(), "--space-id") || !strings.Contains(err.Error(), "+onboard") {
		t.Fatalf("want usage error, got %v", err)
	}
}


func TestStripUTF8BOM(t *testing.T) {
	got := stripUTF8BOM([]byte{0xEF, 0xBB, 0xBF, 'a', 'b'})
	if string(got) != "ab" {
		t.Fatalf("%q", got)
	}
	got = stripUTF8BOM([]byte("plain"))
	if string(got) != "plain" {
		t.Fatalf("%q", got)
	}
}

func TestReadContentInputStripsBOM(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	payload := append([]byte{0xEF, 0xBB, 0xBF}, []byte("你好")...)
	if err := os.WriteFile("bom.txt", payload, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readContentInput("", "bom.txt")
	if err != nil || got != "你好" {
		t.Fatalf("%q %v", got, err)
	}
}
