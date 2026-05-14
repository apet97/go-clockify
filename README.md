# Clockify MCP

![MCP Protocol](https://img.shields.io/badge/MCP-2025--11--25-blue)

A local, single-user, full-access Clockify MCP for one trusted user and one
pinned Clockify workspace.

The server runs over stdio, uses one Clockify API key, loads its tools at
startup, and returns predictable JSON envelopes from every tool call. All tools
are available immediately.

## Product Shape

- One local user.
- One `CLOCKIFY_API_KEY`.
- One required `CLOCKIFY_WORKSPACE_ID`.
- Stdio transport only.
- Full access from startup.
- Workflow tools first.
- Domain tools second.
- Raw API fallback tools last.
- Every write returns IDs.
- Recoverable errors include a recovery hint.

## How to start

0. Install Go and clone the repository:

```bash
git clone https://github.com/apet97/go-clockify.git
cd go-clockify
go version
```

Use the Go version from `go.mod` or newer.

1. Set the required Clockify environment:

```bash
export CLOCKIFY_API_KEY="..."
export CLOCKIFY_WORKSPACE_ID="..."
```

2. Optionally set local defaults:

```bash
export CLOCKIFY_TIMEZONE="Europe/Belgrade"
export CLOCKIFY_BASE_URL="https://api.clockify.me/api/v1"
export MCP_LOG_LEVEL="info"
```

`CLOCKIFY_BASE_URL` defaults to `https://api.clockify.me/api/v1`.

3. Validate configuration before starting the MCP loop:

```bash
go run ./cmd/clockify-mcp doctor
```

4. Run the stdio MCP server:

```bash
go run ./cmd/clockify-mcp
```

MCP clients should launch the binary as a stdio subprocess and pass the
environment variables above. For compiled installs, build once and point the
client at the binary:

```bash
go build -o ./bin/clockify-mcp ./cmd/clockify-mcp
./bin/clockify-mcp
```

5. Smoke-test the repo locally before changing it:

```bash
go test -count=1 ./...
git diff --check
```

Use `make check` before PRs; it runs the stricter race-enabled test target
plus repo hygiene checks.

## Compatibility

| Capability | Support |
| --- | --- |
| MCP Protocol | `2025-11-25` |
| Transport | stdio |
| Clockify scope | one pinned workspace |

## Tool Coverage

The runtime exposes workflow, domain, and raw API tools at startup. Start with
`clockify_status`. Use IDs returned by earlier calls in later calls.

Workflow tools appear first in `tools/list` and carry `category`, `priority`,
`bestFor`, `preferOver`, `domain`, and `entity` annotations so agents can pick
the high-level path before using CRUD tools.

Workflow tools:

- `clockify_status`
- `clockify_tools_guide`
- `clockify_create_work_package`
- `clockify_log_work`
- `clockify_start_work`
- `clockify_stop_work`
- `clockify_switch_work`
- `clockify_review_day`
- `clockify_review_week`
- `clockify_fix_entry`
- `clockify_invoice_client_work`
- `clockify_record_expense`
- `clockify_request_time_off`
- `clockify_schedule_work`
- `clockify_setup_webhook`
- `clockify_demo_seed`
- `clockify_demo_cleanup`

Use `clockify_tools_guide` when the agent needs to choose between workflows,
domain tools, and raw fallback. It is read-only and returns grouped guidance for
common tasks.

Core domain tools:

- clients: list, get, create, update, delete
- projects: list, get, create, update, delete, archive, templates, estimates,
  memberships, rates
- tasks: list, get, create, update, delete, rates
- tags: list, get, create, update, delete
- entries: list, get, create, update, delete, mark invoiced, running timer,
  timer start, timer stop, timer status, timer switch

Migrated Clockify domains:

- reports: detailed, summary, weekly, attendance, money, expense, export
- invoices: list, get, create, update, delete, items, import time, import
  expenses, send, mark paid, export, payments
- expenses: list, get, create, update, delete, categories
- custom fields: list, get, create, update, delete, set value
- time off: policies, requests, balances, approve, deny, archive
- scheduling: assignments, project totals, user totals, capacity
- approvals: list, get, submit, resubmit, approve, reject, withdraw
- webhooks: list, get, create, update, delete, test, events
- groups: list, get, create, update, delete, add user, remove user
- holidays: list, get, create, update, delete, list for user period
- users and workspace: list, profile, invite, deactivate, role, settings

Raw API fallback:

- `clockify_api_get`
- `clockify_api_request`

The deterministic demo helpers create or reuse a prefixed client, project,
task, tag, and time entry, then clean them up by prefix. Cleanup is repeatable,
so running it after the objects are gone is a harmless no-op with a normal
envelope.

## Example Flow

1. Call `clockify_status` to confirm the pinned workspace, user, timezone, and
   current timer.
2. Call `clockify_demo_seed` with a stable `run_id` to create or reuse a
   deterministic client, project, task, tag, and time entry.
3. Use returned IDs with workflow tools such as `clockify_log_work`,
   `clockify_review_day`, and `clockify_review_week`.
4. Call `clockify_demo_cleanup` with the same `run_id`. It is repeatable and
   safe to run again after the demo objects are gone.

## Prompts And Resources

`prompts/list` exposes action-oriented prompts for common work:

- `demo-full-workspace-story`
- `setup-client-project-task`
- `log-week`
- `invoice-client`
- `cleanup-demo`
- `review-week`
- `create-expense`
- `request-time-off`
- `schedule-week`
- `setup-webhook`

Each prompt names the first workflow tool to call, tells the model to use
returned IDs, asks it to continue when an optional paid Clockify feature is not
available, and ends by summarizing what changed.

`resources/list` exposes one-user workspace context:

- `clockify://status`
- `clockify://workspace`
- `clockify://user`
- `clockify://features`
- `clockify://tools`
- `clockify://workflows`
- `clockify://demo/phase1`
- `clockify://recent/entries`
- `clockify://recent/projects`
- `clockify://workspace/{workspace_id}`
- `clockify://workspace/{workspace_id}/user/current`

`clockify_demo_seed` and `clockify_demo_cleanup` update
`clockify://demo/{run_id}` for the run id passed to the tool. Non-default demo
run IDs are exposed through the `clockify://demo/{run_id}` resource template.

## Result Envelope

Successful tools return:

```json
{
  "ok": true,
  "action": "clockify_projects_create",
  "entity": "project",
  "ids": {
    "workspaceId": "workspace_id",
    "projectId": "project_id"
  },
  "data": {},
  "changed": {
    "created": [],
    "updated": [],
    "deleted": [],
    "reused": []
  },
  "warnings": [],
  "next": []
}
```

Recoverable failures return:

```json
{
  "ok": false,
  "action": "clockify_projects_create",
  "error": {
    "code": "invalid_request",
    "message": "name is required"
  },
  "recovery": {
    "hint": "List projects or clients, then retry with returned IDs.",
    "tool": "clockify_projects_list"
  }
}
```

## Tests

```bash
go test ./...
```

The test suite includes MCP startup contracts, full-access tool-list coverage,
workflow-first ordering and annotations, result envelope behavior,
deterministic demo seed, repeatable cleanup behavior, and review workflow
coverage against a stateful fake Clockify server.

### Live tests

Live tests are opt-in and must point at a sacrificial Clockify workspace:

```bash
export CLOCKIFY_API_KEY="..."
export CLOCKIFY_WORKSPACE_ID="..."
export CLOCKIFY_RUN_LIVE_E2E=1
go test -tags=livee2e -count=1 -timeout 5m \
  -run '^(TestLiveOneUserWorkflowMCP|TestLiveRawClockifyReadSideSchemaDiff)$' \
  ./tests/...
```

The workflow live test uses real API calls and does not pass `dry_run`.

## Implementation Goal

The implementation goal lives at
[`docs/goals/perfect-one-user-full-mcp.md`](docs/goals/perfect-one-user-full-mcp.md).

The implementation follows that spec while keeping the runnable product local,
single-user, full-access, and stdio-only.
