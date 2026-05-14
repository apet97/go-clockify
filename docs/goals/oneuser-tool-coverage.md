# One-user tool coverage

Coverage ledger for the one-user full-access Clockify MCP. It is derived from `docs/tool-catalog.json` plus the current fake/live test posture. It is intentionally conservative: descriptor-only coverage is not counted as fake smoke, and live-tested means a sacrificial-workspace live path currently exercises the one-user surface.

Summary:
- Total tools: 151
- Workflow tools: 17
- Domain tools: 132
- Raw fallback tools: 2
- Fake-smoke yes: 42
- Live-tested yes: 29

Remaining honest gaps:
- Fake smoke means the fake server asserts envelope, ID, and recovery shape; it is not a claim that every Clockify plan enables the feature.
- Remaining alias wrappers are classified as `usable_wrapper` or `needs_native_handler`; route descriptors and unprobed native tools carry explicit next actions.
- Live coverage is intentionally narrower than fake coverage and remains limited to the sacrificial workspace. Paid-feature domains such as invoices, expenses, time off, scheduling, webhooks, groups, and holidays may return recovery when the workspace plan or permissions do not allow the operation.
- Workflow schemas and the first CRUD/native-conversion slice now advertise typed data schemas; remaining generic envelopes are tracked per row.

| Tool | Class | Handler | Endpoint / method | Fake smoke | Live-tested | Output schema | Status | Next action |
|------|-------|---------|-------------------|-------------|-------------|---------------|--------|-------------|
| `clockify_status` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_tools_guide` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_create_work_package` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_log_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_start_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_stop_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_switch_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_review_day` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_review_week` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_fix_entry` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_invoice_client_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_record_expense` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_request_time_off` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_schedule_work` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_setup_webhook` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_demo_seed` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_demo_cleanup` | workflow | native handler | native composite; may call multiple Clockify endpoints | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_clients_list` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_clients_create` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_projects_list` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_projects_create` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_tasks_list` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_tasks_create` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_tags_list` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_tags_create` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_entries_list` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_entries_create` | domain | native handler | native handler; endpoint selected in code | yes | no | typed | needs_live_probe | add_live_probe |
| `clockify_clients_get` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_clients_update` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_clients_delete` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_projects_get` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_projects_update` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_projects_delete` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_projects_archive` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_projects_rates_update` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_tasks_get` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_tasks_update` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_tasks_delete` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_tasks_rates_update` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_tags_get` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_tags_update` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_tags_delete` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_entries_get` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_entries_update` | domain | native handler | native handler; endpoint selected in code | no | no | typed | needs_fake_smoke | add_fake_smoke |
| `clockify_entries_delete` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_projects_templates_list` | domain | alias wrapper | wraps clockify_list_project_templates | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_projects_templates_create` | domain | alias wrapper | wraps clockify_create_project_template | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_projects_estimates_update` | domain | alias wrapper | wraps clockify_update_project_estimate | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_projects_memberships_update` | domain | alias wrapper | wraps clockify_update_project_memberships | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_invoices_list` | domain | alias wrapper | wraps clockify_list_invoices | no | yes | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_invoices_get` | domain | alias wrapper | wraps clockify_get_invoice | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_invoices_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_invoices_update` | domain | alias wrapper | wraps clockify_update_invoice | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_invoices_delete` | domain | alias wrapper | wraps clockify_delete_invoice | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_invoices_send` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_invoices_mark_paid` | domain | alias wrapper | wraps clockify_mark_invoice_paid | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_invoices_items_list` | domain | alias wrapper | wraps clockify_list_invoice_items | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_invoices_items_add` | domain | alias wrapper | wraps clockify_add_invoice_item | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_invoices_items_update` | domain | alias wrapper | wraps clockify_update_invoice_item | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_invoices_items_delete` | domain | alias wrapper | wraps clockify_delete_invoice_item | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_expenses_list` | domain | alias wrapper | wraps clockify_list_expenses | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_expenses_get` | domain | alias wrapper | wraps clockify_get_expense | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_expenses_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_expenses_update` | domain | alias wrapper | wraps clockify_update_expense | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_expenses_delete` | domain | alias wrapper | wraps clockify_delete_expense | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_expenses_categories_list` | domain | alias wrapper | wraps clockify_list_expense_categories | no | yes | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_expenses_categories_create` | domain | alias wrapper | wraps clockify_create_expense_category | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_expenses_categories_update` | domain | alias wrapper | wraps clockify_update_expense_category | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_expenses_categories_delete` | domain | alias wrapper | wraps clockify_delete_expense_category | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_custom_fields_list` | domain | alias wrapper | wraps clockify_list_custom_fields | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_custom_fields_get` | domain | alias wrapper | wraps clockify_get_custom_field | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_custom_fields_create` | domain | alias wrapper | wraps clockify_create_custom_field | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_custom_fields_update` | domain | alias wrapper | wraps clockify_update_custom_field | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_custom_fields_delete` | domain | alias wrapper | wraps clockify_delete_custom_field | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_custom_fields_set_value` | domain | alias wrapper | wraps clockify_set_custom_field_value | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_requests_list` | domain | alias wrapper | wraps clockify_list_time_off_requests | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_time_off_requests_get` | domain | alias wrapper | wraps clockify_get_time_off_request | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_requests_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_time_off_requests_update` | domain | alias wrapper | wraps clockify_update_time_off_request | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_requests_delete` | domain | alias wrapper | wraps clockify_delete_time_off_request | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_approve` | domain | alias wrapper | wraps clockify_approve_time_off | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_deny` | domain | alias wrapper | wraps clockify_deny_time_off | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_policies_list` | domain | alias wrapper | wraps clockify_list_time_off_policies | no | yes | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_time_off_policies_get` | domain | alias wrapper | wraps clockify_get_time_off_policy | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_policies_create` | domain | alias wrapper | wraps clockify_create_time_off_policy | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_policies_update` | domain | alias wrapper | wraps clockify_update_time_off_policy | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_time_off_balances` | domain | alias wrapper | wraps clockify_time_off_balance | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_scheduling_assignments_list` | domain | alias wrapper | wraps clockify_list_assignments | no | yes | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_scheduling_assignments_get` | domain | alias wrapper | wraps clockify_get_assignment | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_scheduling_assignments_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_scheduling_assignments_update` | domain | alias wrapper | wraps clockify_update_assignment | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_scheduling_assignments_delete` | domain | alias wrapper | wraps clockify_delete_assignment | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_scheduling_project_totals` | domain | alias wrapper | wraps clockify_get_project_schedule_totals | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_approvals_list` | domain | alias wrapper | wraps clockify_list_approval_requests | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_approvals_get` | domain | alias wrapper | wraps clockify_get_approval_request | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_approvals_submit` | domain | alias wrapper | wraps clockify_submit_for_approval | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_approvals_approve` | domain | alias wrapper | wraps clockify_approve_timesheet | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_approvals_reject` | domain | alias wrapper | wraps clockify_reject_timesheet | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_approvals_withdraw` | domain | alias wrapper | wraps clockify_withdraw_approval | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_webhooks_list` | domain | alias wrapper | wraps clockify_list_webhooks | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_webhooks_get` | domain | alias wrapper | wraps clockify_get_webhook | no | no | typed | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_webhooks_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_webhooks_update` | domain | alias wrapper | wraps clockify_update_webhook | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_webhooks_delete` | domain | alias wrapper | wraps clockify_delete_webhook | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_webhooks_test` | domain | alias wrapper | wraps clockify_test_webhook | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_webhooks_events` | domain | alias wrapper | wraps clockify_list_webhook_events | no | yes | generic | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_groups_list` | domain | alias wrapper | wraps clockify_list_user_groups_admin | no | yes | generic | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_groups_get` | domain | alias wrapper | wraps clockify_get_user_group | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_groups_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_groups_update` | domain | alias wrapper | wraps clockify_update_user_group_admin | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_groups_delete` | domain | alias wrapper | wraps clockify_delete_user_group_admin | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_groups_add_user` | domain | alias wrapper | wraps clockify_add_user_to_group | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_groups_remove_user` | domain | alias wrapper | wraps clockify_remove_user_from_group | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_holidays_list` | domain | alias wrapper | wraps clockify_list_holidays | no | yes | generic | usable_wrapper | monitor_usage_or_convert_if_hot |
| `clockify_holidays_list_for_user_period` | domain | alias wrapper | wraps clockify_list_holidays_in_period | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_holidays_create` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_holidays_delete` | domain | alias wrapper | wraps clockify_delete_holiday | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_users_list` | domain | native handler | native handler; endpoint selected in code | yes | no | generic | needs_live_probe | add_live_probe |
| `clockify_users_profile` | domain | native handler | native handler; endpoint selected in code | yes | no | generic | needs_live_probe | add_live_probe |
| `clockify_users_deactivate` | domain | alias wrapper | wraps clockify_deactivate_user | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_users_role` | domain | alias wrapper | wraps clockify_update_user_role | no | no | generic | needs_native_handler | convert_native_handler |
| `clockify_workspace_settings` | domain | native handler | native handler; endpoint selected in code | yes | no | generic | needs_live_probe | add_live_probe |
| `clockify_projects_memberships_list` | domain | route descriptor | GET /workspaces/{workspaceId}/projects/{project_id}/memberships | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_entries_mark_invoiced` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_reports_attendance` | domain | route descriptor | POST /workspaces/{workspaceId}/reports/attendance | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_reports_money` | domain | route descriptor | POST /workspaces/{workspaceId}/reports/summary | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_reports_expense` | domain | route descriptor | POST /workspaces/{workspaceId}/reports/expenses/detailed | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_reports_export` | domain | route descriptor | POST /workspaces/{workspaceId}/reports/detailed | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_invoices_export` | domain | route descriptor | GET /workspaces/{workspaceId}/invoices/{invoice_id}/export | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_invoices_import_time` | domain | route descriptor | POST /workspaces/{workspaceId}/invoices/{invoice_id}/items/import | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_invoices_import_expenses` | domain | route descriptor | POST /workspaces/{workspaceId}/invoices/{invoice_id}/items/import | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_invoices_payments_list` | domain | route descriptor | GET /workspaces/{workspaceId}/invoices/{invoice_id}/payments | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_invoices_payments_create` | domain | route descriptor | POST /workspaces/{workspaceId}/invoices/{invoice_id}/payments | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_invoices_payments_delete` | domain | route descriptor | DELETE /workspaces/{workspaceId}/invoices/{invoice_id}/payments/{payment_id} | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_time_off_archive` | domain | route descriptor | PATCH /workspaces/{workspaceId}/time-off/policies/{policy_id} | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_scheduling_user_totals` | domain | route descriptor | GET /workspaces/{workspaceId}/scheduling/assignments/users/{user_id}/totals | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_scheduling_capacity` | domain | route descriptor | GET /workspaces/{workspaceId}/scheduling/assignments/user-filter/totals | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_approvals_resubmit` | domain | route descriptor | POST /workspaces/{workspaceId}/approval-requests/resubmit-entries-for-approval | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_holidays_get` | domain | route descriptor | GET /workspaces/{workspaceId}/holidays/{holiday_id} | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_holidays_update` | domain | route descriptor | PUT /workspaces/{workspaceId}/holidays/{holiday_id} | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_users_invite` | domain | native handler | native handler; endpoint selected in code | yes | yes | typed | ready | maintain_contract_tests |
| `clockify_entries_running` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_entries_timer_start` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_entries_timer_stop` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_entries_timer_status` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_entries_timer_switch` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_reports_detailed` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_reports_summary` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_reports_weekly` | domain | native handler | native handler; endpoint selected in code | no | no | generic | needs_fake_smoke | add_fake_smoke |
| `clockify_api_get` | raw | raw fallback | GET caller-supplied path | yes | no | generic | raw_fallback_only | keep_raw_fallback_last |
| `clockify_api_request` | raw | raw fallback | caller-supplied method/path | yes | no | generic | raw_fallback_only | keep_raw_fallback_last |
