package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/browse"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var browsePrintOnly bool

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Open Yunxiao console pages in a browser (or print the URL)",
	Long: `Risk: read

Build a console URL and open it with the OS default browser.
Use --print-only or global --dry-run to print the URL without opening.

Examples:
  yunxiao browse pipeline --pipeline-id 5272454
  yunxiao browse pipeline --pipeline-id 5272454 --run-id 4
  yunxiao browse workitem --space-id <id> --serial ZYPT-1
  yunxiao browse mr --repo-url https://codeup.aliyun.com/org/repo --local-id 3
  yunxiao browse repo --repo-url https://codeup.aliyun.com/org/repo
  yunxiao browse url https://flow.aliyun.com/pipelines/5272454
`,
}

func init() {
	browseCmd.PersistentFlags().BoolVar(&browsePrintOnly, "print-only", false, "print URL only; do not open a browser")
	browseCmd.AddCommand(browsePipelineCmd)
	browseCmd.AddCommand(browseWorkitemCmd)
	browseCmd.AddCommand(browseMRCmd)
	browseCmd.AddCommand(browseRepoCmd)
	browseCmd.AddCommand(browseURLCmd)
}

func emitBrowse(t browse.Target) {
	data := map[string]any{"kind": t.Kind, "url": t.URL}
	meta := map[string]any{"risk": risk.Read}
	open := browse.ShouldOpenBrowser(browsePrintOnly, globalDryRun)
	if !open {
		fmt.Fprintln(os.Stderr, t.URL)
		meta["opened"] = false
		if browsePrintOnly {
			meta["print_only"] = true
		}
		if globalDryRun {
			meta["dry_run"] = true
		}
		handleErr(output.Success(data, meta))
		return
	}
	if err := browse.OpenSystemBrowser(t.URL); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open browser: %v\n", err)
		fmt.Fprintln(os.Stderr, t.URL)
		meta["opened"] = false
		meta["warning"] = err.Error()
		handleErr(output.Success(data, meta))
		return
	}
	meta["opened"] = true
	handleErr(output.Success(data, meta))
}

var browsePipelineID string
var browseRunID string
var browsePipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Open a Flow pipeline or run page",
	Run: func(cmd *cobra.Command, args []string) {
		t, err := browse.Pipeline(browsePipelineID, browseRunID)
		if err != nil {
			handleErr(err)
			return
		}
		emitBrowse(t)
	},
}

func init() {
	browsePipelineCmd.Flags().StringVar(&browsePipelineID, "pipeline-id", "", "pipeline id (required)")
	browsePipelineCmd.Flags().StringVar(&browseRunID, "run-id", "", "optional run/build id")
	_ = browsePipelineCmd.MarkFlagRequired("pipeline-id")
}

var browseSpaceID string
var browseWorkItemID string
var browseSerial string
var browseCategory string
var browseWorkitemCmd = &cobra.Command{
	Use:   "workitem",
	Short: "Open a Projex work item page",
	Run: func(cmd *cobra.Command, args []string) {
		t, err := browse.WorkItem(browseSpaceID, browseWorkItemID, browseSerial, browseCategory)
		if err != nil {
			handleErr(err)
			return
		}
		emitBrowse(t)
	},
}

func init() {
	browseWorkitemCmd.Flags().StringVar(&browseSpaceID, "space-id", "", "Projex space/project id (required)")
	browseWorkitemCmd.Flags().StringVar(&browseWorkItemID, "id", "", "internal work item id")
	browseWorkitemCmd.Flags().StringVar(&browseSerial, "serial", "", "serial number e.g. ZYPT-1")
	browseWorkitemCmd.Flags().StringVar(&browseCategory, "category", "", "Bug|Req|Task (optional)")
	_ = browseWorkitemCmd.MarkFlagRequired("space-id")
}

var browseRepoURL string
var browseLocalID string
var browseDetailURL string
var browseMRCmd = &cobra.Command{
	Use:   "mr",
	Short: "Open a Codeup merge request page",
	Run: func(cmd *cobra.Command, args []string) {
		t, err := browse.MergeRequest(browseRepoURL, browseLocalID, browseDetailURL)
		if err != nil {
			handleErr(err)
			return
		}
		emitBrowse(t)
	},
}

func init() {
	browseMRCmd.Flags().StringVar(&browseRepoURL, "repo-url", "", "repo web home")
	browseMRCmd.Flags().StringVar(&browseLocalID, "local-id", "", "MR local id (change number)")
	browseMRCmd.Flags().StringVar(&browseDetailURL, "detail-url", "", "full MR page URL")
}

var browseRepoOnlyURL string
var browseRepoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Open a Codeup repository page",
	Run: func(cmd *cobra.Command, args []string) {
		t, err := browse.Repo(browseRepoOnlyURL)
		if err != nil {
			handleErr(err)
			return
		}
		emitBrowse(t)
	},
}

func init() {
	browseRepoCmd.Flags().StringVar(&browseRepoOnlyURL, "repo-url", "", "repo web URL (required)")
	_ = browseRepoCmd.MarkFlagRequired("repo-url")
}

var browseURLCmd = &cobra.Command{
	Use:   "url [https://...]",
	Short: "Open an arbitrary http(s) console URL",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		t, err := browse.Raw(args[0])
		if err != nil {
			handleErr(err)
			return
		}
		emitBrowse(t)
	},
}
