#!/usr/bin/env bash
# Verify the npm publish preconditions for a released version (#105).
# Usage: scripts/npm-publish-verify.sh [v]X.Y.Z
# Does not publish. Expects to run on a checkout whose npm/checksums.txt
# already matches the GitHub Release (usually main after chore/checksums-*).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
raw="${1:?usage: $0 [v]X.Y.Z}"
ver="${raw#v}"
tag_name="v${ver}"

pkg_ver="$(node -p "require('./npm/package.json').version")"
if [[ "$pkg_ver" != "$ver" ]]; then
  echo "npm/package.json version (${pkg_ver}) != ${ver}" >&2
  exit 1
fi
echo "OK package.json=${pkg_ver}"

if ! grep -F "yunxiao-cli-${ver}-" npm/checksums.txt; then
  echo "npm/checksums.txt missing yunxiao-cli-${ver}-" >&2
  exit 1
fi
echo "OK checksums list version ${ver}"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
gh release download "$tag_name" --repo xiaoxiaolin0918/yunxiao-cli --pattern checksums.txt --output "$tmp"
tr -d "\r" < "$tmp" > "${tmp}.lf"
tr -d "\r" < npm/checksums.txt > "${tmp}.repo.lf"
diff -u "${tmp}.lf" "${tmp}.repo.lf"
echo "OK matches Release ${tag_name} checksums.txt"

(cd npm && npm publish --dry-run)
echo "OK npm publish --dry-run"
