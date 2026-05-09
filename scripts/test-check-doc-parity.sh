#!/usr/bin/env bash
#
# test-check-doc-parity.sh — regression test for check-doc-parity.sh.
#
# Locks the doc-parity gate's externally-observable contract across
# all eight phases (env-var content, tool-name catalog match, banned
# strings/count drift including OCI metadata, SECURITY.md scope
# coverage plus live auth-model metric labels, public onboarding security/ownership/verification routing,
# README↔npm engines parity, required launch runbooks/monitoring and
# dangling markers, ADR index status parity):
#
#   1.  Pass: clean baseline tree clears all eight phases
#   2.  Phase 1 fail: doc references undefined env var
#   3.  Phase 1 pass: opt-out file allows the otherwise-undefined var
#   4.  Phase 1 pass: inline allowlist (MCP_BEARER_TOKEN_EXAMPLE)
#   5.  Phase 1 pass: docs/superpowers/ excluded from scan
#   6.  Phase 2 fail: doc references unknown tool
#   7.  Phase 2 pass: tool-prefix allowlist (clockify_mcp_*)
#   8.  Phase 2 soft: missing tool-catalog warns, gate still passes
#   9.  Phase 3 fail: banned public-surface string in a doc
#   10. Phase 3 fail: public tool-count drift
#   11. Phase 3 fail: public tool-handler count drift
#   12. Phase 3 fail: OCI image description tool-count drift
#   13. Phase 3 fail: OCI image description policy-count drift
#   14. Phase 4 fail: SECURITY.md scope misses a live surface
#   15. Phase 5 fail: issue-template config missing Security Advisory link
#   16. Phase 5 fail: issue template contains unredacted API-key placeholder
#   17. Phase 5 fail: SUPPORT.md missing security response timing
#   18. Phase 5 fail: CODEOWNERS missing sensitive-path owner
#   19. Phase 5 fail: CONTRIBUTING.md public clone command drift
#   20. Phase 5 fail: CONTRIBUTING.md local verification wording drift
#   21. Phase 5 fail: README local verification wording drift
#   22. Phase 5 fail: Makefile release-check wording drift
#   23. Phase 3 fail: stale release-check shippable wording in docs
#   24. Phase 6 fail: README Node compat row disagrees with package.json
#   25. Phase 6 fail: README missing the Node compat row entirely
#   26. Phase 7 fail: required hosted-launch runbook missing
#   27. Phase 7 fail: shared-service profile gate missing Group 1/6/7 caveat
#   28. Phase 7 fail: production-readiness blocker scope missing repo/legal gates
#   29. Phase 7 fail: agent handoff evidence heading narrows blockers
#   30. Phase 7 fail: required hosted-launch monitoring metric missing
#   31. Phase 7 fail: audit outcome durability alert missing
#   32. Phase 7 fail: dangling marker in operator doc
#   33. Phase 7 pass: marker inside docs/adr/*-superseded.md is filtered
#   34. Phase 6 fail: package.json missing engines.node declaration
#   35. Phase 8 pass: ADR status index matches ADR files
#   36. Phase 8 fail: accepted ADR listed as proposed
#   37. Phase 8 fail: proposed ADR title missing proposed marker
#   38. Phase 4 fail: SECURITY.md audit metric lacks phase label
#   39. Phase 4 fail: SECURITY.md missing cross-origin baseline header docs
#   40. Phase 4 fail: auth-model references removed rate-limit metric
#   41. Phase 3 fail: premature official Clockify launch claim
#   42. Phase 7 fail: brand/legal review artifact missing
#   43. Phase 7 fail: brand/legal dependency/license evidence missing
#   44. Phase 7 fail: ADR 0001 JSON Schema trade-off missing
#   45. Phase 7 fail: brand/legal local license-evidence helper missing
#   46. Phase 7 fail: brand/legal URI and gRPC service-name review missing
#   47. Phase 5 fail: SUPPORT.md stale SLSA mandatory wording
#   48. Phase 3 fail: stale public-content local-artifact wording
#   49. Phase 3 fail: stale shared-service launch-blocking wording
#   50. Phase 7 fail: gRPC reflection dev-only posture missing
#   51. Phase 7 fail: build-tag submodule Dependabot watcher missing
#   52. Phase 7 fail: root Dependabot build-tag ignore missing
#   53. Phase 7 fail: govulncheck version proof missing from CI
#   54. Phase 7 fail: govulncheck tool module missing
#   55. Phase 3 fail: stale unconditional SLSA wording in public docs
#   56. Phase 7 fail: legacy HTTP EOL runbook missing migration headers
#   57. Phase 4 fail: clients doc missing serverInfo identity guidance
#   58. Phase 4 fail: clients doc missing default protocol-version guidance
#   59. Phase 7 fail: AGENTS.md missing May 8 review-ledger read-first routing
#   60. Phase 7 fail: Makefile verify-vuln skips pinned govulncheck module
#   61. Phase 7 fail: gap analysis blocker scope missing repo/legal/product gates
#   62. Phase 7 fail: release-smoke SLSA skip accepts bare HTTP 404
#   63. Phase 3 fail: README unqualified SLSA provenance wording
#   64. Phase 7 fail: workflow action reference is not SHA-pinned
#   65. Phase 7 fail: deploy SLSA skip accepts bare HTTP 404
#   66. Phase 3 fail: release workflow/docs unqualified SLSA chain wording
#   67. Phase 7 fail: agent handoff permissioned landing sequence missing
#   68. Phase 7 fail: dependency-review lacks default-branch evidence trigger
#   69. Phase 7 fail: release-smoke doctor output artifact missing
#   70. Phase 7 fail: docker-image SLSA feature-gate notice missing
#
# Each case builds throwaway fixtures in a per-case tmpdir, runs the
# script with cwd set to the fixture (the script under test uses
# relative paths exclusively), captures combined output and exit
# code, and asserts both. Pure bash; no `go` stub needed because the
# gate itself shells out to nothing.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/scripts/check-doc-parity.sh"

if [ ! -f "$script" ]; then
    echo "FAIL: script not found at $script" >&2
    exit 1
fi

tests_run=0
tests_failed=0

# write_baseline_tree lays down the minimum tree that passes all eight
# phases. Cases mutate this baseline in place to exercise specific
# failure paths.
write_baseline_tree() {
    local dir="$1"

    mkdir -p "$dir/internal/config" "$dir/cmd" "$dir/tests" \
             "$dir/.github/workflows" "$dir/.github/ISSUE_TEMPLATE" "$dir/deploy" \
             "$dir/deploy/monitoring" "$dir/deploy/k8s/base" \
             "$dir/deploy/helm/clockify-mcp/templates" \
             "$dir/docs/runbooks" "$dir/docs/deploy" "$dir/docs/adr" "$dir/docs/release" \
             "$dir/tools/govulncheck" \
             "$dir/npm/clockify-mcp-go"

    cat > "$dir/AGENTS.md" <<'EOF'
# AGENTS

Read first: docs/agent-handoff.md, docs/launch-candidate-checklist.md,
docs/launch-readiness-review-may-8.md. The May 8 review disposition ledger
contains the objective-to-artifact completion audit. Do not mark launch-ready
while that audit says external evidence or approval gates remain open.
EOF

    # Phase 1 known_vars source: stringy literals so the regex
    # '"(MCP|CLOCKIFY)_[A-Z0-9_]+"' matches inside a Go file.
    cat > "$dir/internal/config/config.go" <<'EOF'
package config

const (
    fooEnv = "MCP_FOO"
    barEnv = "CLOCKIFY_BAR"
    apiKeyEnv = "CLOCKIFY_API_KEY"
    httpLegacyPolicyEnv = "MCP_HTTP_LEGACY_POLICY"
    httpPrincipalLimitEnv = "MCP_HTTP_RATELIMIT_PER_PRINCIPAL"
    oidcVerifyCacheTTLEnv = "MCP_OIDC_VERIFY_CACHE_TTL"
    perTokenLimitEnv = "CLOCKIFY_PER_TOKEN_RATE_LIMIT"
    defaultProtocolVersionEnv = "MCP_DEFAULT_PROTOCOL_VERSION"
)
EOF

    # CI-only env var that the script picks up via the .github/ scan.
    cat > "$dir/.github/workflows/live.yml" <<'EOF'
jobs:
  live:
    env:
      CLOCKIFY_LIVE_TOKEN: dummy
EOF

    cat > "$dir/.github/workflows/docker-image.yml" <<'EOF'
# Push/tag builds receive SLSA provenance when GitHub artifact attestations are available.
# The SLSA step is best-effort for the current user-owned private repository.
# image build, scan, cosign signature, and SBOM gates remain mandatory.
jobs:
  image:
    steps:
      - name: Attest SLSA build provenance
        id: slsa_provenance
        continue-on-error: true
        uses: actions/attest-build-provenance@a2bbfa25375fe432b6a289bc6b6cd05ecd0c4c32
      - name: Report SLSA attestation feature-gate skip
        if: github.event_name != 'pull_request' && steps.slsa_provenance.outcome == 'failure'
        run: |
          echo "::notice title=SLSA attestation skipped::GitHub artifact attestations are unavailable for this user-owned private repository per ADR-0013; mandatory image build, Trivy, cosign image signature, and SBOM attestation gates already passed."
labels:
  org.opencontainers.image.description=Go MCP server for Clockify: 1 tools, three transports, five policy modes
EOF

    cat > "$dir/.github/workflows/release.yml" <<'EOF'
jobs:
  release:
    steps:
      - name: Stage release artifacts
        run: |
          echo "mandatory cosign/SBOM chain, plus SLSA provenance when GitHub artifact attestations are available"
EOF

    cat > "$dir/.github/workflows/ci.yml" <<'EOF'
jobs:
  vulncheck:
    steps:
      - name: Run govulncheck
        run: |
          (cd tools/govulncheck && GOWORK=off go install golang.org/x/vuln/cmd/govulncheck)
          govulncheck -version
          govulncheck ./...
EOF

    cat > "$dir/.github/workflows/dependency-review.yml" <<'EOF'
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
jobs:
  dependency-review:
    steps:
      - uses: actions/dependency-review-action@a1d282b36b6f3519aa1f3fc636f609c47dddb294
        with:
          vulnerability-check: true
          license-check: false
          fail-on-severity: high
          base-ref: ${{ github.event_name == 'push' && github.event.before || '' }}
          head-ref: ${{ github.event_name == 'push' && github.sha || '' }}
EOF

    cat > "$dir/.github/workflows/release-smoke.yml" <<'EOF'
jobs:
  smoke:
    steps:
      - name: Verify SLSA build provenance attestation
        run: |
          if ! gh attestation verify artifacts/clockify-mcp-linux-x64 \
            --owner "${GITHUB_REPOSITORY_OWNER}" \
            >/tmp/slsa-default-ok.txt 2>/tmp/slsa-default.err; then
            cat /tmp/slsa-default.err >&2
            # Do not skip on a bare 404: that can also mean the
            # expected attestation is missing. Only the known
            # user-owned-private repository feature gate is non-fatal.
            if grep -qiF 'Feature not available' /tmp/slsa-default.err &&
               grep -qiF 'user-owned private repositor' /tmp/slsa-default.err; then
              echo "::notice title=SLSA attestation skipped::GitHub artifact attestations are unavailable for this user-owned private repository; cosign binary/image checks remain mandatory."
              exit 0
            fi
            exit 1
          fi
      - name: Validate doctor strict evidence files
        if: always()
        run: |
          for path in \
            release-smoke-evidence/release-doctor-strict-ok.txt \
            release-smoke-evidence/release-doctor-strict-fail.txt \
            release-smoke-evidence/release-doctor-postgres-ok.txt
          do
            if [ ! -s "$path" ]; then
              echo "::error title=Missing doctor strict evidence::$path is missing or empty"
              exit 1
            fi
          done
      - name: Upload doctor strict evidence
        if: always()
        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a
        with:
          name: release-smoke-doctor-output
          path: release-smoke-evidence/*.txt
          if-no-files-found: error
          retention-days: 30
EOF

    cat > "$dir/.github/workflows/deploy.yml" <<'EOF'
name: Deploy

# Tag-driven release-deploy pipeline.
#
# Stages:
#   verify           - verify the published image's cosign signature and the
#                      release binaries' SLSA build provenance when GitHub
#                      artifact attestations are available. The SLSA check
#                      note-skips only GitHub's user-owned-private repository
#                      feature gate; the cosign image check always remains
#                      mandatory.

jobs:
  verify:
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
      - uses: sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6 # v4.1.2
      - name: Verify SLSA build provenance
        run: |
          if ! gh attestation verify clockify-mcp-linux-x64 \
            --owner "$OWNER" \
            >/tmp/deploy-slsa-ok.txt 2>/tmp/deploy-slsa.err; then
            cat /tmp/deploy-slsa.err >&2
            # Do not skip on a bare 404: that can also mean the
            # expected attestation is missing. Only the known
            # user-owned-private repository feature gate is non-fatal.
            if grep -qiF 'Feature not available' /tmp/deploy-slsa.err &&
               grep -qiF 'user-owned private repositor' /tmp/deploy-slsa.err; then
              echo "::notice title=SLSA attestation skipped::GitHub artifact attestations are unavailable for this user-owned private repository; cosign image verification remains mandatory."
              exit 0
            fi
            exit 1
          fi
EOF

    cat > "$dir/.github/dependabot.yml" <<'EOF'
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    ignore:
      - dependency-name: "github.com/jackc/pgx/v5"
      - dependency-name: "github.com/testcontainers/testcontainers-go*"
      - dependency-name: "google.golang.org/grpc"
      - dependency-name: "go.opentelemetry.io/*"
  - package-ecosystem: gomod
    directory: /internal/controlplane/postgres
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
  - package-ecosystem: gomod
    directory: /internal/transport/grpc
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
  - package-ecosystem: gomod
    directory: /internal/tracing/otel
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
  - package-ecosystem: gomod
    directory: /tools/govulncheck
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
EOF

    cat > "$dir/tools/govulncheck/go.mod" <<'EOF'
module github.com/apet97/go-clockify/tools/govulncheck

go 1.25.10

require golang.org/x/vuln v1.3.0
EOF

    cat > "$dir/tools/govulncheck/tools.go" <<'EOF'
//go:build tools

package tools

import _ "golang.org/x/vuln/cmd/govulncheck"
EOF

    cat > "$dir/Makefile" <<'EOF'
release-check:
	@echo "release-check: OK — local pre-ship gate passed"

verify-vuln:
	@go_pin="$$(awk '$$1 == "go" { print $$2; exit }' go.mod)"; \
	 if [ -z "$$go_pin" ]; then \
		echo "[verify-vuln] unable to read Go version from go.mod"; \
		exit 1; \
	 fi; \
	 if [ ! -f tools/govulncheck/go.mod ]; then \
		echo "[verify-vuln] tools/govulncheck/go.mod missing; cannot run pinned scanner"; \
		exit 1; \
	 fi; \
	 echo "== govulncheck (tools/govulncheck, GOTOOLCHAIN=go$$go_pin) =="; \
	 tmpdir="$$(mktemp -d)"; \
	 trap 'rm -rf "$$tmpdir"' EXIT; \
	 (cd tools/govulncheck && GOWORK=off GOBIN="$$tmpdir" GOTOOLCHAIN="go$$go_pin" go install golang.org/x/vuln/cmd/govulncheck); \
	 GOTOOLCHAIN="go$$go_pin" "$$tmpdir/govulncheck" -version; \
	 GOTOOLCHAIN="go$$go_pin" "$$tmpdir/govulncheck" ./...

# launch readiness still depends on the external evidence gates.
EOF

    cat > "$dir/.github/ISSUE_TEMPLATE/config.yml" <<'EOF'
blank_issues_enabled: false
contact_links:
  - name: Security vulnerability
    url: https://github.com/apet97/go-clockify/security/advisories/new
    about: Do not open a public issue for vulnerabilities or suspected secrets.
EOF

    cat > "$dir/.github/ISSUE_TEMPLATE/bug_report.yml" <<'EOF'
name: Bug Report
body:
  - type: markdown
    attributes:
      value: Do not use this public form for security vulnerabilities. Use the private GitHub Security Advisory. Redact API keys, tokens, secrets, and personal data.
  - type: textarea
    id: reproduce
    attributes:
      label: Steps
      placeholder: CLOCKIFY_API_KEY=<redacted>
EOF

    cat > "$dir/.github/ISSUE_TEMPLATE/feature_request.yml" <<'EOF'
name: Feature Request
body:
  - type: markdown
    attributes:
      value: Do not include secrets, API keys, tokens, personal data, or vulnerability details in public issues; use the private GitHub Security Advisory for security reports.
EOF

    cat > "$dir/.github/CODEOWNERS" <<'EOF'
*                            @apet97
/internal/authn/             @apet97
/internal/enforcement/       @apet97
/internal/policy/            @apet97
/internal/transport/         @apet97
/internal/clockify/          @apet97
/.github/                    @apet97
/.goreleaser.yaml            @apet97
/deploy/                     @apet97
/SECURITY.md                 @apet97
/SUPPORT.md                  @apet97
EOF

    cat > "$dir/deploy/Dockerfile" <<'EOF'
LABEL org.opencontainers.image.description="Go MCP server for Clockify: 1 tools, three transports, five policy modes"
EOF

    for monitoring_file in \
        "$dir/deploy/monitoring/prometheus-alerts.yaml" \
        "$dir/deploy/k8s/base/prometheus-rule.yaml" \
        "$dir/deploy/helm/clockify-mcp/templates/prometheusrule.yaml" \
        "$dir/deploy/monitoring/grafana-mcp-dashboard.json"; do
        cat > "$monitoring_file" <<'EOF'
clockify_mcp_audit_failures_total
clockify_mcp_audit_failures_total{reason="persist_error",phase="outcome"}
clockify_mcp_http_admission_rejections_total
EOF
    done

    # Empty opt-out: header only.
    cat > "$dir/deploy/.config-parity-opt-out.txt" <<'EOF'
# Test fixture opt-out
EOF

    # Phase 2 known_tools: catalog with one tool entry that matches
    # the script's '"name": *"clockify_[a-z0-9_]+"' regex.
    cat > "$dir/docs/tool-catalog.json" <<'EOF'
{
  "tier1": [
    {"name": "clockify_list_workspaces"}
  ]
}
EOF

    # Phase 4 inputs: package.json with engines.node + README compat row.
    cat > "$dir/npm/clockify-mcp-go/package.json" <<'EOF'
{
  "name": "clockify-mcp-go",
  "engines": {
    "node": ">=18"
  }
}
EOF

    cat > "$dir/README.md" <<'EOF'
# Test fixture README

Configures MCP_FOO at startup; surfaces clockify_list_workspaces.

## Build and test

make verify  # pre-PR local pipeline
make release-check  # pre-ship/tag gate

CI remains authoritative for skipped local tiers.
Neither command replaces the external launch-candidate evidence gates.

## Highlights

Signed releases ship with cosign signatures and SPDX SBOMs. SLSA build provenance is attached when GitHub artifact attestations are available.

## Compatibility

| Component | Version |
|---|---|
| Node.js (npm wrapper) | 18+ |
EOF

    cat > "$dir/docs/verify-release.md" <<'EOF'
# Verify release

Each binary always ships with a raw binary, keyless cosign sigstore bundle,
and SPDX SBOM. When GitHub artifact attestations are available for the repository
account tier, the release workflow also stores SLSA build provenance in the
GitHub attestation service. On the current user-owned private repository,
the mandatory cryptographic gate is the cosign binary/image signature chain.
EOF

    cat > "$dir/SECURITY.md" <<'EOF'
# Security Policy

## Reporting a Vulnerability

Use the private GitHub Security Advisory workflow at
https://github.com/apet97/go-clockify/security/advisories/new.

## Response Timeline

- **Acknowledgment:** Within 48 hours
- **Initial assessment:** Within 1 week

## Scope

The following are in scope:
- API key exposure or leakage
- Authentication bypass in HTTP transport
- OIDC/JWKS validation weaknesses
- Tenant-isolation failures
- Audit-durability failures
- Path traversal in ID validation
- CORS bypass in HTTP transport
- DNS rebinding or private-network exposure
- Timing attacks on bearer token comparison

## Security Features

- AuthN and transport hardening.
- Transport hardening includes `Strict-Transport-Security` when TLS or a trusted HTTPS proxy is active, `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Embedder-Policy: require-corp`, and `Cross-Origin-Resource-Policy: same-origin`.
- Audit failures use `clockify_mcp_audit_failures_total{reason="persist_error",phase="outcome"}`.

GitHub artifact attestations may return "Feature not available" for this
user-owned private repository. ADR-0013 keeps SLSA best-effort and the
mandatory cryptographic gate is the cosign binary/image chain.
EOF

    cat > "$dir/docs/auth-model.md" <<'EOF'
# Auth model

Per-subject rate limiting uses CLOCKIFY_PER_TOKEN_RATE_LIMIT and surfaces
as `clockify_mcp_rate_limit_rejections_total{kind="window",scope="per_token"}`.
EOF

    cat > "$dir/docs/clients.md" <<'EOF'
# Clients

The protocol initialize response uses serverInfo.name: "clockify-go-mcp".
The clockify-mcp binary and @apet97/clockify-mcp-go wrapper are packaging names.
When a client omits params.protocolVersion, MCP_DEFAULT_PROTOCOL_VERSION can
pin the fallback instead of the newest supported version. Explicit supported
client versions are still echoed.
EOF

    cat > "$dir/SUPPORT.md" <<'EOF'
# Support

Private security vulnerability reports go to
https://github.com/apet97/go-clockify/security/advisories/new.

Security disclosures are acknowledged within 48 hours.

GitHub artifact attestations may return "Feature not available" for this
user-owned private repository. ADR-0013 keeps SLSA best-effort and the
mandatory cryptographic gate is the cosign binary/image chain.
EOF

    cat > "$dir/CONTRIBUTING.md" <<'EOF'
# Contributing

```sh
git clone https://github.com/apet97/go-clockify.git
cd go-clockify
```

When the repository is public again, the same clone command works without additional auth.

Tool-gated local tiers print a skip warning when the binary is missing; CI remains authoritative.
Run make release-check before launch-candidate handoff.
| `lint` | golangci-lint | **skips** with a warning if not installed; CI enforces |
| `verify-vuln` | tools/govulncheck | always installs and runs the pinned `govulncheck` module with the repo Go pin |
EOF

    # DOC_FILES_TOP entry inside the strict scan: must contain no
    # markers, banned strings, unknown tools, or undefined env vars.
    cat > "$dir/docs/support-matrix.md" <<'EOF'
# Support matrix

See README for compatibility details.
EOF

    cat > "$dir/docs/production-readiness.md" <<'EOF'
# Production readiness

The remaining blockers are not local test failures. They are still launch
blockers: Group 6 candidate-tag security evidence, Group 7
release/sigstore/SLSA evidence, pushed workflow evidence where still
missing, repository-state cleanup, public-readiness disposition, and
legal/product approval for any official-product claim. Group 1 scheduled
final-SHA evidence is closed. Local checks are useful but not sufficient
for an official/product launch-ready claim.
EOF

    cat > "$dir/docs/official-clockify-mcp-gap-analysis.md" <<'EOF'
# Official Clockify MCP gap analysis

The remaining blockers are not local test failures. They are still launch
blockers: Group 6 candidate-tag security walk-through evidence, Group 7
release/sigstore/SLSA evidence, pushed workflow first-run evidence where still
missing, repository-state cleanup, public-readiness disposition,
hosted/platform evidence, and legal/product approval for any
Clockify-supported product launch claim. Local checks are useful but not sufficient
for a Clockify-supported product launch claim. Group 1 scheduled
live-contract cron greens are now archived on
feef83c641ced93d2ab6ba07ef766d61c82cc703.
EOF

    cat > "$dir/docs/agent-handoff.md" <<'EOF'
# Agent handoff

- **Open launch-evidence gates (not a complete blocker list):**

## Suggested continuation order

1. **Land the remediation tree only after explicit approval.** Before
   staging, rerun `git status --short --branch`, `make doc-parity`,
   `git diff --check`, and any narrow tests touched by last-minute
   edits. Do not use `git add .` from a parent workspace. Commit from
   this repo root only, preserve generated-file lockstep, and end the
   commit body with `Why:` and `Verified:` lines. Do not push until the
   whole staged group is locally green.
2. **Refresh external status on the landed SHA.** After the commit is
   pushed, run `make launch-external-status` and then
   `bash scripts/check-launch-external-status.sh --fail-open`.
EOF

    cat > "$dir/docs/runbooks/rate-limit.md" <<'EOF'
# Rate-limit operations

See rate-limit-saturation.md. Configure MCP_HTTP_RATELIMIT_PER_PRINCIPAL
and gateway quotas together.
EOF

    cat > "$dir/docs/runbooks/rate-limit-saturation.md" <<'EOF'
# Rate-limit saturation

Operational triage.
EOF

    cat > "$dir/docs/runbooks/audit-durability.md" <<'EOF'
# Audit durability

Operational triage.
EOF

    cat > "$dir/docs/runbooks/legacy-http-eol.md" <<'EOF'
# Legacy HTTP EOL

Legacy responses carry Deprecation: true and a successor-version Link.
Set MCP_HTTP_LEGACY_POLICY=deny in production. Sunset handling requires
a major-version removal date.
EOF

    cat > "$dir/docs/runbooks/postgres-restore.md" <<'EOF'
# Postgres restore

Check relrowsecurity, pg_policies, and the paid-hosted RLS launch gate.
EOF

    cat > "$dir/docs/runbooks/tenant-offboarding.md" <<'EOF'
# Tenant offboarding

MCP_OIDC_VERIFY_CACHE_TTL is clamped to 60s. Use a rolling restart to
drain caches and revoke the tenant Clockify API key or credential.
EOF

    cat > "$dir/docs/release/brand-legal-review.md" <<'EOF'
# Brand and Legal Review Questions

Status: NEEDS LEGAL REVIEW.

L-10 requires approval before public copy claims official-product status.
If approval is not granted, record the rebrand decision.

Brand identifiers under review: clockify:// resource URI scheme and
clockify.mcp.v1.MCP gRPC service name. Review
internal/transport/grpc/service.go before clients depend on these names.

Dependency evidence: attach go-licenses output for
internal/controlplane/postgres, internal/transport/grpc, and
internal/tracing/otel. Run make license-evidence, backed by
scripts/collect-license-evidence.sh and go list -deps, as not legal advice
and not license clearance. Review npm/package.json.tmpl and
.github/workflows/dependency-review.yml because license-check: false.
EOF

    cat > "$dir/docs/deploy/production-profile-shared-service.md" <<'EOF'
# Production Profile: Shared Service

Two artifacts gate the shared-service profile.

A green run of both artifacts closes the shared-service profile gate
only (Group 2 of docs/launch-candidate-checklist.md). It does not make
the repository launch-ready or replace the remaining Group 1, Group 6,
or Group 7 external evidence gates.
EOF

    cat > "$dir/docs/deploy/profile-private-network-grpc.md" <<'EOF'
# Private Network gRPC Profile

gRPC reflection is intentionally not registered in supported release
artifacts. Keep it off in production; exposing reflection would reveal
service and method names to any authenticated client on the private
network. For local protocol exploration only, build a throwaway binary
with `-tags=grpc,grpcreflection`; do not deploy that tag outside
development.
EOF

    cat > "$dir/docs/adr/0001-stdlib-only-default-build.md" <<'EOF'
# 0001 - Stdlib-only default build

## Status

Accepted.

Runtime tool-argument validation uses internal/jsonschema as a
Draft 2020-12 subset and is guarded by
internal/tools/schema_keyword_test.go.

Rejected: github.com/santhosh-tekuri/jsonschema/v6, because the current
catalog does not need $ref, $defs, conditionals, or a broader default SBOM.
EOF
}

# run_case <name> <expect-exit> <expect-pattern> [extra-env-pair ...]
#
# expect-pattern is an ERE applied with grep -qE against combined
# stdout+stderr; pass an empty string to skip the assertion.
# The optional MUTATOR function (named via $MUTATOR) runs against
# the per-case fixture directory before invoking the script.
run_case() {
    local name="$1"; shift
    local expect_exit="$1"; shift
    local expect_pattern="$1"; shift

    tests_run=$((tests_run + 1))

    local dir
    dir="$(mktemp -d "${TMPDIR:-/tmp}/test-doc-parity.XXXXXX")"
    # shellcheck disable=SC2064
    trap "rm -rf \"$dir\"" RETURN

    write_baseline_tree "$dir"

    if [ -n "${MUTATOR:-}" ]; then
        "$MUTATOR" "$dir"
    fi

    local out
    local actual_exit=0
    out="$(cd "$dir" && env "$@" bash "$script" 2>&1)" || actual_exit=$?

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
}

# --- Case 1: clean baseline ---
MUTATOR=""
run_case "clean baseline clears all eight phases" \
    0 'doc-parity: OK'

# --- Case 2: undefined env var ref in docs ---
mut_undefined_env() {
    printf '\nFurther reading: see MCP_GHOST notes.\n' >> "$1/README.md"
}
MUTATOR=mut_undefined_env
run_case "Phase 1: undefined env var reference fails closed" \
    1 'env var referenced in docs but not defined'

# --- Case 3: opt-out file allows MCP_GHOST ---
mut_opt_out_ghost() {
    printf '\nFurther reading: see MCP_GHOST notes.\n' >> "$1/README.md"
    printf 'MCP_GHOST   placeholder for fixture\n' \
        >> "$1/deploy/.config-parity-opt-out.txt"
}
MUTATOR=mut_opt_out_ghost
run_case "Phase 1: opt-out file allows otherwise-undefined var" \
    0 'doc-parity: OK'

# --- Case 4: inline allowlist (MCP_BEARER_TOKEN_EXAMPLE) ---
mut_bearer_example() {
    printf 'Example: MCP_BEARER_TOKEN_EXAMPLE=changeme\n' \
        > "$1/docs/runbooks/x.md"
}
MUTATOR=mut_bearer_example
run_case "Phase 1: inline example allowlist is honoured" \
    0 'doc-parity: OK'

# --- Case 5: docs/superpowers/ excluded ---
mut_superpowers_ghost() {
    mkdir -p "$1/docs/superpowers"
    printf '# Future spec\n\nIntroduces MCP_GHOST in a later release.\n' \
        > "$1/docs/superpowers/spec.md"
}
MUTATOR=mut_superpowers_ghost
run_case "Phase 1: docs/superpowers/ is excluded from scan" \
    0 'doc-parity: OK'

# --- Case 6: unknown clockify_* tool token ---
mut_unknown_tool() {
    printf '\nNote: clockify_ghost_tool was renamed last month.\n' \
        >> "$1/README.md"
}
MUTATOR=mut_unknown_tool
run_case "Phase 2: unknown tool token fails closed" \
    1 'tool referenced in operator docs but not in'

# --- Case 7: tool-prefix allowlist (clockify_mcp_*) ---
mut_tool_allowlist() {
    printf '\nInternal helper: clockify_mcp_internal handles bootstrap.\n' \
        >> "$1/README.md"
}
MUTATOR=mut_tool_allowlist
run_case "Phase 2: tool-prefix allowlist is honoured" \
    0 'doc-parity: OK'

# --- Case 8: missing tool-catalog warns, gate passes ---
mut_remove_catalog() {
    rm "$1/docs/tool-catalog.json"
}
MUTATOR=mut_remove_catalog
run_case "Phase 2: missing tool-catalog warns, gate still passes" \
    0 '\[warn\].*tool-catalog\.json'

# --- Case 9: banned public-surface string ---
# `@anycli/clockify-mcp-go` deliberately uses a hyphen, not an
# underscore, so it does NOT also trip Phase 2's `\bclockify_…` regex.
mut_banned_string() {
    printf 'Legacy hint: see @anycli/clockify-mcp-go in older releases.\n' \
        > "$1/docs/runbooks/x.md"
}
MUTATOR=mut_banned_string
run_case "Phase 3: banned stale public-surface string fails closed" \
    1 'banned stale public-surface string'

# --- Case 10: stale current tool-count claim ---
mut_stale_tool_count() {
    printf '\nLaunch surface: 124 tools are available today.\n' \
        >> "$1/README.md"
}
MUTATOR=mut_stale_tool_count
run_case "Phase 3: public tool-count drift fails closed" \
    1 'public-surface tool-count drift'

# --- Case 11: stale current tool-handler count ---
mut_stale_tool_handlers() {
    cat > "$1/CONTRIBUTING.md" <<'EOF'
# Contributing

All 124 tool handlers live under internal/tools.
EOF
}
MUTATOR=mut_stale_tool_handlers
run_case "Phase 3: public tool-handler count drift fails closed" \
    1 'public-surface tool-handler count drift'

# --- Case 12: OCI image description stale tool count ---
mut_oci_tool_count_drift() {
    cat > "$1/.github/workflows/docker-image.yml" <<'EOF'
labels:
  org.opencontainers.image.description=Go MCP server for Clockify: 124 tools, three transports, five policy modes
EOF
}
MUTATOR=mut_oci_tool_count_drift
run_case "Phase 3: OCI image description tool-count drift fails closed" \
    1 'OCI image description tool-count drift'

# --- Case 13: OCI image description stale policy count ---
mut_oci_policy_count_drift() {
    cat > "$1/deploy/Dockerfile" <<'EOF'
LABEL org.opencontainers.image.description="Go MCP server for Clockify: 1 tools, three transports, four policy modes"
EOF
}
MUTATOR=mut_oci_policy_count_drift
run_case "Phase 3: OCI image description policy-count drift fails closed" \
    1 'OCI image description policy-count drift'

# --- Case 14: SECURITY.md scope missing a live surface ---
mut_security_scope_missing_surface() {
    grep -vF 'Tenant-isolation failures' "$1/SECURITY.md" > "$1/SECURITY.md.new"
    mv "$1/SECURITY.md.new" "$1/SECURITY.md"
}
MUTATOR=mut_security_scope_missing_surface
run_case "Phase 4: SECURITY.md scope missing live surface fails closed" \
    1 'SECURITY\.md scope missing live security surface'

# --- Case 15: issue-template config missing Security Advisory link ---
mut_issue_config_missing_advisory() {
    grep -vF 'security/advisories/new' "$1/.github/ISSUE_TEMPLATE/config.yml" \
        > "$1/.github/ISSUE_TEMPLATE/config.yml.new"
    mv "$1/.github/ISSUE_TEMPLATE/config.yml.new" "$1/.github/ISSUE_TEMPLATE/config.yml"
}
MUTATOR=mut_issue_config_missing_advisory
run_case "Phase 5: issue-template config missing Security Advisory link fails closed" \
    1 'must link to the private Security Advisory flow'

# --- Case 16: issue template contains unredacted API key placeholder ---
mut_issue_template_unredacted_key() {
    printf '\n        1. Set CLOCKIFY_API_KEY=...\n' \
        >> "$1/.github/ISSUE_TEMPLATE/bug_report.yml"
}
MUTATOR=mut_issue_template_unredacted_key
run_case "Phase 5: issue template unredacted API key placeholder fails closed" \
    1 'issue template contains unredacted secret placeholder'

# --- Case 17: SUPPORT.md missing security response timing ---
mut_support_missing_security_timing() {
    grep -vF 'Security disclosures are acknowledged within 48 hours.' \
        "$1/SUPPORT.md" > "$1/SUPPORT.md.new"
    mv "$1/SUPPORT.md.new" "$1/SUPPORT.md"
}
MUTATOR=mut_support_missing_security_timing
run_case "Phase 5: SUPPORT.md missing security response timing fails closed" \
    1 'SUPPORT\.md must state security disclosure acknowledgement timing'

# --- Case 18: CODEOWNERS missing sensitive-path owner ---
mut_codeowners_missing_sensitive_path() {
    grep -vF '/internal/authn/' "$1/.github/CODEOWNERS" > "$1/.github/CODEOWNERS.new"
    mv "$1/.github/CODEOWNERS.new" "$1/.github/CODEOWNERS"
}
MUTATOR=mut_codeowners_missing_sensitive_path
run_case "Phase 5: CODEOWNERS missing sensitive-path owner fails closed" \
    1 'CODEOWNERS missing public-sensitive ownership pattern'

# --- Case 19: CONTRIBUTING.md public clone command drift ---
mut_contributing_private_clone() {
    perl -0pi -e 's#git clone https://github.com/apet97/go-clockify\.git#git clone git@github.com:apet97/go-clockify.git#' \
        "$1/CONTRIBUTING.md"
}
MUTATOR=mut_contributing_private_clone
run_case "Phase 5: CONTRIBUTING.md public clone command drift fails closed" \
    1 'CONTRIBUTING\.md must use the public-compatible HTTPS clone command'

# --- Case 20: CONTRIBUTING.md local verification wording drift ---
mut_contributing_verify_semantics() {
    perl -0pi -e 's/\| `lint` \| golangci-lint \| \*\*skips\*\* with a warning if not installed; CI enforces \|/\| `lint` \| golangci-lint \| always runs \|/; s/Tool-gated local tiers print a skip warning when the binary is missing; CI remains authoritative\.\n//' \
        "$1/CONTRIBUTING.md"
}
MUTATOR=mut_contributing_verify_semantics
run_case "Phase 5: CONTRIBUTING.md local verification wording drift fails closed" \
    1 'CONTRIBUTING\.md must state local golangci-lint skips when absent'

# --- Case 21: README local verification wording drift ---
mut_readme_verify_semantics() {
    perl -0pi -e 's/make verify  # pre-PR local pipeline/make verify  # full local pipeline/; s/make release-check  # pre-ship\/tag gate\n//; s/CI remains authoritative for skipped local tiers\.\n//; s/Neither command replaces the external launch-candidate evidence gates\.\n//' \
        "$1/README.md"
}
MUTATOR=mut_readme_verify_semantics
run_case "Phase 5: README local verification wording drift fails closed" \
    1 'README\.md must distinguish pre-PR local verification'

# --- Case 22: Makefile release-check wording drift ---
mut_makefile_release_check_wording() {
    cat > "$1/Makefile" <<'EOF'
release-check:
	@echo "release-check: OK — shippable"

# "release-check: OK — shippable" line is the one-word answer to
# "is this repo shippable right now?".
EOF
}
MUTATOR=mut_makefile_release_check_wording
run_case "Phase 5: Makefile release-check wording drift fails closed" \
    1 'Makefile release-check output must not imply local checks alone prove shippability'

# --- Case 23: stale release-check shippable wording in docs ---
mut_doc_release_check_shippable_wording() {
    printf '\n- `make release-check` passed with `release-check: OK - shippable`.\n' \
        >> "$1/docs/README.md"
}
MUTATOR=mut_doc_release_check_shippable_wording
run_case "Phase 3: stale release-check shippable wording in docs fails closed" \
    1 'banned stale public-surface string'

# --- Case 24: README ↔ package.json Node mismatch ---
mut_node_mismatch() {
    cat > "$1/npm/clockify-mcp-go/package.json" <<'EOF'
{
  "name": "clockify-mcp-go",
  "engines": {
    "node": ">=20"
  }
}
EOF
}
MUTATOR=mut_node_mismatch
run_case "Phase 6: README Node compat does not match package.json fails closed" \
    1 'does not match'

# --- Case 25: README missing Node compat row ---
# Phase 6's `readme_node=$(grep | sed | tr)` pipeline is followed by
# `|| true` so grep returning 1 (row absent) does not abort the
# substitution under `set -euo pipefail` — the dedicated
# `[ -z "$readme_node" ]` err line below it must remain reachable.
mut_missing_node_row() {
    grep -vF '| Node.js (npm wrapper) |' "$1/README.md" > "$1/README.md.new"
    mv "$1/README.md.new" "$1/README.md"
}
MUTATOR=mut_missing_node_row
run_case "Phase 6: README missing Node compat row fails closed" \
    1 'missing Node\.js .* compatibility row'

# --- Case 26: required hosted-launch runbook missing ---
mut_missing_launch_runbook() {
    rm "$1/docs/runbooks/tenant-offboarding.md"
}
MUTATOR=mut_missing_launch_runbook
run_case "Phase 7: required hosted-launch runbook missing fails closed" \
    1 'required launch runbook missing'

# --- Case 27: shared-service profile gate missing Group 1/6/7 caveat ---
mut_shared_service_profile_gate_caveat_missing() {
    perl -0pi -e 's/A green run of both artifacts closes the shared-service profile gate.*?Group 7 external evidence gates\./A green run of both artifacts is the launch-candidate gate for this profile./s' \
        "$1/docs/deploy/production-profile-shared-service.md"
}
MUTATOR=mut_shared_service_profile_gate_caveat_missing
run_case "Phase 7: shared-service profile gate missing Group 1/6/7 caveat fails closed" \
    1 'shared-service\.md must scope its green artifacts to Group 2'

# --- Case 28: production-readiness blocker scope missing repo/legal gates ---
mut_production_readiness_gate_scope_missing() {
    perl -0pi -e 's/The remaining blockers are not local test failures\. They are still launch\s+blockers: Group 6 candidate-tag security evidence, Group 7\s+release\/sigstore\/SLSA evidence, pushed workflow evidence where still\s+missing, repository-state cleanup, public-readiness disposition, and\s+legal\/product approval for any official-product claim\. Group 1 scheduled\s+final-SHA evidence is closed\. Local checks are useful but not sufficient\s+for an official\/product launch-ready claim\./The remaining blockers are external evidence only: scheduled live-contract, candidate-tag security, and release evidence./s' \
        "$1/docs/production-readiness.md"
}
MUTATOR=mut_production_readiness_gate_scope_missing
run_case "Phase 7: production-readiness blocker scope missing repo/legal gates fails closed" \
    1 'production-readiness\.md must not narrow remaining launch blockers'

# --- Case 29: agent handoff evidence heading narrows blockers ---
mut_agent_handoff_evidence_heading_narrows_blockers() {
    perl -0pi -e 's/Open launch-evidence gates \(not a complete blocker list\)/Open external evidence only/' \
        "$1/docs/agent-handoff.md"
}
MUTATOR=mut_agent_handoff_evidence_heading_narrows_blockers
run_case "Phase 7: agent handoff evidence heading narrows blockers fails closed" \
    1 'agent-handoff\.md must not label Group 1/6/7 as the only open evidence'

# --- Case 30: required hosted-launch monitoring metric missing ---
mut_missing_launch_monitoring_metric() {
    perl -0pi -e 's/clockify_mcp_http_admission_rejections_total\n//' \
        "$1/deploy/monitoring/grafana-mcp-dashboard.json"
}
MUTATOR=mut_missing_launch_monitoring_metric
run_case "Phase 7: required hosted-launch monitoring metric missing fails closed" \
    1 'must include HTTP admission monitoring'

# --- Case 31: audit outcome durability alert missing ---
mut_missing_audit_outcome_alert() {
    perl -0pi -e 's/clockify_mcp_audit_failures_total\{reason="persist_error",phase="outcome"\}\n//' \
        "$1/deploy/monitoring/prometheus-alerts.yaml"
}
MUTATOR=mut_missing_audit_outcome_alert
run_case "Phase 7: audit outcome durability alert missing fails closed" \
    1 'must include outcome-phase audit-failure monitoring'

# --- Case 32: dangling marker in operator runbook ---
mut_dangling_marker() {
    printf '# Runbook\n\n- TODO follow up before release\n' \
        > "$1/docs/runbooks/x.md"
}
MUTATOR=mut_dangling_marker
run_case "Phase 7: dangling marker in operator doc fails closed" \
    1 'dangling marker in operator doc'

# --- Case 33: marker inside docs/adr/*-superseded.md is filtered ---
mut_superseded_marker() {
    printf '# Superseded ADR\n\n- TODO drop this once renamed\n' \
        > "$1/docs/adr/0001-x-superseded.md"
}
MUTATOR=mut_superseded_marker
run_case "Phase 7: marker in docs/adr/*-superseded.md is filtered" \
    0 'doc-parity: OK'

# --- Case 34: package.json missing engines.node declaration ---
# Parallel to case 11 for the package_node branch. Lives under the
# same `|| true` contract: without the trailing `|| true` on Phase
# 6's package_node pipeline, this branch would silently abort.
mut_missing_engines_node() {
    cat > "$1/npm/clockify-mcp-go/package.json" <<'EOF'
{
  "name": "clockify-mcp-go"
}
EOF
}
MUTATOR=mut_missing_engines_node
run_case "Phase 6: package.json missing engines.node fails closed" \
    1 'missing node engine declaration'

# --- Case 35: ADR status index clean mapping ---
write_adr_status_fixture() {
    local dir="$1"
    local accepted_title="$2"
    local proposed_title="$3"
    local summary="$4"

    cat > "$dir/docs/adr/README.md" <<EOF
# Architecture Decision Records

| ADR | Title | File |
|-----|-------|------|
| 0001 | $accepted_title | [0001-accepted.md](0001-accepted.md) |
| 0002 | $proposed_title | [0002-proposed.md](0002-proposed.md) |

$summary
EOF

    cat > "$dir/docs/adr/0001-accepted.md" <<'EOF'
# 0001 - Accepted decision

## Status

Accepted - implemented.
EOF

    cat > "$dir/docs/adr/0002-proposed.md" <<'EOF'
# 0002 - Proposed decision

## Status

Proposed - pending implementation.
EOF
}

mut_adr_status_clean() {
    write_adr_status_fixture "$1" \
        "Accepted decision" \
        "Proposed decision (proposed)" \
        "ADR 0001 is Accepted. ADR 0002 is Proposed."
}
MUTATOR=mut_adr_status_clean
run_case "Phase 8: ADR status index clean mapping passes" \
    0 'doc-parity: OK'

# --- Case 36: accepted ADR listed as proposed ---
mut_adr_accepted_listed_proposed() {
    write_adr_status_fixture "$1" \
        "Accepted decision (proposed)" \
        "Proposed decision (proposed)" \
        "ADR 0001 is Proposed. ADR 0002 is Proposed."
}
MUTATOR=mut_adr_accepted_listed_proposed
run_case "Phase 8: accepted ADR listed as proposed fails closed" \
    1 'ADR index status drift'

# --- Case 37: proposed ADR title missing proposed marker ---
mut_adr_proposed_missing_marker() {
    write_adr_status_fixture "$1" \
        "Accepted decision" \
        "Proposed decision" \
        "ADR 0001 is Accepted. ADR 0002 is Proposed."
}
MUTATOR=mut_adr_proposed_missing_marker
run_case "Phase 8: proposed ADR missing marker fails closed" \
    1 'ADR index status drift'

# --- Case 38: SECURITY.md audit metric lacks phase label ---
mut_security_audit_metric_missing_phase() {
    perl -0pi -e 's/clockify_mcp_audit_failures_total\{reason="persist_error",phase="outcome"\}/clockify_mcp_audit_failures_total{reason="persist_error"}/' \
        "$1/SECURITY.md"
}
MUTATOR=mut_security_audit_metric_missing_phase
run_case "Phase 4: SECURITY.md audit metric missing phase label fails closed" \
    1 'audit failure metric is missing the phase label'

# --- Case 39: SECURITY.md missing cross-origin baseline header docs ---
mut_security_missing_cross_origin_header_docs() {
    perl -0pi -e 's/, `Cross-Origin-Embedder-Policy: require-corp`//' \
        "$1/SECURITY.md"
}
MUTATOR=mut_security_missing_cross_origin_header_docs
run_case "Phase 4: SECURITY.md missing cross-origin baseline header docs fails closed" \
    1 'SECURITY\.md must document the P3-5 HTTP baseline header'

# --- Case 40: auth-model references removed rate-limit metric ---
mut_auth_model_removed_rate_limit_metric() {
    perl -0pi -e 's/clockify_mcp_rate_limit_rejections_total\{kind="window",scope="per_token"\}/clockify_mcp_per_subject_rate_limited_total/' \
        "$1/docs/auth-model.md"
}
MUTATOR=mut_auth_model_removed_rate_limit_metric
run_case "Phase 4: auth-model removed rate-limit metric fails closed" \
    1 'references removed rate-limit metric'

# --- Case 41: premature official Clockify launch claim ---
mut_premature_official_launch_claim() {
    printf '\nThis is the official\nClockify launch candidate.\n' \
        >> "$1/docs/support-matrix.md"
}
MUTATOR=mut_premature_official_launch_claim
run_case "Phase 3: premature official Clockify launch claim fails closed" \
    1 'banned premature official-product claim'

# --- Case 42: brand/legal review artifact missing ---
mut_missing_brand_legal_review() {
    rm "$1/docs/release/brand-legal-review.md"
}
MUTATOR=mut_missing_brand_legal_review
run_case "Phase 7: brand/legal review artifact missing fails closed" \
    1 'required launch doc missing'

# --- Case 43: brand/legal dependency/license evidence missing ---
mut_brand_legal_missing_license_evidence() {
    perl -0pi -e 's/\nDependency evidence: attach go-licenses output for\ninternal\/controlplane\/postgres, internal\/transport\/grpc, and\ninternal\/tracing\/otel\. Run make license-evidence, backed by\nscripts\/collect-license-evidence\.sh and go list -deps, as not legal advice\nand not license clearance\. Review npm\/package\.json\.tmpl and\n\.github\/workflows\/dependency-review\.yml because license-check: false\.\n//' \
        "$1/docs/release/brand-legal-review.md"
}
MUTATOR=mut_brand_legal_missing_license_evidence
run_case "Phase 7: brand/legal dependency evidence missing fails closed" \
    1 'must name dependency/license evidence sources'

# --- Case 44: ADR 0001 JSON Schema trade-off missing ---
mut_jsonschema_tradeoff_missing() {
    perl -0pi -e 's/\nRuntime tool-argument validation uses internal\/jsonschema as a\nDraft 2020-12 subset and is guarded by\ninternal\/tools\/schema_keyword_test\.go\.\n\nRejected: github\.com\/santhosh-tekuri\/jsonschema\/v6, because the current\ncatalog does not need \$ref, \$defs, conditionals, or a broader default SBOM\.\n//' \
        "$1/docs/adr/0001-stdlib-only-default-build.md"
}
MUTATOR=mut_jsonschema_tradeoff_missing
run_case "Phase 7: ADR 0001 JSON Schema trade-off missing fails closed" \
    1 'ADR 0001 must document the B\.07 stdlib-only JSON Schema validator trade-off'

# --- Case 45: brand/legal local license-evidence helper missing ---
mut_brand_legal_missing_local_license_helper() {
    perl -0pi -e 's/ Run make license-evidence, backed by\nscripts\/collect-license-evidence\.sh and go list -deps, as not legal advice\nand not license clearance\././' \
        "$1/docs/release/brand-legal-review.md"
}
MUTATOR=mut_brand_legal_missing_local_license_helper
run_case "Phase 7: brand/legal local license-evidence helper missing fails closed" \
    1 'must name dependency/license evidence sources'

# --- Case 46: brand/legal URI and gRPC service-name review missing ---
mut_brand_legal_missing_identifier_review() {
    perl -0pi -e 's/\nBrand identifiers under review: clockify:\/\/ resource URI scheme and\nclockify\.mcp\.v1\.MCP gRPC service name\. Review\ninternal\/transport\/grpc\/service\.go before clients depend on these names\.\n//' \
        "$1/docs/release/brand-legal-review.md"
}
MUTATOR=mut_brand_legal_missing_identifier_review
run_case "Phase 7: brand/legal URI and gRPC service-name review missing fails closed" \
    1 'must keep the URI and gRPC service-name brand review open'

# --- Case 47: SUPPORT.md stale SLSA mandatory wording ---
mut_support_stale_slsa_mandatory_wording() {
    perl -0pi -e 's/GitHub artifact attestations may return "Feature not available" for this\nuser-owned private repository\. ADR-0013 keeps SLSA best-effort and the\nmandatory cryptographic gate is the cosign binary\/image chain\./SLSA has been mandatory on every release since the public flip./' \
        "$1/SUPPORT.md"
}
MUTATOR=mut_support_stale_slsa_mandatory_wording
run_case "Phase 5: SUPPORT.md stale SLSA mandatory wording fails closed" \
    1 'SUPPORT\.md must not claim SLSA is mandatory'

# --- Case 48: stale public-content local-artifact wording ---
mut_stale_public_content_local_artifact_wording() {
    printf '\nThe remaining public-content work is local ignored/nested artifact cleanup.\n' \
        >> "$1/docs/opusreview-implementation-audit.md"
}
MUTATOR=mut_stale_public_content_local_artifact_wording
run_case "Phase 3: stale public-content local-artifact wording fails closed" \
    1 'banned stale public-surface string'

# --- Case 49: stale shared-service launch-blocking wording ---
mut_stale_shared_service_gap_wording() {
    printf '\nThis is the launch-blocking gap today.\nThere is no end-to-end test that boots a Postgres-tagged binary.\n' \
        >> "$1/docs/launch-candidate-checklist.md"
}
MUTATOR=mut_stale_shared_service_gap_wording
run_case "Phase 3: stale shared-service launch-blocking wording fails closed" \
    1 'banned stale public-surface string'

# --- Case 50: gRPC reflection dev-only posture missing ---
mut_grpc_reflection_posture_missing() {
    perl -0pi -e 's/,grpcreflection//' \
        "$1/docs/deploy/profile-private-network-grpc.md"
}
MUTATOR=mut_grpc_reflection_posture_missing
run_case "Phase 7: gRPC reflection dev-only posture missing fails closed" \
    1 'must document T-17 reflection-off posture'

# --- Case 51: build-tag submodule Dependabot watcher missing ---
mut_dependabot_submodule_watcher_missing() {
    perl -0pi -e 's/\n  - package-ecosystem: gomod\n    directory: \/internal\/transport\/grpc\n    schedule:\n      interval: weekly\n    open-pull-requests-limit: 5\n//' \
        "$1/.github/dependabot.yml"
}
MUTATOR=mut_dependabot_submodule_watcher_missing
run_case "Phase 7: build-tag submodule Dependabot watcher missing fails closed" \
    1 'must watch Go module dependency updates for /internal/transport/grpc'

# --- Case 52: root Dependabot build-tag ignore missing ---
mut_dependabot_root_ignore_missing() {
    perl -0pi -e 's/\n      - dependency-name: "github\.com\/jackc\/pgx\/v5"//' \
        "$1/.github/dependabot.yml"
}
MUTATOR=mut_dependabot_root_ignore_missing
run_case "Phase 7: root Dependabot build-tag ignore missing fails closed" \
    1 'root gomod watcher must ignore build-tag dependency updates for github.com/jackc/pgx/v5'

# --- Case 53: govulncheck version proof missing from CI ---
mut_govulncheck_version_proof_missing() {
    perl -0pi -e 's/\n          govulncheck -version//' \
        "$1/.github/workflows/ci.yml"
}
MUTATOR=mut_govulncheck_version_proof_missing
run_case "Phase 7: govulncheck version proof missing from CI fails closed" \
    1 'must print govulncheck -version'

# --- Case 54: govulncheck tool module missing ---
mut_govulncheck_tool_module_missing() {
    rm "$1/tools/govulncheck/go.mod"
}
MUTATOR=mut_govulncheck_tool_module_missing
run_case "Phase 7: govulncheck tool module missing fails closed" \
    1 'tools/govulncheck/go\.mod missing'

# --- Case 55: stale unconditional SLSA wording in public docs ---
mut_stale_public_slsa_wording() {
    printf '\nSigned releases (cosign + SLSA), SBOMs, FIPS variant.\n' \
        >> "$1/docs/support-matrix.md"
}
MUTATOR=mut_stale_public_slsa_wording
run_case "Phase 3: stale unconditional SLSA wording fails closed" \
    1 'banned stale public-surface string'

# --- Case 56: legacy HTTP EOL runbook missing migration headers ---
mut_legacy_http_eol_missing_headers() {
    perl -0pi -e 's/Legacy responses carry Deprecation: true and a successor-version Link\./Legacy clients should migrate eventually./' \
        "$1/docs/runbooks/legacy-http-eol.md"
}
MUTATOR=mut_legacy_http_eol_missing_headers
run_case "Phase 7: legacy HTTP EOL runbook missing migration headers fails closed" \
    1 'legacy-http-eol\.md must document deprecation headers'

# --- Case 57: clients doc missing serverInfo identity guidance ---
mut_clients_missing_server_identity() {
    perl -0pi -e 's/\nThe protocol initialize response uses serverInfo\.name: "clockify-go-mcp"\.\nThe clockify-mcp binary and \@apet97\/clockify-mcp-go wrapper are packaging names\.\n//' \
        "$1/docs/clients.md"
}
MUTATOR=mut_clients_missing_server_identity
run_case "Phase 4: clients doc missing serverInfo identity guidance fails closed" \
    1 'clients\.md must document protocol server identity'

# --- Case 58: clients doc missing default protocol-version guidance ---
mut_clients_missing_protocol_default() {
    perl -0pi -e 's/\nWhen a client omits params\.protocolVersion, MCP_DEFAULT_PROTOCOL_VERSION can\npin the fallback instead of the newest supported version\. Explicit supported\nclient versions are still echoed\.\n//' \
        "$1/docs/clients.md"
}
MUTATOR=mut_clients_missing_protocol_default
run_case "Phase 4: clients doc missing default protocol-version guidance fails closed" \
    1 'clients\.md must document default protocol-version fallback semantics'

# --- Case 59: AGENTS.md missing May 8 review-ledger read-first routing ---
mut_agents_missing_may8_review_ledger() {
    perl -0pi -e 's#docs/launch-readiness-review-may-8\.md\. The May 8 review disposition ledger\ncontains the objective-to-artifact completion audit\. Do not mark launch-ready\nwhile that audit says external evidence or approval gates remain open\.#docs/official-clockify-mcp-gap-analysis.md.#' \
        "$1/AGENTS.md"
}
MUTATOR=mut_agents_missing_may8_review_ledger
run_case "Phase 7: AGENTS.md missing May 8 review-ledger read-first routing fails closed" \
    1 'AGENTS\.md must route agents to the May 8 review ledger'

# --- Case 60: Makefile verify-vuln skips pinned govulncheck module ---
mut_makefile_verify_vuln_skips_pinned_module() {
    cat > "$1/Makefile" <<'EOF'
release-check:
	@echo "release-check: OK — local pre-ship gate passed"

verify-vuln:
	@if command -v govulncheck > /dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "[verify-vuln] govulncheck not installed, skipping."; \
	fi

# launch readiness still depends on the external evidence gates.
EOF
}
MUTATOR=mut_makefile_verify_vuln_skips_pinned_module
run_case "Phase 7: Makefile verify-vuln skipping pinned module fails closed" \
    1 'Makefile verify-vuln must not skip'

# --- Case 61: gap analysis blocker scope missing repo/legal/product gates ---
mut_gap_analysis_gate_scope_missing() {
    perl -0pi -e 's/The remaining blockers are not local test failures.*?feef83c641ced93d2ab6ba07ef766d61c82cc703\./Only external evidence blockers remain after the May 8 launch-review remediation tree is locally green: scheduled live-contract cron greens, candidate-tag security evidence, and release evidence./s' \
        "$1/docs/official-clockify-mcp-gap-analysis.md"
}
MUTATOR=mut_gap_analysis_gate_scope_missing
run_case "Phase 7: gap analysis blocker scope missing repo/legal/product gates fails closed" \
    1 'official-clockify-mcp-gap-analysis\.md must not narrow remaining launch blockers'

# --- Case 62: release-smoke SLSA skip accepts bare HTTP 404 ---
mut_release_smoke_broad_slsa_404_skip() {
    perl -0pi -e "s/if grep -qiF 'Feature not available' \\/tmp\\/slsa-default\\.err &&\\n               grep -qiF 'user-owned private repositor' \\/tmp\\/slsa-default\\.err; then/if grep -qiE 'Feature not available for user-owned private repositor|HTTP 404' \\/tmp\\/slsa-default.err; then/" \
        "$1/.github/workflows/release-smoke.yml"
}
MUTATOR=mut_release_smoke_broad_slsa_404_skip
run_case "Phase 7: release-smoke SLSA bare HTTP 404 skip fails closed" \
    1 'release-smoke SLSA skip must not accept a bare HTTP 404'

# --- Case 63: README unqualified SLSA provenance wording ---
mut_readme_unqualified_slsa_wording() {
    perl -0pi -e 's/Signed releases ship with cosign signatures and SPDX SBOMs\. SLSA build provenance is attached when GitHub artifact attestations are available\./Every binary and container image ships with cosign signatures, SPDX SBOM, and SLSA build provenance./' \
        "$1/README.md"
}
MUTATOR=mut_readme_unqualified_slsa_wording
run_case "Phase 3: README unqualified SLSA provenance wording fails closed" \
    1 'banned stale public-surface string|README\.md must qualify SLSA provenance'

# --- Case 64: workflow action reference is not SHA-pinned ---
mut_workflow_action_not_sha_pinned() {
    perl -0pi -e 's/actions\/checkout\@de0fac2e4500dabe0009e67214ff5f5447ce83dd/actions\/checkout\@v4/' \
        "$1/.github/workflows/deploy.yml"
}
MUTATOR=mut_workflow_action_not_sha_pinned
run_case "Phase 7: workflow action references must be SHA-pinned" \
    1 'GitHub workflow action must be pinned to a full commit SHA'

# --- Case 65: deploy SLSA skip accepts bare HTTP 404 ---
mut_deploy_broad_slsa_404_skip() {
    perl -0pi -e "s/if grep -qiF 'Feature not available' \\/tmp\\/deploy-slsa\\.err &&\\n               grep -qiF 'user-owned private repositor' \\/tmp\\/deploy-slsa\\.err; then/if grep -qiE 'Feature not available for user-owned private repositor|HTTP 404' \\/tmp\\/deploy-slsa.err; then/" \
        "$1/.github/workflows/deploy.yml"
}
MUTATOR=mut_deploy_broad_slsa_404_skip
run_case "Phase 7: deploy SLSA bare HTTP 404 skip fails closed" \
    1 'deploy SLSA skip must not accept a bare HTTP 404'

# --- Case 66: release workflow/docs unqualified SLSA chain wording ---
mut_unqualified_release_slsa_chain_wording() {
    perl -0pi -e 's/SLSA provenance when GitHub artifact attestations\n# are available/attested with SLSA build provenance, and scanned with/' \
        "$1/.github/workflows/docker-image.yml"
    perl -0pi -e 's/mandatory cosign\/SBOM chain, plus SLSA provenance when GitHub artifact attestations are available/cosign + SLSA chain/' \
        "$1/.github/workflows/release.yml"
    perl -0pi -e 's#When GitHub artifact attestations are available for the\nrepository account tier, the release workflow also stores SLSA build\nprovenance in the GitHub attestation service\. On the current user-owned\nprivate repository, the mandatory cryptographic gate is the cosign\nbinary/image signature chain\.#A SLSA build-provenance attestation stored in the GitHub attestation service.#' \
        "$1/docs/verify-release.md"
}
MUTATOR=mut_unqualified_release_slsa_chain_wording
run_case "Phase 3: release workflow/docs unqualified SLSA wording fails closed" \
    1 'banned stale public-surface string|docs/verify-release\.md must qualify SLSA provenance|docker-image\.yml must qualify image SLSA provenance'

# --- Case 67: agent handoff permissioned landing sequence ---
mut_agent_handoff_landing_sequence_missing() {
    perl -0pi -e 's/Land the remediation tree only after explicit approval\./Land when locally green./' \
        "$1/docs/agent-handoff.md"
}
MUTATOR=mut_agent_handoff_landing_sequence_missing
run_case "Phase 7: agent handoff permissioned landing sequence missing fails closed" \
    1 'agent-handoff\.md must preserve the permissioned landing sequence'

# --- Case 68: dependency-review lacks default-branch evidence trigger ---
mut_dependency_review_missing_push_trigger() {
    perl -0pi -e 's/\n  push:\n    branches: \[main\]//' \
        "$1/.github/workflows/dependency-review.yml"
}
MUTATOR=mut_dependency_review_missing_push_trigger
run_case "Phase 7: dependency-review default-branch trigger missing fails closed" \
    1 'dependency-review\.yml must support PR checks and first default-branch evidence runs'

# --- Case 69: release-smoke doctor output artifact missing ---
mut_release_smoke_doctor_artifact_missing() {
    perl -0pi -e 's/\n      - name: Validate doctor strict evidence files\n.*?          retention-days: 30\n//s' \
        "$1/.github/workflows/release-smoke.yml"
}
MUTATOR=mut_release_smoke_doctor_artifact_missing
run_case "Phase 7: release-smoke doctor output artifact missing fails closed" \
    1 'release-smoke\.yml must archive doctor strict output as a workflow artifact'

# --- Case 70: docker-image SLSA feature-gate notice missing ---
mut_docker_image_slsa_notice_missing() {
    perl -0pi -e "s/steps\\.slsa_provenance\\.outcome == 'failure'/steps.slsa_provenance.outcome == 'success'/" \
        "$1/.github/workflows/docker-image.yml"
}
MUTATOR=mut_docker_image_slsa_notice_missing
run_case "Phase 7: docker-image SLSA feature-gate notice missing fails closed" \
    1 '\.github/workflows/docker-image\.yml must qualify image SLSA provenance availability'

# --- Final report ---
echo
if [ "$tests_failed" -gt 0 ]; then
    printf 'check-doc-parity tests: %d/%d FAILED\n' "$tests_failed" "$tests_run" >&2
    exit 1
fi
printf 'check-doc-parity tests: %d/%d OK\n' "$tests_run" "$tests_run"
