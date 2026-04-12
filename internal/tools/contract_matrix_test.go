package tools

import (
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/policy"
)

func TestToolContractMatrix(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	all := map[string]mcpToolContract{}
	for _, d := range svc.Registry() {
		all[d.Tool.Name] = mcpToolContract{readOnly: d.ReadOnlyHint, destructive: d.DestructiveHint, idempotent: d.IdempotentHint, annotations: d.Tool.Annotations}
	}
	for group := range Tier2Groups {
		descriptors, ok := svc.Tier2Handlers(group)
		if !ok {
			t.Fatalf("missing tier2 handlers for group %q", group)
		}
		for _, d := range descriptors {
			all[d.Tool.Name] = mcpToolContract{readOnly: d.ReadOnlyHint, destructive: d.DestructiveHint, idempotent: d.IdempotentHint, annotations: d.Tool.Annotations}
		}
	}
	if len(all) != 124 {
		t.Fatalf("expected 124 tools, got %d", len(all))
	}

	readOnly := &policy.Policy{Mode: policy.ReadOnly, DeniedTools: map[string]bool{}, DeniedGroups: map[string]bool{}}
	safeCore := &policy.Policy{Mode: policy.SafeCore, DeniedTools: map[string]bool{}, DeniedGroups: map[string]bool{}}
	introspection := map[string]bool{
		"clockify_whoami": true, "clockify_current_user": true, "clockify_list_workspaces": true,
		"clockify_search_tools": true, "clockify_policy_info": true, "clockify_resolve_debug": true,
	}
	safeCoreWrites := map[string]bool{
		"clockify_start_timer": true, "clockify_stop_timer": true, "clockify_add_entry": true,
		"clockify_update_entry": true, "clockify_log_time": true, "clockify_switch_project": true,
		"clockify_find_and_update_entry": true, "clockify_create_project": true, "clockify_create_client": true,
		"clockify_create_tag": true, "clockify_create_task": true,
	}

	for name, contract := range all {
		assertBoolAnnotation(t, name, contract.annotations, "readOnlyHint", contract.readOnly)
		assertBoolAnnotation(t, name, contract.annotations, "destructiveHint", contract.destructive)
		assertBoolAnnotation(t, name, contract.annotations, "idempotentHint", contract.idempotent)

		expectReadOnlyAllowed := contract.readOnly || introspection[name]
		if got := readOnly.IsAllowed(name, contract.readOnly); got != expectReadOnlyAllowed {
			t.Fatalf("read_only policy mismatch for %s: got %v want %v", name, got, expectReadOnlyAllowed)
		}

		expectSafeCoreAllowed := contract.readOnly || introspection[name] || safeCoreWrites[name]
		if got := safeCore.IsAllowed(name, contract.readOnly); got != expectSafeCoreAllowed {
			t.Fatalf("safe_core policy mismatch for %s: got %v want %v", name, got, expectSafeCoreAllowed)
		}

		args := map[string]any{"dry_run": true}
		_, isDryRun := dryrun.CheckDryRun(name, args, contract.destructive)
		if contract.destructive && !isDryRun {
			t.Fatalf("destructive tool %s is missing dry-run interception", name)
		}
		if !contract.destructive && isDryRun {
			t.Fatalf("non-destructive tool %s should not be intercepted by global dry-run", name)
		}
	}
}

type mcpToolContract struct {
	readOnly    bool
	destructive bool
	idempotent  bool
	annotations map[string]any
}

func assertBoolAnnotation(t *testing.T, name string, annotations map[string]any, key string, want bool) {
	t.Helper()
	got, ok := annotations[key].(bool)
	if !ok {
		t.Fatalf("%s missing %s annotation", name, key)
	}
	if got != want {
		t.Fatalf("%s annotation %s mismatch: got %v want %v", name, key, got, want)
	}
}
