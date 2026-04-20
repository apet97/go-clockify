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

# Goreleaser 2.x places raw binaries in per-build subdirectories
# (dist/clockify-mcp_linux_amd64_v1/clockify-mcp) and their cosign
# .sigstore.json siblings alongside them. SBOMs and SHA256SUMS.txt
# land at dist/ top level. The script used to assume everything was
# flat; v1.0.1's release workflow hit this and the 18 binary+sig
# assets were flagged missing even though goreleaser had already
# uploaded them under their correct published names.
#
# The authoritative mapping from "expected release asset name" to
# "local path goreleaser wrote" lives in dist/artifacts.json (the
# .name field vs the .path field). When that file exists we prefer
# it; otherwise we fall back to a recursive filesystem walk that
# accepts matches at any depth under dist/.

artifacts_json="$DIST/artifacts.json"

# Pass 1: every expected file must exist somewhere under dist/.
missing=()
for name in "${expected[@]}"; do
    found=0
    if [ -f "$artifacts_json" ]; then
        # Look for a .name match in artifacts.json and verify the
        # corresponding .path exists on disk. jq is present on the
        # ubuntu-22.04 runner image by default.
        if command -v jq >/dev/null 2>&1; then
            path=$(jq -r --arg n "$name" \
                '.[] | select(.name == $n) | .path' \
                "$artifacts_json" 2>/dev/null | head -n 1)
            if [ -n "$path" ] && [ -f "$path" ]; then
                found=1
            fi
        fi
    fi
    if [ "$found" -eq 0 ]; then
        # Fallback: recursive find by basename.
        if find "$DIST" -type f -name "$name" -print -quit | grep -q .; then
            found=1
        fi
    fi
    if [ "$found" -eq 0 ]; then
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

# Pass 2: count distinct published artifact names to catch accidental
# additions (extra SBOM format, rogue matrix entry, duplicate archive
# format). When artifacts.json is present we count its .name entries
# filtered to the clockify-mcp prefix + SHA256SUMS.txt; otherwise we
# walk dist/ recursively and deduplicate by basename.
# Published-artifact name shape:
#   SHA256SUMS.txt
#   clockify-mcp[-fips]-<os>-<arch>[.exe][.spdx.json|.sigstore.json]
# Goreleaser's intermediate binary IDs `clockify-mcp` and
# `clockify-mcp-fips` (no os/arch suffix) are NOT published assets and
# must be filtered out of the count.
ASSET_RE='^(clockify-mcp(-fips)?-(darwin|linux|windows)-(arm64|x64)(\.exe)?(\.spdx\.json|\.sigstore\.json)?|SHA256SUMS\.txt)$'

if [ -f "$artifacts_json" ] && command -v jq >/dev/null 2>&1; then
    found_names=()
    while IFS= read -r n; do
        found_names+=("$n")
    done < <(jq -r '.[].name' "$artifacts_json" | sort -u | grep -E "$ASSET_RE")
    found_count=${#found_names[@]}
else
    declare -A seen
    found_count=0
    found_names=()
    while IFS= read -r f; do
        base="$(basename "$f")"
        if [[ ! "$base" =~ $ASSET_RE ]]; then
            continue
        fi
        if [ -z "${seen[$base]:-}" ]; then
            seen[$base]=1
            found_count=$((found_count + 1))
            found_names+=("$base")
        fi
    done < <(find "$DIST" -type f)
fi

if [ "$found_count" -ne "$EXPECTED_COUNT" ]; then
    printf 'FAIL: found %d matching release artefacts under %s, expected %d\n' \
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
