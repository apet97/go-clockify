# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.3.x   | Yes       |
| 0.2.x   | Security fixes only |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Email security reports to: apet97@github.com

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

- API keys passed via environment variables only (never in config files)
- Config validation rejects non-HTTPS base URLs unless loopback or explicitly opted in
- HTTP transport: constant-time bearer token comparison (`crypto/subtle`)
- HTTP transport: strict `Authorization: Bearer <token>` parsing
- HTTP server timeouts: `ReadHeaderTimeout` (10s), `ReadTimeout` (30s), `WriteTimeout` (60s), `IdleTimeout` (120s) — prevents resource exhaustion
- Security headers: `X-Content-Type-Options: nosniff`, `Cache-Control: no-store` on all HTTP responses
- JSON error responses (not `text/plain`) — prevents content-type sniffing
- ID validation: path-building handlers reject path traversal characters (`/?#`)
- Webhook URL validation: rejects non-HTTPS URLs, embedded credentials, localhost, and private/loopback/link-local/reserved IP literals
- Name resolution: ambiguous matches fail closed (no guessing)
- CORS: cross-origin requests rejected by default (explicit opt-in required)
- Policy modes: destructive tools can be disabled entirely
- Dry-run: destructive operations can be previewed before execution
- Structured logging to stderr only (never stdout in stdio mode)
- All 124 tools carry MCP annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`)
- Response body limits: 10MB on API responses, 2MB on HTTP request bodies
- Zero external dependencies (stdlib only) — minimal supply chain attack surface
- Initialization guard: `tools/call` rejected before `initialize` handshake
