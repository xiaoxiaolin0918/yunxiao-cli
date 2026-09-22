package orguid

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDeleteWorkitemComment_RPCPathAndSigning(t *testing.T) {
	var gotMethod, gotPath, gotAction, gotAuth, gotBody string
	var signed bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAction = r.Header.Get("x-acs-action")
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":"true","deleteFlag":true,"requestId":"r1"}`))
	}))
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	cli := &DevOpsMembersClient{
		AK:   AKEnv{AccessKeyID: "id", AccessKeySecret: "sec", Region: "cn-hangzhou"},
		HTTP: srv.Client(),
		Host: u.Host,
		Scheme: "http",
		Signer: func(method, host, path, action string, query url.Values, body []byte, ak AKEnv, now time.Time) (http.Header, error) {
			signed = true
			if action != "DeleteWorkitemComment" {
				t.Fatalf("action=%q", action)
			}
			if !strings.HasSuffix(path, "/workitems/deleteComent") {
				t.Fatalf("path=%q", path)
			}
			if strings.Contains(path, "deleteComment") {
				t.Fatal("must keep official typo deleteComent")
			}
			h := http.Header{}
			h.Set("Authorization", "ACS3-HMAC-SHA256 Credential=id,SignedHeaders=host,Signature=test")
			h.Set("x-acs-action", action)
			h.Set("x-acs-version", "2021-06-25")
			return h, nil
		},
	}
	out, err := cli.DeleteWorkitemComment(context.Background(), "org-1", "wi-hex", 12345)
	if err != nil {
		t.Fatal(err)
	}
	if !signed {
		t.Fatal("signer not called")
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method=%s", gotMethod)
	}
	if gotPath != "/organization/org-1/workitems/deleteComent" {
		t.Fatalf("path=%s", gotPath)
	}
	if gotAction != "DeleteWorkitemComment" {
		t.Fatalf("header action=%s", gotAction)
	}
	if !strings.Contains(gotAuth, "ACS3-HMAC-SHA256") {
		t.Fatalf("auth=%s", gotAuth)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatal(err)
	}
	if body["identifier"] != "wi-hex" {
		t.Fatalf("body=%v", body)
	}
	if n, ok := body["commentId"].(float64); !ok || int64(n) != 12345 {
		t.Fatalf("commentId=%v", body["commentId"])
	}
	if out["deleteFlag"] != true {
		t.Fatalf("out=%v", out)
	}
}

func TestUpdateWorkitemComment_RPCPath(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"comment":{"id":"99","content":"hi"}}`))
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	cli := &DevOpsMembersClient{
		AK:   AKEnv{AccessKeyID: "id", AccessKeySecret: "sec", Region: "cn-hangzhou"},
		HTTP: srv.Client(),
		Host: u.Host,
		Scheme: "http",
		Signer: func(method, host, path, action string, query url.Values, body []byte, ak AKEnv, now time.Time) (http.Header, error) {
			if action != "UpdateWorkitemComment" {
				t.Fatalf("action=%q", action)
			}
			h := http.Header{}
			h.Set("Authorization", "ACS3-test")
			h.Set("x-acs-action", action)
			return h, nil
		},
	}
	out, err := cli.UpdateWorkitemComment(context.Background(), "org-1", UpdateWorkitemCommentInput{
		Content: "hi", FormatType: "MARKDOWN", WorkitemIdentifier: "wi-1", CommentID: 99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/organization/org-1/workitems/commentUpdate" {
		t.Fatalf("path=%s", gotPath)
	}
	if gotBody["content"] != "hi" || gotBody["workitemIdentifier"] != "wi-1" {
		t.Fatalf("body=%v", gotBody)
	}
	if out["comment"] == nil {
		t.Fatalf("out=%v", out)
	}
}

func TestParseCommentID(t *testing.T) {
	id, err := ParseCommentID("42")
	if err != nil || id != 42 {
		t.Fatalf("%v %v", id, err)
	}
	if _, err := ParseCommentID("0"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPreviewDeleteWorkitemComment(t *testing.T) {
	cli := &DevOpsMembersClient{AK: AKEnv{Region: "cn-hangzhou"}}
	p := cli.PreviewDeleteWorkitemComment("org", "wi", 7)
	url, _ := p["url"].(string)
	if !strings.Contains(url, "deleteComent") || strings.Contains(url, "deleteComment") {
		t.Fatalf("url=%q", url)
	}
}
