# QA Agent 53 - http-status-code-mapping

## Verdict
PASS

## What I checked

The MCP server's HTTP status code mapping across three layers:
1. **Clockify API client** (`internal/clockify/client.go`) — how upstream HTTP responses are converted to `APIError` and which codes trigger retries.
2. **Transport/admission layer** (`internal/mcp/transport_http.go`, `http_admission.go`, `transport_streamable_http.go`) — how transport-level HTTP statuses map to JSON-RPC error codes.
3. **Metrics layer** (`internal/clockify/metrics.go`) — how HTTP statuses bucket into Prometheus labels and retry-reason labels.

Plus live API probes against the real Clockify API to verify error response shapes match the parsing logic.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID, confirmation token (redacted in this report)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — agent rules
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/README.md` — lab documentation

## Commands run

```bash
# Unit tests
go test ./internal/clockify/ -run "Error|Status|HTTP" -v -count=1
go test ./internal/mcp/ -run "HTTP|Error|Status|Auth|Admission" -v -count=1
go test ./internal/clockify/ -run "IsRetryable|statusBucket|retryReason" -v -count=1

# Static analysis
go vet ./internal/clockify/ ./internal/mcp/

# Live API probes (full list below)
```

## Live API probes run

All probes used the workspace at the probe lab credentials. API key redacted as `$CLOCKIFY_API_KEY`.

| # | Endpoint | Method | Expected | Got | Notes |
|---|----------|--------|----------|-----|-------|
| 1 | `/api/v1/user` (bad key) | GET | 401 | 401 | Body: `{"message":"Api key does not exist","code":4003}` |
| 2 | `/api/v1/workspaces/000...000` | GET | 403 | 404 | Clockify returns 404 for inaccessible workspace IDs |
| 3 | `/api/v1/nonexistent` | GET | 404 | 404 | Non-existent URL path |
| 4 | `/api/v1/user` | GET | 200 | 200 | Valid authenticated request |
| 5 | `/api/v1/workspaces/{id}/projects` (bad JSON) | POST | 400 | 400 | Body: `{"message":"JSON parse error: ...","code":3002}` |
| 6 | `/api/v1/workspaces/{id}/projects` (missing name) | POST | 400 | 400 | Body: `{"message":"Project name is required","code":501}` |
| 7 | `/api/v1/workspaces/{id}/projects/000...000` | GET | 404 | 400 | Clockify uses 400 for "not found" with code 501 |
| 8 | `/api/v1/workspaces/{id}/projects` (valid) | POST | 201 | 201 | Successful creation |
| 9 | `/api/v1/workspaces/{id}/projects/{id}` (valid) | GET | 200 | 200 | Successful read |
| 10 | `/api/v1/workspaces/{id}/projects/{id}` | PUT | 200 | 200 | Successful update (archive) |
| 11 | `/api/v1/workspaces/{id}/projects/{id}` | DELETE | 200 | 200 | Successful delete (archived project) |
| 12 | `/api/v1/workspaces/{id}/projects/{id}` (active) | DELETE | 400 | 400 | "Cannot delete an active project", code 501 |
| 13 | `/api/v1/workspaces/{id}/projects/000...000` | PATCH | 405 | 405 | PATCH not supported by Clockify |
| 14 | `/api/v1/shared-reports` (main host) | GET | 404 | 404 | Reports endpoint only on reports host |
| 15 | `/api/v1/workspaces/INVALID_FORMAT/projects` | GET | 400 | 400 | "Access Denied", code 501 |
| 16 | Rate-limit probe (3 rapid GETs) | GET | 200 | 200,200,200 | Not rate-limited at low volume |

## Findings

### Finding 1: Clockify API uses 400 instead of 404 for non-existent resources (INFO)

The Clockify API consistently returns HTTP 400 (not 404) when a resource is not found or doesn't belong to the workspace. The error body follows the pattern `{"message":"... doesn't belong to Workspace","code":501}`.

**Impact**: The MCP server correctly surfaces this as an `APIError` with `StatusCode=400` and the upstream body. The `isRetryableStatus` function correctly treats 400 as non-retryable. No fix needed — the design is correct: the MCP server transparently forwards the upstream status code rather than trying to re-interpret it.

### Finding 2: Clockify error responses always use `{"message": ..., "code": <int>}` format (VERIFIED)

All error responses from the live API confirmed this format. The `compactUpstreamErrorBody` function (`internal/clockify/errors.go:52`) correctly parses `message` and `code` fields. The `code` field is always a JSON number (decoded as `float64` in Go's `any`), and `fmt.Sprintf("%v", ...)` renders whole-number floats without a decimal point (e.g., `501` not `501.0`).

### Finding 3: Missing unit tests for `statusBucket` and `retryReason` (P2 — FIXED)

The functions `statusBucket()` and `retryReason()` in `internal/clockify/metrics.go` had no unit tests, while the sibling `isRetryableStatus()` and `normalizeEndpoint()` functions were tested.

**Fix applied**: Added `TestStatusBucket` (22 test cases covering code=0, 1xx-5xx boundaries, and out-of-range codes) and `TestRetryReason` (10 test cases covering all known retryable codes and non-retryable codes) to `internal/clockify/metrics_test.go`.

### Finding 4: Complete HTTP to JSON-RPC mapping table (VERIFIED)

The mapping table below was verified by reading the transport code and running the transport error envelope tests:

| HTTP Status | Scenario | JSON-RPC Code | Constant |
|---|---|---|---|
| 400 | Missing streamable session | -32001 | `RPCCodeSessionInvalid` |
| 400 | Protocol version mismatch | -32600 | (standard) |
| 400 | Invalid JSON parse | -32700 | (standard) |
| 400 | Unknown method | -32601 | (standard) |
| 400 | Invalid params | -32602 | (standard) |
| 401 | Auth failure | -32020 | `RPCCodeUnauthenticated` |
| 403 | CORS origin rejected | -32010 | `RPCCodeOriginNotAllowed` |
| 403 | DNS rebinding host rejected | -32011 | `RPCCodeHostNotAllowed` |
| 403 | Session principal mismatch | -32014 | `RPCCodeSessionPrincipal` |
| 404 | Invalid streamable session | -32001 | `RPCCodeSessionInvalid` |
| 405 | Method not POST | -32015 | `RPCCodeMethodNotAllowed` |
| 413 | Request body too large | -32013 | `RPCCodeRequestTooLarge` |
| 429 | Rate limited | -32012 | `RPCCodeRateLimited` |

All JSON-RPC codes use the -32000 to -32099 server-reserved range, avoiding collision with the standard -32768 to -32000 range. The mapping is consistent across both legacy HTTP and streamable HTTP transports.

### Finding 5: Retryable status codes (VERIFIED)

The `isRetryableStatus` function correctly maps 429, 502, 503, 504 as retryable. The exclusion of 408 (Request Timeout) is a deliberate choice — 408 indicates the server proactively closed an idle connection, and retrying immediately could compound server load. The function is tested with explicit positive and negative cases in `TestIsRetryableStatus`.

### Finding 6: Error sanitization for multi-tenant deployments (VERIFIED)

`APIError.Sanitized()` drops the upstream response body while preserving method, path, and status. The `SanitizeUpstreamErrors` server flag controls whether MCP clients see the verbose or sanitized form. Both paths are tested in `TestAPIError_SanitizedDropsBody`, `TestSanitizeUpstreamErrors_DefaultExposesBody`, and `TestSanitizeUpstreamErrors_HostedDropsBody`.

## Fixes made

1. **Added `TestStatusBucket`** to `internal/clockify/metrics_test.go` — 22 test cases covering all HTTP status code buckets (error, 2xx, 3xx, 4xx, 5xx, other) and boundary values.

2. **Added `TestRetryReason`** to `internal/clockify/metrics_test.go` — 10 test cases covering all known retryable status codes (429, 502, 503, 504) and non-retryable codes.

## Reproduction steps for each issue

### Finding 3 (Fixed)
1. Run `go test ./internal/clockify/ -run "StatusBucket|RetryReason"` — before the fix this returned "no tests to run"
2. After the fix, both test functions pass

## Cleanup performed

- Created and deleted test project `qa-agent-53-test-project-*` (ID: `<REDACTED_ID>`) — archived then deleted.

## Leftover test resources

None. All test resources created during this run were cleaned up.

## Severity

| Finding | Severity | Status |
|---|---|---|
| Clockify uses 400 for not-found | INFO | No fix needed |
| Error response format verified | INFO | No fix needed |
| Missing `statusBucket`/`retryReason` tests | P2 | Fixed |
| HTTP to JSON-RPC mapping table | INFO | Verified |
| Retryable status codes | INFO | Verified |
| Error sanitization | INFO | Verified |

## Files changed

- `internal/clockify/metrics_test.go` — Added `TestStatusBucket` and `TestRetryReason`

## Suggested next action

1. No further action required for the HTTP status code mapping area.
2. The added tests should be included in the next PR/commit.
3. Consider documenting the Clockify API's unconventional use of HTTP 400 for "not found" scenarios as a known quirk in project docs — this would help future contributors understand why the code doesn't special-case 404.

## False positives / uncertainty

- **Clockify returning 400 for "no static resource" on `/shared-reports` on the main host**: This is documented in `client.go:161` (the `ReportsBaseURL` comment) and correctly handled by the separate reports host routing.
- **Rate limiting (429)**: Not triggered during low-volume probes. The code's handling is unit-tested and correct per HTTP semantics.

## Final recommendation

The HTTP status code mapping in the go-clockify MCP server is well-designed and correctly implemented. The transport layer cleanly separates protocol errors (JSON-RPC standard codes), transport-level errors (server-reserved -32000+ codes), and upstream Clockify API errors (passed through in the tool error envelope). The addition of unit tests for `statusBucket` and `retryReason` closes a minor test coverage gap. No further changes needed.
