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

// Pipeline builds a Flow console URL.
func Pipeline(pipelineID, runID string) (Target, error) {
	pid := strings.TrimSpace(pipelineID)
	if pid == "" {
		return Target{}, fmt.Errorf("pipeline-id is required")
	}
	rid := strings.TrimSpace(runID)
	if rid == "" {
		return Target{Kind: "pipeline", URL: zhiyi.PipelineURL(pid)}, nil
	}
	return Target{Kind: "pipeline-run", URL: zhiyi.PipelineRunURL(pid, rid)}, nil
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
	return Target{Kind: "workitem", URL: u}, nil
}

// MergeRequest builds a Codeup MR URL from repo web home + local id, or uses detail URL.
func MergeRequest(repoWebURL, localID, detailURL string) (Target, error) {
	if d := strings.TrimSpace(detailURL); d != "" {
		if err := AssertHTTPURL(d); err != nil {
			return Target{}, err
		}
		return Target{Kind: "mr", URL: d}, nil
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
	return Target{Kind: "mr", URL: u}, nil
}

// Repo returns a Codeup repo URL.
func Repo(repoWebURL string) (Target, error) {
	u := strings.TrimSpace(repoWebURL)
	if u == "" {
		return Target{}, fmt.Errorf("repo-url is required")
	}
	if err := AssertHTTPURL(u); err != nil {
		return Target{}, err
	}
	return Target{Kind: "repo", URL: u}, nil
}

// Raw opens an arbitrary http(s) URL.
func Raw(raw string) (Target, error) {
	u := strings.TrimSpace(raw)
	if err := AssertHTTPURL(u); err != nil {
		return Target{}, err
	}
	return Target{Kind: "url", URL: u}, nil
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
