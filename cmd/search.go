package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/yunxiao-cli/yunxiao/internal/output"
)

// unifiedSearchRoute describes one type alias group for `yunxiao search`.
type unifiedSearchRoute struct {
	// Canonical is the primary type name shown in help tables.
	Canonical string
	// Aliases are accepted type tokens (including Canonical).
	Aliases []string
	// Target is the existing domain command whose Run we reuse.
	Target *cobra.Command
	// TargetPath is the human path, e.g. "workitem search".
	TargetPath string
	// QueryFlag is the domain keyword flag --q/--query maps to.
	// Empty means the target has no keyword flag; --q/--query must error.
	QueryFlag string
	// SchemaHint points agents at the authoritative schema/help.
	SchemaHint string
}

func unifiedSearchRoutes() []unifiedSearchRoute {
	return []unifiedSearchRoute{
		{
			Canonical:  "workitem",
			Aliases:    []string{"workitem", "issue"},
			Target:     workitemSearchCmd,
			TargetPath: "workitem search",
			QueryFlag:  "subject",
			SchemaHint: "yunxiao schema workitem.search",
		},
		{
			Canonical:  "mr",
			Aliases:    []string{"mr", "mrs"},
			Target:     codeupMrsListCmd,
			TargetPath: "codeup mrs list",
			QueryFlag:  "search",
			SchemaHint: "yunxiao codeup mrs list --help",
		},
		{
			Canonical:  "repo",
			Aliases:    []string{"repo", "repos"},
			Target:     codeupReposListCmd,
			TargetPath: "codeup repos list",
			QueryFlag:  "search",
			SchemaHint: "yunxiao codeup repos list --help",
		},
		{
			Canonical:  "case",
			Aliases:    []string{"case", "cases"},
			Target:     testhubCasesSearchCmd,
			TargetPath: "testhub cases search",
			QueryFlag:  "", // no keyword flag on cases search
			SchemaHint: "yunxiao testhub cases search --help",
		},
		{
			Canonical:  "app-tag",
			Aliases:    []string{"app-tag"},
			Target:     appstackTagsSearchCmd,
			TargetPath: "appstack tags search",
			QueryFlag:  "search",
			SchemaHint: "yunxiao appstack tags search --help",
		},
		{
			Canonical:  "pipeline",
			Aliases:    []string{"pipeline"},
			Target:     pipelineListCmd,
			TargetPath: "pipeline list",
			QueryFlag:  "name",
			SchemaHint: "yunxiao pipeline list --help",
		},
	}
}

func resolveUnifiedSearchRoute(typeName string) (unifiedSearchRoute, error) {
	key := strings.ToLower(strings.TrimSpace(typeName))
	if key == "" {
		return unifiedSearchRoute{}, fmt.Errorf("search type required")
	}
	for _, r := range unifiedSearchRoutes() {
		for _, a := range r.Aliases {
			if a == key {
				return r, nil
			}
		}
	}
	return unifiedSearchRoute{}, fmt.Errorf("unknown search type %q", typeName)
}

func unifiedSearchTypeHelpTable() string {
	var b strings.Builder
	b.WriteString("Supported types (explicit router; no multi-domain merge):\n")
	for _, r := range unifiedSearchRoutes() {
		aliases := strings.Join(r.Aliases, " | ")
		qNote := "(no keyword; --q errors)"
		if r.QueryFlag != "" {
			qNote = fmt.Sprintf("(--q/--query -> --%s)", r.QueryFlag)
		}
		b.WriteString(fmt.Sprintf("  %-20s -> %-25s %s\n", aliases, r.TargetPath, qNote))
	}
	b.WriteString("Not in v1: packages, appstack change-requests, default/all-domain search.\n")
	b.WriteString("Authoritative schemas stay on the target command (this is only a router).")
	return b.String()
}

var searchCmd = &cobra.Command{
	Use:   "search [type]",
	Short: "Unified search by resource type (thin router)",
	Long: `Risk: read

Thin type-routed facade over existing domain search/list commands.
Does NOT invent a new search engine or merge results across domains.

Usage:
  yunxiao search <type> [domain flags...]
  yunxiao search --type <type> [domain flags...]

` + unifiedSearchTypeHelpTable() + `

Examples:
  yunxiao search workitem --subject login --category Bug
  yunxiao search issue --q login --assigned-to self
  yunxiao search --type mr --state opened --search fix
  yunxiao search repo --q yunxiao-cli
  yunxiao search cases --repo-id <id>
  yunxiao search app-tag --q demo
  yunxiao search pipeline --q deploy

Global flags (--jq / --dry-run / --format / --organization-id / --profile / --yes)
pass through to the target command.
`,
	DisableFlagParsing: true,
	Run:                runUnifiedSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
}

func runUnifiedSearch(cmd *cobra.Command, args []string) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			_ = cmd.Help()
			return
		}
	}

	typ, query, targetArgs, err := parseUnifiedSearchArgs(args)
	if err != nil {
		handleErr(fmt.Errorf("%w\n\n%s", err, unifiedSearchTypeHelpTable()))
		return
	}
	if typ == "" {
		handleErr(fmt.Errorf("search type required (positional or --type)\n\n%s", unifiedSearchTypeHelpTable()))
		return
	}

	route, err := resolveUnifiedSearchRoute(typ)
	if err != nil {
		handleErr(fmt.Errorf("%w\n\n%s", err, unifiedSearchTypeHelpTable()))
		return
	}

	if query != "" {
		if route.QueryFlag == "" {
			handleErr(fmt.Errorf("--q/--query is not supported for type %q (%s has no keyword flag); pass domain flags such as --repo-id instead", typ, route.TargetPath))
			return
		}
		targetArgs, err = injectMappedQueryFlag(targetArgs, route.QueryFlag, query)
		if err != nil {
			handleErr(err)
			return
		}
	}

	resetCommandFlagValues(route.Target)
	if err := route.Target.ParseFlags(targetArgs); err != nil {
		handleErr(fmt.Errorf("%s flag parse: %w (see: %s)", route.TargetPath, err, route.SchemaHint))
		return
	}
	// DisableFlagParsing skips root PersistentPreRun's snapshot of late --format/--jq.
	// Target.ParseFlags updates globalFormat/globalJQ via inherited persistent flags;
	// re-apply so output.* matches what the user passed after `search`.
	output.Format = globalFormat
	output.JQ = globalJQ
	pos := route.Target.Flags().Args()
	switch {
	case route.Target.Run != nil:
		route.Target.Run(route.Target, pos)
	case route.Target.RunE != nil:
		handleErr(route.Target.RunE(route.Target, pos))
	default:
		handleErr(fmt.Errorf("internal: %s has no Run", route.TargetPath))
	}
}

// parseUnifiedSearchArgs extracts the router type and optional --q/--query.
// Remaining tokens (including global persistent flags) are returned for the target.
//
// Type must be either:
//   - the first argv token when it is not a flag, or
//   - --type <name> / --type=<name> (first occurrence only; later --type goes to target,
//     e.g. workitem --type <ids>)
//
// Positional type after other flags is NOT accepted (avoids mistaking flag values
// for the type). Use --type in that case.
func parseUnifiedSearchArgs(args []string) (typeName, query string, rest []string, err error) {
	i := 0
	gotType := false
	if len(args) > 0 && args[0] != "" && args[0] != "-" && !strings.HasPrefix(args[0], "-") {
		typeName = args[0]
		gotType = true
		i = 1
	}

	var (
		gotQuery bool
		out      []string
	)
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			out = append(out, args[i:]...)
			i = len(args)
		case a == "--type" || strings.HasPrefix(a, "--type="):
			if gotType {
				out = append(out, a)
				if a == "--type" {
					if i+1 >= len(args) {
						return "", "", nil, fmt.Errorf("--type requires a value")
					}
					out = append(out, args[i+1])
					i += 2
					continue
				}
				i++
				continue
			}
			var val string
			if strings.HasPrefix(a, "--type=") {
				val = strings.TrimPrefix(a, "--type=")
				i++
			} else {
				if i+1 >= len(args) {
					return "", "", nil, fmt.Errorf("--type requires a value")
				}
				val = args[i+1]
				i += 2
			}
			if strings.TrimSpace(val) == "" {
				return "", "", nil, fmt.Errorf("--type requires a value")
			}
			typeName = val
			gotType = true
		case a == "-q" || a == "--q" || a == "--query" || strings.HasPrefix(a, "--q=") || strings.HasPrefix(a, "--query="):
			var val, which string
			switch {
			case a == "-q" || a == "--q":
				which = "q"
				if i+1 >= len(args) {
					return "", "", nil, fmt.Errorf("--q requires a value")
				}
				val = args[i+1]
				i += 2
			case a == "--query":
				which = "query"
				if i+1 >= len(args) {
					return "", "", nil, fmt.Errorf("--query requires a value")
				}
				val = args[i+1]
				i += 2
			case strings.HasPrefix(a, "--q="):
				which = "q"
				val = strings.TrimPrefix(a, "--q=")
				i++
			default:
				which = "query"
				val = strings.TrimPrefix(a, "--query=")
				i++
			}
			if val == "" {
				return "", "", nil, fmt.Errorf("--%s requires a value", which)
			}
			if gotQuery && query != val {
				return "", "", nil, fmt.Errorf("conflicting --q/--query values: %q vs %q", query, val)
			}
			query = val
			gotQuery = true
		default:
			out = append(out, a)
			i++
		}
	}
	return typeName, query, out, nil
}

// injectMappedQueryFlag appends --<flag> <value> unless the flag is already set.
// If already set to a different value, returns an error (never silently drop).
func injectMappedQueryFlag(args []string, flagName, value string) ([]string, error) {
	long := "--" + flagName
	eqPrefix := long + "="
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == long:
			if i+1 >= len(args) {
				return nil, fmt.Errorf("%s is present without a value", long)
			}
			existing := args[i+1]
			if existing != value {
				return nil, fmt.Errorf("--q/--query=%q conflicts with existing %s %q", value, long, existing)
			}
			return args, nil
		case strings.HasPrefix(a, eqPrefix):
			existing := strings.TrimPrefix(a, eqPrefix)
			if existing != value {
				return nil, fmt.Errorf("--q/--query=%q conflicts with existing %s", value, a)
			}
			return args, nil
		}
	}
	return append(args, long, value), nil
}

func resetCommandFlagValues(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = cmd.Flags().Set(f.Name, f.DefValue)
		f.Changed = false
	})
}
