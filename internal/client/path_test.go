package client

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
)

func TestPathBuildersCentral(t *testing.T) {
	c := &Client{Edition: "central", OrgID: "org1", BaseURL: config.DefaultAPIBaseURL, Token: "t"}
	p, err := c.ProjexPath(context.Background(), "/workitems:search")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/projex/organizations/org1/workitems:search" {
		t.Fatal(p)
	}
	p, err = c.CodeupPath(context.Background(), "/repositories")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/codeup/organizations/org1/repositories" {
		t.Fatal(p)
	}
	p, err = c.FlowPath(context.Background(), "/pipelines")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/flow/organizations/org1/pipelines" {
		t.Fatal(p)
	}
	p, err = c.PackagesPath(context.Background(), "/repositories")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/packages/organizations/org1/repositories" {
		t.Fatal(p)
	}
	p, err = c.AppstackPath(context.Background(), "/apps:search")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/appstack/organizations/org1/apps:search" {
		t.Fatal(p)
	}
	p, err = c.TesthubPath(context.Background(), "/testRepo/list")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/testhub/organizations/org1/testRepo/list" {
		t.Fatal(p)
	}
	p, err = c.PlatformPath(context.Background(), "/members")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/platform/organizations/org1/members" {
		t.Fatal(p)
	}
}

func TestPathBuildersRegion(t *testing.T) {
	c := &Client{Edition: "region", BaseURL: "https://region.example", Token: "t"}
	p, err := c.CodeupPath(context.Background(), "/repositories")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/codeup/repositories" {
		t.Fatal(p)
	}
	p, err = c.PackagesPath(context.Background(), "/repositories")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/oapi/v1/packages/repositories" {
		t.Fatal(p)
	}
}

func TestEncodeRepoID(t *testing.T) {
	if EncodeRepoID("123") != "123" {
		t.Fatal()
	}
	got := EncodeRepoID("org/repo name")
	if !strings.Contains(got, "%2F") {
		t.Fatalf("expected encoded, got %s", got)
	}
	if strings.Contains(got, " ") {
		t.Fatalf("space not encoded: %s", got)
	}
}

func TestEncodeFilePath(t *testing.T) {
	got := EncodeFilePath("/src/main.go")
	if !strings.Contains(got, "%2F") {
		t.Fatalf("got %s", got)
	}
	if strings.HasPrefix(got, "/") {
		t.Fatalf("leading slash should be trimmed: %s", got)
	}
}

func TestPreviewRedactsToken(t *testing.T) {
	c := &Client{BaseURL: "https://openapi-rdc.aliyuncs.com", Token: "secret", UserAgent: "t"}
	prev := c.Preview("GET", "/oapi/v1/platform/user", nil, nil)
	if prev.Headers["x-yunxiao-token"] != "(redacted)" {
		t.Fatal(prev.Headers)
	}
}

func TestRedactSecrets(t *testing.T) {
	tok := "supersecrettokenvalue"
	s := RedactSecrets("error with "+tok+" inside", tok)
	if strings.Contains(s, tok) {
		t.Fatal(s)
	}
}

func TestPaginationFromHeader(t *testing.T) {
	h := http.Header{}
	h.Set("x-page", "2")
	h.Set("x-per-page", "20")
	h.Set("x-total", "55")
	h.Set("x-total-pages", "3")
	p := PaginationFromHeader(h)
	if p == nil || p.Page != 2 || p.Total != 55 {
		t.Fatalf("%+v", p)
	}
	if PaginationFromHeader(http.Header{}) != nil {
		t.Fatal("empty should be nil")
	}
}

func TestPageQuery(t *testing.T) {
	q := PageQuery(1, 20)
	if q["page"] != "1" || q["perPage"] != "20" {
		t.Fatal(q)
	}
}

func TestAPIErrorNoTokenLeak(t *testing.T) {
	tok := "leakme-token-xyz"
	err := &APIError{
		Status: 401,
		Method: "GET",
		URL:    RedactSecrets("https://x/y?t="+tok, tok),
		Body:   RedactSecrets("bad "+tok, tok),
	}
	msg := err.Error()
	if strings.Contains(msg, tok) {
		t.Fatal(msg)
	}
}

// #110: ProjexPathOffline must derive the Projex path without any network
// access whenever the org is locally known, and report ok=false exactly when
// the online ProjexPath would first GET /platform/user (central, org unknown).
func TestProjexPathOffline(t *testing.T) {
	central := &Client{Edition: "central", OrgID: "org1"}
	if p, ok := central.ProjexPathOffline("/workitems"); !ok || p != "/oapi/v1/projex/organizations/org1/workitems" {
		t.Fatalf("central+org: p=%q ok=%v", p, ok)
	}
	if _, ok := (&Client{Edition: "central"}).ProjexPathOffline("/workitems"); ok {
		t.Fatal("central without org must not resolve offline")
	}
	if _, ok := (&Client{Edition: "central", OrgID: "default"}).ProjexPathOffline("/workitems"); ok {
		t.Fatal("org=default must not resolve offline")
	}
	region := &Client{Edition: "region"}
	if p, ok := region.ProjexPathOffline("/workitems"); !ok || p != "/oapi/v1/projex/workitems" {
		t.Fatalf("region: p=%q ok=%v", p, ok)
	}
	if p, ok := region.ProjexPathOffline("workitems"); !ok || p != "/oapi/v1/projex/workitems" {
		t.Fatalf("region no slash: p=%q ok=%v", p, ok)
	}
}
