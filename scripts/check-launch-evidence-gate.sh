#!/usr/bin/env bash
#
# check-launch-evidence-gate.sh
#
# Guards the launch-candidate-checklist against premature box-ticking.
# Groups 1, 6, and 7 require external evidence (scheduled cron runs,
# candidate tags, workflow run URLs) that cannot be produced from a
# local checkout. This script fails when it finds a checked box in
# those groups that lacks a corresponding evidence reference.
#
# If you are checking a box because real evidence arrived:
#   1. Add the evidence URL (GitHub Actions run, commit SHA, etc.)
#      in the _Tracking_ annotation below the box.
#   2. Update this script to recognise the new evidence pattern.
#
# Wired into make launch-checklist-parity and make doc-parity.

set -euo pipefail

CHECKLIST="${LAUNCH_CHECKLIST:-docs/launch-candidate-checklist.md}"
fail=0

err() {
  echo "[fail] launch-evidence-gate: $*" >&2
  fail=1
}

if [ ! -f "$CHECKLIST" ]; then
  err "missing checklist: $CHECKLIST"
  echo "launch-evidence-gate: FAIL" >&2
  exit 1
fi

# Group 1 boxes that require scheduled-cron evidence — must be unchecked
# or carry an evidence URL if checked.
#
# We grep for the box text pattern exactly as it appears on the - [ ] line.
# Each pattern must match the UNCHECKED version. If someone changes - [ ] to
# - [x] without adding an evidence URL, the grep fails and we flag it.
#
# A checked box with evidence must have one of these within 3 lines:
#   https://github.com/apet97/go-clockify/actions/runs/
#   workflow_run_id:
#   _Closed YYYY-MM-DD by

check_unchecked_with_evidence() {
  local box_pattern="$1"
  local label="$2"

  if grep -qE "^- \[x\] ${box_pattern}" "$CHECKLIST"; then
    # Box is checked. Look for an evidence reference near it.
    local ctx
    ctx="$(grep -A3 "^- \[x\] ${box_pattern}" "$CHECKLIST" || true)"
    if echo "$ctx" | grep -qE 'https://github\.com/apet97/go-clockify/actions/runs/|workflow_run_id:|_Closed 2026-' ; then
      return 0
    fi
    err "${label}: box is checked but no evidence URL or _Closed_ annotation found within 3 lines. Evidence required: scheduled cron run URL, workflow_run_id, or _Closed_ date with commit reference."
  elif ! grep -qE "^- \[ \] ${box_pattern}" "$CHECKLIST" && ! grep -qE "^- \[x\] ${box_pattern}" "$CHECKLIST"; then
    # Box text changed — the pattern doesn't match at all. This is a soft
    # warning (text may have been reworded legitimately) but we flag it
    # so the script gets updated.
    echo "[warn] launch-evidence-gate: could not find box pattern for '${label}' — text may have been reworded; verify manually" >&2
  fi
}

# ── Group 1: Live API contract ──────────────────────────────────────
# These boxes require scheduled-cron evidence on the candidate SHA.
# Currently all unchecked as of 2026-05-02.

check_unchecked_with_evidence \
  "Latest scheduled run of .live-contract.yml. is green with" \
  "Group 1: scheduled live-contract run green"

check_unchecked_with_evidence \
  ".TestLiveDryRunDoesNotMutate. and" \
  "Group 1: TestLiveDryRunDoesNotMutate + TestLivePolicyTimeTrackingSafeBlocksProjectCreate passing"

check_unchecked_with_evidence \
  "Two consecutive nightly runs green with no flakes" \
  "Group 1: two consecutive nightly greens"

check_unchecked_with_evidence \
  "Read-side schema diff: response shapes returned by the" \
  "Group 1: read-side schema diff"

# ── Group 6: Security and policy review ─────────────────────────────
# Candidate-tag scan boxes stay unchecked until the final tag exists.
# Local preflight evidence is useful but does not close these boxes.

check_unchecked_with_evidence \
  ".make verify-vuln. green for the candidate tag" \
  "Group 6: verify-vuln green on candidate tag"

check_unchecked_with_evidence \
  ".gitleaks. scan green" \
  "Group 6: gitleaks green on candidate tag"

check_unchecked_with_evidence \
  ".semgrep. review green" \
  "Group 6: semgrep green on candidate tag"

check_unchecked_with_evidence \
  ".make verify-fips. green when the FIPS-aware tooling is" \
  "Group 6: verify-fips green on candidate tag"

# ── Group 7: CI / release readiness ─────────────────────────────────
# These boxes require a candidate tag, release-smoke.yml green,
# and signed artefacts — all impossible from a local checkout.

check_unchecked_with_evidence \
  ".make release-check. green from a clean checkout on at" \
  "Group 7: make release-check green on Linux x64 + macOS arm64"

check_unchecked_with_evidence \
  "All required workflows on .main. green: .ci.yml." \
  "Group 7: all required workflows on main green"

check_unchecked_with_evidence \
  ".make verify-bench. and .make bench-baseline-check. green" \
  "Group 7: verify-bench and bench-baseline-check green"

check_unchecked_with_evidence \
  "Release artefacts: signed binaries .cosign . SLSA., SBOMs," \
  "Group 7: release artefacts verified"

check_unchecked_with_evidence \
  ".clockify-mcp doctor --strict. and" \
  "Group 7: doctor --strict exits 0 on reference deployment"

if [ "$fail" -ne 0 ]; then
  echo "" >&2
  echo "  These boxes require external evidence (scheduled cron runs," >&2
  echo "  candidate tags, workflow runs) that cannot be produced from a" >&2
  echo "  local checkout. Do not tick them without referencing the" >&2
  echo "  specific evidence URL or workflow run ID." >&2
  echo "" >&2
  echo "launch-evidence-gate: FAIL" >&2
  exit 1
fi

echo "launch-evidence-gate: OK"
