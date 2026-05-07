# Production Tool Scope and Policy Defaults

This document defines the scope of tools supported for production deployment and recommends policy configurations for various trust environments.

## Tool Categorization

Tools are categorized based on their impact on data and their required privilege level.

### 1. Agent-Safe Tools (Safe for general use)
*   **Time Tracking:** `clockify_start_timer`, `clockify_stop_timer`, `clockify_log_time`, `clockify_timer_status`, `clockify_timesheet_review`, `clockify_timesheet_fill_gap`.
*   **Contextual Information:** `clockify_whoami`, `clockify_policy_info`, `clockify_list_tools`, `clockify_activate_group`, `clockify_activate_tool`, `clockify_deactivate_group`, `clockify_search_tools`, `clockify_get_workspace`.
*   **Reporting (Read-Only):** `clockify_summary_report`, `clockify_detailed_report`, `clockify_weekly_summary`, `clockify_quick_report`, `clockify_timesheet_review`.
*   **Discovery:** `clockify_list_projects`, `clockify_list_tasks`, `clockify_list_clients`.

### 2. Admin-Only / Sensitive Tools
*   **Management:** `clockify_create_project`, `clockify_create_client`, and Tier 2 project-admin tools such as `clockify_update_project_estimate`, `clockify_archive_projects`, and `clockify_set_project_memberships`.
*   **Financials (Tier 2):** `clockify_create_invoice`, `clockify_delete_invoice`.
*   **User / Group Admin (Tier 2):** `clockify_add_user_to_group`, `clockify_remove_user_from_group`, `clockify_update_user_role`, `clockify_deactivate_user`, plus group-admin tools such as `clockify_create_user_group_admin` and `clockify_delete_user_group_admin`.

### 3. Unsupported / High-Risk (Blocked in Production)
*   **Destructive Operations:** Large-scale deletions are generally discouraged for LLM agents.
*   **Bulk Operations:** Any tool that modifies more than 10 records at once should be carefully audited or disabled.

### Runtime metadata

The categorisation above is reflected at runtime on every
`mcp.ToolDescriptor`:

*   `RiskClass` (bitmask) — `Read | Write | Billing | Admin |
    PermissionChange | ExternalSideEffect | Destructive`. Defaults
    are derived from the existing read-only / destructive boolean
    hints; the `internal/tools/risk_overrides.go` registry layers
    finer distinctions on top (billing for invoice tools, admin +
    permission_change for `clockify_update_user_role`, external
    side effect for `clockify_test_webhook` and outbound invoice
    sends).
*   `AuditKeys []string` — argument keys whose values are recorded
    in audit events alongside the implicit `*_id` capture, so a
    permission-change record carries the new role and a billing
    record carries the quantity / unit_price / status that defines
    the action.
*   `annotations.riskClass` — the client-visible `tools/list` form
    of the same taxonomy, emitted as lower-case strings (`read`,
    `write`, `billing`, `admin`, `permission_change`,
    `external_side_effect`, `destructive`) so agents can plan around
    billing/admin/external-side-effect risk before making a tool call.
*   `annotations.dryRun` — whether the tool schema accepts
    `dry_run:true`, also rendered as the `Dry-run` column in
    `docs/tool-catalog.md`.

These fields are matrix-tested
(`internal/tools/risk_class_test.go`) so a new tool added without
a class falls the build, and the audit recorder consumes
`AuditKeys` end-to-end
(`internal/mcp/server_helpers_test.go`,
`internal/mcp/audit_test.go`).

## Recommended Production Policies

The choice between modes depends on whether the agent should be
able to reshape the workspace (create projects/clients/tags/tasks)
in addition to logging time:

| Feature | `read_only` | `time_tracking_safe` (default for hosted AI) | `safe_core` | `standard` |
|---------|:-----------:|:-------------------------------------------:|:-----------:|:----------:|
| Read access | Full | Full | Full | Full |
| Time-entry mutations (own user) | ❌ | ✅ | ✅ | ✅ |
| Timer start/stop | ❌ | ✅ | ✅ | ✅ |
| Project / client / tag / task creation | ❌ | ❌ | ✅ | ✅ |
| Delete access (any kind) | ❌ | ❌ | ❌ | ✅ |
| Tier 2 tools (invoices, admin, …) | ❌ | ❌ | ❌ | On-demand |
| Recommended for | Read-only dashboards, dev clusters | **Untrusted AI agents** | Trusted shared-service agents | Local development, power users |

`full` is intentionally omitted from recommended production defaults:
it preloads the full tool surface, including Tier 2 tools, and is for
explicit admin automation only.

`time_tracking_safe` is the recommended default for any deployment
that exposes the MCP surface to an LLM agent the operator cannot
fully audit. It is the strictest mode that still lets the agent
do its time-tracking job — workspace structure (projects /
clients / tags) stays under human control.

`safe_core` is appropriate when the agent needs to register new
projects or clients to log time against (e.g. a sales-ops bot
ingesting CRM accounts). It still blocks all delete operations
and Tier 2 admin surface.

For a single owner using one API key against one pinned workspace,
start with `read_only` for dashboard-only checks,
`time_tracking_safe` for AI-facing time-tracking workflows, and
`safe_core` only when the local agent is trusted to create project
structure. Avoid `standard` and `full` for exploratory testing on a
real workspace with valuable data.

## Policy Enforcement

Set the policy using the `CLOCKIFY_POLICY` environment variable.
Permitted values: `read_only`, `time_tracking_safe`, `safe_core`,
`standard`, `full`.

```env
# Untrusted agent in a hosted deployment — strongest sensible default.
CLOCKIFY_POLICY=time_tracking_safe

# Or, when the agent needs to register projects/clients to log against:
CLOCKIFY_POLICY=safe_core
```

To further restrict tools, use `CLOCKIFY_BOOTSTRAP_MODE=minimal`, discover tools with `clockify_list_tools`, and activate only the required groups at runtime using `clockify_activate_group` or `clockify_activate_tool`.
