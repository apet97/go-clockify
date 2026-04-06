# Tool Catalog

Complete catalog of all 124 tools in the Clockify MCP server.

## Tier 1 Tools (33 — always available)

### Context & Discovery (4)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_whoami` | Get current user and resolved workspace | none |
| `clockify_policy_info` | Display effective policy configuration | none |
| `clockify_search_tools` | Search and discover available tools by keyword | `query?`, `activate_group?`, `activate_tool?` |
| `clockify_resolve_debug` | Debug name-to-ID resolution | `entity_type`, `name_or_id` |

### Workspaces (2)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_list_workspaces` | List available Clockify workspaces | none |
| `clockify_get_workspace` | Get the resolved workspace | none |

### Users (2)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_current_user` | Get the current Clockify user | none |
| `clockify_list_users` | List users in the resolved workspace | none |

### Timer (3)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_timer_status` | Check if a timer is currently running and show elapsed time | none |
| `clockify_start_timer` | Start a new timer | `project?`, `project_id?`, `description?` |
| `clockify_stop_timer` | Stop the current running timer | `dry_run?` |

### Entries (6)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_list_entries` | List recent time entries with optional filtering | `page?`, `page_size?`, `start?`, `end?`, `project?` |
| `clockify_get_entry` | Get a single time entry by ID | `entry_id` |
| `clockify_today_entries` | List time entries for the current day | `page?`, `page_size?` |
| `clockify_add_entry` | Create a new time entry with flexible time parsing | `start`, `end?`, `description?`, `project?`, `project_id?`, `task_id?`, `billable?`, `dry_run?` |
| `clockify_update_entry` | Update an existing time entry (fetch-then-merge) | `entry_id`, `description?`, `project?`, `project_id?`, `start?`, `end?`, `billable?`, `dry_run?` |
| `clockify_delete_entry` | Delete a time entry by ID | `entry_id`, `dry_run?` |

### Projects (3)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_list_projects` | List projects in the resolved workspace | none |
| `clockify_get_project` | Get a project by ID or exact name | `project` |
| `clockify_create_project` | Create a new project | `name`, `client?`, `color?`, `billable?`, `is_public?` |

### Clients (2)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_list_clients` | List clients in the resolved workspace | none |
| `clockify_create_client` | Create a new client | `name` |

### Tags (2)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_list_tags` | List tags in the resolved workspace | none |
| `clockify_create_tag` | Create a new tag | `name` |

### Tasks (2)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_list_tasks` | List tasks for a project | `project` |
| `clockify_create_task` | Create a new task in a project | `project`, `name`, `billable?` |

### Reports (4)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_summary_report` | Summarize entries for a date range by project | `start?`, `end?`, `include_entries?` |
| `clockify_detailed_report` | Detailed time entry report with project filtering | `start`, `end`, `project?`, `include_entries?` |
| `clockify_weekly_summary` | Weekly summary grouped by day and project | `week_start?`, `timezone?`, `include_entries?` |
| `clockify_quick_report` | Quick high-signal summary for a recent period | `days?`, `include_entries?` |

### Workflows (3)

| Tool | Description | Params |
|------|-------------|--------|
| `clockify_log_time` | Create a finished time entry (no live timer) | `start`, `end`, `project?`, `project_id?`, `description?`, `billable?`, `dry_run?` |
| `clockify_switch_project` | Stop timer and start on a different project | `project`, `description?`, `task_id?`, `billable?` |
| `clockify_find_and_update_entry` | Find and update an entry by filters | `entry_id?`, `description_contains?`, `exact_description?`, `start_after?`, `start_before?`, `new_description?`, `project?`, `project_id?`, `start?`, `end?`, `billable?`, `dry_run?` |

## Tier 2 Tools (91 — on demand)

Activate via `clockify_search_tools { "activate_group": "group_name" }`.

### Invoices (12)

`clockify_list_invoices`, `clockify_get_invoice`, `clockify_create_invoice`, `clockify_update_invoice`, `clockify_delete_invoice`, `clockify_send_invoice`, `clockify_list_invoice_items`, `clockify_add_invoice_item`, `clockify_update_invoice_item`, `clockify_delete_invoice_item`, `clockify_mark_invoice_sent`, `clockify_mark_invoice_paid`

### Expenses (10)

`clockify_list_expenses`, `clockify_get_expense`, `clockify_create_expense`, `clockify_update_expense`, `clockify_delete_expense`, `clockify_list_expense_categories`, `clockify_create_expense_category`, `clockify_update_expense_category`, `clockify_delete_expense_category`, `clockify_get_expense_summary`

### Scheduling (10)

`clockify_list_assignments`, `clockify_get_assignment`, `clockify_create_assignment`, `clockify_update_assignment`, `clockify_delete_assignment`, `clockify_list_schedules`, `clockify_get_schedule`, `clockify_create_schedule`, `clockify_update_schedule`, `clockify_delete_schedule`

### Time Off (12)

`clockify_list_time_off_policies`, `clockify_get_time_off_policy`, `clockify_list_time_off_requests`, `clockify_get_time_off_request`, `clockify_create_time_off_request`, `clockify_update_time_off_request`, `clockify_delete_time_off_request`, `clockify_approve_time_off_request`, `clockify_reject_time_off_request`, `clockify_list_time_off_balances`, `clockify_get_time_off_balance`, `clockify_update_time_off_balance`

### Approvals (6)

`clockify_list_approval_requests`, `clockify_get_approval_request`, `clockify_submit_for_approval`, `clockify_approve_timesheet`, `clockify_reject_timesheet`, `clockify_withdraw_approval`

### Shared Reports (6)

`clockify_list_shared_reports`, `clockify_get_shared_report`, `clockify_create_shared_report`, `clockify_update_shared_report`, `clockify_delete_shared_report`, `clockify_export_shared_report`

### User Admin (8)

`clockify_list_workspace_users`, `clockify_get_workspace_user`, `clockify_invite_user`, `clockify_update_user_role`, `clockify_deactivate_user`, `clockify_reactivate_user`, `clockify_remove_user`, `clockify_update_user_cost_rate`

### Webhooks (7)

`clockify_list_webhooks`, `clockify_get_webhook`, `clockify_create_webhook`, `clockify_update_webhook`, `clockify_delete_webhook`, `clockify_test_webhook`, `clockify_list_webhook_events`

### Custom Fields (6)

`clockify_list_custom_fields`, `clockify_get_custom_field`, `clockify_create_custom_field`, `clockify_update_custom_field`, `clockify_delete_custom_field`, `clockify_set_entry_custom_field`

### Groups & Holidays (8)

`clockify_list_user_groups`, `clockify_get_user_group`, `clockify_create_user_group`, `clockify_delete_user_group`, `clockify_add_user_to_group`, `clockify_remove_user_from_group`, `clockify_list_holidays`, `clockify_delete_holiday`

### Project Admin (6)

`clockify_update_project`, `clockify_archive_project`, `clockify_delete_project`, `clockify_update_project_membership`, `clockify_update_project_estimate`, `clockify_update_project_template`

## Response Format

The server strictly follows the MCP protocol for tool responses:

### 1. Success

```json
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Result data here..."}],"isError":false}}
```

### 2. Tool-Level Errors (v0.3.0+)

Errors encountered during tool execution (e.g. invalid arguments, resource not found) return a successful JSON-RPC result but include `isError: true` and the error message in the `content` array:

```json
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"project 'Nonexistent' not found"}],"isError":true}}
```

### 3. Protocol-Level Errors

Errors in the JSON-RPC request itself or the server state (e.g. unknown method, uninitialized server) return a standard JSON-RPC `error` object:

```json
{"jsonrpc":"2.0","id":1,"error":{"code":-32002,"message":"server not initialized: send initialize first"}}
```
