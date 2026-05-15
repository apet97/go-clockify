package tools

import (
	"reflect"
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
		"clockify_invoices_send":               mcp.RiskWrite | mcp.RiskBilling | mcp.RiskExternalSideEffect,
		"clockify_invoices_payments_create":    mcp.RiskWrite | mcp.RiskBilling,
		"clockify_expenses_categories_delete":  mcp.RiskDestructive | mcp.RiskBilling,
		"clockify_projects_rates_update":       mcp.RiskWrite | mcp.RiskBilling | mcp.RiskAdmin,
		"clockify_webhooks_test":               mcp.RiskWrite | mcp.RiskExternalSideEffect,
		"clockify_users_invite":                mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange | mcp.RiskExternalSideEffect,
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

func riskTestRegistry(t *testing.T) map[string]mcp.ToolDescriptor {
	t.Helper()
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	out := map[string]mcp.ToolDescriptor{}
	for _, descriptor := range svc.FullAccessRegistry() {
		out[descriptor.Tool.Name] = descriptor
	}
	return out
}
