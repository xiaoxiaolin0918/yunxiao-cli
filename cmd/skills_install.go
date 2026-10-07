package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
)

var (
	skillsInstallDir     string
	skillsInstallNames   []string
	skillsInstallSymlink bool
	skillsInstallForce   bool
)

type skillInstallItem struct {
	Name string `json:"name"`
	From string `json:"from"`
	To   string `json:"to"`
	Mode string `json:"mode"` // copy | symlink | skipped reason in Mode for skipped? use Reason
}

type skillSkipItem struct {
	Name   string `json:"name"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason"`
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install companion skills into the agent skills directory",
	Long: `Install skills from the repo skills/yunxiao-* directories into the user's
agent skills directory (default: ~/.agents/skills) so agents can discover them.

Same layout as "npx skills add". Default mode is recursive copy; use --symlink
to link instead. Existing target skill dirs are skipped unless --force: every
skip is reported in data.skipped (reason "exists"), counted in
installed_count/skipped_count, and warned on stderr with a --force hint, so
"installed 0" is never mistaken for a successful refresh (#119).

Risk: write`,
	Run: func(cmd *cobra.Command, args []string) {
		handleErr(runSkillsInstall())
	},
}

func defaultSkillsInstallDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".agents", "skills")
	}
	return filepath.Join(home, ".agents", "skills")
}

// discoverInstallableSkills returns yunxiao-* dirs under root that contain SKILL.md.
// If filter is non-empty, only those names (must still exist and have SKILL.md).
func discoverInstallableSkills(root string, filter []string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("skills source %s: %w", root, err)
	}
	want := map[string]bool{}
	for _, n := range filter {
		n = strings.TrimSpace(n)
		if n != "" {
			want[n] = true
		}
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "yunxiao-") {
			continue
		}
		if len(want) > 0 && !want[e.Name()] {
			continue
		}
		skillMD := filepath.Join(root, e.Name(), "SKILL.md")
		if st, err := os.Stat(skillMD); err != nil || st.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(want) > 0 {
		found := map[string]bool{}
		for _, n := range names {
			found[n] = true
		}
		var missing []string
		for n := range want {
			if !found[n] {
				missing = append(missing, n)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("skill(s) not found or missing SKILL.md: %s", strings.Join(missing, ", "))
		}
	}
	return names, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDirRecursive(src, dst string) error {
	src = filepath.Clean(src)
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		return copyFile(path, target, mode)
	})
}

func installOneSkill(src, dst string, symlink, force, dryRun bool) (installed *skillInstallItem, skipped *skillSkipItem, err error) {
	mode := "copy"
	if symlink {
		mode = "symlink"
	}
	item := &skillInstallItem{Name: filepath.Base(src), From: src, To: dst, Mode: mode}

	if _, err := os.Lstat(dst); err == nil {
		if !force {
			return nil, &skillSkipItem{
				Name:   item.Name,
				From:   src,
				To:     dst,
				Reason: "exists",
			}, nil
		}
		if dryRun {
			return item, nil, nil
		}
		if err := os.RemoveAll(dst); err != nil {
			return nil, nil, fmt.Errorf("remove existing %s: %w", dst, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, err
	}

	if dryRun {
		return item, nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, nil, err
	}
	if symlink {
		absSrc, err := filepath.Abs(src)
		if err != nil {
			return nil, nil, err
		}
		if err := os.Symlink(absSrc, dst); err != nil {
			return nil, nil, fmt.Errorf("symlink %s -> %s: %w", dst, absSrc, err)
		}
		return item, nil, nil
	}
	if err := copyDirRecursive(src, dst); err != nil {
		return nil, nil, fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	return item, nil, nil
}

func runSkillsInstall() error {
	targetDir := skillsInstallDir
	if targetDir == "" {
		targetDir = defaultSkillsInstallDir()
	}
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	return installSkills(skillsRoot(), absTarget, skillsInstallNames, skillsInstallSymlink, skillsInstallForce, globalDryRun)
}

// installSkills copies/links yunxiao-* skills from srcRoot into targetDir and
// reports the result truthfully (#119): every existing target dir skipped
// without --force lands in data.skipped (reason "exists") plus a stderr
// warning with the --force remedy — never a silent installed:0/skipped:[].
func installSkills(srcRoot, targetDir string, filter []string, symlink, force, dryRun bool) error {
	names, err := discoverInstallableSkills(srcRoot, filter)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no installable yunxiao-* skills with SKILL.md under %s", srcRoot)
	}

	installed := []skillInstallItem{}
	skipped := []skillSkipItem{}

	for _, name := range names {
		from := filepath.Join(srcRoot, name)
		to := filepath.Join(targetDir, name)
		ins, sk, err := installOneSkill(from, to, symlink, force, dryRun)
		if err != nil {
			return err
		}
		if sk != nil {
			skipped = append(skipped, *sk)
		}
		if ins != nil {
			installed = append(installed, *ins)
		}
	}

	if len(skipped) > 0 {
		skippedNames := make([]string, 0, len(skipped))
		for _, s := range skipped {
			skippedNames = append(skippedNames, s.Name)
		}
		fmt.Fprintf(output.Stderr, "warning: skills install skipped %d existing skill(s) under %s: %s (rerun with --force to replace them)\n",
			len(skipped), targetDir, strings.Join(skippedNames, ", "))
	}

	result := map[string]any{
		"installed":       installed,
		"skipped":         skipped,
		"installed_count": len(installed),
		"skipped_count":   len(skipped),
		"target_dir":      targetDir,
	}
	if len(skipped) > 0 {
		result["hint"] = "skipped skills already exist at the target dir; rerun with --force to replace them"
	}
	if dryRun {
		return output.DryRunResult("write", result)
	}
	return output.Success(result, nil)
}

func init() {
	skillsInstallCmd.Flags().StringVar(&skillsInstallDir, "dir", "", "install root (default: ~/.agents/skills)")
	skillsInstallCmd.Flags().StringArrayVar(&skillsInstallNames, "skill", nil, "install only named skill (repeatable)")
	skillsInstallCmd.Flags().BoolVar(&skillsInstallSymlink, "symlink", false, "symlink instead of copy")
	skillsInstallCmd.Flags().BoolVar(&skillsInstallForce, "force", false, "replace existing target skill dirs")
}
