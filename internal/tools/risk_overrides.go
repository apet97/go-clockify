package tools

import "github.com/apet97/go-clockify/internal/mcp"

// riskOverride carries a per-tool RiskClass and AuditKeys override applied by
// applyRiskMetadata after the boolean-hint default. Only entries that need
// finer granularity than the default (read / write / destructive) appear here.
type riskOverride struct {
	class     mcp.RiskClass
	auditKeys []string
}

// riskOverrides maps exposed one-user tool names to their structured risk
// metadata. Keep keys aligned with Service.FullAccessRegistry; stale legacy
// handler names are guarded by risk_overrides_test.go.
var riskOverrides = map[string]riskOverride{
	// Sensitive reads.
	"clockify_users_profile":                  sensitiveRead("user_id"),
	"clockify_users_list":                     sensitiveRead("workspace_id"),
	"clockify_groups_list":                    sensitiveRead("workspace_id"),
	"clockify_groups_get":                     sensitiveRead("group_id"),
	"clockify_invoices_list":                  sensitiveRead("client_id", "status"),
	"clockify_invoices_get":                   sensitiveRead("invoice_id"),
	"clockify_invoices_export":                sensitiveRead("invoice_id"),
	"clockify_invoices_send_guidance":         readOnly("invoice_id"),
	"clockify_invoices_items_list":            sensitiveRead("invoice_id"),
	"clockify_invoices_items_update_guidance": readOnly("invoice_id"),
	"clockify_invoices_payments_list":         sensitiveRead("invoice_id"),
	"clockify_expenses_list":                  sensitiveRead("user_id", "project_id", "category_id"),
	"clockify_expenses_get":                   sensitiveRead("expense_id"),
	"clockify_expenses_categories_list":       sensitiveRead("workspace_id"),
	"clockify_webhooks_list":                  sensitiveRead("workspace_id"),
	"clockify_webhooks_get":                   sensitiveRead("webhook_id"),
	"clockify_webhooks_events":                sensitiveRead("webhook_id"),
	"clockify_time_off_requests_list":         sensitiveRead("user_id", "policy_id", "status"),
	"clockify_time_off_requests_get":          sensitiveRead("request_id"),
	"clockify_time_off_policies_list":         sensitiveRead("workspace_id"),
	"clockify_time_off_policies_get":          sensitiveRead("policy_id"),
	"clockify_time_off_balances":              sensitiveRead("user_id", "policy_id"),
	"clockify_projects_memberships_list":      sensitiveRead("project_id"),
	"clockify_scheduling_assignments_list":    sensitiveRead("project_id", "user_id"),
	"clockify_scheduling_assignments_get":     sensitiveRead("assignment_id"),
	"clockify_scheduling_project_totals":      sensitiveRead("project_id"),
	"clockify_scheduling_user_totals":         sensitiveRead("user_id"),
	"clockify_scheduling_capacity":            sensitiveRead("user_ids"),
	"clockify_approvals_list":                 sensitiveRead("status", "user_id"),
	"clockify_approvals_get":                  sensitiveRead("approval_id"),
	"clockify_workspace_settings":             sensitiveRead("workspace_id"),

	// Billing.
	"clockify_invoices_create":            billingWrite("client_id", "number", "issued_date", "currency", "due_date"),
	"clockify_invoices_update":            billingWrite("invoice_id", "status", "client_id"),
	"clockify_invoices_delete":            billingDelete("invoice_id"),
	"clockify_invoices_import_time":       billingWrite("invoice_id", "from", "to", "project_ids", "time_entry_group_type"),
	"clockify_invoices_import_expenses":   billingWrite("invoice_id", "from", "to", "project_ids", "time_entry_group_type"),
	"clockify_invoices_items_add":         billingWrite("invoice_id", "item_type", "description", "quantity", "unit_price"),
	"clockify_invoices_items_delete":      billingDelete("invoice_id", "item_index", "item_id"),
	"clockify_invoices_mark_paid":         billingWrite("invoice_id"),
	"clockify_invoices_payments_create":   billingWrite("invoice_id", "amount", "date"),
	"clockify_invoices_payments_delete":   billingDelete("invoice_id", "payment_id"),
	"clockify_expenses_create":            billingWrite("category_id", "project_id", "amount", "date"),
	"clockify_expenses_update":            billingWrite("expense_id", "category_id", "project_id", "amount", "date"),
	"clockify_expenses_delete":            billingDelete("expense_id"),
	"clockify_expenses_categories_create": billingWrite("name", "has_unit_price", "price_in_cents", "unit"),
	"clockify_expenses_categories_update": billingWrite("category_id", "name", "has_unit_price", "price_in_cents", "unit", "archived"),
	"clockify_expenses_categories_delete": billingDelete("category_id"),
	"clockify_projects_rates_update": {
		class:     mcp.RiskWrite | mcp.RiskBilling | mcp.RiskAdmin,
		auditKeys: []string{"project_id", "user_id", "rate_kind", "amount"},
	},
	"clockify_tasks_rates_update": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"project_id", "task_id", "rate_kind", "amount"},
	},
	"clockify_entries_mark_invoiced": billingWrite("time_entry_ids", "invoiced"),

	// Admin and permission changes.
	"clockify_users_invite": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin | mcp.RiskPermissionChange | mcp.RiskExternalSideEffect,
		auditKeys: []string{"email", "emails", "send_email"},
	},
	"clockify_users_deactivate": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"user_id"},
	},
	"clockify_users_role": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"user_id", "role"},
	},
	"clockify_groups_create": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"name"},
	},
	"clockify_groups_update": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"group_id", "name"},
	},
	"clockify_groups_delete": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin,
		auditKeys: []string{"group_id"},
	},
	"clockify_groups_add_user": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"group_id", "user_id"},
	},
	"clockify_groups_remove_user": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"group_id", "user_id"},
	},
	"clockify_projects_memberships_update": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"project_id", "user_ids", "memberships", "remove"},
	},
	"clockify_custom_fields_create": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"name", "type"},
	},
	"clockify_custom_fields_update": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"custom_field_id", "name", "archived"},
	},
	"clockify_custom_fields_delete": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin,
		auditKeys: []string{"custom_field_id"},
	},
	"clockify_custom_fields_set_value": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"custom_field_id", "entity_id"},
	},
	"clockify_time_off_requests_create": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"user_id", "policy_id", "start", "end"},
	},
	"clockify_time_off_requests_update": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"request_id", "user_id", "policy_id", "start", "end"},
	},
	"clockify_time_off_requests_delete": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin,
		auditKeys: []string{"request_id"},
	},
	"clockify_time_off_approve": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"request_id"},
	},
	"clockify_time_off_deny": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"request_id"},
	},
	"clockify_time_off_policies_create": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"name", "assignee_ids", "user_group_ids"},
	},
	"clockify_time_off_policies_update": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"policy_id", "name", "assignee_ids", "user_group_ids"},
	},
	"clockify_time_off_archive": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin,
		auditKeys: []string{"policy_id", "archived"},
	},
	"clockify_time_off_balances_update": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskBilling,
		auditKeys: []string{"policy_id", "user_ids", "value"},
	},
	"clockify_approvals_submit": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"user_id", "start", "end"},
	},
	"clockify_approvals_approve": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"approval_id"},
	},
	"clockify_approvals_reject": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"approval_id"},
	},
	"clockify_approvals_withdraw": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"approval_id"},
	},
	"clockify_approvals_resubmit": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"approval_id", "entry_ids", "expense_ids"},
	},

	// Webhooks have external side effects because they create/update/delete or
	// trigger outbound delivery.
	"clockify_webhooks_create": {
		class:     mcp.RiskWrite | mcp.RiskExternalSideEffect,
		auditKeys: []string{"name", "url", "webhook_event", "trigger_source_type", "trigger_source"},
	},
	"clockify_webhooks_update": {
		class:     mcp.RiskWrite | mcp.RiskExternalSideEffect,
		auditKeys: []string{"webhook_id", "name", "url", "webhook_event", "trigger_source_type", "trigger_source"},
	},
	"clockify_webhooks_delete": {
		class:     mcp.RiskDestructive | mcp.RiskExternalSideEffect,
		auditKeys: []string{"webhook_id"},
	},
	"clockify_webhooks_test_guidance": readOnly("webhook_id"),
}

func readOnly(auditKeys ...string) riskOverride {
	return riskOverride{class: mcp.RiskRead, auditKeys: auditKeys}
}

func sensitiveRead(auditKeys ...string) riskOverride {
	return riskOverride{class: mcp.RiskRead | mcp.RiskSensitiveRead, auditKeys: auditKeys}
}

func billingWrite(auditKeys ...string) riskOverride {
	return riskOverride{class: mcp.RiskWrite | mcp.RiskBilling, auditKeys: auditKeys}
}

func billingDelete(auditKeys ...string) riskOverride {
	return riskOverride{class: mcp.RiskDestructive | mcp.RiskBilling, auditKeys: auditKeys}
}
