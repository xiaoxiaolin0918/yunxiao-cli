package termui

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisabledByEnv(t *testing.T) {
	t.Setenv("YUNXIAO_NO_TUI", "1")
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	if Enabled() {
		t.Fatal("YUNXIAO_NO_TUI=1 must disable TUI")
	}
}

func TestDisabledByNoColor(t *testing.T) {
	t.Setenv("YUNXIAO_NO_TUI", "")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm")
	if Enabled() {
		t.Fatal("NO_COLOR must disable TUI")
	}
}

func TestDisabledByTermDumb(t *testing.T) {
	t.Setenv("YUNXIAO_NO_TUI", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if Enabled() {
		t.Fatal("TERM=dumb must disable TUI")
	}
}

func TestWriteTTYNoopWhenDisabled(t *testing.T) {
	t.Setenv("YUNXIAO_NO_TUI", "1")
	var buf bytes.Buffer
	n, err := WriteTTY(&buf, []byte("\x1b[H\x1b[2J\x1b[3J"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || buf.Len() != 0 {
		t.Fatalf("disabled WriteTTY must drop clearscreen bytes; n=%d buf=%q", n, buf.Bytes())
	}
}

func TestStripClearScreenSequences(t *testing.T) {
	in := []byte("hello\x1b[H\x1b[2J\x1b[3Jworld\x1b[2J")
	out := StripClearScreen(in)
	if bytes.Contains(out, []byte{0x1b}) {
		t.Fatalf("still has ESC: %q", out)
	}
	if !strings.Contains(string(out), "hello") || !strings.Contains(string(out), "world") {
		t.Fatalf("got %q", out)
	}
}

func TestConfigureForcesNoColorWhenNonInteractive(t *testing.T) {
	t.Setenv("YUNXIAO_NO_TUI", "1")
	t.Setenv("NO_COLOR", "")
	Configure()
	if !Disabled() {
		t.Fatal("expected Disabled after Configure with YUNXIAO_NO_TUI")
	}
}
