package tools

import (
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

// TestEveryDescriptorHasRiskClass enforces that the post-normalize pass
// stamps a non-zero RiskClass on every Tier-1 and Tier-2 descriptor. A
// zero RiskClass would silently skip risk-aware policy/audit gates, so
// the matrix test fails closed.
func TestEveryDescriptorHasRiskClass(t *testing.T) {
	s := &Service{}

	for _, d := range s.Registry() {
		if d.RiskClass == 0 {
			t.Errorf("tier-1 descriptor %q has zero RiskClass", d.Tool.Name)
		}
	}

	for groupName := range Tier2Groups {
		ds, ok := s.Tier2Handlers(groupName)
		if !ok {
			t.Fatalf("Tier2Handlers(%q) returned ok=false", groupName)
		}
		for _, d := range ds {
			if d.RiskClass == 0 {
				t.Errorf("tier-2 descriptor %q (group %q) has zero RiskClass", d.Tool.Name, groupName)
			}
		}
	}
}

// TestRiskOverridesMatchTaxonomy locks in the per-tool overrides that the
// audit findings mandated. If any of these regress to a default class,
// audit/policy consumers downstream silently drop the billing/admin/
// permission_change distinction and the security review fails.
func TestRiskOverridesMatchTaxonomy(t *testing.T) {
	s := &Service{}

	wantClass := map[string]mcp.RiskClass{
		"clockify_send_invoice":           mcp.RiskWrite | mcp.RiskBilling | mcp.RiskExternalSideEffect,
		"clockify_mark_invoice_paid":      mcp.RiskWrite | mcp.RiskBilling,
		"clockify_delete_invoice":         mcp.RiskDestructive | mcp.RiskBilling,
		"clockify_add_invoice_item":       mcp.RiskWrite | mcp.RiskBilling,
		"clockify_update_user_role":       mcp.RiskWrite | mcp.RiskAdmin | mcp.RiskPermissionChange,
		"clockify_deactivate_user":        mcp.RiskWrite | mcp.RiskAdmin,
		"clockify_remove_user_from_group": mcp.RiskDestructive | mcp.RiskAdmin,
		"clockify_test_webhook":           mcp.RiskWrite | mcp.RiskExternalSideEffect,
	}
	wantAuditKeys := map[string][]string{
		"clockify_send_invoice":      {"invoice_id"},
		"clockify_mark_invoice_paid": {"invoice_id"},
		"clockify_update_user_role":  {"user_id", "role"},
		"clockify_add_invoice_item":  {"invoice_id", "description", "quantity", "unit_price"},
		"clockify_test_webhook":      {"webhook_id"},
	}

	got := map[string]mcp.ToolDescriptor{}
	for _, d := range s.Registry() {
		got[d.Tool.Name] = d
	}
	for groupName := range Tier2Groups {
		ds, _ := s.Tier2Handlers(groupName)
		for _, d := range ds {
			got[d.Tool.Name] = d
		}
	}

	for name, want := range wantClass {
		d, ok := got[name]
		if !ok {
			t.Errorf("expected tool %q in registry, not found", name)
			continue
		}
		if d.RiskClass != want {
			t.Errorf("%s: RiskClass=%b want=%b", name, d.RiskClass, want)
		}
	}
	for name, want := range wantAuditKeys {
		d, ok := got[name]
		if !ok {
			continue
		}
		if !equalStrings(d.AuditKeys, want) {
			t.Errorf("%s: AuditKeys=%v want=%v", name, d.AuditKeys, want)
		}
	}
}

// TestDefaultRiskClassFromBooleans verifies the boolean→RiskClass fallback
// for tools that have no explicit override. A read-only tool must default
// to RiskRead, a destructive tool to RiskDestructive, otherwise RiskWrite.
func TestDefaultRiskClassFromBooleans(t *testing.T) {
	s := &Service{}
	got := map[string]mcp.ToolDescriptor{}
	for _, d := range s.Registry() {
		got[d.Tool.Name] = d
	}

	cases := []struct {
		name string
		want mcp.RiskClass
	}{
		// Tier-1 read-only — these have no riskOverrides entry, so they
		// must come from the boolean default.
		{"clockify_get_workspace", mcp.RiskRead},
		{"clockify_list_projects", mcp.RiskRead},
	}
	for _, c := range cases {
		d, ok := got[c.name]
		if !ok {
			t.Logf("skip %s: not in registry", c.name)
			continue
		}
		if _, overridden := riskOverrides[c.name]; overridden {
			t.Logf("skip %s: has explicit override", c.name)
			continue
		}
		if d.RiskClass != c.want {
			t.Errorf("%s: RiskClass=%b want=%b", c.name, d.RiskClass, c.want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
