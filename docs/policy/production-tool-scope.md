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

At runtime, `standard` and `full` have the same policy allow rules:
any non-denied tool or group can run after the bootstrap/activation
surface exposes it. Treat `full` as an explicit operator/admin label,
not as an additional code-level gate. A separate confirmation-token
gate (per ADR-0018, landed on `adr0018-confirmation-tokens`) sits
alongside the policy mode: every high-risk tool call
(`RiskBilling | RiskAdmin | RiskPermissionChange | RiskExternalSideEffect |
RiskDestructive`) requires a server-minted HMAC token obtained via a
`dry_run:true` preview. The gate runs uniformly across all policy
modes including `standard` and `full`; it is configurable via
`CLOCKIFY_CONFIRMATION_TOKENS=disabled` (break-glass only) and via
`CLOCKIFY_CONFIRMATION_TOKEN_SECRET` for shared-service replicas.

`full` is intentionally omitted from recommended production defaults:
it represents an intentionally unrestricted policy posture and is for
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

## Tenant policy ceiling (multi-tenant hosted deployments)

`CLOCKIFY_POLICY` sets the process posture, but in a multi-tenant
deployment the control plane carries per-tenant overrides
(`TenantRecord.PolicyMode`). Without a guardrail, a corrupted or
overly broad tenant row could silently broaden the live posture
past whatever the operator pinned at the profile level. The
**tenant policy ceiling** closes that gap.

The ceiling is a per-process maximum that tenant records may
narrow but cannot exceed. Three behaviours:

1. **Mode broadening is rejected.** If a tenant record sets
   `PolicyMode` strictly broader than `min(CLOCKIFY_POLICY,
   MCP_TENANT_POLICY_CEILING)`, session creation fails with an
   "exceeds" error. Operators see misconfigured rows at
   session-create time, not at first tool call. The same gate
   fires at config load when `CLOCKIFY_POLICY` itself is broader
   than `MCP_TENANT_POLICY_CEILING` — the binary refuses to start
   rather than running with a process posture that would silently
   bypass the hosted ceiling for every tenant.
2. **Tenant deny lists narrow only.** `tenant.DenyTools` and
   `tenant.DenyGroups` union with the process deny lists. A
   tenant record can add denies; it cannot erase a
   process-level deny.
3. **Tenant allow lists narrow only.** `tenant.AllowGroups` is
   intersected with the process `CLOCKIFY_ALLOW_GROUPS` when
   both are set, or defines the whitelist when the process did
   not set one. Under a group-blocking mode (`read_only`,
   `time_tracking_safe`, `safe_core`) it is silently dropped
   and surfaced via `clockify_policy_info` as
   `tenant_allow_groups_ignored: true`.

`clockify_policy_info` also exposes three ceiling fields for
operator triage:

- `configured_ceiling` — the literal `MCP_TENANT_POLICY_CEILING`
  value (or empty if unset).
- `effective_ceiling` — the value the runtime actually caps
  tenant overrides against. Equals `configured_ceiling` when
  set; falls back to the process mode when no explicit ceiling
  is configured.
- `ceiling_source` — `"explicit"` when an env override or
  profile default supplied the ceiling, `"implicit_process_mode"`
  when the process mode is doing the work.

Ranking (see `policy.Rank` in `internal/policy/policy.go`):

```
read_only < time_tracking_safe < safe_core < standard < full
```

`standard` and `full` are explicitly split despite their current
`IsAllowed` equivalence — see ADR-0021 for rationale.

Profile defaults:

| Profile | Default `MCP_TENANT_POLICY_CEILING` |
|---------|-------------------------------------|
| `shared-service` | `time_tracking_safe` |
| `prod-postgres` | `time_tracking_safe` |
| `single-tenant-http` | *(unset; process mode is the implicit ceiling)* |
| `local-stdio` | *(unset; no tenants)* |
| `private-network-grpc` | *(unset; mTLS callers trusted)* |

Empty / unset means "no explicit ceiling" — the process mode
acts as the implicit ceiling so even a hosted operator who forgets
to set the env var still cannot have tenants broaden past
`CLOCKIFY_POLICY`. Operators who explicitly want to expose more
surface than the hosted default set `MCP_TENANT_POLICY_CEILING`
to `safe_core`, `standard`, or `full`.

See [ADR 0021 — Hosted tenant policy ceiling](../adr/0021-hosted-tenant-policy-ceiling.md)
for the full design rationale.

## Clockify API key role requirements

`CLOCKIFY_POLICY` controls what the MCP layer is willing to expose,
but a tool can still fail upstream if the API key's owning user
lacks the Clockify workspace role the underlying endpoint demands.
Two layers, two gates: the policy gate is enforced by this repo;
the role gate is enforced by Clockify. Both must pass.

The table below documents the **minimum** Clockify workspace role a
key needs for each tool family. "Owner" is strictly broader than
"Workspace Admin" → "Team Manager" → "Project Manager" → "Regular";
a key with a stronger role works wherever a weaker one is listed.

| Tool family | Minimum role | Notes |
|---|---|---|
| Personal time tracking (`clockify_start_timer`, `clockify_stop_timer`, `clockify_log_time`, `clockify_list_entries`, `clockify_update_entry`, `clockify_delete_entry`, `clockify_timesheet_review`) | Regular | Mutations are constrained to the API key owner's own entries. |
| Personal context (`clockify_whoami`, `clockify_get_workspace`, `clockify_list_tools`, `clockify_policy_info`, `clockify_search_tools`) | Regular | No workspace data exposed beyond the caller's membership. |
| Project / client / tag / task **reads** (`clockify_list_projects`, `clockify_list_clients`, `clockify_list_tags`, `clockify_list_tasks`, `clockify_get_*`) | Regular | List/get reads are visible to all members. |
| Project / client / tag / task **writes** (`clockify_create_*`, `clockify_update_*`, `clockify_archive_*`, `clockify_delete_*`) | Workspace Admin | Project Managers may also have write access on projects they manage; the upstream API enforces this on a per-resource basis. |
| Reports (`clockify_summary_report`, `clockify_detailed_report`, `clockify_weekly_summary`, `clockify_quick_report`) | Team Manager | Cross-user roll-ups require Team Manager or higher. A Regular key sees only its own entries even if a report tool is invoked. |
| Approvals (`clockify_list_approval_requests`, `clockify_approve_timesheet`, `clockify_reject_timesheet`, `clockify_submit_for_approval`, `clockify_withdraw_approval`) | Team Manager | Approvers must have the role for the workspace's approval policy; Regulars can submit their own requests only. |
| Webhooks (`clockify_create_webhook`, `clockify_delete_webhook`, `clockify_test_webhook`) | Workspace Admin | Webhook surface is admin-only. |
| Shared reports (`clockify_create_shared_report`, `clockify_list_shared_reports`, `clockify_delete_shared_report`) | Workspace Admin | Share targets bypass per-user permissions, so the upstream API requires admin scope. |
| Scheduling and time-off (`clockify_*_assignment`, `clockify_create_time_off_request`, `clockify_approve_time_off`, `clockify_deny_time_off`) | Workspace Admin | Scheduling and time-off rosters cross multiple users. |
| Custom fields (`clockify_list_custom_fields`, `clockify_set_custom_field_value`) | Workspace Admin (manage) / Regular (set on own entries) | Reading values is wide open; defining or assigning fields is admin-only. |
| User and group admin (`clockify_update_user_role`, `clockify_deactivate_user`, `clockify_create_user_group`, `clockify_delete_user_group`, `clockify_add_user_to_group`, `clockify_remove_user_from_group`) | Workspace Admin (some Owner-only) | `clockify_update_user_role` requires Workspace Admin or Owner; promoting to/from Owner requires Owner. See the API endpoint shape in `internal/tools/tier2_user_admin.go`. |
| Billing — invoices (`clockify_create_invoice`, `clockify_send_invoice`, `clockify_delete_invoice`) | Workspace Admin | Some plans gate invoice features behind the paid tier; the API key still needs admin role even on a free plan. |
| Holidays and expenses (`clockify_create_holiday`, `clockify_create_expense`) | Workspace Admin (create) / Regular (read own) | |
| Working with Owner-only surface (workspace settings, billing plan changes, ownership transfer) | Owner | Not exposed as MCP tools; covered here for completeness. |

Operator checklist when provisioning a key:

1. Pick the **lowest** role that covers every tool family you plan to
   activate. Start with Regular for a personal time-tracking
   deployment; escalate one tier at a time as you enable broader
   tool groups.
2. Cross-reference the chosen role with `CLOCKIFY_POLICY` —
   `time_tracking_safe` paired with a Regular key is the safest
   posture for an untrusted AI agent; `safe_core` requires
   Workspace Admin to actually exercise its create-write surface.
3. If a tool returns `403 Forbidden` or `400 Entity id must be
   present`, the key likely lacks the role the endpoint demands.
   Check the table above before raising the policy.

When in doubt, mirror the layout in `docs/auth-model.md` (MCP auth)
and `docs/runbooks/auth-failures.md` (debug flow): the MCP auth
errors are local; the Clockify role errors are upstream — fix them
at the API key, not at this repo's policy gate.

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
