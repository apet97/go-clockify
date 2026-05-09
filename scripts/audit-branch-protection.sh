#!/usr/bin/env bash
# Dump the live branch-protection state of main so docs/branch-protection.md
# can be reconciled against reality. Not CI-gated — run locally (or via
# `gh workflow run`) whenever you suspect the snapshot has drifted.
#
# The script projects only the fields the snapshot table covers; the full
# API response has more detail if you need it (drop the jq filter).
set -euo pipefail

REPO="${GITHUB_REPOSITORY:-apet97/go-clockify}"

command -v gh >/dev/null 2>&1 || {
  echo "ERROR: gh is required to audit branch protection" >&2
  exit 2
}

command -v jq >/dev/null 2>&1 || {
  echo "ERROR: jq is required to project branch protection fields" >&2
  exit 2
}

protection_json=""
if ! protection_json="$(gh api "repos/${REPO}/branches/main/protection" 2>&1)"; then
  protection_error="$(printf '%s\n' "$protection_json" | sed 's/}gh:/}\
gh:/')"
  printf 'branch-protection audit: unable to read main branch protection for %s\n' "$REPO" >&2
  printf '%s\n' "$protection_error" >&2
  if grep -qiE 'Upgrade to GitHub Pro|make this repository public' <<< "$protection_error"; then
    printf '%s\n' \
      "branch-protection audit: GitHub requires GitHub Pro or a public repository for this private-repo API response; reconcile D9 from the GitHub UI or a repository visibility decision." >&2
  fi
  exit 1
fi

printf '%s\n' "$protection_json" | jq '{
  required_pull_request_reviews: .required_pull_request_reviews,
  required_status_checks: (((.required_status_checks.contexts // []) + ((.required_status_checks.checks // []) | map(.context // empty))) | unique),
  required_signatures: .required_signatures.enabled,
  required_linear_history: .required_linear_history.enabled,
  enforce_admins: .enforce_admins.enabled,
  allow_force_pushes: .allow_force_pushes.enabled,
  allow_deletions: .allow_deletions.enabled,
  required_conversation_resolution: .required_conversation_resolution.enabled,
  restrictions: (.restrictions // null)
}'
