# Tool Annotations

Every tool in the Clockify MCP server carries MCP annotations that describe its behavior. Clients can use these annotations to make informed decisions about tool usage.

## Annotation Fields

| Field | Type | Description |
|-------|------|-------------|
| `readOnlyHint` | boolean | `true` if the tool only reads data, `false` if it writes |
| `destructiveHint` | boolean | `true` if the tool can delete data or cause irreversible changes |
| `idempotentHint` | boolean | `true` if calling multiple times has the same effect |

## Tier 1 Tools (33)

### Context & Discovery

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_whoami` | ✅ | ❌ | ✅ |
| `clockify_policy_info` | ✅ | ❌ | ✅ |
| `clockify_search_tools` | ✅ | ❌ | ✅ |
| `clockify_resolve_debug` | ✅ | ❌ | ✅ |

### Workspaces

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_list_workspaces` | ✅ | ❌ | ✅ |
| `clockify_get_workspace` | ✅ | ❌ | ✅ |

### Users

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_current_user` | ✅ | ❌ | ✅ |
| `clockify_list_users` | ✅ | ❌ | ✅ |

### Timer

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_timer_status` | ✅ | ❌ | ✅ |
| `clockify_start_timer` | ❌ | ❌ | ❌ |
| `clockify_stop_timer` | ❌ | ❌ | ✅ |

### Entries

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_list_entries` | ✅ | ❌ | ✅ |
| `clockify_get_entry` | ✅ | ❌ | ✅ |
| `clockify_today_entries` | ✅ | ❌ | ✅ |
| `clockify_add_entry` | ❌ | ❌ | ❌ |
| `clockify_update_entry` | ❌ | ❌ | ✅ |
| `clockify_delete_entry` | ❌ | ✅ | ✅ |

### Projects

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_list_projects` | ✅ | ❌ | ✅ |
| `clockify_get_project` | ✅ | ❌ | ✅ |
| `clockify_create_project` | ❌ | ❌ | ❌ |

### Clients

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_list_clients` | ✅ | ❌ | ✅ |
| `clockify_create_client` | ❌ | ❌ | ❌ |

### Tags

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_list_tags` | ✅ | ❌ | ✅ |
| `clockify_create_tag` | ❌ | ❌ | ❌ |

### Tasks

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_list_tasks` | ✅ | ❌ | ✅ |
| `clockify_create_task` | ❌ | ❌ | ❌ |

### Reports

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_summary_report` | ✅ | ❌ | ✅ |
| `clockify_detailed_report` | ✅ | ❌ | ✅ |
| `clockify_weekly_summary` | ✅ | ❌ | ✅ |
| `clockify_quick_report` | ✅ | ❌ | ✅ |

### Workflows

| Tool | readOnly | destructive | idempotent |
|------|----------|-------------|------------|
| `clockify_log_time` | ❌ | ❌ | ❌ |
| `clockify_switch_project` | ❌ | ❌ | ❌ |
| `clockify_find_and_update_entry` | ❌ | ❌ | ❌ |

## Tier 2 Tools (91 across 11 groups)

Tier 2 tools follow the same annotation pattern. Each group is activated on demand via `clockify_search_tools`.

| Group | Tools | Read | Write | Destructive |
|-------|-------|------|-------|-------------|
| invoices | 12 | 4 | 5 | 3 |
| expenses | 10 | 3 | 4 | 3 |
| scheduling | 10 | 4 | 3 | 3 |
| time_off | 12 | 5 | 4 | 3 |
| approvals | 6 | 2 | 4 | 0 |
| shared_reports | 6 | 3 | 2 | 1 |
| user_admin | 8 | 3 | 4 | 1 |
| webhooks | 7 | 2 | 2 | 3 |
| custom_fields | 6 | 2 | 2 | 2 |
| groups_holidays | 8 | 3 | 3 | 2 |
| project_admin | 6 | 2 | 3 | 1 |

## How Annotations Drive Behavior

1. **Policy enforcement**: `readOnlyHint` determines if a tool is allowed in `read_only` mode
2. **Dry-run routing**: `destructiveHint` determines if `dry_run: true` is supported
3. **tools/list filtering**: Both hints filter what appears in `tools/list` responses
4. **Error response format**: Tool errors return as `result.isError: true` (MCP spec), not JSON-RPC `error`
5. **Client UI**: Clients can use annotations to show warning indicators or require confirmation

## MCP Protocol Notes

- The server requires an `initialize` handshake before accepting `tools/call` requests (error code `-32002`)
- Tool execution errors use `isError: true` in the result object per the MCP spec
- Protocol-level errors (invalid JSON, unknown method, uninitialized) use standard JSON-RPC `error`
- Each tool call is logged with a monotonic request ID for correlation
