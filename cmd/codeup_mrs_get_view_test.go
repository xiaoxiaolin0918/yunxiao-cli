package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// mrsGetFixture is a realistic GetChangeRequest payload: documented fields plus
// the mergeable/checkList extras observed in real responses (#130).
const mrsGetFixture = `{
  "localId": 9,
  "title": "feat: x",
  "status": "TO_BE_MERGED",
  "mergeable": true,
  "conflictCheckStatus": "NO_CONFLICT",
  "supportMergeFastForwardOnly": true,
  "allRequirementsPass": true,
  "ahead": 2,
  "behind": 0,
  "description": "a very long description",
  "targetProjectPathWithNamespace": "org/repo",
  "reviewers": [
    {"name": "alice", "reviewOpinionStatus": "PASS", "state": "active"},
    {"name": "bob", "hasReviewed": false}
  ],
  "checkList": {"requirementRuleItems": [{"name": "code review", "pass": true}]}
}`

// newMrsGetServer serves the MR detail GET once per request and counts hits.
func newMrsGetServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/changeRequests/9") {
			hits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, mrsGetFixture)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"errorMessage":"unexpected path `+r.URL.Path+`"}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-mrs-get-not-real")
	t.Setenv(config.EnvOrganizationID, "org-mrs-get")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return srv, &hits
}

// runMrsGet runs `codeup mrs get --repo 123 --local-id 9` with extra args and
// env; returns the parsed envelope (stdout, or stderr envelope on failure).
func runMrsGet(t *testing.T, dryRun bool, envView string, extra ...string) (output.Envelope, output.ErrorBody, int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prevOut, prevErr, prevJQ, prevFmt := output.Stdout, output.Stderr, output.JQ, output.Format
	output.Stdout = &stdout
	output.Stderr = &stderr
	output.JQ = ""
	output.Format = "json"
	prevGlobals := []any{globalYes, globalDryRun, globalProfile, globalOrg}
	globalYes = false
	globalDryRun = dryRun
	globalProfile = ""
	globalOrg = ""
	t.Cleanup(func() {
		output.Stdout = prevOut
		output.Stderr = prevErr
		output.JQ = prevJQ
		output.Format = prevFmt
		globalYes, globalDryRun, globalProfile, globalOrg = prevGlobals[0].(bool), prevGlobals[1].(bool), prevGlobals[2].(string), prevGlobals[3].(string)
	})
	prevExit := processExit
	code := 0
	processExit = func(c int) { code = c; panic(exitPanic{code: c}) }
	t.Cleanup(func() { processExit = prevExit })

	t.Setenv("YUNXIAO_MRS_GET_VIEW", envView)
	resetStringFlags(t, codeupMrsGetCmd, "repo", "local-id", "summary", "brief", "full")
	args := append([]string{"codeup", "mrs", "get", "--repo", "123", "--local-id", "9"}, extra...)
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	}()
	var env output.Envelope
	_ = json.Unmarshal([]byte(stdout.String()), &env)
	var eenv output.Envelope
	var eb output.ErrorBody
	if err := json.Unmarshal([]byte(stderr.String()), &eenv); err == nil && eenv.Error != nil {
		eb = *eenv.Error
	} else {
		t.Logf("stderr not an error envelope: %v: %s", err, stderr.String())
	}
	return env, eb, code, stdout.String(), stderr.String()
}

// TestMrsGetViews (#130): default summary tier between --brief and --full.
func TestMrsGetViews(t *testing.T) {
	cases := []struct {
		name           string
		envView        string
		extra          []string
		dryRun         bool
		wantOK         bool
		wantProjection string // meta.projection; "" = absent
		wantHas        []string
		wantNot        []string
		wantRequests   int
		wantSubtype    string
	}{
		{
			name:           "default is summary",
			wantOK:         true,
			wantProjection: "summary",
			wantHas:        []string{`"mergeable":true`, `"conflictCheckStatus":"NO_CONFLICT"`, `"checkList":{"requirementRuleItems"`, `"reviewers":[{"name":"alice","opinion":"PASS"},{"name":"bob"}]`},
			wantNot:        []string{`"description"`},
			wantRequests:   1,
		},
		{
			name:           "explicit --summary",
			extra:          []string{"--summary"},
			wantOK:         true,
			wantProjection: "summary",
			wantHas:        []string{`"supportMergeFastForwardOnly":true`, `"allRequirementsPass":true`, `"ahead":2`},
			wantRequests:   1,
		},
		{
			name:           "brief stays minimal",
			extra:          []string{"--brief"},
			wantOK:         true,
			wantProjection: "brief",
			wantHas:        []string{`"localId":"9"`, `"status":"TO_BE_MERGED"`, `"state":"TO_BE_MERGED"`},
			wantNot:        []string{"mergeable", "checkList", "reviewers", "conflictCheckStatus"},
			wantRequests:   1,
		},
		{
			name:         "full returns the raw object",
			extra:        []string{"--full"},
			wantOK:       true,
			wantHas:      []string{`"description":"a very long description"`, `"mergeable":true`, `"checkList":{"requirementRuleItems"`},
			wantRequests: 1,
		},
		{
			name:         "env full without flag",
			envView:      "full",
			wantOK:       true,
			wantHas:      []string{`"description":"a very long description"`},
			wantRequests: 1,
		},
		{
			name:           "env brief without flag",
			envView:        "brief",
			wantOK:         true,
			wantProjection: "brief",
			wantNot:        []string{"mergeable"},
			wantRequests:   1,
		},
		{
			name:         "flag overrides env",
			envView:      "full",
			extra:        []string{"--summary"},
			wantOK:       true,
			wantHas:      []string{`"conflictCheckStatus":"NO_CONFLICT"`},
			wantNot:      []string{`"description"`},
			wantRequests: 1,
		},
		{
			name:         "invalid env fails before any request",
			envView:      "verbose",
			wantOK:       false,
			wantSubtype:  "invalid_env",
			wantRequests: 0,
		},
		{
			name:         "conflicting view flags rejected",
			extra:        []string{"--full", "--brief"},
			wantOK:       false,
			wantSubtype:  "",
			wantRequests: 0,
		},
		{
			name:           "dry-run shows chosen view, sends nothing",
			dryRun:         true,
			wantOK:         true,
			wantProjection: "",
			wantRequests:   0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, hits := newMrsGetServer(t)
			env, eb, code, raw, rawErr := runMrsGet(t, tc.dryRun, tc.envView, tc.extra...)
			if tc.wantOK {
				if code != 0 || !env.OK {
					t.Fatalf("code=%d ok=%v stderr-out=%s", code, env.OK, raw)
				}
				if tc.wantProjection != "" {
					if env.Meta == nil || env.Meta["projection"] != tc.wantProjection {
						t.Fatalf("meta.projection=%v want %q (meta=%#v)", env.Meta, tc.wantProjection, env.Meta)
					}
				} else if tc.dryRun {
					proj := mrsGetDryRunProjection(t, env)
					if proj["mode"] != "summary" {
						t.Fatalf("dry-run request.projection=%#v", proj)
					}
				} else if env.Meta != nil && tc.name == "full returns the raw object" {
					if _, has := env.Meta["projection"]; has {
						t.Fatalf("--full must not set meta.projection: %#v", env.Meta)
					}
				}
				var payload []byte
				if tc.dryRun {
					payload, _ = json.Marshal(env.Request)
				} else {
					payload, _ = json.Marshal(env.Data)
				}
				s := string(payload)
				for _, want := range tc.wantHas {
					if !strings.Contains(s, want) {
						t.Fatalf("payload missing %s: %s", want, s)
					}
				}
				for _, no := range tc.wantNot {
					if strings.Contains(s, no) {
						t.Fatalf("payload must not contain %s: %s", no, s)
					}
				}
			} else {
				if code != 1 {
					t.Fatalf("code=%d want 1: stdout=%s stderr=%s", code, raw, rawErr)
				}
				if tc.wantSubtype != "" && eb.Subtype != tc.wantSubtype {
					t.Fatalf("subtype=%q want %q: eb=%#v stderr=%s", eb.Subtype, tc.wantSubtype, eb, rawErr)
				}
			}
			if *hits != tc.wantRequests {
				t.Fatalf("server hits=%d want %d", *hits, tc.wantRequests)
			}
		})
	}
}

// mrsGetDryRunProjection extracts request.projection from a dry-run envelope.
func mrsGetDryRunProjection(t *testing.T, env output.Envelope) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("request: %v", err)
	}
	proj, _ := req["projection"].(map[string]any)
	if proj == nil {
		t.Fatalf("request.projection missing: %s", raw)
	}
	return proj
}
