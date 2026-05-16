# Clockify MCP

![MCP Protocol](https://img.shields.io/badge/MCP-2025--11--25-blue)

A local, single-user, full-access Clockify MCP server for one trusted user and
one pinned Clockify workspace. It runs over stdio, uses one Clockify API key,
loads every tool at startup, and returns predictable JSON envelopes.

## Product shape

- One local user, one `CLOCKIFY_API_KEY`, one required `CLOCKIFY_WORKSPACE_ID`.
- Stdio transport only; full access from startup.
- Workflow tools first, domain tools second, raw API fallback last.
- Every write returns IDs; recoverable errors include a recovery hint.

## Quick start

```bash
git clone https://github.com/apet97/go-clockify.git
cd go-clockify

export CLOCKIFY_API_KEY="..."
export CLOCKIFY_WORKSPACE_ID="..."

go run ./cmd/clockify-mcp doctor       # validate configuration
go run ./cmd/clockify-mcp              # run the stdio MCP server
```

`doctor --live` also proves the key, workspace, owner role, and feature plan
against Clockify. For a compiled install, `go build -o ./bin/clockify-mcp
./cmd/clockify-mcp` and point the MCP client at the binary. MCP clients launch
the binary as a stdio subprocess with the environment above.

### Optional environment

| Variable | Default |
| --- | --- |
| `CLOCKIFY_BASE_URL` | `https://api.clockify.me/api/v1` |
| `CLOCKIFY_TIMEZONE` | system local |
| `CLOCKIFY_TOOLSET` | `all` |
| `CLOCKIFY_TOOL_TIMEOUT` | `45s` |
| `CLOCKIFY_ENABLE_RAW_WRITES` | `false` |
| `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` | (none) |
| `CLOCKIFY_CIRCUIT_BREAKER` | `enabled` |
| `MCP_LOG_LEVEL` | `info` |

`go run ./cmd/clockify-mcp doctor` prints every resolved setting.

## Compatibility

| Capability | Support |
| --- | --- |
| MCP Protocol | `2025-11-25` |
| Transport | stdio |
| Clockify scope | one pinned workspace |

## Tools

`CLOCKIFY_TOOLSET=all` (the default) preserves the 156-tool startup registry.
Narrower surfaces — `core`, `business`, `admin` — filter that registry without
changing the product model. `docs/tool-catalog.md` is the authoritative,
generated tool list and order.

Start with `clockify_status`, then use IDs returned by earlier calls. Workflow
tools come first in `tools/list` and carry `category`, `priority`, `bestFor`,
and related annotations so an agent picks the high-level path before CRUD.

Workflow tools: `clockify_status`, `clockify_tools_guide`,
`clockify_create_work_package`, `clockify_log_work`, `clockify_start_work`,
`clockify_stop_work`, `clockify_switch_work`, `clockify_review_day`,
`clockify_review_week`, `clockify_fix_entry`, `clockify_invoice_client_work`,
`clockify_record_expense`, `clockify_request_time_off`, `clockify_schedule_work`,
`clockify_setup_webhook`, `clockify_demo_seed`, `clockify_demo_cleanup`.

Domain tools cover clients, projects, tasks, tags, time entries, reports,
invoices, expenses, custom fields, time off, scheduling, approvals, webhooks,
groups, holidays, and users/workspace. Some expose a deliberately simplified
owner-friendly body; per-tool omissions are recorded in
`docs/api-parity-matrix.md`.

Raw API fallback — `clockify_api_get` and `clockify_api_request` — is
pinned-workspace scoped. Raw `GET` is always available; raw `POST`, `PUT`,
`PATCH`, and `DELETE` require `CLOCKIFY_ENABLE_RAW_WRITES=true`. Prefer domain
tools; raw writes are an explicit, high-risk escape hatch.

Webhook tools require HTTPS, reject embedded credentials, and validate DNS so
hostnames resolving to localhost, private, reserved, or link-local IPs are
blocked. `CLOCKIFY_WEBHOOK_ALLOWED_DOMAINS` is the escape hatch for trusted
test domains.

## Result envelope

Success:

```json
{ "ok": true, "action": "clockify_projects_create", "entity": "project",
  "ids": {}, "data": {}, "meta": {},
  "changed": { "created": [], "updated": [], "deleted": [], "reused": [] },
  "warnings": [], "next": [] }
```

Recoverable failure:

```json
{ "ok": false, "action": "clockify_projects_create",
  "error": { "code": "invalid_request", "message": "name is required" },
  "recovery": { "hint": "List projects, then retry with returned IDs.",
                "tool": "clockify_projects_list" } }
```

## Prompts and resources

`prompts/list` exposes action-oriented prompts (demo, setup, log-week,
invoicing, time off, scheduling, webhooks). `resources/list` exposes one-user
workspace context under `clockify://` URIs (status, workspace, user, features,
tools, workflows, recent data).

## Tests

```bash
go test ./...        # full suite against a stateful fake Clockify server
make check           # race-enabled gate plus repo hygiene
```

Live tests are opt-in and must point at a sacrificial Clockify workspace:

```bash
export CLOCKIFY_API_KEY="..." CLOCKIFY_WORKSPACE_ID="..." CLOCKIFY_RUN_LIVE_E2E=1
go test -tags=livee2e -count=1 -timeout 5m ./tests/...
```

## Implementation goal

The spec lives at
[`docs/goals/perfect-one-user-full-mcp.md`](docs/goals/perfect-one-user-full-mcp.md):
keep the product local, single-user, full-access, and stdio-only.
