package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// renderCreateHelp renders `workitem create --help` into a buffer, restoring the
// command output afterwards so other tests are unaffected.
func renderCreateHelp(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	workitemCreateCmd.SetOut(&buf)
	workitemCreateCmd.SetErr(&buf)
	t.Cleanup(func() {
		workitemCreateCmd.SetOut(nil)
		workitemCreateCmd.SetErr(nil)
	})
	if err := workitemCreateCmd.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}
	return buf.String()
}

// helpFlagsSection isolates the "Flags:" block (up to "Global Flags:" / "Usage:")
// so assertions check the command's own flags, not Long text or global flags.
func helpFlagsSection(help string) string {
	start := strings.Index(help, "\nFlags:\n")
	if start < 0 {
		return ""
	}
	rest := help[start+len("\nFlags:\n"):]
	if end := strings.Index(rest, "\nGlobal Flags:"); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// TestWorkitemCreateHelpListsFlags (#130): help and actual flags must not drift
// apart — `--type-id` (and the other required flags) must render in the Flags
// section even though the Long text is long enough to push it below the fold.
func TestWorkitemCreateHelpListsFlags(t *testing.T) {
	cases := []struct {
		flag  string
		usage string
	}{
		{"type-id", "work item type id (required)"},
		{"space-id", "project/space id (required)"},
		{"subject", "title (required unless --subject-file)"},
		{"assigned-to", "assignee user id or self (required)"},
		{"subject-file", "UTF-8 file for title"},
		{"description-file", "UTF-8 file for description"},
		{"custom-fields", "JSON object of customFieldValues"},
		{"custom-fields-file", "UTF-8 JSON object file"},
		{"sprint", "sprint id"},
		{"no-defaults", "skip profile workitem_defaults"},
		{"no-precheck", "skip the required-field precheck"},
		{"full", "print full create JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			f := workitemCreateCmd.Flags().Lookup(tc.flag)
			if f == nil {
				t.Fatalf("flag --%s not registered on workitem create", tc.flag)
			}
			if f.Hidden {
				t.Fatalf("flag --%s is hidden", tc.flag)
			}
			help := renderCreateHelp(t)
			section := helpFlagsSection(help)
			if !strings.Contains(section, "--"+tc.flag) {
				t.Fatalf("--%s missing from help Flags section:\n%s", tc.flag, section)
			}
			if !strings.Contains(section, tc.usage) {
				t.Fatalf("--%s usage %q missing from help Flags section:\n%s", tc.flag, tc.usage, section)
			}
		})
	}
}

// TestWorkitemCreateHelpRendersEveryFlag: every registered non-hidden flag
// appears in the Flags section (inherited root flags in "Global Flags:"), so a
// future registration change cannot make help and reality diverge again.
func TestWorkitemCreateHelpRendersEveryFlag(t *testing.T) {
	help := renderCreateHelp(t)
	section := helpFlagsSection(help)
	if section == "" {
		t.Fatalf("no Flags section in help:\n%s", help)
	}
	globals := workitemCreateCmd.InheritedFlags()
	var missing []string
	workitemCreateCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		if globals.Lookup(f.Name) != nil {
			// Inherited root flags render under "Global Flags:", not "Flags:".
			if !strings.Contains(help, "--"+f.Name) {
				missing = append(missing, f.Name+" (global)")
			}
			return
		}
		if !strings.Contains(section, "--"+f.Name) {
			missing = append(missing, f.Name)
		}
	})
	if len(missing) > 0 {
		t.Fatalf("registered flags not rendered in help Flags section: %s", strings.Join(missing, ", "))
	}
}
