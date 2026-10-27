#!/usr/bin/env bash
# Non-interactive Unix installer for WorkBuddy connector init (darwin|linux).
# Intended URL: https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/main/scripts/workbuddy-init-unix.sh
# Usage: bash workbuddy-init-unix.sh darwin|linux
# Release archives ship the binary as "yunxiao" (see .github/workflows/release.yml).
set -euo pipefail
OS="${1:-}"
case "$OS" in
  darwin|linux) ;;
  *) echo "usage: $0 darwin|linux" >&2; exit 2 ;;
esac
REPO="xiaoxiaolin0918/yunxiao-cli"
VER="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"v\?\([^"]*\)".*/\1/p' | head -1)"
if [[ -z "$VER" ]]; then
  echo "failed to resolve latest release tag" >&2
  exit 1
fi
ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  arm64|aarch64) A=arm64 ;;
  *) A=amd64 ;;
esac
ASSET="yunxiao-cli-${VER}-${OS}-${A}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VER}/${ASSET}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
curl -fsSL -o "$TMP/$ASSET" "$URL"
tar -xzf "$TMP/$ASSET" -C "$TMP"
# Prefer plain "yunxiao"; fall back to yunxiao-cli if a future asset renames.
BIN="$(find "$TMP" -type f \( -name yunxiao -o -name yunxiao-cli \) | head -1)"
test -n "$BIN"
mkdir -p "$HOME/.local/bin"
install -m 755 "$BIN" "$HOME/.local/bin/yunxiao"
ARCH_ROOT="$(dirname "$BIN")"
if [[ -d "$ARCH_ROOT/skills" ]]; then rm -rf "$HOME/.local/bin/skills"; cp -R "$ARCH_ROOT/skills" "$HOME/.local/bin/skills"; fi
if [[ -d "$ARCH_ROOT/profiles" ]]; then rm -rf "$HOME/.local/bin/profiles"; cp -R "$ARCH_ROOT/profiles" "$HOME/.local/bin/profiles"; fi
"$HOME/.local/bin/yunxiao" --version
echo "Installed to $HOME/.local/bin/yunxiao (ensure that dir is on PATH for WorkBuddy-spawned shells)"