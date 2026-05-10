#!/usr/bin/env bash
#
# test-release-workflow-structure.sh — pin the post-v1.2.1 release
# infrastructure so future workflow refactors cannot silently drop:
#
#   1. PR #92's GoReleaser tag-pinning fix in `.github/workflows/release.yml`.
#      Without GORELEASER_CURRENT_TAG, goreleaser can resolve the wrong
#      tag when a final release peels to the same commit as its rc
#      (the v1.2.1 retry incident), upload to the wrong release, and
#      fail with 422 already_exists.
#
#   2. PR #95's two-mode cosign verification in
#      `.github/workflows/release-smoke.yml`. The canonical mode pins
#      `release.yml@refs/tags/.*`; the manual-retry-exception mode is
#      a workflow_dispatch-only opt-in with a literal SAN. Both must
#      survive future edits.
#
#   3. The bounded-marker wording around the v1.2.1 release-smoke SAN
#      exception in `docs/runbooks/release-candidate-evidence.md` and
#      `docs/verification.md`. The exception is bounded to v1.2.1 only
#      and future releases must revert to canonical regex; if those
#      bounded markers ever get removed, the exception silently
#      generalises into precedent. The guard fails closed.
#
# This script is a pure assertion script — no harness, no fixtures.
# It checks real files in the repo. Wired into `make script-tests`.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
release_yml="$repo_root/.github/workflows/release.yml"
release_smoke_yml="$repo_root/.github/workflows/release-smoke.yml"
evidence_md="$repo_root/docs/runbooks/release-candidate-evidence.md"
verification_md="$repo_root/docs/verification.md"

fail=0

err() {
  printf 'ERROR: %s\n' "$1" >&2
  fail=1
}

# Asserts a literal fixed-string is present in $2 (file path) and prints
# a uniform OK / ERROR line.
require_fixed() {
  local label="$1"
  local file="$2"
  local needle="$3"
  if [ ! -f "$file" ]; then
    err "${label}: ${file} not found"
    return
  fi
  if grep -qF -- "$needle" "$file"; then
    printf 'OK: %s\n' "$label"
  else
    err "${label}: missing required marker in ${file}: ${needle}"
  fi
}

# Asserts an extended-regex pattern matches in $2 (file path).
require_regex() {
  local label="$1"
  local file="$2"
  local pattern="$3"
  if [ ! -f "$file" ]; then
    err "${label}: ${file} not found"
    return
  fi
  if grep -qE -- "$pattern" "$file"; then
    printf 'OK: %s\n' "$label"
  else
    err "${label}: missing required pattern in ${file}: ${pattern}"
  fi
}

# 1. release.yml — PR #92 GORELEASER_CURRENT_TAG pin contract.
require_fixed "release.yml carries id: resolve-release-tag" \
  "$release_yml" \
  "id: resolve-release-tag"
require_fixed "release.yml pins GORELEASER_CURRENT_TAG to resolved tag" \
  "$release_yml" \
  "GORELEASER_CURRENT_TAG: \${{ steps.resolve-release-tag.outputs.release_tag }}"
require_fixed "release.yml pins v1.2.1 peeled commit guard" \
  "$release_yml" \
  "ce56414ae012c4a49d21ae0a319b178619c5966a"

# 2. release-smoke.yml — PR #95 two-mode cosign verification contract.
require_fixed "release-smoke.yml exposes verification_mode input" \
  "$release_smoke_yml" \
  "verification_mode:"
require_fixed "release-smoke.yml lists canonical option" \
  "$release_smoke_yml" \
  "- canonical"
require_fixed "release-smoke.yml lists manual-retry-exception option" \
  "$release_smoke_yml" \
  "- manual-retry-exception"
require_fixed "release-smoke.yml exposes expected_workflow_ref input" \
  "$release_smoke_yml" \
  "expected_workflow_ref:"
require_fixed "release-smoke.yml resolve step writes IDENT_MODE/value outputs" \
  "$release_smoke_yml" \
  "id: ident"
require_regex "release-smoke.yml branches verify-blob on IDENT_MODE regex/literal" \
  "$release_smoke_yml" \
  'case "\$IDENT_MODE" in'
require_fixed "release-smoke.yml emits loud manual-retry exception notice" \
  "$release_smoke_yml" \
  "::notice title=Manual-retry SAN exception in effect::"
require_fixed "release-smoke.yml strips v-prefix on container image tag" \
  "$release_smoke_yml" \
  "image_tag=\"\${TAG#v}\""

# 3. v1.2.1 SAN-exception bounded markers — must stay tightly bounded
#    so the exception does not generalise into precedent.
require_fixed "release-candidate-evidence.md contains bounded-to-v1.2.1 marker" \
  "$evidence_md" \
  "Accepted exception for v1.2.1 only"
require_fixed "release-candidate-evidence.md contains revert-to-canonical marker" \
  "$evidence_md" \
  "Future releases that follow a clean tag-push path will revert to the canonical"
require_fixed "verification.md documents two cosign-verify modes" \
  "$verification_md" \
  "## Two cosign-verify modes (canonical vs manual-retry exception)"

if [ "$fail" -ne 0 ]; then
  printf 'release-workflow-structure tests: FAIL\n' >&2
  exit 1
fi

printf 'release-workflow-structure tests: OK\n'
