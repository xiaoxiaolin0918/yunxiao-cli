package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/version"
	examples "github.com/yunxiao-cli/yunxiao/profiles"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Tenant profiles (org/space/bug workflow constants)",
	Long: `Load tenant-specific Projex constants from ~/.config/yunxiao/profiles/<name>.json.

  yunxiao profile list
  yunxiao profile show [name]
  yunxiao profile use <name> [--dry-run] | --unset
  yunxiao profile path [name]
  yunxiao profile doctor [name] [--all-workflows]
  yunxiao profile install-example play|zhiyi [--force]
  yunxiao profile repo-add <alias> <repo> [--force]   # register codeup --repo alias (#125)

Select with --profile <name>, YUNXIAO_PROFILE=<name>, or once with
yunxiao profile use <name> (writes "profile" into config.json as the default
for every new shell session, #130). Precedence: --profile > YUNXIAO_PROFILE >
config default; profile use --unset clears the default.
Ship examples: profiles/play.example.json (generic/sandbox first), profiles/zhiyi.example.json (Zhiyi-oriented full fields; optional).
Examples are embedded in the binary (#92) and also ship in GitHub Release archives
as on-disk profiles/ next to the binary (#116); install-example works from any install.
A valid on-disk copy takes precedence; empty/truncated/name-mismatched copies are skipped (#104).`,
}

var profileUseCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Set the default profile in config.json (no per-shell export needed)",
	Long: `Risk: write
Writes "profile": "<name>" into ~/.config/yunxiao/config.json so new shells pick
the profile without --profile / YUNXIAO_PROFILE (#130 item 2).
The profile must already exist under ~/.config/yunxiao/profiles/<name>.json
(checked before writing; install with profile install-example). Precedence stays
--profile > YUNXIAO_PROFILE > this default, so one-off overrides still work.
--unset clears the default (name optional then). --dry-run previews only.

  yunxiao profile use play
  yunxiao profile use zhiyi --dry-run
  yunxiao profile use --unset`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		unset, _ := cmd.Flags().GetBool("unset")
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if unset {
			name = ""
		} else if name == "" {
			handleErr(fmt.Errorf("pass a profile name (see: yunxiao profile list), or --unset to clear the default"))
			return
		}
		f, cfgPath, err := config.LoadFile()
		if err != nil {
			handleErr(err)
			return
		}
		previous := f.Profile
		if name != "" {
			if name == "." || name == ".." || strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
				handleErr(fmt.Errorf("invalid profile name %q", name))
				return
			}
			if _, err := profile.Load(name); err != nil {
				handleErr(fmt.Errorf("profile %q not usable: %w (hint: yunxiao profile list; install with: yunxiao profile install-example %s)", name, err, name))
				return
			}
		}
		preview := map[string]any{"name": name, "config_path": cfgPath, "previous": previous, "unset": unset}
		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.Write), preview))
			return
		}
		f.Profile = name
		written, err := config.SaveFile(f)
		if err != nil {
			handleErr(err)
			return
		}
		out := map[string]any{
			"config_path": written,
			"profile":     name,
			"previous":    previous,
		}
		if unset {
			out["unset"] = true
			out["hint"] = "default profile cleared; --profile / YUNXIAO_PROFILE still work"
		} else {
			out["hint"] = "default set for new shells; precedence: --profile > YUNXIAO_PROFILE > config default"
		}
		handleErr(output.Success(out, map[string]any{"risk": risk.Write}))
	},
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed profiles",
	Long:  "Risk: read",
	Run: func(cmd *cobra.Command, args []string) {
		names, dir, err := profile.ListNames()
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(map[string]any{
			"profiles":     names,
			"profiles_dir": dir,
			"active":       activeProfileName(),
		}, map[string]any{"risk": risk.Read}))
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show a profile (default: active --profile / YUNXIAO_PROFILE)",
	Long:  "Risk: read",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		} else {
			name = activeProfileName()
		}
		if name == "" {
			handleErr(profile.HintMissing())
			return
		}
		p, err := profile.Load(name)
		if err != nil {
			handleErr(err)
			return
		}
		path, _ := profile.Path(name)
		// Never print raw PAT from profile show.
		if p.AccessToken != "" {
			p.AccessToken = config.MaskToken(p.AccessToken)
		}
		handleErr(output.Success(p, map[string]any{"risk": risk.Read, "path": path}))
	},
}

var profilePathCmd = &cobra.Command{
	Use:   "path [name]",
	Short: "Print profiles dir or a profile file path",
	Long:  "Risk: read",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			d, err := profile.Dir()
			if err != nil {
				handleErr(err)
				return
			}
			handleErr(output.Success(map[string]any{"path": d}, map[string]any{"risk": risk.Read}))
			return
		}
		p, err := profile.Path(args[0])
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(map[string]any{"path": p}, map[string]any{"risk": risk.Read}))
	},
}

var profileInstallExampleCmd = &cobra.Command{
	Use:   "install-example <name>",
	Short: "Copy profiles/<name>.example.json into ~/.config/yunxiao/profiles/",
	Long:  "Risk: write\nExamples ship embedded in the binary (zhiyi, play) and in release archives as profiles/ next to the binary (#92, #116); a valid on-disk copy wins, empty/truncated/name-mismatched disk copies are skipped (#104).\nExample: yunxiao profile install-example zhiyi",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")
		src, data, err := findProfileExample(name)
		if err != nil {
			handleErr(err)
			return
		}
		preview := map[string]any{"from": src, "name": name, "force": force}
		if globalDryRun {
			dst, _ := profile.Path(name)
			preview["to"] = dst
			handleErr(output.DryRunResult(string(risk.Write), preview))
			return
		}
		dst, err := profile.InstallExampleData(name, data, force)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(output.Success(map[string]any{
			"installed": dst,
			"hint":      fmt.Sprintf("example uses placeholders only — edit org/space/type IDs in %s, then refresh graphs via `workitem +explore-workflow --cleanup --write-profile --yes`. WARNING: that shortcut is a write operation (creates/moves/deletes a probe work item); run it in a sandbox project first and never auto-run it against a production space (`--dry-run` previews the plan offline). Verify with `profile doctor`; then: export YUNXIAO_PROFILE=%s  # or --profile %s", dst, name, name),
		}, map[string]any{"risk": risk.Write}))
	},
}

// profileExampleSearchDirs lists on-disk directories that may hold <name>.example.json
// (next to the binary). Tests override it. Cwd and runtime.Caller
// paths are intentionally omitted (#104): they are unreliable under -trimpath and can
// pick up empty/truncated junk from the working directory.
var profileExampleSearchDirs = defaultProfileExampleSearchDirs

func defaultProfileExampleSearchDirs() []string {
	dirs := []string{}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs,
			filepath.Join(filepath.Dir(exe), "profiles"),
			filepath.Join(filepath.Dir(exe), "..", "profiles"),
		)
	}
	return dirs
}

// profileExampleRawBase returns the GitHub raw URL prefix for the release tag matching
// this binary's version (v<Version>), so manual downloads stay aligned with the install (#104).
func profileExampleRawBase() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/v%s/profiles/", version.Version)
}

// validProfileExample reports whether data is usable as profiles/<name>.example.json:
// well-formed JSON with a top-level "name" equal to the requested example name (#104).
func validProfileExample(name string, data []byte) bool {
	if len(data) == 0 || !json.Valid(data) {
		return false
	}
	var doc struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return false
	}
	return doc.Name == name
}

// findProfileExample resolves <name>.example.json: a *valid* on-disk copy (release
// archive profiles/, next to the binary) wins; empty/truncated/name-mismatched disk
// files are skipped; otherwise the copy embedded in the binary is used (#92, #104, #116).
// Returns a display source (absolute path or "embedded:profiles/<file>") and content.
func findProfileExample(name string) (string, []byte, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
		return "", nil, fmt.Errorf("invalid example name %q (expected one of: %s)", name, strings.Join(examples.Names(), ", "))
	}
	filename := name + examples.ExampleSuffix
	for _, d := range profileExampleSearchDirs() {
		c := filepath.Join(d, filename)
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			b, err := os.ReadFile(c)
			if err != nil {
				return "", nil, fmt.Errorf("read example %s: %w", c, err)
			}
			if !validProfileExample(name, b) {
				continue
			}
			abs, _ := filepath.Abs(c)
			return abs, b, nil
		}
	}
	if b, err := examples.Example(name); err == nil {
		return "embedded:profiles/" + filename, b, nil
	}
	return "", nil, fmt.Errorf("example not found: profiles/%s (searched next to the binary and in embedded examples: %s); or download %s%s to ~/.config/yunxiao/profiles/%s.json",
		filename, strings.Join(examples.Names(), ", "), profileExampleRawBase(), filename, name)
}

func init() {
	profileInstallExampleCmd.Flags().Bool("force", false, "overwrite existing profile")
	profileUseCmd.Flags().Bool("unset", false, "clear the default profile instead of setting one")
	profileCmd.AddCommand(profileListCmd, profileShowCmd, profilePathCmd, profileUseCmd, profileInstallExampleCmd)
}
