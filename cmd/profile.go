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
  yunxiao profile path [name]
  yunxiao profile doctor [name] [--all-workflows]
  yunxiao profile install-example zhiyi|play [--force]

Select with --profile <name> or YUNXIAO_PROFILE=<name>.
Ship examples: profiles/zhiyi.example.json (full Zhiyi fields), profiles/play.example.json (sandbox-minimal).
Examples are embedded in the binary, so install-example also works from npm / GitHub Release installs;
an on-disk profiles/<name>.example.json (npm package / next to the binary) takes precedence when it is valid JSON with matching "name".`,
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
	Long:  "Risk: write\nExamples ship embedded in the binary (zhiyi, play); a valid on-disk profiles/<name>.example.json (npm package / next to the binary) wins when present. Empty, truncated, or name-mismatched disk copies are skipped (#104).\nExample: yunxiao profile install-example zhiyi",
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
			"hint":      fmt.Sprintf("example uses placeholders only — edit org/space/type IDs in %s, refresh graphs via workitem +explore-workflow --write-profile, verify with profile doctor; then: export YUNXIAO_PROFILE=%s  # or --profile %s", dst, name, name),
		}, map[string]any{"risk": risk.Write}))
	},
}

// profileExampleSearchDirs lists on-disk directories that may hold <name>.example.json
// (next to the binary, npm package root). Tests override it. Cwd and runtime.Caller
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

// findProfileExample resolves <name>.example.json: a *valid* on-disk copy (npm package
// profiles/, next to the binary) wins; empty/truncated/name-mismatched disk files are
// skipped; otherwise the copy embedded in the binary is used (#92, #104).
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
	return "", nil, fmt.Errorf("example not found: profiles/%s (searched under binary/npm and embedded examples: %s); or download %s%s to ~/.config/yunxiao/profiles/%s.json",
		filename, strings.Join(examples.Names(), ", "), profileExampleRawBase(), filename, name)
}

func init() {
	profileInstallExampleCmd.Flags().Bool("force", false, "overwrite existing profile")
	profileCmd.AddCommand(profileListCmd, profileShowCmd, profilePathCmd, profileInstallExampleCmd)
}
