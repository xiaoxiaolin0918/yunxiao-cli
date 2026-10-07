package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// EnvGithubRepo overrides the default GitHub owner/repo.
const EnvGithubRepo = "YUNXIAO_CLI_GITHUB_REPO"

// EnvDownloadBase optional https:// base for archives.
const EnvDownloadBase = "YUNXIAO_CLI_DOWNLOAD_BASE"

// EnvUpdateCheck disables optional update hints when set to "0" / "false" / "off".
const EnvUpdateCheck = "YUNXIAO_UPDATE_CHECK"

// ExitUpdateAvailable is returned by update --check when a newer release exists.
const ExitUpdateAvailable = 2

// Release is a subset of the GitHub Releases API payload.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
	HTMLURL string  `json:"html_url"`
}

// Asset is a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// HTTPDoer is the minimal client surface (tests inject httptest).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient is used when no client is injected.
func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

// GithubRepo returns owner/repo for Releases.
func GithubRepo() string {
	if v := strings.Trim(strings.TrimSpace(os.Getenv(EnvGithubRepo)), "/"); v != "" {
		return v
	}
	return DefaultRepo
}

// UpdateCheckDisabled reports YUNXIAO_UPDATE_CHECK=0|false|off.
func UpdateCheckDisabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvUpdateCheck)))
	return v == "0" || v == "false" || v == "off" || v == "no"
}

// LatestReleaseURL is the GitHub API URL for the latest release.
func LatestReleaseURL(repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
}

// FetchLatestRelease loads the latest non-draft release metadata.
func FetchLatestRelease(ctx context.Context, client HTTPDoer, repo string) (*Release, error) {
	if client == nil {
		client = DefaultHTTPClient()
	}
	if repo == "" {
		repo = GithubRepo()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LatestReleaseURL(repo), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "yunxiao-cli-self-update")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %s: HTTP %d: %s", LatestReleaseURL(repo), resp.StatusCode, truncate(string(body), 200))
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return nil, fmt.Errorf("GitHub latest release missing tag_name")
	}
	return &rel, nil
}

// FindAsset returns the asset whose name matches want (exact).
func FindAsset(rel *Release, want string) (*Asset, error) {
	if rel == nil {
		return nil, fmt.Errorf("nil release")
	}
	for i := range rel.Assets {
		if rel.Assets[i].Name == want {
			return &rel.Assets[i], nil
		}
	}
	var names []string
	for _, a := range rel.Assets {
		names = append(names, a.Name)
	}
	return nil, fmt.Errorf("asset %q not found in release %s (have: %s)", want, rel.TagName, strings.Join(names, ", "))
}

// ChecksumMap parses a sha256sum-style checksums.txt (hash  name).
func ChecksumMap(content string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// "hash  name" or "hash *name"
		var hash, name string
		if i := strings.Index(line, "  "); i >= 0 {
			hash = strings.TrimSpace(line[:i])
			name = strings.TrimSpace(line[i+2:])
		} else if i := strings.Index(line, " *"); i >= 0 {
			hash = strings.TrimSpace(line[:i])
			name = strings.TrimSpace(line[i+2:])
		} else {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			hash, name = fields[0], fields[len(fields)-1]
		}
		name = strings.TrimPrefix(name, "*")
		if len(hash) == 64 {
			out[name] = strings.ToLower(hash)
		}
	}
	return out
}

// DownloadURLCandidates returns primary + optional mirrors.
func DownloadURLCandidates(primary string) []string {
	urls := []string{}
	if base := strings.TrimRight(strings.TrimSpace(os.Getenv(EnvDownloadBase)), "/"); base != "" {
		// Caller usually passes full asset URL; if DOWNLOAD_BASE is set they
		// typically want base/<archiveName>. Keep primary first; base is added by caller.
		_ = base
	}
	if primary != "" {
		urls = append(urls, primary)
		if strings.Contains(primary, "github.com") {
			urls = append(urls,
				"https://ghproxy.net/"+primary,
				"https://mirror.ghproxy.com/"+primary,
			)
		}
	}
	return urls
}

// ArchiveDownloadCandidates builds URL list for an archive name + release asset URL.
func ArchiveDownloadCandidates(archiveName, assetURL string) []string {
	var urls []string
	if base := strings.TrimRight(strings.TrimSpace(os.Getenv(EnvDownloadBase)), "/"); base != "" {
		if strings.HasPrefix(strings.ToLower(base), "https://") {
			urls = append(urls, base+"/"+archiveName)
		}
	}
	urls = append(urls, DownloadURLCandidates(assetURL)...)
	return urls
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
