package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ExecutablePath resolves the running binary (symlink-evaluated when possible).
func ExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err2 := filepath.EvalSymlinks(path); err2 == nil {
		path = resolved
	}
	return path, nil
}

// DownloadFile downloads url to destPath (overwrites).
func DownloadFile(ctx context.Context, client HTTPDoer, url, destPath string) error {
	if client == nil {
		client = DefaultHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "yunxiao-cli-self-update")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 256<<20)); err != nil {
		return err
	}
	return nil
}

// DownloadFirstOK tries candidates in order.
func DownloadFirstOK(ctx context.Context, client HTTPDoer, urls []string, destPath string) (used string, err error) {
	var errs []string
	for _, u := range urls {
		if u == "" {
			continue
		}
		if err := DownloadFile(ctx, client, u, destPath); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", u, err))
			_ = os.Remove(destPath)
			continue
		}
		fi, statErr := os.Stat(destPath)
		if statErr != nil || fi.Size() == 0 {
			errs = append(errs, fmt.Sprintf("%s: empty download", u))
			_ = os.Remove(destPath)
			continue
		}
		return u, nil
	}
	if len(errs) == 0 {
		return "", fmt.Errorf("no download URLs")
	}
	return "", fmt.Errorf("all downloads failed:\n  %s", strings.Join(errs, "\n  "))
}

// FileSHA256 returns hex sha256 of path.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum checks path against expected hex digest.
func VerifyChecksum(path, expectedHex string) error {
	expectedHex = strings.ToLower(strings.TrimSpace(expectedHex))
	if len(expectedHex) != 64 {
		return fmt.Errorf("invalid expected checksum")
	}
	actual, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if actual != expectedHex {
		return fmt.Errorf("checksum mismatch: expected %s got %s", expectedHex, actual)
	}
	return nil
}

// ExtractBinary extracts BinaryName(platform) from archivePath into destDir; returns path.
func ExtractBinary(archivePath, destDir, platform string) (string, error) {
	want := BinaryName(platform)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZipBinary(archivePath, destDir, want)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGzBinary(archivePath, destDir, want)
	default:
		return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
}

func extractTarGzBinary(archivePath, destDir, want string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != want {
			continue
		}
		dest := filepath.Join(destDir, want)
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, io.LimitReader(tr, 256<<20)); err != nil {
			out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return dest, nil
	}
	return "", fmt.Errorf("binary %s not found in archive", want)
}

func extractZipBinary(archivePath, destDir, want string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for _, zf := range r.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(zf.Name) != want {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return "", err
		}
		dest := filepath.Join(destDir, want)
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, copyErr := io.Copy(out, io.LimitReader(rc, 256<<20))
		rc.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return dest, nil
	}
	return "", fmt.Errorf("binary %s not found in archive", want)
}

// SidecarDirs are release-archive directories shipped next to the binary
// (see .github/workflows/release.yml). `yunxiao update` refreshes them
// beside the executable so `skills list|install` and on-disk example
// profiles stay in sync with the binary version.
var SidecarDirs = []string{"skills", "profiles"}

// ExtractSidecars extracts SidecarDirs trees from a release archive into
// destDir. Missing sidecars are skipped (returns the ones found). Paths
// with ".." are rejected (zip-slip).
func ExtractSidecars(archivePath, destDir string) (found []string, err error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZipSidecars(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGzSidecars(archivePath, destDir)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
}

func sidecarRoot(rel string) (root string, ok bool) {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	rel = strings.TrimPrefix(rel, "/")
	for _, name := range SidecarDirs {
		if rel == name || strings.HasPrefix(rel, name+"/") {
			return name, true
		}
	}
	return "", false
}

func safeSidecarDest(destDir, rel string) (string, error) {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", fmt.Errorf("refusing unsafe archive path %q", rel)
	}
	if _, ok := sidecarRoot(rel); !ok {
		return "", fmt.Errorf("not a sidecar path: %q", rel)
	}
	dest := filepath.Join(destDir, filepath.FromSlash(rel))
	cleanDest := filepath.Clean(dest)
	cleanRoot := filepath.Clean(destDir)
	sep := string(os.PathSeparator)
	if cleanDest != cleanRoot && !strings.HasPrefix(cleanDest, cleanRoot+sep) {
		return "", fmt.Errorf("path escapes dest: %q", rel)
	}
	return cleanDest, nil
}

func markFound(found map[string]struct{}, rel string) {
	if root, ok := sidecarRoot(rel); ok {
		found[root] = struct{}{}
	}
}

func foundList(found map[string]struct{}) []string {
	out := make([]string, 0, len(SidecarDirs))
	for _, name := range SidecarDirs {
		if _, ok := found[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

func writeSidecarFile(dest string, r io.Reader, mode int64) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	perm := os.FileMode(0o644)
	if mode&0o111 != 0 {
		perm = 0o755
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, io.LimitReader(r, 64<<20))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func extractTarGzSidecars(archivePath, destDir string) ([]string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	found := map[string]struct{}{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
		if _, ok := sidecarRoot(rel); !ok {
			continue
		}
		if hdr.Typeflag == tar.TypeDir {
			dest, err := safeSidecarDest(destDir, rel)
			if err != nil {
				return nil, err
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return nil, err
			}
			markFound(found, rel)
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		dest, err := safeSidecarDest(destDir, rel)
		if err != nil {
			return nil, err
		}
		if err := writeSidecarFile(dest, tr, hdr.Mode); err != nil {
			return nil, err
		}
		markFound(found, rel)
	}
	return foundList(found), nil
}

func extractZipSidecars(archivePath, destDir string) ([]string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	found := map[string]struct{}{}
	for _, zf := range r.File {
		rel := strings.TrimPrefix(filepath.ToSlash(zf.Name), "./")
		if _, ok := sidecarRoot(rel); !ok {
			continue
		}
		if strings.HasSuffix(rel, "/") || zf.FileInfo().IsDir() {
			dest, err := safeSidecarDest(destDir, strings.TrimSuffix(rel, "/"))
			if err != nil {
				return nil, err
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return nil, err
			}
			markFound(found, rel)
			continue
		}
		dest, err := safeSidecarDest(destDir, rel)
		if err != nil {
			return nil, err
		}
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		err = writeSidecarFile(dest, rc, int64(zf.Mode()))
		rc.Close()
		if err != nil {
			return nil, err
		}
		markFound(found, rel)
	}
	return foundList(found), nil
}

// InstallSidecars copies extracted sidecar directories from sourceDir into
// targetDir (typically the directory of the yunxiao binary), replacing any
// existing trees. names should come from ExtractSidecars; unknown/missing
// names are skipped.
func InstallSidecars(targetDir, sourceDir string, names []string) (installed []string, err error) {
	targetDir = filepath.Clean(targetDir)
	sourceDir = filepath.Clean(sourceDir)
	for _, name := range names {
		if name != "skills" && name != "profiles" {
			continue
		}
		src := filepath.Join(sourceDir, name)
		st, err := os.Stat(src)
		if err != nil || !st.IsDir() {
			continue
		}
		dst := filepath.Join(targetDir, name)
		if err := os.RemoveAll(dst); err != nil {
			return installed, fmt.Errorf("remove old %s: %w", dst, err)
		}
		if err := copyDir(src, dst); err != nil {
			return installed, fmt.Errorf("install %s: %w", name, err)
		}
		installed = append(installed, name)
	}
	return installed, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFileMode(path, out, info.Mode())
	})
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	perm := mode.Perm()
	if perm == 0 {
		perm = 0o644
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// ReplaceExecutable writes newBinary over targetPath safely:
// write beside as .new, rename target → .old, rename .new → target.
// On Windows the running image can be renamed; .old may remain until reboot.
func ReplaceExecutable(targetPath, newBinary string) error {
	targetPath = filepath.Clean(targetPath)
	newBinary = filepath.Clean(newBinary)
	dir := filepath.Dir(targetPath)
	base := filepath.Base(targetPath)
	staging := filepath.Join(dir, base+".new")
	backup := filepath.Join(dir, base+".old")

	// Copy to staging in the same directory (rename requires same volume).
	if err := copyFile(newBinary, staging); err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(staging, 0o755); err != nil {
			_ = os.Remove(staging)
			return err
		}
	}

	_ = os.Remove(backup) // best-effort clear previous leftover
	if err := os.Rename(targetPath, backup); err != nil {
		_ = os.Remove(staging)
		return fmt.Errorf("rename current → .old: %w (on Windows, close other yunxiao processes and retry)", err)
	}
	if err := os.Rename(staging, targetPath); err != nil {
		// try rollback
		_ = os.Rename(backup, targetPath)
		_ = os.Remove(staging)
		return fmt.Errorf("rename .new → current: %w", err)
	}
	_ = os.Remove(backup) // may fail on Windows while process still maps old image
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// VerifyBinaryVersion runs `path --version` and checks it contains wantVersion.
func VerifyBinaryVersion(path, wantVersion string) error {
	wantVersion = NormalizeVersion(wantVersion)
	cmd := exec.Command(path, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run %s --version: %w (%s)", path, err, truncate(string(out), 200))
	}
	if !strings.Contains(string(out), wantVersion) {
		return fmt.Errorf("updated binary reports %q, expected to contain %s", strings.TrimSpace(string(out)), wantVersion)
	}
	return nil
}

// FetchChecksums tries to download checksums.txt from the release assets.
func FetchChecksums(ctx context.Context, client HTTPDoer, rel *Release) (map[string]string, error) {
	asset, err := FindAsset(rel, "checksums.txt")
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "yunxiao-checksums-*.txt")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)
	urls := DownloadURLCandidates(asset.BrowserDownloadURL)
	if _, err := DownloadFirstOK(ctx, client, urls, path); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ChecksumMap(string(b)), nil
}
