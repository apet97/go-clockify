# Production Tool Scope and Policy Defaults

This document defines the scope of tools supported for production deployment and recommends policy configurations for various trust environments.

## Tool Categorization

Tools are categorized based on their impact on data and their required privilege level.

### 1. Agent-Safe Tools (Safe for general use)
*   **Time Tracking:** `clockify_start_timer`, `clockify_stop_timer`, `clockify_log_time`, `clockify_timer_status`.
*   **Contextual Information:** `clockify_whoami`, `clockify_policy_info`, `clockify_search_tools`, `clockify_get_workspace`.
*   **Reporting (Read-Only):** `clockify_summary_report`, `clockify_detailed_report`, `clockify_weekly_summary`, `clockify_quick_report`.
*   **Discovery:** `clockify_list_projects`, `clockify_list_tasks`, `clockify_list_clients`.

### 2. Admin-Only / Sensitive Tools
*   **Management:** `clockify_create_project`, `clockify_create_client`, and Tier 2 project-admin tools such as `clockify_update_project_estimate`, `clockify_archive_projects`, and `clockify_set_project_memberships`.
*   **Financials (Tier 2):** `clockify_create_invoice`, `clockify_delete_invoice`.
*   **User / Group Admin (Tier 2):** `clockify_add_user_to_group`, `clockify_remove_user_from_group`, `clockify_update_user_role`, `clockify_deactivate_user`, plus group-admin tools such as `clockify_create_user_group_admin` and `clockify_delete_user_group_admin`.

### 3. Unsupported / High-Risk (Blocked in Production)
*   **Destructive Operations:** Large-scale deletions are generally discouraged for LLM agents.
*   **Bulk Operations:** Any tool that modifies more than 10 records at once should be carefully audited or disabled.

## Recommended Production Policies

The choice between modes depends on whether the agent should be
able to reshape the workspace (create projects/clients/tags/tasks)
in addition to logging time:

| Feature | `read_only` | `time_tracking_safe` | `safe_core` (Default for hosted) | `standard` |
|---------|:-----------:|:--------------------:|:--------------------------------:|:----------:|
| Read access | Full | Full | Full | Full |
| Time-entry mutations (own user) | ❌ | ✅ | ✅ | ✅ |
| Timer start/stop | ❌ | ✅ | ✅ | ✅ |
| Project / client / tag / task creation | ❌ | ❌ | ✅ | ✅ |
| Delete access (any kind) | ❌ | ❌ | ❌ | ✅ |
| Tier 2 tools (invoices, admin, …) | ❌ | ❌ | ❌ | On-demand |
| Recommended for | Read-only dashboards, dev clusters | **Untrusted AI agents** | Trusted shared-service agents | Local development, power users |

`time_tracking_safe` is the recommended default for any deployment
that exposes the MCP surface to an LLM agent the operator cannot
fully audit. It is the strictest mode that still lets the agent
do its time-tracking job — workspace structure (projects /
clients / tags) stays under human control.

`safe_core` is appropriate when the agent needs to register new
projects or clients to log time against (e.g. a sales-ops bot
ingesting CRM accounts). It still blocks all delete operations
and Tier 2 admin surface.

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

To further restrict tools, use `CLOCKIFY_BOOTSTRAP_MODE=minimal` and only activate the required tools at runtime using `clockify_search_tools`.
