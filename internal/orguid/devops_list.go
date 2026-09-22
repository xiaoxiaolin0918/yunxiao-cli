package orguid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// AKEnv holds Alibaba Cloud AccessKey used for devops 2021-06-25 ROA APIs.
type AKEnv struct {
	AccessKeyID     string
	AccessKeySecret string
	Region          string
}

// LoadAKEnv reads ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET (or ALIYUN_* / ALICLOUD_*).
func LoadAKEnv() (AKEnv, bool) {
	id := strings.TrimSpace(firstEnv("ALIBABA_CLOUD_ACCESS_KEY_ID", "ALICLOUD_ACCESS_KEY_ID", "ALIYUN_ACCESS_KEY_ID"))
	secret := strings.TrimSpace(firstEnv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "ALICLOUD_ACCESS_KEY_SECRET", "ALIYUN_ACCESS_KEY_SECRET"))
	region := strings.TrimSpace(os.Getenv("ALIBABA_CLOUD_REGION_ID"))
	if region == "" {
		region = "cn-hangzhou"
	}
	if id == "" || secret == "" {
		return AKEnv{}, false
	}
	return AKEnv{AccessKeyID: id, AccessKeySecret: secret, Region: region}, true
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// ACS3Signer signs Aliyun ACS3-HMAC-SHA256 ROA requests (devops 2021-06-25).
type ACS3Signer func(method, host, path, action string, query url.Values, body []byte, ak AKEnv, now time.Time) (http.Header, error)

// DevOpsMembersClient calls devops.{region}.aliyuncs.com ROA APIs with AccessKey ACS3 signing.
// Also used for workitem comment delete/update (DeleteWorkitemComment / UpdateWorkitemComment).
type DevOpsMembersClient struct {
	AK     AKEnv
	HTTP   *http.Client
	Signer ACS3Signer
	Host   string
	Scheme string // optional; default https (tests may set http)
}

func (c *DevOpsMembersClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c *DevOpsMembersClient) host() string {
	if c.Host != "" {
		return c.Host
	}
	return fmt.Sprintf("devops.%s.aliyuncs.com", c.AK.Region)
}

// EndpointURL builds https://{host}{path}?{query} for previews / dry-run.
func (c *DevOpsMembersClient) EndpointURL(path string, query url.Values) string {
	scheme := c.Scheme
	if scheme == "" {
		scheme = "https"
	}
	u := scheme + "://" + c.host() + path
	if enc := query.Encode(); enc != "" {
		u += "?" + enc
	}
	return u
}

// DoROA performs a signed ROA request and decodes JSON into a generic map.
func (c *DevOpsMembersClient) DoROA(ctx context.Context, method, path, action string, query url.Values, reqBody any) (map[string]any, error) {
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}
	}
	host := c.host()
	signer := c.Signer
	if signer == nil {
		signer = SignACS3
	}
	hdr, err := signer(method, host, path, action, query, bodyBytes, c.AK, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	u := c.EndpointURL(path, query)
	var rdr io.Reader
	if len(bodyBytes) > 0 {
		rdr = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Host = host
	for k, vs := range hdr {
		for _, v := range vs {
			if strings.EqualFold(k, "host") {
				continue
			}
			req.Header.Set(k, v)
		}
	}
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s HTTP %d: %s", action, resp.StatusCode, truncate(string(raw), 300))
	}
	var out map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("%s decode: %w", action, err)
	}
	if success, ok := out["success"]; ok {
		switch v := success.(type) {
		case bool:
			if !v {
				return out, fmt.Errorf("%s: %v", action, firstNonEmpty(asString(out["errorMessage"]), asString(out["errorMsg"]), asString(out["errorCode"]), "success=false"))
			}
		case string:
			if !strings.EqualFold(v, "true") && v != "" {
				return out, fmt.Errorf("%s: %v", action, firstNonEmpty(asString(out["errorMessage"]), asString(out["errorMsg"]), asString(out["errorCode"]), v))
			}
		}
	}
	return out, nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// ListOrganizationMembers pages with nextToken until exhausted (cap 50 pages).
func (c *DevOpsMembersClient) ListOrganizationMembers(ctx context.Context, orgID, nameQuery string) ([]map[string]any, error) {
	if orgID == "" {
		return nil, fmt.Errorf("organizationId required")
	}
	var all []map[string]any
	next := ""
	for page := 0; page < 50; page++ {
		q := url.Values{}
		q.Set("maxResults", "50")
		if nameQuery != "" {
			q.Set("organizationMemberName", nameQuery)
		}
		if next != "" {
			q.Set("nextToken", next)
		}
		path := "/organization/" + orgID + "/members"
		out, err := c.DoROA(ctx, http.MethodGet, path, "ListOrganizationMembers", q, nil)
		if err != nil {
			return nil, err
		}
		if members, ok := out["members"].([]any); ok {
			for _, m := range members {
				if row, ok := m.(map[string]any); ok {
					all = append(all, row)
				}
			}
		}
		next = strings.TrimSpace(asString(out["nextToken"]))
		if next == "" {
			break
		}
	}
	return all, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
