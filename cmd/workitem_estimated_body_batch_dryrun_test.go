package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

func wiEstBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-est-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-est-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, workitemEstimatedCreateCmd, "id", "owner", "description", "operator-id", "work-type")
	_ = workitemEstimatedCreateCmd.Flags().Set("spent-time", "0")
	resetStringFlags(t, workitemEstimatedUpdateCmd, "workitem-id", "id", "owner", "description", "operator-id", "work-type")
	_ = workitemEstimatedUpdateCmd.Flags().Set("spent-time", "0")
	return stdout, hits
}

func wiEstBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantMethod, pathSub string, check func(t *testing.T, body map[string]any, req map[string]any)) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.Write) && env.Risk != "write" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, pathSub) {
		t.Fatalf("url=%q want %q", url, pathSub)
	}
	if req["method"] != wantMethod {
		t.Fatalf("method=%v want %s", req["method"], wantMethod)
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body nil: %v", req)
	}
	if check != nil {
		check(t, body, req)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestWorkitemEstimatedCreateMinimalDryRun(t *testing.T) {
	stdout, hits := wiEstBodySetup(t, "create-min")
	rootCmd.SetArgs([]string{
		"workitem", "estimated-efforts", "create",
		"--id", "wi-batch",
		"--owner", "u-1",
		"--spent-time", "2",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEstBodyAssert(t, stdout, hits, "POST", "/workitems/wi-batch/estimatedEfforts", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["owner"] != "u-1" || body["spentTime"] != float64(2) {
			t.Fatalf("body=%v", body)
		}
		for _, k := range []string{"description", "operatorId", "workType"} {
			if _, has := body[k]; has {
				t.Fatalf("optional %s must be omitted: %#v", k, body)
			}
		}
	})
}

func TestWorkitemEstimatedCreateOperatorWorkTypeDryRun(t *testing.T) {
	stdout, hits := wiEstBodySetup(t, "create-op")
	rootCmd.SetArgs([]string{
		"workitem", "estimated-efforts", "create",
		"--id", "wi-batch",
		"--owner", "u-2",
		"--spent-time", "3.5",
		"--operator-id", "op-9",
		"--work-type", "DEV",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEstBodyAssert(t, stdout, hits, "POST", "/workitems/wi-batch/estimatedEfforts", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["operatorId"] != "op-9" || body["workType"] != "DEV" {
			t.Fatalf("body=%v", body)
		}
		if body["spentTime"] != float64(3.5) {
			t.Fatalf("spentTime=%v", body["spentTime"])
		}
		if _, has := body["description"]; has {
			t.Fatalf("description must be omitted: %#v", body)
		}
	})
}

func TestWorkitemEstimatedCreateFullOptionalDryRun(t *testing.T) {
	stdout, hits := wiEstBodySetup(t, "create-full")
	rootCmd.SetArgs([]string{
		"workitem", "estimated-efforts", "create",
		"--id", "wi-batch",
		"--owner", "u-3",
		"--spent-time", "6",
		"--description", "batch est",
		"--operator-id", "op-1",
		"--work-type", "TEST",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEstBodyAssert(t, stdout, hits, "POST", "/workitems/wi-batch/estimatedEfforts", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["description"] != "batch est" || body["operatorId"] != "op-1" || body["workType"] != "TEST" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemEstimatedCreateEmptyOptionalOmitsDryRun(t *testing.T) {
	stdout, hits := wiEstBodySetup(t, "create-empty")
	rootCmd.SetArgs([]string{
		"workitem", "estimated-efforts", "create",
		"--id", "wi-batch",
		"--owner", "u-4",
		"--spent-time", "1",
		"--description", "",
		"--operator-id", "",
		"--work-type", "",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEstBodyAssert(t, stdout, hits, "POST", "/workitems/wi-batch/estimatedEfforts", func(t *testing.T, body map[string]any, _ map[string]any) {
		for _, k := range []string{"description", "operatorId", "workType"} {
			if _, has := body[k]; has {
				t.Fatalf("empty %s must omit: %#v", k, body)
			}
		}
	})
}

func TestWorkitemEstimatedUpdateMinimalDryRun(t *testing.T) {
	stdout, hits := wiEstBodySetup(t, "update-min")
	rootCmd.SetArgs([]string{
		"workitem", "estimated-efforts", "update",
		"--workitem-id", "wi-batch",
		"--id", "ee-1",
		"--owner", "u-5",
		"--spent-time", "5",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEstBodyAssert(t, stdout, hits, "PUT", "/workitems/wi-batch/estimatedEfforts/ee-1", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["owner"] != "u-5" || body["spentTime"] != float64(5) {
			t.Fatalf("body=%v", body)
		}
		for _, k := range []string{"description", "operatorId", "workType"} {
			if _, has := body[k]; has {
				t.Fatalf("optional %s must be omitted: %#v", k, body)
			}
		}
	})
}

func TestWorkitemEstimatedUpdateDescriptionDryRun(t *testing.T) {
	stdout, hits := wiEstBodySetup(t, "update-desc")
	rootCmd.SetArgs([]string{
		"workitem", "estimated-efforts", "update",
		"--workitem-id", "wi-batch",
		"--id", "ee-2",
		"--owner", "u-6",
		"--spent-time", "7",
		"--description", "revised",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEstBodyAssert(t, stdout, hits, "PUT", "/workitems/wi-batch/estimatedEfforts/ee-2", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["description"] != "revised" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemEstimatedUpdateOperatorWorkTypeDryRun(t *testing.T) {
	stdout, hits := wiEstBodySetup(t, "update-op")
	rootCmd.SetArgs([]string{
		"workitem", "estimated-efforts", "update",
		"--workitem-id", "wi-batch",
		"--id", "ee-3",
		"--owner", "u-7",
		"--spent-time", "9",
		"--operator-id", "op-upd",
		"--work-type", "REVIEW",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEstBodyAssert(t, stdout, hits, "PUT", "/workitems/wi-batch/estimatedEfforts/ee-3", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["operatorId"] != "op-upd" || body["workType"] != "REVIEW" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemEstimatedUpdateFullBodyDryRun(t *testing.T) {
	stdout, hits := wiEstBodySetup(t, "update-full")
	rootCmd.SetArgs([]string{
		"workitem", "estimated-efforts", "update",
		"--workitem-id", "wi-batch",
		"--id", "ee-4",
		"--owner", "u-8",
		"--spent-time", "10.25",
		"--description", "full upd",
		"--operator-id", "op-full",
		"--work-type", "DESIGN",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEstBodyAssert(t, stdout, hits, "PUT", "/workitems/wi-batch/estimatedEfforts/ee-4", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["owner"] != "u-8" || body["spentTime"] != float64(10.25) {
			t.Fatalf("body=%v", body)
		}
		if body["description"] != "full upd" || body["operatorId"] != "op-full" || body["workType"] != "DESIGN" {
			t.Fatalf("body=%v", body)
		}
	})
}