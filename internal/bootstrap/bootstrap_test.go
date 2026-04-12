package bootstrap

import (
	"os"
	"testing"
)

// helper to build a Tier1Names map from the catalog.
func allTier1Names() map[string]bool {
	names := make(map[string]bool, len(Tier1Catalog))
	for _, e := range Tier1Catalog {
		names[e.Name] = true
	}
	return names
}

func clearEnv(t *testing.T) {
	t.Helper()
	os.Unsetenv("CLOCKIFY_BOOTSTRAP_MODE")
	os.Unsetenv("CLOCKIFY_BOOTSTRAP_TOOLS")
}

func TestConfigFromEnvDefault(t *testing.T) {
	clearEnv(t)
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode != FullTier1 {
		t.Errorf("expected FullTier1, got %v", cfg.Mode)
	}
}

func TestConfigFromEnvMinimal(t *testing.T) {
	clearEnv(t)
	os.Setenv("CLOCKIFY_BOOTSTRAP_MODE", "minimal")
	defer clearEnv(t)

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode != Minimal {
		t.Errorf("expected Minimal, got %v", cfg.Mode)
	}
}

func TestConfigFromEnvCustom(t *testing.T) {
	clearEnv(t)
	os.Setenv("CLOCKIFY_BOOTSTRAP_MODE", "custom")
	os.Setenv("CLOCKIFY_BOOTSTRAP_TOOLS", "clockify_start_timer,clockify_stop_timer")
	defer clearEnv(t)

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode != Custom {
		t.Errorf("expected Custom, got %v", cfg.Mode)
	}
	if !cfg.CustomTools["clockify_start_timer"] {
		t.Error("expected clockify_start_timer in custom tools")
	}
	if !cfg.CustomTools["clockify_stop_timer"] {
		t.Error("expected clockify_stop_timer in custom tools")
	}
	if len(cfg.CustomTools) != 2 {
		t.Errorf("expected 2 custom tools, got %d", len(cfg.CustomTools))
	}
}

func TestConfigFromEnvCustomNoTools(t *testing.T) {
	clearEnv(t)
	os.Setenv("CLOCKIFY_BOOTSTRAP_MODE", "custom")
	os.Unsetenv("CLOCKIFY_BOOTSTRAP_TOOLS")
	defer clearEnv(t)

	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for custom mode without tools")
	}
}

func TestConfigFromEnvInvalid(t *testing.T) {
	clearEnv(t)
	os.Setenv("CLOCKIFY_BOOTSTRAP_MODE", "turbo")
	defer clearEnv(t)

	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestAlwaysVisible(t *testing.T) {
	alwaysNames := []string{
		"clockify_whoami",
		"clockify_search_tools",
		"clockify_policy_info",
		"clockify_resolve_debug",
	}

	modes := []Mode{FullTier1, Minimal, Custom}
	for _, mode := range modes {
		cfg := Config{
			Mode:        mode,
			CustomTools: map[string]bool{},
			Tier1Names:  map[string]bool{},
		}
		for _, name := range alwaysNames {
			if !cfg.IsVisible(name) {
				t.Errorf("expected %s to be always visible in mode %v", name, mode)
			}
		}
	}
}

func TestIsVisibleFullTier1(t *testing.T) {
	cfg := Config{Mode: FullTier1}
	cfg.SetTier1Tools(allTier1Names())

	for _, entry := range Tier1Catalog {
		if !cfg.IsVisible(entry.Name) {
			t.Errorf("expected %s to be visible in FullTier1 mode", entry.Name)
		}
	}
	// A tool not in Tier 1 should not be visible.
	if cfg.IsVisible("clockify_nonexistent_tool") {
		t.Error("unexpected visibility for non-Tier1 tool")
	}
}

func TestIsVisibleMinimal(t *testing.T) {
	cfg := Config{Mode: Minimal}
	cfg.SetTier1Tools(allTier1Names())

	for name := range MinimalSet {
		if !cfg.IsVisible(name) {
			t.Errorf("expected %s to be visible in Minimal mode", name)
		}
	}
	// A Tier 1 tool NOT in MinimalSet should be invisible (unless always-visible).
	if cfg.IsVisible("clockify_create_project") {
		t.Error("clockify_create_project should not be visible in Minimal mode")
	}
}

func TestIsVisibleCustom(t *testing.T) {
	cfg := Config{
		Mode: Custom,
		CustomTools: map[string]bool{
			"clockify_start_timer": true,
			"clockify_stop_timer":  true,
		},
	}
	cfg.SetTier1Tools(allTier1Names())

	if !cfg.IsVisible("clockify_start_timer") {
		t.Error("expected clockify_start_timer visible in Custom mode")
	}
	if !cfg.IsVisible("clockify_stop_timer") {
		t.Error("expected clockify_stop_timer visible in Custom mode")
	}
	// Not in custom list and not always-visible.
	if cfg.IsVisible("clockify_create_project") {
		t.Error("clockify_create_project should not be visible in Custom mode")
	}
}

func TestSearchCatalogByName(t *testing.T) {
	results := SearchCatalog("timer")
	if len(results) == 0 {
		t.Fatal("expected results for 'timer' query")
	}
	found := false
	for _, r := range results {
		if r.Name == "clockify_start_timer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected clockify_start_timer in results")
	}
}

func TestSearchCatalogByDomain(t *testing.T) {
	results := SearchCatalog("entries")
	if len(results) == 0 {
		t.Fatal("expected results for 'entries' domain query")
	}
	// All entries-domain tools should appear.
	for _, r := range results {
		if r.Domain == "entries" {
			return // at least one found
		}
	}
	t.Error("expected at least one entry with domain 'entries'")
}

func TestSearchCatalogByKeyword(t *testing.T) {
	results := SearchCatalog("create")
	if len(results) == 0 {
		t.Fatal("expected results for 'create' keyword query")
	}
	foundNames := make(map[string]bool)
	for _, r := range results {
		foundNames[r.Name] = true
	}
	expected := []string{
		"clockify_create_project",
		"clockify_create_client",
		"clockify_create_tag",
		"clockify_create_task",
	}
	for _, name := range expected {
		if !foundNames[name] {
			t.Errorf("expected %s in 'create' search results", name)
		}
	}
}

func TestSearchCatalogEmpty(t *testing.T) {
	results := SearchCatalog("")
	if len(results) != 33 {
		t.Errorf("expected 33 catalog entries for empty query, got %d", len(results))
	}
}

func TestVisibleCount(t *testing.T) {
	tier1 := allTier1Names()

	// FullTier1: all 33 visible.
	cfg := Config{Mode: FullTier1}
	cfg.SetTier1Tools(tier1)
	if got := cfg.VisibleCount(); got != 33 {
		t.Errorf("FullTier1 VisibleCount: expected 33, got %d", got)
	}

	// Minimal: only MinimalSet tools that are also in Tier1Names.
	cfg = Config{Mode: Minimal}
	cfg.SetTier1Tools(tier1)
	if got := cfg.VisibleCount(); got != len(MinimalSet) {
		t.Errorf("Minimal VisibleCount: expected %d, got %d", len(MinimalSet), got)
	}

	// Custom: only custom tools (plus always-visible ones that are in Tier1Names).
	cfg = Config{
		Mode: Custom,
		CustomTools: map[string]bool{
			"clockify_start_timer": true,
			"clockify_stop_timer":  true,
		},
	}
	cfg.SetTier1Tools(tier1)
	// Custom tools (2) + AlwaysVisible (4) = 6, but always-visible are not in
	// custom tools. VisibleCount iterates Tier1Names and checks IsVisible, so
	// the 4 always-visible + 2 custom = 6.
	if got := cfg.VisibleCount(); got != 6 {
		t.Errorf("Custom VisibleCount: expected 6, got %d", got)
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{FullTier1, "full_tier1"},
		{Minimal, "minimal"},
		{Custom, "custom"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Mode(%d).String() = %q, want %q", int(tt.mode), got, tt.want)
		}
	}
}
