package update

import (
	"fmt"
	"runtime"
)

const (
	ArchivePrefix = "yunxiao-cli"
	BinaryBase    = "yunxiao"
	DefaultRepo   = "xiaoxiaolin0918/yunxiao-cli"
)

// Platform returns the release platform name (darwin|linux|windows) for goos.
func Platform(goos string) (string, error) {
	switch goos {
	case "darwin":
		return "darwin", nil
	case "linux":
		return "linux", nil
	case "windows":
		return "windows", nil
	default:
		return "", fmt.Errorf("unsupported OS %q (need darwin/linux/windows)", goos)
	}
}

// Arch returns the release arch name (amd64|arm64) for goarch.
func Arch(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported arch %q (need amd64/arm64)", goarch)
	}
}

// CurrentPlatformArch uses runtime.GOOS / GOARCH.
func CurrentPlatformArch() (platform, arch string, err error) {
	platform, err = Platform(runtime.GOOS)
	if err != nil {
		return "", "", err
	}
	arch, err = Arch(runtime.GOARCH)
	if err != nil {
		return "", "", err
	}
	return platform, arch, nil
}

// ArchiveName matches the release archive naming (darwin/linux/windows × amd64/arm64):
// yunxiao-cli-{version}-{platform}-{arch}.tar.gz|.zip
func ArchiveName(version, platform, arch string) string {
	version = NormalizeVersion(version)
	ext := ".tar.gz"
	if platform == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("%s-%s-%s-%s%s", ArchivePrefix, version, platform, arch, ext)
}

// BinaryName is yunxiao or yunxiao.exe.
func BinaryName(platform string) string {
	if platform == "windows" {
		return BinaryBase + ".exe"
	}
	return BinaryBase
}

// SelectPlatformAsset returns the archive filename for version + GOOS/GOARCH
// (or release platform/arch names — they match).
func SelectPlatformAsset(version, platformOrGOOS, archOrGOARCH string) (string, error) {
	platform, err := Platform(platformOrGOOS)
	if err != nil {
		return "", err
	}
	arch, err := Arch(archOrGOARCH)
	if err != nil {
		return "", err
	}
	return ArchiveName(version, platform, arch), nil
}
