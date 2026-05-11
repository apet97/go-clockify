package tools

import (
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

// TestHighRiskToolsExposeConfirmationAnnotations pins the
// client-discoverability surface required by ADR 0018 Q4: every
// high-risk tool descriptor must carry
// annotations.requiresConfirmationToken=true and
// annotations.confirmationRiskClass=[...] so MCP clients that read
// annotations.* can surface a "preview first" prompt before issuing
// the call.
func TestHighRiskToolsExposeConfirmationAnnotations(t *testing.T) {
	s := &Service{}
	descriptors := allDescriptors(t, s)

	highRiskNames := []string{
		// Destructive.
		"clockify_delete_entry",
		"clockify_delete_project",
		"clockify_delete_client",
		"clockify_delete_tag",
		"clockify_delete_task",
		// Billing.
		"clockify_send_invoice",
		"clockify_delete_invoice",
		// Admin / permission change.
		"clockify_update_user_role",
		"clockify_deactivate_user",
		"clockify_delete_user_group",
		// External side effect.
		"clockify_test_webhook",
		"clockify_create_webhook",
		"clockify_delete_webhook",
	}
	for _, name := range highRiskNames {
		d, ok := descriptors[name]
		if !ok {
			t.Errorf("%s missing from descriptor registry", name)
			continue
		}
		if !d.RiskClass.IsHighRisk() {
			t.Errorf("%s RiskClass=%b is not high-risk; check internal/tools/risk_overrides.go", name, d.RiskClass)
			continue
		}
		got, _ := d.Tool.Annotations["requiresConfirmationToken"].(bool)
		if !got {
			t.Errorf("%s missing annotations.requiresConfirmationToken=true", name)
		}
		classes, _ := d.Tool.Annotations["confirmationRiskClass"].([]string)
		if len(classes) == 0 {
			t.Errorf("%s missing annotations.confirmationRiskClass", name)
		}
	}
}

// TestLowRiskToolsHaveNoConfirmationAnnotation pins the inverse:
// ordinary writes and read-only tools must not advertise the
// confirmation annotations. Otherwise the time-tracking surface
// (clockify_update_entry, clockify_log_time, etc.) would look like
// it needed a dry-run preview before every call.
func TestLowRiskToolsHaveNoConfirmationAnnotation(t *testing.T) {
	s := &Service{}
	descriptors := allDescriptors(t, s)

	lowRiskNames := []string{
		// Reads.
		"clockify_list_entries",
		"clockify_today_entries",
		"clockify_get_workspace",
		// Ordinary writes.
		"clockify_start_timer",
		"clockify_stop_timer",
		"clockify_add_entry",
		"clockify_log_time",
		"clockify_update_entry",
		"clockify_create_project",
		"clockify_create_client",
		"clockify_create_tag",
		"clockify_create_task",
	}
	for _, name := range lowRiskNames {
		d, ok := descriptors[name]
		if !ok {
			t.Errorf("%s missing from descriptor registry", name)
			continue
		}
		if d.RiskClass.IsHighRisk() {
			t.Errorf("%s RiskClass=%b is unexpectedly high-risk; either update lowRiskNames or fix the override", name, d.RiskClass)
			continue
		}
		if got, _ := d.Tool.Annotations["requiresConfirmationToken"].(bool); got {
			t.Errorf("%s annotations.requiresConfirmationToken should be absent/false", name)
		}
		if _, ok := d.Tool.Annotations["confirmationRiskClass"]; ok {
			t.Errorf("%s annotations.confirmationRiskClass should be absent for low-risk tool", name)
		}
	}
}

// TestHighRiskToolsAcceptConfirmationTokenArg pins the input-schema
// half of Q4 — clients that respect annotations.requiresConfirmationToken
// must be able to pass the returned token back as a normal tool argument
// without tripping additionalProperties:false. Every high-risk mutating
// tool's InputSchema.properties must therefore contain a string-typed
// confirmation_token property.
func TestHighRiskToolsAcceptConfirmationTokenArg(t *testing.T) {
	s := &Service{}
	descriptors := allDescriptors(t, s)

	for name, d := range descriptors {
		if d.ReadOnlyHint {
			continue
		}
		if !d.RiskClass.IsHighRisk() {
			continue
		}
		props, ok := d.Tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Errorf("%s InputSchema has no properties block", name)
			continue
		}
		prop, ok := props["confirmation_token"].(map[string]any)
		if !ok {
			t.Errorf("%s InputSchema.properties.confirmation_token missing", name)
			continue
		}
		if typ, _ := prop["type"].(string); typ != "string" {
			t.Errorf("%s confirmation_token.type = %v, want \"string\"", name, typ)
		}
	}
}

// TestLowRiskToolsDoNotAcceptConfirmationTokenArg keeps the
// confirmation_token property off of low-risk tools so the property
// schema doubles as discoverability: a tool that advertises the
// argument is a tool that requires it.
func TestLowRiskToolsDoNotAcceptConfirmationTokenArg(t *testing.T) {
	s := &Service{}
	descriptors := allDescriptors(t, s)

	for name, d := range descriptors {
		if d.RiskClass.IsHighRisk() {
			continue
		}
		props, _ := d.Tool.InputSchema["properties"].(map[string]any)
		if _, ok := props["confirmation_token"]; ok {
			t.Errorf("%s exposes confirmation_token property but is not high-risk (RiskClass=%b)", name, d.RiskClass)
		}
	}
}

// allDescriptors aggregates Tier-1 + Tier-2 descriptors so the tests
// can look up a tool by name regardless of tier.
func allDescriptors(t *testing.T, s *Service) map[string]mcp.ToolDescriptor {
	t.Helper()
	out := map[string]mcp.ToolDescriptor{}
	for _, d := range s.Registry() {
		out[d.Tool.Name] = d
	}
	for groupName := range Tier2Groups {
		ds, ok := s.Tier2Handlers(groupName)
		if !ok {
			t.Fatalf("Tier2Handlers(%q) returned ok=false", groupName)
		}
		for _, d := range ds {
			out[d.Tool.Name] = d
		}
	}
	return out
}
