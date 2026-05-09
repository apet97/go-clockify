#!/usr/bin/env bash
#
# test-check-launch-review-ledger.sh - regression test for
# check-launch-review-ledger.sh.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-launch-review-ledger.sh"

if [ ! -f "$script" ]; then
  echo "FAIL: script not found at $script" >&2
  exit 1
fi

tests_run=0
tests_failed=0

append_all_ids() {
  local path="$1"

  {
    i=1
    while [ "$i" -le 24 ]; do
      printf -- '- T-%02d disposition.\n' "$i"
      i=$((i + 1))
    done
    i=1
    while [ "$i" -le 13 ]; do
      printf -- '- MP-%02d disposition.\n' "$i"
      i=$((i + 1))
    done
    tier=1
    while [ "$tier" -le 2 ]; do
      i=1
      while [ "$i" -le 10 ]; do
        printf -- '- P%s-%d disposition.\n' "$tier" "$i"
        i=$((i + 1))
      done
      tier=$((tier + 1))
    done
    i=1
    while [ "$i" -le 7 ]; do
      printf -- '- P3-%d disposition.\n' "$i"
      i=$((i + 1))
    done
    i=1
    while [ "$i" -le 10 ]; do
	    printf -- '- D%d disposition.\n' "$i"
	    i=$((i + 1))
	  done
	  i=1
	  while [ "$i" -le 5 ]; do
	    printf -- '- G-%02d disposition.\n' "$i"
	    i=$((i + 1))
	  done
	  i=1
	  while [ "$i" -le 5 ]; do
	    printf -- '- L-%02d disposition.\n' "$i"
	    i=$((i + 1))
	  done
	  printf -- '- L-08 disposition.\n'
	  printf -- '- L-10 disposition.\n'
	} >> "$path"
}

write_source_fixture() {
  local dir="$1"
  local source_dir="$dir/review-source"

  mkdir -p "$source_dir"
  for source in \
    "00_COORDINATOR_INDEX.md" \
    "01_MCP_PROTOCOL_TRANSPORTS.md" \
    "02_SECURITY_AUTH_TENANT_ISOLATION.md" \
    "10_FINAL_INTEGRATED_LAUNCH_PLAN.md"; do
    printf '# %s\n\n' "$source" > "$source_dir/$source"
  done
  append_all_ids "$source_dir/10_FINAL_INTEGRATED_LAUNCH_PLAN.md"
}

write_fixture() {
  local dir="$1"
  local path="$dir/docs/launch-readiness-review-may-8.md"

  mkdir -p "$dir/docs"
  cat > "$path" <<'EOF'
# May 8 Launch-Readiness Review Disposition

Source inventory:
- 00_COORDINATOR_INDEX.md
- 01_MCP_PROTOCOL_TRANSPORTS.md
- 02_SECURITY_AUTH_TENANT_ISOLATION.md
- 10_FINAL_INTEGRATED_LAUNCH_PLAN.md

## Closed in this remediation pass
EOF
  append_all_ids "$path"
  cat >> "$path" <<'EOF'

- **B.03 live-contract coverage inventory.** Static live coverage guard exists.
- **B.04 tool contract matrix.** Tool registry contract matrix exists.
- **B.05 performance and large-workspace evidence.** Bench evidence exists.
- **B.06 CI/release evidence pipeline.** Workflow evidence pipeline is documented.
- **B.07 operator-facing marker audit.** Operator marker audit is closed.
- **B.07 per-package coverage outlier.** Coverage outlier is documented.
- **B.07 proposed-ADR audit.** Proposed ADR status is audited.
- **B.07 `internal/jsonschema` architecture trade-off.** ADR rationale exists.
- **B.08 dependency/license evidence path.** License evidence path exists.
- **Coordinator Appendix B file-scope audit.** The coordinator's "files this coordinator did NOT deeply read" list is covered by transport/session code, auth/policy and enforcement, Clockify pagination, GoReleaser/release assets, deploy manifests, API coverage, and CODEOWNERS dispositions.
- **Final integrated plan checklist coverage.** The §9 public repo reopening checklist and §10 paid hosted launch checklist are covered by make public-content-audit, make launch-external-status, branch-protection, repository-description, public-history, local-artifact, tenant-offboarding, §10.1, §10.2, §10.3, §10.4, §10.5, and external-attestation gates.

## Review ID coverage audit

- Summary prose is not a disposition.

## Objective-to-artifact completion audit

| Objective requirement | Evidence in this tree | Current status |
| --- | --- | --- |
| Use the review folder as the source of truth. | Source inventory exists. | Locally satisfied. |
| Prioritize the highest-impact blockers first. | Highest-impact findings are dispositioned first. | Locally satisfied. |
| Fix safe actionable code findings. | Safe code findings are dispositioned. | Locally satisfied. |
| Fix safe documentation, CI, and public-surface drift. | Safe docs and CI drift are dispositioned. | Locally satisfied. |
| Leave human/legal/product approval items clearly documented. | External gates below name legal/product approval and the `clockify://` URI plus gRPC service-name branding review. | Open by design. |
| Keep tests and release posture verifiable. | Verification log exists. | Locally green. |
| Decide whether the objective is actually complete. | Completion requires no missing objective requirement. Group 1 scheduled final-SHA evidence, Group 6 candidate-tag security evidence, Group 7 release/sigstore/SLSA evidence, pushed workflow evidence, repository-state cleanup, hosted quota evidence, and legal/product approval are still missing. | **Not complete. Do not mark launch-ready.** |

## Prompt-to-artifact checklist

| Prompt requirement / gate | Artifact or verifier | Concrete evidence inspected | Current status |
| --- | --- | --- | --- |
| Use `~/Downloads/review may 8/` as source of truth. | `00_COORDINATOR_INDEX.md`, `01_MCP_PROTOCOL_TRANSPORTS.md`, `02_SECURITY_AUTH_TENANT_ISOLATION.md`, `10_FINAL_INTEGRATED_LAUNCH_PLAN.md`; `scripts/check-launch-review-ledger.sh`. | Source IDs are compared against the actual source bundle. | Locally covered and guarded. |
| Preserve every reviewed finding disposition. | `docs/launch-readiness-review-may-8.md`; `scripts/test-check-launch-review-ledger.sh`. | All expected IDs are dispositioned outside summary sections. | Locally covered and guarded. |
| Prioritize high-impact blockers before polish. | Closed finding order. | High-impact blockers appear before polish. | Locally covered by disposition order. |
| Fix safe code findings. | `GOTOOLCHAIN=go1.25.10 make release-check`. | Release-check passed. | Locally green. |
| Fix safe docs, CI, release, and public-surface drift. | `make doc-parity`; `bash scripts/test-check-doc-parity.sh`. | The current doc-parity regression suite has 69 cases covering README/CONTRIBUTING local-verification wording, Makefile release-check wording, stale shippable release-check wording in docs, shared-service profile Group 2 scoping, production-readiness blocker-scope wording, gap-analysis blocker-scope wording, P3-5 baseline header docs, serverInfo identity guidance, default protocol-version guidance, May 8 ledger read-first routing, brand/legal URI plus gRPC service-name review docs, T-17 gRPC reflection dev-only posture, build-tag/tool-module Dependabot watcher coverage, pinned verify-vuln tool-module execution, govulncheck CI version proof, SUPPORT.md SLSA private-repo cosign fallback, stale unconditional SLSA public wording, release-smoke SLSA bare-404 skip guard, README SLSA provenance availability wording, workflow action SHA-pin guard, deploy SLSA bare-404 skip guard, release workflow/docs SLSA availability wording, release-smoke doctor-output artifact guard, docker-image SLSA feature-gate notice guard, gh release view <tag> plus scripts/check-release-assets.sh 46-asset validation, legacy HTTP EOL runbook, stale public-content local-artifact wording, stale shared-service launch-blocking wording, agent handoff permissioned landing sequence, and dependency-review default-branch evidence trigger. | Locally green; workflow first-run evidence still external. |
| Keep Group 6 security posture verifiable. | `docs/launch-candidate-checklist.md`; `docs/runbooks/release-candidate-evidence.md`; `scripts/prepare-rc-evidence.sh`. | `govulncheck@v1.3.0`, Semgrep `p/default`, and FIPS passed locally; `make secret-scan` needs a clean candidate tag because clean candidate-tag gitleaks remains required. | Locally documented; final candidate-tag evidence open. |
| Keep CI/release/external state honest. | `make launch-external-status`. | Latest snapshot reports `11 open, 0 unknown`, and the helper directly verifies `CLOCKIFY_LIVE_AUDIT_REQUIRED=true`, then verifies live-contract cron log markers including `TestLiveCreateUpdateDeleteEntryAuditPhases` and `TestLiveReadSideSchemaDiff` before Group 1 can close. | Open external/repo-state gates. |
| Keep public-readiness story honest. | `make public-content-audit`; `docs/release/public-history-review.md`; `docs/release/local-artifact-review.md`. | Latest snapshot reports `0 open, 0 unknown`; candidate, history, and local artifact buckets are clean. | Public-content audit clean locally; public flip still requires external, repo-state, and legal/product gates. |
| Leave human/legal/product approvals documented, not guessed. | `docs/release/brand-legal-review.md`; `make license-evidence`. | Dependency evidence is not legal advice or license clearance; gRPC service-name branding review stays external. | Evidence input exists; legal/product approval open. |
| Decide completion from real evidence only. | `git status --short --branch`; external/public status helpers. | The tree is dirty and external gates remain open. | **Not complete. Do not mark launch-ready.** |

## External evidence or approval gates

- **Group 1 scheduled live-contract evidence.** Still open.
- **Main freeze while Group 1 is pending.** Requires operator coordination.
- **Group 6 candidate-tag security walk-through.** Still open.
- **Group 7 release/sigstore/SLSA evidence.** Still open.
- **Launch-candidate tracking issue.** Requires approval-gated issue creation.
- **D3 SLSA/private-repo stance.** Requires maintainer decision.
- **D1 / D2 GitHub repository description drift.** Requires maintainer action to set `128 tools, three transports (stdio / streamable HTTP / optional gRPC), five policy modes, cosign-signed releases.`.
- **P1-3 SLSA fail-closed posture.** Requires rc/private-repo evidence.
- **NPM publish path on the next release.** Requires next release evidence.
- **D7 / D9 branch protection, mutation cron, and stale local branches.** Requires maintainer/GitHub state. If branch protection is readable, D9 launch required checks must include `Doctor strict smoke`, `Doctor Postgres backend`, and `Shared-service Postgres E2E`, including checks returned through `required_status_checks.checks[].context`.
- **Public-repo stale PR hygiene.** Read-only checked.
- **D8 issue #28 stale.** Requires maintainer close/update.
- **P2-5 dependency-review first-run evidence.** Requires pushed workflow evidence.
- **P1-4 CodeQL first-run evidence.** Requires pushed workflow evidence.
- **Semgrep first-run evidence.** Requires pushed workflow evidence.
- **P1-8 paid-commercial RLS decision.** Requires product/platform decision.
- **Cross-replica hosted HTTP quotas.** Requires hosted gateway evidence.
- **Paid-hosted external security review.** Requires third-party or peer review.
- **DPA / terms / privacy posture.** Requires counsel review.
- **Trademark / "official Clockify" language.** Requires legal/product approval.
- **Clockify URI scheme and gRPC service-name branding review.** Requires legal/product approval.
- **Public repo content audit before visibility flip.** `make public-content-audit`
  reports `0 open, 0 unknown`, with `Candidate branch file content: 0 open, 0 unknown`,
  `Public history review: 0 open, 0 unknown`, and
  `Local artifact/full-tree review: 0 open, 0 unknown`. The candidate branch-content gitleaks scan is closed. The candidate branch-content env-like file check is closed.
  The candidate branch-content TLS verification bypass marker check is closed.
  The candidate branch-content MIT LICENSE check is closed.
  The candidate branch-content .gitignore coverage check is closed.
  The candidate branch-content .gitleaks.toml allowlist description check is closed.
  The candidate branch-content CLAUDE.md workstation context check is closed.
  The candidate branch-content live Clockify secret assignment check is closed.
  The recent commit message sensitive-word matches are documented in `docs/release/public-history-review.md`.
  The ignored full-tree findings are documented in `docs/release/local-artifact-review.md`.
  Public visibility still depends on the external, repository-state,
  and legal/product gates above.

## Deferred low-risk follow-ups

- Deferred section stays part of the checked disposition body.

## Verification used for this pass

- Command history is not a disposition.
EOF
}

run_case() {
  local name="$1"; shift
  local expect_exit="$1"; shift
  local expect_pattern="$1"; shift

  tests_run=$((tests_run + 1))

  local dir
  dir="$(mktemp -d "${TMPDIR:-/tmp}/test-launch-review-ledger.XXXXXX")"
  trap 'rm -rf "$dir"' RETURN

  write_fixture "$dir"
  write_source_fixture "$dir"

  if [ -n "${MUTATOR:-}" ]; then
    "$MUTATOR" "$dir/docs/launch-readiness-review-may-8.md"
  fi
  if [ -n "${SOURCE_MUTATOR:-}" ]; then
    "$SOURCE_MUTATOR" "$dir/review-source"
  fi

  local out
  local actual_exit=0
  out="$(cd "$dir" && REVIEW_SOURCE_DIR="$dir/review-source" bash "$script" "docs/launch-readiness-review-may-8.md" 2>&1)" || actual_exit=$?

  local pass=1
  if [ "$actual_exit" != "$expect_exit" ]; then
    pass=0
  fi
  if [ -n "$expect_pattern" ] && ! grep -qE -- "$expect_pattern" <<< "$out"; then
    pass=0
  fi

  if [ "$pass" = "1" ]; then
    printf 'PASS: %s\n' "$name"
  else
    tests_failed=$((tests_failed + 1))
    printf 'FAIL: %s\n' "$name" >&2
    printf '  expected exit=%s, got=%s\n' "$expect_exit" "$actual_exit" >&2
    printf '  expected pattern=%q\n' "$expect_pattern" >&2
    printf '  --- output ---\n%s\n  --- end ---\n' "$out" >&2
  fi

  rm -rf "$dir"
  trap - RETURN
  MUTATOR=""
  SOURCE_MUTATOR=""
}

MUTATOR=""
SOURCE_MUTATOR=""
run_case "complete fixture clears the gate" 0 'launch-review-ledger: OK'

mut_missing_id() {
  perl -0pi -e 's/- T-14 disposition\.\n//' "$1"
}
MUTATOR=mut_missing_id
run_case "missing disposition ID fails closed" 1 'missing a disposition.*T-14'

mut_summary_only_id() {
  perl -0pi -e 's/- P2-9 disposition\.\n//' "$1"
  perl -0pi -e 's/Summary prose is not a disposition\./Summary prose mentions P2-9 only./' "$1"
}
MUTATOR=mut_summary_only_id
run_case "coverage-audit-only ID does not satisfy disposition" 1 'missing a disposition.*P2-9'

mut_unexpected_id() {
  perl -0pi -e 's/- D10 disposition\./- D10 disposition.\n- D11 typo disposition./' "$1"
}
MUTATOR=mut_unexpected_id
run_case "unexpected ID fails closed" 1 'unexpected review finding ID.*D11'

mut_unicode_hyphen() {
  local hyphen
  hyphen="$(printf '\342\200\221')"
  perl -0pi -e "s/T-01/T${hyphen}01/" "$1"
}
MUTATOR=mut_unicode_hyphen
run_case "unicode hyphen IDs are normalized" 0 'launch-review-ledger: OK'

mut_missing_mp_id() {
  perl -0pi -e 's/- MP-11 disposition\.\n//' "$1"
}
MUTATOR=mut_missing_mp_id
run_case "missing MP matrix disposition fails closed" 1 'missing a disposition.*MP-11'

mut_missing_source_inventory() {
  perl -0pi -e 's/- 01_MCP_PROTOCOL_TRANSPORTS\.md\n//; s/`01_MCP_PROTOCOL_TRANSPORTS\.md`, //g' "$1"
}
MUTATOR=mut_missing_source_inventory
run_case "missing source inventory file fails closed" 1 'source inventory missing review file'

mut_missing_final_plan_inventory() {
  perl -0pi -e 's/- 10_FINAL_INTEGRATED_LAUNCH_PLAN\.md\n//; s/, `10_FINAL_INTEGRATED_LAUNCH_PLAN\.md`//g' "$1"
}
MUTATOR=mut_missing_final_plan_inventory
run_case "missing final integrated plan inventory fails closed" 1 'source inventory missing review file.*10_FINAL'

mut_missing_governance_id() {
  perl -0pi -e 's/- G-03 disposition\.\n//' "$1"
}
MUTATOR=mut_missing_governance_id
run_case "missing G finding disposition fails closed" 1 'missing a disposition.*G-03'

mut_missing_launch_id() {
  perl -0pi -e 's/- L-10 disposition\.\n//' "$1"
}
MUTATOR=mut_missing_launch_id
run_case "missing L launch disposition fails closed" 1 'missing a disposition.*L-10'

mut_missing_launch_alias_id() {
  perl -0pi -e 's/- L-08 disposition\.\n//' "$1"
}
MUTATOR=mut_missing_launch_alias_id
run_case "missing L-08 launch caveat alias fails closed" 1 'missing a disposition.*L-08'

mut_source_has_unpinned_id() {
  printf -- '- L-99 surprise source ID.\n' >> "$1/10_FINAL_INTEGRATED_LAUNCH_PLAN.md"
}
SOURCE_MUTATOR=mut_source_has_unpinned_id
run_case "source bundle ID not in pinned inventory fails closed" \
  1 'review source ID is not pinned.*L-99'

mut_source_missing_pinned_id() {
  perl -0pi -e 's/- T-24 disposition\.\n//' "$1/10_FINAL_INTEGRATED_LAUNCH_PLAN.md"
}
SOURCE_MUTATOR=mut_source_missing_pinned_id
run_case "pinned inventory absent from source bundle fails closed" \
  1 'pinned review ID not found in source bundle.*T-24'

mut_missing_completion_audit() {
  perl -0pi -e 's/\n## Objective-to-artifact completion audit\n.*?\n## Deferred low-risk follow-ups\n/\n## Deferred low-risk follow-ups\n/s' "$1"
}
MUTATOR=mut_missing_completion_audit
run_case "missing objective completion audit fails closed" 1 'objective completion audit missing required text'

mut_complete_claim() {
  perl -0pi -e 's/\*\*Not complete\. Do not mark launch-ready\.\*\*/Complete./g' "$1"
}
MUTATOR=mut_complete_claim
run_case "completion claim fails while launch evidence remains open" 1 'Not complete\. Do not mark launch-ready'

mut_missing_external_gate() {
  perl -0pi -e 's/Group 6 candidate-tag security evidence, //' "$1"
}
MUTATOR=mut_missing_external_gate
run_case "missing Group 6 completion-audit blocker fails closed" 1 'Group 6 candidate-tag security evidence'

mut_missing_prompt_artifact_checklist() {
  perl -0pi -e 's/\n## Prompt-to-artifact checklist\n.*?\n## External evidence or approval gates\n/\n## External evidence or approval gates\n/s' "$1"
}
MUTATOR=mut_missing_prompt_artifact_checklist
run_case "missing prompt-to-artifact checklist fails closed" \
  1 'prompt-to-artifact checklist missing required text'

mut_missing_prompt_public_content_gate() {
  perl -0pi -e 's/\| Keep public-readiness story honest\.[^\n]*\n//' "$1"
}
MUTATOR=mut_missing_prompt_public_content_gate
run_case "missing prompt public-content artifact mapping fails closed" \
  1 'prompt-to-artifact checklist missing required text.*Keep public-readiness story honest'

mut_stale_doc_parity_case_count() {
  perl -0pi -e 's/current doc-parity regression suite has 69 cases/current doc-parity regression suite has 65 cases/' "$1"
}
MUTATOR=mut_stale_doc_parity_case_count
run_case "stale doc-parity case count fails closed" \
  1 'prompt-to-artifact checklist missing required text.*69 cases'

mut_missing_appendix_b_disposition() {
  perl -0pi -e 's/- \*\*B\.05 performance and large-workspace evidence\.\*\* Bench evidence exists\.\n//' "$1"
}
MUTATOR=mut_missing_appendix_b_disposition
run_case "missing Appendix B disposition fails closed" \
  1 'Appendix B open-question disposition missing required text.*B\.05'

mut_missing_coordinator_appendix_scope() {
  perl -0pi -e 's/- \*\*Coordinator Appendix B file-scope audit\.\*\* The coordinator'\''s "files this coordinator did NOT deeply read" list is covered by transport\/session code, auth\/policy and enforcement, Clockify pagination, GoReleaser\/release assets, deploy manifests, API coverage, and CODEOWNERS dispositions\.\n//' "$1"
}
MUTATOR=mut_missing_coordinator_appendix_scope
run_case "missing coordinator Appendix B file-scope disposition fails closed" \
  1 'coordinator Appendix B file-scope disposition missing required text'

mut_missing_final_plan_checklist_coverage() {
  perl -0pi -e 's/- \*\*Final integrated plan checklist coverage\.\*\* The §9 public repo reopening checklist and §10 paid hosted launch checklist are covered by make public-content-audit, make launch-external-status, branch-protection, repository-description, public-history, local-artifact, tenant-offboarding, §10\.1, §10\.2, §10\.3, §10\.4, §10\.5, and external-attestation gates\.\n//' "$1"
}
MUTATOR=mut_missing_final_plan_checklist_coverage
run_case "missing final plan checklist coverage fails closed" \
  1 'final integrated plan checklist coverage missing required text'

mut_missing_issue28_external_gate() {
  perl -0pi -e 's/- \*\*D8 issue #28 stale\.\*\* Requires maintainer close\/update\.\n//' "$1"
}
MUTATOR=mut_missing_issue28_external_gate
run_case "missing issue #28 external gate fails closed" \
  1 'external gate disposition missing required text.*D8 issue #28 stale'

mut_missing_tracking_issue_external_gate() {
  perl -0pi -e 's/- \*\*Launch-candidate tracking issue\.\*\* Requires approval-gated issue creation\.\n//' "$1"
}
MUTATOR=mut_missing_tracking_issue_external_gate
run_case "missing launch-candidate tracking issue gate fails closed" \
  1 'external gate disposition missing required text.*Launch-candidate tracking issue'

mut_missing_main_freeze_external_gate() {
  perl -0pi -e 's/- \*\*Main freeze while Group 1 is pending\.\*\* Requires operator coordination\.\n//' "$1"
}
MUTATOR=mut_missing_main_freeze_external_gate
run_case "missing main-freeze external gate fails closed" \
  1 'external gate disposition missing required text.*Main freeze while Group 1 is pending'

mut_missing_repo_description_text() {
  perl -0pi -e 's/ to set `128 tools, three transports \(stdio \/ streamable HTTP \/ optional gRPC\), five policy modes, cosign-signed releases\.`//' "$1"
}
MUTATOR=mut_missing_repo_description_text
run_case "missing exact repo description action fails closed" \
  1 'external gate disposition missing required text.*cosign-signed releases'

mut_missing_external_security_review_gate() {
  perl -0pi -e 's/- \*\*Paid-hosted external security review\.\*\* Requires third-party or peer review\.\n//' "$1"
}
MUTATOR=mut_missing_external_security_review_gate
run_case "missing external security review gate fails closed" \
  1 'external gate disposition missing required text.*Paid-hosted external security review'

mut_missing_privacy_counsel_gate() {
  perl -0pi -e 's/- \*\*DPA \/ terms \/ privacy posture\.\*\* Requires counsel review\.\n//' "$1"
}
MUTATOR=mut_missing_privacy_counsel_gate
run_case "missing DPA privacy counsel gate fails closed" \
  1 'external gate disposition missing required text.*DPA / terms / privacy posture'

mut_missing_trademark_external_gate() {
  perl -0pi -e 's/- \*\*Trademark \/ "official Clockify" language\.\*\* Requires legal\/product approval\.\n//' "$1"
}
MUTATOR=mut_missing_trademark_external_gate
run_case "missing trademark external gate fails closed" \
  1 'external gate disposition missing required text.*Trademark'

mut_missing_service_name_external_gate() {
  perl -0pi -e 's/- \*\*Clockify URI scheme and gRPC service-name branding review\.\*\* Requires legal\/product approval\.\n//' "$1"
}
MUTATOR=mut_missing_service_name_external_gate
run_case "missing gRPC service-name external gate fails closed" \
  1 "external gate disposition missing required text.*gRPC service-name"

mut_missing_public_content_scope() {
  perl -0pi -e 's/, with `Candidate branch file content: 0 open, 0 unknown`//' "$1"
}
MUTATOR=mut_missing_public_content_scope
run_case "missing public-content candidate scope fails closed" 1 'public-content audit disposition missing required text.*Candidate branch file content'

mut_missing_public_tls_scope() {
  perl -0pi -e 's/  The candidate branch-content TLS verification bypass marker check is closed\.\n//' "$1"
}
MUTATOR=mut_missing_public_tls_scope
run_case "missing public-content TLS bypass scope fails closed" 1 'public-content audit disposition missing required text.*TLS verification'

mut_missing_public_license_scope() {
  perl -0pi -e 's/  The candidate branch-content MIT LICENSE check is closed\.\n//' "$1"
}
MUTATOR=mut_missing_public_license_scope
run_case "missing public-content MIT LICENSE scope fails closed" 1 'public-content audit disposition missing required text.*MIT LICENSE'

mut_missing_public_gitignore_scope() {
  perl -0pi -e 's/  The candidate branch-content \.gitignore coverage check is closed\.\n//' "$1"
}
MUTATOR=mut_missing_public_gitignore_scope
run_case "missing public-content gitignore coverage scope fails closed" 1 'public-content audit disposition missing required text.*gitignore coverage'

mut_missing_public_gitleaks_allowlist_scope() {
  perl -0pi -e 's/  The candidate branch-content \.gitleaks\.toml allowlist description check is closed\.\n//' "$1"
}
MUTATOR=mut_missing_public_gitleaks_allowlist_scope
run_case "missing public-content gitleaks allowlist scope fails closed" 1 'public-content audit disposition missing required text.*gitleaks.toml allowlist'

mut_missing_public_claude_scope() {
  perl -0pi -e 's/  The candidate branch-content CLAUDE\.md workstation context check is closed\.\n//' "$1"
}
MUTATOR=mut_missing_public_claude_scope
run_case "missing public-content CLAUDE.md scope fails closed" 1 'public-content audit disposition missing required text.*CLAUDE.md workstation'

mut_missing_public_live_secret_scope() {
  perl -0pi -e 's/  The candidate branch-content live Clockify secret assignment check is closed\.\n//' "$1"
}
MUTATOR=mut_missing_public_live_secret_scope
run_case "missing public-content live secret scope fails closed" 1 'public-content audit disposition missing required text.*live Clockify secret'

mut_missing_public_history_scope() {
  perl -0pi -e 's/,\n  `Public history review: 0 open, 0 unknown`//' "$1"
}
MUTATOR=mut_missing_public_history_scope
run_case "missing public-content history scope fails closed" 1 'public-content audit disposition missing required text.*Public history review'

mut_missing_public_local_scope() {
  perl -0pi -e 's/, and\n  `Local artifact\/full-tree review: 0 open, 0 unknown`//' "$1"
}
MUTATOR=mut_missing_public_local_scope
run_case "missing public-content local scope fails closed" 1 'public-content audit disposition missing required text.*Local artifact/full-tree review'

if [ "$tests_failed" -ne 0 ]; then
  printf 'check-launch-review-ledger tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
  exit 1
fi

printf 'check-launch-review-ledger tests: %d/%d OK\n' "$tests_run" "$tests_run"
