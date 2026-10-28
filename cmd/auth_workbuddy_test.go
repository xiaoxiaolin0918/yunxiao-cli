package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// A3: non-TTY with no --browser/--token must fail immediately (no PAT stdin).
func TestAuthLoginNonTTYRequiresFlag(t *testing.T) {
	prev := stdinIsInteractive
	stdinIsInteractive = func() bool { return false }
	t.Cleanup(func() { stdinIsInteractive = prev })

	_ = authLoginCmd.Flags().Set("browser", "false")
	_ = authLoginCmd.Flags().Set("dry-run", "false")
	_ = authLoginCmd.Flags().Set("token", "")
	t.Cleanup(func() {
		_ = authLoginCmd.Flags().Set("browser", "false")
		_ = authLoginCmd.Flags().Set("dry-run", "false")
		_ = authLoginCmd.Flags().Set("token", "")
	})

	stdout, stderr, code := runRootForOAuthExpiry(t, "auth", "login")
	if code == 0 {
		t.Fatalf("want non-zero, stdout=%s stderr=%s", stdout, stderr)
	}
	blob := stdout + stderr
	if !strings.Contains(blob, "--browser") || !strings.Contains(blob, "--token") {
		t.Fatalf("want hint for --browser/--token, got %s", blob)
	}
	if strings.Contains(blob, "Paste Yunxiao personal access token") {
		t.Fatalf("must not prompt for PAT on non-TTY: %s", blob)
	}
}

// A3: runPATLogin on non-TTY with empty token must Fail (no stdin ReadString).
func TestRunPATLoginNonTTYEmptyTokenNoPrompt(t *testing.T) {
	prev := stdinIsInteractive
	stdinIsInteractive = func() bool { return false }
	t.Cleanup(func() { stdinIsInteractive = prev })

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, stderr, code := func() (string, string, int) {
		var out, errb bytes.Buffer
		prevOut, prevE := output.Stdout, output.Stderr
		output.Stdout, output.Stderr = &out, &errb
		defer func() { output.Stdout, output.Stderr = prevOut, prevE }()
		output.Format = "json"
		prevExit := processExit
		c := 0
		processExit = func(code int) { c = code; panic(exitPanic{code: code}) }
		defer func() { processExit = prevExit }()
		func() {
			defer func() {
				if r := recover(); r != nil {
					if _, ok := r.(exitPanic); !ok {
						panic(r)
					}
				}
			}()
			handleErr(runPATLogin(authLoginCmd, "", "https://openapi-rdc.aliyuncs.com", "", ""))
		}()
		return out.String(), errb.String(), c
	}()
	if code == 0 {
		t.Fatalf("want non-zero exit, stdout=%s stderr=%s", stdout, stderr)
	}
	combined := stdout + stderr
	if strings.Contains(combined, "Paste Yunxiao personal access token") {
		t.Fatalf("must not print PAT prompt on non-TTY: %s", combined)
	}
	if !strings.Contains(combined, "--token") && !strings.Contains(combined, "--browser") {
		t.Fatalf("want hint mentioning --token or --browser, got %s", combined)
	}
}

// A1 helper: bare auth URL line is whitespace-delimited, not JSON-quoted.
func TestPrintBareAuthURL(t *testing.T) {
	var buf bytes.Buffer
	prev := authURLOut
	authURLOut = &buf
	t.Cleanup(func() { authURLOut = prev })

	u := "https://account.example.test/authorize?client_id=cid&redirect_uri=http%3A%2F%2F127.0.0.1%3A9%2Fcallback&code_challenge=abc"
	printBareAuthURL(u)
	got := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(got), "https://") {
		t.Fatalf("want bare https line, got %q", got)
	}
	if strings.Contains(got, `"`) || strings.Contains(got, `\u0026`) {
		t.Fatalf("must not quote/escape URL: %q", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 1 || lines[0] != u {
		t.Fatalf("want single bare URL line, got %#v", lines)
	}
}