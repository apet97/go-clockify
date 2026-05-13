# QA Agent 35 - integration-tests-mocked-only

## Verdict
PASS WITH CONCERNS

## What I checked

1. **Mock-based integration test infrastructure**: The `tests/harness/` package providing `TransportHarness` with factories for stdio, legacy HTTP, streamable HTTP, and gRPC transports. All parity tests run across the transport matrix using mock `mcp.ToolDescriptor` handlers in-process — no live API needed.

2. **Parity test suite** (all non-live):
   - `TestParity_InitializeReturnsProtocolVersion` — all transports
   - `TestParity_ToolsListContainsMockTool` — all transports
   - `TestParity_ToolsCallSucceeds` — all transports
   - `TestParity_ToolsCallUnknown_ReturnsError` — all transports
   - `TestParity_ToolsCallSchemaValidationErrorCarriesPointer` — all transports
   - `TestParity_ToolPanicReturnsStableErrorEnvelope` — all transports (with legacy_http exception)
   - `TestCancellation_AbortsInflightHandler` — stdio, streamable, gRPC (skips HTTP)
   - `TestSizeLimit_ParityAcrossTransports` — all transports with 4 KiB cap
   - `TestSizeLimit_MalformedJSONParity` — all transports with -32700 parse error
   - `TestListChanged_ParityAcrossTransports` — stdio, streamable, gRPC
   - `TestSSE_ResumeReplaysBacklog` — streamable HTTP SSE Last-Event-ID
   - `TestE2E_StdioLifecycle` / `TestE2E_LegacyHTTPLifecycle` / `TestE2E_StreamableHTTPLifecycle`
   - `TestE2E_InvalidToolName_ReturnsRPCError` — stdio, legacy HTTP, streamable

3. **Internal/mcp package tests**: ~50+ tests covering server, panic recovery, audit, HTTP admission, structured content, resources, prompts, integration, activation, fuzzing, and benchmarks — all pass.

4. **Internal/tools package tests**: ~40+ dispatch tests covering Tier-1 and Tier-2 tools across all domains (invoices, expenses, scheduling, time-off, approvals, shared-reports, user-admin, webhooks, custom-fields, groups-holidays, project-admin) — all pass.

5. **MCP server doctor**: `clockify-mcp doctor --strict` correctly audits config and surfaces errors for missing strict-posture requirements (MCP_DISABLE_INLINE_SECRETS, MCP_CONTROL_PLANE_DSN, MCP_AUDIT_DURABILITY, CLOCKIFY_POLICY).

6. **MCP server live stdio smoke tests**: Connected the MCP server to the live Clockify probe workspace and verified `clockify_whoami`, `clockify_list_projects`, `clockify_create_project` (dry_run), and `clockify_list_entries` work correctly end-to-end.

7. **Direct Clockify API probes**: Verified create-read-delete cycles for projects and tags, error handling (missing name -> 400, invalid ID -> 400/404, no auth -> 401), and archive-before-delete workflow.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID, confirmation guard
- `probes/lib/common.sh` — curl wrapper, redaction, cleanup registry
- `PROJECTSDOC.md` — project endpoint reference
- `TIMEENTRYDOC.md` — time entry endpoint reference
- `findings/SUMMARY.md` — consolidated findings from prior probe campaigns (27 issues)
- `findings/*.md` — per-domain findings (scheduling, shared-reports, custom-fields, webhooks, invoices, holidays, expenses, time-off, project-memberships)
- `CLAUDE.md` — probe lab rules

Secrets are redacted throughout. The API key is stored at `/tmp/clockify-livetest.env` only.

## Commands run

All commands redacted. Pattern:

```bash
# Run mock-based integration tests (all pass)
go test ./tests/ -run "TestParity|TestE2E_Stdio|..." -count=1 -v

# Run internal MCP package tests (all pass)
go test ./internal/mcp/ -count=1 -v

# Run internal tools package tests (all pass)
go test ./internal/tools/ -count=1 -v

# Run MCP doctor
./clockify-mcp doctor --strict

# Start MCP server with live API key via stdio and call tools
CLOCKIFY_API_KEY={REDACTED} CLOCKIFY_WORKSPACE_ID={REDACTED} ./clockify-mcp

# Direct API: create-read-delete cycle
curl -H "X-Api-Key: {REDACTED}" \
  -X POST -d '{"name":"qa-agent-35-..."}' \
  "https://api.clockify.me/api/v1/workspaces/{REDACTED}/projects"

# Direct API: edge cases (missing name, invalid ID, no auth)
curl -H "X-Api-Key: {REDACTED}" \
  -X POST -d '{"billable":true}' \
  "https://api.clockify.me/api/v1/workspaces/{REDACTED}/projects"
# -> 400 {"message":"Project name is required","code":501}

curl -H "X-Api-Key: {REDACTED}" \
  "https://api.clockify.me/api/v1/workspaces/{REDACTED}/projects/nonexistent-id"
# -> 400 {"message":"Project doesn't belong to Workspace","code":501}

curl "https://api.clockify.me/api/v1/workspaces/{REDACTED}/projects"
# -> 401 {"message":"Multiple or none auth tokens present","code":1000}
```

## Live API probes run

| # | Probe | Method | Endpoint | Status | Notes |
|---|-------|--------|----------|--------|-------|
| 1 | List projects | GET | `/workspaces/{ws}/projects?page-size=3` | 200 | Returns array |
| 2 | List tags | GET | `/workspaces/{ws}/tags?page-size=3` | 200 | Returns array |
| 3 | List clients | GET | `/workspaces/{ws}/clients?page-size=3` | 200 | Returns array |
| 4 | Create project | POST | `/workspaces/{ws}/projects` | 200 | ID returned, cleaned up |
| 5 | Get project | GET | `/workspaces/{ws}/projects/{id}` | 200 | Verified round-trip |
| 6 | Archive project | PUT | `/workspaces/{ws}/projects/{id}` | 200 | `{"archived":true}` |
| 7 | Delete project | DELETE | `/workspaces/{ws}/projects/{id}` | 200 | Must archive first |
| 8 | Create tag | POST | `/workspaces/{ws}/tags` | 200 | ID returned, cleaned up |
| 9 | Delete tag | DELETE | `/workspaces/{ws}/tags/{id}` | 200 | Direct delete possible |
| 10 | Missing name | POST | `/workspaces/{ws}/projects` | 400 | `"Project name is required"` |
| 11 | Invalid project ID | GET | `/workspaces/{ws}/projects/nonexistent` | 400 | Returns 400 not 404 |
| 12 | No auth | GET | `/workspaces/{ws}/projects` | 401 | Auth enforced |
| 13 | Time entries GET | GET | `/workspaces/{ws}/time-entries` | 405 | Method not supported |
| 14 | MCP whoami | tools/call | `clockify_whoami` | OK | User + workspace resolved |
| 15 | MCP list projects | tools/call | `clockify_list_projects` | OK | 5 projects, pagination correct |
| 16 | MCP create project (dry) | tools/call | `clockify_create_project` | OK | dry_run=true works |
| 17 | MCP create project (live) | tools/call | `clockify_create_project` | OK | Created, then cleaned up |
| 18 | MCP list entries | tools/call | `clockify_list_entries` | OK | 3 entries returned |
| 19 | MCP invalid param | tools/call | `clockify_list_projects` | -32602 | `pageSize` rejected, `page_size` accepted |

## Findings

### Finding 1: Parameter naming convention mismatch (P2)

**Description**: The MCP tool schemas use snake_case parameter names (e.g., `page_size`) while the underlying Clockify API uses hyphenated query parameters (e.g., `page-size`). When a client sends camelCase (`pageSize`), the MCP server rejects with `-32602` ("invalid params at /pageSize: unknown property"). In contrast, the live Clockify API silently ignores unknown query params.

**Impact**: AI agents that default to camelCase (JavaScript/TypeScript convention) will hit validation errors. The MCP server correctly validates schemas but the error message could be more helpful (suggesting the correct parameter name).

**Live evidence**:
- `"pageSize": 5` -> MCP: `-32602 invalid params at /pageSize: unknown property`
- `"page_size": 5` -> MCP: works correctly
- Direct API: `?page-size=5` -> works
- Direct API: `?pageSize=5` -> silently ignored (returns default page size)

**Recommendation**: Document the snake_case convention clearly in the `instructions` field returned by `initialize`. Consider adding a helper tool or error hint that maps parameter names.

### Finding 2: Clockify returns 400 for "not found" (P2)

**Description**: The Clockify API returns HTTP 400 (not 404) when a resource ID doesn't exist or doesn't belong to the workspace. The error body uses `code:501` with message like "Project doesn't belong to Workspace". This is a known Clockify API quirk where security through obscurity is applied to resource IDs — the API doesn't distinguish between "not found" and "not yours".

**Impact**: Error handling code that expects 404 for missing resources will incorrectly treat this as a validation error. The go-clockify `internal/clockify/` client layer should consistently map `code:501` responses to a "not found" semantic.

### Finding 3: Time entries GET returns 405 (P1) — Known issue

**Description**: `GET /workspaces/{ws}/time-entries` returns 405 "Request method 'GET' is not supported". This is a known finding already documented in the probe lab. The go-clockify code at `internal/tools/entries.go:233` constructs a GET to `/workspaces/{ws}/time-entries` which would fail against the live API.

**Impact**: `clockify_list_entries` would fail if it used this path directly. However, the MCP smoke test with `clockify_list_entries` DID succeed (returned 3 entries), suggesting the code uses a different path (possibly the user-specific time entries endpoint at `/workspaces/{ws}/user/{userId}/time-entries`).

**Verification**: The MCP `clockify_list_entries` called via stdio with live API key returned 3 entries successfully, suggesting the code path in `entries.go` uses `listEntriesWithQuery` -> `/workspaces/{ws}/user/{user.id}/time-entries` (line 539) when the user ID is resolved, which works correctly.

### Finding 4: 27 known issues from probe lab remain (P1/P2)

**Description**: The probe lab's `findings/SUMMARY.md` documents 27 issues across Tier-2 domains (invoices, expenses, shared-reports, scheduling, time-off, project-memberships, holidays, custom-fields, webhooks). Issues range from BLOCK (wrong HTTP method, wrong path) to SHAPE (wrong response deserialization) to ENUM (wrong constant values). These were discovered by the live API probe campaign.

**Impact**: The mock-based dispatch tests in `internal/tools/` use `net/http/httptest` mock servers that match the CURRENT (incorrect) handler behavior. When the handlers are fixed per the SUMMARY.md recommendations, the mock tests will also need updating (as documented in the "Tests that flip from pinned-error to success-path" section of SUMMARY.md).

**Relevant tests that need updates**: 18 test changes listed in SUMMARY.md lines 59-80.

### Finding 5: gRPC tests skip cleanly when not compiled (P3 - OK)

**Description**: Tests that require gRPC (tagged with `-tags=grpc`) skip cleanly with `t.Skip("gRPC harness unavailable")` when the binary is compiled without the gRPC tag. This is correct behavior.

### Finding 6: All mock-based parity tests pass (P0 - POSITIVE)

**Description**: Every parity test across all transport implementations passes. The transport matrix covers stdio, legacy HTTP, and streamable HTTP (gRPC requires build tag). Coverage includes:
- Protocol initialization
- Tool listing
- Tool calling (success and unknown-tool)
- Schema validation with pointer
- Panic recovery with stable error envelope
- Cancellation (unblocks inflight handler)
- Size limits (under-limit control + over-limit rejection)
- Malformed JSON -> parse error -32700
- Server-initiated list_changed notifications
- SSE Last-Event-ID resume replay

## Fixes made

None. No repo issues were identified that could be fixed safely within the scope of this QA audit. The 27 known issues in SUMMARY.md require coordinated handler + test changes that are better addressed in a dedicated PR.

## Reproduction steps for each issue

### Finding 1: camelCase params rejected
```bash
# Start MCP server with API key
CLOCKIFY_API_KEY={REDACTED} CLOCKIFY_WORKSPACE_ID={REDACTED} ./clockify-mcp

# Send tools/call with camelCase pageSize
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"clockify_list_projects","arguments":{"page":1,"pageSize":5}}}
# -> {"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"invalid params at /pageSize: unknown property","data":{"pointer":"/pageSize"}}}

# Same with snake_case
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"clockify_list_projects","arguments":{"page":1,"page_size":5}}}
# -> works
```

### Finding 3: Time entries GET 405
```bash
curl -H "X-Api-Key: {REDACTED}" \
  "https://api.clockify.me/api/v1/workspaces/{REDACTED}/time-entries?page=1&page-size=3"
# -> 405 {"message":"Request method 'GET' is not supported","code":3000}
```

## Cleanup performed

| Resource | ID | Cleanup method | Status |
|----------|----|----------------|--------|
| Project | `6a00f5272568d3d293060609` | Archive then Delete | 200 |
| Tag | `6a00fab9385b9fac085a65c9` | Delete | 200 |
| Project | `6a00fab1385b9fac085a6562` | Archive then Delete | 200 |
| Project | `6a00fe03d9647159dc10999b` | Archive then Delete | 200 |

## Leftover test resources

None. All `qa-agent-35-` prefixed resources were successfully cleaned up.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| #1 Parameter naming | P2 | Snake_case convention is documented in tool schemas; MCP schema validation correctly rejects unknown properties; error message could be more helpful but doesn't block operation |
| #2 Clockify 400-for-404 | P2 | Low-level HTTP behavior; the go-clockify client already handles `code:501` errors; only relevant for new error mapping code |
| #3 Time entries 405 | P1 | Would block `clockify_list_entries` if it used the workspace-level endpoint, but the MCP tool succeeds because it uses the user-specific path — needs verification in source |
| #4 27 probe lab issues | P1 | Confirmed by prior campaign; Tier-2 tools (invoices, expenses, scheduling, etc.) have known blockers; mock dispatch tests will need updating alongside handler fixes |
| #5 gRPC skip | P3 | Correct behavior, no fix needed |
| #6 Parity tests | P0 | All pass — positive finding |

## Files changed

None.

## Suggested next action

1. **Address the 27 probe lab findings** (SUMMARY.md lines 26-55) in priority order: BLOCK issues first (invoices #1, expenses #2, shared-reports #3, scheduling #4, time-off #5, project-memberships #6, holidays #8, scheduling #18, shared-reports #24-25-27), then SHAPE issues, then ENUM issues.

2. **Update the mock dispatch tests** as handlers are fixed. The probe lab SUMMARY.md lines 59-80 provide the exact test changes needed for 18 test cases that currently assert pinned-error behavior.

3. **Add a parameter name mapping hint** in the MCP `instructions` field to help AI agents use snake_case consistently for Clockify MCP tools.

4. **Verify `clockify_list_entries` path**: Confirm that the handler at `entries.go:233` is never reached in practice (it uses a user-specific path via `listEntriesWithQuery` -> line 539) and either remove the dead workspace-level path or add a comment explaining why it's there.

## False positives / uncertainty

- **Finding 3 (Time entries 405)**: The `clockify_list_entries` MCP tool succeeded in the live smoke test, so the code path used at runtime avoids the broken `GET /workspaces/{ws}/time-entries` endpoint. I traced the code to `entries.go:539` which uses `/workspaces/{ws}/user/{userId}/time-entries`. The dead path at `entries.go:233` needs a code-level review to confirm it's truly unreachable.

- **27 probe lab issues**: These are documented findings from a prior probe campaign, not independently verified in this QA run. However, the probe lab findings are based on live API fixtures with documented HTTP status codes and response bodies, which gives high confidence.

- **gRPC transport**: Not tested directly because the binary was compiled without `-tags=grpc`. The gRPC tests skip cleanly. A full gRPC build and test run would be needed for complete coverage.

## Final recommendation

**PASS WITH CONCERNS** — The mock-based integration test infrastructure is solid and all parity tests pass across transports. The MCP server initializes and operates correctly against the live Clockify API for Tier-1 tools. The main concerns are:

1. The 27 known probe lab issues for Tier-2 tools require handler fixes before those tools can work against the live API.
2. The mock dispatch tests for Tier-2 tools will need updating alongside handler fixes (18 test changes documented in SUMMARY.md).
3. The workspace-level time entries GET endpoint is non-functional (returns 405), though the MCP server appears to use a working user-specific path.

The repository is in good shape for local/self-hosted use of Tier-1 tools. Tier-2 tool readiness depends on resolving the SUMMARY.md issues.
