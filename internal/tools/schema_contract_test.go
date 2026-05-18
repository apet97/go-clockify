package tools

import (
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
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
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
