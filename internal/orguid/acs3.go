package orguid

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// SignACS3 builds Authorization headers for Aliyun ACS3-HMAC-SHA256 (ROA style).
// action is the OpenAPI action name (e.g. ListOrganizationMembers, DeleteWorkitemComment).
func SignACS3(method, host, path, action string, query url.Values, body []byte, ak AKEnv, now time.Time) (http.Header, error) {
	if ak.AccessKeyID == "" || ak.AccessKeySecret == "" {
		return nil, fmt.Errorf("missing access key")
	}
	if action == "" {
		return nil, fmt.Errorf("missing x-acs-action")
	}
	if body == nil {
		body = []byte{}
	}
	hashedPayload := sha256Hex(body)
	xDate := now.UTC().Format("2006-01-02T15:04:05Z") // ACS3 ISO8601, not AWS compact
	hdr := http.Header{}
	hdr.Set("host", host)
	hdr.Set("x-acs-action", action)
	hdr.Set("x-acs-version", "2021-06-25")
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	xNonce := hex.EncodeToString(nonce)
	hdr.Set("x-acs-date", xDate)
	hdr.Set("x-acs-signature-nonce", xNonce)
	hdr.Set("x-acs-content-sha256", hashedPayload)
	hdr.Set("Accept", "application/json")

	signedKeys := []string{"host", "x-acs-action", "x-acs-content-sha256", "x-acs-date", "x-acs-signature-nonce", "x-acs-version"}
	sort.Strings(signedKeys)
	var canonicalHeaders strings.Builder
	for _, k := range signedKeys {
		canonicalHeaders.WriteString(k)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(hdr.Get(k)))
		canonicalHeaders.WriteByte('\n')
	}
	canonicalQuery := canonicalQueryString(query)
	canonicalRequest := strings.Join([]string{
		strings.ToUpper(method),
		path,
		canonicalQuery,
		canonicalHeaders.String(),
		strings.Join(signedKeys, ";"),
		hashedPayload,
	}, "\n")
	hashedRequest := sha256Hex([]byte(canonicalRequest))
	stringToSign := "ACS3-HMAC-SHA256\n" + hashedRequest
	sig := hmacSHA256Hex(ak.AccessKeySecret, stringToSign)
	auth := fmt.Sprintf("ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%s",
		ak.AccessKeyID, strings.Join(signedKeys, ";"), sig)
	hdr.Set("Authorization", auth)
	return hdr, nil
}

func canonicalQueryString(q url.Values) string {
	if len(q) == 0 {
		return ""
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		vs := q[k]
		sort.Strings(vs)
		for _, v := range vs {
			parts = append(parts, percentEncode(k)+"="+percentEncode(v))
		}
	}
	return strings.Join(parts, "&")
}

func percentEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256Hex(key, msg string) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}
