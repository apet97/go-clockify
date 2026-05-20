package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
)

func (s *Service) nativeHighValueDescriptors() []mcp.ToolDescriptor {
	sources := s.nativeDomainDescriptorMap()
	out := make([]mcp.ToolDescriptor, 0, 96)
	add := func(priority int, name, oldName, entity, change string, handler func(context.Context, map[string]any) (ResultEnvelope, error)) {
		old, ok := sources[oldName]
		if !ok {
			// A missing source key means a typo or partial merge dropped a
			// high-value tool. Surface it loudly instead of vanishing the
			// tool silently; the registry count test is the hard gate.
			fmt.Fprintf(os.Stderr, "WARNING: descriptor source %q missing for tool %q — skipping\n", oldName, name)
			return
		}
		out = append(out, nativeDirectDescriptor(priority, name, old, entity, change, handler))
	}
	add(200, "clockify_invoices_list", "clockify_list_invoices", "invoice", "", s.listInvoices)
	add(201, "clockify_invoices_get", "clockify_get_invoice", "invoice", "", s.getInvoice)
	add(202, "clockify_invoices_create", "clockify_create_invoice", "invoice", "created", s.createInvoice)
	add(203, "clockify_invoices_update", "clockify_update_invoice", "invoice", "updated", s.updateInvoice)
	add(204, "clockify_invoices_delete", "clockify_delete_invoice", "invoice", "deleted", s.deleteInvoice)
	add(205, "clockify_invoices_send", "clockify_send_invoice", "invoice", "updated", s.sendInvoice)
	add(206, "clockify_invoices_mark_paid", "clockify_mark_invoice_paid", "invoice", "updated", s.markInvoicePaid)
	add(207, "clockify_invoices_items_list", "clockify_list_invoice_items", "invoice_item", "", s.listInvoiceItems)
	add(208, "clockify_invoices_items_add", "clockify_add_invoice_item", "invoice_item", "created", s.addInvoiceItem)
	add(209, "clockify_invoices_items_update", "clockify_update_invoice_item", "invoice_item", "updated", s.updateInvoiceItem)
	add(216, "clockify_invoices_items_delete", "clockify_delete_invoice_item", "invoice_item", "deleted", s.deleteInvoiceItem)
	add(36, "clockify_projects_templates_list", "clockify_list_project_templates", "project_template", "", s.ListProjectTemplates)
	add(37, "clockify_projects_templates_create", "clockify_create_project_template", "project_template", "created", s.CreateProjectTemplate)
	add(38, "clockify_projects_estimates_update", "clockify_update_project_estimate", "project", "updated", s.UpdateProjectEstimate)
	add(40, "clockify_projects_memberships_update", "clockify_update_project_memberships", "membership", "updated", s.updateProjectMembershipsOneUser)
	add(300, "clockify_expenses_list", "clockify_list_expenses", "expense", "", s.listExpenses)
	add(301, "clockify_expenses_get", "clockify_get_expense", "expense", "", s.getExpense)
	add(302, "clockify_expenses_create", "clockify_create_expense", "expense", "created", s.createExpense)
	add(303, "clockify_expenses_update", "clockify_update_expense", "expense", "updated", s.updateExpense)
	add(304, "clockify_expenses_delete", "clockify_delete_expense", "expense", "deleted", s.deleteExpense)
	add(305, "clockify_expenses_categories_list", "clockify_list_expense_categories", "expense_category", "", s.listExpenseCategories)
	add(306, "clockify_expenses_categories_create", "clockify_create_expense_category", "expense_category", "created", s.createExpenseCategory)
	add(307, "clockify_expenses_categories_update", "clockify_update_expense_category", "expense_category", "updated", s.updateExpenseCategory)
	add(308, "clockify_expenses_categories_delete", "clockify_delete_expense_category", "expense_category", "deleted", s.deleteExpenseCategory)
	add(400, "clockify_custom_fields_list", "clockify_list_custom_fields", "custom_field", "", s.ListCustomFields)
	add(401, "clockify_custom_fields_get", "clockify_get_custom_field", "custom_field", "", s.GetCustomField)
	add(402, "clockify_custom_fields_create", "clockify_create_custom_field", "custom_field", "created", s.CreateCustomField)
	add(403, "clockify_custom_fields_update", "clockify_update_custom_field", "custom_field", "updated", s.UpdateCustomField)
	add(404, "clockify_custom_fields_delete", "clockify_delete_custom_field", "custom_field", "deleted", s.DeleteCustomField)
	add(405, "clockify_custom_fields_set_value", "clockify_set_custom_field_value", "custom_field_value", "updated", s.SetCustomFieldValue)
	add(500, "clockify_time_off_requests_list", "clockify_list_time_off_requests", "time_off_request", "", s.listTimeOffRequests)
	add(501, "clockify_time_off_requests_get", "clockify_get_time_off_request", "time_off_request", "", s.getTimeOffRequest)
	add(502, "clockify_time_off_requests_create", "clockify_create_time_off_request", "time_off_request", "created", s.createTimeOffRequest)
	add(503, "clockify_time_off_requests_update", "clockify_update_time_off_request", "time_off_request", "updated", s.updateTimeOffRequest)
	add(504, "clockify_time_off_requests_delete", "clockify_delete_time_off_request", "time_off_request", "deleted", s.deleteTimeOffRequest)
	add(505, "clockify_time_off_approve", "clockify_approve_time_off", "time_off_request", "updated", s.approveTimeOff)
	add(506, "clockify_time_off_deny", "clockify_deny_time_off", "time_off_request", "updated", s.denyTimeOff)
	add(507, "clockify_time_off_policies_list", "clockify_list_time_off_policies", "time_off_policy", "", s.listTimeOffPolicies)
	add(508, "clockify_time_off_policies_get", "clockify_get_time_off_policy", "time_off_policy", "", s.getTimeOffPolicy)
	add(509, "clockify_time_off_policies_create", "clockify_create_time_off_policy", "time_off_policy", "created", s.createTimeOffPolicy)
	add(510, "clockify_time_off_policies_update", "clockify_update_time_off_policy", "time_off_policy", "updated", s.updateTimeOffPolicy)
	add(511, "clockify_time_off_balances", "clockify_time_off_balance", "time_off_balance", "", s.timeOffBalance)
	add(512, "clockify_time_off_balances_update", "clockify_time_off_balance_update", "time_off_balance", "updated", s.updateTimeOffBalance)
	add(600, "clockify_scheduling_assignments_list", "clockify_list_assignments", "assignment", "", s.listAssignments)
	add(601, "clockify_scheduling_assignments_get", "clockify_get_assignment", "assignment", "", s.getAssignment)
	add(602, "clockify_scheduling_assignments_create", "clockify_create_assignment", "assignment", "created", s.createAssignment)
	add(603, "clockify_scheduling_assignments_update", "clockify_update_assignment", "assignment", "updated", s.updateAssignment)
	add(604, "clockify_scheduling_assignments_delete", "clockify_delete_assignment", "assignment", "deleted", s.deleteAssignment)
	add(605, "clockify_scheduling_project_totals", "clockify_get_project_schedule_totals", "scheduling", "", s.getProjectScheduleTotals)
	add(700, "clockify_approvals_list", "clockify_list_approval_requests", "approval", "", s.listApprovalRequests)
	add(701, "clockify_approvals_get", "clockify_get_approval_request", "approval", "", s.getApprovalRequest)
	add(702, "clockify_approvals_submit", "clockify_submit_for_approval", "approval", "created", s.submitForApproval)
	add(703, "clockify_approvals_approve", "clockify_approve_timesheet", "approval", "updated", s.approveTimesheet)
	add(704, "clockify_approvals_reject", "clockify_reject_timesheet", "approval", "updated", s.rejectTimesheet)
	add(705, "clockify_approvals_withdraw", "clockify_withdraw_approval", "approval", "updated", s.withdrawApproval)
	add(800, "clockify_webhooks_list", "clockify_list_webhooks", "webhook", "", s.ListWebhooks)
	add(801, "clockify_webhooks_get", "clockify_get_webhook", "webhook", "", s.GetWebhook)
	add(802, "clockify_webhooks_create", "clockify_create_webhook", "webhook", "created", s.CreateWebhook)
	add(803, "clockify_webhooks_update", "clockify_update_webhook", "webhook", "updated", s.UpdateWebhook)
	add(804, "clockify_webhooks_delete", "clockify_delete_webhook", "webhook", "deleted", s.DeleteWebhook)
	add(805, "clockify_webhooks_test", "clockify_test_webhook", "webhook", "updated", s.TestWebhook)
	add(806, "clockify_webhooks_events", "clockify_list_webhook_events", "webhook_event", "", s.ListWebhookEvents)
	add(900, "clockify_groups_list", "clockify_list_user_groups_admin", "group", "", s.ListUserGroupsAdmin)
	add(901, "clockify_groups_get", "clockify_get_user_group", "group", "", s.GetUserGroup)
	add(902, "clockify_groups_create", "clockify_create_user_group_admin", "group", "created", s.CreateUserGroupAdmin)
	add(903, "clockify_groups_update", "clockify_update_user_group_admin", "group", "updated", s.UpdateUserGroupAdmin)
	add(904, "clockify_groups_delete", "clockify_delete_user_group_admin", "group", "deleted", s.DeleteUserGroupAdmin)
	add(905, "clockify_groups_add_user", "clockify_add_user_to_group", "group_member", "created", s.AddUserToGroup)
	add(906, "clockify_groups_remove_user", "clockify_remove_user_from_group", "group_member", "deleted", s.RemoveUserFromGroup)
	add(1000, "clockify_holidays_list", "clockify_list_holidays", "holiday", "", func(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
		return s.ListHolidays(ctx)
	})
	add(1003, "clockify_holidays_list_for_user_period", "clockify_list_holidays_in_period", "holiday", "", s.ListHolidaysInPeriod)
	add(1004, "clockify_holidays_create", "clockify_create_holiday", "holiday", "created", s.CreateHoliday)
	add(1005, "clockify_holidays_delete", "clockify_delete_holiday", "holiday", "deleted", s.DeleteHoliday)
	add(1103, "clockify_users_deactivate", "clockify_deactivate_user", "user", "updated", s.DeactivateUser)
	add(1104, "clockify_users_role", "clockify_update_user_role", "user", "updated", s.UpdateUserRole)
	out = append(out, s.nativeRouteDescriptors()...)
	out = append(out,
		nativeDomainTool(65, toolRWIdem("clockify_entries_mark_invoiced", "Mark time entries as invoiced or not invoiced.", objectSchema(map[string]any{"required": []string{"time_entry_ids", "invoiced"}, "properties": map[string]any{
			"time_entry_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"invoiced":       map[string]any{"type": "boolean"},
		}})), "entry", "updated", s.EntriesMarkInvoiced),
		nativeDomainTool(1102, toolRW("clockify_users_invite", "Invite users by email. Destructive permission-changing external side effect when send_email is true; supports dry_run.", objectSchema(map[string]any{"properties": map[string]any{
			"email":      map[string]any{"type": "string", "format": "email"},
			"emails":     map[string]any{"type": "array", "items": map[string]any{"type": "string", "format": "email"}},
			"dry_run":    map[string]any{"type": "boolean"},
			"send_email": map[string]any{"type": "boolean", "description": "Whether Clockify should send invitation email. Defaults to true."},
		}})), "user", "created", s.UsersInvite),
	)
	return out
}

func (s *Service) nativeRouteDescriptors() []mcp.ToolDescriptor {
	out := []mcp.ToolDescriptor{
		nativeDomainTool(39, toolRO("clockify_projects_memberships_list", "List project memberships read from the hydrated project record; the full membership set is returned on a single page.", objectSchema(map[string]any{
			"required":   []string{"project_id"},
			"properties": map[string]any{"project_id": map[string]any{"type": "string", "description": "Project ID"}},
		})), "membership", "", s.ListProjectMemberships),
	}
	out = append(out,
		nativeDomainTool(100, toolRO("clockify_reports_attendance", "Run the attendance report. Raw report amounts are in minor units (cents); meta.totalAmount gives normalized major-unit totals per currency. Large results truncate to the size cap.", reportInputSchema()), "report", "", s.AttendanceReport),
		nativeDomainTool(101, toolRO("clockify_reports_money", "Run money summary for billing-rate breakdowns by user or project. Use clockify_reports_summary for simple time totals. Raw amounts are minor units; meta.totalAmount gives major-unit totals. Large results truncate to the size cap.", reportInputSchema()), "report", "", s.MoneyReport),
		nativeDomainTool(102, toolRO("clockify_reports_expense", "Run the detailed expense report. Raw report amounts are in minor units (cents); meta.totalAmount gives normalized major-unit totals per currency. Large results truncate to the size cap.", reportInputSchema()), "report", "", s.ExpenseReport),
		nativeDomainTool(103, toolRO("clockify_reports_export", "Export a detailed report. JSON returns contentType, filename, bytes, bodyEncoding, body base64 payload, base64Bytes, truncated:false. CSV/PDF/XLSX/ZIP use bodyEncoding:\"file\" and path. Amounts are minor units.", reportInputSchema()), "report", "", s.DetailedReport),
	)
	out = append(out, s.explicitInvoiceNativeDescriptors()...)
	out = append(out, nativeDomainTool(506, toolRWIdem("clockify_time_off_archive", "Archive or reactivate a time off policy. Destructive safety hint: archived policies leave active use.", objectSchema(map[string]any{
		"required": []string{"policy_id", "archived"},
		"properties": map[string]any{
			"policy_id": map[string]any{"type": "string", "description": "Time-off policy ID"},
			"archived":  map[string]any{"type": "boolean", "description": "Required. Pass true to archive the policy; pass false to reactivate it."},
		},
	})), "time_off", "updated", s.archiveTimeOffPolicy))
	out = append(out, s.explicitPostTimeOffNativeDescriptors()...)
	return out
}

func (s *Service) explicitInvoiceNativeDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		nativeDomainTool(210, toolRO("clockify_invoices_export", "Export invoice PDF only; Clockify invoice export has no CSV/XLSX (use clockify_reports_export). Returns contentType, filename, bytes, bodyEncoding, body/base64 payload, base64Bytes, truncated:false, or path.", objectSchema(map[string]any{
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id":  map[string]any{"type": "string", "description": "Invoice ID"},
				"format":      map[string]any{"type": "string", "enum": []string{"PDF"}, "description": "Invoice export format. Clockify only produces PDF here; use clockify_reports_export for CSV/XLSX data exports."},
				"user_locale": map[string]any{"type": "string", "description": "Locale for the exported document, e.g. en"},
			},
		})), "invoice_export", "", s.exportInvoiceOneUser),
		nativeDomainTool(211, toolRW("clockify_invoices_import_time", "Import a client's billable time entries onto an invoice. Clockify imports every billable time entry in the from..to date range; narrow it with project_ids.", objectSchema(map[string]any{
			"required": []string{"invoice_id", "from", "to"},
			"properties": map[string]any{
				"invoice_id":                  map[string]any{"type": "string", "description": "Invoice ID."},
				"from":                        map[string]any{"type": "string", "description": "Start of the billing period (YYYY-MM-DD or RFC3339). All billable time on or after this is imported."},
				"to":                          map[string]any{"type": "string", "description": "End of the billing period (YYYY-MM-DD or RFC3339)."},
				"project_ids":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional. Limit the import to these project IDs; omit to import all of the client's billable time in the range."},
				"time_entry_group_type":       map[string]any{"type": "string", "enum": []string{"SINGLE_ITEM", "GROUPED", "DETAILED"}, "description": "How imported entries are grouped on the invoice: SINGLE_ITEM, GROUPED, or DETAILED (default). GROUPED also requires time_entry_primary_group_by."},
				"time_entry_primary_group_by": map[string]any{"type": "string", "enum": []string{"USER", "PROJECT", "DATE"}, "description": "Primary grouping dimension. Required when time_entry_group_type is GROUPED."},
			},
		})), "invoice_item", "updated", s.importInvoiceTimeOneUser),
		nativeDomainTool(212, toolRW("clockify_invoices_import_expenses", "Import a client's billable time and expenses onto an invoice for a date range. Clockify's import endpoint always imports time; this tool also imports billable expenses.", objectSchema(map[string]any{
			"required": []string{"invoice_id", "from", "to"},
			"properties": map[string]any{
				"invoice_id":                  map[string]any{"type": "string", "description": "Invoice ID."},
				"from":                        map[string]any{"type": "string", "description": "Start of the billing period (YYYY-MM-DD or RFC3339)."},
				"to":                          map[string]any{"type": "string", "description": "End of the billing period (YYYY-MM-DD or RFC3339)."},
				"project_ids":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional. Limit the import to these project IDs; omit to import all of the client's billable items in the range."},
				"time_entry_group_type":       map[string]any{"type": "string", "enum": []string{"SINGLE_ITEM", "GROUPED", "DETAILED"}, "description": "How imported entries are grouped on the invoice: SINGLE_ITEM, GROUPED, or DETAILED (default). GROUPED also requires time_entry_primary_group_by."},
				"time_entry_primary_group_by": map[string]any{"type": "string", "enum": []string{"USER", "PROJECT", "DATE"}, "description": "Primary grouping dimension. Required when time_entry_group_type is GROUPED."},
			},
		})), "invoice_item", "updated", s.importInvoiceExpensesOneUser),
		nativeDomainTool(213, toolRO("clockify_invoices_payments_list", "List the payments recorded against an invoice, paginated via page and page_size.", objectSchema(map[string]any{
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string", "description": "Invoice ID"},
				"page":       map[string]any{"type": "integer"},
				"page_size":  map[string]any{"type": "integer"},
			},
		})), "payment", "", s.listInvoicePayments),
		nativeDomainTool(214, toolRW("clockify_invoices_payments_create", "Create an invoice payment. amount defaults to minor units (cents), matching the live AddInvoicePaymentRequest body; pass amount_unit:\"major\" to enter the value in major currency units instead.", objectSchema(map[string]any{
			"required": []string{"invoice_id", "amount", "date"},
			"properties": map[string]any{
				"invoice_id":  map[string]any{"type": "string", "description": "Invoice ID"},
				"amount":      map[string]any{"type": "number", "description": "Payment amount"},
				"amount_unit": invoiceMinorUnitSchema("amount"),
				"date":        map[string]any{"type": "string", "description": "Payment date"},
				"note":        map[string]any{"type": "string", "description": "Optional payment note"},
			},
		})), "payment", "created", s.createInvoicePaymentOneUser),
		nativeDomainTool(215, toolDestructive("clockify_invoices_payments_delete", "Permanently delete an invoice payment. Billing impact; destructive; supports dry_run preview.", objectSchema(map[string]any{
			"required": []string{"invoice_id", "payment_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"payment_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean", "description": "If true, preview the deletion without executing it."},
			},
		})), "payment", "deleted", s.deleteInvoicePaymentOneUser),
	}
}

func (s *Service) explicitPostTimeOffNativeDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		nativeDomainTool(606, toolRO("clockify_scheduling_user_totals", "Get scheduled assignment totals for one user.", objectSchema(map[string]any{
			"required": []string{"user_id"},
			"properties": map[string]any{
				"user_id": map[string]any{"type": "string", "description": "User ID"},
				"start":   map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":     map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			},
		})), "scheduling", "", s.schedulingUserTotalsOneUser),
		nativeDomainTool(607, toolRO("clockify_scheduling_capacity", "Get workspace capacity totals. Defaults to every workspace user; pass user_ids to scope to specific users.", objectSchema(map[string]any{
			"properties": map[string]any{
				"start":    map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":      map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"user_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional. User IDs to scope the capacity totals; omit to include all workspace users."},
			},
		})), "scheduling", "", s.schedulingCapacityOneUser),
		nativeDomainTool(704, toolRW("clockify_approvals_resubmit", "Resubmit rejected or withdrawn entries and expenses and update approval state.", objectSchema(map[string]any{
			"required": []string{"approval_id", "entry_ids", "period", "period_start"},
			"properties": map[string]any{
				"approval_id":  map[string]any{"type": "string", "description": "Approval request ID"},
				"entry_ids":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Time entry IDs to resubmit"},
				"expense_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Expense IDs to resubmit"},
				"period":       map[string]any{"type": "string", "enum": []string{"WEEKLY", "SEMI_MONTHLY", "MONTHLY"}, "description": "Approval period; must match workspace approval settings"},
				"period_start": map[string]any{"type": "string", "description": "Period start in RFC3339/millisecond form, e.g. 2026-05-01T00:00:00.000Z"},
				"note":         map[string]any{"type": "string", "description": "Optional resubmission note"},
			},
		})), "approval", "updated", s.resubmitApprovalOneUser),
		nativeDomainTool(1001, toolRO("clockify_holidays_get", "Get one holiday by ID from the pinned workspace.", objectSchema(map[string]any{
			"required":   []string{"holiday_id"},
			"properties": map[string]any{"holiday_id": map[string]any{"type": "string"}},
		})), "holiday", "", s.GetHoliday),
		nativeDomainTool(1002, toolRWIdem("clockify_holidays_update", "Update a holiday by ID; unspecified fields merge from the existing record.", objectSchema(map[string]any{
			"required": []string{"holiday_id"},
			"properties": map[string]any{
				"holiday_id":      map[string]any{"type": "string", "description": "Holiday ID"},
				"name":            map[string]any{"type": "string", "description": "Holiday name"},
				"start_date":      map[string]any{"type": "string", "description": "Holiday start date (YYYY-MM-DD)"},
				"end_date":        map[string]any{"type": "string", "description": "Holiday end date (YYYY-MM-DD)"},
				"occurs_annually": map[string]any{"type": "boolean", "description": "Whether the holiday repeats every year"},
				"user_ids":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User IDs the holiday applies to"},
				"user_group_ids":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User group IDs the holiday applies to"},
			},
		})), "holiday", "updated", s.UpdateHoliday),
	}
}

func (s *Service) updateProjectMembershipsOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	if _, ok := args["memberships"]; ok {
		return s.UpdateProjectMemberships(ctx, args)
	}
	if _, ok := args["user_ids"]; ok {
		return s.SetProjectMemberships(ctx, args)
	}
	return s.UpdateProjectMemberships(ctx, args)
}

func nativeDirectDescriptor(priority int, name string, old mcp.ToolDescriptor, entity, change string, handler func(context.Context, map[string]any) (ResultEnvelope, error)) mcp.ToolDescriptor {
	tool := old.Tool
	tool.Name = name
	adjustOneUserNativeSchema(name, tool.InputSchema)
	if tool.Annotations == nil {
		tool.Annotations = map[string]any{}
	}
	delete(tool.Annotations, "wraps")
	tool.Annotations["handlerKind"] = "native handler"
	return firstSliceDescriptor(priority, tool, aliasHandler(name, entity, change, handler))
}

func adjustOneUserNativeSchema(name string, schema map[string]any) {
	if schema == nil {
		return
	}
	switch name {
	case "clockify_groups_update":
		schema["required"] = []string{"group_id", "name"}
	case "clockify_projects_memberships_update":
		props, _ := schema["properties"].(map[string]any)
		if props == nil {
			props = map[string]any{}
			schema["properties"] = props
		}
		props["user_ids"] = stringArraySchema("User IDs to set as members")
		props["hourly_rate"] = map[string]any{"type": "number", "description": "Hourly rate for all members when using user_ids compatibility input"}
		schema["required"] = []string{"project_id"}
		// The "memberships OR user_ids" rule is enforced by the routed
		// handler (Update/SetProjectMemberships). It is intentionally not a
		// top-level anyOf: Anthropic tool input schemas must be a plain
		// object, and a sibling anyOf breaks subagent launches.
	}
}

func (s *Service) nativeDomainDescriptorMap() map[string]mcp.ToolDescriptor {
	out := map[string]mcp.ToolDescriptor{}
	add := func(descriptors []mcp.ToolDescriptor) {
		descriptors = normalizeDescriptors(descriptors)
		for _, descriptor := range descriptors {
			out[descriptor.Tool.Name] = descriptor
		}
	}
	add([]mcp.ToolDescriptor{
		{Tool: toolRO("clockify_users_list", "List users in the pinned workspace, paginated via page and page_size.", userListSchema()), Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListUsers(ctx, args)
		}},
		{Tool: toolRO("clockify_users_profile", "Get the current Clockify user.", map[string]any{"type": "object"}), Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			return s.CurrentUser(ctx)
		}},
	})
	add(invoiceHandlers(s))
	add(expenseHandlers(s))
	add(customFieldHandlers(s))
	add(timeOffHandlers(s))
	add(schedulingHandlers(s))
	add(approvalHandlers(s))
	add(webhookHandlers(s))
	add(groupsHolidaysHandlers(s))
	add(projectAdminHandlers(s))
	add(userAdminHandlers(s))
	return out
}

func nativeDomainTool(priority int, tool mcp.Tool, entity, change string, handler func(context.Context, map[string]any) (ResultEnvelope, error)) mcp.ToolDescriptor {
	if tool.Annotations == nil {
		tool.Annotations = map[string]any{}
	}
	tool.Annotations["handlerKind"] = "native handler"
	return firstSliceDescriptor(priority, tool, aliasHandler(tool.Name, entity, change, handler))
}

func rateKindEndpoint(args map[string]any) (string, error) {
	switch strings.ToLower(strings.TrimSpace(stringArg(args, "rate_kind"))) {
	case "hourly":
		return "hourly-rate", nil
	case "cost":
		return "cost-rate", nil
	default:
		return "", fmt.Errorf("rate_kind must be hourly or cost")
	}
}
