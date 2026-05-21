# QA Agent 11 - tenant-boundary-safety

## Verdict
PASS WITH CONCERNS

## What I checked

- **Workspace ID injection prevention**: Verified that no MCP tool accepts a `workspace_id` parameter from the caller. All tools derive workspace ID from `Service.ResolveWorkspaceID(ctx)`, which returns the configured `CLOCKIFY_WORKSPACE_ID` or auto-detects from a single-workspace API key. No caller-controlled override exists.
- **Cross-workspace access via API**: Tested direct Clockify API calls attempting to read/update resources across workspace boundaries. The Clockify API correctly rejects cross-workspace resource access at the individual resource level (HTTP 400 "Project doesn't belong to Workspace").
- **Multi-tenant deployment isolation**: Inspected `internal/runtime/service.go::tenantRuntime()` — each tenant gets its own credential ref, API key, workspace ID, and base URL resolved from the control plane. The shared-service E2E test (`internal/controlplane/postgres/e2e_shared_service_test.go`) verifies tenant isolation invariants: audit events are tenant-scoped, sessions carry the correct tenant ID, per-tenant policy_mode is honored.
- **Path injection surface**: The `paths.Workspace()` function validates workspace IDs and percent-encodes sub-segments. `resolve.ValidateID()` rejects `/`, `?`, `#`, `%`, `..`, and control characters in non-workspace IDs. Path safety tests (`path_safety_test.go`) enforce that every handler file validates non-workspace IDs before concatenation.
- **Auto-detection behavior**: When `CLOCKIFY_WORKSPACE_ID` is unset and the API key has access to multiple workspaces, `ResolveWorkspaceID` fails-closed with "multiple workspaces found; set CLOCKIFY_WORKSPACE_ID".
- **Workspace enumeration via MCP**: The `clockify_list_workspaces` tool calls `GET /workspaces` (no workspace scoping) and returns ALL workspaces the API key can access.
- **User context leakage**: `clockify_whoami` exposes `activeWorkspace` and `defaultWorkspace` fields which may differ from the configured workspace.
- **Doctor command**: `go run ./cmd/clockify-mcp/ doctor --strict` shows effective configuration including workspace ID scoping. Strict-hosted posture correctly requires `MCP_DISABLE_INLINE_SECRETS`, `MCP_CONTROL_PLANE_DSN`, `MCP_AUDIT_DURABILITY=fail_closed`, and `CLOCKIFY_POLICY` no broader than `time_tracking_safe`.
- **Build verification**: `go build ./cmd/clockify-mcp/` succeeds cleanly.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID, and confirmation variable (redacted)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — lab rules and constraints
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/WORKSPACESDOC.md` — workspace API documentation
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/USERDOC.md` — user API documentation

## Commands run

```
# Build
go build ./cmd/clockify-mcp/

# Doctor
CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  go run ./cmd/clockify-mcp/ doctor --strict

# MCP stdio initialize + tool calls
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | \
  CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  go run ./cmd/clockify-mcp/

# MCP tools/call: clockify_list_workspaces
# MCP tools/call: clockify_whoami
```

## Live API probes run

| # | Probe | Method | Result |
|---|-------|--------|--------|
| 1 | List all workspaces via direct API | GET /workspaces | API key has access to 25 workspaces |
| 2 | List projects in configured workspace | GET /workspaces/{CONFIGURED}/projects | 50 projects returned |
| 3 | List projects in different workspace | GET /workspaces/{OTHER}/projects | 0 projects (empty workspace, access granted) |
| 4 | List time entries in different workspace | GET /workspaces/{OTHER}/time-entries | 3000 entries returned (cross-workspace access possible via direct API) |
| 5 | Create test project in configured workspace | POST /workspaces/{CONFIGURED}/projects | HTTP 201, created <REDACTED_ID> |
| 6 | Read test project via different workspace path | GET /workspaces/{OTHER}/projects/{ID} | HTTP 400 "Project doesn't belong to Workspace" |
| 7 | Update test project via different workspace path | PUT /workspaces/{OTHER}/projects/{ID} | HTTP 400 "Project doesn't belong to Workspace" |
| 8 | List users in different workspace | GET /workspaces/{OTHER}/users | 1 user returned (cross-workspace listing possible) |
| 9 | Whoami via direct API | GET /user | activeWorkspace differs from configured workspace |
| 10 | MCP clockify_list_workspaces | tools/call | Returns all 25 workspace names + IDs |
| 11 | MCP clockify_whoami | tools/call | Exposes activeWorkspace and defaultWorkspace |

## Findings

### Finding 1 [P2] — clockify_list_workspaces enumerates all accessible workspaces

**Description**: The `clockify_list_workspaces` tool calls `GET /workspaces` which returns all workspaces the API key has access to. In the probe lab, this exposed 25 workspace names and IDs, many of which are unrelated to the configured workspace.

**Impact**: Information disclosure — an attacker or unauthorized user who gains access to the MCP server can enumerate all workspaces associated with the API key, revealing organizational structure (workspace names like "COMPANY B", "WEBHOOKS", "TIME OFF", "STASK1" through "STASK9").

**Root cause**: The Clockify `/workspaces` endpoint is unscoped — it returns all workspaces for the authenticated API key. There is no query parameter to filter by a specific workspace ID.

**Recommendation**:
- Operators should use API keys scoped to a single workspace (Clockify supports workspace-scoped API keys).
- Optionally, the MCP server could filter the response to only include the configured workspace ID, or add a config flag `CLOCKIFY_HIDE_OTHER_WORKSPACES` that suppresses non-configured workspaces from the list.

### Finding 2 [P3] — clockify_whoami leaks cross-workspace context

**Description**: The `clockify_whoami` tool returns the user's `activeWorkspace` and `defaultWorkspace` fields from the Clockify `/user` endpoint. In the probe lab, these differ from the configured workspace.

**Impact**: Minor information disclosure — reveals that the API key owner operates across multiple workspaces and exposes workspace IDs outside the configured scope. Low severity because workspace IDs alone are not secrets.

**Recommendation**: Consider redacting `activeWorkspace` and `defaultWorkspace` from the `clockify_whoami` response when they differ from the configured workspace, or adding a note to the operator documentation.

### Finding 3 [P3] — Credential hygiene: broad API key scope

**Description**: The API key used in the probe lab has access to 25 workspaces. This is a credential management concern, not an MCP server bug. The MCP server correctly scopes all operations to the configured `CLOCKIFY_WORKSPACE_ID`, but the underlying API key's broad access increases blast radius if the key is compromised.

**Impact**: Operational risk — a compromised API key grants access to 25 workspaces instead of 1.

**Recommendation**: Use workspace-scoped API keys for production deployments.

## Fixes made

No code fixes were made. All findings are configuration/operational concerns, not code defects. The MCP server's tenant-boundary enforcement architecture is correct:

1. No tool accepts a caller-supplied `workspace_id` parameter
2. All tools derive workspace from configured `CLOCKIFY_WORKSPACE_ID` via `ResolveWorkspaceID()`
3. Multi-tenant deployment isolates tenants via separate credential refs
4. Path construction validates all IDs
5. Fails-closed when workspace is ambiguous

## Reproduction steps for each issue

### Finding 1 — workspace enumeration

```bash
CLOCKIFY_API_KEY=****REDACTED**** CLOCKIFY_WORKSPACE_ID=<REDACTED_ID> \
  go run ./cmd/clockify-mcp/
# Send initialize + tools/call for clockify_list_workspaces
# Observe response includes ALL workspaces, not just the configured one
```

### Finding 2 — workspace context leak

```bash
# Same setup, call clockify_whoami
# Observe activeWorkspace and defaultWorkspace may differ from CLOCKIFY_WORKSPACE_ID
```

### Finding 3 — broad API key scope

```bash
curl -s -H "X-Api-Key: ****REDACTED****" "https://api.clockify.me/api/v1/workspaces" | jq length
# Returns 25 (multiple workspaces)
```

## Cleanup performed

- Created test project `qa-agent-11-tenant-boundary-test` (ID: `<REDACTED_ID>`) in configured workspace — archived then deleted. Confirmed deletion returned HTTP 200.

## Leftover test resources

None. All test resources were cleaned up successfully.

## Severity

| Severity | Count | Description |
|----------|-------|-------------|
| P0 | 0 | No critical tenant-boundary bypasses found |
| P1 | 0 | No high-severity isolation failures |
| P2 | 1 | Workspace enumeration via clockify_list_workspaces |
| P3 | 2 | Workspace context leak via whoami; broad API key scope |

## Files changed

None.

## Suggested next action

1. **Documentation**: Add a note in operator docs recommending single-workspace API keys for production deployments.
2. **Optional hardening**: Consider adding a `CLOCKIFY_HIDE_OTHER_WORKSPACES` config flag (default false) that, when enabled, filters `clockify_list_workspaces` to only show the configured workspace.
3. **Optional hardening**: Consider redacting `activeWorkspace`/`defaultWorkspace` from `clockify_whoami` output when they differ from the configured workspace ID.
4. **Probe lab**: Consider generating a workspace-scoped API key for the probe workspace to match production best practices.

## False positives / uncertainty

- **Workspace enumeration severity**: The severity of Finding 1 (P2) depends on deployment context. For self-hosted single-tenant deployments, workspace enumeration is expected behavior and not a concern. For multi-tenant hosted deployments, this information could reveal organizational structure. The current behavior is correct for single-tenant use and the MCP server does not claim to hide workspace enumeration.
- **Cross-workspace list access**: The Clockify API allows listing resources across workspaces when using a multi-workspace API key. This is Clockify's API design, not an MCP server vulnerability. The MCP server cannot prevent this — it correctly scopes operations to the configured workspace, but the underlying API key's permissions are beyond its control.

## Final recommendation

The MCP server's tenant-boundary safety is well-implemented. The architecture correctly enforces workspace isolation: no tool accepts a caller-controlled workspace ID, all operations are scoped to the configured workspace, multi-tenant deployments isolate tenants through separate credential refs, and path construction validates all IDs. The concerns raised are operational/configurational rather than code defects. The server is ready for community/self-hosted use with the recommendation that operators use workspace-scoped API keys.
