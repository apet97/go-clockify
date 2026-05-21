package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/mcp"
)

const (
	productContractFullRegistryTools      = 156
	productContractDefaultAdvertisedTools = 16
	productContractWorkflowTools          = 17
	productContractDomainTools            = 137
	productContractRawTools               = 2
)

// TestProductContractDoesNotRegress pins the binding product invariants from
// AGENTS.md so an accidental tool add/remove/rename or a missing schema is
// caught immediately.
func TestProductContractDoesNotRegress(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	reg := svc.FullAccessRegistry()

	if len(reg) != productContractFullRegistryTools {
		t.Fatalf("registry has %d tools, want exactly %d", len(reg), productContractFullRegistryTools)
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
	if workflow != productContractWorkflowTools {
		t.Errorf("workflow tool count = %d, want %d", workflow, productContractWorkflowTools)
	}
	if domain != productContractDomainTools {
		t.Errorf("domain tool count = %d, want %d", domain, productContractDomainTools)
	}
	if raw != productContractRawTools {
		t.Errorf("raw fallback tool count = %d, want %d", raw, productContractRawTools)
	}

	for i := 0; i < productContractWorkflowTools && i < len(reg); i++ {
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

// TestDefaultToolsetEnvAdvertises16WhileRegistryLoads156 proves the binding
// invariant from AGENTS.md end-to-end: with no CLOCKIFY_TOOLSET in the
// environment, the startup registry remains the full 156-tool surface while
// RegistryForToolset(cfg.Toolset) advertises the narrower default surface.
// Operator toolsets narrow or widen only the advertised surface and never the
// loaded registry.
func TestDefaultToolsetEnvAdvertises16WhileRegistryLoads156(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "test-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", "000000000000000000000001")
	t.Setenv("CLOCKIFY_TOOLSET", "")

	cfg, err := config.LoadOneUser()
	if err != nil {
		t.Fatalf("LoadOneUser with default env: %v", err)
	}
	if cfg.Toolset != "default" {
		t.Fatalf("default CLOCKIFY_TOOLSET resolved to %q, want \"default\"", cfg.Toolset)
	}

	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), cfg.WorkspaceID)
	if got := len(svc.FullAccessRegistry()); got != productContractFullRegistryTools {
		t.Fatalf("full registry loaded %d tools, want exactly %d", got, productContractFullRegistryTools)
	}
	if got := len(svc.RegistryForToolset(cfg.Toolset)); got != productContractDefaultAdvertisedTools {
		t.Fatalf("default toolset advertised %d tools, want exactly %d", got, productContractDefaultAdvertisedTools)
	}
	if got := len(svc.RegistryForToolset("all")); got != productContractFullRegistryTools {
		t.Fatalf("CLOCKIFY_TOOLSET=all advertised %d tools, want exactly %d", got, productContractFullRegistryTools)
	}

	for _, toolset := range []string{"core", "business", "admin"} {
		got := len(svc.RegistryForToolset(toolset))
		if got <= 0 || got >= productContractFullRegistryTools {
			t.Errorf("CLOCKIFY_TOOLSET=%s loaded %d tools, want a non-empty documented subset below %d", toolset, got, productContractFullRegistryTools)
		}
	}
}
