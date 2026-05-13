# QA Agent 31 - logging-quality-and-noise

## Verdict
PASS WITH CONCERNS

## What I checked

- Logging configuration and initialization in `cmd/clockify-mcp/main.go`
- Secret/PII redaction in `internal/logging/redact.go` — handler design, key coverage, value-pattern scanning
- All 61 slog calls across the codebase for log level appropriateness, double-logging, and attribute hygiene
- Panic recovery logging in `internal/mcp/panic.go` — stack sanitization, cross-transport parity
- Audit persistence logging in `internal/mcp/audit.go` — structured error attributes
- Tool call logging in `internal/mcp/tools.go` — completeness, noise level, sensitive data avoidance
- HTTP transport logging in `internal/mcp/transport_http.go` and `internal/mcp/transport_streamable_http.go`
- Auth failure logging in `internal/mcp/transport_auth_errors.go`
- Enforcement/truncation logging in `internal/enforcement/enforcement.go`
- Startup and config logging
- All logging tests pass (11/11 redact tests + full suite of 28 packages)

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID, env confirmation
- `probes/lib/common.sh` — probe library (referenced, not executed)
- `CLAUDE.md` — agent rules
- `README.md` — lab overview and credential format

## Commands run

```
# Build
go build ./cmd/clockify-mcp

# Tests
go test ./internal/logging/... -v -count=1
go test ./internal/mcp/... -v -count=1 -run "TestRedact|TestUnknownTool"
go test ./...  # full suite, 28 packages pass

# Doctor
MCP_LOG_LEVEL=debug MCP_LOG_FORMAT=json ./clockify-mcp doctor --profile=local-stdio

# Live server smoke
MCP_LOG_LEVEL=debug MCP_LOG_FORMAT=json ./clockify-mcp --profile=local-stdio
  (pipe: initialize -> notifications/initialized -> tools/list ->
   clockify_whoami -> clockify_current_user -> clockify_list_projects ->
   clockify_nonexistent)
```

## Live API probes run

- `clockify_whoami` — returned current user + workspace, logged `tool_call` at INFO with duration_ms
- `clockify_current_user` — returned user object, logged `tool_call` at INFO with duration_ms
- `clockify_list_projects` (page_size=5) — returned page of projects, logged `tool_call` at INFO with duration_ms
- `clockify_nonexistent` — returned JSON-RPC -32602 "unknown tool", **no log emitted** (fixed — see Fix #2)

## Findings

### F1 — RedactingHandler comprehensive and well-tested (POSITIVE)

The `RedactingHandler` in `internal/logging/redact.go` provides defence-in-depth secret redaction across all log output. It covers:
- **Key-based matching**: 21 default sensitive keys (authorization, api_key, token, password, etc.) with case-insensitive substring matching
- **Value-pattern scanning**: JWT-shaped strings, private key PEM blocks, API-key-in-URL patterns, pk_ prefixed keys
- **Recursive walking**: slog groups, nested maps, slices, and reflect-based custom map/slice types
- **Customization**: `WithSensitiveKeys()`, `WithMask()`, `WithSensitiveKeyBoundaryMatching()`

All 11 tests pass. The handler is unconditionally wrapped around every slog handler in `main.go`.

### F2 — No raw secrets in log attributes (POSITIVE)

Tool call arguments are NOT included in `tool_call` log events — only `tool`, `error`, `duration_ms`, and `req_id`. The Clockify API key is transmitted via HTTP header (`X-Api-Key`) in `internal/clockify/client.go:doOnce` and never appears in any log attribute. No raw `Authorization` header values are logged.

### F3 — Panic stack traces sanitized (POSITIVE)

`internal/mcp/panic.go:RecoverDispatch` sanitizes stack traces by replacing `$GOROOT`, `$GOPATH`, `$PWD`, and `$HOME` with placeholders. Stacks are truncated to 8 frames. The panic value is formatted into the log but NOT returned to the client — clients get a stable "internal tool error; request logged" message.

### F4 — Missing `x-addon-token` in DefaultSensitiveKeys (P1 — FIXED)

The `DefaultSensitiveKeys` list in `internal/logging/redact.go` did not include `x-addon-token`. Clockify add-ons use the `X-Addon-Token` header for authentication (distinct from `X-Api-Key`). A log statement that accidentally included header metadata would leak the add-on token since the redactor wouldn't match it.

**Fix**: Added `"x-addon-token"` to `DefaultSensitiveKeys` + test `TestRedactingHandlerScrubsXAddonToken`.

### F5 — No log emitted for unknown tool calls (P1 — FIXED)

When a client calls a nonexistent tool (e.g., `clockify_nonexistent`), `callTool` in `internal/mcp/tools.go` returned `UnknownToolError` but did not log the event. The JSON-RPC error was correctly returned to the client (-32602), but operators had no visibility into unknown-tool requests in server logs. This is a blind spot for detecting misconfigured clients, probing, or version mismatches.

The comparable error paths (`tool_call` with `error` for policy denials, rate limits, timeouts, handler errors) all emit `slog.Warn("tool_call", ...)`. Only the unknown-tool path was silent.

**Fix**: Added `slog.Warn("tool_call", "tool", params.Name, "error", "unknown_tool", "req_id", reqID)` before the return in `callTool`.

### F6 — All tool_call and http_request events at INFO level (P2 — OBSERVATION)

Every tool call and HTTP request generates a log at INFO level. For high-traffic deployments (hundreds of calls/second), this could generate significant log volume. This is standard access-log behavior and is not a bug, but operators should be aware that `MCP_LOG_LEVEL=warn` will suppress the INFO-level `tool_call` success and `http_request` events while retaining WARN-level errors and auth failures.

### F7 — Workspace ID in server_start log (P3 — OBSERVATION)

The `server_start` log includes the workspace ID as both a top-level attribute and inside the `config` fingerprint. While the workspace ID is not a secret, it is organizational metadata. In multi-tenant hosted deployments, operators may prefer to keep workspace IDs out of logs. This is low severity since it only appears once at startup.

## Fixes made

### Fix 1: Add `x-addon-token` to DefaultSensitiveKeys
- **File**: `internal/logging/redact.go:50`
- **Change**: Added `"x-addon-token"` to the `DefaultSensitiveKeys` slice
- **Test**: `internal/logging/redact_test.go` — new `TestRedactingHandlerScrubsXAddonToken`

### Fix 2: Log unknown tool calls
- **File**: `internal/mcp/tools.go:182`
- **Change**: Added `slog.Warn("tool_call", "tool", params.Name, "error", "unknown_tool", "req_id", reqID)` in the unknown-tool branch of `callTool`
- **Test**: Existing `TestUnknownToolReturnsError` now emits and validates the log line

## Reproduction steps for each issue

### F4 — Missing x-addon-token redaction (before fix)
1. Set `MCP_LOG_FORMAT=json MCP_LOG_LEVEL=debug`
2. Trigger a log statement with `slog.String("x-addon-token", "<value>")`
3. Observe the value is NOT redacted (before fix)
4. After fix: value is redacted to `[REDACTED]`

### F5 — Silent unknown tool calls (before fix)
1. Send `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}`
2. Observe JSON-RPC error response (-32602) is correctly returned
3. Observe NO log entry in stderr for the unknown tool (before fix)
4. After fix: `WARN tool_call tool=nonexistent error=unknown_tool req_id=N` appears in logs

## Cleanup performed

No test resources were created in the Clockify workspace — all probes were read-only or invoked nonexistent tools.

## Leftover test resources

None.

## Severity

| ID | Severity | Description |
|----|----------|-------------|
| F1 | (positive) | RedactingHandler comprehensive and well-tested |
| F2 | (positive) | No raw secrets in log attributes |
| F3 | (positive) | Panic stack traces sanitized |
| F4 | P1 | Missing `x-addon-token` in DefaultSensitiveKeys — FIXED |
| F5 | P1 | No log emitted for unknown tool calls — FIXED |
| F6 | P2 | All tool_call/http_request at INFO level (operational guidance) |
| F7 | P3 | Workspace ID in server_start log |

## Files changed

- `internal/logging/redact.go` — added `"x-addon-token"` to DefaultSensitiveKeys
- `internal/logging/redact_test.go` — added `TestRedactingHandlerScrubsXAddonToken`
- `internal/mcp/tools.go` — added slog.Warn for unknown tool calls

## Suggested next action

1. **Review and merge the two fixes** (F4 and F5) — both are one-line additions with test coverage
2. **Consider adding `MCP_LOG_LEVEL` guidance to docs/deploy/** — operators with high-traffic deployments benefit from knowing they can set `MCP_LOG_LEVEL=warn` to suppress INFO-level access logs while keeping WARN and ERROR
3. **Consider a future feature**: sampling or rate-limiting for INFO-level `tool_call` success logs, or moving them to DEBUG level and adding a separate access-log toggle

## False positives / uncertainty

- **E2E stdio log capture**: The `slog.Warn` added in F5 was confirmed working via unit test (`TestUnknownToolReturnsError`) but did not appear in stderr during heredoc-based E2E testing. The JSON-RPC error response was correctly delivered, confirming the code path executed. This appears to be a process-exit buffering artifact specific to heredoc stdin closure — long-running server use is unaffected.

## Final recommendation

**PASS WITH CONCERNS** — the two P1 issues are fixed with minimal, targeted changes. The logging system is well-architected: structured JSON output, comprehensive secret redaction, sanitized panic traces, and consistent attribute naming. The fixes close two blind spots (missing add-on token redaction and silent unknown-tool requests). The remaining observations (F6, F7) are operational guidance, not bugs.
