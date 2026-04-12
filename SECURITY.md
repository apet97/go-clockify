# Security Policy

## Supported Versions

| Version | Supported           |
|---------|---------------------|
| 0.5.x   | Yes                 |
| 0.4.x   | Security fixes only |

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
- **Transport hardening**: `ReadHeaderTimeout` (10s), `ReadTimeout` (30s), `WriteTimeout` (60s), `IdleTimeout` (120s) prevent resource exhaustion. Every response carries `Strict-Transport-Security`, `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Permissions-Policy: ()`, `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`.
- **CORS**: cross-origin requests rejected by default. Explicit opt-in required via `MCP_ALLOWED_ORIGINS` (allowlist) or `MCP_ALLOW_ANY_ORIGIN=1`.
- **DNS rebinding protection**: opt-in via `MCP_STRICT_HOST_CHECK=1` — when enabled, the Host header must match `localhost`, `127.0.0.1`, `::1`, or a host component of an allowed origin. Non-loopback hosts are rejected unless explicitly allowlisted; `0.0.0.0` is never accepted as a Host header.
- **Config validation**: non-HTTPS `CLOCKIFY_BASE_URL` rejected unless loopback or explicitly opted in with `CLOCKIFY_INSECURE=1`.
- **Panic containment**: both the stdio dispatch goroutine and the HTTP handlers recover panics, emit a `panic_recovered` slog event with the stack, increment `clockify_mcp_panics_recovered_total{site}`, and return a tool-error envelope instead of crashing the process.
- **PII-redacting logs**: the default slog handler is wrapped in `internal/logging.RedactingHandler`, which recursively masks 20+ well-known secret-key patterns (`authorization`, `api_key`, `bearer`, `token`, `cookie`, `client_secret`, `refresh_token`, …) before encoding.
- **Webhook URL validation**: rejects non-HTTPS URLs, embedded credentials, localhost, and private/loopback/link-local/reserved IP literals.
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
request/response bodies travel in plain HTTP. See `deploy/Caddyfile.example`
for a reference configuration that uses Caddy's automatic Let's Encrypt
support.

`CLOCKIFY_INSECURE=1` only bypasses base-URL scheme validation when resolving
`CLOCKIFY_BASE_URL`; it does not disable TLS certificate verification in the
outbound Clockify client. See `docs/safe-usage.md` for the full scope.

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
