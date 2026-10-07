package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// workitemItemView resolves the output view of the work item embedded in transition
// results (+transition / +bug-transition, #114), mirroring `workitem get` (#98):
// --full / --brief flag > YUNXIAO_WORKITEM_GET_VIEW (full | brief) > default brief.
// Scripts that must run on CLIs before and after this change set
// YUNXIAO_WORKITEM_GET_VIEW=full (old CLIs ignore it).
type workitemItemView struct {
	mode   string // "brief" (default) or "full"
	source string // "flag", "env" or "default"
}

// transitionItemViewFromFlags validates the view flags and env before any request,
// the same contract as workitemGetViewFromFlags. --full=false / --brief=false count
// as not given.
func transitionItemViewFromFlags(cmd *cobra.Command) (workitemItemView, error) {
	var set []string
	for _, f := range []string{"full", "brief"} {
		if on, _ := cmd.Flags().GetBool(f); on && cmd.Flags().Changed(f) {
			set = append(set, "--"+f)
		}
	}
	if len(set) > 1 {
		return workitemItemView{}, fmt.Errorf("--full and --brief are mutually exclusive (got %s)", strings.Join(set, " and "))
	}
	if len(set) == 1 {
		if set[0] == "--full" {
			return workitemItemView{mode: "full", source: "flag"}, nil
		}
		return workitemItemView{mode: "brief", source: "flag"}, nil
	}
	raw, ok := os.LookupEnv(envWorkitemGetView)
	switch v := strings.ToLower(strings.TrimSpace(raw)); {
	case !ok || v == "":
		return workitemItemView{mode: "brief", source: "default"}, nil
	case v == "full" || v == "brief":
		return workitemItemView{mode: v, source: "env"}, nil
	default:
		return workitemItemView{}, &detailedError{
			Subtype: "invalid_env",
			Message: fmt.Sprintf("%s=%q: expected full or brief", envWorkitemGetView, raw),
			Hint:    fmt.Sprintf("unset %s or set it to full / brief; a --full / --brief flag overrides it", envWorkitemGetView),
			Details: map[string]any{"env": envWorkitemGetView, "value": raw, "allowed": []string{"full", "brief"}},
		}
	}
}

// previewProjection is request.projection for --dry-run. nil for --full given as a
// flag, whose dry-run output stays as before (same rule as workitem get).
func (v workitemItemView) previewProjection() map[string]any {
	if v.mode == "full" && v.source == "flag" {
		return nil
	}
	return map[string]any{"mode": v.mode, "source": v.source}
}

// itemValue projects the refreshed work item for the success envelope (#114): raw
// (may be nil) for --full — identical to pre-#114 output — else the brief projection
// (falls back to the pre-PUT item when the refresh failed).
func (v workitemItemView) itemValue(refreshed, preItem map[string]any) any {
	if v.mode == "full" {
		return refreshed
	}
	src := refreshed
	if src == nil {
		src = preItem
	}
	return zhiyi.BriefWorkItem(src)
}

// addTransitionStatusBriefs adds from_status / to_status ({id, displayName}) to a
// brief transition result (#114). from is the pre-PUT item's status; to prefers the
// refreshed status and falls back to the profile alias→id map reversed.
func addTransitionStatusBriefs(result map[string]any, preItem, refreshed map[string]any, refreshOK bool, target string, statuses map[string]string) {
	if from := zhiyi.StatusBrief(preItem); len(from) > 0 {
		result["from_status"] = from
	}
	if to := zhiyi.TransitionStatusBrief(refreshed, refreshOK, target, statuses); len(to) > 0 {
		result["to_status"] = to
	}
}
