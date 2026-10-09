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

func tagsListQSetup(t *testing.T, tag string) (stdout *bytes.Buffer, hits *int) {
	t.Helper()
	var n int
	hits = &n
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(config.EnvAccessToken, "test-token-tags-list-q-"+tag+"-not-real")
	t.Setenv(config.EnvOrganizationID, "org-tags-list-q-"+tag)
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	stdout = withCmdJSONCapture(t)
	resetStringFlags(t, codeupTagsListCmd, "repo", "search", "sort", "order-by", "page", "per-page")
	return stdout, hits
}

func tagsListQAssert(t *testing.T, stdout *bytes.Buffer, hits *int, checkURL func(t *testing.T, url string)) {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("json: %v / %s", err, stdout.Bytes())
	}
	if !env.OK || !env.DryRun {
		t.Fatalf("%+v", env)
	}
	if env.Risk != string(risk.Read) && env.Risk != "read" {
		t.Fatalf("risk=%q", env.Risk)
	}
	raw, _ := json.Marshal(env.Request)
	var req map[string]any
	_ = json.Unmarshal(raw, &req)
	url, _ := req["url"].(string)
	if !strings.Contains(url, "/repositories/4952001/tags") {
		t.Fatalf("url=%q", url)
	}
	if req["method"] != "GET" {
		t.Fatalf("method=%v", req["method"])
	}
	if checkURL != nil {
		checkURL(t, url)
	}
	if *hits != 0 {
		t.Fatalf("hits=%d", *hits)
	}
}

func TestCodeupTagsListDefaultsDryRun(t *testing.T) {
	stdout, hits := tagsListQSetup(t, "defaults")
	rootCmd.SetArgs([]string{"codeup", "tags", "list", "--repo", "4952001", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	tagsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=1") || !strings.Contains(url, "perPage=20") {
			t.Fatalf("defaults missing: url=%q", url)
		}
		if strings.Contains(url, "search=") || strings.Contains(url, "sort=") || strings.Contains(url, "orderBy=") {
			t.Fatalf("optional query must be omitted: url=%q", url)
		}
	})
}

func TestCodeupTagsListSearchOnlyDryRun(t *testing.T) {
	stdout, hits := tagsListQSetup(t, "search")
	rootCmd.SetArgs([]string{"codeup", "tags", "list", "--repo", "4952001", "--search", "v1.", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	tagsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "search=v1.") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestCodeupTagsListSortDescDryRun(t *testing.T) {
	stdout, hits := tagsListQSetup(t, "sort-desc")
	rootCmd.SetArgs([]string{"codeup", "tags", "list", "--repo", "4952001", "--sort", "desc", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	tagsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "sort=desc") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "orderBy=") {
			t.Fatalf("orderBy must be omitted: url=%q", url)
		}
	})
}

func TestCodeupTagsListSortAscDryRun(t *testing.T) {
	stdout, hits := tagsListQSetup(t, "sort-asc")
	rootCmd.SetArgs([]string{"codeup", "tags", "list", "--repo", "4952001", "--sort", "asc", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	tagsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "sort=asc") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestCodeupTagsListOrderByNameDryRun(t *testing.T) {
	stdout, hits := tagsListQSetup(t, "order-name")
	rootCmd.SetArgs([]string{"codeup", "tags", "list", "--repo", "4952001", "--order-by", "name", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	tagsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "orderBy=name") {
			t.Fatalf("url=%q", url)
		}
		if strings.Contains(url, "sort=") {
			t.Fatalf("sort must be omitted: url=%q", url)
		}
	})
}

func TestCodeupTagsListOrderByCreateDryRun(t *testing.T) {
	stdout, hits := tagsListQSetup(t, "order-create")
	rootCmd.SetArgs([]string{"codeup", "tags", "list", "--repo", "4952001", "--order-by", "create", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	tagsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "orderBy=create") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestCodeupTagsListPagePerPageDryRun(t *testing.T) {
	stdout, hits := tagsListQSetup(t, "page")
	rootCmd.SetArgs([]string{"codeup", "tags", "list", "--repo", "4952001", "--page", "3", "--per-page", "50", "--dry-run"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	tagsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "page=3") || !strings.Contains(url, "perPage=50") {
			t.Fatalf("url=%q", url)
		}
	})
}

func TestCodeupTagsListSearchSortOrderDryRun(t *testing.T) {
	stdout, hits := tagsListQSetup(t, "combo")
	rootCmd.SetArgs([]string{
		"codeup", "tags", "list",
		"--repo", "4952001",
		"--search", "rc",
		"--sort", "desc",
		"--order-by", "name",
		"--page", "2",
		"--per-page", "10",
		"--dry-run",
	})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stdout.String())
	}
	tagsListQAssert(t, stdout, hits, func(t *testing.T, url string) {
		if !strings.Contains(url, "search=rc") || !strings.Contains(url, "sort=desc") || !strings.Contains(url, "orderBy=name") {
			t.Fatalf("url=%q", url)
		}
		if !strings.Contains(url, "page=2") || !strings.Contains(url, "perPage=10") {
			t.Fatalf("url=%q", url)
		}
	})
}
