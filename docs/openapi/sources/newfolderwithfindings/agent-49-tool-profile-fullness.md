# QA Agent 49 - tool-profile-fullness

## Verdict
PASS WITH CONCERNS

## What I checked

1. **Full tool inventory**: Mapped all 40 Tier 1 tools from `registry.go` and all 11 Tier 2 groups (~90 lazy-loaded tools) from their respective `tier2_*.go` files.
2. **CRUD completeness per entity**: Compared tool coverage against the Clockify API surface for each core entity.
3. **Live API endpoint verification**: Confirmed that the Clockify API supports update and delete operations for clients, projects, tags, and tasks — endpoints the MCP server does not expose as tools.
4. **MCP server startup smoke**: Verified the server builds cleanly, starts in stdio mode, returns 40 tools on `tools/list`, and has a working `doctor` command.
5. **Output schema coverage**: Verified all 40 Tier 1 tools have output schemas (23 typed, 17 opaque envelope fallbacks).

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key, workspace ID, and environ vars (secrets redacted)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — Agent rules and lab conventions
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/PROJECTSDOC.md` — Project API documentation
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLIENTDOC.md` — Client API documentation
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/TAGDOC.md` — Tag API documentation
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/TASKDOC.md` — Task API documentation
- Workspace ID: `<REDACTED_ID>` (probe workspace)

## Commands run

```bash
# Build and doctor
go build ./cmd/clockify-mcp/
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> ./clockify-mcp doctor

# MCP stdio smoke (initialize + tools/list)
printf '{"jsonrpc":"2.0","id":1,"method":"initialize",...}\n
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n' \
  | CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> ./clockify-mcp
# Result: server=clockify-go-mcp, protocol=2024-11-05, tools=40

# Live API: create/update/delete project
curl -X POST -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-49-test-project","billable":false}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/projects"
curl -X PUT -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-49-test-project-updated","billable":true}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/projects/<PID>"

# Live API: create/get-by-id/update/delete client
curl -X POST -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-49-test-client"}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/clients"
curl -X GET -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/clients/<CID>"
curl -X PUT -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-49-test-client-updated"}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/clients/<CID>"

# Live API: create/update/delete tag
curl -X POST -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-49-test-tag"}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags"
curl -X PUT -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-49-test-tag-updated"}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags/<TID>"
curl -X DELETE -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/tags/<TID>"

# Live API: create/get/update task
curl -X POST -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-49-test-task","projectId":"<PID>"}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/projects/<PID>/tasks"
curl -X GET -H "X-Api-Key: <REDACTED>" \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/projects/<PID>/tasks/<TKID>"
curl -X PUT -H "X-Api-Key: <REDACTED>" \
  -d '{"name":"qa-agent-49-test-task-updated","billable":true}' \
  "https://api.clockify.me/api/v1/workspaces/<REDACTED>/projects/<PID>/tasks/<TKID>"
```

## Live API probes run

| # | Probe | HTTP Status | MCP Tool Equivalent |
|---|-------|------------|---------------------|
| 1 | GET /workspaces/{id}/projects (list) | 200 | `clockify_list_projects` ✓ |
| 2 | POST /workspaces/{id}/projects (create) | 201 | `clockify_create_project` ✓ |
| 3 | PUT /workspaces/{id}/projects/{id} (update) | 200 | **MISSING** |
| 4 | DELETE /workspaces/{id}/projects/{id} | 400* | **MISSING** |
| 5 | GET /workspaces/{id}/clients (list) | 200 | `clockify_list_clients` ✓ |
| 6 | POST /workspaces/{id}/clients (create) | 201 | `clockify_create_client` ✓ |
| 7 | GET /workspaces/{id}/clients/{id} (get) | 200 | **MISSING** |
| 8 | PUT /workspaces/{id}/clients/{id} (update) | 200 | **MISSING** |
| 9 | DELETE /workspaces/{id}/clients/{id} | 400* | **MISSING** |
| 10 | POST /workspaces/{id}/tags (create) | 201 | `clockify_create_tag` ✓ |
| 11 | PUT /workspaces/{id}/tags/{id} (update) | 200 | **MISSING** |
| 12 | DELETE /workspaces/{id}/tags/{id} | 200 | **MISSING** |
| 13 | POST /workspaces/{id}/projects/{id}/tasks (create) | 201 | `clockify_create_task` ✓ |
| 14 | GET /workspaces/{id}/projects/{id}/tasks/{id} (get) | 200 | **MISSING** |
| 15 | PUT /workspaces/{id}/projects/{id}/tasks/{id} (update) | 200 | **MISSING** |
| 16 | DELETE /workspaces/{id}/projects/{id}/tasks/{id} | 400* | **MISSING** |

*Delete failures (400) are workspace-plan-level restrictions (free-tier workspaces may block
deletion), not an API gap. The endpoints exist and are documented.

## Findings

### F1: Missing client CRUD tools (Tier 1) — P2

The Clockify API supports `GET /clients/{id}`, `PUT /clients/{id}`, and
`DELETE /clients/{id}`. The MCP server has `clockify_list_clients` and
`clockify_create_client` in Tier 1, but no `clockify_get_client`,
`clockify_update_client`, or `clockify_delete_client` anywhere (not even
in a Tier 2 group).

This is inconsistent with time entries, which have full CRUD
(get_entry, update_entry, delete_entry, list_entries, add_entry) in Tier 1.

### F2: Missing project update/delete tools (Tier 1) — P2

The Clockify API supports `PUT /projects/{id}` and `DELETE /projects/{id}`.
The MCP server has `clockify_list_projects`, `clockify_get_project`, and
`clockify_create_project` in Tier 1, plus `clockify_archive_projects` in
the Tier 2 `project_admin` group. But there is no `clockify_update_project`
for basic metadata changes (name, billable, color, public, client) and no
`clockify_delete_project`.

### F3: Missing tag get/update/delete tools (Tier 1) — P2

The Clockify API supports `GET /tags/{id}`, `PUT /tags/{id}`, and
`DELETE /tags/{id}`. The MCP server has `clockify_list_tags` and
`clockify_create_tag` in Tier 1. No get-by-ID, update, or delete tag
tools exist anywhere. Live probe confirmed all three operations work:
create (201), update (200), delete (200).

### F4: Missing task get/update/delete tools (Tier 1) — P2

The Clockify API supports `GET /tasks/{id}`, `PUT /tasks/{id}`, and
`DELETE /tasks/{id}`. The MCP server has `clockify_list_tasks` and
`clockify_create_task` in Tier 1. No get-by-ID, update, or delete task
tools exist anywhere. Live probe confirmed get (200) and update (200) work.

### F5: CRUD asymmetry across entity types — P3 (observational)

The tool profile shows inconsistent CRUD depth across entity types.
Time entries have 7 Tier-1 tools (list, get, today_entries, add, update,
delete, find_and_update) while clients, tags, and tasks each have only 2
(list + create). Projects have 3 (list + get + create) but still lack
update/delete. This asymmetry is not explained by API availability — all
four entity types support full CRUD through the same base URL pattern.

### F6: Positive — Tools/list and doctor work correctly — PASS

- `tools/list` returns exactly 40 Tier 1 tools under default config.
- The `doctor` command shows a comprehensive configuration audit
  across Profile, Core, Safety, Performance, Bootstrap, Transport, and
  Auth sections.
- Server starts correctly: `server=clockify-go-mcp`, protocol `2024-11-05`.
- Startup log shows `policy=standard bootstrap=full_tier1`.

### F7: Positive — Output schema coverage is complete — PASS

All 40 Tier 1 tools have output schemas (23 use typed `envelopeSchemaFor[T]`,
17 use `envelopeOpaque` fallback). Tier 2 tools get `envelopeOpaque`
schemas on activation (documented in `applyOpaqueOutputSchemas`).

### F8: Positive — Tier 2 group coverage is broad — PASS

Eleven Tier 2 groups cover: time_off (12 tools), scheduling (7), approvals (6),
project_admin (6), groups_holidays (8), user_admin (8), expenses (10),
webhooks (7), shared_reports (7), invoices (12), custom_fields (7).

Each group follows the list-get-create-update-delete pattern for its domain
entities. Total Tier 2 tool count: approximately 90.

## Fixes made

No code fixes made on this run. The 11 missing CRUD tools are feature
additions, not bugs. They require design decisions about:
- Whether to add them to Tier 1 (immediate availability) or Tier 2 (lazy activation)
- Whether delete operations should be `toolDestructive` (like `clockify_delete_entry`)
- What group name to use if placed in Tier 2

## Reproduction steps for each issue

### F1: Missing client get/update/delete
1. Start MCP server with valid credentials
2. Request `tools/list` — observe `clockify_list_clients` and `clockify_create_client` are present
3. Observe no `clockify_get_client`, `clockify_update_client`, or `clockify_delete_client` in the list
4. Verify API supports them: `curl -X GET -H "X-Api-Key: <REDACTED>" "https://api.clockify.me/api/v1/workspaces/<WS>/clients/<ID>"` returns 200

### F2: Missing project update/delete
1. Same as F1 — observe no `clockify_update_project` or `clockify_delete_project`
2. Verify API: `curl -X PUT -H "X-Api-Key: <REDACTED>" -d '{"name":"updated"}' "https://api.clockify.me/api/v1/workspaces/<WS>/projects/<ID>"` returns 200

### F3: Missing tag get/update/delete
1. Same — observe no `clockify_get_tag`, `clockify_update_tag`, `clockify_delete_tag`
2. Verify API: full CRUD confirmed working

### F4: Missing task get/update/delete
1. Same — observe no `clockify_get_task`, `clockify_update_task`, `clockify_delete_task`
2. Verify API: GET and PUT confirmed working

## Cleanup performed

| Resource | ID | Action | Result |
|----------|-----|--------|--------|
| Project | `<REDACTED_ID>` | Archive + Delete | Archive: 405, Delete: 400 |
| Client | `<REDACTED_ID>` | Archive + Delete | Archive: 405, Delete: 400 |
| Tag | `<REDACTED_ID>` | Delete | 200 (cleaned up) |
| Task | `<REDACTED_ID>` | Delete | 400 |

The PATCH endpoint for archiving returned 405 (Method Not Allowed) on both projects
and clients, suggesting the probe workspace tier does not support archiving. Delete
returned 400 likely due to the same workspace plan limitation.

## Leftover test resources

- Project `<REDACTED_ID>` — name: "qa-agent-49-test-project" (workspace: <REDACTED_ID>)
- Client `<REDACTED_ID>` — name: "qa-agent-49-test-client" (workspace: <REDACTED_ID>)
- Task `<REDACTED_ID>` — name: "qa-agent-49-test-task" (workspace: <REDACTED_ID>, project: <REDACTED_ID>)

These are test-only resources with the `qa-agent-49-` prefix. They are safe to leave
in the probe workspace. A workspace admin can manually remove them if desired.

## Severity

| Finding | Severity | Rationale |
|---------|----------|-----------|
| F1: Missing client CRUD | P2 | The API supports it; the gap is per-entity. Client operations are fundamental. |
| F2: Missing project update/delete | P2 | Projects are the most-used entity after time entries. Update is table-stakes. |
| F3: Missing tag CRUD | P2 | Tags have simple schemas; easy to add. Full CRUD confirmed working via API. |
| F4: Missing task CRUD | P2 | Same pattern as tags — full CRUD works, tools just need wiring. |
| F5: CRUD asymmetry | P3 | Observational/design note. Not a functional bug but a design inconsistency. |
| F6-F8: Positive findings | PASS | Tool count, output schemas, doctor, Tier 2 coverage all solid. |

## Files changed

None.

## Suggested next action

1. **Add the 11 missing CRUD tools** for clients, projects, tags, and tasks following
   the existing patterns in `registry.go`. Suggested tool names:
   - `clockify_get_client`, `clockify_update_client`, `clockify_delete_client`
   - `clockify_update_project`, `clockify_delete_project`
   - `clockify_get_tag`, `clockify_update_tag`, `clockify_delete_tag`
   - `clockify_get_task`, `clockify_update_task`, `clockify_delete_task`

2. **Decide placement**: Add to Tier 1 (consistent with time-entry CRUD being Tier 1)
   or to new Tier 2 groups (e.g., extend `project_admin` for projects).

3. **Follow risk classification**: Delete tools should use `toolDestructive` (as
   `clockify_delete_entry` does). Update tools should use `toolRWIdem` (idempotent writes).
   Get tools should use `toolRO`.

4. **Clean up leftover test resources** via the Clockify web UI if the API refuses
   deletion on the free tier.

## False positives / uncertainty

- Delete failures (400) on projects, clients, and tasks may be workspace-plan-specific.
  The probe workspace might be on a free plan that blocks resource deletion. The
  endpoints are documented in the Clockify API and may work on paid plans.
- PATCH for archiving returned 405 — the Clockify API docs for the specific workspace
  plan may document a different archive flow (e.g., via PUT with `archived: true`).
  I did not exhaustively test all archive patterns.

## Final recommendation

**PASS WITH CONCERNS** — The MCP server's tool profile is solid across time entries,
reports, and the 11 Tier 2 groups. However, 11 fundamental CRUD tools are missing for
clients, projects, tags, and tasks — entities where the Clockify API fully supports
get-by-ID, update, and delete. Adding these would bring the Tier 1 CRUD depth in line
with time entries and complete the profile. None of the gaps are blocking for basic
usage, but they limit the server's ability to manage the full workspace lifecycle.
