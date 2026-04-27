# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.1.x   | Yes       |
| 1.0.x   | Yes       |

The `0.x` line reached end-of-life when `v1.0.0` shipped. Security
fixes will not be backported beyond the supported lines listed above.
See [docs/release-policy.md](docs/release-policy.md) for the full
support window, deprecation policy, and definition of "breaking
change" used by this project.

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
- Path traversal in ID validation
- CORS bypass in HTTP transport
- Timing attacks on bearer token comparison

## Security Features

- **AuthN**: API keys passed via environment variables only (never in config files); HTTP transport requires a ≥16-char bearer token compared with `crypto/subtle`; strict `Authorization: Bearer <token>` parsing.
- **Inline /metrics security**: `/metrics` on the main HTTP listener is **disabled by default** (`MCP_HTTP_INLINE_METRICS_ENABLED`). When enabled, access requires authentication: `inherit_main_bearer` reuses the primary bearer token; `static_bearer` uses a dedicated separate token; `none` requires explicit opt-in and emits a startup warning. The dedicated `MCP_METRICS_BIND` listener is the recommended alternative for shared-service deployments.
- **Audit durability**: non-read-only tool calls emit an `AuditEvent` and increment `clockify_mcp_audit_events_total`. Persistence failures are always logged at `ERROR` level and increment `clockify_mcp_audit_failures_total{reason="persist_error"}`. In `fail_closed` mode (`MCP_AUDIT_DURABILITY=fail_closed`) a persistence failure causes the tool call to return an error rather than silently proceeding; in `best_effort` mode (default) the tool call succeeds and the audit failure is observable only through logs and metrics.
- **Audit fidelity**: every tool descriptor carries a `RiskClass` bitmask (`Read | Write | Billing | Admin | PermissionChange | ExternalSideEffect | Destructive`) and an `AuditKeys []string` listing action-defining argument keys. The audit recorder consumes both: `RiskClass` is recorded on every event so downstream filters can isolate billing / admin / permission-change calls, and `AuditKeys` causes the recorder to capture the named arguments alongside the `*_id` fields (e.g. `role`, `status`, `quantity`, `unit_price` for permission/billing changes — not just the IDs that were touched). Closes the gap from audit Finding 8 where audit events recorded *what* was touched but not *what change* was applied.
- **Transport hardening**: `ReadHeaderTimeout` (10s), `ReadTimeout` (30s), `WriteTimeout` (60s), `IdleTimeout` (120s) prevent resource exhaustion. Every response carries `Strict-Transport-Security`, `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Permissions-Policy: ()`, `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`.
- **CORS**: cross-origin requests rejected by default. Explicit opt-in required via `MCP_ALLOWED_ORIGINS` (allowlist) or `MCP_ALLOW_ANY_ORIGIN=1`.
- **DNS rebinding protection**: opt-in via `MCP_STRICT_HOST_CHECK=1` — when enabled, the Host header must match `localhost`, `127.0.0.1`, `::1`, or a host component of an allowed origin. Non-loopback hosts are rejected unless explicitly allowlisted; `0.0.0.0` is never accepted as a Host header.
- **Config validation**: non-HTTPS `CLOCKIFY_BASE_URL` rejected unless loopback or explicitly opted in with `CLOCKIFY_INSECURE=1`. `CLOCKIFY_WORKSPACE_ID` is run through `resolve.ValidateID` at startup so path-traversal-shaped values (`/`, `?`, `#`, `%`, `..`, control bytes) fail config load instead of silently propagating into every `/workspaces/{id}/...` URL.
- **Panic containment**: both the stdio dispatch goroutine and the HTTP handlers recover panics, emit a `panic_recovered` slog event with the stack, increment `clockify_mcp_panics_recovered_total{site}`, and return a tool-error envelope instead of crashing the process.
- **PII-redacting logs**: the default slog handler is wrapped in `internal/logging.RedactingHandler`, which recursively masks 20+ well-known secret-key patterns (`authorization`, `api_key`, `bearer`, `token`, `cookie`, `client_secret`, `refresh_token`, …) before encoding.
- **Hosted-profile error sanitisation**: tool-error responses on the `shared-service` and `prod-postgres` profiles omit upstream Clockify response bodies (`CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=1` is the profile default). A 4xx body from Clockify can carry per-tenant identifiers; without sanitisation those leak across tenant boundaries via the MCP wire. The full upstream `APIError` is still logged server-side via slog for operator debugging. Operator override: `CLOCKIFY_SANITIZE_UPSTREAM_ERRORS=0/1`.
- **Webhook URL validation**: rejects non-HTTPS URLs, embedded credentials, localhost, and private/loopback/link-local/reserved IP literals. Hosted profiles (`shared-service`, `prod-postgres`) additionally resolve the host via DNS and reject any reply containing a private/reserved IP — closing the literal-IP-only gap (a hostname pointing at `169.254.169.254` would otherwise sail through the literal check). Operator override: `CLOCKIFY_WEBHOOK_VALIDATE_DNS=0/1`. Per-deployment escape hatch: `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS=<host>[,<host>...]` admits known-trusted hostnames (exact or leading-dot suffix) for split-horizon DNS environments.
- **Path injection**: ID validation rejects path traversal characters (`/?#%`, `..`, control bytes).
- **Policy modes**: destructive tools can be disabled entirely; fine-grained deny/allow lists for both individual tools and Tier 2 groups.
- **Dry-run**: three-strategy (confirm pattern, GET preview, minimal fallback) dry-run for every destructive operation; enabled by default.
- **Name resolution**: ambiguous matches fail closed (no guessing).
- **Stdout purity**: protocol responses only on stdout; every log goes to stderr via slog — never mixes with JSON-RPC frames in stdio mode.
- **Tool annotations**: all 124 tools carry `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`, and `title`. Spec-strict MCP clients get the correct safety hints on every descriptor.
- **Response limits**: 10MB on Clockify API responses, 2MB default on HTTP request bodies.
- **Zero external dependencies** (stdlib only) — minimal supply chain attack surface; no `go.sum` at all.
- **Initialization guard**: `tools/call` rejected before `initialize` handshake (`-32002 server not initialized`).

## TLS / HTTP Transport

The HTTP transport does **not** terminate TLS. Production deployments MUST
front the server with a TLS-terminating reverse proxy (Caddy, nginx, Envoy,
Traefik, or a cloud load balancer). Without a proxy, the bearer token and all
request/response bodies travel in plain HTTP. See `deploy/Caddyfile`
for a reference configuration that uses Caddy's automatic Let's Encrypt
support.

`CLOCKIFY_INSECURE=1` only bypasses base-URL scheme validation when resolving
`CLOCKIFY_BASE_URL`; it does not disable TLS certificate verification in the
outbound Clockify client. Hosted profiles (`shared-service`,
`prod-postgres`) refuse `CLOCKIFY_INSECURE=1` outright at startup —
only `local-stdio` / `single-tenant-http` honour the override.

## Verifying release artifacts

Every release ships with:

- The binary (`clockify-mcp-<platform>[.exe]`)
- A sigstore bundle (`clockify-mcp-<platform>.sigstore.json`) produced
  by keyless cosign signing
- A SPDX SBOM (`clockify-mcp-<platform>.spdx.json`)
- A GitHub build provenance attestation (SLSA-aligned, stored in the
  GitHub attestation service)
- A multi-arch container image at `ghcr.io/apet97/go-clockify:v<version>`
  that is Trivy-scanned (HIGH+CRITICAL blocking), cosign-signed, carries
  a SPDX SBOM attestation, and an attested SLSA build provenance with the
  image digest as subject

Release binaries are built with `-trimpath` so they do not embed the
builder's absolute paths.

See [docs/verification.md](docs/verification.md) for step-by-step
verification commands using `cosign verify-blob --bundle`,
`cosign verify <image>`, and `gh attestation verify`.
