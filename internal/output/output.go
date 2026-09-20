package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"
)

type Envelope struct {
	OK      bool           `json:"ok"`
	Data    any            `json:"data,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
	Error   *ErrorBody     `json:"error,omitempty"`
	DryRun  bool           `json:"dry_run,omitempty"`
	Risk    string         `json:"risk,omitempty"`
	Request any            `json:"request,omitempty"`
}

type ErrorBody struct {
	Type    string         `json:"type"`
	Subtype string         `json:"subtype,omitempty"`
	Message string         `json:"message"`
	Hint    string         `json:"hint,omitempty"`
	Risk    string         `json:"risk,omitempty"`
	Action  string         `json:"action,omitempty"`
	Code    int            `json:"code,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

var (
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
	Format           = "json" // json | pretty
	JQ               = ""
)

func Success(data any, meta map[string]any) error {
	env := Envelope{OK: true, Data: data, Meta: meta}
	return write(Stdout, env)
}

func DryRunResult(risk string, req any) error {
	env := Envelope{OK: true, DryRun: true, Risk: risk, Request: req}
	return write(Stdout, env)
}

func Fail(errBody ErrorBody, exitHint int) error {
	env := Envelope{OK: false, Error: &errBody}
	_ = write(Stderr, env)
	return ExitError{Code: exitHint, Msg: errBody.Message}
}

type ExitError struct {
	Code int
	Msg  string
}

func (e ExitError) Error() string { return e.Msg }

func write(w io.Writer, env Envelope) error {
	var payload any = env
	if JQ != "" {
		filtered, err := applyJQ(env, JQ)
		if err != nil {
			return fmt.Errorf("jq: %w", err)
		}
		payload = filtered
	}
	enc := json.NewEncoder(w)
	if Format == "pretty" {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(payload)
}

func applyJQ(env Envelope, expr string) (any, error) {
	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, err
	}
	// Convert envelope to plain map via JSON round-trip for gojq.
	b, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	iter := q.Run(v)
	var results []any
	for {
		x, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := x.(error); ok {
			return nil, err
		}
		results = append(results, x)
	}
	if len(results) == 0 {
		return nil, nil
	}
	if len(results) == 1 {
		return results[0], nil
	}
	return results, nil
}

func PrintHelpExtra(s string) {
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	fmt.Fprint(os.Stderr, s)
}
