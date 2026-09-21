package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/alias"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage local command aliases (cannot embed --yes)",
	Long: `Risk: read for list; write for set/delete (local file only).

Aliases expand the first command token into argv tokens. They must NOT embed
--yes / -y — high-risk confirmation stays on the invocation line.

Stored in ~/.config/yunxiao/aliases.json.

Examples:
  yunxiao alias set pending pipeline +pending --all-pipelines
  yunxiao pending
  yunxiao alias list
  yunxiao alias delete pending
`,
}

func init() {
	aliasCmd.AddCommand(aliasSetCmd)
	aliasCmd.AddCommand(aliasListCmd)
	aliasCmd.AddCommand(aliasDeleteCmd)
}

func reservedRootNames() map[string]bool {
	m := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		m[c.Name()] = true
		for _, a := range c.Aliases {
			m[a] = true
		}
	}
	m["help"] = true
	m["completion"] = true
	return m
}

var aliasSetCmd = &cobra.Command{
	Use:                "set <name> <command> [args...]",
	Short:              "Create or replace an alias",
	DisableFlagParsing: true,
	Args:               cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		exp := args[1:]
		if len(exp) > 0 && exp[0] == "--" {
			exp = exp[1:]
		}
		if err := alias.ValidateName(name, reservedRootNames()); err != nil {
			handleErr(err)
			return
		}
		if err := alias.ValidateExpansion(exp); err != nil {
			handleErr(err)
			return
		}
		s, err := alias.Load()
		if err != nil {
			handleErr(err)
			return
		}
		s[name] = append([]string{}, exp...)
		if err := alias.Save(s); err != nil {
			handleErr(err)
			return
		}
		p, _ := alias.Path()
		handleErr(output.Success(map[string]any{
			"name":      name,
			"expansion": exp,
			"path":      p,
		}, map[string]any{"risk": risk.Write}))
	},
}

var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List aliases",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := alias.Load()
		if err != nil {
			handleErr(err)
			return
		}
		items := make([]map[string]any, 0, len(s))
		for name, exp := range s {
			items = append(items, map[string]any{
				"name":      name,
				"expansion": exp,
				"command":   strings.Join(exp, " "),
			})
		}
		p, _ := alias.Path()
		handleErr(output.Success(items, map[string]any{"risk": risk.Read, "path": p, "count": len(items)}))
	},
}

var aliasDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an alias",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		s, err := alias.Load()
		if err != nil {
			handleErr(err)
			return
		}
		if _, ok := s[name]; !ok {
			handleErr(fmt.Errorf("alias %q not found", name))
			return
		}
		delete(s, name)
		if err := alias.Save(s); err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(map[string]any{"deleted": name}, map[string]any{"risk": risk.Write}))
	},
}
