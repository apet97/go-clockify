package tools

import (
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

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
