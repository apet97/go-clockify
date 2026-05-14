# One-user tool coverage

Coverage ledger for the one-user full-access Clockify MCP. It is derived from `docs/tool-catalog.json` plus the current fake/live test posture. It is intentionally conservative: descriptor-only coverage is not counted as fake smoke, live protocol/recovery coverage means the sacrificial workspace exercises either success or a useful recovery envelope, and live happy-path coverage means a real created/read entity succeeded.

Summary:
- Total tools: 151
- Workflow tools: 17
- Domain tools: 132
- Raw fallback tools: 2
- Fake-smoke yes: 151
- Live protocol/recovery tested yes: 149
- Live happy-path tested yes: 77

Remaining honest gaps:
- Fake smoke means the fake server asserts envelope, ID, and recovery shape; it is not a claim that every Clockify plan enables the feature.
- No alias wrappers remain in the one-user surface; former read/list/get wrappers now use native one-user handlers.
- Live protocol/recovery coverage is intentionally broader than live happy-path coverage. Optional paid-feature domains may still return recovery when the workspace plan or permissions do not allow the operation.
- Every `Live protocol/recovery tested: yes` row is backed by named live-test evidence and required gate metadata in `internal/tools/oneuser_quality_test.go`; happy-path rows are tracked separately and only mark real success paths.
- Workflow schemas and the first CRUD/native-conversion slice now advertise typed data schemas; remaining generic envelopes are tracked per row.

| Tool | Class | Handler | Endpoint / method | Fake smoke | Live protocol/recovery tested | Live happy-path tested | Output schema | Status | Next action |
|------|-------|---------|-------------------|-------------|--------------------------------|------------------------|---------------|--------|-------------|
| `clockify_status` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tools_guide` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_create_work_package` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_log_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_start_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_stop_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_switch_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_review_day` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_review_week` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_fix_entry` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_invoice_client_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_record_expense` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_request_time_off` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_schedule_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_setup_webhook` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_demo_seed` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_demo_cleanup` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_clients_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_clients_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_projects_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tasks_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_tasks_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tags_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tags_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_entries_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_entries_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_clients_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_clients_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_clients_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_archive` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_rates_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_tasks_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tasks_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tasks_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tasks_rates_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_tags_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tags_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tags_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_entries_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_entries_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_entries_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_templates_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_templates_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_estimates_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_projects_memberships_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_invoices_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_invoices_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_send` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_mark_paid` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_items_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_items_add` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_items_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_items_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_expenses_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_categories_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_categories_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_categories_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_categories_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_custom_fields_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_custom_fields_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_custom_fields_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_custom_fields_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_custom_fields_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_custom_fields_set_value` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_time_off_requests_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_time_off_requests_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_requests_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_requests_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_requests_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_approve` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_deny` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_policies_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_time_off_policies_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_policies_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_policies_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_balances` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_scheduling_assignments_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_scheduling_assignments_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_scheduling_assignments_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_scheduling_assignments_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_scheduling_assignments_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_scheduling_project_totals` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_approvals_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_approvals_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_approvals_submit` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_approvals_approve` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_approvals_reject` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_approvals_withdraw` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_webhooks_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_webhooks_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_webhooks_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_webhooks_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_webhooks_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_webhooks_test` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_webhooks_events` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_groups_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_groups_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_groups_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_groups_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_groups_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_groups_add_user` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_groups_remove_user` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_holidays_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_holidays_list_for_user_period` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_holidays_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_holidays_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_users_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_users_profile` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_users_deactivate` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_users_role` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_workspace_settings` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_projects_memberships_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_entries_mark_invoiced` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_reports_attendance` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_reports_money` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_reports_expense` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_reports_export` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_export` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_import_time` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_import_expenses` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_payments_list` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_payments_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_invoices_payments_delete` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_time_off_archive` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_scheduling_user_totals` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_scheduling_capacity` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_approvals_resubmit` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_holidays_get` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_holidays_update` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_users_invite` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_entries_running` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_entries_timer_start` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_entries_timer_stop` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_entries_timer_status` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_entries_timer_switch` | domain | native handler | native handler; endpoint selected in code | yes | yes | no | typed | ready | maintain_recovery_probe |
| `clockify_reports_detailed` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_reports_summary` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_reports_weekly` | domain | native handler | native handler; endpoint selected in code | yes | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_api_get` | raw | raw fallback | GET caller-supplied path | yes | raw_fallback_only | raw_fallback_only | generic | raw_fallback_only | keep_raw_fallback_last |
| `clockify_api_request` | raw | raw fallback | caller-supplied method/path | yes | raw_fallback_only | raw_fallback_only | generic | raw_fallback_only | keep_raw_fallback_last |
