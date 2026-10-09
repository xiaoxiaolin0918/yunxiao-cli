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

func wiEffBodySetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-wi-eff-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-wi-eff-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	prevYes := globalYes
	prevDry := globalDryRun
	globalYes = false
	globalDryRun = false
	t.Cleanup(func() { globalYes = prevYes; globalDryRun = prevDry })

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, workitemEffortsCreateCmd, "id", "gmt-start", "gmt-end", "description", "operator-id", "work-type")
	_ = workitemEffortsCreateCmd.Flags().Set("actual-time", "0")
	resetStringFlags(t, workitemEffortsUpdateCmd, "workitem-id", "id", "gmt-start", "gmt-end", "description", "operator-id", "work-type")
	_ = workitemEffortsUpdateCmd.Flags().Set("actual-time", "0")
	return stdout, hits
}

func wiEffBodyAssert(t *testing.T, stdout *bytes.Buffer, hits *int, wantMethod, pathSub string, check func(t *testing.T, body map[string]any, req map[string]any)) {
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

func TestWorkitemEffortsCreateMinimalDryRun(t *testing.T) {
	stdout, hits := wiEffBodySetup(t, "create-min")
	rootCmd.SetArgs([]string{
		"workitem", "efforts", "create",
		"--id", "wi-eff",
		"--actual-time", "1.5",
		"--gmt-start", "2026-10-01",
		"--gmt-end", "2026-10-01",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEffBodyAssert(t, stdout, hits, "POST", "/workitems/wi-eff/effortRecords", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["actualTime"] != float64(1.5) || body["gmtStart"] != "2026-10-01" || body["gmtEnd"] != "2026-10-01" {
			t.Fatalf("body=%v", body)
		}
		for _, k := range []string{"description", "operatorId", "workType"} {
			if _, has := body[k]; has {
				t.Fatalf("optional %s must be omitted: %#v", k, body)
			}
		}
	})
}

func TestWorkitemEffortsCreateDescriptionDryRun(t *testing.T) {
	stdout, hits := wiEffBodySetup(t, "create-desc")
	rootCmd.SetArgs([]string{
		"workitem", "efforts", "create",
		"--id", "wi-eff",
		"--actual-time", "2",
		"--gmt-start", "2026-10-02",
		"--gmt-end", "2026-10-02",
		"--description", "coding",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEffBodyAssert(t, stdout, hits, "POST", "/workitems/wi-eff/effortRecords", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["description"] != "coding" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemEffortsCreateOperatorWorkTypeDryRun(t *testing.T) {
	stdout, hits := wiEffBodySetup(t, "create-op")
	rootCmd.SetArgs([]string{
		"workitem", "efforts", "create",
		"--id", "wi-eff",
		"--actual-time", "3",
		"--gmt-start", "2026-10-03",
		"--gmt-end", "2026-10-03",
		"--operator-id", "op-eff",
		"--work-type", "DEV",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEffBodyAssert(t, stdout, hits, "POST", "/workitems/wi-eff/effortRecords", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["operatorId"] != "op-eff" || body["workType"] != "DEV" {
			t.Fatalf("body=%v", body)
		}
		if _, has := body["description"]; has {
			t.Fatalf("description must be omitted: %#v", body)
		}
	})
}

func TestWorkitemEffortsCreateFullOptionalDryRun(t *testing.T) {
	stdout, hits := wiEffBodySetup(t, "create-full")
	rootCmd.SetArgs([]string{
		"workitem", "efforts", "create",
		"--id", "wi-eff",
		"--actual-time", "4.25",
		"--gmt-start", "2026-10-04",
		"--gmt-end", "2026-10-05",
		"--description", "full",
		"--operator-id", "op-full",
		"--work-type", "TEST",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEffBodyAssert(t, stdout, hits, "POST", "/workitems/wi-eff/effortRecords", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["actualTime"] != float64(4.25) || body["description"] != "full" {
			t.Fatalf("body=%v", body)
		}
		if body["operatorId"] != "op-full" || body["workType"] != "TEST" {
			t.Fatalf("body=%v", body)
		}
		if body["gmtStart"] != "2026-10-04" || body["gmtEnd"] != "2026-10-05" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemEffortsUpdateMinimalDryRun(t *testing.T) {
	stdout, hits := wiEffBodySetup(t, "update-min")
	rootCmd.SetArgs([]string{
		"workitem", "efforts", "update",
		"--workitem-id", "wi-eff",
		"--id", "er-1",
		"--actual-time", "5",
		"--gmt-start", "2026-10-06",
		"--gmt-end", "2026-10-06",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEffBodyAssert(t, stdout, hits, "PUT", "/workitems/wi-eff/effortRecords/er-1", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["actualTime"] != float64(5) {
			t.Fatalf("body=%v", body)
		}
		for _, k := range []string{"description", "operatorId", "workType"} {
			if _, has := body[k]; has {
				t.Fatalf("optional %s must be omitted: %#v", k, body)
			}
		}
	})
}

func TestWorkitemEffortsUpdateDescriptionDryRun(t *testing.T) {
	stdout, hits := wiEffBodySetup(t, "update-desc")
	rootCmd.SetArgs([]string{
		"workitem", "efforts", "update",
		"--workitem-id", "wi-eff",
		"--id", "er-2",
		"--actual-time", "6",
		"--gmt-start", "2026-10-07",
		"--gmt-end", "2026-10-07",
		"--description", "revised effort",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEffBodyAssert(t, stdout, hits, "PUT", "/workitems/wi-eff/effortRecords/er-2", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["description"] != "revised effort" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemEffortsUpdateOperatorWorkTypeDryRun(t *testing.T) {
	stdout, hits := wiEffBodySetup(t, "update-op")
	rootCmd.SetArgs([]string{
		"workitem", "efforts", "update",
		"--workitem-id", "wi-eff",
		"--id", "er-3",
		"--actual-time", "7",
		"--gmt-start", "2026-10-08",
		"--gmt-end", "2026-10-08",
		"--operator-id", "op-upd",
		"--work-type", "REVIEW",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEffBodyAssert(t, stdout, hits, "PUT", "/workitems/wi-eff/effortRecords/er-3", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["operatorId"] != "op-upd" || body["workType"] != "REVIEW" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestWorkitemEffortsUpdateFullBodyDryRun(t *testing.T) {
	stdout, hits := wiEffBodySetup(t, "update-full")
	rootCmd.SetArgs([]string{
		"workitem", "efforts", "update",
		"--workitem-id", "wi-eff",
		"--id", "er-4",
		"--actual-time", "8.5",
		"--gmt-start", "2026-10-09",
		"--gmt-end", "2026-10-10",
		"--description", "full upd",
		"--operator-id", "op-f",
		"--work-type", "DESIGN",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	wiEffBodyAssert(t, stdout, hits, "PUT", "/workitems/wi-eff/effortRecords/er-4", func(t *testing.T, body map[string]any, _ map[string]any) {
		if body["actualTime"] != float64(8.5) || body["description"] != "full upd" {
			t.Fatalf("body=%v", body)
		}
		if body["operatorId"] != "op-f" || body["workType"] != "DESIGN" {
			t.Fatalf("body=%v", body)
		}
		if body["gmtStart"] != "2026-10-09" || body["gmtEnd"] != "2026-10-10" {
			t.Fatalf("body=%v", body)
		}
	})
}