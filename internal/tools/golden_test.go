package tools

import (
	"sort"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// ---------------------------------------------------------------------------
// 1. Golden Tier 1 tool list — exact count and names
// ---------------------------------------------------------------------------

func TestGoldenTier1ToolList(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	reg := svc.Registry()

	names := make([]string, len(reg))
	for i, d := range reg {
		names[i] = d.Tool.Name
	}
	sort.Strings(names)

	expected := []string{
		"clockify_add_entry",
		"clockify_create_client",
		"clockify_create_project",
		"clockify_create_tag",
		"clockify_create_task",
		"clockify_current_user",
		"clockify_delete_entry",
		"clockify_detailed_report",
		"clockify_find_and_update_entry",
		"clockify_get_entry",
		"clockify_get_project",
		"clockify_get_workspace",
		"clockify_list_clients",
		"clockify_list_entries",
		"clockify_list_projects",
		"clockify_list_tags",
		"clockify_list_tasks",
		"clockify_list_users",
		"clockify_list_workspaces",
		"clockify_log_time",
		"clockify_policy_info",
		"clockify_quick_report",
		"clockify_resolve_debug",
		"clockify_search_tools",
		"clockify_start_timer",
		"clockify_stop_timer",
		"clockify_summary_report",
		"clockify_switch_project",
		"clockify_timer_status",
		"clockify_today_entries",
		"clockify_update_entry",
		"clockify_weekly_summary",
		"clockify_whoami",
	}

	if len(names) != len(expected) {
		t.Fatalf("expected %d tools, got %d: %v", len(expected), len(names), names)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("tool %d: expected %s, got %s", i, expected[i], names[i])
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Golden Tier 2 group catalog — exact groups and total tool count
// ---------------------------------------------------------------------------

func TestGoldenTier2GroupCatalog(t *testing.T) {
	expectedGroups := []string{
		"approvals", "custom_fields", "expenses", "groups_holidays",
		"invoices", "project_admin", "scheduling", "shared_reports",
		"time_off", "user_admin", "webhooks",
	}

	for _, name := range expectedGroups {
		if _, ok := Tier2Groups[name]; !ok {
			t.Fatalf("missing Tier 2 group: %s", name)
		}
	}

	if len(Tier2Groups) != len(expectedGroups) {
		t.Fatalf("expected %d Tier 2 groups, got %d", len(expectedGroups), len(Tier2Groups))
	}
}

func TestTier2TotalToolCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	total := 0
	for name, group := range Tier2Groups {
		handlers := group.Builder(svc)
		total += len(handlers)
		t.Logf("group %s: %d tools", name, len(handlers))
	}
	if total != 91 {
		t.Fatalf("expected 91 Tier 2 tools, got %d", total)
	}
}

func TestTier2PerGroupToolCounts(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	expectedCounts := map[string]int{
		"invoices":        12,
		"approvals":       6,
		"expenses":        10,
		"custom_fields":   6,
		"scheduling":      10,
		"user_admin":      8,
		"webhooks":        7,
		"shared_reports":  6,
		"time_off":        12,
		"project_admin":   6,
		"groups_holidays": 8,
	}
	for name, expected := range expectedCounts {
		group, ok := Tier2Groups[name]
		if !ok {
			t.Fatalf("missing group %s", name)
		}
		handlers := group.Builder(svc)
		if len(handlers) != expected {
			t.Errorf("group %s: expected %d tools, got %d", name, expected, len(handlers))
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Schema validation — every tool has a valid InputSchema
// ---------------------------------------------------------------------------

func TestAllToolsHaveValidSchema(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")

	// Check Tier 1
	for _, d := range svc.Registry() {
		if d.Tool.InputSchema == nil {
			t.Fatalf("tool %s has nil InputSchema", d.Tool.Name)
		}
		typ, ok := d.Tool.InputSchema["type"].(string)
		if !ok || typ != "object" {
			t.Fatalf("tool %s InputSchema type should be 'object', got %v", d.Tool.Name, d.Tool.InputSchema["type"])
		}
	}

	// Check Tier 2
	for name, group := range Tier2Groups {
		handlers := group.Builder(svc)
		for _, d := range handlers {
			if d.Tool.InputSchema == nil {
				t.Fatalf("tier2 group %s: tool %s has nil InputSchema", name, d.Tool.Name)
			}
			typ, ok := d.Tool.InputSchema["type"].(string)
			if !ok || typ != "object" {
				t.Fatalf("tier2 group %s: tool %s InputSchema type should be 'object', got %v", name, d.Tool.Name, d.Tool.InputSchema["type"])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Annotation consistency — readOnlyHint and destructiveHint
// ---------------------------------------------------------------------------

func TestAnnotationConsistency(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")

	checkDescriptor := func(label string, d mcp.ToolDescriptor) {
		t.Helper()
		ann := d.Tool.Annotations
		if ann == nil {
			t.Fatalf("%s: tool %s has nil Annotations", label, d.Tool.Name)
		}
		roHint, ok := ann["readOnlyHint"].(bool)
		if !ok {
			t.Fatalf("%s: tool %s missing readOnlyHint annotation", label, d.Tool.Name)
		}
		// ReadOnlyHint on the descriptor must match the annotation
		if d.ReadOnlyHint != roHint {
			t.Fatalf("%s: tool %s descriptor ReadOnlyHint (%v) != annotation readOnlyHint (%v)",
				label, d.Tool.Name, d.ReadOnlyHint, roHint)
		}
		// Destructive tools must have destructiveHint annotation
		if d.DestructiveHint {
			dh, ok := ann["destructiveHint"].(bool)
			if !ok || !dh {
				t.Fatalf("%s: tool %s is marked DestructiveHint but missing destructiveHint annotation",
					label, d.Tool.Name)
			}
		}
		// IdempotentHint: the descriptor flag and annotation must agree.
		idemAnn, hasIdemAnn := ann["idempotentHint"].(bool)
		if d.IdempotentHint && (!hasIdemAnn || !idemAnn) {
			t.Fatalf("%s: tool %s is marked IdempotentHint but missing idempotentHint annotation",
				label, d.Tool.Name)
		}
		if hasIdemAnn && idemAnn && !d.IdempotentHint {
			t.Fatalf("%s: tool %s has idempotentHint annotation but descriptor IdempotentHint is false",
				label, d.Tool.Name)
		}
		// All read-only tools must carry IdempotentHint — reads are inherently
		// idempotent and clients rely on this signal.
		if d.ReadOnlyHint && !d.IdempotentHint {
			t.Fatalf("%s: read-only tool %s missing IdempotentHint", label, d.Tool.Name)
		}
	}

	// Check Tier 1
	for _, d := range svc.Registry() {
		checkDescriptor("tier1", d)
	}

	// Check Tier 2
	for name, group := range Tier2Groups {
		handlers := group.Builder(svc)
		for _, d := range handlers {
			checkDescriptor("tier2/"+name, d)
		}
	}
}

// ---------------------------------------------------------------------------
// 5. Every tool has a non-nil handler
// ---------------------------------------------------------------------------

func TestAllToolsHaveHandlers(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")

	// Tier 1
	for _, d := range svc.Registry() {
		if d.Handler == nil {
			t.Fatalf("tier1 tool %s has nil Handler", d.Tool.Name)
		}
	}

	// Tier 2
	for name, group := range Tier2Groups {
		for _, d := range group.Builder(svc) {
			if d.Handler == nil {
				t.Fatalf("tier2/%s tool %s has nil Handler", name, d.Tool.Name)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 6. No duplicate tool names across all tiers
// ---------------------------------------------------------------------------

func TestNoDuplicateToolNames(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	seen := map[string]string{} // name -> source

	for _, d := range svc.Registry() {
		if source, exists := seen[d.Tool.Name]; exists {
			t.Fatalf("duplicate tool name %s: first in %s, also in tier1", d.Tool.Name, source)
		}
		seen[d.Tool.Name] = "tier1"
	}

	for name, group := range Tier2Groups {
		for _, d := range group.Builder(svc) {
			if source, exists := seen[d.Tool.Name]; exists {
				t.Fatalf("duplicate tool name %s: first in %s, also in tier2/%s", d.Tool.Name, source, name)
			}
			seen[d.Tool.Name] = "tier2/" + name
		}
	}
}

// ---------------------------------------------------------------------------
// 7. All tools have non-empty name and description
// ---------------------------------------------------------------------------

func TestAllToolsHaveNameAndDescription(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")

	check := func(label string, d mcp.ToolDescriptor) {
		t.Helper()
		if d.Tool.Name == "" {
			t.Fatalf("%s: tool has empty name", label)
		}
		if d.Tool.Description == "" {
			t.Fatalf("%s: tool %s has empty description", label, d.Tool.Name)
		}
	}

	for _, d := range svc.Registry() {
		check("tier1", d)
	}
	for name, group := range Tier2Groups {
		for _, d := range group.Builder(svc) {
			check("tier2/"+name, d)
		}
	}
}

// ---------------------------------------------------------------------------
// 8. Tier 2 groups have metadata
// ---------------------------------------------------------------------------

func TestTier2GroupsHaveMetadata(t *testing.T) {
	for name, group := range Tier2Groups {
		if group.Name != name {
			t.Fatalf("group key %q != group.Name %q", name, group.Name)
		}
		if group.Description == "" {
			t.Fatalf("group %s has empty description", name)
		}
		if len(group.Keywords) == 0 {
			t.Fatalf("group %s has no keywords", name)
		}
		if group.Builder == nil {
			t.Fatalf("group %s has nil builder", name)
		}
	}
}

// ---------------------------------------------------------------------------
// 9. Tier 1 catalog golden count
// ---------------------------------------------------------------------------

func TestTier1CatalogGoldenCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	reg := svc.Registry()
	if len(reg) != 33 {
		t.Fatalf("expected 33 Tier 1 tools, got %d", len(reg))
	}
}

// ---------------------------------------------------------------------------
// 10. Total tool count (Tier 1 + Tier 2 = 124)
// ---------------------------------------------------------------------------

func TestTotalToolCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	tier1 := len(svc.Registry())
	tier2 := 0
	for _, group := range Tier2Groups {
		tier2 += len(group.Builder(svc))
	}
	total := tier1 + tier2
	if total != 124 {
		t.Fatalf("expected 124 total tools (33 Tier1 + 91 Tier2), got %d (%d + %d)", total, tier1, tier2)
	}
}
