package tools

import (
	"context"
	"encoding/json"
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
			respondJSON(t, w, []map[string]any{{"id": "f1", "name": "Region", "type": "TXT", "required": true}})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/custom-fields/f1":
			t.Fatalf("single custom-field GET must not be used; live Clockify returns 405 for this path")
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/custom-fields":
			respondJSON(t, w, map[string]any{"id": "f-new", "name": "Priority"})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/custom-fields/f1":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			if got, _ := body["type"].(string); got != "TXT" {
				t.Fatalf("update body type = %q, want current TXT (body=%#v)", got, body)
			}
			respondJSON(t, w, map[string]any{"id": "f1", "name": "Region (updated)"})
		case r.Method == "PATCH" && r.URL.Path == "/workspaces/ws1/projects/p1/custom-fields/f1":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode project custom-field body: %v", err)
			}
			if got, _ := body["defaultValue"].(string); got != "North" {
				t.Fatalf("project custom-field defaultValue = %q, want North (body=%#v)", got, body)
			}
			respondJSON(t, w, map[string]any{"id": "f1", "defaultValue": "North"})
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
	// SUMMARY rev 3 #19: the historical TEXT / DROPDOWN aliases are
	// rejected by the upstream — fail locally with the corrected
	// enum list rather than round-tripping the 400.
	if _, err := svc.CreateCustomField(ctx, map[string]any{"name": "x", "field_type": "TEXT"}); err == nil {
		t.Fatal("expected enum-validation error for legacy TEXT alias")
	}
	if _, err := svc.CreateCustomField(ctx, map[string]any{"name": "x", "field_type": "DROPDOWN"}); err == nil {
		t.Fatal("expected enum-validation error for legacy DROPDOWN alias")
	}
	// allowed_values is implicitly required for both DROPDOWN variants
	// — every live dropdown carries a non-empty list per probe data.
	if _, err := svc.CreateCustomField(ctx, map[string]any{"name": "x", "field_type": "DROPDOWN_SINGLE"}); err == nil {
		t.Fatal("expected error: DROPDOWN_SINGLE without allowed_values")
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

	res, err = svc.SetCustomFieldValue(ctx, map[string]any{
		"field_id":   "f1",
		"project_id": "p1",
		"value":      "North",
	})
	mustOK(t, res, err, "clockify_set_custom_field_value")

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
