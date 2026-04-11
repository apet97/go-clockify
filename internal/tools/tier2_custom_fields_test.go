package tools

import (
	"context"
	"net/http"
	"testing"
)

// TestTier2_CustomFields_FullSweep covers the custom_fields Tier 2 group:
// list/get/create/update/delete plus the SetCustomFieldValue helper.
func TestTier2_CustomFields_FullSweep(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/custom-fields":
			respondJSON(t, w, []map[string]any{{"id": "f1", "name": "Region"}})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/custom-fields/f1":
			respondJSON(t, w, map[string]any{"id": "f1", "name": "Region", "type": "DROPDOWN_SINGLE"})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/custom-fields":
			respondJSON(t, w, map[string]any{"id": "f-new", "name": "Priority"})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/custom-fields/f1":
			respondJSON(t, w, map[string]any{"id": "f1", "name": "Region (updated)"})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/custom-fields/f1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	client, cleanup := newTestClient(t, mux.ServeHTTP)
	defer cleanup()
	svc := New(client, "ws1")
	ctx := context.Background()

	// List
	res, err := svc.ListCustomFields(ctx, map[string]any{"page": 1, "page_size": 25})
	mustOK(t, res, err, "clockify_list_custom_fields")

	// Get + validation
	res, err = svc.GetCustomField(ctx, map[string]any{"field_id": "f1"})
	mustOK(t, res, err, "clockify_get_custom_field")
	if _, err := svc.GetCustomField(ctx, map[string]any{"field_id": ""}); err == nil {
		t.Fatal("expected validation error for empty field_id")
	}

	// Create — happy with optional flags
	res, err = svc.CreateCustomField(ctx, map[string]any{
		"name":           "Priority",
		"field_type":     "dropdown_single",
		"allowed_values": []any{"P0", "P1", "P2"},
		"required":       true,
	})
	mustOK(t, res, err, "clockify_create_custom_field")
	if _, err := svc.CreateCustomField(ctx, map[string]any{"field_type": "TEXT"}); err == nil {
		t.Fatal("expected error for missing name")
	}
	if _, err := svc.CreateCustomField(ctx, map[string]any{"name": "x"}); err == nil {
		t.Fatal("expected error for missing field_type")
	}

	// Update + validation
	res, err = svc.UpdateCustomField(ctx, map[string]any{
		"field_id":       "f1",
		"name":           "Region (updated)",
		"allowed_values": []any{"NA", "EMEA"},
		"required":       false,
	})
	mustOK(t, res, err, "clockify_update_custom_field")
	if _, err := svc.UpdateCustomField(ctx, map[string]any{"field_id": ""}); err == nil {
		t.Fatal("expected validation error for empty field_id")
	}

	// Delete — dry-run, executed, validation
	res, err = svc.DeleteCustomField(ctx, map[string]any{"field_id": "f1", "dry_run": true})
	mustOK(t, res, err, "clockify_delete_custom_field")
	res, err = svc.DeleteCustomField(ctx, map[string]any{"field_id": "f1"})
	mustOK(t, res, err, "clockify_delete_custom_field")
	if _, err := svc.DeleteCustomField(ctx, map[string]any{"field_id": ""}); err == nil {
		t.Fatal("expected validation error for empty field_id")
	}

	// SetCustomFieldValue validation branches (no upstream call needed)
	if _, err := svc.SetCustomFieldValue(ctx, map[string]any{}); err == nil {
		t.Fatal("expected validation error for empty field_id")
	}
	if _, err := svc.SetCustomFieldValue(ctx, map[string]any{"field_id": "f1"}); err == nil {
		t.Fatal("expected error for missing value")
	}
	if _, err := svc.SetCustomFieldValue(ctx, map[string]any{"field_id": "f1", "value": "x"}); err == nil {
		t.Fatal("expected error: must specify project_id or entry_id")
	}
}
