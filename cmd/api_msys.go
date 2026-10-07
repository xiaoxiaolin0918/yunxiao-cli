package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MSYS path mangling (#117): git-bash / MSYS2 / Cygwin rewrite command-line
// arguments that start with "/" into Windows paths BEFORE the child process is
// exec'd, so
//
//	yunxiao api GET "/oapi/v1/platform/user"
//
// arrives as argv "C:/Program Files/Git/oapi/v1/platform/user" (the MSYS
// install root is prepended; the root varies per machine). The CLI then
// requests a broken URL and the gateway answers with an HTML landing page.
//
// unmangleAPIPathArg detects that shape and restores the original /oapi/...
// path, printing a `note:` line on stderr. Detection is intentionally strict
// (better to leave the argument untouched than to "fix" a legit one): a
// candidate is restored only when
//
//   - the argument looks like "<drive>:/<msys-root>/oapi/..." AND the part
//     before /oapi/ exists on disk and looks like an MSYS install (usr/bin or
//     bin/bash.exe), OR
//   - no candidate root probes valid, but an MSYS environment signal is
//     present (MSYSTEM / MSYS non-empty — git-bash always exports
//     MSYSTEM=MINGW64/MSYS/UCRT64...) and the argument is drive-shaped with a
//     /oapi/ segment.
//
// The git-bash double-slash escape "//oapi/..." (leading // bypasses the
// conversion but keeps both slashes) is collapsed to "/oapi/...".
//
// YUNXIAO_API_NO_UNMANGLE=1 disables all rewriting (restore + collapse).

// envAPIUnmangleOff disables MSYS path restore/collapse for `yunxiao api`.
const envAPIUnmangleOff = "YUNXIAO_API_NO_UNMANGLE"

// msysAPIPathPrefix is the only confirmed Yunxiao OpenAPI root; restores are
// limited to it (never invent endpoints).
const msysAPIPathPrefix = "/oapi/"

// msysNoteOut receives the one-line stderr note when a path was restored.
// Tests redirect it.
var msysNoteOut io.Writer = os.Stderr

// unmangleAPIPathArg normalizes the <path> argument of `yunxiao api`.
// Returns the (possibly unchanged) path and a stderr note line ("" when
// nothing changed).
func unmangleAPIPathArg(arg string) (string, string) {
	if strings.TrimSpace(arg) == "" || apiUnmangleDisabled() {
		return arg, ""
	}
	// git-bash escape: "//oapi/..." reaches argv untouched but keeps the extra
	// leading slash. No legit OpenAPI path starts with "//".
	if strings.HasPrefix(arg, "//") && strings.HasPrefix(arg[1:], msysAPIPathPrefix) {
		restored := arg[1:]
		return restored, msysUnmangleNote(arg, restored)
	}
	restored, ok := restoreMSYSMangledAPIPath(arg, msysEnvSignal(), probeMSYSRootOnDisk)
	if !ok {
		return arg, ""
	}
	return restored, msysUnmangleNote(arg, restored)
}

// restoreMSYSMangledAPIPath restores "<drive>:/<msys-root>/oapi/..." to
// "/oapi/...". envSignal reports an MSYS-like environment; probeRoot reports
// whether a candidate MSYS root looks like an MSYS install on disk.
//
// Candidates are the /oapi/ occurrences left to right, so a probe-validated
// root that itself contains an "oapi" directory still restores correctly.
// ok=false means "not confident — leave unchanged".
func restoreMSYSMangledAPIPath(arg string, envSignal bool, probeRoot func(string) bool) (string, bool) {
	if probeRoot == nil {
		probeRoot = func(string) bool { return false }
	}
	for _, idx := range allIndexes(arg, msysAPIPathPrefix) {
		if idx <= 0 {
			continue
		}
		root := arg[:idx]
		if !isWindowsDrivePath(root) {
			continue
		}
		if probeRoot(root) {
			return arg[idx:], true
		}
	}
	// No disk-provable root: accept only with an explicit MSYS env signal.
	if envSignal {
		for _, idx := range allIndexes(arg, msysAPIPathPrefix) {
			if idx > 0 && isWindowsDrivePath(arg[:idx]) {
				return arg[idx:], true
			}
		}
	}
	return "", false
}

// msysUnmangleNote is the stderr note printed on restore (one line).
func msysUnmangleNote(from, to string) string {
	return fmt.Sprintf(
		"note: restored MSYS-mangled api path %q -> %q (git-bash/MSYS rewrites leading-/ args to Windows paths); "+
			"alternatives: MSYS_NO_PATHCONV=1, MSYS2_ARG_CONV_EXCL='*', or a double slash %q; disable with %s=1",
		from, to, "//oapi/...", envAPIUnmangleOff)
}

// apiUnmangleDisabled: YUNXIAO_API_NO_UNMANGLE unset/0/empty keeps the restore.
func apiUnmangleDisabled() bool {
	v := strings.TrimSpace(os.Getenv(envAPIUnmangleOff))
	return v != "" && v != "0"
}

// msysEnvSignal: git-bash exports MSYSTEM (MINGW64/MSYS/UCRT64/CLANG64...);
// MSYS2 also understands MSYS. Either present means we are under an
// MSYS-derived shell (or one was in the parent env).
func msysEnvSignal() bool {
	return os.Getenv("MSYSTEM") != "" || os.Getenv("MSYS") != ""
}

// probeMSYSRootOnDisk reports whether root looks like an MSYS/git-bash install
// (has usr/bin or bin/bash.exe). Cheap stat-only probe, called only for
// drive-shaped candidates containing /oapi/.
func probeMSYSRootOnDisk(root string) bool {
	if root == "" {
		return false
	}
	if fi, err := os.Stat(filepath.Join(root, "usr", "bin")); err == nil && fi.IsDir() {
		return true
	}
	if fi, err := os.Stat(filepath.Join(root, "bin", "bash.exe")); err == nil && !fi.IsDir() {
		return true
	}
	return false
}

// isWindowsDrivePath reports whether p is an absolute Windows drive path
// ("C:/..." / "C:\\..."; MSYS emits forward slashes). UNC paths are not
// MSYS-mangle output and are rejected.
func isWindowsDrivePath(p string) bool {
	if len(p) < 3 {
		return false
	}
	drive := p[0]
	if !((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) {
		return false
	}
	return p[1] == ':' && (p[2] == '/' || p[2] == '\\')
}

func allIndexes(s, sub string) []int {
	var out []int
	for i := 0; i+len(sub) <= len(s); {
		j := strings.Index(s[i:], sub)
		if j < 0 {
			break
		}
		out = append(out, i+j)
		i += j + 1
	}
	return out
}

// msysHint is the shared error hint for requests whose URL still contains an
// MSYS-mangled drive prefix (restore skipped or older CLI).
const msysHint = "final request path contains a Windows drive prefix: git-bash/MSYS rewrote the leading-/ argument into a Windows path before the CLI saw it. Fix: upgrade yunxiao-cli (auto-restores /oapi/... with a stderr note), set MSYS_NO_PATHCONV=1 or MSYS2_ARG_CONV_EXCL='*', or pass a double slash (//oapi/v1/...)"

// htmlResponseHint is the hint for 2xx HTML (or other non-JSON) responses that
// fail JSON decoding — usually a gateway landing/auth page for a wrong path.
const htmlResponseHint = "server returned a non-JSON body (likely an HTML landing/auth page): the path or API base URL is probably wrong — check error.details.url; on Windows git-bash also mind MSYS path conversion (MSYS_NO_PATHCONV=1, MSYS2_ARG_CONV_EXCL='*', or //oapi/v1/...)"

// hasWindowsDriveInURLPath reports whether the URL path contains a drive-letter
// segment like "/C:/..." (an MSYS-mangled path that leaked into the request).
func hasWindowsDriveInURLPath(u string) bool {
	if i := strings.IndexByte(u, '?'); i >= 0 {
		u = u[:i]
	}
	for i := 0; i+3 < len(u); i++ {
		if u[i] != '/' {
			continue
		}
		drive := u[i+1]
		if ((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) && u[i+2] == ':' && (u[i+3] == '/' || u[i+3] == '\\') {
			return true
		}
	}
	return false
}
