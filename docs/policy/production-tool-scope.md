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

## Recommended Production Policy: `safe_core`

For most production deployments, the `safe_core` policy is the recommended default.

| Feature | `safe_core` (Recommended) | `standard` (Default) |
|---------|-------------------------|---------------------|
| Read access | Full | Full |
| Write access | Allowlist (Time Tracking) | Full |
| Delete access | Blocked | Full |
| Tier 2 tools | Disabled | On-demand |
| Best for | Shared services, LLM agents | Local development, power users |

## Policy Enforcement

Set the policy using the `CLOCKIFY_POLICY` environment variable.

```env
CLOCKIFY_POLICY=safe_core
```

To further restrict tools, use `CLOCKIFY_BOOTSTRAP_MODE=minimal` and only activate the required tools at runtime using `clockify_search_tools`.
