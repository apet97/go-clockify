# QA Agent 13 - client-crud-safety

## Verdict
FAIL

## What I checked

- MCP server `internal/tools/clients.go` — client tool implementations
- MCP server `internal/tools/registry.go` — tool registration and schemas
- MCP server `internal/clockify/models.go` — `ClientEntity` struct
- MCP server `internal/tools/resources.go` — resource templates and ReadResource
- MCP server `tests/e2e_live_test.go` — client cleanup in live tests
- MCP server `tests/live_helpers_test.go` — `rawArchiveAndDeleteClient` helper
- API probe lab `CLIENTDOC.md` — all 5 client endpoint specs
- Live workspace `<REDACTED_ID>` — full CRUD lifecycle, edge cases, pagination, sorting

## Live API probe lab files used

| File | Purpose |
|------|---------|
| `/tmp/clockify-livetest.env` | API key, workspace ID (****REDACTED****) |
| `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLIENTDOC.md` | Client endpoint reference docs |
| `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` | Lab rules and constraints |

## Commands run

```bash
# Build check
go build ./...

# Doctor audit
go run ./cmd/clockify-mcp doctor

# Unit tests
go test ./internal/tools/ -run TestCreateClient -count=1 -v

# Vet
go vet ./tests/
```

## Live API probes run

All probes against workspace `<REDACTED_ID>` using CLOCKIFY_API_KEY=****REDACTED****.

| # | Endpoint | Method | Status | Result |
|---|----------|--------|--------|--------|
| 1 | `/v1/workspaces/{ws}/clients?page-size=5` | GET | 200 | Returned 5 clients, correct shape |
| 2 | `/v1/workspaces/{ws}/clients` | POST | 201 | Created client `<REDACTED_ID>` |
| 3 | `/v1/workspaces/{ws}/clients/{id}` | GET | 200 | Returned created client, correct shape |
| 4 | `/v1/workspaces/{ws}/clients/{id}` | PUT | 200 | Updated name, note, address. **Note: clears omitted fields** |
| 5 | `/v1/workspaces/{ws}/clients/{id}?archive-projects=true` | PUT | 200 | Archived client successfully (requires `name` in body) |
| 6 | `/v1/workspaces/{ws}/clients/{id}` | DELETE | 200 | Deleted archived client; returned deleted object |
| 7 | `/v1/workspaces/{ws}/clients/{id}` (verify) | GET | 400 | "Client doesn't belong to Workspace" — confirmed deleted |
| 8 | `/v1/workspaces/{ws}/clients/{id}` (invalid ID) | GET | 400 | "Client doesn't belong to Workspace", code 501 |
| 9 | `/v1/workspaces/{ws}/clients` (no name) | POST | 400 | "Client name is required", code 501 |
| 10 | `/v1/workspaces/{ws}/clients` (invalid email) | POST | 400 | "Please enter valid value for email", code 501 |
| 11 | `/v1/workspaces/{ws}/clients` (bad auth) | GET | 401 | "Api key does not exist", code 4003 |
| 12 | `/v1/workspaces/{ws}/clients?page-size=2&page=2` | GET | 200 | Pagination works |
| 13 | `/v1/workspaces/{ws}/clients?sort-column=NAME&sort-order=DESCENDING` | GET | 200 | Sorting works |
| 14 | DELETE on active (non-archived) client | DELETE | 400 | "Cannot delete an active client" — confirmed guard |
| 15 | PUT with only `{archived:true}` (no name) | PUT | 200 | Returned null fields — **name is required in PUT body** |

## Findings

### F1 [P0] Missing client CRUD tools — `clockify_get_client`, `clockify_update_client`, `clockify_delete_client`

The MCP server at `internal/tools/clients.go` only implements `ListClients` and `CreateClient`. Three canonical CRUD endpoints are missing:

| Tool | API endpoint | Status |
|------|-------------|--------|
| `clockify_list_clients` | GET `/v1/workspaces/{ws}/clients` | Implemented |
| `clockify_create_client` | POST `/v1/workspaces/{ws}/clients` | Implemented (incomplete — see F2) |
| `clockify_get_client` | GET `/v1/workspaces/{ws}/clients/{id}` | **MISSING** |
| `clockify_update_client` | PUT `/v1/workspaces/{ws}/clients/{id}` | **MISSING** |
| `clockify_delete_client` | DELETE `/v1/workspaces/{ws}/clients/{id}` | **MISSING** |

Compare with `projects.go` which has `GetProject`, and `entries.go` which has `GetEntry`, `UpdateEntry`, `DeleteEntry`. The same pattern should apply to clients.

**Impact**: Agents cannot read a single client by ID, update a client, or delete a client through the MCP tool surface.

### F2 [P1] `clockify_create_client` only sends `name` — missing `address`, `email`, `note`

**Registry**: `internal/tools/registry.go:179-184`
**Implementation**: `internal/tools/clients.go:40-63`

The tool schema only exposes `name` and `dry_run`. The implementation only sends `name` in the payload:

```go
// clients.go:49 — only name is sent
payload := map[string]any{"name": name}
```

The Clockify API accepts (and the live probe confirmed):
- `address` — string, 0-3000 chars
- `email` — string, email format
- `name` — string, 0-100 chars (required) ✓
- `note` — string, 0-3000 chars

The `ClientEntity` struct in `models.go` already has all five fields, so the data model is ready.

**Impact**: Agents cannot set address, email, or note when creating clients through the MCP tool.

### F3 [P1] DELETE requires archive-first — any `clockify_delete_client` tool must handle this

Live probe confirmed: Clockify API returns `400 "Cannot delete an active client"` when attempting DELETE on a non-archived client. The existing test helper `rawArchiveAndDeleteClient` (`tests/live_helpers_test.go:205`) already documents this constraint:

> Like projects, Clockify rejects DELETE on active clients; the resource must first be archived. Unlike projects, the upstream's PUT validator on /clients/{id} also requires `name` in the body (verified by direct probe: PUT {archived:true} alone returns "Client name is required").

Any future `clockify_delete_client` implementation MUST archive first (or fail with a clear error if the client is active).

### F4 [P2] PUT is full replacement, not merge — update tool needs fetch-then-merge

Live probe confirmed: `PUT /clients/{id}` with `{"name":"new-name","note":"new-note"}` **clears** `address` and `email` to null. This is a full replacement, not a merge/patch.

The entry update tool (`clockify_update_entry`) uses a fetch-then-merge pattern. Any `clockify_update_client` implementation MUST do the same: GET the existing client, merge the fields the caller wants to change, then PUT the full object back. Otherwise, omitted fields will be silently nulled.

### F5 [P2] No client resource template or ReadResource support

**File**: `internal/tools/resources.go:426-465`

`ListResourceTemplates` includes `clockify://workspace/{workspaceId}/project/{projectId}` but has no equivalent for clients. `ReadResource` has a `case "project":` branch but no `case "client":` branch.

### F6 [P2] Client cleanup in `e2e_live_test.go` was silently failing

**File**: `tests/e2e_live_test.go:124-128` (before fix)

The test's cleanup did a raw DELETE without archiving first. On active (non-archived) clients, Clockify returns "Cannot delete an active client", so the cleanup logged the error but the client was never actually deleted. Fixed in this QA run — see Fixes.

### F7 [P3] UPDATE query params `archive-projects` and `mark-tasks-as-done` not documented in MCP codebase

The Clockify API PUT endpoint supports two query parameters not reflected in any MCP code or schema:
- `archive-projects` (boolean) — archive all projects belonging to this client
- `mark-tasks-as-done` (boolean) — mark all tasks as done for this client

These should be exposed when `clockify_update_client` is implemented.

### F8 [P3] `ClientEntity.CCEmails` typed as `any`

**File**: `internal/clockify/models.go:60`

```go
CCEmails any `json:"ccEmails,omitempty"`
```

The API docs describe `ccEmails` as `Array of strings <email> [ 0 .. 3 ]`. This should be `[]string` for type safety.

## Fixes made

### Fix 1: Archive before delete in `e2e_live_test.go` client cleanup

**File**: `tests/e2e_live_test.go:124-128`

Added archive-first logic to the client cleanup path. Before the fix, the raw DELETE silently failed on active clients, leaving test resources behind. Now the cleanup archives the client before deleting it, matching the pattern already used by `rawArchiveAndDeleteClient` in other tests.

```diff
 if client.ID != "" {
+    // Archive first — Clockify rejects DELETE on active clients.
+    _ = svc.Client.Put(cleanupCtx, "/workspaces/"+wsID+"/clients/"+client.ID, map[string]any{"name": client.Name, "archived": true}, nil)
     if err := svc.Client.Delete(cleanupCtx, "/workspaces/"+wsID+"/clients/"+client.ID); err != nil {
         t.Logf("cleanup delete client %s failed: %v", client.ID, err)
     }
 }
```

Build and vet confirmed clean after the fix.

## Reproduction steps for each issue

### F1: Missing tools
1. Start the MCP server
2. Run `tools/list`
3. Observe `clockify_get_client`, `clockify_update_client`, `clockify_delete_client` are absent
4. Compare against `clockify_get_project`, `clockify_update_entry`, `clockify_delete_entry` which exist

### F2: Incomplete CreateClient
1. Call `clockify_create_client` with `address`, `email`, or `note`
2. Observe these fields are silently ignored
3. Compare against API docs which list them as accepted fields

### F3: Delete requires archive
```bash
# Create a client
curl -s -X POST -H "X-Api-Key: <REDACTED>" -H "Content-Type: application/json" \
  -d '{"name":"test-delete"}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/clients"
# Try to delete without archiving → 400 "Cannot delete an active client"
# Archive first:
curl -s -X PUT ... -d '{"name":"test-delete","archived":true}' ...
# Then delete → 200 OK
```

### F4: PUT is full replacement
```bash
# Create with all fields
curl -s -X POST ... -d '{"name":"test","address":"123 St","email":"<EMAIL>"}' ...
# Update with only name → address and email cleared to null
curl -s -X PUT ... -d '{"name":"new-name"}' ...
# GET shows address:null, email:null
```

## Cleanup performed

All `qa-agent-13-*` prefixed test clients were cleaned up:
- `<REDACTED_ID>` (qa-agent-13-test-client-1) — archived → deleted
- `<REDACTED_ID>` (qa-agent-13-full-crud) — archived → deleted
- `<REDACTED_ID>` (qa-agent-13-del-test) — archived → deleted

## Leftover test resources

None.

## Severity

| Finding | Severity | Justification |
|---------|----------|---------------|
| F1: Missing Get/Update/Delete tools | P0 | Three of five CRUD operations missing. Agents cannot read, update, or delete clients through MCP. |
| F2: CreateClient missing fields | P1 | Functional gap — agents cannot set address, email, or note on creation. |
| F3: Delete requires archive | P1 | API constraint that MUST be handled in any future delete tool. |
| F4: PUT full replacement | P2 | Design constraint — must use fetch-then-merge pattern. |
| F5: No client resource template | P2 | Clients not addressable as MCP resources. |
| F6: Test cleanup silently failing | P2 | Leftover test resources in workspace. FIXED. |
| F7: Undocumented PUT query params | P3 | Documentation gap for future implementation. |
| F8: CCEmails typed as `any` | P3 | Minor type safety issue; not currently blocking. |

## Files changed

| File | Change |
|------|--------|
| `tests/e2e_live_test.go` | Added `svc.Client.Put()` archive step before client deletion in cleanup |

## Suggested next action

1. **Implement `clockify_get_client`**: Add to `clients.go` and `registry.go`, following the `GetProject` pattern. Small, self-contained change.
2. **Implement `clockify_delete_client`**: Fetch client → archive if active → delete. Follow the `rawArchiveAndDeleteClient` pattern in `tests/live_helpers_test.go:205`.
3. **Implement `clockify_update_client`**: Fetch-then-merge pattern, matching `clockify_update_entry`. Expose `archive-projects` and `mark-tasks-as-done` as boolean flags.
4. **Expand `clockify_create_client`**: Add `address`, `email`, `note` to schema and payload.
5. **Add client resource template**: `clockify://workspace/{workspaceId}/client/{clientId}` in `ListResourceTemplates` + `ReadResource`.
6. **Fix `CCEmails` type**: Change `any` to `[]string` in `models.go:60`.

## False positives / uncertainty

- **F3/F4**: The archive-first and full-replacement behaviors are confirmed by direct live API probes. No uncertainty.
- **F2**: The live probe confirmed that the API accepts and returns `address`, `email`, `note`. The fields are safe to add.
- **F8**: The `any` type on `CCEmails` hasn't caused observable bugs yet because no code reads/writes it through the MCP tool surface. Migration to `[]string` should be done alongside F2 implementation to validate with live responses.

## Final recommendation

The client CRUD surface is **incomplete and under-implemented**. Only 2 of 5 endpoints are covered, and the create tool is missing 3 of 4 optional fields. There are no architectural blockers — the `ClientEntity` model, API client, and path utilities are all ready. The missing pieces are straightforward tool implementations following patterns already established for projects and time entries. The one non-obvious constraint discovered (archive-before-delete) is already documented and handled in the test helpers. **Priority actions**: implement `clockify_get_client` (smallest, highest-impact), then `clockify_delete_client`, then `clockify_update_client`, then expand `clockify_create_client`.
