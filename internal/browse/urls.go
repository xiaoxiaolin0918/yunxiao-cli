package browse

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// Target is a resolved console URL ready to open or print.
type Target struct {
	Kind string
	URL  string
}

func target(kind, raw string) (Target, error) {
	if err := AssertHTTPURL(raw); err != nil {
		return Target{}, err
	}
	return Target{Kind: kind, URL: strings.TrimSpace(raw)}, nil
}

// Pipeline builds a Flow console URL.
func Pipeline(pipelineID, runID string) (Target, error) {
	pid := strings.TrimSpace(pipelineID)
	if pid == "" {
		return Target{}, fmt.Errorf("pipeline-id is required")
	}
	rid := strings.TrimSpace(runID)
	if rid == "" {
		return target("pipeline", zhiyi.PipelineURL(pid))
	}
	return target("pipeline-run", zhiyi.PipelineRunURL(pid, rid))
}

// WorkItem builds a Projex console URL from ids.
func WorkItem(spaceID, internalID, serial, category string) (Target, error) {
	sid := strings.TrimSpace(spaceID)
	if sid == "" {
		return Target{}, fmt.Errorf("space-id is required")
	}
	item := map[string]any{}
	if id := strings.TrimSpace(internalID); id != "" {
		item["id"] = id
	}
	if sn := strings.TrimSpace(serial); sn != "" {
		item["serialNumber"] = sn
	}
	if cat := strings.TrimSpace(category); cat != "" {
		item["categoryId"] = cat
	}
	u := zhiyi.WorkItemURL(item, sid)
	if u == "" {
		return Target{}, fmt.Errorf("need --id or --serial to build workitem URL")
	}
	return target("workitem", u)
}

// MergeRequest builds a Codeup MR URL from repo web home + local id, or uses detail URL.
func MergeRequest(repoWebURL, localID, detailURL string) (Target, error) {
	if d := strings.TrimSpace(detailURL); d != "" {
		return target("mr", d)
	}
	mr := map[string]any{}
	if w := strings.TrimSpace(repoWebURL); w != "" {
		mr["webUrl"] = w
	}
	if lid := strings.TrimSpace(localID); lid != "" {
		mr["localId"] = lid
	}
	u := zhiyi.MergeRequestURL(mr)
	if u == "" {
		return Target{}, fmt.Errorf("need --detail-url, or --repo-url plus --local-id")
	}
	return target("mr", u)
}

// Repo returns a Codeup repo URL.
func Repo(repoWebURL string) (Target, error) {
	u := strings.TrimSpace(repoWebURL)
	if u == "" {
		return Target{}, fmt.Errorf("repo-url is required")
	}
	return target("repo", u)
}

// Raw opens an arbitrary http(s) URL.
func Raw(raw string) (Target, error) {
	return target("url", strings.TrimSpace(raw))
}

// AssertHTTPURL allows only http/https absolute URLs.
func AssertHTTPURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL %q", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("only http(s) URLs allowed, got %s", u.Scheme)
	}
}
