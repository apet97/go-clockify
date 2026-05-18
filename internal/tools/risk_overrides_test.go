package tools

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

func TestRiskOverridesApplyToRenamedOneUserTools(t *testing.T) {
	registry := riskTestRegistry(t)
	tests := map[string]mcp.RiskClass{
		"clockify_invoices_create":             mcp.RiskWrite | mcp.RiskBilling,
		"clockify_invoices_delete":             mcp.RiskDestructive | mcp.RiskBilling,
		"clockify_invoices_send":               mcp.RiskDestructive | mcp.RiskBilling | mcp.RiskExternalSideEffect,
		"clockify_invoices_payments_create":    mcp.RiskWrite | mcp.RiskBilling,
		"clockify_expenses_categories_delete":  mcp.RiskDestructive | mcp.RiskBilling,
		"clockify_projects_rates_update":       mcp.RiskWrite | mcp.RiskBilling | mcp.RiskAdmin,
		"clockify_webhooks_test":               mcp.RiskWrite | mcp.RiskExternalSideEffect,
		"clockify_users_invite":                mcp.RiskDestructive | mcp.RiskAdmin | mcp.RiskPermissionChange | mcp.RiskExternalSideEffect,
		"clockify_groups_remove_user":          mcp.RiskDestructive | mcp.RiskAdmin | mcp.RiskPermissionChange,
		"clockify_projects_memberships_update": mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		"clockify_time_off_approve":            mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		"clockify_approvals_approve":           mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		"clockify_custom_fields_set_value":     mcp.RiskWrite | mcp.RiskAdmin,
		"clockify_invoices_payments_list":      mcp.RiskRead | mcp.RiskSensitiveRead,
		"clockify_workspace_settings":          mcp.RiskRead | mcp.RiskSensitiveRead,
	}

	for name, want := range tests {
		descriptor, ok := registry[name]
		if !ok {
			t.Fatalf("FullAccessRegistry missing %s", name)
		}
		if descriptor.RiskClass != want {
			t.Fatalf("%s RiskClass = %v, want %v", name, descriptor.RiskClass, want)
		}
		gotNames, ok := descriptor.Tool.Annotations["riskClass"].([]string)
		if !ok {
			t.Fatalf("%s riskClass annotation = %#v, want []string", name, descriptor.Tool.Annotations["riskClass"])
		}
		if wantNames := riskClassAnnotationNames(want); !reflect.DeepEqual(gotNames, wantNames) {
			t.Fatalf("%s riskClass annotation = %#v, want %#v", name, gotNames, wantNames)
		}
	}
}

func TestRiskOverridesCoverSideEffectToolFamilies(t *testing.T) {
	registry := riskTestRegistry(t)
	tests := []struct {
		name string
		want mcp.RiskClass
	}{
		// Invoices and invoice payments are billing; send also delivers externally.
		{"clockify_invoices_create", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_invoices_update", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_invoices_delete", mcp.RiskDestructive | mcp.RiskBilling},
		{"clockify_invoices_import_time", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_invoices_import_expenses", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_invoices_items_add", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_invoices_items_update", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_invoices_items_delete", mcp.RiskDestructive | mcp.RiskBilling},
		{"clockify_invoices_mark_paid", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_invoices_payments_create", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_invoices_payments_delete", mcp.RiskDestructive | mcp.RiskBilling},
		{"clockify_invoices_send", mcp.RiskDestructive | mcp.RiskBilling | mcp.RiskExternalSideEffect},

		// Expenses and expense categories affect billable money.
		{"clockify_expenses_create", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_expenses_update", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_expenses_delete", mcp.RiskDestructive | mcp.RiskBilling},
		{"clockify_expenses_categories_create", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_expenses_categories_update", mcp.RiskWrite | mcp.RiskBilling},
		{"clockify_expenses_categories_delete", mcp.RiskDestructive | mcp.RiskBilling},

		// Admin and permission-changing surfaces.
		{"clockify_users_invite", mcp.RiskDestructive | mcp.RiskAdmin | mcp.RiskPermissionChange | mcp.RiskExternalSideEffect},
		{"clockify_users_deactivate", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_users_role", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_groups_create", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_groups_update", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_groups_delete", mcp.RiskDestructive | mcp.RiskAdmin},
		{"clockify_groups_add_user", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_groups_remove_user", mcp.RiskDestructive | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_projects_memberships_update", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_projects_rates_update", mcp.RiskWrite | mcp.RiskBilling | mcp.RiskAdmin},
		{"clockify_tasks_rates_update", mcp.RiskWrite | mcp.RiskBilling},

		// Webhooks create/update/test/delete affect outbound delivery.
		{"clockify_webhooks_create", mcp.RiskWrite | mcp.RiskExternalSideEffect},
		{"clockify_webhooks_update", mcp.RiskWrite | mcp.RiskExternalSideEffect},
		{"clockify_webhooks_test", mcp.RiskWrite | mcp.RiskExternalSideEffect},
		{"clockify_webhooks_delete", mcp.RiskDestructive | mcp.RiskExternalSideEffect},

		// Approval state changes are admin-sensitive; decision-state changes alter permissions/workflow state.
		{"clockify_approvals_submit", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_approvals_approve", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_approvals_reject", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_approvals_withdraw", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_approvals_resubmit", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},

		// Time-off writes shape sensitive workspace policy/request state.
		{"clockify_time_off_requests_create", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_time_off_requests_update", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_time_off_requests_delete", mcp.RiskDestructive | mcp.RiskAdmin},
		{"clockify_time_off_approve", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_time_off_deny", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange},
		{"clockify_time_off_policies_create", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_time_off_policies_update", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_time_off_archive", mcp.RiskDestructive | mcp.RiskAdmin},
		// Balance writes adjust accrued PTO; admin scope plus billing
		// because the balance drives future paid-time-off liability.
		{"clockify_time_off_balances_update", mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskBilling},

		// Custom fields are workspace configuration; delete is destructive.
		{"clockify_custom_fields_create", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_custom_fields_update", mcp.RiskWrite | mcp.RiskAdmin},
		{"clockify_custom_fields_delete", mcp.RiskDestructive | mcp.RiskAdmin},
		{"clockify_custom_fields_set_value", mcp.RiskWrite | mcp.RiskAdmin},
	}

	for _, tt := range tests {
		assertToolRiskClass(t, registry, tt.name, tt.want)
	}
}

func TestRiskOverridesMarkSensitiveReads(t *testing.T) {
	registry := riskTestRegistry(t)
	tests := []string{
		"clockify_users_profile",
		"clockify_users_list",
		"clockify_groups_list",
		"clockify_groups_get",
		"clockify_invoices_list",
		"clockify_invoices_get",
		"clockify_invoices_export",
		"clockify_invoices_items_list",
		"clockify_invoices_payments_list",
		"clockify_expenses_list",
		"clockify_expenses_get",
		"clockify_expenses_categories_list",
		"clockify_webhooks_list",
		"clockify_webhooks_get",
		"clockify_webhooks_events",
		"clockify_time_off_requests_list",
		"clockify_time_off_requests_get",
		"clockify_time_off_policies_list",
		"clockify_time_off_policies_get",
		"clockify_time_off_balances",
		"clockify_projects_memberships_list",
		"clockify_scheduling_assignments_list",
		"clockify_scheduling_assignments_get",
		"clockify_scheduling_project_totals",
		"clockify_scheduling_user_totals",
		"clockify_scheduling_capacity",
		"clockify_approvals_list",
		"clockify_approvals_get",
		"clockify_workspace_settings",
	}

	for _, name := range tests {
		assertToolRiskClass(t, registry, name, mcp.RiskRead|mcp.RiskSensitiveRead)
	}
}

// TestHighRiskToolDescriptionsSignalSideEffects pins the agent-facing UX
// contract: every high-risk tool description must surface at least one
// keyword per active risk bit so callers can decide intent without
// inspecting the riskClass annotation. Keyword sets are intentionally
// generous to allow rephrasing, but each bit must have *some* signal.
func TestHighRiskToolDescriptionsSignalSideEffects(t *testing.T) {
	keywords := map[mcp.RiskClass][]string{
		mcp.RiskBilling: {
			"bill", "invoice", "expense", "payment", "rate ",
			"balance", "money", "pto", "accrual", "currency",
		},
		mcp.RiskAdmin: {
			"admin", "workspace", "policy", "membership",
			"custom field", "user group", "approval", "approve",
			"deny", "reject", "withdraw", "resubmit", "invite",
			"deactivat", "role", "permission", "submit",
			"holiday",
		},
		mcp.RiskPermissionChange: {
			"permission", "role", "approval", "approve", "deny",
			"reject", "withdraw", "resubmit", "membership",
			"invite", "remove", "add a user",
		},
		mcp.RiskExternalSideEffect: {
			"external", "email", "outbound", "deliver", "send",
			"test ", "invite",
		},
		mcp.RiskDestructive: {
			"delete", "remove", "permanent", "destructive",
		},
	}

	type bit struct {
		mask mcp.RiskClass
		name string
	}
	highBits := []bit{
		{mcp.RiskBilling, "RiskBilling"},
		{mcp.RiskAdmin, "RiskAdmin"},
		{mcp.RiskPermissionChange, "RiskPermissionChange"},
		{mcp.RiskExternalSideEffect, "RiskExternalSideEffect"},
		{mcp.RiskDestructive, "RiskDestructive"},
	}

	registry := riskTestRegistry(t)
	for name, descriptor := range registry {
		if !descriptor.RiskClass.IsHighRisk() {
			continue
		}
		desc := strings.ToLower(descriptor.Tool.Description)
		for _, b := range highBits {
			if !descriptor.RiskClass.Has(b.mask) {
				continue
			}
			if !containsAnyKeyword(desc, keywords[b.mask]) {
				t.Errorf("%s carries %s but description lacks any matching keyword (one of %v): %q",
					name, b.name, keywords[b.mask], descriptor.Tool.Description)
			}
		}
	}
}

func containsAnyKeyword(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func TestRiskOverridesKeysAreExposedOneUserTools(t *testing.T) {
	registry := riskTestRegistry(t)
	allowStale := map[string]string{}
	for name := range riskOverrides {
		if _, ok := registry[name]; ok {
			continue
		}
		if reason := allowStale[name]; reason != "" {
			continue
		}
		t.Fatalf("riskOverrides contains stale key %q; update it to the exposed one-user tool name or document an inline allowlist reason", name)
	}
}

func assertToolRiskClass(t *testing.T, registry map[string]mcp.ToolDescriptor, name string, want mcp.RiskClass) {
	t.Helper()
	descriptor, ok := registry[name]
	if !ok {
		t.Fatalf("FullAccessRegistry missing %s", name)
	}
	if descriptor.RiskClass != want {
		t.Fatalf("%s RiskClass = %v, want %v", name, descriptor.RiskClass, want)
	}
	gotNames, ok := descriptor.Tool.Annotations["riskClass"].([]string)
	if !ok {
		t.Fatalf("%s riskClass annotation = %#v, want []string", name, descriptor.Tool.Annotations["riskClass"])
	}
	if wantNames := riskClassAnnotationNames(want); !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("%s riskClass annotation = %#v, want %#v", name, gotNames, wantNames)
	}
}

func riskTestRegistry(t *testing.T) map[string]mcp.ToolDescriptor {
	t.Helper()
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	out := map[string]mcp.ToolDescriptor{}
	for _, descriptor := range svc.FullAccessRegistry() {
		out[descriptor.Tool.Name] = descriptor
	}
	return out
}
