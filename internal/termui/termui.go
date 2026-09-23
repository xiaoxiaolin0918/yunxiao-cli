// Package termui gates fancy terminal control (clear-screen / cursor) for agent captures.
//
// Issue #76: some hosts (notably Windows Git Bash + agent capture) observe ESC[H ESC[2J ESC[3J
// on the tty device even when stdout/stderr are redirected. yunxiao itself does not emit those
// sequences today, but we still:
//   - honor YUNXIAO_NO_TUI=1, NO_COLOR, TERM=dumb
//   - treat non-TTY stdout as non-interactive
//   - provide WriteTTY / StripClearScreen so any future TUI path can no-op or sanitize
package termui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync/atomic"
)

const EnvNoTUI = "YUNXIAO_NO_TUI"

var (
	// forcedDisabled is set by Configure when env/TTY says no TUI.
	forcedDisabled atomic.Bool
	// stdoutIsTerminal overridable in tests.
	stdoutIsTerminal = func() bool {
		fi, err := os.Stdout.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
)

// clearScreenCSI matches common full-clear sequences (cursor home + erase + scrollback).
var clearScreenCSI = [][]byte{
	[]byte("\x1b[H\x1b[2J\x1b[3J"),
	[]byte("\x1b[2J"),
	[]byte("\x1b[3J"),
	[]byte("\x1b[H"),
	[]byte("\x1b[0;0H"),
}

// Disabled reports whether TUI / tty control should be off.
func Disabled() bool {
	if forcedDisabled.Load() {
		return true
	}
	return envDisables()
}

// Enabled is the inverse of Disabled.
func Enabled() bool { return !Disabled() }

func envDisables() bool {
	if truthy(os.Getenv(EnvNoTUI)) {
		return true
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		// NO_COLOR present (any value, including empty) disables — https://no-color.org/
		return true
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return true
	}
	return false
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Configure applies non-interactive policy early in PersistentPreRun.
// When disabled, forces NO_COLOR=1 so downstream libs stay quiet.
func Configure() {
	disable := envDisables() || !stdoutIsTerminal()
	forcedDisabled.Store(disable)
	if disable {
		if _, ok := os.LookupEnv("NO_COLOR"); !ok {
			_ = os.Setenv("NO_COLOR", "1")
		}
	}
}

// WriteTTY writes b to w only when TUI is enabled; otherwise drops clear-screen bytes
// and returns (0, nil) for pure clear payloads (or writes stripped content).
func WriteTTY(w io.Writer, b []byte) (int, error) {
	if w == nil {
		return 0, nil
	}
	if Disabled() {
		stripped := StripClearScreen(b)
		if len(stripped) == 0 {
			return 0, nil
		}
		return w.Write(stripped)
	}
	return w.Write(b)
}

// StripClearScreen removes common clear-screen / cursor-home CSI sequences.
func StripClearScreen(b []byte) []byte {
	out := b
	for _, seq := range clearScreenCSI {
		out = bytes.ReplaceAll(out, seq, nil)
	}
	return out
}
