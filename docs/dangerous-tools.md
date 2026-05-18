# Dangerous Tools And Dry Runs

This page lists tools whose generated `risk_class` includes destructive,
billing, admin, permission-change, or external-side-effect risk. Prefer
`dry_run:true` where available, confirm returned IDs, and keep live happy-path
campaigns limited to the sacrificial workspace described in `docs/live-tests.md`.

| Tool | Risk | Changes | Dry run |
| --- | --- | --- | --- |
| `clockify_clients_delete` | destructive | Deletes a client after the handler checks active projects. | yes |
| `clockify_projects_delete` | destructive | Deletes a project by ID or name. | yes |
| `clockify_projects_rates_update` | billing, admin | Changes project member hourly or cost rates. | yes |
| `clockify_tasks_delete` | destructive | Marks a non-DONE task done when required, then deletes it. | yes |
| `clockify_tasks_rates_update` | billing | Changes task rates used by future billable entries. | yes |
| `clockify_tags_delete` | destructive | Deletes a tag. | yes |
| `clockify_entries_delete` | destructive | Deletes a time entry. | yes |
| `clockify_invoices_create` | billing | Creates an invoice. | yes |
| `clockify_invoices_update` | billing | Updates invoice fields or status. | yes |
| `clockify_invoices_delete` | billing, destructive | Deletes an invoice. | yes |
| `clockify_invoices_send` | billing, external side effect | Unsupported by Clockify API; returns guidance for UI email send. | no |
| `clockify_invoices_mark_paid` | billing | Checks status and guides payment creation when needed. | yes |
| `clockify_invoices_items_add` | billing | Adds invoice line items. | yes |
| `clockify_invoices_items_update` | billing | Unsupported by Clockify API; delete and re-add the line instead. | no |
| `clockify_invoices_items_delete` | billing, destructive | Deletes an invoice line by index. | yes |
| `clockify_invoices_import_time` | billing | Imports billable time into an invoice. | no |
| `clockify_invoices_import_expenses` | billing | Imports billable time and expenses into an invoice. | no |
| `clockify_invoices_payments_create` | billing | Creates an invoice payment. | no |
| `clockify_invoices_payments_delete` | billing, destructive | Deletes an invoice payment. | yes |
| `clockify_projects_memberships_update` | admin, permission change | Replaces project memberships, groups, and rate metadata. | yes |
| `clockify_expenses_create` | billing | Creates an expense, optionally with receipt upload. | yes |
| `clockify_expenses_update` | billing | Updates an expense multipart body. | no |
| `clockify_expenses_delete` | billing, destructive | Deletes an expense. | yes |
| `clockify_expenses_categories_create` | billing | Creates an expense category. | yes |
| `clockify_expenses_categories_update` | billing | Updates an expense category. | yes |
| `clockify_expenses_categories_delete` | billing, destructive | Deletes an expense category. | yes |
| `clockify_custom_fields_create` | admin | Creates a custom field. | no |
| `clockify_custom_fields_update` | admin | Updates a custom field. | no |
| `clockify_custom_fields_delete` | admin, destructive | Deletes a custom field. | yes |
| `clockify_custom_fields_set_value` | admin | Sets a project or time-entry custom-field value. | no |
| `clockify_time_off_requests_create` | admin | Creates a time-off request under a policy. | yes |
| `clockify_time_off_requests_update` | admin | Updates a time-off request. | yes |
| `clockify_time_off_requests_delete` | admin, destructive | Deletes a time-off request. | yes |
| `clockify_time_off_approve` | admin, permission change | Approves a pending time-off request. | no |
| `clockify_time_off_deny` | admin, permission change | Denies a pending time-off request. | no |
| `clockify_time_off_policies_create` | admin | Creates a simplified time-off policy. | no |
| `clockify_time_off_policies_update` | admin | Updates a time-off policy by merging supplied fields. | no |
| `clockify_time_off_archive` | admin | Archives or reactivates a time-off policy. | no |
| `clockify_time_off_balances_update` | billing, admin | Adjusts user balances under a policy. | yes |
| `clockify_scheduling_assignments_delete` | destructive | Deletes a recurring scheduling assignment. | yes |
| `clockify_scheduling_publish` | write | Publishes schedule changes for a date range. | yes |
| `clockify_approvals_submit` | admin | Submits the caller's timesheet for approval. | yes |
| `clockify_approvals_approve` | admin, permission change | Approves an approval request. | yes |
| `clockify_approvals_reject` | admin, permission change | Rejects an approval request. | yes |
| `clockify_approvals_withdraw` | admin, permission change | Withdraws an approval request. | yes |
| `clockify_approvals_resubmit` | admin, permission change | Resubmits rejected or withdrawn entries and expenses. | no |
| `clockify_webhooks_create` | external side effect | Creates a subscription that delivers to an HTTPS URL. | yes |
| `clockify_webhooks_update` | external side effect | Changes future outbound webhook deliveries. | yes |
| `clockify_webhooks_delete` | external side effect, destructive | Stops future deliveries by deleting a webhook. | yes |
| `clockify_webhooks_test` | external side effect | Unsupported by Clockify API; returns guidance for triggering a real event. | no |
| `clockify_groups_create` | admin | Creates a user group. | no |
| `clockify_groups_update` | admin | Updates a user group. | no |
| `clockify_groups_delete` | admin, destructive | Deletes a user group. | yes |
| `clockify_groups_add_user` | admin, permission change | Adds a user to a group. | yes |
| `clockify_groups_remove_user` | admin, permission change, destructive | Removes a user from a group. | yes |
| `clockify_holidays_delete` | destructive | Deletes a holiday. | yes |
| `clockify_users_deactivate` | admin | Deactivates a workspace user and removes access. | yes |
| `clockify_users_role` | admin, permission change | Updates a user's workspace role. | yes |
| `clockify_users_invite` | admin, permission change, external side effect | Invites users by email when `send_email` is true. | yes |
| `clockify_entries_mark_invoiced` | billing | Marks time entries invoiced or not invoiced. | no |

Unsupported tools with risky labels do not call upstream; they return clean
`unsupported` guidance. `dry_run:true` previews the resolved payload and should
not be treated as live happy-path evidence unless the live test explicitly says
that recovery-only evidence is the target.
