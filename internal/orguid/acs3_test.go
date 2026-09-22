package orguid

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignACS3_SetsAuthHeader(t *testing.T) {
	ak := AKEnv{AccessKeyID: "LTAI_test", AccessKeySecret: "secret"}
	hdr, err := SignACS3("GET", "devops.cn-hangzhou.aliyuncs.com", "/organization/o1/members", "ListOrganizationMembers", url.Values{"maxResults": {"2"}}, nil, ak, time.Date(2026, 9, 20, 4, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	auth := hdr.Get("Authorization")
	if !strings.HasPrefix(auth, "ACS3-HMAC-SHA256 Credential=LTAI_test,") {
		t.Fatalf("auth=%q", auth)
	}
	if hdr.Get("x-acs-action") != "ListOrganizationMembers" {
		t.Fatalf("action=%q", hdr.Get("x-acs-action"))
	}
	if hdr.Get("x-acs-date") != "2026-09-20T04:00:00Z" {
		t.Fatalf("date=%q", hdr.Get("x-acs-date"))
	}
	if hdr.Get("x-acs-signature-nonce") == "" {
		t.Fatal("missing nonce")
	}
}

func TestSignACS3_DeleteWorkitemCommentAction(t *testing.T) {
	ak := AKEnv{AccessKeyID: "LTAI_test", AccessKeySecret: "secret"}
	body := []byte(`{"identifier":"wi1","commentId":12}`)
	hdr, err := SignACS3("POST", "devops.cn-hangzhou.aliyuncs.com", "/organization/o1/workitems/deleteComent", "DeleteWorkitemComment", nil, body, ak, time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Get("x-acs-action") != "DeleteWorkitemComment" {
		t.Fatalf("action=%q", hdr.Get("x-acs-action"))
	}
	if hdr.Get("x-acs-content-sha256") == "" {
		t.Fatal("missing content sha256")
	}
}

func TestLoadAKEnv_Empty(t *testing.T) {
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "")
	t.Setenv("ALICLOUD_ACCESS_KEY_ID", "")
	t.Setenv("ALICLOUD_ACCESS_KEY_SECRET", "")
	t.Setenv("ALIYUN_ACCESS_KEY_ID", "")
	t.Setenv("ALIYUN_ACCESS_KEY_SECRET", "")
	if _, ok := LoadAKEnv(); ok {
		t.Fatal("expected missing")
	}
}
