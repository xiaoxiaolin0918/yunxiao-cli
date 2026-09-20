package orguid

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAccountIDString(t *testing.T) {
	if AccountIDString(int64(123456677888)) != "123456677888" {
		t.Fatalf("int64")
	}
	if AccountIDString(float64(123456677888)) != "123456677888" {
		t.Fatalf("float64 from json")
	}
	if AccountIDString("123456677888") != "123456677888" {
		t.Fatalf("string")
	}
	if AccountIDString(nil) != "" {
		t.Fatalf("nil")
	}
}

func TestAttachAliyunUID_ByEmail(t *testing.T) {
	oapi := []any{
		map[string]any{"userId": "hex1", "name": "Zhang", "email": "a@ex.com"},
		map[string]any{"userId": "hex2", "name": "Li", "email": "b@ex.com"},
	}
	legacy := []map[string]any{
		{"accountId": float64(111), "organizationMemberName": "Zhang", "email": "a@ex.com"},
		{"accountId": "222", "organizationMemberName": "Li", "email": "b@ex.com"},
	}
	out, meta := AttachAliyunUID(oapi, legacy)
	if meta["aliyun_uid_matched"].(int) != 2 {
		t.Fatalf("matched=%v", meta["aliyun_uid_matched"])
	}
	m0 := out[0].(map[string]any)
	if m0["aliyunUid"] != "111" {
		t.Fatalf("got %v", m0["aliyunUid"])
	}
	if m0["accountId"] != "111" {
		t.Fatalf("accountId alias %v", m0["accountId"])
	}
}

func TestAttachAliyunUID_ByNameWhenEmailMissing(t *testing.T) {
	oapi := []any{map[string]any{"userId": "hex1", "name": "Wang"}}
	legacy := []map[string]any{{"accountId": "999", "organizationMemberName": "Wang"}}
	out, meta := AttachAliyunUID(oapi, legacy)
	if meta["aliyun_uid_matched"].(int) != 1 {
		t.Fatalf("meta %#v", meta)
	}
	if out[0].(map[string]any)["aliyunUid"] != "999" {
		t.Fatalf("%v", out[0])
	}
}

func TestMissingAKMessage(t *testing.T) {
	msg := MissingAKMessage()
	for _, p := range []string{"ManualValidate", "accountId", "ALIBABA_CLOUD_ACCESS_KEY"} {
		if !strings.Contains(msg, p) {
			t.Fatalf("missing %q in %q", p, msg)
		}
	}
}
func TestAccountIDString_JSONNumberPreservesLargeUID(t *testing.T) {
	// > 2^53; float64 would round
	got := AccountIDString(json.Number("9007199254740993"))
	if got != "9007199254740993" {
		t.Fatalf("got %q", got)
	}
}
