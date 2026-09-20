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

// DevOpsMembersClient calls GET /organization/{org}/members on devops.{region}.aliyuncs.com.
type DevOpsMembersClient struct {
	AK     AKEnv
	HTTP   *http.Client
	Signer func(method, host, path string, query url.Values, body []byte, ak AKEnv, now time.Time) (http.Header, error)
	Host   string
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
		host := c.host()
		signer := c.Signer
		if signer == nil {
			signer = SignACS3
		}
		hdr, err := signer(http.MethodGet, host, path, q, nil, c.AK, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		u := "https://" + host + path
		if enc := q.Encode(); enc != "" {
			u += "?" + enc
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
		resp, err := c.httpClient().Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("ListOrganizationMembers HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
		}
		var parsed struct {
			Success   bool             `json:"success"`
			ErrorMsg  string           `json:"errorMessage"`
			NextToken string           `json:"nextToken"`
			Members   []map[string]any `json:"members"`
		}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		if err := dec.Decode(&parsed); err != nil {
			return nil, err
		}
		if !parsed.Success && parsed.ErrorMsg != "" {
			return nil, fmt.Errorf("ListOrganizationMembers: %s", parsed.ErrorMsg)
		}
		all = append(all, parsed.Members...)
		next = strings.TrimSpace(parsed.NextToken)
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
