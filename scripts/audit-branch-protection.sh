#!/usr/bin/env bash
# Dump the live branch-protection state of main so docs/branch-protection.md
# can be reconciled against reality. Not CI-gated — run locally (or via
# `gh workflow run`) whenever you suspect the snapshot has drifted.
#
# The script projects only the fields the snapshot table covers; the full
# API response has more detail if you need it (drop the jq filter).
set -euo pipefail

REPO="${GITHUB_REPOSITORY:-apet97/go-clockify}"

gh api "repos/${REPO}/branches/main/protection" | jq '{
  required_pull_request_reviews: .required_pull_request_reviews,
  required_status_checks: .required_status_checks.contexts,
  required_signatures: .required_signatures.enabled,
  required_linear_history: .required_linear_history.enabled,
  enforce_admins: .enforce_admins.enabled,
  allow_force_pushes: .allow_force_pushes.enabled,
  allow_deletions: .allow_deletions.enabled,
  required_conversation_resolution: .required_conversation_resolution.enabled,
  restrictions: (.restrictions // null)
}'
