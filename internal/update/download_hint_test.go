package update

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failHTTPClient struct{}

func (failHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return nil, errors.New("forced network failure")
}

func TestDownloadFirstOKHintsDownloadBase(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "out.bin")
	urls := []string{
		"https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/download/v0.16.46/yunxiao-cli-0.16.46-windows-amd64.zip",
		"https://ghproxy.net/https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/download/v0.16.46/yunxiao-cli-0.16.46-windows-amd64.zip",
	}
	_, err := DownloadFirstOK(context.Background(), failHTTPClient{}, urls, dest)
	if err == nil {
		t.Fatal("expected all-downloads-failed error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "all downloads failed") {
		t.Fatalf("missing failure header: %q", msg)
	}
	if !strings.Contains(msg, EnvDownloadBase) {
		t.Fatalf("error should mention %s, got: %q", EnvDownloadBase, msg)
	}
	if !strings.Contains(msg, "https://gh-proxy.com/https://github.com/"+DefaultRepo+"/releases/download") {
		t.Fatalf("error should include copyable YUNXIAO_CLI_DOWNLOAD_BASE example with gh-proxy.com, got: %q", msg)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("dest should be removed after failure: %v", statErr)
	}
}

func TestDownloadURLCandidatesIncludesGhProxyCom(t *testing.T) {
	t.Setenv(EnvDownloadBase, "")
	primary := "https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/download/v0.16.46/yunxiao-cli-0.16.46-linux-amd64.tar.gz"
	got := DownloadURLCandidates(primary)
	if len(got) < 4 {
		t.Fatalf("want primary + at least 3 mirrors, got %v", got)
	}
	if got[0] != primary {
		t.Fatalf("primary must stay first: %v", got)
	}
	if got[1] != "https://ghproxy.net/"+primary {
		t.Fatalf("ghproxy.net should stay second: %v", got)
	}
	if got[2] != "https://mirror.ghproxy.com/"+primary {
		t.Fatalf("mirror.ghproxy.com should stay third: %v", got)
	}
	wantGhProxy := "https://gh-proxy.com/" + primary
	found := false
	for i := 3; i < len(got); i++ {
		if got[i] == wantGhProxy {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing gh-proxy.com mirror after existing sources: %v", got)
	}
}

func TestArchiveDownloadCandidatesDownloadBaseFirst(t *testing.T) {
	t.Setenv(EnvDownloadBase, "https://gh-proxy.com/https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/download")
	name := "yunxiao-cli-0.16.46-linux-amd64.tar.gz"
	asset := "https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/download/v0.16.46/" + name
	got := ArchiveDownloadCandidates(name, asset)
	wantFirst := "https://gh-proxy.com/https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/download/" + name
	if len(got) == 0 || got[0] != wantFirst {
		t.Fatalf("DOWNLOAD_BASE archive URL should be first: %v", got)
	}
}
