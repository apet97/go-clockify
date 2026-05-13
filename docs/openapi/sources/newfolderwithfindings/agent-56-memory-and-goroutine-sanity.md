# QA Agent 56 - memory-and-goroutine-sanity

## Verdict
PASS

## What I checked

I audited the go-clockify MCP server's goroutine lifecycle management, memory safety, and runtime metrics across all transport modes (stdio, legacy HTTP, streamable HTTP, gRPC). I also verified goroutine cleanup, cancellation propagation, bounded dispatch, subject/session reaping, and live API behavior through the MCP tool path.

### Code paths audited

| Area | Files reviewed | Key findings |
|------|---------------|--------------|
| Stdio dispatch loop | `internal/mcp/server.go:492-642` | Bounded semaphore (`toolCallSem`), `wg.Wait()` drains in-flight goroutines, structured panic recovery per goroutine |
| Tool call lifecycle | `internal/mcp/tools.go:162-334` | Cancellable context registered in inflight map, inflight cleanup on all exit paths, 45s default tool timeout |
| Cancellation | `internal/mcp/server.go:682-698`, `internal/mcp/cancel_test.go` | notifications/cancelled aborts via context cancellation, inflight map properly drained on repeat-initialize |
| Panic recovery | `internal/mcp/panic.go:55-86` | Centralized `RecoverDispatch` at all transport boundaries, stack sanitization, stable JSON-RPC error envelope |
| Rate limiter subject reaper | `internal/ratelimit/ratelimit.go:330-371` | Background goroutine respects ctx.Done(), per-subject idle TTL with semaphore-aware eviction |
| Streamable HTTP session reaper | `internal/mcp/transport_streamable_http.go:905-966` | TTL + orphaned-subscriber eviction, per-minute sweep, `closeAll()` on server shutdown |
| Audit retention reaper | `internal/runtime/retain.go:22-39` | Background ticker, ctx-cancellable, one-shot on restart |
| Session event hub (SSE) | `internal/mcp/transport_streamable_http.go:1010-1149` | Ring-buffer backlog, bounded subscriber channels, slow-subscriber eviction with metric |
| Runtime metrics | `internal/metrics/runtime.go` | Lazy scrape via `runtime/metrics.Read` (lock-free), FDs cached with 5s rate limit |
| Metrics registry | `internal/metrics/metrics.go` | `sync.Map` for counters/histograms, `sync.Mutex` for gauges, deterministic sample ordering |
| GRPC readiness ticker | `internal/runtime/grpc.go:25-43` | Background goroutine with ticker, respects ctx.Done() |
| Legacy HTTP transport | `internal/runtime/legacy_http.go` | No long-lived goroutines beyond stdio dispatch |

### Test suite results

- `go test -race -count=1 ./internal/...` — all 27 packages PASS (clean race detector)
- `go vet ./internal/...` — clean
- `go vet ./cmd/...` — clean
- `go build ./cmd/clockify-mcp/` — clean
- Specific goroutine tests: `TestStdioDispatch_BoundedConcurrency`, `TestStdioDispatch_ContextCancelReleases`, `TestStdioDispatch_Unlimited`, `TestCancellation_*` (5 tests), `TestPanicRecovery`, `TestServerBench*`, `TestSessionReap*` — all PASS with race detector
- Benchmarks under `-race`: `BenchmarkDispatchInitialize`, `BenchmarkDispatchToolsCall` — PASS
- Live E2E read-only: `TestE2EReadOnly` — PASS
- Live E2E mutating: `TestE2EMutating`/`TestLiveCreateUpdateDelete` — PASS

### Goroutine lifecycle summary

| Goroutine source | Startup | Cleanup | Leak risk |
|-----------------|---------|---------|-----------|
| Stdio scanner | `Run()` line 529 | exits on stdin EOF or ctx.Done() | None — eventual exit via pipe close |
| Tools/call handlers | `Run()` line 588 | `defer wg.Done()`, `defer wg.Wait()` | None — WaitGroup ensures drain |
| Metrics listener | `main.go` line 156 | ctx cancellation calls `cancel()` | None — server Shutdown on ctx |
| Subject reaper | `StartSubjectReaper` line 355 | `case <-ctx.Done(): return` | None |
| Session reaper | `reapLoop` line 905 | `case <-ctx.Done(): return` | None |
| Audit retention | `RetainAuditLoop` line 22 | `case <-ctx.Done(): return` | None |
| gRPC readiness | `runGRPC` line 25 | `case <-ctx.Done(): return` | None |
| OTel exporter | `main.go` line 153 | `defer otelShutdown()` | None |
| SSE handler | `streamableEventsHandler` line 464 | `case <-r.Context().Done(): return` | None — per-request ctx |

### Concurrency safety

- **toolCallSem**: Buffered channel prevents goroutine amplification from bursty `tools/call` input. Semaphore slot acquired before goroutine spawn, released via `defer`. Context cancellation checked on both acquire paths.
- **inflight map**: `sync.Mutex` protected, double-checked on register/unregister. Repeat-initialize drains entire map via `cancelAllInflight()`.
- **encoder serialization**: All stdio writes go through `s.encoderMu`, safe for concurrent handler goroutines.
- **Notifier hub**: `sync.RWMutex` protected fan-out, slow subscribers evicted with metric.

### Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key (****REDACTED****), workspace ID (`65b382b606de527a7ee2b60e`)
- Probe lab: `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/` (CLAUDE.md, README.md, docs/, probes/)

## Commands run

```bash
# Race detector on all internal packages
go test -race -count=1 -timeout 120s ./internal/...

# Static analysis
go vet ./internal/... && go vet ./cmd/...

# Build
go build ./cmd/clockify-mcp/

# Doctor
./clockify-mcp doctor --profile=local-stdio --strict --allow-broad-policy
./clockify-mcp doctor --profile=single-tenant-http --strict

# Live API connectivity
curl -s -H "X-Api-Key: $CLOCKIFY_API_KEY" "https://api.clockify.me/api/v1/workspaces/$CLOCKIFY_WORKSPACE_ID"

# MCP stdio smoke: initialize + tools/list + tools/call
echo '...' | ./clockify-mcp

# Live E2E tests
go test -race -count=1 -run "TestE2EReadOnly|TestE2EMutating" -tags=livee2e -timeout 120s ./tests/...

# Benchmarks under race
go test -race -bench=BenchmarkDispatch -benchtime=100ms -timeout 60s ./internal/mcp/...
```

## Live API probes run

Probe workspace: `65b382b606de527a7ee2b60e` (WORKSPACE)

| Probe | Method | Result |
|-------|--------|--------|
| `initialize` (protocol 2025-03-26) | MCP stdio | PASS — negotiated version, server info, capabilities |
| `tools/list` | MCP stdio | PASS — 35 visible tier-1 tools |
| `clockify_get_workspace` | MCP stdio | PASS — returns workspace data with structuredContent |
| `clockify_current_user` | MCP stdio | PASS — returns user id, name, email, activeWorkspace |
| `clockify_whoami` | MCP stdio | PASS — returns user + workspaceId |
| `clockify_list_entries` (page_size=5) | MCP stdio | PASS — returns entries array with meta |
| `clockify_add_entry` (create test entry) | MCP stdio | PASS — created entry `6a00fd4cd9647159dc1091be` |
| Delete test entry | Direct API (DELETE) | PASS — 204 No Content |
| Doctor config audit (local-stdio) | Doctor | PASS — config loads, warns about strict posture |
| Doctor config audit (single-tenant-http) | Doctor | PASS — correctly identifies missing API key |

## Findings

### Finding 1: Misleading `syncWriter` name in concurrency tests (P3, cosmetic)
**File**: `internal/mcp/server_concurrency_test.go:248-258`

The struct `syncWriter` is documented as "serializing concurrent Write calls" but contains no synchronization primitives. It relies on the server's `encoderMu` for safety. The name and comment are misleading. Rename to `noopWriter` or remove the struct entirely and pass `&output` directly.

### Finding 2: Scanner goroutine not WaitGroup-tracked (P3, defensive)
**File**: `internal/mcp/server.go:529-545`

The scanner goroutine that feeds `lines` channel is launched without `wg.Add(1)`. When `Run()` returns due to `ctx.Done()`, the scanner goroutine continues until stdin produces a line or EOF. In practice this is safe because stdin closure triggers EOF, and the goroutine eventually exits. No production impact, but could confuse readers. Documenting the exit path would help.

### Finding 3: Subject reaper skip condition uses `len(sub.semaphore) > 0` (P3, documentation)
**File**: `internal/ratelimit/ratelimit.go:342`

The reaper skips subjects whose semaphore has any in-flight call (`len(sub.semaphore) > 0`). This is correct for preventing stranded releases, but means a subject with a stuck handler (ignoring context cancellation) will never be evicted, consuming map memory. This is documented inline but worth noting in operations runbooks.

## Fixes made

No code fixes needed. All goroutine lifecycle patterns are correct and well-tested. The findings above are cosmetic/documentation-level observations.

## Reproduction steps for each issue

### Finding 1 (syncWriter)
1. Read `internal/mcp/server_concurrency_test.go:248-258`
2. Observe struct named `syncWriter` has no mutex or channel
3. Note that concurrent safety comes from `s.encoderMu` in the server, not from this wrapper
4. Severity: cosmetic only — no test race exists

### Finding 2 (scanner goroutine)
1. Start server with `./clockify-mcp` (stdio transport)
2. Send a signal (SIGTERM) that triggers ctx cancellation via `signal.NotifyContext`
3. Observe that `Run()` returns immediately via `<-ctx.Done()` without waiting for scanner goroutine
4. Scanner goroutine continues until stdin EOF — occurs naturally when parent process disconnects
5. Severity: harmless in practice — no resource leak in production

### Finding 3 (subject reaper skip)
1. Configure per-token rate limiting: `CLOCKIFY_PER_TOKEN_CONCURRENCY=5`
2. A handler that ignores context cancellation holds a subject's semaphore slot
3. `ReapIdleSubjects()` skips that subject because `len(sub.semaphore) > 0`
4. Subject entry stays in map until handler eventually returns
5. Severity: requires a buggy handler to trigger; graceful-degradation design is intentional

## Cleanup performed

- Deleted test time entry `6a00fd4cd9647159dc1091be` via direct Clockify API
- No other test resources were created

## Leftover test resources

None.

## Severity

All findings are P3 (cosmetic/documentation). No P0, P1, or P2 issues found.

## Files changed

None. No code changes were required.

## Suggested next action

1. Consider renaming `syncWriter` to `bytesWriter` in `server_concurrency_test.go` or adding a comment explaining why it's safe without synchronization.
2. Optionally add a comment in `Run()` noting the scanner goroutine's exit path (stdin EOF).
3. Run the full `make check` suite on the current HEAD to confirm no regressions.
4. Consider adding a `go_leak` test using `runtime.NumGoroutine()` before/after server startup/shutdown for long-term regression prevention.

## False positives / uncertainty

- The `syncWriter` struct is functionally safe due to the server's encoder mutex — the name is the only issue.
- The scanner goroutine "leak" during shutdown is a standard Go pattern for stdin-scanner loops — the goroutine exits on EOF which is the normal shutdown path.
- Without a multi-hour soak test, I cannot definitively rule out very slow memory growth in the metrics `sync.Map` values (labels cardinality). The metric label sets are bounded (tool names, status codes, etc.) so this is not a real concern.

## Final recommendation

PASS — The go-clockify MCP server's goroutine and memory management is production-grade. Bounded dispatch with semaphore backpressure, proper WaitGroup tracking, context-cancellable background goroutines, structured panic recovery at all transport boundaries, and comprehensive race-detector coverage. No goroutine leaks, no data races, no unsafe memory patterns were found. The three P3 observations are cosmetic/documentation improvements that do not affect correctness.
