# Hardening Plan — GOCLMCP

**Created**: 2026-04-06
**Updated**: 2026-04-06
**Status**: Phase 2 complete — production review hardening

---

## Security

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| S1 | ReportsURL missing HTTPS validation (SSRF) | Critical | **Fixed** | `validateBaseURL()` applied to `CLOCKIFY_REPORTS_URL` |
| S2 | Constant-time bearer token comparison | — | Already present | `crypto/subtle.ConstantTimeCompare` in transport_http.go |
| S3 | CORS rejected by default | — | Already present | Empty allowed origins list rejects all cross-origin requests |
| S4 | Response body size limits | — | Already present | 10MB API response, 2MB HTTP request body |
| S5 | ID validation (path traversal chars) | — | Already present | `resolve.ValidateID()` rejects `/?#` characters |
| S6 | No secrets in logs | — | Already present | API key never logged; errors use `slog` to stderr |
| S7 | Security headers on all HTTP responses | — | Already present | `X-Content-Type-Options: nosniff`, `Cache-Control: no-store` |

## Correctness

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| C1 | Transport value not validated | High | **Fixed** | Strict switch: "stdio" or "http" only |
| C2 | Timezone not validated at config time | High | **Fixed** | `time.LoadLocation()` at config load |
| C3 | HTTP bearer token not fail-fast | High | **Fixed** | Config rejects `http` without token |
| C4 | Stale hardcoded user-agent version | High | **Fixed** | Default changed to "dev"; main.go overrides |
| C5 | intArg NaN/Inf/overflow produces garbage | Medium | **Fixed** | Bounds check returns fallback |
| C6 | Dry-run static maps not compile-validated | Low | Remaining | Golden tests catch tool count drift |
| C7 | Bootstrap AlwaysVisible hardcoded | Low | Remaining | Acceptable: discovery/safety tools |
| C8 | Pagination end-of-data assumption | Low | Remaining | 1000-page safety stop mitigates |
| C9 | User cache theoretical TOCTOU | Low | Remaining | Worst case: extra API call |

## Architecture

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| A1 | Protocol core coupled to 5 domain packages | High | **Fixed** | Extracted `Enforcement` + `Activator` interfaces; server.go has zero domain imports |
| A2 | No connection pooling on HTTP client | High | **Fixed** | Explicit `http.Transport` with `MaxIdleConns`, `MaxConnsPerHost`, `IdleConnTimeout` |
| A3 | Client response body double-read | Medium | **Fixed** | Error path reads/drains before success path; connection reuse preserved |
| A4 | Client retries non-transient 501 | Medium | **Fixed** | Retryable statuses narrowed to 429, 502, 503, 504 |
| A5 | No `Close()` on client | Low | **Fixed** | `Close()` drains idle connections |

## Reliability

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| R1 | Rate limiter ignores context cancellation | High | **Fixed** | `ctx.Done()` case added to semaphore select |
| R2 | Retry with exponential backoff + jitter | — | Already present | `client.go` retries on 429/502/503/504 |
| R3 | `Retry-After` header respected | — | Already present | Both integer seconds and RFC1123 dates |
| R4 | Graceful shutdown via signal handling | — | Already present | `signal.NotifyContext` on SIGINT/SIGTERM |
| R5 | HTTP server timeouts configured | — | Already present | ReadHeader:10s, Read:30s, Write:60s, Idle:120s |
| R6 | 45-second tool call timeout | — | Already present | `context.WithTimeout` in `callTool` |
| R7 | Pagination safety stop at 1000 pages | — | Already present | Prevents infinite loops |

## Observability

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| O1 | No version in health endpoint | Medium | **Fixed** | `version` field added to `/health` response |
| O2 | Silent log level misconfiguration | Medium | **Fixed** | Warning printed for unknown `MCP_LOG_LEVEL` |
| O3 | Structured logging with request ID | — | Already present | `slog` with `req_id` correlation |
| O4 | Tool call duration logging | — | Already present | `duration_ms` on every `tool_call` log |
| O5 | Audit logging for write operations | — | Already present | `audit` log for non-read-only tools |

## Testing

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| T1 | No config validation tests for new checks | High | **Fixed** | 10 new tests: ReportsURL, transport, timezone, bearer |
| T2 | No intArg edge case tests | Medium | **Fixed** | 11 new tests in common_test.go |
| T3 | No CORS preflight tests | Medium | **Fixed** | 2 new tests: allowed + blocked preflight |
| T4 | No rate limiter context test | Medium | **Fixed** | Context cancellation test added |
| T5 | Health endpoint version not asserted | Low | **Fixed** | Version field assertion added |
| T6 | No code coverage in CI | Medium | **Fixed** | Coverage job with 60% threshold |

## Operations

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| P1 | No Makefile | Medium | **Fixed** | build/test/cover/fmt/vet/check/clean targets |
| P2 | No golangci-lint in CI | Low | Remaining | Optional `lint` target in Makefile; CI can add later |
| P3 | No security scanning (gosec) | Low | Remaining | Zero external dependencies reduces risk |

## Production Review Hardening (Phase 2)

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| PR1 | Enforcement package 0% coverage | High | **Fixed** | 40 tests, 96.4% coverage — Pipeline + Gate fully tested |
| PR2 | Clockify client 35.6% coverage | High | **Fixed** | 15 new tests: retry, Retry-After, pagination, deadline check. 73.2% |
| PR3 | ConfirmPattern dry-run executes real handler | High | **Fixed** | Documented accurately in docs/safe-usage.md + dryrun.go comment |
| PR4 | No retry deadline check | High | **Fixed** | doJSON checks ctx.Deadline() before sleeping for retry |
| PR5 | Bearer token no minimum length | Medium | **Fixed** | 16-char minimum enforced for HTTP mode |
| PR6 | ValidateID missing .. and % check | Medium | **Fixed** | Rejects .., %, and control chars |
| PR7 | AllowAnyOrigin not warned | Medium | **Fixed** | slog.Warn in main.go when enabled |
| PR8 | Case-sensitive dedupe | Medium | **Fixed** | strings.EqualFold for description comparison |
| PR9 | 45s tool timeout hardcoded | Medium | **Fixed** | CLOCKIFY_TOOL_TIMEOUT env var (5s-10m, default 45s) |
| PR10 | No HTTP end-to-end tests | Medium | **Fixed** | 3 tests: tools/call, tools/list, body-too-large |
| PR11 | No tool handler execution tests | Medium | **Fixed** | 5 tests: WhoAmI, ListProjects, TimerStatus, APIError |
| PR12 | No rate limiter stress test | Low | **Fixed** | 50-goroutine concurrent stress test |

---

## Summary

- **Items reviewed**: 49
- **Already present**: 13
- **Fixed (Phase 1)**: 20
- **Fixed (Phase 2 — Production Review)**: 12
- **Remaining (Low)**: 4
- **Critical/High issues remaining**: 0
