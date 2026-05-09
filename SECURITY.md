# Security Policy

## Supported Versions

| Version | Status                                                                                          |
|---------|-------------------------------------------------------------------------------------------------|
| 1.2.x   | Active — receives security fixes alongside features and bug fixes                               |
| 1.1.x   | Superseded — upgrade to `1.2.x` for security fixes                                              |
| 1.0.x   | Patch-only for correctness regressions on the stable v1 wire format (security CVEs that meet that bar are backported) |
| 0.x     | End-of-life since `v1.0.0`                                                                      |

Security fixes always land on the Active minor (`1.2.x` today). The
prior minor (`1.1.x`) is superseded; operators on it should upgrade
rather than wait for a backport. The `1.0.x` line receives only
correctness-regression patches on the stable v1 wire format — security
CVEs that meet that bar are backported, others are not.

See [SUPPORT.md](SUPPORT.md) for the canonical version-status state and
[docs/release-policy.md](docs/release-policy.md) for the full support
window, deprecation policy, and definition of "breaking change" used by
this project.

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Use the private **GitHub Security Advisory** workflow at
<https://github.com/apet97/go-clockify/security/advisories/new> to
disclose a vulnerability. That channel is end-to-end encrypted with
project maintainers and provides an audit trail for the fix lifecycle.

Include:
- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

## Response Timeline

- **Acknowledgment:** Within 48 hours
- **Initial assessment:** Within 1 week
- **Fix release:** Depends on severity (critical: ASAP, high: 1-2 weeks, medium: next release)

## Scope

The following are in scope:
- API key exposure or leakage
- Command injection via tool inputs
- SSRF through webhook URL parameters
- Authentication bypass in HTTP transport
- OIDC/JWKS validation weaknesses, including token-binding and key-selection failures
- Tenant-isolation failures in shared-service HTTP, gRPC, and control-plane storage
- Audit-durability failures for non-read-only tool calls
- Path traversal in ID validation
- CORS bypass in HTTP transport
- DNS rebinding or private-network exposure in webhook and HTTP transport paths
- Timing attacks on bearer token comparison

## Security Features

- **AuthN**: API keys passed via environment variables only (never in config files); HTTP transport requires a ≥16-char bearer token compared with `crypto/subtle`; strict `Authorization: Bearer <token>` parsing.
- **Inline /metrics security**: `/metrics` on the main HTTP listener is **disabled by default** (`MCP_HTTP_INLINE_METRICS_ENABLED`). When enabled, access requires authentication: `inherit_main_bearer` reuses the primary bearer token; `static_bearer` uses a dedicated separate token; `none` requires explicit opt-in and emits a startup warning. The dedicated `MCP_METRICS_BIND` listener is the recommended alternative for shared-service deployments.
- **Audit durability**: non-read-only tool calls emit intent/outcome `AuditEvent` records and increment `clockify_mcp_audit_events_total`. Persistence failures are always logged at `ERROR` level and increment `clockify_mcp_audit_failures_total` with `reason="persist_error"` and `phase` set to `intent`, `outcome`, or `single`; outcome durability alerts key on `clockify_mcp_audit_failures_total{reason="persist_error",phase="outcome"}`. In `fail_closed` mode (`MCP_AUDIT_DURABILITY=fail_closed`) an intent persistence failure causes the tool call to return an error before mutation; outcome persistence failure is logged and metered. In `fail_closed_strict`, outcome persistence failure is also returned to the client after the mutation completes. In `best_effort` mode (default) the tool call succeeds and audit failures are observable only through logs and metrics.
- **Audit fidelity**: every tool descriptor carries a `RiskClass` bitmask (`Read | Write | Billing | Admin | PermissionChange | ExternalSideEffect | Destructive`) and an `AuditKeys []string` listing action-defining argument keys. The audit recorder consumes both: `RiskClass` is recorded on every event so downstream filters can isolate billing / admin / permission-change calls, and `AuditKeys` causes the recorder to capture the named arguments alongside the `*_id` fields (e.g. `role`, `status`, `quantity`, `unit_price` for permission/billing changes — not just the IDs that were touched). Closes the gap from audit Finding 8 where audit events recorded *what* was touched but not *what change* was applied.
- **Transport hardening**: `ReadHeaderTimeout` (10s), `ReadTimeout` (30s), `WriteTimeout` (60s), `IdleTimeout` (120s) prevent resource exhaustion. Every MCP HTTP response carries `Strict-Transport-Security` when TLS or a trusted HTTPS proxy is active, plus `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`, `X-Frame-Options: DENY`, `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Embedder-Policy: require-corp`, `Cross-Origin-Resource-Policy: same-origin`, `Referrer-Policy: no-referrer`, `Permissions-Policy: ()`, `X-Content-Type-Options: nosniff`, and `Cache-Control: no-store`.
- **CORS**: cross-origin requests rejected by default. Explicit opt-in required via `MCP_ALLOWED_ORIGINS` (allowlist) or `MCP_ALLOW_ANY_ORIGIN=1`.
- **DNS rebinding protection**: opt-in via `MCP_STRICT_HOST_CHECK=1` — when enabled, the Host header must match `localhost`, `127.0.0.1`, `::1`, or a host component of an allowed origin. Non-loopback hosts are rejected unless explicitly allowlisted; `0.0.0.0` is never accepted as a Host header.
- **Config validation**: non-HTTPS `CLOCKIFY_BASE_URL` rejected unless loopback or explicitly opted in with `CLOCKIFY_INSECURE=1` (hosted profiles `shared-service` / `prod-postgres` refuse the override outright at startup — see TLS / HTTP Transport below). `CLOCKIFY_WORKSPACE_ID` is run through `resolve.ValidateID` at startup so path-traversal-shaped values (`/`, `?`, `#`, `%`, `..`, control bytes) fail config load instead of silently propagating into every `/workspaces/{id}/...` URL.
- **Panic containment**: both the stdio dispatch goroutine and the HTTP handlers recover panics, emit a `panic_recovered` slog event with a bounded, path-sanitized stack, increment `clockify_mcp_panics_recovered_total{site}`, and return a tool-error envelope instead of crashing the process.
- **PII-redacting logs**: the default slog handler is wrapped in `internal/logging.RedactingHandler`, which recursively masks 20+ well-known secret-key patterns (`authorization`, `api_key`, `bearer`, `token`, `cookie`, `client_secret`, `refresh_token`, …) and obvious secret-shaped string values before encoding.
- **Hosted-profile error sanitisation**: tool-error responses on the `shared-service` and `prod-postgres` profiles omit upstream Clockify response bodies (`CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` is the profile default). A 4xx body from Clockify can carry per-tenant identifiers; without sanitisation those leak across tenant boundaries via the MCP wire. The full upstream `APIError` is still logged server-side via slog for operator debugging. Operator override: `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=0/1`.
- **Webhook URL validation**: rejects non-HTTPS URLs, embedded credentials, localhost, and private/loopback/link-local/reserved IP literals. Hosted profiles (`shared-service`, `prod-postgres`) additionally resolve the host via DNS and reject any reply containing a private/reserved IP — closing the literal-IP-only gap (a hostname pointing at `169.254.169.254` would otherwise sail through the literal check). Operator override: `CLOCKIFY_WEBHOOK_VALIDATE_DNS=0/1`. Per-deployment escape hatch: `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=<host>[,<host>...]` admits known-trusted hostnames (exact or leading-dot suffix) for split-horizon DNS environments.
- **Path injection**: ID validation rejects path traversal characters (`/?#%`, `..`, control bytes).
- **Policy modes**: five modes (`read_only`, `time_tracking_safe`, `safe_core`, `standard`, `full`) let operators disable destructive tools entirely or apply fine-grained deny/allow lists for both individual tools and Tier 2 groups.
- **Dry-run**: three-strategy (confirm pattern, GET preview, minimal fallback) dry-run for every destructive operation; enabled by default. Non-destructive RW tools whose execution triggers an external side effect (`clockify_send_invoice`, `clockify_mark_invoice_paid`, `clockify_test_webhook`, `clockify_deactivate_user`) also honour `dry_run:true` — the handler GETs a preview and returns it without issuing the PUT/POST, so agent flows can stage a confirmation step before billing or admin actions land.
- **Name resolution**: ambiguous matches fail closed (no guessing).
- **Stdout purity**: protocol responses only on stdout; every log goes to stderr via slog — never mixes with JSON-RPC frames in stdio mode.
- **Tool annotations**: all 128 current catalog tools carry `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`, and `title`. Spec-strict MCP clients get the correct safety hints on every descriptor.
- **Response limits**: 10MB on Clockify API responses, 4MB default on HTTP request bodies (`MCP_MAX_MESSAGE_SIZE`, capped at 100MB).
- **Zero external dependencies in the default binary** (stdlib only) — minimal supply chain attack surface. The root `go.mod` has zero `require` lines beyond the workspace pointer to the local `internal/tracing/otel` sub-module; the build-tagged sub-modules (`internal/transport/grpc`, `internal/controlplane/postgres`, `internal/tracing/otel`) live in their own `go.mod` files and only enter the build under `-tags=grpc`/`postgres`/`otel`. The root `go.sum` covers those sub-module deps for reproducibility but is irrelevant to the default-binary attack surface.
- **Initialization guard**: `tools/call` rejected before `initialize` handshake (`-32002 server not initialized`).

## TLS / HTTP Transport

By default the HTTP transport does **not** terminate TLS — production
deployments using `static_bearer`, `oidc`, or `forward_auth` MUST front the
server with a TLS-terminating reverse proxy (Caddy, nginx, Envoy, Traefik,
or a cloud load balancer). Without a proxy, the bearer token and all
request/response bodies travel in plain HTTP. See `deploy/Caddyfile`
for a reference configuration that uses Caddy's automatic Let's Encrypt
support.

In-process TLS termination is supported on the `streamable_http` and `grpc`
transports when explicit cert + key paths are set:

- `streamable_http`: set `MCP_HTTP_TLS_CERT` + `MCP_HTTP_TLS_KEY` (and
  `MCP_MTLS_CA_CERT_PATH` for `MCP_AUTH_MODE=mtls`). The legacy `http`
  transport rejects `MCP_HTTP_TLS_CERT` at config load — terminate TLS
  upstream and use `forward_auth`.
- `grpc` (build-tag opt-in): set `MCP_GRPC_TLS_CERT` + `MCP_GRPC_TLS_KEY`
  (and `MCP_MTLS_CA_CERT_PATH` for `MCP_AUTH_MODE=mtls`).

mTLS-anchored deployments rely on this in-process termination — see
[`docs/support-matrix.md`](docs/support-matrix.md) for the per-transport
auth-mode matrix.

`CLOCKIFY_INSECURE=1` only bypasses base-URL scheme validation when resolving
`CLOCKIFY_BASE_URL`; it does not disable TLS certificate verification in the
outbound Clockify client. Hosted profiles (`shared-service`,
`prod-postgres`) refuse `CLOCKIFY_INSECURE=1` outright at startup —
only `local-stdio` / `single-tenant-http` honour the override.

## Verifying release artifacts

Every release ships **15 binaries across five tag combinations**
(see [`docs/release-policy.md`](docs/release-policy.md#release-artifacts)
for the full enumeration and `scripts/check-release-assets.sh` for
the canonical platform matrix):

- 5 default (stdlib only): `darwin-arm64`, `darwin-x64`,
  `linux-x64`, `linux-arm64`, `windows-x64.exe`.
- 4 FIPS-tagged: `darwin/linux × arm64/x64` (Go's FIPS 140-3 module
  has no Windows toolchain support).
- 2 Postgres-tagged: `linux × arm64/x64`. Required by
  `doctor --strict --check-backends` per the hosted-launch checklist.
- 2 gRPC-tagged: `linux × arm64/x64`. Backs the
  `private-network-grpc` profile.
- 2 gRPC + Postgres: `linux × arm64/x64`. The hosted gRPC shape.

Each binary ships with:

- A sigstore bundle (`<binary>.sigstore.json`) produced by per-binary
  keyless cosign signing.
- A SPDX SBOM (`<binary>.spdx.json`).
- A GitHub build provenance attestation (SLSA-aligned, stored in the
  GitHub attestation service) when GitHub artifact attestations are
  available for the repository account tier. On the current user-owned
  private repository, ADR-0013 keeps SLSA best-effort and the mandatory
  cryptographic gate is the cosign binary/image chain.

`SHA256SUMS.txt` is shipped alongside as an unsigned manifest; it
lets `sha256sum -c` cross-check downloads against goreleaser's staged
hashes once a binary is independently verified via cosign.

A multi-arch container image at `ghcr.io/apet97/go-clockify:v<version>`
ships in parallel: Trivy-scanned (HIGH+CRITICAL blocking),
cosign-signed, carries a SPDX SBOM attestation, and carries SLSA build
provenance for the image digest when GitHub artifact attestations are
available.

Release binaries are built with `-trimpath` so they do not embed the
builder's absolute paths.

See [docs/verification.md](docs/verification.md) for step-by-step
verification commands using `cosign verify-blob --bundle`,
`cosign verify <image>`, and `gh attestation verify`.

## Candidate-tag security evidence

This section is the per-candidate-tag evidence ledger for Group 6 of
[`docs/launch-candidate-checklist.md`](docs/launch-candidate-checklist.md).
Each entry must record the candidate tag, the peeled commit SHA,
sanitised host short name (no user-identifying or IP context), date,
exact command, exit code, and tool version, plus any nontrivial output.
Local logs alone are not enough for the workflow-backed Group 7 boxes;
this section is the candidate-tag preflight-on-tag evidence anchor for
Group 6 only. External reviewer attestations remain a separate gate
documented under "Paid-hosted external security review" in
[`docs/launch-readiness-review-may-8.md`](docs/launch-readiness-review-may-8.md).

### v1.2.1-rc.1 — 2026-05-09

- **Candidate tag:** `v1.2.1-rc.1`
- **Annotated tag SHA:** `49351c73b6cc60f93427dd9e633f606f2df341a9`
- **Peeled commit SHA:** `a5d5f75769dc834a268f6ab24949b139ac4cff85`
- **Host short name:** `darwin-arm64-launch-host` (sanitised)
- **Working directory:** `GOCLMCP-worktrees/opus-group6-security-20260509`
  (worktree pinned at the peeled tag SHA; not a tag checkout, so
  `git status -sb` reports the worktree branch, not a detached
  `HEAD`)
- **Date (UTC):** 2026-05-09
- **Toolchain envelope:** `GOTOOLCHAIN=go1.25.10` for `make check`,
  `make verify-vuln`, and `make verify-fips`. The host system Go is
  `go1.26.2 darwin/arm64`; the pin keeps results reproducible against
  the launch-candidate Go 1.25.10 line documented in
  [`docs/support-matrix.md`](docs/support-matrix.md).

| # | Command | Exit | Tool version and key output |
|---|---------|-----:|----------------------------|
| 1 | `GOTOOLCHAIN=go1.25.10 make check` | `0` | `go1.25.10`; ran `gofmt -l`, `go vet ./...`, and `go test -race -count=1 -timeout 120s ./...`. Every package reported `ok`; no `FAIL` lines. |
| 2 | `make verify-vuln` | `0` | Pinned `govulncheck@v1.3.0` invoked under `GOTOOLCHAIN=go1.25.10` with `Go: go1.25.10`. Vulnerability database `https://vuln.go.dev` updated 2026-05-07 19:21:40 UTC. Final line: `No vulnerabilities found.` |
| 3 | `make secret-scan` | `0` | Ran `gitleaks detect --no-git --source . --redact --config .gitleaks.toml` with `gitleaks 8.30.1`. Final lines: `scanned ~4954224 bytes (4.95 MB) in 642ms` and `no leaks found`. |
| 4 | `semgrep scan --config p/default --metrics=off --error --exclude .git --exclude .bench --exclude clockify-mcp .` | `0` | `semgrep 1.157.0` with the `p/default` rule pack. Scan summary: 558 rules / 1154 targets / 0 findings (0 blocking); `~99.9%` parsed lines; 27 `--exclude` matches and 210 `.semgrepignore` matches. |
| 5 | `git grep -n -C 5 nosemgrep -- ':!CHANGELOG.md'` | `0` | Local `git` (host); the command runs with `\|\| true` in the runbook plan so any non-zero exit only signals "no matches". Enumerated five code-side `nosemgrep` directives, all with inline justification within five lines (see list below); no untracked or undocumented suppression sites. |
| 6 | `GOTOOLCHAIN=go1.25.10 make verify-fips` | `0` | `go1.25.10` with `GOFIPS140=latest`. The `-tags=fips` binary printed `fips140_enabled` on startup; the `-tags=fips` race test suite reported every package `ok`. The `-tags=fips,grpc` build combination completed cleanly; no `FAIL` lines. |

`nosemgrep` directive enumeration (item 5) — every code-side
suppression has an inline justification within five lines and points
back to a tracked ADR or runbook:

- `tests/harness/grpc.go:71` — `grpc.WithTransportCredentials(insecure.NewCredentials())`
  for the `bufconn`-backed in-memory test transport. ADR
  [`0008-grpc-auth-interceptor`](docs/adr/0008-grpc-auth-interceptor.md)
  records why this is scoped to the in-memory test harness only;
  production gRPC auth and mTLS coverage live in
  `internal/transport/grpc/` tests and the `grpc-auth-smoke` target.
- `internal/mcp/transport_streamable_http.go:541` — `: session <id>\n\n`
  SSE comment frame.
- `internal/mcp/transport_streamable_http.go:563` — `id: <int>\n` SSE
  ID line (server-generated monotonic integer).
- `internal/mcp/transport_streamable_http.go:565` — `event: <method>\n`
  SSE event line (server constants only).
- `internal/mcp/transport_streamable_http.go:568` — `data: <json>\n\n`
  SSE payload line (`json.Marshal`-encoded before framing).

  All four SSE suppressions are scoped to `text/event-stream` framing,
  not HTML, and are recorded in ADR
  [`0017-streamable-http-session-rehydration`](docs/adr/0017-streamable-http-session-rehydration.md)
  under "Security-review note".

The remaining `nosemgrep` hits captured by the `git grep` enumeration
are documentation references in this file family
(`docs/adr/0008-*.md`, `docs/adr/0017-*.md`,
`docs/claude-code-opus-remaining-prompts.md`,
`docs/launch-candidate-checklist.md`,
`docs/launch-readiness-review-may-8.md`,
`docs/runbooks/release-candidate-evidence.md`,
`scripts/prepare-rc-evidence.sh`,
`scripts/test-prepare-rc-evidence.sh`) describing the directive itself,
not new uses.

**Scope and non-claims.** This evidence closes only the four
candidate-tag scan boxes in Group 6 of the launch-candidate checklist
(`make verify-vuln`, gitleaks, Semgrep with `nosemgrep` audit, and
`make verify-fips`). It does **not** declare the repository
launch-ready: Group 7 release/sigstore/SLSA evidence, the mutation
cron evidence on the candidate SHA, the npm expected-version proof,
the paid-hosted external security review, the
DPA/terms/privacy/trademark/branding gates, the P1-8 paid-commercial
RLS decision, the cross-replica hosted HTTP quotas, and the
launch-candidate tracking issue all remain open as documented in
[`docs/launch-readiness-review-may-8.md`](docs/launch-readiness-review-may-8.md).
Issue #78 (19-context branch-protection restoration) is preserved as
open by operator instruction and is not affected by this evidence.
