package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// requiredSet extracts a tool input schema's `required` array as a set,
// tolerating both []string (in-memory descriptors) and []any (post-JSON).
func requiredSet(schema map[string]any) map[string]bool {
	out := map[string]bool{}
	switch req := schema["required"].(type) {
	case []string:
		for _, r := range req {
			out[r] = true
		}
	case []any:
		for _, r := range req {
			if s, ok := r.(string); ok {
				out[s] = true
			}
		}
	}
	return out
}

func registryToolsByName(t *testing.T) map[string]mcp.Tool {
	t.Helper()
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	tools := map[string]mcp.Tool{}
	for _, d := range svc.FullAccessRegistry() {
		tools[d.Tool.Name] = d.Tool
	}
	return tools
}

func TestToolRequiredArraysMatchHandlerContracts(t *testing.T) {
	tools := registryToolsByName(t)
	// Each tool's handler rejects these args when absent; the schema
	// `required` array must advertise the same contract.
	cases := map[string][]string{
		"clockify_schedule_work":       {"start", "end", "hours_per_day", "user", "project"},
		"clockify_setup_webhook":       {"name", "url"},
		"clockify_invoice_client_work": {"currency", "client"},
		"clockify_tasks_create":        {"name", "project"},
		"clockify_tasks_list":          {"project"},
		"clockify_tasks_rates_update":  {"rate_kind", "amount", "project", "task"},
		"clockify_time_off_archive":    {"policy_id", "archived"},
	}
	for name, want := range cases {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("tool %q not in registry", name)
		}
		got := requiredSet(tool.InputSchema)
		for _, field := range want {
			if !got[field] {
				t.Errorf("%s: schema `required` is missing %q (have %v)", name, field, got)
			}
		}
	}
}

func TestRequiredNameOrIDAliasesAreAgentVisible(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	for _, descriptor := range svc.FullAccessRegistry() {
		required := requiredSet(descriptor.Tool.InputSchema)
		props, _ := descriptor.Tool.InputSchema["properties"].(map[string]any)
		for base := range required {
			if strings.HasSuffix(base, "_id") {
				continue
			}
			alias := base + "_id"
			if _, ok := props[alias]; !ok {
				continue
			}
			baseDesc := strings.ToLower(schemaPropertyDescription(props[base]))
			aliasDesc := strings.ToLower(schemaPropertyDescription(props[alias]))
			if !strings.Contains(baseDesc, "id") {
				t.Errorf("%s.%s is required but has %s alias; base description should mention ID input, got %q",
					descriptor.Tool.Name, base, alias, schemaPropertyDescription(props[base]))
			}
			if !strings.Contains(aliasDesc, "alias") && !strings.Contains(aliasDesc, "accepted in place") {
				t.Errorf("%s.%s description should tell agents it is accepted in place of %s, got %q",
					descriptor.Tool.Name, alias, base, schemaPropertyDescription(props[alias]))
			}
		}
	}
}

func schemaPropertyDescription(prop any) string {
	m, _ := prop.(map[string]any)
	desc, _ := m["description"].(string)
	return desc
}

func TestSchedulingCapacityUserIDsOptional(t *testing.T) {
	tools := registryToolsByName(t)
	tool, ok := tools["clockify_scheduling_capacity"]
	if !ok {
		t.Fatal("clockify_scheduling_capacity not in registry")
	}
	if requiredSet(tool.InputSchema)["user_ids"] {
		t.Error("clockify_scheduling_capacity must not require user_ids")
	}
}

func TestHighRiskToolsAcceptConfirmTokenWithoutOpeningSchema(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	for _, d := range svc.FullAccessRegistry() {
		hasConfirmToken := schemaHasProperty(d.Tool.InputSchema, "confirm_token")
		if d.RiskClass.IsHighRisk() {
			if !hasConfirmToken {
				t.Fatalf("%s is high-risk but lacks confirm_token", d.Tool.Name)
			}
			if got := d.Tool.InputSchema["additionalProperties"]; got != false {
				t.Fatalf("%s additionalProperties = %v, want false", d.Tool.Name, got)
			}
			continue
		}
		if d.RiskClass.Has(mcp.RiskRead) && hasConfirmToken {
			t.Fatalf("%s is read-only but unexpectedly accepts confirm_token", d.Tool.Name)
		}
	}
}

func TestHighRiskToolsSupportDryRunForConfirmationPreview(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	for _, d := range svc.FullAccessRegistry() {
		if !d.RiskClass.IsHighRisk() || strings.TrimSpace(d.SafetyExemption) != "" {
			continue
		}
		if !schemaHasProperty(d.Tool.InputSchema, "dry_run") {
			t.Fatalf("%s is high-risk but lacks dry_run for confirmation preview", d.Tool.Name)
		}
	}
}

func TestBusinessWorkflowSchemasExposeDryRunAndWebhookConstraints(t *testing.T) {
	tools := registryToolsByName(t)
	for _, name := range []string{
		"clockify_invoice_client_work",
		"clockify_record_expense",
		"clockify_request_time_off",
		"clockify_schedule_work",
		"clockify_setup_webhook",
	} {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("tool %q not in registry", name)
		}
		if !schemaHasProperty(tool.InputSchema, "dry_run") {
			t.Fatalf("%s must expose dry_run for business-impact preview", name)
		}
	}

	webhook := tools["clockify_setup_webhook"]
	props := webhook.InputSchema["properties"].(map[string]any)
	urlProp := props["url"].(map[string]any)
	if urlProp["format"] != "uri" {
		t.Fatalf("clockify_setup_webhook.url format = %v, want uri", urlProp["format"])
	}
	eventProp := props["webhook_event"].(map[string]any)
	enum, ok := eventProp["enum"].([]string)
	if !ok || len(enum) == 0 {
		t.Fatalf("clockify_setup_webhook.webhook_event must expose the webhook event enum, got %#v", eventProp["enum"])
	}
}

func TestBusinessWorkflowRiskClassesMatchSideEffects(t *testing.T) {
	registry := riskTestRegistry(t)
	cases := map[string]mcp.RiskClass{
		"clockify_invoice_client_work": mcp.RiskWrite | mcp.RiskBilling,
		"clockify_record_expense":      mcp.RiskWrite | mcp.RiskBilling,
		"clockify_request_time_off":    mcp.RiskWrite | mcp.RiskAdmin,
		"clockify_schedule_work":       mcp.RiskWrite | mcp.RiskAdmin,
		"clockify_setup_webhook":       mcp.RiskWrite | mcp.RiskExternalSideEffect,
	}
	for name, want := range cases {
		assertToolRiskClass(t, registry, name, want)
	}
}

func TestEveryToolParameterHasADescription(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	for _, d := range svc.FullAccessRegistry() {
		schema := d.Tool.InputSchema
		if schema == nil {
			continue
		}
		props, ok := schema["properties"].(map[string]any)
		if !ok {
			continue
		}
		for name, raw := range props {
			prop, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			desc, _ := prop["description"].(string)
			if strings.TrimSpace(desc) == "" {
				t.Errorf("%s: parameter %q has no description", d.Tool.Name, name)
			}
		}
	}
}
