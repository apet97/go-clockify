package dryrun

import (
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// Original tests (backward compatibility)
// ---------------------------------------------------------------------------

func TestEnabled(t *testing.T) {
	if !Enabled(map[string]any{"dry_run": true}) {
		t.Fatal("expected dry_run true to be enabled")
	}
	if Enabled(map[string]any{"dry_run": false}) {
		t.Fatal("expected dry_run false to be disabled")
	}
}

// ---------------------------------------------------------------------------
// ConfigFromEnv
// ---------------------------------------------------------------------------

func TestConfigFromEnvDefault(t *testing.T) {
	os.Unsetenv("CLOCKIFY_DRY_RUN")
	c := ConfigFromEnv()
	if !c.Enabled {
		t.Fatal("expected Config.Enabled=true by default")
	}
}

func TestConfigFromEnvOff(t *testing.T) {
	for _, v := range []string{"off", "disabled", "0", "false", "OFF", "False"} {
		os.Setenv("CLOCKIFY_DRY_RUN", v)
		c := ConfigFromEnv()
		if c.Enabled {
			t.Fatalf("expected Config.Enabled=false for CLOCKIFY_DRY_RUN=%q", v)
		}
	}
	os.Unsetenv("CLOCKIFY_DRY_RUN")
}

// ---------------------------------------------------------------------------
// CheckDryRun
// ---------------------------------------------------------------------------

func TestCheckDryRunNotSet(t *testing.T) {
	args := map[string]any{"workspace_id": "w1"}
	action, active := CheckDryRun("clockify_get_entry", args, false)
	if active {
		t.Fatal("expected active=false when dry_run not in args")
	}
	if action != 0 {
		t.Fatalf("expected action=0, got %d", action)
	}
}

func TestCheckDryRunNonDestructive_PassThrough(t *testing.T) {
	args := map[string]any{"dry_run": true, "description": "test"}
	action, active := CheckDryRun("clockify_add_entry", args, false)
	if active {
		t.Fatal("expected active=false for non-destructive tool (pass through to handler)")
	}
	if action != 0 {
		t.Fatalf("expected action=0, got %d", action)
	}
	// Flag must remain in args so the handler's dryrun.Enabled() check sees it.
	if _, exists := args["dry_run"]; !exists {
		t.Fatal("expected dry_run key to remain in args for non-destructive tools")
	}
}

func TestCheckDryRunPreviewTool(t *testing.T) {
	args := map[string]any{"dry_run": true}
	action, active := CheckDryRun("clockify_entries_delete", args, true)
	if !active {
		t.Fatal("expected active=true")
	}
	if action != PreviewTool {
		t.Fatalf("expected PreviewTool, got %d", action)
	}
}

func TestCheckDryRunMinimal(t *testing.T) {
	args := map[string]any{"dry_run": true}
	action, active := CheckDryRun("clockify_invoices_items_delete", args, true)
	if !active {
		t.Fatal("expected active=true")
	}
	if action != MinimalFallback {
		t.Fatalf("expected MinimalFallback, got %d", action)
	}

	args = map[string]any{"dry_run": true}
	action, active = CheckDryRun("clockify_invoices_payments_delete", args, true)
	if !active {
		t.Fatal("expected active=true")
	}
	if action != MinimalFallback {
		t.Fatalf("expected MinimalFallback for payment delete, got %d", action)
	}
}

func TestCheckDryRunDefaultDestructive(t *testing.T) {
	args := map[string]any{"dry_run": true}
	action, active := CheckDryRun("clockify_unknown_destructive_tool", args, true)
	if !active {
		t.Fatal("expected active=true")
	}
	if action != MinimalFallback {
		t.Fatalf("expected MinimalFallback for unknown destructive tool, got %d", action)
	}
}

func TestCheckDryRunConsumesFlag(t *testing.T) {
	args := map[string]any{"dry_run": true, "entry_id": "e1"}
	_, active := CheckDryRun("clockify_entries_delete", args, true)
	if !active {
		t.Fatal("expected active=true")
	}
	if _, exists := args["dry_run"]; exists {
		t.Fatal("expected dry_run key to be deleted from args after CheckDryRun")
	}
	if _, exists := args["entry_id"]; !exists {
		t.Fatal("expected entry_id to remain in args")
	}
}

// ---------------------------------------------------------------------------
// PreviewToolFor
// ---------------------------------------------------------------------------

func TestPreviewToolFor(t *testing.T) {
	expected := map[string]string{
		"clockify_clients_delete":           "clockify_clients_get",
		"clockify_projects_delete":          "clockify_projects_get",
		"clockify_tasks_delete":             "clockify_tasks_get",
		"clockify_tags_delete":              "clockify_tags_get",
		"clockify_entries_delete":           "clockify_entries_get",
		"clockify_invoices_delete":          "clockify_invoices_get",
		"clockify_expenses_delete":          "clockify_expenses_get",
		"clockify_custom_fields_delete":     "clockify_custom_fields_get",
		"clockify_time_off_requests_delete": "clockify_time_off_requests_get",
		"clockify_webhooks_delete":          "clockify_webhooks_get",
	}
	for del, get := range expected {
		got, ok := PreviewToolFor(del)
		if !ok {
			t.Fatalf("PreviewToolFor(%q): expected ok=true", del)
		}
		if got != get {
			t.Fatalf("PreviewToolFor(%q): expected %q, got %q", del, get, got)
		}
	}

	// Non-existent tool should return false.
	_, ok := PreviewToolFor("clockify_delete_unknown")
	if ok {
		t.Fatal("expected ok=false for unknown tool")
	}
}

// ---------------------------------------------------------------------------
// BuildPreviewArgs
// ---------------------------------------------------------------------------

func TestBuildPreviewArgs(t *testing.T) {
	args := map[string]any{
		"workspace_id": "w1",
		"entry_id":     "e1",
		"project_id":   "p1",
		"description":  "some text",
		"confirm":      true,
	}
	out := BuildPreviewArgs(args)
	if _, exists := out["workspace_id"]; exists {
		t.Fatal("workspace_id should be excluded")
	}
	if out["entry_id"] != "e1" {
		t.Fatal("expected entry_id=e1")
	}
	if out["project_id"] != "p1" {
		t.Fatal("expected project_id=p1")
	}
	if _, exists := out["description"]; exists {
		t.Fatal("non-_id keys should be excluded")
	}
	if _, exists := out["confirm"]; exists {
		t.Fatal("non-_id keys should be excluded")
	}
}

// ---------------------------------------------------------------------------
// WrapResult
// ---------------------------------------------------------------------------

func TestWrapResult(t *testing.T) {
	result := map[string]any{"id": "e1", "description": "test"}
	wrapped := WrapResult(result, "clockify_delete_entry")

	if wrapped["dry_run"] != true {
		t.Fatal("expected dry_run=true")
	}
	if wrapped["tool"] != "clockify_delete_entry" {
		t.Fatal("expected tool=clockify_delete_entry")
	}
	if wrapped["preview"] == nil {
		t.Fatal("expected preview to be non-nil")
	}
	if wrapped["note"] != "This is a dry-run preview. No changes were made." {
		t.Fatalf("unexpected note: %v", wrapped["note"])
	}
}

// ---------------------------------------------------------------------------
// MinimalResult
// ---------------------------------------------------------------------------

func TestMinimalResult(t *testing.T) {
	args := map[string]any{"entry_id": "e1"}
	m := MinimalResult("clockify_invoices_items_delete", args)

	if m["dry_run"] != true {
		t.Fatal("expected dry_run=true")
	}
	if m["tool"] != "clockify_invoices_items_delete" {
		t.Fatal("expected tool=clockify_invoices_items_delete")
	}
	if m["args"] == nil {
		t.Fatal("expected args to be non-nil")
	}
	if m["resource"] != nil {
		t.Fatal("expected resource=nil")
	}
	if m["note"] != "This is a dry-run preview. No changes were made. No preview data available for this tool." {
		t.Fatalf("unexpected note: %v", m["note"])
	}
}
