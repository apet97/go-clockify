package tools

import (
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
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
		"clockify_send_invoice":        {"invoice_id"},
		"clockify_mark_invoice_paid":   {"invoice_id"},
		"clockify_create_invoice":      {"client_id", "number", "issued_date", "currency", "due_date"},
		"clockify_update_user_role":    {"user_id", "role"},
		"clockify_add_invoice_item":    {"invoice_id", "item_type", "description", "quantity", "unit_price"},
		"clockify_update_invoice_item": {"invoice_id", "item_index", "item_id", "item_type", "description", "quantity", "unit_price"},
		"clockify_create_webhook":      {"name", "url", "webhook_event", "trigger_source_type", "trigger_source"},
		"clockify_update_webhook":      {"webhook_id", "name", "url", "webhook_event", "trigger_source_type", "trigger_source"},
		"clockify_test_webhook":        {"webhook_id"},
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

// TestRiskOverridesTargetRegisteredTools catches ghost risk metadata for
// tools that are not exposed in the generated catalog. User-invite style
// endpoints are particularly easy to leave behind as planned names; keeping
// the override map descriptor-backed prevents docs and audit review from
// implying unsupported tools exist.
func TestRiskOverridesTargetRegisteredTools(t *testing.T) {
	s := &Service{}

	known := map[string]bool{}
	for _, d := range s.Registry() {
		known[d.Tool.Name] = true
	}
	for groupName := range Tier2Groups {
		ds, ok := s.Tier2Handlers(groupName)
		if !ok {
			t.Fatalf("Tier2Handlers(%q) returned ok=false", groupName)
		}
		for _, d := range ds {
			known[d.Tool.Name] = true
		}
	}

	for name := range riskOverrides {
		if !known[name] {
			t.Errorf("risk override %q has no registered tool descriptor", name)
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

// TestSafeCoreAllowlistExcludesDestructiveTools is the registry-level
// pin matching the policy-level TestSafeCoreBlocksDestructiveDeletes:
// every tool advertised in `policy.Describe()["safe_core_writes"]`
// must have both `DestructiveHint == false` and `RiskClass` without
// the `RiskDestructive` bit. The docs and the policy mode agree that
// safe_core is non-destructive; this test prevents a future
// contributor from adding a tool back to the allowlist whose
// descriptor is destructive without also flipping the contract.
func TestSafeCoreAllowlistExcludesDestructiveTools(t *testing.T) {
	s := &Service{}
	descriptors := map[string]mcp.ToolDescriptor{}
	for _, d := range s.Registry() {
		descriptors[d.Tool.Name] = d
	}
	for groupName := range Tier2Groups {
		ds, ok := s.Tier2Handlers(groupName)
		if !ok {
			t.Fatalf("Tier2Handlers(%q) returned ok=false", groupName)
		}
		for _, d := range ds {
			descriptors[d.Tool.Name] = d
		}
	}

	p := &policy.Policy{Mode: policy.SafeCore}
	desc := p.Describe()
	list, ok := desc["safe_core_writes"].([]string)
	if !ok {
		t.Fatalf("policy.Describe()[\"safe_core_writes\"] missing or wrong type: %T", desc["safe_core_writes"])
	}
	if len(list) == 0 {
		t.Fatal("policy.Describe()[\"safe_core_writes\"] is empty; the allowlist must contain non-destructive writes")
	}
	for _, name := range list {
		d, ok := descriptors[name]
		if !ok {
			// A name in the allowlist without a registered descriptor
			// would be caught by TestRiskOverridesTargetRegisteredTools
			// or contract-matrix drift; do not double-report here.
			continue
		}
		if d.DestructiveHint {
			t.Errorf("%s appears in safe_core_writes but DestructiveHint=true; safe_core must not include destructive tools", name)
		}
		if d.RiskClass.Has(mcp.RiskDestructive) {
			t.Errorf("%s appears in safe_core_writes but RiskClass has RiskDestructive set", name)
		}
	}
}
