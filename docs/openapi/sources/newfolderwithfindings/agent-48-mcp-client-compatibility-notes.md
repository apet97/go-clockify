# QA Agent 48 — mcp-client-compatibility-notes

## Verdict
**PASS**

## What I checked

- MCP protocol version negotiation across all four supported versions (2024-11-05, 2025-03-26, 2025-06-18, 2025-11-25)
- Initialize handshake response shape (capabilities, serverInfo, instructions, protocolVersion)
- Dual-emit contract (content + structuredContent) on every tools/call response
- Error envelope format: tool-errors (isError:true) vs JSON-RPC protocol errors
- Unknown tool error codes and messages
- Uninitialized access rejection
- Parameter validation with JSON Schema pointers
- Tool activation/deactivation lifecycle with group tools
- tools/list annotation format (riskClass, destructiveHint, readOnlyHint, dryRun, idempotentHint)
- resources/list and prompts/list response shapes
- clockify_list_tools search/discovery semantics
- Live CRUD lifecycle: create entry → read entry → delete entry through the MCP stdio path
- Upstream API error pass-through and sanitization
- Doctor command (plain and --strict --check-backends modes)
- Build success with Go 1.26.2
- Parameter naming convention (snake_case inputs → camelCase outputs)
- docs/clients.md accuracy against observed behavior
- docs/support-matrix.md transport/auth/production-readiness table against observed behavior

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | Credentials: CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, CLOCKIFY_LIVE_WORKSPACE_CONFIRM |
| `docs/official-api-notes.md` | Per-domain Clockify API endpoint reference with observed behaviors |
| `docs/domain-queue.md` | Domain priority and scoping |
| `docs/safety-rules.md` | Expanded safety rules for live probing |
| `TIMEENTRYDOC.md` | Time entry domain API documentation with endpoint shapes |
| `PROJECTSDOC.md` | Projects API documentation |
| `USERDOC.md` | Users API documentation |

## Commands run

```bash
# Build verification
go build ./cmd/clockify-mcp

# Doctor (plain)
go run ./cmd/clockify-mcp doctor

# Doctor (strict with backend checks)
go run ./cmd/clockify-mcp doctor --strict --check-backends

# Initialize handshake (protocol 2025-11-25)
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",
  "params":{"protocolVersion":"2025-11-25","capabilities":{},
  "clientInfo":{"name":"qa-agent-48","version":"1.0"}}}' \
  | go run ./cmd/clockify-mcp

# Protocol negotiation across all versions (batch)
for v in "2024-11-05" "2025-03-26" "2025-06-18" "2025-11-25" "" "2099-01-01"; do
  # initialize with protocolVersion=$v and verify negotiated version
done

# tools/list (Tier 1 surface)
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}
{"jsonrpc":"2.0","id":2,"method":"notifications/initialized",...}
{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}' \
  | go run ./cmd/clockify-mcp

# Live CRUD: clockify_add_entry → clockify_get_entry → clockify_delete_entry
# Via MCP stdio with project_id=<REDACTED_ID>

# Uninitialized access test
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call",
  "params":{"name":"clockify_whoami","arguments":{}}}' \
  | go run ./cmd/clockify-mcp

# Unknown tool test
# tools/call with name="clockify_nonexistent"

# resources/list, prompts/list validation

# Tool activation: clockify_activate_group approvals/webhooks
# Tool deactivation: clockify_deactivate_group webhooks

# clockify_list_tools search with various queries

# Direct API validation
curl -H "X-Api-Key: <REDACTED>" https://api.clockify.me/api/v1/user
```

## Live API probes run

| # | Probe | Transport | Result |
|---|-------|-----------|--------|
| 1 | MCP initialize handshake (v2025-11-25) | stdio | PASS |
| 2 | Protocol negotiation: v2024-11-05 | stdio | PASS |
| 3 | Protocol negotiation: v2025-03-26 | stdio | PASS |
| 4 | Protocol negotiation: v2025-06-18 | stdio | PASS |
| 5 | Protocol negotiation: empty/missing | stdio | PASS |
| 6 | Protocol negotiation: unsupported future (2099-01-01) | stdio | PASS |
| 7 | tools/list response shape | stdio | PASS |
| 8 | clockify_whoami via MCP | stdio | PASS |
| 9 | clockify_list_projects via MCP | stdio | PASS |
| 10 | clockify_list_entries via MCP (page_size=5) | stdio | PASS |
| 11 | clockify_add_entry (create test resource) | stdio | PASS |
| 12 | clockify_get_entry (verify created resource) | stdio | PASS |
| 13 | clockify_delete_entry (cleanup) | stdio | PASS |
| 14 | clockify_add_entry + clockify_delete_entry (2nd cycle) | stdio | PASS |
| 15 | Error envelope: non-existent entry ID | stdio | PASS |
| 16 | Error: unknown tool name | stdio | PASS |
| 17 | Error: uninitialized access | stdio | PASS |
| 18 | Error: missing required param (entry_id) | stdio | PASS |
| 19 | Error: unknown property (projectId vs project_id) | stdio | PASS |
| 20 | clockify_activate_group approvals | stdio | PASS |
| 21 | clockify_activate_group webhooks | stdio | PASS |
| 22 | clockify_deactivate_group webhooks | stdio | PASS |
| 23 | clockify_list_tools search (query="approve") | stdio | PASS |
| 24 | clockify_list_tools search (query="") | stdio | PASS |
| 25 | resources/list | stdio | PASS |
| 26 | prompts/list | stdio | PASS |
| 27 | clockify_get_workspace | stdio | PASS |
| 28 | Direct API call: GET /user | HTTPS | PASS |
| 29 | Direct API call: GET /workspaces/{ws}/projects | HTTPS | PASS |
| 30 | tools/list annotation riskClass format | stdio | PASS |
| 31 | tools/list annotation dryRun flag | stdio | PASS |
| 32 | Parameter naming: input uses snake_case (project_id) | stdio | PASS |
| 33 | Parameter naming: output uses camelCase (projectId) | stdio | PASS |
| 34 | doctor command (plain) | stdio | PASS |
| 35 | doctor command (--strict --check-backends) | stdio | PASS |

## Findings

### F1. Protocol version negotiation — PASS
The server correctly negotiates all four MCP protocol versions. Future/unknown versions gracefully downgrade to the newest supported version. The `MCP_DEFAULT_PROTOCOL_VERSION` env var for operator-controlled fallback works only when `params.protocolVersion` is omitted, respecting explicit client requests.

### F2. Dual-emit contract — PASS
Every tools/call response includes both `content` (array of `{"type":"text","text":"..."}` objects) and `structuredContent` (parsed JSON object). This is critical for backward compatibility with older MCP clients that don't support structuredContent while serving modern clients.

### F3. Error envelope correctness — PASS
Tool-level errors (API failures, not-found) correctly return `result.isError: true` with a `content` array. Protocol-level errors (unknown tool, schema validation, uninitialized) correctly return `error` objects with correct JSON-RPC codes. The JSON Schema validation provides `error.data.pointer` as an RFC 6901 JSON Pointer to the offending field.

### F4. Parameter naming convention — PASS
Tool input schemas consistently use snake_case (`project_id`, `entry_id`, `task_id`, `group_name`, `page_size`). Tool output schemas consistently use camelCase (`projectId`, `taskId`, `timeInterval`), matching the upstream Clockify API. This is well-documented and consistent.

### F5. Annotations completeness — PASS
Every tool in `tools/list` carries `annotations` with all documented fields: `destructiveHint`, `dryRun`, `idempotentHint`, `openWorldHint`, `readOnlyHint`, `riskClass`, and `title`. The `riskClass` taxonomy (`read`, `write`, `billing`, `admin`, `permission_change`, `external_side_effect`, `destructive`) is fully populated.

### F6. Tool activation lifecycle — PASS
`clockify_activate_group` correctly activates all tools in a group, returns `activated_tools` list, `tool_count`, and `total_visible_tools`. Handles idempotent re-activation and groups blocked by policy (via `activated_tools_blocked_by_policy`). `clockify_deactivate_group` is idempotent and correctly reports 0 tools removed when the group was not active.

### F7. Search/discovery semantics — PASS
`clockify_list_tools` with non-empty `query` searches Tier 1 catalog entries by name/description/domain/keywords and Tier 2 groups by name/description/keywords. Empty query returns all Tier 1 tools and all Tier 2 groups (unfiltered). Results are in `data.all_results` and `data.by_domain` (grouped). The `clockify_search_tools` deprecated shim delegates to `ListTools`.

### F8. Live CRUD lifecycle — PASS
Creating entries via `clockify_add_entry` with snake_case params, reading them back via `clockify_get_entry`, and deleting via `clockify_delete_entry` all work correctly against the live Clockify API. The MCP server correctly passes auth, routes requests to the right endpoints, and returns properly formatted responses.

### F9. Uninitialized access guard — PASS
Calling `tools/call` before `initialize` correctly returns JSON-RPC error code `-32002` ("server not initialized: send initialize first"). This is the MCP-defined error code for this case.

### F10. Server identity — PASS
The `initialize` response correctly advertises `serverInfo.name: "clockify-go-mcp"` as documented in `docs/clients.md`. The version string is "dev" for local builds and the actual semver for release artifacts.

### F11. Doctor command — PASS
The `clockify-mcp doctor` subcommand correctly audits the effective configuration, showing source attribution (explicit | profile | default | empty), redacting sensitive values, and reporting strict-posture findings. The `--check-backends` flag works as intended.

### F12. Minor observation: Unknown tool uses -32602 — OBSERVATION
Unknown tool names return JSON-RPC error code `-32602` (Invalid params) rather than `-32601` (Method not found). In the MCP ecosystem, this is a deliberate choice because the tool name is treated as a parameter of `tools/call`. Not a spec violation, but worth noting for clients that distinguish between method-not-found and invalid-params at the RPC level.

## Fixes made

No code changes were made. The repository is in good shape for client compatibility.

## Reproduction steps for each issue

### Verifying protocol version negotiation
```bash
export CLOCKIFY_API_KEY=<REDACTED>
export CLOCKIFY_WORKSPACE_ID=<REDACTED>
for v in "2024-11-05" "2025-03-26" "2025-06-18" "2025-11-25" "" "2099-01-01"; do
  echo "Testing protocolVersion=$v"
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"%s","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}\n' "$v" \
    | go run ./cmd/clockify-mcp 2>/dev/null | python3 -c "import sys,json; r=json.load(sys.stdin); print('  negotiated:', r['result']['protocolVersion'])"
done
```

### Reproducing CRUD lifecycle
```bash
# 1. Start with a known project ID from clockify_list_projects
# 2. Create an entry
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"qa-agent-48","version":"1.0"}}}\n{"jsonrpc":"2.0","id":2,"method":"notifications/initialized","params":{}}\n{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"clockify_add_entry","arguments":{"description":"qa-agent-48-test","start":"2025-12-31T10:00:00Z","end":"2025-12-31T10:05:00Z","project_id":"<PROJECT_ID>"}}}\n' | CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp 2>/dev/null
# 3. Read back with the returned entry ID
# 4. Delete with clockify_delete_entry
```

### Reproducing tool activation
```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"qa-agent-48","version":"1.0"}}}\n{"jsonrpc":"2.0","id":2,"method":"notifications/initialized","params":{}}\n{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"clockify_activate_group","arguments":{"name":"approvals"}}}\n' | CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp 2>/dev/null
```

## Cleanup performed

All test resources were successfully cleaned up:
- Deleted time entry `<REDACTED_ID>` (description: `qa-agent-48-mcp-compat-test`)
- Deleted time entry `<REDACTED_ID>` (description: `qa-agent-48-compat-test-2`)
- Deactivated webhooks group (2 sessions)

## Leftover test resources

None.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| F1-F11 (all PASS) | — | No issues found |
| F12 (Unknown tool code) | P3 | Cosmetic; no client observed treating -32602 differently from -32601 for unknown tools/call names |

## Files changed

No files changed.

## Suggested next action

1. **Streamable HTTP smoke test**: The stdio transport is thoroughly validated. A next step is spinning up the streamable-HTTP transport with `MCP_TRANSPORT=streamable_http` and testing all the documented HTTP-specific behaviors: `Mcp-Protocol-Version` header enforcement, SSE notification streaming, session rehydration, and cross-transport parity.

2. **gRPC transport smoke**: Build with `-tags=grpc` and verify the bidirectional stream behaves as documented for notifications/tools/list_changed.

3. **Client-specific integration tests**: While the server transport is solid, running integration tests against specific client configurations (Claude Code config, Claude Desktop config, Cursor config) from `docs/clients.md` would close the loop on end-to-end compatibility.

4. **Hosted auth mode testing**: Test OIDC, forward_auth, and mTLS auth modes against streamable_http to validate the "Supported Client Matrix" row for "Custom Streamable HTTP client."

5. **Remove deprecated clockify_search_tools alias**: The tool is marked deprecated and delegates to ListTools. Consider removing it in a future major version to reduce API surface.

## False positives / uncertainty

- The `--strict` doctor mode flags 5 errors for the local stdio setup. These are **expected** — the strict posture is designed for hosted production deployments and correctly identifies gaps that a local stdio setup intentionally does not fill (no Postgres DSN, no fail_closed audit, no inline-secrets disabled, etc.).
- The direct API call `GET /workspaces/{ws}/time-entries` returned 405 "Request method 'GET' is not supported." The MCP server uses `GET /workspaces/{ws}/user/{uid}/time-entries` instead, which is correct for per-user entry listing.

## Final recommendation

**Ship as-is for stdio-based MCP client compatibility.** The server correctly implements the MCP protocol across all four supported versions, handles errors with proper envelopes, validates parameters with JSON Schema pointers, exposes complete tool annotations, and passes live API CRUD lifecycle tests. The one minor observation (F12, unknown tool error code) does not affect any known client and is consistent with MCP ecosystem practice.

Priority for the next QA cycle: streamable-HTTP transport validation and client-specific E2E testing (Claude Desktop, Cursor, Codex configs) to close the "Untested combinations" column in the support matrix.
