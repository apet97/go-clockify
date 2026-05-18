# Permissions And Plan Requirements

This MCP runs with one Clockify API key against one pinned workspace. `doctor
--live` checks the key, workspace, owner status, feature plan, feature flags,
and prints the same optional feature summary produced by
`doctorFeatureSummary` in `cmd/clockify-mcp/main.go`:

```text
invoices=...,expenses=...,timeOff=...,scheduling=...,approvals=...,webhooks=...,customFields=...,reports=...
```

`OK (owner)` is the cleanest signal. A non-owner key can still pass live doctor
with a warning, but admin, billing, and settings families may fail with
Clockify permission errors or `feature_unavailable` recovery envelopes.

| Tool family | Typical tools | Role needed | Plan / feature dependency | Notes |
| --- | --- | --- | --- | --- |
| Core entries / projects | `clockify_entries_*`, `clockify_projects_*`, `clockify_tasks_*`, `clockify_tags_*`, workflow tools | Member for own time; admin/owner for workspace-wide edits and memberships | Time tracking base product | Project and task rate edits are billing/admin-sensitive even though basic project reads are broadly available. |
| Reports | `clockify_reports_*`, `clockify_review_*` | Member for visible data; admin/owner for workspace-wide visibility | `reports` optional feature may be advertised by `doctor --live` | Report exports can be large and may return a temp-file path instead of inline content. |
| Invoices / expenses | `clockify_invoices_*`, `clockify_expenses_*` | Admin or owner | `invoices` and `expenses`; workspace plan dependent | These affect billing records and often support `dry_run` before mutation. |
| Time off | `clockify_time_off_*` | Admin or owner for policies, approvals, balances, and other users | `timeOff`; workspace plan dependent | Current-user request flows are safer than policy and balance operations, but Clockify still enforces workspace permissions. |
| Scheduling | `clockify_scheduling_*` | Admin or owner for assignment writes and publish | `scheduling`; workspace plan dependent | Publish changes shared schedule state; live evidence keeps it recovery-only unless a happy-path campaign opts in. |
| Approvals | `clockify_approvals_*` | Submit can use the caller; approve/reject/withdraw/resubmit require approver/admin rights | `approvals`; workspace plan dependent | Dry-run previews are available for submit/approve/reject/withdraw. |
| Webhooks | `clockify_webhooks_*` | Admin or owner | `webhooks`; may be plan dependent | Create/update deliver events to external HTTPS URLs; private and loopback targets are rejected. |
| Users / groups | `clockify_users_*`, `clockify_groups_*` | Admin or owner | Workspace administration | Invites, role changes, access removal, and group membership edits are permission-change operations. |
| Workspace settings | `clockify_workspace_settings` | Admin or owner | Settings surface; workspace plan dependent | Treat writes as admin changes even when reads succeed. |
| Custom fields / holidays | `clockify_custom_fields_*`, `clockify_holidays_*` | Admin or owner for writes | `customFields` for custom fields; holidays may be plan dependent | Custom fields affect validation and downstream project/entry data. |
| Raw fallback | `clockify_api_get`, `clockify_api_request` | Same role/plan as the target Clockify endpoint | Same as target endpoint | Raw writes are disabled unless `CLOCKIFY_ENABLE_RAW_WRITES=true`, and documented-route fencing is on by default. |

When a feature is absent or not visible in `doctor --live`, prefer the workflow
or domain tool first anyway. The tool should return `ok:false` with a recovery
hint instead of forcing the operator to guess whether the failure is permission,
plan, or endpoint availability.
