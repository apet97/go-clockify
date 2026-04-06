# GOCLMCP Production Readiness Plan

**Audit date:** 2026-04-06
**Auditor:** Claude Opus 4.6 deep repo audit
**Repo:** github.com/apet97/go-clockify (Go 1.25.0, stdlib-only)
**Version:** 0.3.0
**Scope:** 124 tools (33 Tier 1, 91 Tier 2), stdio + HTTP transports

---

## 1. Executive Summary

GOCLMCP is a well-engineered MCP server with clean layered architecture, zero external dependencies, and strong fundamentals in authentication, rate limiting, policy enforcement, retry semantics, and release hygiene. The codebase is beyond MVP — most packages have >75% test coverage, the enforcement pipeline is correctly composed, and the release process includes SBOM generation, cosign signing, and multi-platform builds.

However, this audit identified **3 confirmed bugs**, **4 dead config/code items**, and **3 operational gaps** that must be resolved before production deployment, particularly in HTTP mode. The most impactful finding is that **dry-run is broken for all non-destructive write tools** — the schema advertises the feature but the enforcement layer rejects it.

**Test results (2026-04-06):**
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `gofmt -l .` — PASS (no formatting issues)
- `go test -race ./...` — PASS (no race conditions detected)
- Total coverage: **44.9%** (dragged down by `tools` at 29.4% and `cmd` at 0%)

---

## 2. What Is Already Production-Grade

These areas are strong and should NOT be churned unless a specific defect is found:

| Area | Why It's Good | Key Files |
|------|--------------|-----------|
| **Stdlib-only** | Zero supply chain risk. No external dependencies to audit, update, or worry about. | `go.mod` |
| **Layered architecture** | Protocol core (`mcp/`) has zero domain imports. Enforcement is cleanly composed via interfaces. | `mcp/server.go`, `mcp/types.go`, `enforcement/enforcement.go` |
| **Enforcement pipeline** | Correct ordering (policy → rate limit → dry-run → handler → truncation). Release semantics are sound in all code paths. | `enforcement/enforcement.go:46-77` |
| **Retry/backoff** | Exponential backoff with jitter (250ms base), `Retry-After` header support (both integer and RFC1123), context deadline checks before sleeping, correct retryable status set (429, 502, 503, 504). | `clockify/client.go:204-238` |
| **Connection pool** | Well-tuned: MaxIdleConns=100, MaxIdleConnsPerHost=10, MaxConnsPerHost=20, IdleTimeout=90s. | `clockify/client.go:37-42` |
| **Rate limiter** | Race-safe dual control: buffered-channel semaphore + atomic window counter with double-checked locking for resets. | `ratelimit/ratelimit.go` |
| **Name resolution** | Fail-closed on ambiguity (multiple matches rejected), input validation (path traversal, null bytes, control chars, URL injection), email-aware user resolution. | `resolve/resolve.go:11-24,87-136` |
| **Policy enforcement** | Deny-first, introspection always-allowed (except explicit deny override), group control with deny/allow-list. | `policy/policy.go` |
| **Config validation** | HTTPS enforcement on base/reports URLs, bearer token min 16 chars, transport whitelist, tool timeout bounds (5s-10m), loopback exemption for HTTP. | `config/config.go` |
| **HTTP security** | Constant-time bearer comparison (`crypto/subtle`), `MaxBytesReader` for body limits, `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `ReadHeaderTimeout` against slowloris. | `mcp/transport_http.go:96-182` |
| **Test suite** | 27 test files across 16 packages. Golden tests verify exact tool counts. Integration tests cover full MCP handshake (20 scenarios). Per-domain handler tests with httptest mocks. | `tools/golden_test.go`, `mcp/integration_test.go` |
| **Release process** | Multi-platform matrix (5 targets), SBOM via anchore/sbom-action, cosign keyless signing, SHA256 checksums, npm package publishing, smoke tests on release binaries. | `.github/workflows/release.yml` |
| **Dockerfile** | Multi-stage, `gcr.io/distroless/static-debian12` runtime, CGO_ENABLED=0, stripped binary, non-root user. | `deploy/Dockerfile` |
| **Structured logging** | `log/slog` to stderr only, stdout purity for protocol, JSON format option, configurable level, per-request audit entries. | `cmd/clockify-mcp/main.go:49-54` |
| **Dedup/overlap detection** | Three modes (warn/block/off), minute-granularity matching, same-project overlap checking, running-timer exclusion. | `dedupe/dedupe.go` |
| **Time parsing** | 13 input formats, UTC normalization, timezone-aware, input validation (hour/minute/second bounds). | `timeparse/timeparse.go` |
| **Live E2E tests** | Properly gated behind build tag AND env var. Read-only and mutating paths both tested. Cleanup via `t.Cleanup()`. | `tests/e2e_live_test.go` |

---

## 3. Confirmed Gaps

### 3.1 Critical — Correctness Bugs

#### C-1: Dry-run broken for non-destructive write tools [DONE]

**Files:** `internal/dryrun/dryrun.go:125-127`, `internal/enforcement/enforcement.go:64,91`
**Affected tools:** `clockify_add_entry`, `clockify_update_entry`, `clockify_stop_timer`, `clockify_log_time`, `clockify_find_and_update_entry`

**What happens:** These tools declare `dry_run` as a boolean parameter in their schemas (e.g., `registry.go:67,79,85`, `registry.go:55-56`) and their handlers check `dryrun.Enabled(args)` (e.g., `entries.go:186,281,319`, `timer.go:40`, `workflows.go:15`). However, the enforcement pipeline's `CheckDryRun` function tests `isDestructive` (line 125), which comes from `DestructiveHint` on the tool descriptor. Since these tools use `toolRW()` (not `toolDestructive()`), `DestructiveHint=false`, so `CheckDryRun` returns `NotDestructive`. The enforcement then returns an error at line 91: "dry_run is not supported for non-destructive tool: X".

**Why it matters:** Users/LLMs see `dry_run` in the tool schema and set it, expecting a preview. Instead they get an error. The handler-level fallback (`dryrun.Enabled`) is unreachable because enforcement intercepts first and deletes the flag.

**Fix:** Two options:
- **Option A (recommended):** In `dryrun.CheckDryRun`, change the `!isDestructive` branch to also check whether the tool handler internally supports dry-run (add a `supportsDryRun` field to ToolHints). For tools that support it, return a new action type that delegates to the handler's own dry-run logic.
- **Option B (simpler):** In `dryrun.CheckDryRun`, when `!isDestructive` and `dry_run=true`, do NOT consume the flag — return `(0, false)` so the enforcement passes through and the handler's own `dryrun.Enabled(args)` check runs normally.

**Test:** `go test ./internal/enforcement/... -run TestDryRun` — add a test case where a non-destructive write tool receives `dry_run: true` and verify the handler-level dry-run envelope is returned (not an error).

**Done:** `dry_run: true` on `clockify_add_entry` returns a preview envelope. No error.

---

#### C-2: `confirmTools` dry-run map is dead code [DONE]

**Files:** `internal/dryrun/dryrun.go:88-93`, `internal/tools/tier2_approvals.go:57,69`, `internal/tools/tier2_invoices.go` (send_invoice), `internal/tools/tier2_user_admin.go` (deactivate_user)

**What happens:** Four tools are in the `confirmTools` map: `clockify_send_invoice`, `clockify_approve_timesheet`, `clockify_reject_timesheet`, `clockify_deactivate_user`. All four are registered with `toolRW()`, not `toolDestructive()`. When `dry_run: true` is set, enforcement hits the `!isDestructive` branch (C-1 above) and returns `NotDestructive` error. The `confirmTools` check at `dryrun.go:128` is never reached.

**Note on the ConfirmPattern design:** The `ConfirmPattern` action (dryrun.go:65) calls the actual handler and wraps the result in a dry-run envelope — the mutation IS performed. The `WrapResult` says "No changes were made" which would be FALSE. If these tools were ever marked destructive and the confirm path became reachable, this would be a dangerous misrepresentation. However, since the path is currently dead code, no mutation occurs.

**Why it matters:** These tools advertise `dry_run` in their schemas and handlers support it (e.g., `tier2_approvals.go:176,208`), but the feature is broken. Additionally, if the `ConfirmPattern` path were ever made reachable, it would perform real mutations while claiming otherwise.

**Fix:**
1. Decide whether these tools should be `toolDestructive()` (which enables enforcement-level dry-run) or `toolRW()` (which relies on handler-level dry-run).
2. If `toolRW()`: apply the same fix as C-1 (pass through to handler).
3. If `toolDestructive()`: fix the `ConfirmPattern` action to NOT execute the handler — instead use MinimalFallback or PreviewTool with a GET counterpart.
4. Either way, remove or fix the ConfirmPattern action so `WrapResult("No changes were made")` is never used for a path that actually mutates.

**Test:** Add test cases in `enforcement_test.go` for each confirmTools entry verifying the expected dry-run behavior.

**Done:** `dry_run: true` on `clockify_approve_timesheet` either previews without mutating, or the tool is explicitly documented as not supporting dry-run (and `dry_run` removed from schema).

---

#### C-3: `CLOCKIFY_DRY_RUN` env var has no runtime effect [DONE]

**Files:** `internal/dryrun/dryrun.go:42-56`, `internal/enforcement/enforcement.go:64`

**What happens:** `ConfigFromEnv()` reads `CLOCKIFY_DRY_RUN` and stores it in `Config.Enabled`. The `Pipeline` struct holds `DryRun dryrun.Config`. But `BeforeCall` at line 64 calls `dryrun.CheckDryRun(name, args, hints.Destructive)` without ever checking `p.DryRun.Enabled`. Setting `CLOCKIFY_DRY_RUN=off` has zero effect.

**Why it matters:** The env var is documented in `--help` (main.go:226) and README as a functional toggle. Operators expect it to disable dry-run globally. It does nothing.

**Fix:** In `enforcement.go:BeforeCall`, check `p.DryRun.Enabled` before calling `CheckDryRun`. If disabled, skip dry-run entirely.

**Test:** Add a test in `enforcement_test.go`: set `DryRun.Enabled=false`, send a destructive tool call with `dry_run: true`, verify the flag is ignored and the handler runs normally (or the flag is passed through as a regular parameter).

**Done:** `CLOCKIFY_DRY_RUN=off` causes the enforcement to skip all dry-run interception.

---

### 3.2 High — Dead Config / Resource Leaks

#### H-1: `CLOCKIFY_REPORTS_URL` is dead config [DONE — REMOVED]

**Files:** `internal/config/config.go:22,58-63`, `cmd/clockify-mcp/main.go` (never references `cfg.ReportsURL`)

**What happens:** The `ReportsURL` field is parsed from `CLOCKIFY_REPORTS_URL`, trimmed, validated for HTTPS, and stored — but no code outside `config.go` ever reads it. The Clockify client is constructed with only `cfg.BaseURL` (main.go:87). Report tools like `DetailedReport` use the standard base URL.

**Why it matters:** Users may set `CLOCKIFY_REPORTS_URL` expecting it to be used for report API calls. It is silently ignored. Documentation claims it exists as a functional config option.

**Fix:** Either:
- **Wire it:** Pass `cfg.ReportsURL` to the Clockify client or tool service, and use it for report endpoints.
- **Remove it:** Delete the field from config, remove from `--help`, remove from README. Simpler if the reports API doesn't actually need a separate URL.

**Test:** If wired: test that report API calls use the reports URL. If removed: test that config parsing no longer accepts the env var.

**Done:** `CLOCKIFY_REPORTS_URL` either affects report API calls or is removed from config/docs.

---

#### H-2: `CLOCKIFY_TIMEZONE` is dead config [DONE — WIRED]

**Files:** `internal/config/config.go:23,67-71`, `internal/tools/common.go:191-200`

**What happens:** `CLOCKIFY_TIMEZONE` is parsed, validated via `time.LoadLocation`, and stored in `cfg.Timezone` — but `main.go` never passes it to any consumer. The `loadLocation` function in `common.go:191` takes a per-request timezone argument; if empty, it falls back to `time.Now().Location()` (process-local timezone), NOT `CLOCKIFY_TIMEZONE`.

**Why it matters:** Users may set `CLOCKIFY_TIMEZONE=America/New_York` expecting all time parsing to use that timezone. It is silently ignored. The server uses the system timezone instead.

**Fix:** Either:
- **Wire it:** Pass `cfg.Timezone` (as `*time.Location`) to the tool service, and use it as the default location in `loadLocation` when no per-request timezone is provided.
- **Remove it:** Delete from config, `--help`, and README.

**Test:** If wired: set `CLOCKIFY_TIMEZONE=America/New_York`, call a time-parsing tool without a timezone arg, verify the result uses New York time. If removed: test config parsing.

**Done:** `CLOCKIFY_TIMEZONE` either affects time parsing or is removed from config/docs.

---

#### H-3: `client.Close()` never called [DONE]

**File:** `cmd/clockify-mcp/main.go:87`

**What happens:** `clockify.NewClient(...)` is called at line 87. The client has a `Close()` method (clockify/client.go:56) that calls `httpClient.CloseIdleConnections()`. But `main.go` never calls `defer client.Close()`. When the process shuts down, idle connections in the transport pool are leaked.

**Why it matters:** In short-lived processes this is negligible. In long-running HTTP mode servers that may be restarted gracefully, leaked connections can accumulate if the process is reused (e.g., in test harnesses or libraries).

**Fix:** Add `defer client.Close()` after line 87 in `main.go`.

**Test:** Not directly testable in isolation (process exit cleans up anyway). Verify by code inspection.

**Done:** `defer client.Close()` exists after client creation.

---

#### H-4: `/ready` always returns OK in HTTP mode [DONE]

**Files:** `internal/mcp/transport_http.go:64-66,84-94`

**What happens:** `s.initialized.Store(true)` is called unconditionally at line 64 before `srv.Serve()`. The `/ready` handler checks `s.initialized.Load()` and returns `{"status":"ok"}`. Since initialized is always true, `/ready` always passes. It never checks whether the Clockify API is reachable, the API key is valid, or the workspace exists.

**Why it matters:** Kubernetes readiness probes will route traffic to an instance that cannot actually serve requests (e.g., Clockify API is down, API key is revoked). This defeats the purpose of readiness checks.

**Fix:** Enhance `/ready` to make a lightweight Clockify API call (e.g., `GET /api/v1/user` with a short timeout). Cache the result for 10-30 seconds to avoid per-probe API calls. Return 503 if the upstream is unreachable.

**Test:** In `transport_http_test.go`, add a test where the mock Clockify server returns 500, and verify `/ready` returns 503.

**Done:** `/ready` returns 503 when the Clockify API is unreachable; returns 200 when healthy.

---

### 3.3 Medium

#### M-1: CORS missing `Vary: Origin` header [DONE]

**File:** `internal/mcp/transport_http.go:115`

**What happens:** When `allowAnyOrigin=false`, the server reflects the request's `Origin` header as `Access-Control-Allow-Origin`. Per HTTP caching specs, this requires `Vary: Origin` to prevent intermediate caches from serving a response with the wrong ACAO header to a different origin.

**Fix:** Add `w.Header().Set("Vary", "Origin")` when reflecting the origin (not when using `*`).

**Test:** Existing CORS tests + verify `Vary: Origin` header is present in response.

---

#### M-2: `srv.Shutdown()` error silently discarded [DONE]

**File:** `internal/mcp/transport_http.go:54`

**What happens:** `srv.Shutdown(shutdownCtx)` return value is not checked. If shutdown times out, no error is logged.

**Fix:** Log the error: `if err := srv.Shutdown(shutdownCtx); err != nil { slog.Error("http_shutdown_error", "error", err) }`

---

#### M-3: Bootstrap is not a security boundary — hidden tools callable [DONE — DOCUMENTED]

**File:** `internal/enforcement/enforcement.go:46`

**What happens:** `BeforeCall` checks policy but NOT bootstrap visibility. A tool hidden by bootstrap mode (e.g., `Minimal` mode hides most Tier 1 tools) can still be called if the client knows the tool name. Bootstrap only controls `tools/list` discovery.

**Why it matters:** If an operator sets `CLOCKIFY_BOOTSTRAP_MODE=minimal` expecting to restrict tool access, tools are only hidden from discovery but remain callable.

**Fix (document, not code):** This is intentional design (lazy activation requires callable-but-hidden tools). Add a clear note in `docs/safe-usage.md` and README: "Bootstrap controls tool discovery, not access. Use policy modes to restrict tool execution."

---

#### M-4: List tools capped at 50 items without pagination [DEFERRED — schema changes affect golden tests; separate PR]

**Files:** `internal/tools/projects.go:18`, `clients.go:17`, `tags.go:17`, `tasks.go:25`, `users.go:53`

**What happens:** `ListProjects`, `ListClients`, `ListTags`, `ListTasks`, `ListUsers` all hardcode `page-size: 50` with no `page` parameter. Workspaces with >50 items silently lose data.

**Fix:** Add `page` and `page_size` parameters to these tools (matching `ListEntries` which already has them). Use the existing `helpers.PaginatedResult` for response envelopes.

**Test:** Add tests with mock responses containing >50 items. Verify pagination parameters are forwarded.

---

#### M-5: Report queries capped at 100 entries [DONE — truncation warning added]

**File:** `internal/tools/common.go:240` (`entryRangeQuery` uses `page-size: 100`)

**What happens:** All report tools that use `entryRangeQuery` will silently truncate results to 100 entries for a given date range. No indication is given to the user.

**Fix:** Either increase the limit, use `ListAll` pagination, or add a warning in the response when the result count equals the page size (indicating potential truncation).

---

#### M-6: `notifications/tools/list_changed` dropped in HTTP mode [DONE — DOCUMENTED]

**File:** `internal/mcp/server.go:336-339`

**What happens:** `notifyToolsChanged()` writes to `s.encoder`, which is only set in the stdio `Run()` method. In HTTP mode, `s.encoder` is nil, so the notification is silently dropped.

**Why it matters:** HTTP clients are never informed of tool list changes after group activation. They must poll `tools/list`.

**Fix (document):** This is inherent to HTTP request-response semantics (no persistent connection for push notifications). Document this limitation in `docs/http-transport.md`: "Tool list change notifications are not supported in HTTP mode. Clients should re-fetch tools/list after activating groups."

---

#### M-7: No staticcheck or golangci-lint in CI [DEFERRED — CI infra change; separate PR]

**File:** `.github/workflows/ci.yml`

**What happens:** CI runs `gofmt`, `go vet`, `go test`, and coverage check, but no static analysis beyond vet.

**Fix:** Add a `lint` job that runs `golangci-lint` with a config file. The Makefile already has a `lint` target (line 22) but it silently skips if golangci-lint is not installed.

---

#### M-8: GitHub Actions not SHA-pinned [DEFERRED — CI infra change; separate PR]

**Files:** `.github/workflows/ci.yml`, `.github/workflows/release.yml`

**What happens:** All actions use major version tags (`actions/checkout@v6`, `actions/setup-go@v7`, etc.) instead of SHA digests. A compromised action could inject malicious code.

**Fix:** Pin all third-party actions to full SHA digests. Add a comment with the version for readability.

---

#### M-9: `WriteResult` is dead code; docs claim it's used [DONE — REMOVED]

**Files:** `internal/helpers/helpers.go:53-66`, `CLAUDE.md:152`, `AGENTS.md:151`

**What happens:** `helpers.WriteResult` is defined and tested but never called by any handler. All handlers use `ok()` from `common.go`. CLAUDE.md says "Write tools use `helpers.WriteResult`" which is false.

**Fix:** Remove `WriteResult` and its tests, or migrate handlers to use it for consistency. Update CLAUDE.md/AGENTS.md either way.

---

#### M-10: Standard and Full policy modes are functionally identical [DONE — DOCUMENTED]

**File:** `internal/policy/policy.go:97`

**What happens:** Both `Standard` and `Full` fall through to `return true` in `IsAllowed` and `IsGroupAllowed`. There is no behavioral distinction.

**Fix (document):** Either remove `Full` mode, or reserve it for a future capability (e.g., bypassing dry-run defaults). Document the equivalence.

---

#### M-11: No fuzz tests for attack surfaces [DEFERRED — nice-to-have; separate PR]

**What's missing:** `timeparse.ParseDatetime`, `resolve.ValidateID`, and JSON-RPC request parsing are input-parsing surfaces that would benefit from fuzz testing. Go's built-in `testing.F` makes this low-effort.

**Fix:** Add `Fuzz*` functions in `timeparse_test.go`, `resolve_test.go`, and `server_test.go`.

---

### 3.4 Low

#### L-1: No upper bound on `MCP_HTTP_MAX_BODY` [DONE]

**File:** `internal/config/config.go:113-123`

A misconfigured value like `MCP_HTTP_MAX_BODY=99999999999` (~93GB) would be accepted. Add a reasonable ceiling (e.g., 50MB).

#### L-2: JSON-RPC `id` type not validated [DONE]

**File:** `internal/mcp/types.go:7`

The `id` field is `any`. A malicious client could send a large nested object as the ID, which would be echoed in every response. Add type validation (string, number, or null per JSON-RPC 2.0 spec).

#### L-3: JSON-RPC version not validated [DONE]

**File:** `internal/mcp/server.go:149`

The `req.JSONRPC` field is never checked against `"2.0"`. Minor spec compliance gap.

#### L-4: Truncation error silently swallowed [DEFERRED — low risk; existing behavior fails safe]

**File:** `internal/enforcement/enforcement.go:82`

`Truncate()` returns `(result, error)` but the error is discarded with `_`. If truncation fails, a partially-truncated or nil result could be returned.

#### L-5: Truncation mixes metadata into data arrays [DEFERRED — would break consumers; needs design]

**File:** `internal/truncate/truncate.go:209-211`

`reduceArrays` appends `{"_truncated": true, "_remaining": N}` to arrays. Consumers expecting homogeneous arrays will break.

#### L-6: Rate limiter window counter doesn't decrement on rejection [DEFERRED — fails safe]

**File:** `internal/ratelimit/ratelimit.go:107-110`

When a call passes the window pre-check (line 86) but fails the post-increment check (line 108), the counter stays incremented. Under high contention, this causes conservative over-counting and unnecessary rejections. Fails safe but wastes capacity.

#### L-7: `ListAll` pagination no safety ceiling [DEFERRED — 50k limit acceptable]

**File:** `internal/clockify/client.go:85-110`

The loop can accumulate up to 50,000 items (page-size 50 × 1000 pages) with no configurable ceiling. Add a max-items parameter or a reasonable default ceiling.

#### L-8: Fragile error string matching in SwitchProject [DONE]

**File:** `internal/tools/workflows.go:138`

Checks for "404" and "400" substrings in error messages. Should use typed errors from the Clockify client instead.

#### L-9: Missing `Access-Control-Max-Age` on preflight [DONE]

**File:** `internal/mcp/transport_http.go:120-124`

Without this header, browsers re-send preflight OPTIONS for every request. Add `Access-Control-Max-Age: 86400`.

#### L-10: CI coverage threshold too low [DEFERRED — requires tools package coverage investment]

**File:** `.github/workflows/ci.yml:75`

Threshold is 40%. Actual coverage is 44.9%. The `tools` package is at 29.4%. Raise to 55% and invest in tools package coverage.

#### L-11: No `govulncheck` in CI [DEFERRED — CI infra change; separate PR]

Even with zero external dependencies, stdlib vulnerabilities matter. Add `govulncheck ./...` to CI.

---

## 4. Suspected Gaps Needing Verification

These could not be fully confirmed during the audit and require manual testing or environment-specific verification:

| # | Suspicion | Why | How to Verify |
|---|-----------|-----|---------------|
| S-1 | `CLOCKIFY_INSECURE=1` may mislead users | It bypasses URL scheme validation but does NOT disable TLS certificate verification in the HTTP client. Users expecting to connect to self-signed certs will still get TLS errors. | Set `CLOCKIFY_INSECURE=1` with a self-signed HTTPS endpoint and verify behavior. |
| S-2 | `DetailedReport` may use a different API endpoint | Some Clockify report endpoints use a separate `reports.clockify.me` host. If `DetailedReport` calls the wrong base URL, results may be wrong or 404. | Check Clockify API docs for the detailed report endpoint and compare with `reports.go`. |
| S-3 | Scanner goroutine leak on shutdown | In stdio mode, the scanner goroutine (`server.go:73-89`) may block on `scanner.Scan()` after context cancellation. In practice, process exit cleans this up, but in a library context it would leak. | Only matters if the server is used as a library. Low risk for CLI usage. |
| S-4 | Unbounded goroutine creation in stdio mode | Every `tools/call` spawns a goroutine (`server.go:114-126`). The rate limiter provides a concurrency semaphore, but goroutines are created before `BeforeCall`. Under heavy load, thousands of goroutines could be waiting on the semaphore. | Load test stdio mode with concurrent tool calls and monitor goroutine count. |
| S-5 | `examples/docker-compose.env` referenced correctly | README line 278 references `examples/docker-compose.env`. File exists at repo root `examples/docker-compose.env`. Verify the path is correct relative to the Docker Compose working directory. | `ls examples/docker-compose.env` — confirmed present. May need relative path adjustment in compose file. |

---

## 5. Prioritized Remediation Plan

### Phase 0: Correctness / Security Blockers

*Must be fixed before any production deployment.*

| # | Finding | Effort | Files |
|---|---------|--------|-------|
| 0.1 | Fix dry-run for non-destructive write tools (C-1) | 2h | `dryrun/dryrun.go`, `enforcement/enforcement.go`, `enforcement/enforcement_test.go` |
| 0.2 | Fix `confirmTools` dead code — decide toolRW vs toolDestructive, fix ConfirmPattern (C-2) | 3h | `dryrun/dryrun.go`, `tier2_approvals.go`, `tier2_invoices.go`, `tier2_user_admin.go`, `enforcement/enforcement_test.go` |
| 0.3 | Wire `CLOCKIFY_DRY_RUN` env var to enforcement (C-3) | 30m | `enforcement/enforcement.go`, `enforcement/enforcement_test.go` |
| 0.4 | Add `defer client.Close()` (H-3) | 5m | `cmd/clockify-mcp/main.go` |
| 0.5 | Fix CORS `Vary: Origin` header (M-1) | 10m | `mcp/transport_http.go`, `mcp/transport_http_test.go` |
| 0.6 | Log `srv.Shutdown()` error (M-2) | 5m | `mcp/transport_http.go` |

### Phase 1: Operational Hardening

*Required for HTTP/Kubernetes deployments.*

| # | Finding | Effort | Files |
|---|---------|--------|-------|
| 1.1 | Enhance `/ready` with upstream health check (H-4) | 2h | `mcp/transport_http.go`, `mcp/transport_http_test.go` |
| 1.2 | Wire or remove `CLOCKIFY_REPORTS_URL` (H-1) | 1-3h | `config/config.go`, `cmd/clockify-mcp/main.go`, `tools/reports.go` or remove from config/docs |
| 1.3 | Wire or remove `CLOCKIFY_TIMEZONE` (H-2) | 1-2h | `config/config.go`, `cmd/clockify-mcp/main.go`, `tools/common.go` or remove from config/docs |
| 1.4 | Add pagination to list tools (M-4) | 3h | `tools/projects.go`, `clients.go`, `tags.go`, `tasks.go`, `users.go`, tests |
| 1.5 | Address report query 100-entry cap (M-5) | 1h | `tools/common.go:240`, report tool tests |
| 1.6 | Add `Access-Control-Max-Age` to preflight (L-9) | 5m | `mcp/transport_http.go` |
| 1.7 | Add `MCP_HTTP_MAX_BODY` upper bound (L-1) | 15m | `config/config.go`, `config/config_test.go` |

### Phase 2: Test & CI Confidence

| # | Finding | Effort | Files |
|---|---------|--------|-------|
| 2.1 | Add golangci-lint to CI (M-7) | 1h | `.github/workflows/ci.yml`, `.golangci.yml` (new) |
| 2.2 | Pin GitHub Actions to SHA digests (M-8) | 30m | `.github/workflows/ci.yml`, `.github/workflows/release.yml` |
| 2.3 | Add fuzz tests for timeparse, resolve, JSON-RPC (M-11) | 3h | `timeparse/timeparse_test.go`, `resolve/resolve_test.go`, `mcp/server_test.go` |
| 2.4 | Raise coverage threshold to 55% (L-10) | 30m | `.github/workflows/ci.yml` |
| 2.5 | Increase `tools` package coverage from 29.4% (L-10) | 4h | `tools/*_test.go` (new/expanded) |
| 2.6 | Add `govulncheck` to CI (L-11) | 30m | `.github/workflows/ci.yml` |
| 2.7 | Add JSON-RPC `id` type validation (L-2) | 30m | `mcp/server.go`, `mcp/server_test.go` |
| 2.8 | Add JSON-RPC version validation (L-3) | 15m | `mcp/server.go`, `mcp/server_test.go` |

### Phase 3: DX / Docs / Polish

| # | Finding | Effort | Files |
|---|---------|--------|-------|
| 3.1 | Remove `WriteResult` dead code, update docs (M-9) | 30m | `helpers/helpers.go`, `helpers/helpers_test.go`, `CLAUDE.md`, `AGENTS.md` |
| 3.2 | Document bootstrap vs policy security boundary (M-3) | 30m | `docs/safe-usage.md`, README.md |
| 3.3 | Document HTTP mode notification limitation (M-6) | 15m | `docs/http-transport.md` |
| 3.4 | Document or remove Standard vs Full equivalence (M-10) | 15m | `policy/policy.go`, `docs/safe-usage.md` |
| 3.5 | Handle truncation error instead of swallowing (L-4) | 15m | `enforcement/enforcement.go:82` |
| 3.6 | Fix fragile error string matching in SwitchProject (L-8) | 30m | `tools/workflows.go:138` |
| 3.7 | Document `CLOCKIFY_INSECURE` behavior clearly (S-1) | 15m | README.md, `docs/http-transport.md` |

---

## 6. File-by-File Change Map

| File | Changes |
|------|---------|
| `cmd/clockify-mcp/main.go` | Add `defer client.Close()`. Wire `cfg.ReportsURL` and/or `cfg.Timezone` to consumers (or remove them from config). |
| `internal/config/config.go` | Remove `ReportsURL`/`Timezone` fields if unwired. Add max-body upper bound. |
| `internal/dryrun/dryrun.go` | Fix `CheckDryRun` to handle non-destructive write tools with `dry_run` support. Decide on ConfirmPattern behavior. |
| `internal/enforcement/enforcement.go` | Check `p.DryRun.Enabled` before calling `CheckDryRun`. Handle truncation error at line 82. |
| `internal/mcp/transport_http.go` | Add `Vary: Origin`. Log shutdown error. Add `Access-Control-Max-Age`. Enhance `/ready` with health check. |
| `internal/mcp/server.go` | Add JSON-RPC `id` type validation. Add version validation. |
| `internal/tools/projects.go` | Add `page`/`page_size` parameters. |
| `internal/tools/clients.go` | Add `page`/`page_size` parameters. |
| `internal/tools/tags.go` | Add `page`/`page_size` parameters. |
| `internal/tools/tasks.go` | Add `page`/`page_size` parameters. |
| `internal/tools/users.go` | Add `page`/`page_size` parameters. |
| `internal/tools/common.go` | Address 100-entry cap in `entryRangeQuery`. |
| `internal/tools/workflows.go` | Replace string-based error matching with typed errors. |
| `internal/tools/tier2_approvals.go` | Change `toolRW` to `toolDestructive` or adjust dry-run handling. |
| `internal/tools/tier2_invoices.go` | Same for `send_invoice`. |
| `internal/tools/tier2_user_admin.go` | Same for `deactivate_user`. |
| `internal/helpers/helpers.go` | Remove `WriteResult` (dead code). |
| `.github/workflows/ci.yml` | Add lint job, govulncheck, raise coverage threshold, pin actions. |
| `.github/workflows/release.yml` | Pin actions to SHA. |
| `CLAUDE.md` | Fix "Write tools use helpers.WriteResult" claim. |
| `AGENTS.md` | Same fix. |
| `docs/safe-usage.md` | Document bootstrap vs policy boundary. Document Standard/Full equivalence. |
| `docs/http-transport.md` | Document notification limitation. Document INSECURE behavior. |

---

## 7. Test Strategy & CI Hardening

### Current State

| Package | Coverage | Assessment |
|---------|----------|------------|
| `enforcement` | 96.4% | Excellent |
| `config` | 92.9% | Excellent |
| `helpers` | 90.9% | Good (but tests dead code WriteResult) |
| `timeparse` | 90.4% | Good; add fuzz tests |
| `truncate` | 87.0% | Good |
| `policy` | 87.1% | Good |
| `bootstrap` | 86.7% | Good |
| `dryrun` | 85.4% | Good; needs tests for C-1/C-2 fixes |
| `resolve` | 79.5% | Good; add fuzz tests |
| `ratelimit` | 76.1% | Good |
| `mcp` | 74.9% | Good; add fuzz for JSON-RPC parsing |
| `clockify` | 73.2% | Adequate |
| `dedupe` | 64.1% | Adequate; add edge case tests |
| `tools` | **29.4%** | **Needs investment** — only 10 test files for 25 impl files |
| `cmd` | **0%** | Expected for main; consider smoke test |

### Required New Tests

**Phase 0:**
- `enforcement_test.go`: Non-destructive write tool + `dry_run: true` → preview (not error)
- `enforcement_test.go`: ConfirmPattern tools with correct destructive annotations
- `enforcement_test.go`: `DryRun.Enabled=false` skips interception

**Phase 1:**
- `transport_http_test.go`: `/ready` returns 503 when upstream down
- `transport_http_test.go`: `Vary: Origin` header present
- Per-list-tool tests: pagination parameters forwarded, >50 items returned

**Phase 2:**
- `timeparse_test.go`: `FuzzParseDatetime`
- `resolve_test.go`: `FuzzValidateID`
- `server_test.go`: `FuzzJSONRPCParse`
- Expanded `tools/*_test.go` for uncovered Tier 1 handlers
- `ci.yml`: `golangci-lint`, `govulncheck`, raised coverage threshold

### CI Pipeline Target State

```yaml
jobs:
  fmt:        gofmt -l .
  vet:        go vet ./...
  lint:       golangci-lint run           # NEW
  vulncheck:  govulncheck ./...           # NEW
  build:      go build ./...
  test:       go test -race -count=1 -timeout 120s ./...
  coverage:   go test -coverprofile → threshold 55%  # RAISED from 40%
  fuzz:       go test -fuzz=. -fuzztime=30s ./internal/timeparse ./internal/resolve  # NEW
  test-http:  HTTP smoke test (existing)
```

---

## 8. Release/Operations Hardening

### Current Release Strengths (Keep)

- Multi-platform matrix (darwin-arm64/amd64, linux-amd64/arm64, windows-amd64)
- SBOM generation (SPDX JSON via anchore/sbom-action)
- Sigstore/Fulcio keyless signing via cosign
- SHA256 checksums
- Release binary smoke tests on native platforms
- npm package publishing with platform-specific optional dependencies

### Recommended Improvements

| # | Change | Why |
|---|--------|-----|
| R-1 | Pin all GitHub Actions to SHA digests | Supply chain hardening |
| R-2 | Add `govulncheck` to CI | Stdlib vulnerability detection |
| R-3 | Add golangci-lint to CI | Catch real bugs (nil pointer, unreachable code, printf format) |
| R-4 | Add CHANGELOG.md | Release notes beyond auto-generated PR summaries |
| R-5 | Add Dockerfile HEALTHCHECK (if using Docker directly, not K8s) | Container orchestration awareness |
| R-6 | Document TLS requirement in SECURITY.md | The HTTP transport has no TLS; relies on external proxy. This should be explicit. |

---

## 9. Validation Matrix

This matrix covers all critical behaviors that must be validated after remediation:

| Category | Test | How to Validate | Current Status |
|----------|------|-----------------|----------------|
| **Protocol** | JSON-RPC 2.0 compliance | `mcp/integration_test.go` — init, tools/list, tools/call, ping, error codes | PASS |
| **Protocol** | Unknown method → -32601 | `mcp/integration_test.go:TestUnknownMethod` | PASS |
| **Protocol** | Invalid JSON → -32700 | `mcp/integration_test.go:TestInvalidJSON` | PASS |
| **Protocol** | tools/call before init → -32002 | `mcp/integration_test.go:TestToolsCallBeforeInit` | PASS (stdio only; HTTP auto-inits) |
| **Stdio** | Clean shutdown on SIGTERM | Manual test: `kill -TERM <pid>`, verify clean exit | NEEDS VERIFICATION |
| **Stdio** | Large response handling | Integration test with large result → truncation | PASS |
| **HTTP** | Bearer auth (valid/invalid/missing) | `transport_http_test.go` | PASS |
| **HTTP** | CORS (allowed/blocked/preflight) | `transport_http_test.go` | PASS |
| **HTTP** | Body size limit (413) | `transport_http_test.go:TestBodyTooLarge` | PASS |
| **HTTP** | `/health` returns version | `transport_http_test.go` | PASS |
| **HTTP** | `/ready` reflects dependency health | `transport_http_test.go` | **FAIL** — always returns OK |
| **HTTP** | `Vary: Origin` on CORS responses | Not tested | **MISSING** |
| **HTTP** | Security headers on all responses | Partial — only tested on /health | **INCOMPLETE** |
| **Policy** | `read_only` blocks writes | `mcp/integration_test.go`, `policy/policy_test.go` | PASS |
| **Policy** | `safe_core` allows curated writes | `policy/policy_test.go` | PASS |
| **Policy** | Denied tools blocked | `mcp/integration_test.go`, `policy/policy_test.go` | PASS |
| **Policy** | Group deny/allow-list | `policy/policy_test.go` | PASS |
| **Policy** | Introspection always allowed | `policy/policy_test.go` | PASS |
| **Dry-run** | Destructive tool → PreviewTool | `enforcement/enforcement_test.go` | PASS |
| **Dry-run** | Destructive tool → MinimalFallback | `enforcement/enforcement_test.go` | PASS |
| **Dry-run** | Non-destructive write + dry_run → preview | Not tested | **FAIL** — returns error |
| **Dry-run** | ConfirmPattern tools → safe behavior | Not tested | **FAIL** — dead code path |
| **Dry-run** | `CLOCKIFY_DRY_RUN=off` disables | Not tested | **FAIL** — no effect |
| **Rate limit** | Concurrency limit enforced | `ratelimit/ratelimit_test.go` | PASS |
| **Rate limit** | Window limit enforced | `ratelimit/ratelimit_test.go` | PASS |
| **Rate limit** | Concurrent stress (50 goroutines) | `ratelimit/ratelimit_test.go` | PASS |
| **Truncation** | Large results truncated | `enforcement/enforcement_test.go` | PASS |
| **Truncation** | Token budget respected | `truncate/truncate_test.go` | PASS |
| **Activation** | Tier 2 group activation | `mcp/activation_integration_test.go` | PASS |
| **Activation** | Policy blocks group activation | `mcp/integration_test.go` | PASS |
| **Resolution** | Ambiguous name → error | `resolve/resolve_test.go` | PASS |
| **Resolution** | 24-char hex passthrough | `resolve/resolve_test.go` | PASS |
| **Resolution** | Path traversal rejected | `resolve/resolve_test.go` | PASS |
| **Resolution** | Email-aware user lookup | `resolve/resolve_test.go` | PASS |
| **Dedup** | Duplicate detection | `dedupe/dedupe_test.go` | PASS |
| **Dedup** | Overlap detection | `dedupe/dedupe_test.go` | PASS |
| **Retry** | 429 → retry with backoff | `clockify/client_test.go` | PASS |
| **Retry** | Retry-After header respected | `clockify/client_test.go` | PASS |
| **Retry** | Context cancel during backoff | `clockify/client_test.go` | PASS |
| **Retry** | Non-retryable (401/404) → no retry | `clockify/client_test.go` | PASS |
| **Golden** | Tier 1 = 33 tools | `tools/golden_test.go` | PASS |
| **Golden** | Tier 2 = 91 tools, 11 groups | `tools/golden_test.go` | PASS |
| **Golden** | Total = 124 tools | `tools/golden_test.go` | PASS |
| **Live E2E** | Read-only operations | `tests/e2e_live_test.go` (gated) | PASS (when enabled) |
| **Live E2E** | Mutating operations + cleanup | `tests/e2e_live_test.go` (gated) | PASS (when enabled) |

---

## 10. Definition of Done

The server is production-ready when ALL of the following are true:

- [ ] `dry_run: true` on non-destructive write tools returns a preview envelope (not an error)
- [ ] `confirmTools` are either properly destructive-annotated or removed from the map
- [ ] `CLOCKIFY_DRY_RUN=off` disables enforcement-level dry-run interception
- [ ] `CLOCKIFY_REPORTS_URL` is either wired to report API calls or removed from config/docs
- [ ] `CLOCKIFY_TIMEZONE` is either wired to time parsing or removed from config/docs
- [ ] `defer client.Close()` is called after client creation
- [ ] `/ready` returns 503 when Clockify API is unreachable
- [ ] CORS responses include `Vary: Origin`
- [ ] `srv.Shutdown()` error is logged
- [ ] CI includes golangci-lint and govulncheck
- [ ] GitHub Actions are SHA-pinned
- [ ] Coverage threshold is ≥55% and actual coverage exceeds it
- [ ] `tools` package coverage is ≥50%
- [ ] All validation matrix items marked FAIL or MISSING are resolved
- [ ] `go test -race ./...` passes
- [ ] `go vet ./...` passes
- [ ] `gofmt -l .` produces no output
- [ ] Dead code (`WriteResult`, `DryRun.Config.Enabled`) is removed
- [ ] CLAUDE.md/AGENTS.md claims match code reality
- [ ] docs accurately reflect behavior (bootstrap boundary, HTTP notifications, INSECURE behavior)

---

## 11. Suggested Execution Order

```
Phase 0 (Day 1-2):  Correctness / security blockers
  0.1 → 0.2 → 0.3  (dry-run fixes, interdependent)
  0.4               (client.Close, trivial)
  0.5 → 0.6         (HTTP fixes, quick)

Phase 1 (Day 3-5):  Operational hardening
  1.1               (/ready health check)
  1.2 + 1.3         (dead config — decide wire or remove)
  1.4               (list tool pagination)
  1.5 + 1.6 + 1.7   (quick fixes)

Phase 2 (Day 6-8):  Test & CI
  2.1 + 2.2         (CI hardening)
  2.3               (fuzz tests)
  2.4 + 2.5         (coverage)
  2.6 + 2.7 + 2.8   (small additions)

Phase 3 (Day 9-10): DX / Docs / Polish
  3.1 → 3.7         (all independent, can parallelize)
```

**Total estimated effort:** 25-35 hours across 10 working days.

**Critical path:** Phase 0 items 0.1-0.3 (dry-run fixes) are the highest-priority blockers and should be done first as a single coherent change.
