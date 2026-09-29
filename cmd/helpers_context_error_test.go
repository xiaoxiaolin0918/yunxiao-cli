package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// handleErrBody runs handleErr and returns the stderr error body and exit code.
func handleErrBody(t *testing.T, err error) (output.ErrorBody, int) {
	t.Helper()
	var stderr bytes.Buffer
	prevErr := output.Stderr
	output.Stderr = &stderr
	t.Cleanup(func() { output.Stderr = prevErr })
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		handleErr(err)
	}()
	var env output.Envelope
	if e := json.Unmarshal(stderr.Bytes(), &env); e != nil || env.Error == nil {
		t.Fatalf("stderr JSON: %v / %s", e, stderr.String())
	}
	return *env.Error, code
}

// Hint concatenation: API hint first, then the context hint.
func TestHandleErrContextErrorJoinsAPIHint(t *testing.T) {
	ae := &client.APIError{Status: 400, Method: "GET", URL: "https://x/y", Body: `{"errorMessage":"未启用此字段【模块】"}`}
	apiHint := apiErrorHint(ae)
	if apiHint == "" {
		t.Fatal("fixture must trigger a non-empty API hint")
	}
	eb, code := handleErrBody(t, &contextError{Context: "resolve latest patchset for MR 7", Hint: "pass --patchset-biz-id explicitly", Err: ae})
	if code != 1 || eb.Type != "api" || eb.Code != 400 {
		t.Fatalf("code=%d body=%+v", code, eb)
	}
	if eb.Hint != apiHint+"; pass --patchset-biz-id explicitly" {
		t.Fatalf("hint=%q", eb.Hint)
	}
	if !strings.HasPrefix(eb.Message, "resolve latest patchset for MR 7: yunxiao API GET") {
		t.Fatalf("message=%q", eb.Message)
	}
}

// Wrapped APIError keeps Subtype / Details (e.g. yaml_validation) like the unwrapped path.
func TestHandleErrContextErrorKeepsAPISubtypeAndDetails(t *testing.T) {
	body := `{"errorCode":"1209300","errorMessage":"yaml校验失败 {\"errorMessage\":\"validators invalid\",\"path\":\"stages[0].jobs[0]\"}"}`
	ae := &client.APIError{Status: 400, Method: "POST", URL: "https://x/pipelines", Body: body}
	plain, _ := handleErrBody(t, ae)
	wrapped, _ := handleErrBody(t, &contextError{Context: "ctx", Hint: "extra", Err: ae})
	if plain.Subtype != "yaml_validation" || wrapped.Subtype != plain.Subtype {
		t.Fatalf("subtype plain=%q wrapped=%q", plain.Subtype, wrapped.Subtype)
	}
	if wrapped.Details == nil || wrapped.Details["errorCode"] != "1209300" || wrapped.Details["issues"] == nil {
		t.Fatalf("details=%#v", wrapped.Details)
	}
	if wrapped.Hint != plain.Hint+"; extra" {
		t.Fatalf("hint plain=%q wrapped=%q", plain.Hint, wrapped.Hint)
	}
}

// Non-API cause: type cli, context hint only.
func TestHandleErrContextErrorNonAPI(t *testing.T) {
	eb, _ := handleErrBody(t, &contextError{Context: "ctx", Hint: "h", Err: errors.New("dial tcp: refused")})
	if eb.Type != "cli" || eb.Message != "ctx: dial tcp: refused" || eb.Hint != "h" || eb.Code != 0 {
		t.Fatalf("body=%+v", eb)
	}
}

// An empty context hint must not leave a dangling "; " after the API hint.
func TestHandleErrContextErrorEmptyHint(t *testing.T) {
	ae := &client.APIError{Status: 400, Method: "GET", URL: "https://x/y", Body: `{"errorMessage":"未启用此字段【模块】"}`}
	apiHint := apiErrorHint(ae)
	eb, _ := handleErrBody(t, &contextError{Context: "ctx", Err: ae})
	if eb.Hint != apiHint || strings.HasSuffix(eb.Hint, "; ") {
		t.Fatalf("hint=%q want %q", eb.Hint, apiHint)
	}
}
