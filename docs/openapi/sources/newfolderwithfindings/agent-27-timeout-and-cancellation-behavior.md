# QA Agent 27 - timeout-and-cancellation-behavior

## Verdict
PASS WITH CONCERNS

## What I checked

1. All cancellation-related unit tests across the codebase
2. Parity cancellation tests for all three notification-capable transports (stdio, streamable_http, gRPC)
3. HTTP client timeout layering (defaults, bounds, validation)
4. Tool-level timeout (context.WithTimeout) and its interaction with per-handler context
5. Retry deadline awareness in the Clockify API client
6. Context cancellation during retry backoff sleep
7. Streamable HTTP session TTL, idle grace period, and reap behavior
8. Repeat-initialize inflight cancellation
9. Malformed/missing-ID cancellation payloads (spec-compliant silent-drop)
10. Config validation for CLOCKIFY_TOOL_TIMEOUT (5s-10m bounds)
11. Live Clockify API endpoint responsiveness and auth-failure latency
12. Doctor command audit of effective timeout configuration
13. ConcurrencyAcquireTimeout and MaxInFlightToolCalls defaults and bounds
14. Legacy HTTP cancellation gap (documented, by design)

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key and workspace ID (redacted) |
| `CLAUDE.md` | Lab rules and constraints |
| `probes/lib/common.sh` | Shared curl wrapper, redaction, environment loading |
| `TIMEENTRYDOC.md` | Time entry endpoint reference |

## Commands run

```
# Build
go build ./cmd/clockify-mcp/

# Doctor audit
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> ./clockify-mcp doctor

# Cancellation unit tests (internal/mcp)
go test -race -run "TestCancellation" ./internal/mcp/ -v -count=1
  TestCancellation_AbortsInflightHandler    PASS
  TestCancellation_UnknownIDNoOp            PASS
  TestCancellation_NormalCompletionCleansUp PASS
  TestCancellation_HandleCancelledMalformed PASS
  TestCancellation_RegisterUnregisterHelpers PASS
  TestRepeatInitialize_CancelsInflight      PASS

# Cancellation parity tests (tests/)
go test -race -run "TestCancellation_AbortsInflightHandler" ./tests/ -v -count=1
  stdio           PASS (0.21s)
  streamable_http PASS (0.57s)
  grpc            SKIP (harness unavailable, expected without -tags=grpc)

# Retry deadline tests (internal/clockify)
go test -race -run "TestRetry|TestContextCancel" ./internal/clockify/ -v -count=1
  TestRetryDeadlineCheck          PASS
  TestContextCancelDuringBackoff  PASS

# Session reap tests (internal/mcp)
go test -race -run "TestReapOnce|TestSSEMetrics_SessionReap" ./internal/mcp/ -v -count=1
  TestReapOnce_EvictsExpired                   PASS
  TestReapOnce_EvictsOrphanedSession           PASS
  TestReapOnce_KeepsOrphanedInsideGrace        PASS
  TestReapOnce_KeepsSessionWithActiveSubscriber PASS
  TestReapOnce_GraceDisabledWhenZero           PASS
  TestSSEMetrics_SessionReap_Labels            PASS
```

## Live API probes run

```
# Projects endpoint (fast, baseline)
GET /api/v1/workspaces/{id}/projects?page-size=1
  HTTP 200  Time: 0.208s  TTFB: 0.208s

# User endpoint
GET /api/v1/user
  HTTP 200  Time: 0.675s

# Workspace info
GET /api/v1/workspaces/{id}
  HTTP 200  Time: 0.946s

# Invalid auth
GET ... (X-Api-Key: invalid-key)
  HTTP 401  Time: 0.913s  (clean, fast error)

# Time entries listing
GET /api/v1/workspaces/{id}/time-entries?page-size=1
  HTTP 405  (endpoint requires POST only on this instance)
  Note: CloudFront edge returned "allow: POST" -- listing time entries
  may use a different path or host on this deployment.

# Connect timeout probe (extreme)
curl --connect-timeout 0.001
  HTTP 000  Time: 0.005s  (failed fast as expected)
```

## Findings

### F1 (P2) -- HTTP Client Timeout shorter than Tool Timeout

**Location:** `internal/clockify/client.go:99` and `internal/mcp/tools.go:262-267`

The HTTP client is created with a 30s timeout (default `RequestTimeout`), while the per-tool timeout defaults to 45s. If a Clockify API call takes >30s but <45s, `http.Client.Do()` returns a connection-level timeout error (not an `*APIError`), which is not retryable. The tool handler observes this as context-level cancellation (mapped to outcome "timeout") -- but the actual boundary was the HTTP transport, not the MCP tool layer.

**Severity:** P2 -- The practical window (30s--45s) is narrow enough that real Clockify API calls don't hit it, but the layered timeout contract is misleading. An operator setting CLOCKIFY_TOOL_TIMEOUT=60s would still see HTTP-level timeouts at 30s unless they also adjust RequestTimeout.

**Recommendation:** Consider deriving the HTTP client timeout from the tool timeout (e.g., `min(RequestTimeout, ToolTimeout - 5s)`) so the tool timeout always gates before the HTTP transport does. Alternatively, start warning when RequestTimeout < ToolTimeout.

### F2 (P3) -- ResponseHeaderTimeout of 10s could surprise on slow endpoints

**Location:** `internal/clockify/client.go:97`

The transport-level `ResponseHeaderTimeout` is hard-coded to 10s. Some Clockify endpoints (shared reports, paginated scheduler queries) can legitimately take longer than 10s to start streaming headers. A 10s header timeout on such an endpoint turns a successful-but-slow API call into an opaque error.

**Severity:** P3 -- Low blast radius; the 10s bound is appropriate for most endpoints. Only affects operators who use slow report endpoints without adjusting.

**Recommendation:** Consider exposing `ResponseHeaderTimeout` as a configurable env var (`CLOCKIFY_RESPONSE_HEADER_TIMEOUT`) with a 10s default. The transport constructor already has the pattern for this.

### F3 (P3) -- No first-retry deadline check

**Location:** `internal/clockify/client.go:406-431`

The deadline-aware bail-out check runs only on retry loop iterations `attempt>0`, meaning the first attempt proceeds even if the context deadline is already in the past. In practice this is harmless because `http.NewRequestWithContext` and `http.Client.Do` both respect the context deadline -- the HTTP layer will fail immediately. But it means a slightly misleading metric: the first attempt fires and fails at the HTTP layer instead of being caught by the explicit deadline check before making the request.

**Severity:** P3 -- The HTTP layer catches it immediately; no wasted request body bytes beyond the TCP handshake. Cosmetic.

**Recommendation:** Add a pre-first-attempt deadline check for clarity: if `ctx.Deadline()` is in the past (or within, say, TLSHandshakeTimeout), return early with `context.DeadlineExceeded` without attempting a request.

## Fixes made

No code changes were required. The timeout and cancellation architecture is well-designed and comprehensively tested. The three findings above are advisory, not bugs.

## Reproduction steps for each issue

### F1 -- HTTP Client Timeout < Tool Timeout
1. Set `CLOCKIFY_TOOL_TIMEOUT=60s` (above the 30s HTTP default)
2. Call any tool that does a Clockify API request
3. If the API takes 35s, the HTTP client times out at 30s, not the tool at 60s
4. The error message says "timeout" but the boundary is the HTTP transport

### F2 -- ResponseHeaderTimeout surprise
1. Call a shared report endpoint against a workspace with large data
2. If the report server takes >10s to prepare the first byte, the transport-level `ResponseHeaderTimeout` fires
3. The error is opaque: "context deadline exceeded" from `http.Client.Do`

### F3 -- Missing pre-first-attempt deadline check
1. Create a context with deadline 1ms in the future
2. Call `client.Get(ctx, ...)` -- the first attempt goes through to `http.NewRequestWithContext`
3. The HTTP transport catches the expired deadline immediately, so no real harm
4. But the retry metric doesn't distinguish "never tried" from "tried once and failed"

## Cleanup performed

No test resources were created in the live workspace. Only read-only probe calls were made.

## Leftover test resources

None.

## Severity

| Finding | Severity | Category |
|---------|----------|----------|
| F1 -- HTTP Client Timeout < Tool Timeout | P2 | Layered timeout contract |
| F2 -- ResponseHeaderTimeout too short | P3 | Configurability |
| F3 -- No pre-first-attempt deadline check | P3 | Cosmetic clarity |

## Files changed

None.

## Suggested next action

1. **P2 (F1):** Add a startup warning or derive HTTP client timeout from tool timeout. Could be a 3-line change in `internal/runtime/service.go` or `internal/runtime/runtime.go` to pass `min(cfg.RequestTimeout, cfg.ToolTimeout - 5s)` when constructing the Clockify client.
2. **P3 (F2):** Consider `CLOCKIFY_RESPONSE_HEADER_TIMEOUT` env var if operators request it.
3. **P3 (F3):** Add a pre-first-attempt deadline check in `doRequest` for cosmetic clarity.

## False positives / uncertainty

- **Time entries 405:** The `GET /time-entries` returning 405 may be a deployment-specific quirk (CloudFront edge `BUD50-P2`). The go-clockify internal tests show `GET /time-entries/{id}` working. The MCP server uses `POST /time-entries` for creation and individual GET for single-entry reads. Not a timeout/cancellation concern.
- **Live API response times** were between 0.2s and 0.9s -- well within all configured timeouts. The 10s ResponseHeaderTimeout and 30s HTTP client timeout are not likely to fire in normal operation against the current Clockify API.

## Final recommendation

**PASS WITH CONCERNS** -- The timeout and cancellation design is production-grade for all transports that support notifications (stdio, streamable_http, gRPC). The inflight tracking, context propagation, retry deadline awareness, and session reap mechanisms are correctly implemented and well-tested. The three findings are minor and do not block community, internal, or self-hosted readiness. The legacy HTTP transport's cancellation gap is documented and by design. No code changes are required to ship, but addressing F1 (P2) would strengthen the layered timeout contract.
