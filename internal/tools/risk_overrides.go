package tools

import "github.com/apet97/go-clockify/internal/mcp"

// riskOverride carries a per-tool RiskClass and AuditKeys override applied by
// applyRiskMetadata after the boolean-hint default. Only entries that need
// finer granularity than the default (read / write / destructive) appear
// here. The taxonomy mirrors docs/policy/production-tool-scope.md.
type riskOverride struct {
	class     mcp.RiskClass
	auditKeys []string
}

// riskOverrides maps Tier-1 / Tier-2 tool names to their structured risk
// metadata. Adding a new billing, admin, permission-change, or
// external-side-effect tool means adding it here so that the audit recorder
// captures the action-defining fields and policy/enforcement consumers see
// the right risk bits.
var riskOverrides = map[string]riskOverride{
	// Probe-lab generic callers can reach documented billing, admin,
	// permission-change, and external-side-effect routes, so their static
	// classification must be conservatively broad.
	"clockify_call_documented_write_api": {
		class:     mcp.RiskWrite | mcp.RiskBilling | mcp.RiskAdmin | mcp.RiskPermissionChange | mcp.RiskExternalSideEffect,
		auditKeys: []string{"operation", "method", "path"},
	},
	"clockify_call_documented_delete_api": {
		class:     mcp.RiskDestructive | mcp.RiskBilling | mcp.RiskAdmin | mcp.RiskPermissionChange | mcp.RiskExternalSideEffect,
		auditKeys: []string{"operation", "method", "path"},
	},

	// Sensitive reads — user, policy, billing, balance, and webhook
	// surfaces should be visible to audit/agents without joining the
	// confirmation-token high-risk mask.
	"clockify_current_user": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"user_id"},
	},
	"clockify_list_users": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"workspace_id"},
	},
	"clockify_policy_info": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"policy"},
	},
	"clockify_get_user_group": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"group_id"},
	},
	"clockify_list_user_groups_admin": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"workspace_id"},
	},
	"clockify_export_invoice": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"invoice_id"},
	},
	"clockify_get_invoice": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"invoice_id"},
	},
	"clockify_invoice_report": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"date_range", "client_id"},
	},
	"clockify_list_invoice_items": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"invoice_id"},
	},
	"clockify_list_invoices": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"client_id", "status"},
	},
	"clockify_get_time_off_policy": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"policy_id"},
	},
	"clockify_time_off_balance": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"user_id", "policy_id"},
	},
	"clockify_get_member_profile": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"user_id"},
	},
	"clockify_list_user_groups": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"workspace_id"},
	},
	"clockify_get_webhook": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"webhook_id"},
	},
	"clockify_list_webhook_events": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"webhook_id"},
	},
	"clockify_list_webhooks": {
		class:     mcp.RiskRead | mcp.RiskSensitiveRead,
		auditKeys: []string{"workspace_id"},
	},

	// Billing — invoices.
	"clockify_send_invoice": {
		class:     mcp.RiskWrite | mcp.RiskBilling | mcp.RiskExternalSideEffect,
		auditKeys: []string{"invoice_id"},
	},
	"clockify_mark_invoice_paid": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"invoice_id"},
	},
	"clockify_create_invoice": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"client_id", "number", "issued_date", "currency", "due_date"},
	},
	"clockify_update_invoice": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"invoice_id", "status", "client_id"},
	},
	"clockify_delete_invoice": {
		class:     mcp.RiskDestructive | mcp.RiskBilling,
		auditKeys: []string{"invoice_id"},
	},
	"clockify_add_invoice_item": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"invoice_id", "item_type", "description", "quantity", "unit_price"},
	},
	"clockify_update_invoice_item": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"invoice_id", "item_index", "item_id", "item_type", "description", "quantity", "unit_price"},
	},
	"clockify_delete_invoice_item": {
		class:     mcp.RiskDestructive | mcp.RiskBilling,
		auditKeys: []string{"invoice_id", "item_index", "item_id"},
	},

	// Admin / permission changes — user_admin.
	"clockify_update_user_role": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"user_id", "role"},
	},
	"clockify_deactivate_user": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"user_id"},
	},

	// Admin — user groups.
	"clockify_create_user_group": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"name"},
	},
	"clockify_update_user_group": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"group_id", "name"},
	},
	"clockify_delete_user_group": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin,
		auditKeys: []string{"group_id"},
	},
	"clockify_add_user_to_group": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"group_id", "user_id"},
	},
	"clockify_remove_user_from_group": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin,
		auditKeys: []string{"group_id", "user_id"},
	},
	"clockify_update_project_memberships": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"project_id"},
	},
	"clockify_set_project_memberships": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"project_id", "user_ids"},
	},
	"clockify_assign_project_memberships": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		auditKeys: []string{"project_id", "user_ids", "remove"},
	},
	"clockify_update_project_user_cost_rate": {
		class:     mcp.RiskWrite | mcp.RiskBilling | mcp.RiskAdmin,
		auditKeys: []string{"project_id", "user_id", "amount"},
	},
	"clockify_update_project_user_hourly_rate": {
		class:     mcp.RiskWrite | mcp.RiskBilling | mcp.RiskAdmin,
		auditKeys: []string{"project_id", "user_id", "amount"},
	},
	"clockify_update_task_cost_rate": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"project_id", "task_id", "amount"},
	},
	"clockify_update_task_hourly_rate": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"project_id", "task_id", "amount"},
	},

	// Webhooks — external side effects.
	"clockify_create_webhook": {
		class:     mcp.RiskWrite | mcp.RiskExternalSideEffect,
		auditKeys: []string{"name", "url", "webhook_event", "trigger_source_type", "trigger_source"},
	},
	"clockify_update_webhook": {
		class:     mcp.RiskWrite | mcp.RiskExternalSideEffect,
		auditKeys: []string{"webhook_id", "name", "url", "webhook_event", "trigger_source_type", "trigger_source"},
	},
	"clockify_delete_webhook": {
		class:     mcp.RiskDestructive | mcp.RiskExternalSideEffect,
		auditKeys: []string{"webhook_id"},
	},
	"clockify_test_webhook": {
		class:     mcp.RiskWrite | mcp.RiskExternalSideEffect,
		auditKeys: []string{"webhook_id"},
	},
}
