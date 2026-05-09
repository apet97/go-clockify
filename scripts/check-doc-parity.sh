#!/usr/bin/env bash
#
# check-doc-parity.sh
#
# Release-blocking gate that keeps operator-facing documentation in
# sync with the code. The config-parity script (check-config-parity.sh)
# checks env-var *presence* in deployment manifests. This script
# checks *content correctness* — the docs that operators rely on
# during an incident or an upgrade must reference real symbols, not
# stale ones.
#
# Checks performed (all non-fatal warnings print, only the strict
# ones below fail the gate):
#
#   1. Every env var referenced in docs/ or README.md appears in
#      config.go OR is
#      explicitly listed as deprecated in
#      deploy/.config-parity-opt-out.txt. A doc referencing a
#      removed env var misleads operators.
#   2. Every tool name referenced in operator-facing docs or README.md
#      exists in docs/tool-catalog.json. ADR narrative is excluded so
#      hypothetical rename examples do not trigger false positives.
#      Prevents runbooks pointing at tools we never shipped or already
#      removed.
#   3. Public-surface banned strings do not appear in shipped
#      docs/README, and current public-surface tool/policy counts plus
#      OCI image descriptions match the generated catalog / config
#      surface.
#   4. SECURITY.md scope names the live auth, audit, tenant isolation,
#      and transport surfaces described by its Security Features
#      section, and auth-model observability references live metric
#      labels. Prevents D6-style drift where a mitigation is advertised
#      but not treated as an in-scope vulnerability report category.
#   5. GitHub issue templates/support/security/CODEOWNERS/onboarding
#      docs route vulnerability reports privately, name response timing,
#      assign sensitive-path ownership, and keep the public HTTPS clone
#      path usable after a visibility flip. CONTRIBUTING.md also keeps
#      local verification wording aligned with Makefile's tool-gated
#      skip behavior so skipped laptop checks cannot be mistaken for
#      launch evidence.
#   6. README npm compatibility matches the published npm package's
#      engine requirement.
#   7. Required hosted-launch runbook and monitoring entry points exist,
#      and no dangling TODO / TBD / FIXME / XXX appears in
#      docs/runbooks/, docs/deploy/, docs/adr/, or the top-level
#      operator docs. These markers signal unfinished operator guidance
#      and belong in issue tracker or PRs, not in shipped docs.
#   8. ADR index status labels match each ADR file's Status section.
#      Prevents accepted decisions from being shown as proposed, or
#      unresolved decisions from being shown as accepted.
#
# Usage:
#   bash scripts/check-doc-parity.sh
#
# Exit codes:
#   0 — all checks passed
#   1 — at least one strict check failed

set -euo pipefail

CONFIG_FILE="internal/config/config.go"
CATALOG_FILE="docs/tool-catalog.json"
OPT_OUT="deploy/.config-parity-opt-out.txt"
DOC_DIRS=(docs/runbooks docs/deploy docs/adr docs)
DOC_FILES_TOP=(README.md docs/support-matrix.md docs/upgrade-checklist.md docs/verify-release.md docs/production-readiness.md docs/clients.md)
NPM_PACKAGE_JSON="npm/clockify-mcp-go/package.json"

fail=0

warn() { echo "[warn] $*" >&2; }
err() { echo "[fail] $*" >&2; fail=1; }

# ---------------------------------------------------------------------------
# 1. Env-var content check
# ---------------------------------------------------------------------------

if [ ! -f "$CONFIG_FILE" ]; then
  err "config file missing: $CONFIG_FILE"
  exit 1
fi

# Known env vars are every "MCP_*" / "CLOCKIFY_*" string literal
# referenced anywhere in internal/** or cmd/** source. This
# deliberately over-includes — test-only literals
# (CLOCKIFY_LIVE_*, CLOCKIFY_RUN_LIVE_E2E, etc.) count as "defined"
# so a runbook that documents them for test harnesses does not
# trip the gate. Operator-facing mis-references still fail on
# removed vars, which is the case the gate exists for.
known_vars=$(
  {
    grep -rhoE '"(MCP|CLOCKIFY)_[A-Z0-9_]+"' internal/ cmd/ tests/ 2>/dev/null | sed 's/"//g'
    # Workflow files reference CLOCKIFY_LIVE_* and other CI-only env
    # vars that never appear in Go source. Counting them as defined
    # keeps operator docs honest without forcing a Go-side shim.
    grep -rhoE '\b(MCP|CLOCKIFY)_[A-Z0-9_]+' .github/ 2>/dev/null | sort -u
  } | sort -u)

opt_out_list=""
if [ -f "$OPT_OUT" ]; then
  opt_out_list=$(grep -v '^#' "$OPT_OUT" | grep -v '^$' | awk '{print $1}' || true)
fi

# Find every MCP_* / CLOCKIFY_* token in docs/README.
# `docs/superpowers/` is excluded — that subtree holds AI-assisted design
# specs which by definition describe future state (env vars to be added in
# upcoming PRs). They are planning artifacts, not operator documentation,
# and including them would force every brainstorm-spec commit to land
# alongside its corresponding implementation, defeating the point of writing
# the spec first.
referenced_vars=$(grep -rhoE --exclude-dir=superpowers '\b(MCP|CLOCKIFY)_[A-Z0-9_]{2,}' \
                   "${DOC_DIRS[@]}" 2>/dev/null | sort -u || true)
referenced_vars="$referenced_vars"$'\n'"$(grep -hoE '\b(MCP|CLOCKIFY)_[A-Z0-9_]{2,}' "${DOC_FILES_TOP[@]}" 2>/dev/null | sort -u || true)"
referenced_vars=$(printf "%s\n" "$referenced_vars" | sort -u | sed '/^$/d')

for var in $referenced_vars; do
  if echo "$known_vars" | grep -qx "$var"; then continue; fi
  if echo "$opt_out_list" | grep -qx "$var"; then continue; fi
  # Trailing underscore = grep stopped at a non-word boundary inside
  # a wider env-var name (e.g. an ASCII-art diagram broke a long var
  # over two lines, or doc prose used `MCP_FOO_*` as a glob). Skip;
  # the full name will appear elsewhere in the same doc and be
  # validated through that match.
  case "$var" in
    *_) continue ;;
  esac
  # A small allowlist for example-only names that never existed and
  # never will (e.g. placeholders in snippets). Expand here rather
  # than adding them to the real opt-out list.
  case "$var" in
    MCP_BEARER_TOKEN_EXAMPLE|CLOCKIFY_API_KEY_EXAMPLE) continue ;;
  esac
  err "env var referenced in docs but not defined in $CONFIG_FILE or opt-out: $var"
done

# ---------------------------------------------------------------------------
# 2. Tool-name content check
# ---------------------------------------------------------------------------

if [ -f "$CATALOG_FILE" ]; then
  known_tools=$(grep -oE '"name": *"clockify_[a-z0-9_]+"' "$CATALOG_FILE" \
                | sed 's/"name": *"//;s/"//' | sort -u)

  strict_tool_files=(README.md)
  while IFS= read -r -d '' file; do
    strict_tool_files+=("$file")
  done < <(find docs -type f -name '*.md' ! -path 'docs/adr/*' ! -path 'docs/superpowers/*' -print0)

  referenced_tools=$(
    grep -hoE '\bclockify_[a-z0-9_]{3,}' "${strict_tool_files[@]}" 2>/dev/null \
      | sort -u || true
  )

  for tool in $referenced_tools; do
    # Trailing underscore = the regex matched a glob-prefix
    # reference like `clockify_list_*` or `clockify_get_*` in
    # narrative prose; the asterisk is not [a-z0-9_] so the
    # capture truncates at the underscore. These are wildcards,
    # not real tool names — skip without faulting.
    case "$tool" in
      *_) continue ;;
    esac
    # Skip common non-tool prefixes / snippets that share the clockify_ stem.
    case "$tool" in
      clockify_mcp*|clockify_outage*|clockify_upstream*|clockify_policy|clockify_api_key*|clockify_admin) continue ;;
    esac
    if echo "$known_tools" | grep -qx "$tool"; then continue; fi
    err "tool referenced in operator docs but not in $CATALOG_FILE: $tool"
  done
else
  warn "$CATALOG_FILE missing — run 'make gen-tool-catalog'"
fi

# ---------------------------------------------------------------------------
# 3. Banned stale public-surface strings
# ---------------------------------------------------------------------------

banned_strings=(
  "@anycli/clockify-mcp-go"
  "defaulting to a preview if the \`dry_run\` parameter is omitted"
  "Destructive tools run through a dry-run interceptor by default"
  "through the dry-run interceptor unless the caller opts into"
  "four policy modes"
  "4 policy modes"
  "official Clockify launch candidate"
  "official Clockify launch-candidate"
  "officially-supported Clockify product"
  "official Clockify product launch"
  "release-check: OK - shippable"
  "release-check: OK — shippable"
  "is this repo shippable right now?"
  "remaining public-content work is local ignored/nested"
  "This is the launch-blocking gap today"
  "There is no end-to-end test that boots a Postgres-tagged binary"
  "covers all 15 binaries since the"
  "Signed releases (cosign + SLSA)"
  "same SBOM + cosign + SLSA chain"
  "same SBOM + cosign sigstore + SLSA"
  "cosign / SLSA chain"
  "cosign + SLSA chain"
  "every binary and container image ships with cosign signatures, SPDX SBOM, and SLSA build provenance"
  "attested with SLSA build provenance, and scanned with"
  "A SLSA build-provenance attestation stored in the GitHub"
)

banned_scan_files=("${DOC_DIRS[@]}" "${DOC_FILES_TOP[@]}" AGENTS.md .github/workflows/docker-image.yml .github/workflows/release.yml)
for needle in "${banned_strings[@]}"; do
  hits=$(grep -rnF "$needle" "${banned_scan_files[@]}" 2>/dev/null || true)
  if [ -n "$hits" ]; then
    while IFS= read -r line; do
      err "banned stale public-surface string found: $line"
    done <<< "$hits"
  fi
done

multiline_brand_claim_hits=$(
  perl -0ne '
    if (/official\s+Clockify\s+launch[-[:space:]]candidate/i ||
        /officially-supported\s+Clockify\s+product/i ||
        /official\s+Clockify\s+product\s+launch/i) {
      print "$ARGV\n";
    }
  ' "${DOC_DIRS[@]}" "${DOC_FILES_TOP[@]}" AGENTS.md 2>/dev/null | sort -u || true)

if [ -n "$multiline_brand_claim_hits" ]; then
  while IFS= read -r line; do
    err "banned premature official-product claim found: $line"
  done <<< "$multiline_brand_claim_hits"
fi

if [ ! -f README.md ]; then
  err "README.md missing"
else
  if ! grep -qF "SLSA build provenance is attached when GitHub artifact attestations are available" README.md; then
    err "README.md must qualify SLSA provenance on GitHub artifact attestation availability"
  fi
fi

if [ -f docs/verify-release.md ]; then
  required_verify_release_slsa_patterns=(
    "Each binary always ships"
    "When GitHub artifact attestations are available for the repository"
    "mandatory cryptographic gate is the cosign"
    "binary/image signature chain"
  )
  for pattern in "${required_verify_release_slsa_patterns[@]}"; do
    if ! grep -qF "$pattern" docs/verify-release.md; then
      err "docs/verify-release.md must qualify SLSA provenance on GitHub artifact attestation availability: $pattern"
    fi
  done
fi

if [ -f .github/workflows/docker-image.yml ]; then
	  required_docker_slsa_patterns=(
	    "SLSA provenance when"
	    "GitHub artifact attestations are available"
	    "best-effort for the current user-owned private repository"
	    "image build, scan, cosign signature, and SBOM gates remain mandatory"
	    "id: slsa_provenance"
	    "Report SLSA attestation feature-gate skip"
	    "steps.slsa_provenance.outcome == 'failure'"
	    "SLSA attestation skipped"
	    "ADR-0013"
	    "mandatory image build, Trivy, cosign image signature, and SBOM attestation gates already passed"
	  )
  for pattern in "${required_docker_slsa_patterns[@]}"; do
    if ! grep -qF "$pattern" .github/workflows/docker-image.yml; then
      err ".github/workflows/docker-image.yml must qualify image SLSA provenance availability: $pattern"
    fi
  done
fi

if [ -d .github/workflows ]; then
  unpinned_workflow_uses="$(
    grep -RInE 'uses:[[:space:]]+[^[:space:]#]+@[^[:space:]#]+' .github/workflows \
      | grep -Ev '@[0-9a-f]{40}([[:space:]#]|$)' || true
  )"
  if [ -n "$unpinned_workflow_uses" ]; then
    while IFS= read -r line; do
      err "GitHub workflow action must be pinned to a full commit SHA: $line"
    done <<< "$unpinned_workflow_uses"
  fi
fi

if [ -f "$CATALOG_FILE" ]; then
  expected_tool_count=$(grep -oE '"name": *"clockify_[a-z0-9_]+"' "$CATALOG_FILE" | wc -l | tr -d '[:space:]')
  current_count_files=(README.md CONTRIBUTING.md SECURITY.md AGENTS.md docs/README.md docs/clients.md docs/production-readiness.md docs/support-matrix.md .github/workflows/docker-image.yml deploy/Dockerfile)

  for file in "${current_count_files[@]}"; do
    [ -f "$file" ] || continue
    while IFS= read -r line; do
      line_no=${line%%:*}
      text=${line#*:}
      # Tier-specific counts (for example "Tier 1 (40 tools)") are not
      # total-catalog claims. Everything else on the current public surface
      # that says "N tools" is expected to name the generated catalog total.
      if grep -Eiq 'Tier[- ]?[12]|tool_count|total_visible_tools|visible tools|on-demand|always[- ]on|always loaded|\bthen[[:space:]]+[0-9]{2,3}[[:space:]]+generated tools\b' <<< "$text"; then
        continue
      fi
      while IFS= read -r n; do
        [ -n "$n" ] || continue
        if [ "$n" != "$expected_tool_count" ]; then
          err "public-surface tool-count drift in $file:$line_no: found $n tools, expected $expected_tool_count from $CATALOG_FILE"
        fi
      done < <(grep -oE '\b[0-9]{2,3}[[:space:]]+tools\b' <<< "$text" | grep -oE '[0-9]{2,3}' || true)
    done < <(grep -nE '\b[0-9]{2,3}[[:space:]]+tools\b' "$file" 2>/dev/null || true)

    while IFS= read -r line; do
      line_no=${line%%:*}
      text=${line#*:}
      n=$(grep -oE '\b[0-9]{2,3}\b' <<< "$text" | head -1 || true)
      if [ -n "$n" ] && [ "$n" != "$expected_tool_count" ]; then
        err "public-surface tool-handler count drift in $file:$line_no: found $n handlers, expected $expected_tool_count from $CATALOG_FILE"
      fi
    done < <(grep -nE '\b[Aa]ll[[:space:]]+[0-9]{2,3}[[:space:]]+tool handlers\b' "$file" 2>/dev/null || true)
  done

  oci_metadata_files=(.github/workflows/docker-image.yml deploy/Dockerfile)
  for file in "${oci_metadata_files[@]}"; do
    [ -f "$file" ] || continue
    oci_description_lines=$(grep -nE 'org\.opencontainers\.image\.description[= ]' "$file" 2>/dev/null || true)
    if [ -z "$oci_description_lines" ]; then
      err "OCI image description missing from public metadata surface: $file"
      continue
    fi
    while IFS= read -r line; do
      line_no=${line%%:*}
      text=${line#*:}
      if ! grep -Eq "\b${expected_tool_count}[[:space:]]+tools\b" <<< "$text"; then
        err "OCI image description tool-count drift in $file:$line_no: expected ${expected_tool_count} tools"
      fi
      if ! grep -Eiq '\bfive[[:space:]-]+policy[[:space:]-]+modes\b' <<< "$text"; then
        err "OCI image description policy-count drift in $file:$line_no: expected five policy modes"
      fi
    done <<< "$oci_description_lines"
  done
fi

# ---------------------------------------------------------------------------
# 4. SECURITY.md scope must cover live security surfaces
# ---------------------------------------------------------------------------

if [ ! -f SECURITY.md ]; then
  err "SECURITY.md missing"
else
  security_scope="$(awk '
    /^## Scope[[:space:]]*$/ { in_scope = 1; next }
    /^## Security Features[[:space:]]*$/ { in_scope = 0 }
    in_scope { print }
  ' SECURITY.md)"

  if [ -z "$security_scope" ]; then
    err "SECURITY.md Scope section is missing or empty"
  else
    required_security_scope=(
      "API key exposure or leakage"
      "Authentication bypass in HTTP transport"
      "OIDC/JWKS validation weaknesses"
      "Tenant-isolation failures"
      "Audit-durability failures"
      "Path traversal in ID validation"
      "CORS bypass in HTTP transport"
      "DNS rebinding or private-network exposure"
      "Timing attacks on bearer token comparison"
    )

    for pattern in "${required_security_scope[@]}"; do
      if ! grep -qiF "$pattern" <<< "$security_scope"; then
        err "SECURITY.md scope missing live security surface: $pattern"
      fi
    done
  fi

  if grep -qF 'clockify_mcp_audit_failures_total{reason="persist_error"}' SECURITY.md; then
    err "SECURITY.md audit failure metric is missing the phase label"
  fi
  if ! grep -qF 'clockify_mcp_audit_failures_total{reason="persist_error",phase="outcome"}' SECURITY.md; then
    err "SECURITY.md must document outcome-phase audit-failure monitoring"
  fi
  required_transport_header_terms=(
    "Strict-Transport-Security"
    "when TLS or a trusted HTTPS proxy is active"
    "Cross-Origin-Opener-Policy: same-origin"
    "Cross-Origin-Embedder-Policy: require-corp"
    "Cross-Origin-Resource-Policy: same-origin"
  )
  for pattern in "${required_transport_header_terms[@]}"; do
    if ! grep -qF "$pattern" SECURITY.md; then
      err "SECURITY.md must document the P3-5 HTTP baseline header: $pattern"
    fi
  done
fi

auth_model_doc="docs/auth-model.md"
if [ ! -f "$auth_model_doc" ]; then
  err "$auth_model_doc missing"
else
  if grep -qF "clockify_mcp_per_subject_rate_limited_total" "$auth_model_doc"; then
    err "$auth_model_doc references removed rate-limit metric clockify_mcp_per_subject_rate_limited_total"
  fi
  if ! grep -qF "clockify_mcp_rate_limit_rejections_total" "$auth_model_doc"; then
    err "$auth_model_doc must document clockify_mcp_rate_limit_rejections_total for per-subject rate-limit observability"
  fi
  if ! grep -qF 'scope="per_token"' "$auth_model_doc"; then
    err "$auth_model_doc must document the per-token rate-limit metric scope label"
  fi
fi

clients_doc="docs/clients.md"
if [ ! -f "$clients_doc" ]; then
  err "$clients_doc missing"
else
  required_clients_server_identity=(
    "serverInfo.name"
    "clockify-go-mcp"
    "clockify-mcp"
    "@apet97/clockify-mcp-go"
    "packaging names"
  )
  for pattern in "${required_clients_server_identity[@]}"; do
    if ! grep -qF "$pattern" "$clients_doc"; then
      err "$clients_doc must document protocol server identity versus packaging names: $pattern"
    fi
  done
  required_clients_protocol_default=(
    "MCP_DEFAULT_PROTOCOL_VERSION"
    "params.protocolVersion"
    "omits"
    "newest supported version"
    "client versions are still echoed"
  )
  for pattern in "${required_clients_protocol_default[@]}"; do
    if ! grep -qF "$pattern" "$clients_doc"; then
      err "$clients_doc must document default protocol-version fallback semantics: $pattern"
    fi
  done
fi

# ---------------------------------------------------------------------------
# 5. Public-repo onboarding surfaces must route security / ownership clearly
# ---------------------------------------------------------------------------

issue_template_config=".github/ISSUE_TEMPLATE/config.yml"
if [ ! -f "$issue_template_config" ]; then
  err "GitHub issue-template config missing: $issue_template_config"
else
  if ! grep -qF "blank_issues_enabled: false" "$issue_template_config"; then
    err "$issue_template_config must disable blank public issues"
  fi
  if ! grep -qF "security/advisories/new" "$issue_template_config"; then
    err "$issue_template_config must link to the private Security Advisory flow"
  fi
fi

for file in .github/ISSUE_TEMPLATE/bug_report.yml .github/ISSUE_TEMPLATE/feature_request.yml; do
  if [ ! -f "$file" ]; then
    err "GitHub issue template missing: $file"
    continue
  fi
  if ! grep -qiE 'security advisory|security vulnerabilities|vulnerability details' "$file"; then
    err "$file must steer security reports away from public issues"
  fi
  if ! grep -qiE 'secret|api key|token|personal data' "$file"; then
    err "$file must warn reporters not to paste secrets or personal data"
  fi
done

unredacted_issue_examples=$(grep -RIn 'CLOCKIFY_API_KEY=\.\.\.' .github/ISSUE_TEMPLATE 2>/dev/null || true)
if [ -n "$unredacted_issue_examples" ]; then
  while IFS= read -r line; do
    err "issue template contains unredacted secret placeholder: $line"
  done <<< "$unredacted_issue_examples"
fi

if [ ! -f SECURITY.md ]; then
  err "SECURITY.md missing"
else
  if ! grep -qF "security/advisories/new" SECURITY.md; then
    err "SECURITY.md must link to the private Security Advisory flow"
  fi
  if ! grep -qEi 'Acknowledg(e)?ment:\*\*[[:space:]]+Within 48 hours' SECURITY.md; then
    err "SECURITY.md response timeline must promise vulnerability acknowledgement within 48 hours"
  fi
  if ! grep -qEi 'Initial assessment:\*\*[[:space:]]+Within 1 week' SECURITY.md; then
    err "SECURITY.md response timeline must include initial assessment within 1 week"
  fi
  required_security_slsa_patterns=(
    "GitHub artifact attestations"
    "ADR-0013 keeps SLSA best-effort"
    "mandatory"
    "cryptographic gate"
    "cosign binary/image chain"
  )
  for pattern in "${required_security_slsa_patterns[@]}"; do
    if ! grep -qF "$pattern" SECURITY.md; then
      err "SECURITY.md must qualify private-repo SLSA availability and cosign fallback: $pattern"
    fi
  done
fi

if [ ! -f SUPPORT.md ]; then
  err "SUPPORT.md missing"
else
  if ! grep -qF "security/advisories/new" SUPPORT.md; then
    err "SUPPORT.md must link security reports to GitHub Security Advisories"
  fi
  if ! grep -qEi 'Security disclosures.*48 hours' SUPPORT.md; then
    err "SUPPORT.md must state security disclosure acknowledgement timing"
  fi
  if grep -qF "mandatory on every release since the public flip" SUPPORT.md; then
    err "SUPPORT.md must not claim SLSA is mandatory while private-repo attestation remains best-effort"
  fi
  required_support_slsa_patterns=(
    "GitHub artifact attestations"
    "Feature not available"
    "ADR-0013 keeps SLSA best-effort"
    "mandatory cryptographic"
    "cosign binary/image chain"
  )
  for pattern in "${required_support_slsa_patterns[@]}"; do
    if ! grep -qF "$pattern" SUPPORT.md; then
      err "SUPPORT.md must document private-repo SLSA limitation and cosign fallback: $pattern"
    fi
  done
fi

release_smoke_workflow=".github/workflows/release-smoke.yml"
if [ ! -f "$release_smoke_workflow" ]; then
  err "release-smoke workflow missing: $release_smoke_workflow"
else
  broad_slsa_skip="$(grep -nE 'grep[[:space:]]+-qiE?[[:space:]]+.*HTTP 404' "$release_smoke_workflow" || true)"
  if [ -n "$broad_slsa_skip" ]; then
    while IFS= read -r line; do
      err "release-smoke SLSA skip must not accept a bare HTTP 404: $line"
    done <<< "$broad_slsa_skip"
  fi
  if ! grep -qF "Do not skip on a bare 404" "$release_smoke_workflow" ||
     ! grep -qF "Feature not available" "$release_smoke_workflow" ||
     ! grep -qF "user-owned private repositor" "$release_smoke_workflow" ||
     ! grep -qF "cosign binary/image checks remain mandatory" "$release_smoke_workflow"; then
    err "release-smoke.yml must narrowly skip only GitHub's user-owned-private attestation feature gate"
  fi
  required_doctor_artifact_patterns=(
    "release-smoke-evidence/release-doctor-strict-ok.txt"
    "release-smoke-evidence/release-doctor-strict-fail.txt"
    "release-smoke-evidence/release-doctor-postgres-ok.txt"
    "Validate doctor strict evidence files"
    "Missing doctor strict evidence"
    "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a"
    "release-smoke-doctor-output"
    "if-no-files-found: error"
  )
  for pattern in "${required_doctor_artifact_patterns[@]}"; do
    if ! grep -qF "$pattern" "$release_smoke_workflow"; then
      err "release-smoke.yml must archive doctor strict output as a workflow artifact: $pattern"
    fi
  done
fi

deploy_workflow=".github/workflows/deploy.yml"
if [ ! -f "$deploy_workflow" ]; then
  err "deploy workflow missing: $deploy_workflow"
else
  broad_deploy_slsa_skip="$(grep -nE 'grep[[:space:]]+-qiE?[[:space:]]+.*HTTP 404' "$deploy_workflow" || true)"
  if [ -n "$broad_deploy_slsa_skip" ]; then
    while IFS= read -r line; do
      err "deploy SLSA skip must not accept a bare HTTP 404: $line"
    done <<< "$broad_deploy_slsa_skip"
  fi
  if grep -qF "Always runs" "$deploy_workflow"; then
    err "deploy.yml must not claim SLSA provenance always runs while artifact attestations are account-tier gated"
  fi
  if ! grep -qF "Do not skip on a bare 404" "$deploy_workflow" ||
     ! grep -qF "Feature not available" "$deploy_workflow" ||
     ! grep -qF "user-owned private repositor" "$deploy_workflow" ||
     ! grep -qF "cosign image verification remains mandatory" "$deploy_workflow"; then
    err "deploy.yml must narrowly skip only GitHub's user-owned-private attestation feature gate"
  fi
fi

codeowners_file=".github/CODEOWNERS"
if [ ! -f "$codeowners_file" ]; then
  err "CODEOWNERS missing: $codeowners_file"
else
  required_codeowners=(
    '^\*[[:space:]]+@apet97([[:space:]]|$)'
    '^/internal/authn/[[:space:]]+@apet97([[:space:]]|$)'
    '^/internal/enforcement/[[:space:]]+@apet97([[:space:]]|$)'
    '^/internal/policy/[[:space:]]+@apet97([[:space:]]|$)'
    '^/internal/transport/[[:space:]]+@apet97([[:space:]]|$)'
    '^/internal/clockify/[[:space:]]+@apet97([[:space:]]|$)'
    '^/.github/[[:space:]]+@apet97([[:space:]]|$)'
    '^/.goreleaser\.yaml[[:space:]]+@apet97([[:space:]]|$)'
    '^/deploy/[[:space:]]+@apet97([[:space:]]|$)'
    '^/SECURITY\.md[[:space:]]+@apet97([[:space:]]|$)'
    '^/SUPPORT\.md[[:space:]]+@apet97([[:space:]]|$)'
  )
  for pattern in "${required_codeowners[@]}"; do
    if ! grep -qE "$pattern" "$codeowners_file"; then
      err "$codeowners_file missing public-sensitive ownership pattern: $pattern"
    fi
  done
fi

if [ ! -f CONTRIBUTING.md ]; then
  err "CONTRIBUTING.md missing"
else
  if ! grep -qF "git clone https://github.com/apet97/go-clockify.git" CONTRIBUTING.md; then
    err "CONTRIBUTING.md must use the public-compatible HTTPS clone command"
  fi
  if ! grep -qF "When the repository is public" CONTRIBUTING.md ||
     ! grep -qF "works without additional auth" CONTRIBUTING.md; then
    err "CONTRIBUTING.md must state that the HTTPS clone works without additional auth when public"
  fi
  if ! grep -qF "golangci-lint | **skips** with a warning if not installed; CI enforces" CONTRIBUTING.md; then
    err "CONTRIBUTING.md must state local golangci-lint skips when absent and CI enforces it"
  fi
  if ! grep -qF 'verify-vuln` | tools/govulncheck | always installs and runs the pinned `govulncheck` module with the repo Go pin' CONTRIBUTING.md; then
    err "CONTRIBUTING.md must state verify-vuln runs the pinned govulncheck tool module"
  fi
  if ! grep -qF "Tool-gated" CONTRIBUTING.md ||
     ! grep -qF "CI remains authoritative" CONTRIBUTING.md ||
     ! grep -qF "make release-check" CONTRIBUTING.md; then
    err "CONTRIBUTING.md must distinguish pre-PR local verification from the pre-ship release-check gate"
  fi
fi

if [ ! -f README.md ]; then
  err "README.md missing"
else
  if ! grep -qF "make verify  # pre-PR local pipeline" README.md ||
     ! grep -qF "make release-check  # pre-ship/tag gate" README.md ||
     ! grep -qF "CI remains authoritative for skipped" README.md ||
     ! grep -qF "Neither command replaces the external launch-candidate evidence gates" README.md; then
    err "README.md must distinguish pre-PR local verification, pre-ship release-check, skipped local tiers, and external launch-candidate evidence"
  fi
fi

if [ ! -f Makefile ]; then
  err "Makefile missing"
else
  if grep -qF "release-check: OK — shippable" Makefile ||
     grep -qF "is this repo shippable right now?" Makefile; then
    err "Makefile release-check output must not imply local checks alone prove shippability"
  fi
  if ! grep -qF "release-check: OK — local pre-ship gate passed" Makefile ||
     ! grep -qF "launch readiness still" Makefile; then
    err "Makefile release-check output must state local pre-ship success without replacing external launch evidence"
  fi
fi

# ---------------------------------------------------------------------------
# 6. README npm compatibility must match published package engines
# ---------------------------------------------------------------------------

if [ ! -f "$NPM_PACKAGE_JSON" ]; then
  err "npm package file missing: $NPM_PACKAGE_JSON"
else
  # `|| true` is load-bearing: when the row / engine line is missing,
  # grep returns 1, and under `set -euo pipefail` the substitution
  # would otherwise abort silently before the dedicated `[ -z … ]`
  # err lines below could fire — leaving CI to fail closed with no
  # diagnostic.
  readme_node=$(grep -E '^\| Node\.js \(npm wrapper\) \|' README.md | sed -E 's/.*\| ([0-9]+)\+ \|/\1/' | tr -d '[:space:]' || true)
  package_node=$(grep -E '"node": *">=[0-9]+"' "$NPM_PACKAGE_JSON" | sed -E 's/.*">=([0-9]+)".*/\1/' | tr -d '[:space:]' || true)
  if [ -z "$readme_node" ]; then
    err "README.md missing Node.js (npm wrapper) compatibility row"
  fi
  if [ -z "$package_node" ]; then
    err "$NPM_PACKAGE_JSON missing node engine declaration"
  fi
  if [ -n "$readme_node" ] && [ -n "$package_node" ] && [ "$readme_node" != "$package_node" ]; then
    err "README Node.js (npm wrapper) compatibility ($readme_node+) does not match $NPM_PACKAGE_JSON (>=${package_node})"
  fi
fi

# ---------------------------------------------------------------------------
# 7. Required runbooks / monitoring + TODO / TBD / FIXME / XXX check
# ---------------------------------------------------------------------------

required_runbooks=(
  "docs/runbooks/rate-limit.md"
  "docs/runbooks/rate-limit-saturation.md"
  "docs/runbooks/audit-durability.md"
  "docs/runbooks/legacy-http-eol.md"
  "docs/runbooks/postgres-restore.md"
  "docs/runbooks/tenant-offboarding.md"
)

for file in "${required_runbooks[@]}"; do
  if [ ! -f "$file" ]; then
    err "required launch runbook missing: $file"
  fi
done

required_launch_docs=(
  "AGENTS.md"
  "docs/release/brand-legal-review.md"
  "docs/deploy/production-profile-shared-service.md"
  "docs/production-readiness.md"
  "docs/official-clockify-mcp-gap-analysis.md"
)

for file in "${required_launch_docs[@]}"; do
  if [ ! -f "$file" ]; then
    err "required launch doc missing: $file"
  fi
done

if [ ! -f .github/dependabot.yml ]; then
  err ".github/dependabot.yml missing"
else
  dependabot_gomod_dirs=$(
    awk '
      /^[[:space:]]*-[[:space:]]*package-ecosystem:/ {
        ecosystem=$0
        sub(/^.*package-ecosystem:[[:space:]]*/, "", ecosystem)
        gsub(/"/, "", ecosystem)
        next
      }
      /^[[:space:]]*directory:/ {
        dir=$0
        sub(/^.*directory:[[:space:]]*/, "", dir)
        gsub(/"/, "", dir)
        if (ecosystem == "gomod") {
          print dir
        }
      }
    ' .github/dependabot.yml
  )
  required_dependabot_gomod_dirs=(
    "/"
    "/internal/controlplane/postgres"
    "/internal/transport/grpc"
    "/internal/tracing/otel"
    "/tools/govulncheck"
  )
  for dir in "${required_dependabot_gomod_dirs[@]}"; do
    if ! printf "%s\n" "$dependabot_gomod_dirs" | grep -qxF "$dir"; then
      err ".github/dependabot.yml must watch Go module dependency updates for $dir"
    fi
  done
fi

if [ ! -f .github/workflows/ci.yml ]; then
  err ".github/workflows/ci.yml missing"
else
  if ! grep -qF "tools/govulncheck && GOWORK=off go install golang.org/x/vuln/cmd/govulncheck" .github/workflows/ci.yml; then
    err ".github/workflows/ci.yml must install govulncheck from the tools/govulncheck module"
  fi
  if ! grep -qF "govulncheck -version" .github/workflows/ci.yml; then
    err ".github/workflows/ci.yml must print govulncheck -version before scanning"
  fi
fi

if [ ! -f tools/govulncheck/go.mod ]; then
  err "tools/govulncheck/go.mod missing"
else
  if ! grep -qF "module github.com/apet97/go-clockify/tools/govulncheck" tools/govulncheck/go.mod; then
    err "tools/govulncheck/go.mod must declare the govulncheck tool module"
  fi
  if ! grep -qF "require golang.org/x/vuln v1.3.0" tools/govulncheck/go.mod; then
    err "tools/govulncheck/go.mod must pin golang.org/x/vuln v1.3.0"
  fi
fi

if [ ! -f tools/govulncheck/tools.go ]; then
  err "tools/govulncheck/tools.go missing"
else
  if ! grep -qF 'import _ "golang.org/x/vuln/cmd/govulncheck"' tools/govulncheck/tools.go; then
    err "tools/govulncheck/tools.go must keep govulncheck reachable for module updates"
  fi
fi

if [ ! -f Makefile ]; then
  err "Makefile missing"
else
  if grep -qF "[verify-vuln] govulncheck not installed, skipping" Makefile; then
    err "Makefile verify-vuln must not skip when govulncheck is absent; it must run the pinned tool module"
  fi
  required_verify_vuln_terms=(
    "tools/govulncheck/go.mod"
    "cd tools/govulncheck"
    "GOWORK=off"
    'GOBIN="$$tmpdir"'
    'GOTOOLCHAIN="go$$go_pin"'
    "go install golang.org/x/vuln/cmd/govulncheck"
    'GOTOOLCHAIN="go$$go_pin" "$$tmpdir/govulncheck" -version'
    '$$tmpdir/govulncheck" ./...'
  )
  for pattern in "${required_verify_vuln_terms[@]}"; do
    if ! grep -qF "$pattern" Makefile; then
      err "Makefile verify-vuln must run the pinned govulncheck tool module with the repo Go pin: $pattern"
    fi
  done
fi

if [ -f .github/workflows/dependency-review.yml ]; then
  required_dependency_review_terms=(
    "pull_request:"
    "push:"
    "branches: [main]"
    "actions/dependency-review-action@2031cfc080254a8a887f58cffee85186f0e49e48"
    "vulnerability-check: true"
    "license-check: false"
    "fail-on-severity: high"
    "base-ref:"
    "head-ref:"
    "github.event.before"
    "github.sha"
  )
  for pattern in "${required_dependency_review_terms[@]}"; do
    if ! grep -qF "$pattern" .github/workflows/dependency-review.yml; then
      err "dependency-review.yml must support PR checks and first default-branch evidence runs: $pattern"
    fi
  done
fi

if [ -f docs/release/brand-legal-review.md ]; then
  if ! grep -qF "NEEDS LEGAL REVIEW" docs/release/brand-legal-review.md ||
     ! grep -qF "L-10" docs/release/brand-legal-review.md ||
     ! grep -qF "official-product" docs/release/brand-legal-review.md ||
     ! grep -qF "rebrand decision" docs/release/brand-legal-review.md; then
    err "docs/release/brand-legal-review.md must frame the L-10 legal/product approval gate without closing it"
  fi
  required_brand_identifier_terms=(
    "clockify://"
    "resource URI scheme"
    "clockify.mcp.v1.MCP"
    "gRPC service name"
    "internal/transport/grpc/service.go"
    "before clients depend"
  )
  for pattern in "${required_brand_identifier_terms[@]}"; do
    if ! grep -qF "$pattern" docs/release/brand-legal-review.md; then
      err "docs/release/brand-legal-review.md must keep the URI and gRPC service-name brand review open: $pattern"
    fi
  done
  required_brand_license_terms=(
    "go-licenses"
    "make license-evidence"
    "scripts/collect-license-evidence.sh"
    "go list -deps"
    "internal/controlplane/postgres"
    "internal/transport/grpc"
    "internal/tracing/otel"
    "npm/package.json.tmpl"
    ".github/workflows/dependency-review.yml"
    "license-check: false"
    "not legal advice"
    "not license clearance"
  )
  for pattern in "${required_brand_license_terms[@]}"; do
    if ! grep -qF "$pattern" docs/release/brand-legal-review.md; then
      err "docs/release/brand-legal-review.md must name dependency/license evidence sources for B.08: $pattern"
    fi
  done
fi

if [ -f docs/deploy/production-profile-shared-service.md ]; then
  if ! grep -qF "shared-service profile gate" docs/deploy/production-profile-shared-service.md ||
     ! grep -qF "Group 2" docs/deploy/production-profile-shared-service.md ||
     ! grep -qF "does not" docs/deploy/production-profile-shared-service.md ||
     ! grep -qF "launch-ready" docs/deploy/production-profile-shared-service.md ||
     ! grep -qF "Group 1" docs/deploy/production-profile-shared-service.md ||
     ! grep -qF "Group 6" docs/deploy/production-profile-shared-service.md ||
     ! grep -qF "Group 7 external evidence gates" docs/deploy/production-profile-shared-service.md; then
    err "docs/deploy/production-profile-shared-service.md must scope its green artifacts to Group 2 without replacing Group 1/6/7 launch evidence"
  fi
fi

if [ -f docs/deploy/profile-private-network-grpc.md ]; then
  required_grpc_reflection_terms=(
    "gRPC reflection is intentionally not registered"
    "release"
    "artifacts"
    "Keep it off in production"
    "local protocol exploration only"
    "-tags=grpc,grpcreflection"
    "do not deploy"
  )
  for pattern in "${required_grpc_reflection_terms[@]}"; do
    if ! grep -qF -- "$pattern" docs/deploy/profile-private-network-grpc.md; then
      err "docs/deploy/profile-private-network-grpc.md must document T-17 reflection-off posture and dev-only grpcreflection tag: $pattern"
    fi
  done
fi

if [ -f AGENTS.md ]; then
  required_agents_read_first_terms=(
    "docs/launch-readiness-review-may-8.md"
    "May 8 review disposition ledger"
    "objective-to-artifact completion audit"
    "Do not mark launch-ready"
  )
  for pattern in "${required_agents_read_first_terms[@]}"; do
    if ! grep -qF "$pattern" AGENTS.md; then
      err "AGENTS.md must route agents to the May 8 review ledger and completion audit: $pattern"
    fi
  done
fi

if [ -f docs/production-readiness.md ]; then
  if grep -qF "external evidence only" docs/production-readiness.md; then
    err "docs/production-readiness.md must not narrow remaining launch blockers to external evidence only"
  fi
  required_production_readiness_terms=(
    "blockers are not local test failures"
    "Group 1"
    "Group 6"
    "Group 7"
    "pushed workflow"
    "repository-state cleanup"
    "public-readiness"
    "legal/product approval"
    "not sufficient"
    "official/product launch-ready claim"
  )
  for pattern in "${required_production_readiness_terms[@]}"; do
    if ! grep -qF "$pattern" docs/production-readiness.md; then
      err "docs/production-readiness.md must keep local green checks scoped below external/repo/legal launch gates: $pattern"
    fi
  done
fi

if [ -f docs/official-clockify-mcp-gap-analysis.md ]; then
  if grep -qF "Only external evidence blockers remain" docs/official-clockify-mcp-gap-analysis.md; then
    err "docs/official-clockify-mcp-gap-analysis.md must not narrow remaining launch blockers to external evidence only"
  fi
  required_gap_analysis_blocker_terms=(
    "blockers are not local test failures"
    "Group 1"
    "Group 6"
    "Group 7"
    "pushed workflow"
    "repository-state cleanup"
    "public-readiness"
    "hosted/platform"
    "legal/product approval"
    "not sufficient"
    "Clockify-supported product launch"
  )
  for pattern in "${required_gap_analysis_blocker_terms[@]}"; do
    if ! grep -qF "$pattern" docs/official-clockify-mcp-gap-analysis.md; then
      err "docs/official-clockify-mcp-gap-analysis.md must keep local green checks scoped below external/repo/legal/product launch gates: $pattern"
    fi
  done
fi

if [ -f docs/agent-handoff.md ]; then
  if grep -qF "Open external evidence only" docs/agent-handoff.md; then
    err "docs/agent-handoff.md must not label Group 1/6/7 as the only open evidence"
  fi
  if ! grep -qF "Open launch-evidence gates (not a complete blocker list)" docs/agent-handoff.md; then
    err "docs/agent-handoff.md must scope its Group 1/6/7 summary as an incomplete launch-evidence list"
  fi
  required_agent_handoff_landing_terms=(
    "Land the remediation tree only after explicit approval."
    "git status --short --branch"
    "make doc-parity"
    "git diff --check"
    "Do not use \`git add .\` from a parent workspace."
    "Why:"
    "Verified:"
    "Do not push until the"
    "whole staged group is locally green."
    "Refresh external status on the landed SHA."
    "bash scripts/check-launch-external-status.sh --fail-open"
  )
  for pattern in "${required_agent_handoff_landing_terms[@]}"; do
    if ! grep -qF -- "$pattern" docs/agent-handoff.md; then
      err "docs/agent-handoff.md must preserve the permissioned landing sequence: $pattern"
    fi
  done
fi

if [ ! -f docs/adr/0001-stdlib-only-default-build.md ]; then
  err "ADR 0001 missing: docs/adr/0001-stdlib-only-default-build.md"
else
  required_jsonschema_adr_terms=(
    "internal/jsonschema"
    "Draft 2020-12 subset"
    'internal/tools/schema_keyword_test.go'
    'github.com/santhosh-tekuri/jsonschema/v6'
    '$ref'
    '$defs'
    "conditionals"
    "default SBOM"
  )
  for pattern in "${required_jsonschema_adr_terms[@]}"; do
    if ! grep -qF "$pattern" docs/adr/0001-stdlib-only-default-build.md; then
      err "ADR 0001 must document the B.07 stdlib-only JSON Schema validator trade-off: $pattern"
    fi
  done
fi

if [ -f docs/runbooks/rate-limit.md ]; then
  if ! grep -qF "rate-limit-saturation.md" docs/runbooks/rate-limit.md ||
     ! grep -qF "MCP_HTTP_RATELIMIT_PER_PRINCIPAL" docs/runbooks/rate-limit.md ||
     ! grep -qiF "gateway" docs/runbooks/rate-limit.md; then
    err "docs/runbooks/rate-limit.md must document saturation handoff, HTTP principal limit, and gateway quotas"
  fi
fi

if [ -f docs/runbooks/legacy-http-eol.md ]; then
  if ! grep -qF "Deprecation: true" docs/runbooks/legacy-http-eol.md ||
     ! grep -qF "successor-version" docs/runbooks/legacy-http-eol.md ||
     ! grep -qF "MCP_HTTP_LEGACY_POLICY=deny" docs/runbooks/legacy-http-eol.md ||
     ! grep -qF "Sunset" docs/runbooks/legacy-http-eol.md ||
     ! grep -qF "major-version" docs/runbooks/legacy-http-eol.md ||
     ! grep -qF "removal date" docs/runbooks/legacy-http-eol.md; then
    err "docs/runbooks/legacy-http-eol.md must document deprecation headers, deny policy, and release-owned Sunset handling"
  fi
fi

if [ -f docs/runbooks/tenant-offboarding.md ]; then
  if ! grep -qF "MCP_OIDC_VERIFY_CACHE_TTL" docs/runbooks/tenant-offboarding.md ||
     ! grep -qF "60s" docs/runbooks/tenant-offboarding.md ||
     ! grep -qiF "rolling restart" docs/runbooks/tenant-offboarding.md ||
     ! grep -qF "Clockify API key" docs/runbooks/tenant-offboarding.md; then
    err "docs/runbooks/tenant-offboarding.md must document OIDC cache TTL, restart drain, and Clockify credential revocation"
  fi
fi

if [ -f docs/runbooks/postgres-restore.md ]; then
  if ! grep -qF "relrowsecurity" docs/runbooks/postgres-restore.md ||
     ! grep -qF "pg_policies" docs/runbooks/postgres-restore.md ||
     ! grep -qF "paid-hosted RLS launch gate" docs/runbooks/postgres-restore.md; then
    err "docs/runbooks/postgres-restore.md must document RLS-aware restore verification and current RLS gate status"
  fi
fi

required_monitoring_files=(
  "deploy/monitoring/prometheus-alerts.yaml"
  "deploy/k8s/base/prometheus-rule.yaml"
  "deploy/helm/clockify-mcp/templates/prometheusrule.yaml"
  "deploy/monitoring/grafana-mcp-dashboard.json"
)

for file in "${required_monitoring_files[@]}"; do
  if [ ! -f "$file" ]; then
    err "required hosted-launch monitoring artifact missing: $file"
    continue
  fi
  if ! grep -qF "clockify_mcp_audit_failures_total" "$file"; then
    err "$file must include audit-failure monitoring for clockify_mcp_audit_failures_total"
  fi
  if ! grep -qF 'clockify_mcp_audit_failures_total{reason="persist_error",phase="outcome"}' "$file" &&
     ! grep -qF 'clockify_mcp_audit_failures_total{reason=\"persist_error\",phase=\"outcome\"}' "$file"; then
    err "$file must include outcome-phase audit-failure monitoring for fail_closed_strict outcome durability"
  fi
  if ! grep -qF "clockify_mcp_http_admission_rejections_total" "$file"; then
    err "$file must include HTTP admission monitoring for clockify_mcp_http_admission_rejections_total"
  fi
done

dangling=$(grep -rnE --exclude-dir=superpowers '\b(TODO|TBD|FIXME|XXX)\b' "${DOC_DIRS[@]}" "${DOC_FILES_TOP[@]}" 2>/dev/null \
            | grep -v "^docs/adr/.*superseded" \
            | grep -v "check-doc-parity" || true)

if [ -n "$dangling" ]; then
  while IFS= read -r line; do
    err "dangling marker in operator doc: $line"
  done <<< "$dangling"
fi

# ---------------------------------------------------------------------------
# 8. ADR index status parity
# ---------------------------------------------------------------------------

adr_index="docs/adr/README.md"
if [ -f "$adr_index" ]; then
  while IFS= read -r row; do
    adr_id=$(awk -F'|' '{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $2); print $2}' <<< "$row")
    title=$(awk -F'|' '{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $3); print $3}' <<< "$row")
    rel_path=$(sed -nE 's/.*\]\(([^)]+)\).*/\1/p' <<< "$row")
    [ -n "$adr_id" ] || continue
    [ -n "$rel_path" ] || continue
    adr_path="docs/adr/$rel_path"
    if [ ! -f "$adr_path" ]; then
      err "ADR index references missing file for $adr_id: $adr_path"
      continue
    fi
    status_line=$(awk '
      tolower($0) == "## status" {
        while (getline) {
          if ($0 != "") {
            print
            exit
          }
        }
      }
    ' "$adr_path")
    status_norm=$(printf '%s' "$status_line" | tr '[:upper:]' '[:lower:]' | sed -E 's/^[*[:space:]]+//;s/[*.[:space:]]+$//')
    status_kind=""
    case "$status_norm" in
      accepted*) status_kind="accepted" ;;
      proposed*) status_kind="proposed" ;;
    esac
    if [ -z "$status_kind" ]; then
      err "ADR $adr_id has unrecognised Status line in $adr_path: $status_line"
      continue
    fi

    title_norm=$(printf '%s' "$title" | tr '[:upper:]' '[:lower:]')
    case "$status_kind" in
      accepted)
        if grep -Eq '\(proposed\)' <<< "$title_norm"; then
          err "ADR index status drift for $adr_id: file is Accepted but index title says Proposed"
        fi
        proposed_group_hits=$(grep -nE '\b[Aa]re[[:space:]]+(\*\*)?[Pp]roposed(\*\*)?' "$adr_index" | grep -E "\b${adr_id}\b" || true)
        if [ -n "$proposed_group_hits" ]; then
          while IFS= read -r line; do
            err "ADR index status drift for $adr_id: file is Accepted but index summary says Proposed: $line"
          done <<< "$proposed_group_hits"
        fi
        ;;
      proposed)
        if ! grep -Eq '\(proposed\)' <<< "$title_norm"; then
          err "ADR index status drift for $adr_id: file is Proposed but index title lacks '(proposed)'"
        fi
        ;;
    esac
  done < <(grep -E '^\|[[:space:]]*[0-9]{4}[[:space:]]*\|' "$adr_index" || true)
fi

# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------

if [ "$fail" -ne 0 ]; then
  echo >&2
  echo "doc-parity: FAIL — fix the issues above before merging" >&2
  exit 1
fi

echo "doc-parity: OK"
