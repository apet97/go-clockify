#!/usr/bin/env bash
# Downloads a pinned gitleaks release from GitHub and verifies its
# SHA256 before extracting. The version and digest below are the
# single source of truth -- bumping gitleaks is a deliberate
# supply-chain action that should happen in its own commit alongside
# a note on what changed in the gitleaks rules and why.
#
# Why a pinned tarball and not a third-party GitHub Action? Every
# action adds a transitive dependency to the CI supply chain.
# gitleaks' own release tarball comes from the same repo as the rules
# we are about to execute, so the blast radius of a compromised
# release is identical whether we pull it via `uses: gitleaks/...@sha`
# or via curl + sha256sum. The direct curl is one fewer link in the
# chain and is auditable from one shell script.
#
# Usage:
#   scripts/gitleaks-install.sh [install_dir]
# default install_dir: $TMPDIR/gitleaks-bin
#
# After install, the gitleaks binary lives at $install_dir/gitleaks.
# Callers typically want:
#   export PATH="$install_dir:$PATH"
#   gitleaks version  # should print the pinned version
#
# The script is idempotent: re-running with the same version skips
# the download if the binary already exists and passes the digest
# check.

set -euo pipefail

# ---- pin ----
GITLEAKS_VERSION="8.30.1"
# SHA256 of gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz
# obtained via:
#   gh api repos/zricethezav/gitleaks/releases/tags/v${GITLEAKS_VERSION} \
#     --jq '.assets[] | select(.name=="gitleaks_'${GITLEAKS_VERSION}'_linux_x64.tar.gz") | .digest'
GITLEAKS_LINUX_X64_SHA256="551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb"
# ---- /pin ----

INSTALL_DIR="${1:-${TMPDIR:-/tmp}/gitleaks-bin}"
mkdir -p "$INSTALL_DIR"

# Platform selection. CI runs linux_x64 on ubuntu-latest; local dev on
# mac/arm is supported best-effort by falling back to the homebrew
# install path. The CI path is the one the pin guards.
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$os-$arch" in
    linux-x86_64)
        asset="gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz"
        expected_sha="$GITLEAKS_LINUX_X64_SHA256"
        ;;
    *)
        echo "WARN: unsupported host ($os-$arch). Falling back to PATH gitleaks." >&2
        if ! command -v gitleaks >/dev/null 2>&1; then
            echo "ERROR: gitleaks not in PATH and this script only auto-installs on linux_x64." >&2
            echo "Install locally with: brew install gitleaks" >&2
            exit 2
        fi
        ln -sf "$(command -v gitleaks)" "$INSTALL_DIR/gitleaks"
        exit 0
        ;;
esac

dest="$INSTALL_DIR/$asset"
binary="$INSTALL_DIR/gitleaks"

if [ -x "$binary" ] && "$binary" version 2>/dev/null | grep -q "$GITLEAKS_VERSION"; then
    echo "gitleaks $GITLEAKS_VERSION already installed at $binary"
    exit 0
fi

url="https://github.com/zricethezav/gitleaks/releases/download/v${GITLEAKS_VERSION}/${asset}"
echo "Downloading $url"
curl -fsSL -o "$dest" "$url"

echo "Verifying SHA256"
actual_sha=$(shasum -a 256 "$dest" | awk '{print $1}')
if [ "$actual_sha" != "$expected_sha" ]; then
    printf 'FAIL: sha256 mismatch for %s\n' "$asset" >&2
    printf '  expected: %s\n' "$expected_sha" >&2
    printf '  actual:   %s\n' "$actual_sha" >&2
    rm -f "$dest"
    exit 1
fi

echo "Extracting"
tar -xzf "$dest" -C "$INSTALL_DIR" gitleaks
chmod +x "$binary"
rm -f "$dest"

"$binary" version
echo "gitleaks $GITLEAKS_VERSION installed at $binary"
