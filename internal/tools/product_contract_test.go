package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// TestProductContractDoesNotRegress pins the binding product invariants from
// AGENTS.md so an accidental tool add/remove/rename or a missing schema is
// caught immediately.
func TestProductContractDoesNotRegress(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	reg := svc.FullAccessRegistry()

	if len(reg) != 156 {
		t.Fatalf("registry has %d tools, want exactly 156", len(reg))
	}

	var workflow, domain, raw int
	for _, d := range reg {
		switch d.Tool.Annotations["category"] {
		case "workflow":
			workflow++
		case "raw":
			raw++
		case "domain":
			domain++
		default:
			t.Errorf("tool %q has unexpected category %v", d.Tool.Name, d.Tool.Annotations["category"])
		}
	}
	if workflow != 17 {
		t.Errorf("workflow tool count = %d, want 17", workflow)
	}
	if domain != 137 {
		t.Errorf("domain tool count = %d, want 137", domain)
	}
	if raw != 2 {
		t.Errorf("raw fallback tool count = %d, want 2", raw)
	}

	for i := 0; i < 17 && i < len(reg); i++ {
		if reg[i].Tool.Annotations["category"] != "workflow" {
			t.Errorf("tool %d (%q) should be a workflow tool (workflow-first ordering)", i, reg[i].Tool.Name)
		}
	}
	if reg[len(reg)-2].Tool.Name != "clockify_api_get" || reg[len(reg)-1].Tool.Name != "clockify_api_request" {
		t.Errorf("raw fallback tools must be last, got %q and %q",
			reg[len(reg)-2].Tool.Name, reg[len(reg)-1].Tool.Name)
	}

	for _, d := range reg {
		if !strings.HasPrefix(d.Tool.Name, "clockify_") {
			t.Errorf("tool %q does not use the clockify_ prefix (legacy name regression?)", d.Tool.Name)
		}
		if d.Tool.Title == "" {
			t.Errorf("tool %q has no title", d.Tool.Name)
		}
		if d.Tool.InputSchema == nil {
			t.Errorf("tool %q has no input schema", d.Tool.Name)
		}
		if d.Tool.OutputSchema == nil {
			t.Errorf("tool %q has no output schema", d.Tool.Name)
		}
		if d.RiskClass == 0 {
			t.Errorf("tool %q has no risk class", d.Tool.Name)
		}
		if d.DestructiveHint && !d.RiskClass.Has(mcp.RiskDestructive) {
			t.Errorf("destructive tool %q is missing the RiskDestructive bit", d.Tool.Name)
		}
	}

	// The registry tool-name set must match the committed catalog exactly.
	catalogNames := catalogToolNames(t) // helper from docs_tool_names_lint_test.go
	if len(catalogNames) != len(reg) {
		t.Errorf("catalog lists %d tool names, registry has %d", len(catalogNames), len(reg))
	}
	for _, d := range reg {
		if !catalogNames[d.Tool.Name] {
			t.Errorf("registry tool %q is missing from docs/tool-catalog.json (run make gen-tool-catalog)", d.Tool.Name)
		}
	}
}
