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
		"clockify_setup_webhook":       {"name", "url", "webhook_event"},
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
