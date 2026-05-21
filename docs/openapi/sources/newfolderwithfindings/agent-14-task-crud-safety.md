# QA Agent 14 - task-crud-safety

## Verdict
PASS WITH CONCERNS

## What I checked

1. **Task model (`internal/clockify/models.go`)** — Verified the `Task` struct matches the live API response shape (TASKDOC.md).
2. **Tier 1 task tools** — `clockify_list_tasks` (RO) and `clockify_create_task` (RW) operate correctly against the live API.
3. **Missing Tier 2 task tools** — No `clockify_get_task`, `clockify_update_task`, or `clockify_delete_task` tools exist anywhere in the codebase. Full task CRUD lifecycle is incomplete.
4. **`billable` field `omitempty` bug** — `Billable bool \`json:"billable,omitempty"\`` in the Task struct dropped `false` values, causing data loss in MCP responses.
5. **Missing `estimate` and `status` fields in CreateTask** — The `clockify_create_task` tool only sends `name` and optionally `billable`; does not support `estimate` (ISO-8601) or `status` (ACTIVE/DONE).
6. **Missing filtering on ListTasks** — The `clockify_list_tasks` tool only supports pagination (page/page-size); the live API also supports `name`, `strict-name-search`, `is-active`, `sort-column`, and `sort-order` query parameters.
7. **`clockify_resolve_name`** — Correctly resolves tasks by name within a project scope.
8. **Doctor command** — Runs cleanly, config is detected correctly.
9. **MCP stdio transport** — Initializes correctly, tools/list includes both task tools.
10. **Dry-run** — Works correctly for `clockify_create_task`.

## Live API probe lab files used

- `/tmp/clockify-livetest.env` — API key and workspace ID (redacted)
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/TASKDOC.md` — Clockify Task API reference docs
- `/Users/15x/Downloads/WORKING/clockify-api-probe-lab/CLAUDE.md` — Safety rules for the probe lab

## Commands run

```sh
# Build and tests
go build ./...
go test ./... -count=1

# Doctor
CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp doctor

# MCP stdio smoke test (initialize + tools/list)
printf '{"jsonrpc":"2.0","id":1,"method":"initialize",...}\n{"jsonrpc":"2.0","id":2,"method":"tools/list",...}\n' \
  | CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp

# MCP task tool calls (list, create, resolve)
printf '...{"method":"tools/call","params":{"name":"clockify_list_tasks",...}}...' \
  | CLOCKIFY_API_KEY=<REDACTED> CLOCKIFY_WORKSPACE_ID=<REDACTED> go run ./cmd/clockify-mcp
```

## Live API probes run

| # | Probe | Endpoint | Result |
|---|-------|----------|--------|
| 1 | Create project | `POST .../workspaces/{ws}/projects` | 201 OK |
| 2 | Create task | `POST .../projects/{pid}/tasks` | 201 OK, task created with ACTIVE status |
| 3 | List tasks | `GET .../projects/{pid}/tasks` | 200 OK, returned task list |
| 4 | Get task by ID | `GET .../projects/{pid}/tasks/{tid}` | 200 OK, returned single task |
| 5 | Update task | `PUT .../projects/{pid}/tasks/{tid}` | 200 OK, name/status/billable updated |
| 6 | Verify update | `GET .../projects/{pid}/tasks/{tid}` | 200 OK, updated fields persisted |
| 7 | Delete task | `DELETE .../projects/{pid}/tasks/{tid}` | 200 OK, task soft-deleted |
| 8 | Verify delete | `GET .../projects/{pid}/tasks/{tid}` | 400, "Task doesn't belong to Project" |
| 9 | Empty name | `POST .../tasks` with `name:""` | 400, "Task name has to be between 1 and 1000 characters" |
| 10 | Invalid workspace | `POST .../tasks` with bad workspace ID | 500, "Internal server error" |
| 11 | Non-existent task | `GET .../tasks/NONEXISTENT123` | 400, "Task doesn't belong to Project" |
| 12 | Create with estimate | `POST .../tasks` with `estimate:"PT2H30M"` | 201 OK, estimate persisted |
| 13 | Create with status | `POST .../tasks` with `status:"ACTIVE"` | 201 OK |
| 14 | Name filter | `GET .../tasks?name=qa-agent-14-page` | 200 OK, filtered correctly |
| 15 | Strict name search | `GET .../tasks?name=...&strict-name-search=true` | 200 OK, exact match |
| 16 | Pagination | `GET .../tasks?page=1&page-size=2` | 200 OK, 2 items returned |
| 17 | Sort by name DESC | `GET .../tasks?sort-column=NAME&sort-order=DESCENDING` | 200 OK, sorted correctly |
| 18 | Active-only filter | `GET .../tasks?is-active=true` | 200 OK, only ACTIVE tasks |
| 19 | Create billable:false | `POST .../tasks` with `billable:false` | API returns billable:true (ignores false on create) |
| 20 | Update billable:false | `PUT .../tasks/{tid}` with `billable:false` | 200 OK, billable:false persisted |

### MCP tool probes

| Tool | Outcome |
|------|---------|
| `clockify_list_tasks` | Works: resolves project by name, paginates, returns typed Task array |
| `clockify_create_task` | Works: resolves project, supports dry_run, creates task |
| `clockify_create_task` (dry_run) | Works: previews payload without mutating |
| `clockify_resolve_name` (entity_type=task) | Works: resolves task name to ID within project scope |

## Findings

### F1 (P2): `billable: false` dropped from MCP output — FIXED

**File:** `internal/clockify/models.go:81`

The `Task.Billable` field used `json:"billable,omitempty"`. Go's `encoding/json` treats `false` as the zero value for `bool`, so `omitempty` strips it. When the API returns `billable: false` (e.g., after a PUT update), the MCP server would drop the field entirely, making it appear as if billable is unknown/unset.

This also affects `Project.Billable` (line 40) and `TimeEntry.Billable` (line 104), which are outside the task-crud-safety scope but worth noting for follow-up.

### F2 (P2): Missing Tier 2 task tools — no get/update/delete

The MCP server has no `clockify_get_task`, `clockify_update_task`, or `clockify_delete_task` tools. These are standard CRUD operations and the live API supports all three. The Tier 1 surface only covers list and create. A client wanting to rename, change status, or delete a task must fall back to raw API calls.

### F3 (P3): `clockify_create_task` missing `estimate` and `status` fields

**File:** `internal/tools/tasks.go:70-73`

The CreateTask handler only builds `{"name": name}` plus optional `billable`. The API accepts `estimate` (ISO-8601 duration string) and `status` (ACTIVE/DONE). These fields are in the API docs (TASKDOC.md) and the live API accepts them, but the MCP tool drops them silently.

### F4 (P3): `clockify_list_tasks` missing query filters

**File:** `internal/tools/tasks.go:31-34`

The ListTasks handler only passes `page` and `page-size` query params. The API supports `name`, `strict-name-search`, `is-active`, `sort-column` (ID/NAME), and `sort-order` (ASCENDING/DESCENDING). These are useful for clients filtering task lists.

## Fixes made

1. **`internal/clockify/models.go:81`** — Removed `omitempty` from `Task.Billable`'s JSON tag. Before: `json:"billable,omitempty"`. After: `json:"billable"`. Verified that `billable: false` now appears in MCP responses.

## Reproduction steps for each issue

### F1 (`billable: false` dropped)
1. Create a task: `POST /workspaces/{ws}/projects/{pid}/tasks` with `{"name":"test"}`
2. Update it to non-billable: `PUT .../tasks/{tid}` with `{"name":"test","billable":false}`
3. Call `clockify_list_tasks` via MCP — before fix: `billable` field absent; after fix: `"billable": false`

### F2 (missing get/update/delete)
1. Call `tools/list` via MCP — no `clockify_get_task`, `clockify_update_task`, or `clockify_delete_task` present

### F3 (missing estimate/status)
1. Call `clockify_create_task` with `{"project":"...", "name":"test", "estimate":"PT1H", "status":"DONE"}` — `estimate` and `status` silently ignored

### F4 (missing filters)
1. Call `clockify_list_tasks` with `{"project":"...", "name":"search-term"}` — `name` filter is silently ignored

## Cleanup performed

- All 7 test tasks deleted (<REDACTED_ID>, <REDACTED_ID>, <REDACTED_ID>, <REDACTED_ID>, <REDACTED_ID>, <REDACTED_ID>, <REDACTED_ID>)
- Test project "qa-agent-14-task-test-project" archived then deleted (<REDACTED_ID>)

## Leftover test resources

None.

## Severity

| Severity | Count | Items |
|----------|-------|-------|
| P0 | 0 | — |
| P1 | 0 | — |
| P2 | 2 | F1 (billable:false dropped — FIXED), F2 (missing get/update/delete tools) |
| P3 | 2 | F3 (missing estimate/status in create), F4 (missing list filters) |

## Files changed

- `internal/clockify/models.go` — Removed `omitempty` from `Task.Billable` JSON tag

## Suggested next action

1. **Apply F1 fix to Project and TimeEntry structs** — same `billable,omitempty` issue at lines 40 and 104 of `models.go`
2. **Add Tier 2 task tools** — `clockify_get_task`, `clockify_update_task`, `clockify_delete_task` as a `tasks` Tier 2 group (or promote update/delete to Tier 1 if task management is core enough)
3. **Add `estimate` and `status` to `clockify_create_task`** — update `tasks.go` payload builder, registry schema, and tests
4. **Add query filters to `clockify_list_tasks`** — pass through `name`, `is-active`, `sort-column`, `sort-order`, `strict-name-search` from args

## False positives / uncertainty

- The `TestJWKSCache_KidMissRateLimited` test in `internal/authn` failed during the full test run. This is a pre-existing flaky test (rate-limit timing), unrelated to task CRUD safety.
- The API returns `billable: true` on POST create regardless of the input value. This is upstream Clockify behavior, not an MCP bug. The `billable` flag is only honored on PUT (update).

## Final recommendation

The Tier 1 task tools (`clockify_list_tasks`, `clockify_create_task`) work correctly and have good safety properties (dry-run, project name resolution, pagination). The `billable:false` data-loss bug is fixed. The main concern is the missing get/update/delete tools (F2), which means the MCP server only covers half of the task CRUD lifecycle. For a server billed as a Clockify API wrapper, this is a meaningful gap.
