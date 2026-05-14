# One-user tool coverage

Coverage ledger for the one-user full-access Clockify MCP. It is derived from `docs/tool-catalog.json` and current fake/live tests. `fake-tested` is `yes` where current tests exercise the advertised tool path directly; `live-tested` remains `yes` only where sacrificial-workspace live tests cover the path.

Summary:
- Total tools: 151
- Workflow tools: 17
- Domain tools: 132
- Raw fallback tools: 2
- Fake-tested paths: all advertised workflow, domain, and raw fallback tools

Live test boundary:
- Live coverage is intentionally narrower than fake coverage and stays limited to the sacrificial workspace.
- No known one-user implementation gaps remain in this ledger.

| Tool | Class | Handler | Fake-tested | Live-tested | Output schema | Status |
|------|-------|---------|-------------|-------------|---------------|--------|
| `clockify_status` | workflow | native handler | yes | yes | action-pinned envelope | ready |
| `clockify_tools_guide` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_create_work_package` | workflow | native handler | yes | yes | action-pinned envelope | ready |
| `clockify_log_work` | workflow | native handler | yes | yes | action-pinned envelope | ready |
| `clockify_start_work` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_stop_work` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_switch_work` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_review_day` | workflow | native handler | yes | yes | action-pinned envelope | ready |
| `clockify_review_week` | workflow | native handler | yes | yes | action-pinned envelope | ready |
| `clockify_fix_entry` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_invoice_client_work` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_record_expense` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_request_time_off` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_schedule_work` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_setup_webhook` | workflow | native handler | yes | no | action-pinned envelope | ready |
| `clockify_demo_seed` | workflow | native handler | yes | yes | action-pinned envelope | ready |
| `clockify_demo_cleanup` | workflow | native handler | yes | yes | action-pinned envelope | ready |
| `clockify_clients_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_clients_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tasks_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tasks_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tags_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tags_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_clients_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_clients_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_clients_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_archive` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_rates_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tasks_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tasks_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tasks_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tasks_rates_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tags_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tags_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_tags_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_templates_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_templates_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_estimates_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_memberships_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_send` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_mark_paid` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_items_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_items_add` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_items_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_items_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_categories_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_categories_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_categories_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_expenses_categories_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_custom_fields_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_custom_fields_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_custom_fields_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_custom_fields_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_custom_fields_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_custom_fields_set_value` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_requests_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_requests_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_requests_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_requests_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_requests_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_approve` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_deny` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_policies_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_policies_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_policies_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_policies_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_balances` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_scheduling_assignments_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_scheduling_assignments_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_scheduling_assignments_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_scheduling_assignments_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_scheduling_assignments_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_scheduling_project_totals` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_approvals_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_approvals_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_approvals_submit` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_approvals_approve` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_approvals_reject` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_approvals_withdraw` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_webhooks_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_webhooks_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_webhooks_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_webhooks_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_webhooks_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_webhooks_test` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_webhooks_events` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_groups_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_groups_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_groups_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_groups_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_groups_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_groups_add_user` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_groups_remove_user` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_holidays_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_holidays_list_for_user_period` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_holidays_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_holidays_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_users_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_users_profile` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_users_deactivate` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_users_role` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_workspace_settings` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_projects_memberships_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_mark_invoiced` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_reports_attendance` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_reports_money` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_reports_expense` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_reports_export` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_export` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_import_time` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_import_expenses` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_payments_list` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_payments_create` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_invoices_payments_delete` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_time_off_archive` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_scheduling_user_totals` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_scheduling_capacity` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_approvals_resubmit` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_holidays_get` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_holidays_update` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_users_invite` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_running` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_timer_start` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_timer_stop` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_timer_status` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_entries_timer_switch` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_reports_detailed` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_reports_summary` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_reports_weekly` | domain | wrapper | yes | no | action-pinned envelope | ready |
| `clockify_api_get` | raw | raw fallback | yes | no | action-pinned envelope | ready |
| `clockify_api_request` | raw | raw fallback | yes | no | action-pinned envelope | ready |
