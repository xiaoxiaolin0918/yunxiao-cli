package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yunxiao-cli/yunxiao/internal/config"
)

func TestPushReviewStatusFlagsSelfTargetAndShaAnomaly(t *testing.T) {
	const selfTarget = `{
 "localId":192,"title":"self target","status":"TO_BE_MERGED",
 "sourceBranch":"hifox-agent/pao-tui-xiao-di/ZYPT-6522X",
 "targetBranch":"hifox-agent/pao-tui-xiao-di/ZYPT-6522X",
 "ahead":0,"behind":0,"allRequirementsPass":true,
 "detailUrl":"https://codeup.aliyun.com/org/zhiyi_workspace/change/192"
}`
	const shaSource = `{
 "localId":194,"title":"sha source","status":"UNDER_REVIEW",
 "sourceBranch":"8b08d11abcdef0123",
 "targetBranch":"hifox-agent/pao-tui-xiao-di/ZYPT-6522X",
 "ahead":1,"behind":0,"allRequirementsPass":true,
 "detailUrl":"https://codeup.aliyun.com/org/zhiyi_workspace/change/194"
}`
	const normal = `{
 "localId":139,"title":"feat: push review","status":"UNDER_DEV",
 "sourceBranch":"feat/x","targetBranch":"master","ahead":3,"behind":1,
 "allRequirementsPass":false,
 "detailUrl":"https://codeup.aliyun.com/org/zhiyi_doc/change/139"
}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/changeRequests/192"):
			_, _ = io.WriteString(w, selfTarget)
		case strings.HasSuffix(r.URL.Path, "/changeRequests/194"):
			_, _ = io.WriteString(w, shaSource)
		case strings.HasSuffix(r.URL.Path, "/changeRequests/139"):
			_, _ = io.WriteString(w, normal)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(config.EnvAccessToken, "test-token-push-review-target-not-real")
	t.Setenv(config.EnvOrganizationID, "org-push-review-target")
	t.Setenv(config.EnvEdition, "central")
	t.Setenv(config.EnvAPIBaseURL, srv.URL)
	t.Setenv("YUNXIAO_PROFILE", "")

	cases := []struct {
		localID          string
		wantEqual        bool
		wantBlocked      bool
		wantNextStepPart string
	}{
		{localID: "192", wantEqual: true, wantBlocked: true, wantNextStepPart: "不可直接合入 master"},
		{localID: "194", wantEqual: false, wantBlocked: true, wantNextStepPart: "--target-branch master"},
		{localID: "139", wantBlocked: false},
	}
	for _, tc := range cases {
		t.Run("local-id "+tc.localID, func(t *testing.T) {
			stdout, stderr, code := runPushReviewStatus(t, false, "--local-id", tc.localID)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
			env := decodeMrsStatusEnvelope(t, stdout)
			items, _ := env.Data.([]any)
			if len(items) != 1 {
				t.Fatalf("data=%#v", env.Data)
			}
			m, _ := items[0].(map[string]any)
			if tc.wantBlocked {
				if m["cannot_merge_to_master"] != true {
					t.Fatalf("cannot_merge_to_master: %#v", m)
				}
				next, _ := m["next_step"].(string)
				if !strings.Contains(next, tc.wantNextStepPart) || !strings.Contains(next, "mrs update") {
					t.Fatalf("next_step=%q want substring %q", next, tc.wantNextStepPart)
				}
				if eq, _ := m["target_equals_source"].(bool); eq != tc.wantEqual {
					t.Fatalf("target_equals_source=%v want %v in %#v", eq, tc.wantEqual, m)
				}
			} else {
				if _, ok := m["cannot_merge_to_master"]; ok {
					t.Fatalf("normal MR must omit cannot_merge_to_master: %#v", m)
				}
				if _, ok := m["next_step"]; ok {
					t.Fatalf("normal MR must omit next_step: %#v", m)
				}
			}
			// Agents must still be able to parse the envelope as JSON.
			var raw map[string]any
			if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
				t.Fatalf("stdout JSON: %v", err)
			}
		})
	}
}

func TestPushReviewStatusHelpMentionsTargetHint(t *testing.T) {
	h := codeupMrsPushReviewStatusCmd.Long
	for _, want := range []string{"cannot_merge_to_master", "target-branch", "不可直接合入"} {
		if !strings.Contains(h, want) {
			t.Fatalf("help missing %q: %s", want, h)
		}
	}
}
