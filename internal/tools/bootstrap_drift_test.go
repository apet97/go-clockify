package tools

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/clockify"
)

// TestBootstrapLists_NoDrift locks down the D1 contract: every tool
// name hardcoded in bootstrap.{AlwaysVisible,MinimalSet,Tier1Catalog}
// must resolve to a real registered tool. Before this check, renaming
// or removing a tool in the registry left the bootstrap lists silently
// stale — operators ran a Minimal mode referencing tools that no
// longer existed, and the server quietly ignored the dead entries.
//
// The test does NOT assert tier membership (Tier 1 vs Tier 2) — that
// is handled by TestToolContractMatrix's tier-aware descriptor walk.
// The purpose here is pure drift detection: names ↔ registry.
func TestBootstrapLists_NoDrift(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	registered := map[string]struct{}{}
	for _, d := range svc.Registry() {
		registered[d.Tool.Name] = struct{}{}
	}
	// Tier 2 groups are lazy-activated by name; exercise every group so
	// the drift check sees the full registered surface.
	for group := range Tier2Groups {
		descriptors, ok := svc.Tier2Handlers(group)
		if !ok {
			t.Fatalf("missing tier2 handlers for group %q", group)
		}
		for _, d := range descriptors {
			registered[d.Tool.Name] = struct{}{}
		}
	}

	check := func(source string, names []string) {
		t.Helper()
		for _, name := range names {
			if _, ok := registered[name]; !ok {
				t.Errorf("%s references %q which is not a registered tool (drift: remove or rename the list entry, or re-register the tool)",
					source, name)
			}
		}
	}

	alwaysVisible := keysOf(bootstrap.AlwaysVisible)
	minimalSet := keysOf(bootstrap.MinimalSet)
	catalogNames := make([]string, 0, len(bootstrap.Tier1Catalog))
	for _, e := range bootstrap.Tier1Catalog {
		catalogNames = append(catalogNames, e.Name)
	}

	check("bootstrap.AlwaysVisible", alwaysVisible)
	check("bootstrap.MinimalSet", minimalSet)
	check("bootstrap.Tier1Catalog", catalogNames)
}

func TestPolicyInfoReportsBootstrapVisibility(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	cfg := bootstrap.Config{Mode: bootstrap.Minimal}
	tier1 := tier1NamesForService(svc)
	cfg.SetTier1Tools(tier1)
	svc.Bootstrap = &cfg
	svc.PolicyDescribe = func() map[string]any {
		return map[string]any{"mode": "standard"}
	}

	result, err := svc.PolicyInfo(context.Background())
	if err != nil {
		t.Fatalf("policy info failed: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["bootstrap_mode"] != "minimal" {
		t.Fatalf("bootstrap_mode = %v, want minimal", data["bootstrap_mode"])
	}
	visible := cfg.VisibleCount()
	if data["tier1_visible_count"] != visible {
		t.Fatalf("tier1_visible_count = %v, want %d", data["tier1_visible_count"], visible)
	}
	if data["tier1_hidden_count"] != len(tier1)-visible {
		t.Fatalf("tier1_hidden_count = %v, want %d", data["tier1_hidden_count"], len(tier1)-visible)
	}
	if data["mode"] != "standard" {
		t.Fatalf("policy data was not preserved: %+v", data)
	}
}

func TestSearchToolsMarksBootstrapHiddenTier1(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	cfg := bootstrap.Config{Mode: bootstrap.Minimal}
	cfg.SetTier1Tools(tier1NamesForService(svc))
	svc.Bootstrap = &cfg

	result, err := svc.SearchTools(context.Background(), map[string]any{"query": "workspace"})
	if err != nil {
		t.Fatalf("search tools failed: %v", err)
	}
	data := result.Data.(map[string]any)
	allResults := data["all_results"].([]map[string]any)
	for _, entry := range allResults {
		if entry["name"] == "clockify_list_workspaces" {
			if entry["availability"] != "tier1_hidden_by_bootstrap" {
				t.Fatalf("availability = %v, want tier1_hidden_by_bootstrap in %+v", entry["availability"], entry)
			}
			return
		}
	}
	t.Fatal("expected clockify_list_workspaces in workspace search results")
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func tier1NamesForService(svc *Service) map[string]bool {
	out := map[string]bool{}
	for _, d := range svc.Registry() {
		out[d.Tool.Name] = true
	}
	return out
}
