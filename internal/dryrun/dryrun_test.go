package dryrun

import "testing"

func TestEnabled(t *testing.T) {
	if !Enabled(map[string]any{"dry_run": true}) {
		t.Fatal("expected dry_run true to be enabled")
	}
	if Enabled(map[string]any{"dry_run": false}) {
		t.Fatal("expected dry_run false to be disabled")
	}
	if Enabled(map[string]any{"workspace_id": "w1"}) {
		t.Fatal("expected missing dry_run key to be disabled")
	}
	if Enabled(nil) {
		t.Fatal("expected nil args to be disabled")
	}
}

func TestWrapResult(t *testing.T) {
	result := map[string]any{"id": "e1", "description": "test"}
	wrapped := WrapResult(result, "clockify_entries_delete")

	if wrapped["dry_run"] != true {
		t.Fatal("expected dry_run=true")
	}
	if wrapped["tool"] != "clockify_entries_delete" {
		t.Fatal("expected tool=clockify_entries_delete")
	}
	if wrapped["preview"] == nil {
		t.Fatal("expected preview to be non-nil")
	}
	if wrapped["note"] != "This is a dry-run preview. No changes were made." {
		t.Fatalf("unexpected note: %v", wrapped["note"])
	}
}

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
