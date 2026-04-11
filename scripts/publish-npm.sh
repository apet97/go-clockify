#!/usr/bin/env bash
# publish-npm.sh — publish the six npm packages (5 platform + 1 base) for a
# clockify-mcp release. Called from .github/workflows/release.yml after
# goreleaser builds the binaries into dist/.
#
# Usage: scripts/publish-npm.sh <version>
#   <version> — the raw git ref (e.g. "v0.7.0"). The leading "v" is stripped
#               before it becomes the npm semver.
#
# Expects:
#   - dist/clockify-mcp_{os}_{arch}_*/clockify-mcp[.exe]  (goreleaser output)
#   - NODE_AUTH_TOKEN env var set to an npm automation token with publish
#     rights on the @apet97 scope. See docs/wave2-backlog.md W2-13.
#   - node >= 18 and npm >= 9 on PATH.

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi

RAW_VERSION="$1"
VERSION="${RAW_VERSION#v}"  # strip leading v — npm semver must be bare

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEMPLATE="$REPO_ROOT/npm/package.json.tmpl"
BASE_DIR="$REPO_ROOT/npm/clockify-mcp-go"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

if [ ! -f "$TEMPLATE" ]; then
  echo "error: $TEMPLATE not found" >&2
  exit 1
fi
if [ ! -d "$BASE_DIR" ]; then
  echo "error: $BASE_DIR not found" >&2
  exit 1
fi

# Platform table: (suffix, os, cpu, bin_path_relative_to_repo_root, bin_filename, description)
# suffix is the npm package suffix (e.g. "darwin-arm64" becomes
# @apet97/clockify-mcp-go-darwin-arm64). bin_path is the goreleaser output
# location; bin_filename is what ends up inside the package's bin/ dir.
PLATFORMS=(
  "darwin-arm64|darwin|arm64|dist/clockify-mcp_darwin_arm64_v8.0/clockify-mcp|clockify-mcp|macOS (Apple silicon)"
  "darwin-x64|darwin|x64|dist/clockify-mcp_darwin_amd64_v1/clockify-mcp|clockify-mcp|macOS (Intel)"
  "linux-x64|linux|x64|dist/clockify-mcp_linux_amd64_v1/clockify-mcp|clockify-mcp|Linux (x86_64)"
  "linux-arm64|linux|arm64|dist/clockify-mcp_linux_arm64_v8.0/clockify-mcp|clockify-mcp|Linux (arm64)"
  "windows-x64|win32|x64|dist/clockify-mcp_windows_amd64_v1/clockify-mcp.exe|clockify-mcp.exe|Windows (x86_64)"
)

publish_platform() {
  local suffix="$1" os="$2" cpu="$3" src="$4" bin_name="$5" description="$6"
  local pkg_dir="$WORK_DIR/platform-$suffix"
  local pkg_name="@apet97/clockify-mcp-go-$suffix"

  echo "[npm] staging $pkg_name@$VERSION from $src"
  if [ ! -f "$REPO_ROOT/$src" ]; then
    echo "error: expected binary $REPO_ROOT/$src not found — did goreleaser run?" >&2
    exit 1
  fi

  mkdir -p "$pkg_dir/bin"
  cp "$REPO_ROOT/$src" "$pkg_dir/bin/$bin_name"
  chmod +x "$pkg_dir/bin/$bin_name"

  # Substitute placeholders in the template. PLATFORM_DESCRIPTION must
  # be replaced before PLATFORM, otherwise the PLATFORM pass mangles
  # the longer placeholder into "<suffix>_DESCRIPTION" (which is what
  # v0.7.1 shipped — every description read as "linux-x64_DESCRIPTION"
  # etc. in `npm view`). The order below fixes it forward; v0.7.1 is
  # immutable. Using | as the sed delimiter so the description (which
  # may contain slashes or spaces) is safe.
  sed \
    -e "s|PLATFORM_DESCRIPTION|$description|g" \
    -e "s|PLATFORM|$suffix|g" \
    -e "s|VERSION|$VERSION|g" \
    -e "s|OS_VALUE|$os|g" \
    -e "s|CPU_VALUE|$cpu|g" \
    "$TEMPLATE" > "$pkg_dir/package.json"

  (cd "$pkg_dir" && npm publish --access public)
}

publish_base() {
  local pkg_dir="$WORK_DIR/base"
  cp -R "$BASE_DIR" "$pkg_dir"
  # Swap VERSION in place in every JSON file.
  find "$pkg_dir" -type f -name '*.json' -print0 |
    xargs -0 sed -i.bak -e "s|VERSION|$VERSION|g"
  find "$pkg_dir" -type f -name '*.bak' -delete
  chmod +x "$pkg_dir/bin/clockify-mcp.js"
  echo "[npm] publishing base package @apet97/clockify-mcp-go@$VERSION"
  (cd "$pkg_dir" && npm publish --access public)
}

for row in "${PLATFORMS[@]}"; do
  IFS='|' read -r suffix os cpu src bin_name description <<< "$row"
  publish_platform "$suffix" "$os" "$cpu" "$src" "$bin_name" "$description"
done

publish_base

# Workaround for npm's "tarball uploaded but version index stuck at 404"
# failure mode we hit during the v0.7.1 cutover (2026-04-11). The dispatcher
# package tarball was retrievable via direct URL but the package metadata
# document returned 404 for `npm install`, `npm view`, and every GET.
# Running a no-op `npm deprecate` against a version range that matches
# nothing forces npm to rewrite the metadata document, which clears the
# stuck 404 within seconds. Idempotent — no effect if the metadata is
# already healthy. `|| true` so a failed nudge doesn't abort the release
# (the packages are already published at this point).
echo "[npm] nudging dispatcher metadata to clear any stale 404 cache"
npm deprecate "@apet97/clockify-mcp-go@<0.0.1" "" 2>/dev/null || true

echo "[npm] all packages published for $VERSION"
