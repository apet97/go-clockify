# QA Agent 10 - clockify-workspace-isolation

## Verdict
**PASS WITH CONCERNS** — One P2 defense-in-depth gap found and fixed. All critical workspace isolation controls are in place and functioning correctly.

## What I checked

1. **Workspace ID resolution architecture** — traced `CLOCKIFY_WORKSPACE_ID` from env → config.Load → Service.WorkspaceID → ResolveWorkspaceID → paths.Workspace → every handler
2. **Multi-workspace auto-detection safety** — verified the server refuses to auto-detect when an API key has access to 25 workspaces
3. **Cross-workspace data isolation** — live API probes confirmed resources created in one workspace do not appear in another
4. **ID validation at each layer** — config-load, path construction, and (after fix) ResolveWorkspaceID all validate workspace IDs
5. **Path safety static enforcement** — verified the static test correctly exempts workspace IDs (validated at config load) while requiring non-workspace ID validation
6. **Doctor command** — verified it surfaces CLOCKIFY_WORKSPACE_ID configuration correctly
7. **Build and test integrity** — confirmed the fix compiles and passes all existing tests

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key and workspace ID (source only, never written)
- `probes/lib/common.sh` — probe library (referenced for patterns, not executed)
- `CLAUDE.md` — safety rules
- `README.md` — lab overview

## Commands run

```bash
# Build
go build ./cmd/clockify-mcp

# Doctor (with workspace ID)
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp doctor --strict

# Doctor (without workspace ID — startup OK, tools would fail on multi-workspace)
CLOCKIFY_API_KEY=<REDACTED> go run ./cmd/clockify-mcp doctor

# Existing tests
go test ./internal/tools/ ./internal/paths/ ./internal/config/ ./internal/resolve/ -count=1

# Config workspace ID validation tests
go test ./internal/config/ -run "Workspace" -v -count=1
```

## Live API probes run

### Probe 1: Workspace listing
```bash
GET /api/v1/workspaces → 200, 25 workspaces returned
```
Confirmed the API key has broad workspace access — the MCP server's multi-workspace detection is load-bearing.

### Probe 2: Cross-workspace data isolation
```
1. Created project "qa-agent-10-{ts}-cross-ws-test" in target workspace (201)
2. Verified project visible in target workspace (200, count=1)
3. Verified project NOT visible in other workspace (200, count=0)
4. Verified other workspace's projects NOT visible in target workspace (200, count=0)
```
**Result: Cross-workspace isolation confirmed.** The Clockify API correctly scopes resources per workspace, and the MCP server's workspace-scoped path construction preserves this boundary.

### Probe 3: Workspace details and user context
```
GET /workspaces/{ws} → workspace details with correct ID, memberships, settings
GET /user → activeWorkspace = <REDACTED_ID> (NOT the probe workspace)
```
Confirmed user's activeWorkspace is different from the configured workspace — validates that CLOCKIFY_WORKSPACE_ID explicit setting is essential.

## Findings

### Finding 1 (P2 — FIXED): ResolveWorkspaceID returned unvalidated explicit workspace ID

**Location:** `internal/tools/workspaces.go:20-25`

**What:** `ResolveWorkspaceID` returned `s.WorkspaceID` directly without validation when explicitly set. The validation was deferred to `paths.Workspace()` called by each handler. While every existing handler uses `paths.Workspace()` (enforced by the static path safety test), this was a defense-in-depth gap: a future handler using the workspace ID directly in path construction would bypass validation.

**Why it matters:** The path safety static test (`TestPathSafety_HandlersValidateIDsBeforeConcat`) explicitly exempts workspace IDs from its check because "they are validated at config load." But `ResolveWorkspaceID` can also return the explicit workspace ID set via `Service.WorkspaceID` (set by `New(client, workspaceID)`), which bypasses config-load validation for programmatic wiring paths.

**Fix:** Added `resolve.ValidateID(s.WorkspaceID, "workspace_id")` in `ResolveWorkspaceID` before returning the explicit workspace ID. This means every path through `ResolveWorkspaceID` now validates:
- Explicit workspace ID → validated inline (new)
- Cached auto-detected ID → was already validated before caching (via `/workspaces` response)
- Fresh auto-detected ID → comes from Clockify API (trusted upstream)

**Verification:** All 6 workspace isolation test cases pass after fix (empty, path traversal, invalid chars, valid, null byte, oversized).

### Finding 2 (P1 — CONFIRMED SAFE): Multi-workspace auto-detection correctly fails-closed

**What:** When `CLOCKIFY_WORKSPACE_ID` is unset and the API key has 25 workspaces, `ResolveWorkspaceID` correctly returns `"multiple workspaces found; set CLOCKIFY_WORKSPACE_ID"`. This prevents accidental cross-workspace operations.

**Why this matters:** Without explicit configuration, the MCP server could silently pick the wrong workspace. The multi-workspace detection is a critical safety guard that forces explicit operator intent.

### Finding 3 (GOOD): Three-layer workspace ID validation

The workspace isolation architecture has three layers of defense, each independently validating the workspace ID:

| Layer | File | Mechanism |
|-------|------|-----------|
| Config load | `internal/config/config.go:297-300` | `resolve.ValidateID(cfg.WorkspaceID, "CLOCKIFY_WORKSPACE_ID")` |
| ResolveWorkspaceID | `internal/tools/workspaces.go:21-23` | `resolve.ValidateID(s.WorkspaceID, "workspace_id")` (added by this fix) |
| Path construction | `internal/paths/paths.go:41` | `resolve.ValidateID(wsID, "workspace_id")` + `url.PathEscape` |

### Finding 4 (GOOD): All 27 handler files use paths.Workspace for workspace-scoped paths

Every handler file that constructs workspace-scoped API paths goes through `paths.Workspace()`, which validates the workspace ID and percent-encodes all path segments. This is verified by:
- Code audit across all 27 files referencing `ResolveWorkspaceID`
- The static test `TestPathSafety_HandlersValidateIDsBeforeConcat` that enforces non-workspace ID validation

### Finding 5 (GOOD): Resolve cache keys include workspace scope

The name-to-ID resolve cache (`resolve_cache.go`) includes the workspace ID in the cache key scope, ensuring cached results from one workspace cannot be returned for another workspace's queries.

## Fixes made

### Fix: `internal/tools/workspaces.go` — Add ValidateID in ResolveWorkspaceID

**Before:**
```go
func (s *Service) ResolveWorkspaceID(ctx context.Context) (string, error) {
    if s.WorkspaceID != "" {
        return s.WorkspaceID, nil  // unvalidated
    }
```

**After:**
```go
func (s *Service) ResolveWorkspaceID(ctx context.Context) (string, error) {
    if s.WorkspaceID != "" {
        if err := resolve.ValidateID(s.WorkspaceID, "workspace_id"); err != nil {
            return "", err
        }
        return s.WorkspaceID, nil
    }
```

Also added `"github.com/apet97/go-clockify/internal/resolve"` to imports.

## Reproduction steps for each issue

### Finding 1 (P2)

1. Create a Service with a malicious workspace ID: `svc := tools.New(client, "../../etc/passwd")`
2. Call `svc.ResolveWorkspaceID(ctx)` — before fix, returned `"../../etc/passwd"` without error
3. After fix, returns `"workspace_id contains invalid characters"` error

### Finding 2 (P1 — verified safe)

1. Set `CLOCKIFY_API_KEY` only (no `CLOCKIFY_WORKSPACE_ID`) with a key that has 25 workspaces
2. Start the server: `go run ./cmd/clockify-mcp`
3. Call any workspace-scoped tool — returns `"multiple workspaces found; set CLOCKIFY_WORKSPACE_ID"`

## Cleanup performed

- Project `qa-agent-10-{ts}-cross-ws-test` (id=`<REDACTED_ID>`) — DELETE returned 400, ARCHIVE returned 405. Could not clean up via API.
- Temporary test file `workspace_isolation_agent10_test.go` — removed
- Temporary script `/tmp/ws_iso_check.go` — removed

## Leftover test resources

| Resource | ID | Workspace | Notes |
|----------|----|-----------|-------|
| Project "qa-agent-10-{ts}-cross-ws-test" | `<REDACTED_ID>` | `<REDACTED_ID>` | Could not delete via API (400 on DELETE, 405 on PATCH archive). Safe to leave — prefixed, tiny, no time entries. |

## Severity

| ID | Severity | Description | Status |
|----|----------|-------------|--------|
| Finding 1 | P2 | ResolveWorkspaceID unvalidated return | FIXED |
| Finding 2 | P1 (safe) | Multi-workspace detection works correctly | VERIFIED |
| Finding 3 | INFO | Three-layer validation architecture | DOCUMENTED |
| Finding 4 | INFO | All handlers use paths.Workspace | VERIFIED |
| Finding 5 | INFO | Resolve cache scoped by workspace | VERIFIED |

## Files changed

- `internal/tools/workspaces.go` — Added `resolve.ValidateID` call + import (3 lines added)

## Suggested next action

1. **Accept the fix** — the `ResolveWorkspaceID` validation is a minimal, safe defense-in-depth improvement
2. **Add config-load validation test for workspace ID injected via New()** — the existing `TestLoad_RejectsBadWorkspaceID` tests config.Load validation, but there's no test validating that `New(client, badID)` followed by `ResolveWorkspaceID` fails (now it does, but should be locked in)
3. **Consider adding a `CLOCKIFY_WORKSPACE_ID` presence check to `doctor --strict`** — currently doctor only checks strict-posture settings (hosted profiles). A note that workspace ID should be set when the API key has multiple workspaces would be a helpful proactive check
4. **Clean up leftover test project** — `<REDACTED_ID>` in workspace `<REDACTED_ID>`. Manual cleanup via Clockify web UI or a future API call with different parameters

## False positives / uncertainty

- The `doctor --strict` reported 4 "ERROR" findings related to hosted-strict-posture settings (MCP_DISABLE_INLINE_SECRETS, MCP_CONTROL_PLANE_DSN, MCP_AUDIT_DURABILITY, CLOCKIFY_POLICY). These are expected for a local-stdio deployment and are NOT workspace isolation issues.
- The `go.work.sum` diff in the git changes is unrelated — it's Go toolchain metadata updated when running tests.
- The pre-existing diagnostic on `tier2_config_test.go:269` (tautological condition) is unrelated to workspace isolation.

## Final recommendation

**The workspace isolation architecture is sound.** The three-layer validation (config → ResolveWorkspaceID → paths.Workspace) plus the multi-workspace auto-detection safety gate provide robust defense against cross-workspace operations. The P2 fix closes the only identified gap — a defense-in-depth issue where the middle layer (ResolveWorkspaceID) didn't validate before returning. All handler code uses workspace-scoped paths, the resolve cache is keyed by workspace, and cross-workspace isolation was confirmed via live API probes.

**Recommended for release with the one-line fix applied.**
