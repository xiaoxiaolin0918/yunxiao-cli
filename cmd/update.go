package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/update"
	"github.com/yunxiao-cli/yunxiao/internal/version"
)

var updateCheckOnly bool

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"self-update"},
	Short:   "从 GitHub Releases 自更新 yunxiao 二进制",
	Long: `Risk: write（替换本地 yunxiao 可执行文件）

对比当前版本与 GitHub Releases 最新版，下载匹配平台归档
（windows/linux/darwin × amd64/arm64），有 checksums.txt 时校验 SHA-256，
并安全替换本二进制（先写旁边再 rename；Windows 友好）。

  yunxiao update --check          # 仅检查；有新版本则 exit 2
  yunxiao update --dry-run        # 同 --check
  yunxiao update                  # TTY：确认后替换；非 TTY 需 --yes
  yunxiao update --yes            # 无提示直接更新（脚本/CI）

其他命令可能每 24h 最多检查一次网络；仅在发现新版本时才在 stderr 打印中文更新提示（json / update / completion 会跳过）。
可选：yunxiao doctor --check-update。
关闭机会性提示：YUNXIAO_UPDATE_CHECK=0。

覆盖发布源：
  YUNXIAO_CLI_GITHUB_REPO=owner/repo
  YUNXIAO_CLI_DOWNLOAD_BASE=https://example.com/path

更新时除替换二进制外，还会把归档内的 skills/ 与 profiles/ 解压到可执行文件同目录（覆盖旧目录；skills install 源目录随之刷新）。

GitHub Releases 是唯一安装渠道（npm 薄包装已停用，#115）；从旧 npm 渠道迁移：卸载后从
https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest 下载归档覆盖安装。`,
	Run: func(cmd *cobra.Command, args []string) {
		handleErr(runUpdate(cmd))
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "仅检查是否有新版本（不下载）；有则 exit 2")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command) error {
	checkOnly := updateCheckOnly || globalDryRun

	platform, arch, err := update.CurrentPlatformArch()
	if err != nil {
		return err
	}
	exe, err := update.ExecutablePath()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	client := update.DefaultHTTPClient()
	repo := update.GithubRepo()
	rel, err := update.FetchLatestRelease(cmd.Context(), client, repo)
	if err != nil {
		return err
	}
	latest := update.NormalizeVersion(rel.TagName)
	current := update.NormalizeVersion(version.Version)
	newer := update.NewerAvailable(current, latest)

	meta := map[string]any{
		"current":          current,
		"latest":           latest,
		"tag":              rel.TagName,
		"repo":             repo,
		"platform":         platform,
		"arch":             arch,
		"executable":       exe,
		"release_url":      rel.HTMLURL,
		"update_available": newer,
	}

	if checkOnly {
		data := map[string]any{
			"status":  update.FormatPair(current, latest),
			"message": update.FormatPair(current, latest),
		}
		if err := output.Success(data, meta); err != nil {
			return err
		}
		if newer {
			return output.ExitError{Code: update.ExitUpdateAvailable, Msg: "update available"}
		}
		return nil
	}

	if !newer {
		return output.Success(map[string]any{
			"status":  "already_latest",
			"message": update.FormatPair(current, latest),
		}, meta)
	}

	archiveName := update.ArchiveName(latest, platform, arch)
	asset, err := update.FindAsset(rel, archiveName)
	if err != nil {
		return err
	}
	meta["archive"] = archiveName

	if err := confirmSelfUpdate(latest, exe, globalYes); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "yunxiao-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	urls := update.ArchiveDownloadCandidates(archiveName, asset.BrowserDownloadURL)
	usedURL, err := update.DownloadFirstOK(cmd.Context(), client, urls, archivePath)
	if err != nil {
		return err
	}
	meta["download_url"] = usedURL

	sums, sumErr := update.FetchChecksums(cmd.Context(), client, rel)
	if err := verifyArchiveChecksum(archivePath, archiveName, sums, sumErr, meta); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "extract")
	binPath, err := update.ExtractBinary(archivePath, extractDir, platform)
	if err != nil {
		return err
	}
	sidecars, sideErr := update.ExtractSidecars(archivePath, extractDir)
	if sideErr != nil {
		return fmt.Errorf("extract skills/profiles: %w", sideErr)
	}
	if err := update.ReplaceExecutable(exe, binPath); err != nil {
		return err
	}
	installed, installErr := update.InstallSidecars(filepath.Dir(exe), extractDir, sidecars)
	if installErr != nil {
		meta["sidecars_warning"] = installErr.Error()
	} else if len(installed) > 0 {
		meta["sidecars_installed"] = installed
		for _, name := range installed {
			meta[name+"_dir"] = filepath.Join(filepath.Dir(exe), name)
		}
	} else {
		meta["sidecars_note"] = "archive had no skills/ or profiles/; binary-only update"
	}
	if err := update.VerifyBinaryVersion(exe, latest); err != nil {
		meta["verify_warning"] = err.Error()
		meta["hint"] = "binary replaced; restart the process if --version still shows the old build (common on Windows)"
	} else {
		meta["verified_version"] = latest
	}

	return output.Success(map[string]any{
		"status":  "updated",
		"message": fmt.Sprintf("updated %s → %s", current, latest),
		"path":    exe,
	}, meta)
}

func verifyArchiveChecksum(archivePath, archiveName string, sums map[string]string, fetchErr error, meta map[string]any) error {
	if fetchErr != nil {
		meta["checksum_ok"] = false
		meta["checksum_note"] = "checksums.txt not available; skipped verify"
		return nil
	}

	expected, ok := sums[archiveName]
	if !ok {
		return fmt.Errorf("checksums.txt missing checksum for %q", archiveName)
	}
	if err := update.VerifyChecksum(archivePath, expected); err != nil {
		return err
	}
	meta["checksum_ok"] = true
	return nil
}

func confirmSelfUpdate(latest, exe string, yes bool) error {
	if yes {
		return nil
	}
	if stdinIsInteractive() {
		fmt.Fprintf(os.Stderr, "Update yunxiao to %s?\n  replace: %s\nConfirm [y/N]: ", latest, exe)
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans == "y" || ans == "yes" {
			return nil
		}
		return output.Fail(output.ErrorBody{
			Type:    "cli",
			Message: "update cancelled",
		}, 1)
	}
	return risk.CheckConfirmed("yunxiao update (replace local binary)", risk.Write, false)
}
