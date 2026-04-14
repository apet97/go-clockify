#!/usr/bin/env bash
# Guards against the v0.7.0 class of release bugs: goreleaser silently
# dropped all nine SBOMs because `sboms.artifacts: archive` didn't match
# the binary-format archives produced by the build matrix. The release
# shipped anyway — 19 assets instead of 28 — and operators didn't notice
# for several days. v0.7.1 fixed the goreleaser config by switching to
# `sboms.artifacts: binary`, but nothing in CI actually *counts* what
# goreleaser produced. A future filter regression would ship the same
# way.
#
# This script is the missing post-goreleaser gate. Called from
# `.github/workflows/release.yml` after `goreleaser release --clean`
# and before any step that uploads, signs, or publishes. If the asset
# shape doesn't match the expected matrix, the release workflow fails
# before tag-visible side effects happen.
#
# The check is structural: it enumerates the exact filenames the
# goreleaser config is contracted to produce (derived from the `archives`
# + `sboms` + `signs` blocks of .goreleaser.yaml) and verifies every one
# is present AND no extra clockify-mcp-* top-level artifact exists. This
# catches both drops (missing SBOM, missing signature) and accidents
# (second SBOM format, rogue matrix entry) at the source.
#
# Expected matrix (28 artifacts), derived from .goreleaser.yaml:
#   - 5 default binaries   (darwin-{arm64,x64}, linux-{arm64,x64}, windows-x64.exe)
#   - 4 FIPS binaries      (darwin-{arm64,x64}, linux-{arm64,x64}; windows skipped)
#   - 9 SPDX SBOMs         (one per binary, via syft, `*.spdx.json`)
#   - 9 sigstore bundles   (one per binary, via cosign, `*.sigstore.json`)
#   - 1 SHA256SUMS.txt
#
# Usage:
#   scripts/check-release-assets.sh            # checks ./dist
#   scripts/check-release-assets.sh /path/dir  # checks an explicit directory
#
# The directory argument makes the script testable end-to-end from a
# fixture layout without cutting a real release; the release pipeline
# calls it with no arguments so the default of `dist` applies.

set -euo pipefail

DIST="${1:-dist}"

if [ ! -d "$DIST" ]; then
    echo "ERROR: $DIST not found (expected goreleaser output directory)" >&2
    exit 2
fi

# Platforms that ship in both the default and FIPS build matrices.
# If .goreleaser.yaml's matrix changes, this array must move with it —
# the comment at the top is the source of truth and this list is derived.
DEFAULT_UNIX_PLATFORMS=(
    "darwin-arm64"
    "darwin-x64"
    "linux-arm64"
    "linux-x64"
)
DEFAULT_WINDOWS="windows-x64.exe"

FIPS_PLATFORMS=(
    "darwin-arm64"
    "darwin-x64"
    "linux-arm64"
    "linux-x64"
)

expected=()

# Default build: 4 unix binaries + 1 windows binary, each with SBOM and sig.
for p in "${DEFAULT_UNIX_PLATFORMS[@]}"; do
    expected+=("clockify-mcp-$p")
    expected+=("clockify-mcp-$p.spdx.json")
    expected+=("clockify-mcp-$p.sigstore.json")
done
expected+=("clockify-mcp-$DEFAULT_WINDOWS")
expected+=("clockify-mcp-$DEFAULT_WINDOWS.spdx.json")
expected+=("clockify-mcp-$DEFAULT_WINDOWS.sigstore.json")

# FIPS build: 4 unix binaries (windows/fips intentionally skipped in yaml).
for p in "${FIPS_PLATFORMS[@]}"; do
    expected+=("clockify-mcp-fips-$p")
    expected+=("clockify-mcp-fips-$p.spdx.json")
    expected+=("clockify-mcp-fips-$p.sigstore.json")
done

expected+=("SHA256SUMS.txt")

# Sanity-check: the array must have exactly 28 entries. If the matrix
# above is edited without updating this number (or vice versa), fail
# loudly so the mismatch can't ship.
EXPECTED_COUNT=28
if [ "${#expected[@]}" -ne "$EXPECTED_COUNT" ]; then
    printf 'BUG: expected array has %d entries, script says %d\n' \
        "${#expected[@]}" "$EXPECTED_COUNT" >&2
    echo "This is a script/matrix drift. Update both together." >&2
    exit 3
fi

# Pass 1: every expected file must exist.
missing=()
for name in "${expected[@]}"; do
    if [ ! -e "$DIST/$name" ]; then
        missing+=("$name")
    fi
done

if [ "${#missing[@]}" -gt 0 ]; then
    printf 'FAIL: %d expected release asset(s) missing from %s:\n' \
        "${#missing[@]}" "$DIST" >&2
    for m in "${missing[@]}"; do
        printf '  - %s\n' "$m" >&2
    done
    echo >&2
    echo "See docs/runbooks/release-asset-count.md for triage." >&2
    exit 1
fi

# Pass 2: count the top-level artifacts in dist/ that match the naming
# shape and assert the count is exactly EXPECTED_COUNT. This catches
# accidental additions (a new SBOM format, a rogue matrix entry, a
# duplicate archive format) that pass 1 wouldn't see.
#
# The glob `clockify-mcp-*` matches final artifacts (dashes) but NOT
# goreleaser's per-build scratch directories (underscores, e.g.
# `clockify-mcp_darwin_arm64_v8.0/`). `-d` is skipped just in case a
# future goreleaser version names a scratch dir with a dash.
found_count=0
found_names=()
shopt -s nullglob
for f in "$DIST"/clockify-mcp-* "$DIST"/SHA256SUMS.txt; do
    [ -d "$f" ] && continue
    found_count=$((found_count + 1))
    found_names+=("$(basename "$f")")
done
shopt -u nullglob

if [ "$found_count" -ne "$EXPECTED_COUNT" ]; then
    printf 'FAIL: found %d matching top-level files in %s, expected %d\n' \
        "$found_count" "$DIST" "$EXPECTED_COUNT" >&2
    echo "Artifact shape drift. Listing below:" >&2
    for n in "${found_names[@]}"; do
        printf '  - %s\n' "$n" >&2
    done
    echo >&2
    echo "See docs/runbooks/release-asset-count.md for triage." >&2
    exit 1
fi

printf 'OK: all %d expected release assets present in %s\n' \
    "$EXPECTED_COUNT" "$DIST"
