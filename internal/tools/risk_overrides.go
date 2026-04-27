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
		auditKeys: []string{"client_id", "currency", "due_date"},
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
		auditKeys: []string{"invoice_id", "description", "quantity", "unit_price"},
	},
	"clockify_update_invoice_item": {
		class:     mcp.RiskWrite | mcp.RiskBilling,
		auditKeys: []string{"invoice_id", "item_id", "description", "quantity", "unit_price"},
	},
	"clockify_delete_invoice_item": {
		class:     mcp.RiskDestructive | mcp.RiskBilling,
		auditKeys: []string{"invoice_id", "item_id"},
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
	"clockify_activate_user": {
		class:     mcp.RiskWrite | mcp.RiskAdmin,
		auditKeys: []string{"user_id"},
	},
	"clockify_invite_user": {
		class:     mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskExternalSideEffect,
		auditKeys: []string{"email", "role"},
	},
	"clockify_remove_user_from_workspace": {
		class:     mcp.RiskDestructive | mcp.RiskAdmin,
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

	// Webhooks — external side effects.
	"clockify_create_webhook": {
		class:     mcp.RiskWrite | mcp.RiskExternalSideEffect,
		auditKeys: []string{"url", "events"},
	},
	"clockify_update_webhook": {
		class:     mcp.RiskWrite | mcp.RiskExternalSideEffect,
		auditKeys: []string{"webhook_id", "url", "events"},
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
