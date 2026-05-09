#!/usr/bin/env bash
#
# check-launch-review-ledger.sh
#
# Keeps docs/launch-readiness-review-may-8.md honest against the
# May 8 review bundle. The review source directory lives outside this
# repository, so the expected ID inventory is pinned here: T-01..T-24,
# MP-01..MP-13, P1-1..P1-10, P2-1..P2-10, P3-1..P3-7,
# D1..D10, G-01..G-05, L-01..L-05, L-08, and L-10.
#
# The gate intentionally ignores the ledger's "Review ID coverage
# audit" and "Verification used" sections. Those sections summarize
# the coverage and command history; they must not be enough to satisfy
# a per-finding disposition. When REVIEW_SOURCE_DIR or
# ~/Downloads/review may 8 exists, the gate also compares the pinned
# inventory against the actual source bundle; CI remains self-contained
# by falling back to the pinned inventory when the external folder is
# absent.

set -euo pipefail

ledger="${1:-docs/launch-readiness-review-may-8.md}"
source_dir="${REVIEW_SOURCE_DIR:-${2:-$HOME/Downloads/review may 8}}"
fail=0

err() {
  echo "[fail] $*" >&2
  fail=1
}

if [ ! -f "$ledger" ]; then
  err "launch review ledger missing: $ledger"
  echo "launch-review-ledger: FAIL" >&2
  exit 1
fi

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/launch-review-ledger.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

normalized="$tmpdir/ledger.normalized.md"
disposition="$tmpdir/ledger.disposition.md"
expected="$tmpdir/expected.ids"
present="$tmpdir/present.ids"
source_ids="$tmpdir/source.ids"

source_files=(
  "00_COORDINATOR_INDEX.md"
  "01_MCP_PROTOCOL_TRANSPORTS.md"
  "02_SECURITY_AUTH_TENANT_ISOLATION.md"
  "10_FINAL_INTEGRATED_LAUNCH_PLAN.md"
)

# Normalize common Unicode dash characters that appear in copied review
# IDs so `T-01` and `T<non-breaking-hyphen>01` compare the same.
perl -CSDA -pe 's/[\x{2010}\x{2011}\x{2012}\x{2013}\x{2014}]/-/g' "$ledger" > "$normalized"

awk '
  /^## Review ID coverage audit[[:space:]]*$/ { skip = 1; next }
  /^## Deferred low-risk follow-ups[[:space:]]*$/ { skip = 0 }
  /^## Verification used for this pass[[:space:]]*$/ { skip = 1; next }
  !skip { print }
' "$normalized" > "$disposition"

{
  i=1
  while [ "$i" -le 24 ]; do
    printf 'T-%02d\n' "$i"
    i=$((i + 1))
  done
  i=1
  while [ "$i" -le 13 ]; do
    printf 'MP-%02d\n' "$i"
    i=$((i + 1))
  done
  tier=1
  while [ "$tier" -le 2 ]; do
    i=1
    while [ "$i" -le 10 ]; do
      printf 'P%s-%d\n' "$tier" "$i"
      i=$((i + 1))
    done
    tier=$((tier + 1))
  done
  i=1
  while [ "$i" -le 7 ]; do
    printf 'P3-%d\n' "$i"
    i=$((i + 1))
  done
	  i=1
	  while [ "$i" -le 10 ]; do
	    printf 'D%d\n' "$i"
	    i=$((i + 1))
	  done
	  i=1
	  while [ "$i" -le 5 ]; do
	    printf 'G-%02d\n' "$i"
	    i=$((i + 1))
	  done
	  i=1
	  while [ "$i" -le 5 ]; do
	    printf 'L-%02d\n' "$i"
	    i=$((i + 1))
	  done
	  printf 'L-08\n'
	  printf 'L-10\n'
	} | sort -u > "$expected"

if [ -d "$source_dir" ]; then
  source_args=()
  for source in "${source_files[@]}"; do
    if [ ! -f "$source_dir/$source" ]; then
      err "review source file missing from source bundle: $source_dir/$source"
      continue
    fi
    source_args+=("$source_dir/$source")
  done

  if [ "${#source_args[@]}" -gt 0 ]; then
    perl -CSDA -ne 's/[\x{2010}\x{2011}\x{2012}\x{2013}\x{2014}]/-/g; while(/\b(T-[0-9]{2}|MP-[0-9]{2}|P[123]-[0-9]+|D[0-9]+|G-[0-9]{2}|L-[0-9]{2})\b/g){print "$1\n"}' "${source_args[@]}" \
      | sort -u > "$source_ids"

    unpinned_source_ids="$(comm -23 "$source_ids" "$expected" || true)"
    stale_pinned_ids="$(comm -23 "$expected" "$source_ids" || true)"

    if [ -n "$unpinned_source_ids" ]; then
      while IFS= read -r id; do
        [ -n "$id" ] || continue
        err "review source ID is not pinned in launch-review-ledger gate: $id"
      done <<< "$unpinned_source_ids"
    fi

    if [ -n "$stale_pinned_ids" ]; then
      while IFS= read -r id; do
        [ -n "$id" ] || continue
        err "pinned review ID not found in source bundle: $id"
      done <<< "$stale_pinned_ids"
    fi
  fi
fi

	grep -Eho '\b(T-[0-9]{2}|MP-[0-9]{2}|P[123]-[0-9]+|D[0-9]+|G-[0-9]{2}|L-[0-9]{2})\b' "$disposition" \
	  | sort -u > "$present" || true

missing="$(comm -23 "$expected" "$present" || true)"
extra="$(comm -13 "$expected" "$present" || true)"

if [ -n "$missing" ]; then
  while IFS= read -r id; do
    [ -n "$id" ] || continue
    err "review finding missing a disposition outside summary sections: $id"
  done <<< "$missing"
fi

if [ -n "$extra" ]; then
  while IFS= read -r id; do
    [ -n "$id" ] || continue
    err "unexpected review finding ID in disposition ledger: $id"
  done <<< "$extra"
fi

for source in "${source_files[@]}"; do
  if ! grep -qF "$source" "$normalized"; then
    err "source inventory missing review file: $source"
  fi
done

required_appendix_b_patterns=(
  "B.03 live-contract coverage inventory."
  "B.04 tool contract matrix."
  "B.05 performance and large-workspace evidence."
  "B.06 CI/release evidence pipeline."
  "B.07 operator-facing marker audit."
  "B.07 per-package coverage outlier."
  "B.07 proposed-ADR audit."
  "B.07 \`internal/jsonschema\` architecture trade-off."
  "B.08 dependency/license evidence path."
)

for pattern in "${required_appendix_b_patterns[@]}"; do
  if ! grep -qF "$pattern" "$normalized"; then
    err "Appendix B open-question disposition missing required text: $pattern"
  fi
done

required_coordinator_appendix_patterns=(
  "Coordinator Appendix B file-scope audit."
  "files this coordinator did NOT deeply read"
  "transport/session code"
  "auth/policy"
  "enforcement"
  "Clockify pagination"
  "GoReleaser/release assets"
  "deploy manifests"
  "API coverage"
  "CODEOWNERS"
)

for pattern in "${required_coordinator_appendix_patterns[@]}"; do
  if ! grep -qF "$pattern" "$normalized"; then
    err "coordinator Appendix B file-scope disposition missing required text: $pattern"
  fi
done

required_final_plan_checklist_patterns=(
  "Final integrated plan checklist coverage."
  "§9 public repo reopening checklist"
  "§10 paid hosted launch checklist"
  "make public-content-audit"
  "make launch-external-status"
  "branch-protection"
  "repository-description"
  "public-history"
  "local-artifact"
  "tenant-offboarding"
  "§10.1"
  "§10.2"
  "§10.3"
  "§10.4"
  "§10.5"
  "external-attestation"
)

for pattern in "${required_final_plan_checklist_patterns[@]}"; do
  if ! grep -qF "$pattern" "$normalized"; then
    err "final integrated plan checklist coverage missing required text: $pattern"
  fi
done

required_audit_patterns=(
  "## Objective-to-artifact completion audit"
  "| Objective requirement | Evidence in this tree | Current status |"
  "## Prompt-to-artifact checklist"
  "| Prompt requirement / gate | Artifact or verifier | Concrete evidence inspected | Current status |"
  "Use the review folder as the source of truth."
  "Prioritize the highest-impact blockers first."
  "Fix safe actionable code findings."
  "Fix safe documentation, CI, and public-surface drift."
  "Leave human/legal/product approval items clearly documented."
  "Keep tests and release posture verifiable."
  "Decide whether the objective is actually complete."
  "**Not complete. Do not mark launch-ready.**"
  "Group 1 scheduled final-SHA evidence"
  "Group 6 candidate-tag security evidence"
  "Group 7 release/sigstore/SLSA evidence"
  "pushed workflow evidence"
  "repository-state cleanup"
  "hosted quota evidence"
  "legal/product approval"
  'clockify://` URI plus gRPC service-name branding review'
)

for pattern in "${required_audit_patterns[@]}"; do
  if ! grep -qF "$pattern" "$normalized"; then
    err "objective completion audit missing required text: $pattern"
  fi
done

required_prompt_artifact_patterns=(
  'Use `~/Downloads/review may 8/` as source of truth.'
  "00_COORDINATOR_INDEX.md"
  "01_MCP_PROTOCOL_TRANSPORTS.md"
  "02_SECURITY_AUTH_TENANT_ISOLATION.md"
  "10_FINAL_INTEGRATED_LAUNCH_PLAN.md"
  "Preserve every reviewed finding disposition."
  "scripts/test-check-launch-review-ledger.sh"
  "Prioritize high-impact blockers before polish."
  "Fix safe code findings."
  "GOTOOLCHAIN=go1.25.10 make release-check"
  "Fix safe docs, CI, release, and public-surface drift."
  "bash scripts/test-check-doc-parity.sh"
  "current doc-parity regression suite has 70 cases"
  "README/CONTRIBUTING local-verification wording"
  "Makefile release-check wording"
  "stale shippable release-check wording in docs"
  "shared-service profile Group 2 scoping"
  "production-readiness blocker-scope wording"
  "gap-analysis blocker-scope wording"
  "P3-5 baseline header docs"
  "serverInfo identity guidance"
  "default protocol-version guidance"
  "May 8 ledger read-first routing"
  "brand/legal URI plus gRPC service-name review docs"
  "build-tag/tool-module Dependabot watcher coverage"
  "pinned verify-vuln tool-module execution"
  "govulncheck CI version proof"
  "SUPPORT.md SLSA private-repo cosign fallback"
  "stale unconditional SLSA public wording"
  "release-smoke SLSA bare-404 skip guard"
  "README SLSA provenance availability wording"
  "workflow action SHA-pin guard"
  "deploy SLSA bare-404 skip guard"
  "release workflow/docs SLSA availability wording"
  "release-smoke doctor-output artifact guard"
  "docker-image SLSA feature-gate notice guard"
  "gh release view <tag>"
  "scripts/check-release-assets.sh"
  "46-asset validation"
  "legacy HTTP EOL runbook"
  "stale shared-service launch-blocking wording"
  "agent handoff permissioned landing sequence"
  "dependency-review default-branch evidence trigger"
  "Keep Group 6 security posture verifiable."
  "govulncheck@v1.3.0"
  'Semgrep `p/default`'
  "make secret-scan"
  "clean candidate-tag gitleaks remains required"
  "Keep CI/release/external state honest."
  "make launch-external-status"
  "6 open, 0 unknown"
  "CLOCKIFY_LIVE_AUDIT_REQUIRED=true"
  "live-contract cron log markers"
  "TestLiveCreateUpdateDeleteEntryAuditPhases"
  "TestLiveReadSideSchemaDiff"
  "Keep public-readiness story honest."
  "make public-content-audit"
  "0 open, 0 unknown"
  "docs/release/local-artifact-review.md"
  "Public-content audit clean locally; public flip still requires external, repo-state, and legal/product gates."
  "Leave human/legal/product approvals documented, not guessed."
  "docs/release/brand-legal-review.md"
  "make license-evidence"
  "not legal advice or license clearance"
  "gRPC service-name branding review"
  "Decide completion from real evidence only."
  "git status --short --branch"
  "**Not complete. Do not mark launch-ready.**"
)

for pattern in "${required_prompt_artifact_patterns[@]}"; do
  if ! grep -qF "$pattern" "$normalized"; then
    err "prompt-to-artifact checklist missing required text: $pattern"
  fi
done

required_external_gate_patterns=(
  "Group 1 scheduled live-contract evidence."
  "Main freeze while Group 1 is pending."
  "Group 6 candidate-tag security walk-through."
  "Group 7 release/sigstore/SLSA evidence."
  "Launch-candidate tracking issue."
  "D3 SLSA/private-repo stance."
  "D1 / D2 GitHub repository description drift."
  "128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases."
  "P1-3 SLSA fail-closed posture."
  "NPM publish path on the next release."
  "D7 / D9 branch protection, mutation cron, and stale local branches."
  "D9 launch required checks"
  "Doctor strict smoke"
  "Doctor Postgres backend"
  "Shared-service Postgres E2E"
  "required_status_checks.checks[].context"
  "Public-repo stale PR hygiene."
  "D8 issue #28 stale."
  "P2-5 dependency-review first-run evidence."
  "P1-4 CodeQL first-run evidence."
  "Semgrep first-run evidence."
  "P1-8 paid-commercial RLS decision."
  "Cross-replica hosted HTTP quotas."
  "Paid-hosted external security review."
  "DPA / terms / privacy posture."
  'Trademark / "official Clockify" language.'
  "Clockify URI scheme and gRPC service-name branding review."
)

for pattern in "${required_external_gate_patterns[@]}"; do
  if ! grep -qF "$pattern" "$normalized"; then
    err "external gate disposition missing required text: $pattern"
  fi
done

required_public_content_patterns=(
  "Public repo content audit before visibility flip."
  "make public-content-audit"
  "Candidate branch file content: 0 open, 0 unknown"
  "Public history review: 0 open, 0 unknown"
  "Local artifact/full-tree review: 0 open, 0 unknown"
  "candidate branch-content gitleaks scan"
  "candidate branch-content TLS verification bypass marker check"
  "candidate branch-content MIT LICENSE check"
  "candidate branch-content .gitignore coverage check"
  "candidate branch-content .gitleaks.toml allowlist description check"
  "candidate branch-content CLAUDE.md workstation context check"
  "candidate branch-content live Clockify secret assignment check"
  "candidate branch-content env-like file check"
  "docs/release/public-history-review.md"
  "docs/release/local-artifact-review.md"
)

for pattern in "${required_public_content_patterns[@]}"; do
  if ! grep -qF "$pattern" "$normalized"; then
    err "public-content audit disposition missing required text: $pattern"
  fi
done

if [ "$fail" != "0" ]; then
  echo "launch-review-ledger: FAIL" >&2
  exit 1
fi

echo "launch-review-ledger: OK"
