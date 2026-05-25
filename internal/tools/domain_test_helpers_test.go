package tools

import (
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

// mustRegistry calls FullAccessRegistryChecked and fatals the test on error.
// Use this everywhere a test previously called the removed FullAccessRegistry()
// panic-wrapper so that registry build failures surface as clean test failures.
func mustRegistry(t *testing.T, svc *Service) []mcp.ToolDescriptor {
	t.Helper()
	reg, err := svc.FullAccessRegistryChecked()
	if err != nil {
		t.Fatalf("FullAccessRegistryChecked: %v", err)
	}
	return reg
}

// domainGroupBuilders maps family names onto the handler builders
// they grouped. The product binary now ships every tool in
// FullAccessRegistry at startup, but tests still find it convenient to
// fetch descriptors by family.
var domainGroupBuilders = map[string]func(*Service) []mcp.ToolDescriptor{
	"invoices":        invoiceHandlers,
	"expenses":        expenseHandlers,
	"custom_fields":   customFieldHandlers,
	"time_off":        timeOffHandlers,
	"scheduling":      schedulingHandlers,
	"approvals":       approvalHandlers,
	"webhooks":        webhookHandlers,
	"groups_holidays": groupsHolidaysHandlers,
	"project_admin":   projectAdminHandlers,
	"user_admin":      userAdminHandlers,
}

func domainHandlers(svc *Service, name string) ([]mcp.ToolDescriptor, bool) {
	build, ok := domainGroupBuilders[name]
	if !ok {
		return nil, false
	}
	return applyOpaqueOutputSchemas(normalizeDescriptors(build(svc))), true
}

func applyOpaqueOutputSchemas(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	for i := range in {
		if in[i].Tool.OutputSchema == nil {
			in[i].Tool.OutputSchema = envelopeOpaque(in[i].Tool.Name)
		}
	}
	return in
}

func envelopeOpaque(action string) map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"ok", "action"},
		"properties": map[string]any{
			"ok":     map[string]any{"type": "boolean"},
			"action": map[string]any{"type": "string", "const": action},
		},
	}
}
