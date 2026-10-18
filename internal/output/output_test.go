package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSuccessEnvelope(t *testing.T) {
	var buf bytes.Buffer
	Stdout = &buf
	JQ = ""
	Format = "json"
	if err := Success(map[string]any{"a": 1}, map[string]any{"count": 1}); err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatal("ok")
	}
}

func TestJQFilter(t *testing.T) {
	var buf bytes.Buffer
	Stdout = &buf
	JQ = ".data.x"
	Format = "json"
	if err := Success(map[string]any{"x": 42}, nil); err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.(float64) != 42 {
		t.Fatalf("%v", v)
	}
	JQ = ""
}

func TestFailConfirmationShape(t *testing.T) {
	var buf bytes.Buffer
	Stderr = &buf
	JQ = ""
	err := Fail(ErrorBody{
		Type: "confirmation", Subtype: "confirmation_required",
		Message: "needs yes", Hint: "add --yes", Risk: "high-risk-write", Action: "codeup mrs create",
	}, 10)
	ee, ok := err.(ExitError)
	if !ok || ee.Code != 10 {
		t.Fatalf("%v", err)
	}
	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.OK || env.Error == nil || env.Error.Subtype != "confirmation_required" {
		t.Fatalf("%+v", env)
	}
}

func TestJQParseErrorHasHint(t *testing.T) {
	var buf bytes.Buffer
	Stdout = &buf
	JQ = "{broken"
	Format = "json"
	err := Success(map[string]any{"x": 1}, nil)
	JQ = ""
	jqe, ok := err.(*JQError)
	if !ok || jqe == nil {
		t.Fatalf("want *JQError, got %T %v", err, err)
	}
	if !strings.Contains(jqe.Error(), "jq:") {
		t.Fatalf("msg=%q", jqe.Error())
	}
	hint := jqe.Hint()
	if !strings.Contains(hint, "*-file") || !strings.Contains(hint, "#370") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestFailSkipsBrokenJQ(t *testing.T) {
	// Regression (#370): error envelopes must still print when --jq is broken.
	var buf bytes.Buffer
	Stderr = &buf
	JQ = "{broken"
	Format = "json"
	_ = Fail(ErrorBody{Type: "cli", Subtype: "jq_failed", Message: "jq: boom", Hint: "use *-file"}, 1)
	JQ = ""
	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("stderr not JSON: %v / %q", err, buf.String())
	}
	if env.OK || env.Error == nil || env.Error.Subtype != "jq_failed" {
		t.Fatalf("%+v", env)
	}
}
