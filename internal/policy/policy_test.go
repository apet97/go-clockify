package policy

import (
	"os"
	"testing"
)

func TestIsAllowed(t *testing.T) {
	p := &Policy{Mode: ReadOnly, DeniedTools: map[string]bool{}}
	if !p.IsAllowed("clockify_list_projects", true) {
		t.Fatal("read-only tool should be allowed in read_only mode")
	}
	if p.IsAllowed("clockify_start_timer", false) {
		t.Fatal("write tool should be blocked in read_only mode")
	}

	p = &Policy{Mode: SafeCore, DeniedTools: map[string]bool{}}
	if !p.IsAllowed("clockify_start_timer", false) {
		t.Fatal("safe core write should be allowed")
	}
	if p.IsAllowed("clockify_delete_entry", false) {
		t.Fatal("unsafe write should not be allowed")
	}
}

func TestIsGroupAllowed(t *testing.T) {
	// Standard mode allows groups by default
	p := &Policy{Mode: Standard, DeniedTools: map[string]bool{}, DeniedGroups: map[string]bool{}}
	if !p.IsGroupAllowed("reports") {
		t.Fatal("standard mode should allow groups")
	}

	// ReadOnly blocks all groups
	p = &Policy{Mode: ReadOnly, DeniedTools: map[string]bool{}, DeniedGroups: map[string]bool{}}
	if p.IsGroupAllowed("reports") {
		t.Fatal("read_only mode should block all groups")
	}

	// SafeCore blocks all groups
	p = &Policy{Mode: SafeCore, DeniedTools: map[string]bool{}, DeniedGroups: map[string]bool{}}
	if p.IsGroupAllowed("reports") {
		t.Fatal("safe_core mode should block all groups")
	}

	// Deny override in standard mode
	p = &Policy{Mode: Standard, DeniedTools: map[string]bool{}, DeniedGroups: map[string]bool{"billing": true}}
	if p.IsGroupAllowed("billing") {
		t.Fatal("denied group should be blocked in standard mode")
	}
	if !p.IsGroupAllowed("reports") {
		t.Fatal("non-denied group should be allowed in standard mode")
	}

	// nil policy allows everything
	var nilP *Policy
	if !nilP.IsGroupAllowed("anything") {
		t.Fatal("nil policy should allow all groups")
	}
}

func TestAllowGroupsWhitelist(t *testing.T) {
	p := &Policy{
		Mode:          Standard,
		DeniedTools:   map[string]bool{},
		DeniedGroups:  map[string]bool{},
		AllowedGroups: map[string]bool{"time_tracking": true, "projects": true},
	}

	if !p.IsGroupAllowed("time_tracking") {
		t.Fatal("whitelisted group should be allowed")
	}
	if !p.IsGroupAllowed("projects") {
		t.Fatal("whitelisted group should be allowed")
	}
	if p.IsGroupAllowed("billing") {
		t.Fatal("non-whitelisted group should be blocked when AllowedGroups is set")
	}
}

func TestBlockReason(t *testing.T) {
	// Explicitly denied
	p := &Policy{Mode: Standard, DeniedTools: map[string]bool{"clockify_delete_entry": true}}
	reason := p.BlockReason("clockify_delete_entry", false)
	expected := "tool 'clockify_delete_entry' is explicitly denied"
	if reason != expected {
		t.Fatalf("expected %q, got %q", expected, reason)
	}

	// ReadOnly write tool
	p = &Policy{Mode: ReadOnly, DeniedTools: map[string]bool{}}
	reason = p.BlockReason("clockify_start_timer", false)
	expected = "policy is read_only; 'clockify_start_timer' is a write tool"
	if reason != expected {
		t.Fatalf("expected %q, got %q", expected, reason)
	}

	// SafeCore non-safe write
	p = &Policy{Mode: SafeCore, DeniedTools: map[string]bool{}}
	reason = p.BlockReason("clockify_delete_entry", false)
	expected = "policy is safe_core; 'clockify_delete_entry' is not in the safe write list"
	if reason != expected {
		t.Fatalf("expected %q, got %q", expected, reason)
	}

	// Generic block
	p = &Policy{Mode: Standard, DeniedTools: map[string]bool{}}
	reason = p.BlockReason("clockify_some_tool", true)
	expected = "tool 'clockify_some_tool' is blocked by policy mode 'standard'"
	if reason != expected {
		t.Fatalf("expected %q, got %q", expected, reason)
	}
}

func TestDescribe(t *testing.T) {
	p := &Policy{
		Mode:          SafeCore,
		DeniedTools:   map[string]bool{"clockify_delete_entry": true},
		DeniedGroups:  map[string]bool{"billing": true},
		AllowedGroups: map[string]bool{"time_tracking": true},
	}

	desc := p.Describe()

	if desc["mode"] != "safe_core" {
		t.Fatalf("expected mode safe_core, got %v", desc["mode"])
	}

	deniedTools := desc["denied_tools"].([]string)
	if len(deniedTools) != 1 || deniedTools[0] != "clockify_delete_entry" {
		t.Fatalf("unexpected denied_tools: %v", deniedTools)
	}

	deniedGroups := desc["denied_groups"].([]string)
	if len(deniedGroups) != 1 || deniedGroups[0] != "billing" {
		t.Fatalf("unexpected denied_groups: %v", deniedGroups)
	}

	allowedGroups := desc["allowed_groups"].([]string)
	if len(allowedGroups) != 1 || allowedGroups[0] != "time_tracking" {
		t.Fatalf("unexpected allowed_groups: %v", allowedGroups)
	}

	introTools := desc["introspection_tools"].([]string)
	if len(introTools) != 6 {
		t.Fatalf("expected 6 introspection tools, got %d", len(introTools))
	}

	scWrites := desc["safe_core_writes"].([]string)
	if len(scWrites) != 11 {
		t.Fatalf("expected 11 safe_core_writes, got %d", len(scWrites))
	}

	// Describe with nil AllowedGroups
	p2 := &Policy{
		Mode:         Standard,
		DeniedTools:  map[string]bool{},
		DeniedGroups: map[string]bool{},
	}
	desc2 := p2.Describe()
	if desc2["allowed_groups"] != nil {
		t.Fatalf("expected nil allowed_groups, got %v", desc2["allowed_groups"])
	}
}

func TestSafeCoreExpandedAllowlist(t *testing.T) {
	p := &Policy{Mode: SafeCore, DeniedTools: map[string]bool{}}
	safeCoreTools := []string{
		"clockify_start_timer", "clockify_stop_timer",
		"clockify_add_entry", "clockify_update_entry",
		"clockify_log_time", "clockify_switch_project",
		"clockify_find_and_update_entry",
		"clockify_create_project", "clockify_create_client",
		"clockify_create_tag", "clockify_create_task",
	}
	for _, name := range safeCoreTools {
		if !p.IsAllowed(name, false) {
			t.Fatalf("safe core write tool %q should be allowed in safe_core mode", name)
		}
	}
	if len(safeCoreTools) != 11 {
		t.Fatalf("expected 11 safe core write tools, got %d", len(safeCoreTools))
	}
}

func TestIntrospectionAlwaysAllowed(t *testing.T) {
	p := &Policy{Mode: ReadOnly, DeniedTools: map[string]bool{}}
	introTools := []string{
		"clockify_whoami", "clockify_current_user", "clockify_list_workspaces",
		"clockify_search_tools", "clockify_policy_info", "clockify_resolve_debug",
	}
	for _, name := range introTools {
		if !p.IsAllowed(name, false) {
			t.Fatalf("introspection tool %q should be allowed in read_only mode", name)
		}
	}
	if len(introTools) != 6 {
		t.Fatalf("expected 6 introspection tools, got %d", len(introTools))
	}
}

func TestFromEnvWithGroups(t *testing.T) {
	// Set up env vars
	os.Setenv("CLOCKIFY_POLICY", "standard")
	os.Setenv("CLOCKIFY_DENY_TOOLS", "")
	os.Setenv("CLOCKIFY_DENY_GROUPS", "billing, invoices")
	os.Setenv("CLOCKIFY_ALLOW_GROUPS", "time_tracking, projects")
	defer func() {
		os.Unsetenv("CLOCKIFY_POLICY")
		os.Unsetenv("CLOCKIFY_DENY_TOOLS")
		os.Unsetenv("CLOCKIFY_DENY_GROUPS")
		os.Unsetenv("CLOCKIFY_ALLOW_GROUPS")
	}()

	p, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error: %v", err)
	}

	if !p.DeniedGroups["billing"] {
		t.Fatal("expected billing in DeniedGroups")
	}
	if !p.DeniedGroups["invoices"] {
		t.Fatal("expected invoices in DeniedGroups")
	}
	if len(p.DeniedGroups) != 2 {
		t.Fatalf("expected 2 denied groups, got %d", len(p.DeniedGroups))
	}

	if p.AllowedGroups == nil {
		t.Fatal("expected AllowedGroups to be set")
	}
	if !p.AllowedGroups["time_tracking"] {
		t.Fatal("expected time_tracking in AllowedGroups")
	}
	if !p.AllowedGroups["projects"] {
		t.Fatal("expected projects in AllowedGroups")
	}
	if len(p.AllowedGroups) != 2 {
		t.Fatalf("expected 2 allowed groups, got %d", len(p.AllowedGroups))
	}

	// Test with CLOCKIFY_ALLOW_GROUPS unset -> AllowedGroups should be nil
	os.Unsetenv("CLOCKIFY_ALLOW_GROUPS")
	p2, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error: %v", err)
	}
	if p2.AllowedGroups != nil {
		t.Fatal("expected AllowedGroups to be nil when env var is unset")
	}
}
