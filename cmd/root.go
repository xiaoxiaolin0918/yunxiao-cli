package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/alias"
	"github.com/yunxiao-cli/yunxiao/internal/termui"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/version"
)

var rootCmd = &cobra.Command{
	Use:   "yunxiao",
	Short: "Yunxiao (阿里云云效) CLI — Feishu/Lark-style progressive discovery for humans and agents",
	Long: `yunxiao — Alibaba Cloud DevOps (云效 / Yunxiao) CLI.

AGENT QUICKSTART (driving this as an agent? start here):
    Browse commands:  yunxiao <domain> --help            # +shortcuts (preferred) and typed API resources
    Inspect a call:   yunxiao schema <domain.resource.method>   # params, types, risk, examples
    Prefer a +shortcut over the typed API resource when one matches the task.
    Risk: each command's --help shows read | write | high-risk-write;
          high-risk-write needs --yes, only after the user confirms.
    On any API call: --jq <expr> filters JSON output, --dry-run previews the request (runs nothing).

EXAMPLES (one per command style, in order of preference):
    yunxiao project +my-open-items                       # +shortcut — a high-level task, prefer these
    yunxiao pipeline +status --pipeline-id <id>          # +shortcut — latest run status
    yunxiao codeup mrs list --state opened               # typed command for one API method
    yunxiao schema codeup.mrs.create                     # inspect a method's params before calling
    yunxiao api GET /oapi/v1/platform/user               # raw escape hatch — any endpoint by HTTP path

Auth: YUNXIAO_ACCESS_TOKEN > credentials.json (last auth login) > profile > config.json
Interactive: yunxiao auth login --browser   (OAuth = full account API capability)
CI/PAT:      yunxiao auth login --token <PAT>
PAT console: https://account-devops.aliyun.com/settings/personalAccessToken
Help: https://help.aliyun.com/zh/yunxiao/user-guide/personal-access-token`,
	Version:       version.Version,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		termui.Configure() // #76: disable clear-screen/TUI when non-TTY / NO_COLOR / YUNXIAO_NO_TUI
		output.Format = globalFormat
		output.JQ = globalJQ
		maybeStartUpdateHint(cmd)
	},
}

var (
	globalFormat  string
	globalJQ      string
	globalOrg     string
	globalProfile string
	globalYes     bool
	globalDryRun  bool
)

func Execute() {
	expandAliases()
	if err := rootCmd.Execute(); err != nil {
		handleErr(err)
	}
}

func expandAliases() {
	s, err := alias.Load()
	if err != nil || len(s) == 0 {
		return
	}
	newArgs, name, ok, err := alias.ExpandArgs(os.Args, s, reservedRootNames())
	if err != nil {
		handleErr(err)
		return
	}
	if !ok {
		return
	}
	os.Args = newArgs
	fmt.Fprintf(os.Stderr, "alias: %s -> %s\n", name, strings.Join(newArgs[1:], " "))
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("yunxiao %s\n", version.Version))
	rootCmd.PersistentFlags().StringVar(&globalFormat, "format", "json", "output format: json|pretty")
	rootCmd.PersistentFlags().StringVar(&globalJQ, "jq", "", "jq expression to filter JSON output")
	rootCmd.PersistentFlags().StringVar(&globalOrg, "organization-id", "", "organization ID (or YUNXIAO_ORGANIZATION_ID)")
	rootCmd.PersistentFlags().StringVar(&globalProfile, "profile", "", "tenant profile name (or YUNXIAO_PROFILE), e.g. zhiyi")
	rootCmd.PersistentFlags().BoolVarP(&globalYes, "yes", "y", false, "confirm high-risk-write / multi-step write operations")
	rootCmd.PersistentFlags().BoolVar(&globalDryRun, "dry-run", false, "preview request without executing")

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(browseCmd)
	rootCmd.AddCommand(aliasCmd)
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(apiCmd)
	rootCmd.AddCommand(schemaCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(organizationCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(sprintCmd)
	rootCmd.AddCommand(versionsCmd)
	rootCmd.AddCommand(workitemCmd)
	rootCmd.AddCommand(codeupCmd)
	rootCmd.AddCommand(pipelineCmd)
	rootCmd.AddCommand(packagesCmd)
	rootCmd.AddCommand(testhubCmd)
	rootCmd.AddCommand(appstackCmd)

	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
}
