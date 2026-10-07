package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v0.16.1", "0.16.1"},
		{"V1.2.3", "1.2.3"},
		{"0.16.1+build", "0.16.1"},
		{"  v0.1.0-rc.1 ", "0.1.0-rc.1"},
	}
	for _, c := range cases {
		if got := NormalizeVersion(c.in); got != c.want {
			t.Fatalf("NormalizeVersion(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.16.1", "0.16.1", 0},
		{"v0.16.0", "0.16.1", -1},
		{"0.17.0", "0.16.9", 1},
		{"0.16.1", "v0.16.1", 0},
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0", "1.0.0-rc.1", 1},
		{"0.9.0", "0.10.0", -1},
		{"0.16", "0.16.0", 0},
		{"0.16.1-dirty", "0.16.1", -1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Fatalf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
	if !NewerAvailable("0.16.0", "0.16.1") {
		t.Fatal("expected newer")
	}
	if NewerAvailable("0.16.1", "0.16.1") {
		t.Fatal("same should not be newer")
	}
}

func TestArchiveNameAndBinary(t *testing.T) {
	if got := ArchiveName("v0.16.1", "linux", "amd64"); got != "yunxiao-cli-0.16.1-linux-amd64.tar.gz" {
		t.Fatalf("linux archive: %s", got)
	}
	if got := ArchiveName("0.16.1", "windows", "amd64"); got != "yunxiao-cli-0.16.1-windows-amd64.zip" {
		t.Fatalf("windows archive: %s", got)
	}
	if got := ArchiveName("0.16.1", "darwin", "arm64"); got != "yunxiao-cli-0.16.1-darwin-arm64.tar.gz" {
		t.Fatalf("darwin archive: %s", got)
	}
	if BinaryName("windows") != "yunxiao.exe" {
		t.Fatal("windows binary name")
	}
	if BinaryName("linux") != "yunxiao" {
		t.Fatal("unix binary name")
	}
}

func TestPlatformArch(t *testing.T) {
	if _, err := Platform("plan9"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Arch("riscv64"); err == nil {
		t.Fatal("expected error")
	}
	p, a, err := CurrentPlatformArch()
	if err != nil {
		// Current box should be linux/amd64 or similar supported.
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			t.Fatalf("CurrentPlatformArch: %v", err)
		}
		return
	}
	if p == "" || a == "" {
		t.Fatal("empty platform/arch")
	}
}

func TestChecksumMap(t *testing.T) {
	m := ChecksumMap(`
33a7ba0b8188d9f94bb627c2015813ba2e5e5f6ae181f4a5828d6e7c6a67f530  yunxiao-cli-0.16.1-darwin-amd64.tar.gz
deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef *yunxiao-cli-0.16.1-linux-amd64.tar.gz
not-a-hash  skip-me.tar.gz
`)
	if m["yunxiao-cli-0.16.1-darwin-amd64.tar.gz"] != "33a7ba0b8188d9f94bb627c2015813ba2e5e5f6ae181f4a5828d6e7c6a67f530" {
		t.Fatalf("darwin hash: %#v", m)
	}
	if m["yunxiao-cli-0.16.1-linux-amd64.tar.gz"] != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Fatalf("linux hash: %#v", m)
	}
	if _, ok := m["skip-me.tar.gz"]; ok {
		t.Fatal("should skip short hash")
	}
}

func TestFindAsset(t *testing.T) {
	rel := &Release{
		TagName: "v0.16.1",
		Assets: []Asset{
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
			{Name: "yunxiao-cli-0.16.1-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/a.tar.gz"},
		},
	}
	a, err := FindAsset(rel, "yunxiao-cli-0.16.1-linux-amd64.tar.gz")
	if err != nil || a.BrowserDownloadURL == "" {
		t.Fatalf("FindAsset: %v %#v", err, a)
	}
	if _, err := FindAsset(rel, "missing.zip"); err == nil {
		t.Fatal("expected missing asset error")
	}
}

func TestSelectPlatformAsset(t *testing.T) {
	name, err := SelectPlatformAsset("0.16.1", "linux", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if name != "yunxiao-cli-0.16.1-linux-arm64.tar.gz" {
		t.Fatalf("got %s", name)
	}
	if _, err := SelectPlatformAsset("0.16.1", "solaris", "amd64"); err == nil {
		t.Fatal("expected error")
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "yunxiao")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	newer := filepath.Join(dir, "fresh")
	if err := os.WriteFile(newer, []byte("new-content"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceExecutable(target, newer); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new-content" {
		t.Fatalf("got %q", b)
	}
}

func TestExtractTarGzBinary(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	if err := writeTarGzWithFile(archive, "yunxiao-cli-0.1.0-linux-amd64/yunxiao", []byte("#!/bin/sh\necho ok\n")); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "out")
	got, err := ExtractBinary(archive, outDir, "linux")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "#!/bin/sh\necho ok\n" {
		t.Fatalf("content %q", b)
	}
}

func writeTarGzWithFile(archivePath, name string, content []byte) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	hdr := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = tw.Write(content)
	return err
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := FileSHA256(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyChecksum(p, sum); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChecksum(p, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestFormatPair(t *testing.T) {
	s := FormatPair("0.16.0", "0.16.1")
	if !strings.Contains(s, "0.16.0") || !strings.Contains(s, "0.16.1") {
		t.Fatalf("%q", s)
	}
}

func TestGithubRepoDefault(t *testing.T) {
	t.Setenv(EnvGithubRepo, "")
	if GithubRepo() != DefaultRepo {
		t.Fatalf("got %s", GithubRepo())
	}
	t.Setenv(EnvGithubRepo, "acme/yunxiao-cli")
	if GithubRepo() != "acme/yunxiao-cli" {
		t.Fatalf("got %s", GithubRepo())
	}
}

func TestExtractAndInstallSidecarsTarGz(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	files := map[string]string{
		"yunxiao":                        "bin\n",
		"skills/yunxiao-shared/SKILL.md": "# shared\n",
		"profiles/zhiyi.example.json":    "{\"ok\":true}\n",
		"README.md":                      "docs\n",
	}
	if err := writeTarGzFiles(archive, files); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "out")
	found, err := ExtractSidecars(archive, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0] != "skills" || found[1] != "profiles" {
		t.Fatalf("found=%v", found)
	}
	skillMD := filepath.Join(outDir, "skills", "yunxiao-shared", "SKILL.md")
	b, err := os.ReadFile(skillMD)
	if err != nil || string(b) != "# shared\n" {
		t.Fatalf("skill content: %v %q", err, b)
	}
	if _, err := ExtractBinary(archive, filepath.Join(dir, "binout"), "linux"); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "install")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(target, "skills", "old", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	installed, err := InstallSidecars(target, outDir, found)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 2 {
		t.Fatalf("installed=%v", installed)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale skill should be gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "yunxiao-shared", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "profiles", "zhiyi.example.json")); err != nil {
		t.Fatal(err)
	}
}

func TestExtractSidecarsZip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.zip")
	if err := writeZipFiles(archive, map[string]string{
		"yunxiao.exe":                    "MZ",
		"skills/yunxiao-codeup/SKILL.md": "# codeup\n",
		"profiles/play.example.json":     "{}\n",
	}); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "out")
	found, err := ExtractSidecars(archive, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("found=%v", found)
	}
	b, err := os.ReadFile(filepath.Join(outDir, "skills", "yunxiao-codeup", "SKILL.md"))
	if err != nil || string(b) != "# codeup\n" {
		t.Fatalf("got %v %q", err, b)
	}
}

func TestExtractSidecarsRejectsZipSlip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil.tar.gz")
	if err := writeTarGzFiles(archive, map[string]string{
		"skills/../../escape.txt": "nope\n",
	}); err != nil {
		t.Fatal(err)
	}
	_, err := ExtractSidecars(archive, filepath.Join(dir, "out"))
	if err == nil {
		t.Fatal("expected zip-slip error")
	}
}

func TestExtractSidecarsMissingOK(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "binonly.tar.gz")
	if err := writeTarGzFiles(archive, map[string]string{"yunxiao": "x\n"}); err != nil {
		t.Fatal(err)
	}
	found, err := ExtractSidecars(archive, filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("found=%v", found)
	}
}

func writeTarGzFiles(archivePath string, files map[string]string) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	for name, content := range files {
		b := []byte(content)
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(b))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFiles(archivePath string, files map[string]string) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return err
		}
	}
	return nil
}
