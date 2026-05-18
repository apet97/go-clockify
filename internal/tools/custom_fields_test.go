package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// TestTier2_CustomFields_FullSweep covers the custom_fields domain group:
// list/get/create/update/delete plus the SetCustomFieldValue helper.
func TestTier2_CustomFields_FullSweep(t *testing.T) {
	var createBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/custom-fields":
			respondJSON(t, w, []map[string]any{{"id": "f1", "name": "Region", "type": "TXT", "required": true}})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/custom-fields/f1":
			t.Fatalf("single custom-field GET must not be used; live Clockify returns 405 for this path")
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/custom-fields":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
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
	if createBody["status"] != "VISIBLE" {
		t.Fatalf("new custom fields should default status=VISIBLE, got %#v", createBody)
	}
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

func TestSetCustomFieldValueRejectsInactiveField(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/custom-fields":
			respondJSON(t, w, []map[string]any{{"id": "f1", "name": "Region", "type": "TXT", "status": "INACTIVE"}})
		default:
			t.Fatalf("inactive field must fail before value write; got %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.SetCustomFieldValue(context.Background(), map[string]any{
		"field_id":   "f1",
		"project_id": "p1",
		"value":      "North",
	})
	if err == nil || !strings.Contains(err.Error(), "status INACTIVE") {
		t.Fatalf("expected inactive custom-field error, got %v", err)
	}
}

func TestValidateCustomFieldSetValueRejectsDropdownValuesOutsideAllowedValues(t *testing.T) {
	cases := []struct {
		name    string
		field   map[string]any
		value   any
		wantErr string
	}{
		{
			name:  "single accepts allowed value id",
			field: map[string]any{"type": "DROPDOWN_SINGLE", "allowedValues": []any{map[string]any{"id": "opt-1", "name": "North"}}},
			value: "opt-1",
		},
		{
			name:    "single rejects unknown scalar",
			field:   map[string]any{"type": "DROPDOWN_SINGLE", "allowedValues": []any{"North", "South"}},
			value:   "East",
			wantErr: "not in allowedValues",
		},
		{
			name:  "multiple accepts allowed subset",
			field: map[string]any{"type": "DROPDOWN_MULTIPLE", "allowedValues": []any{"North", "South", "East"}},
			value: []any{"North", "East"},
		},
		{
			name:    "multiple rejects scalar",
			field:   map[string]any{"type": "DROPDOWN_MULTIPLE", "allowedValues": []any{"North", "South"}},
			value:   "North",
			wantErr: "must be an array",
		},
		{
			name:    "multiple rejects unknown array item",
			field:   map[string]any{"type": "DROPDOWN_MULTIPLE", "allowedValues": []any{"North", "South"}},
			value:   []any{"South", "East"},
			wantErr: "not in allowedValues",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCustomFieldSetValue(tc.field, tc.value)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
		})
	}
}

// TestValidateCustomFieldDefaultValueRejectsTypeMismatch is the
// table-driven contract for validateCustomFieldDefaultValue: every
// upstream-accepted custom-field type must accept its documented value
// shape and reject everything else with a clear message that mentions
// both the field type and the bad input.
func TestValidateCustomFieldDefaultValueRejectsTypeMismatch(t *testing.T) {
	cases := []struct {
		name      string
		fieldType string
		raw       any
		allowed   []string
		want      any
		wantErr   string
	}{
		{name: "TXT accepts string", fieldType: "TXT", raw: "hello", want: "hello"},
		{name: "TXT rejects number", fieldType: "TXT", raw: 1.0, wantErr: "string for TXT"},
		{name: "LINK accepts string", fieldType: "LINK", raw: "https://example.com", want: "https://example.com"},
		{name: "LINK rejects boolean", fieldType: "LINK", raw: true, wantErr: "string for LINK"},
		{name: "NUMBER accepts float", fieldType: "NUMBER", raw: 3.5, want: 3.5},
		{name: "NUMBER accepts int", fieldType: "NUMBER", raw: 5, want: 5.0},
		{name: "NUMBER rejects string", fieldType: "NUMBER", raw: "5", wantErr: "number for NUMBER"},
		{name: "NUMBER accepts json.Number", fieldType: "NUMBER", raw: json.Number("42"), want: 42.0},
		{name: "CHECKBOX accepts true", fieldType: "CHECKBOX", raw: true, want: true},
		{name: "CHECKBOX accepts false", fieldType: "CHECKBOX", raw: false, want: false},
		{name: "CHECKBOX rejects string", fieldType: "CHECKBOX", raw: "true", wantErr: "boolean for CHECKBOX"},
		{name: "DROPDOWN_SINGLE accepts allowed string", fieldType: "DROPDOWN_SINGLE", raw: "North", allowed: []string{"North", "South"}, want: "North"},
		{name: "DROPDOWN_SINGLE rejects out-of-allowed", fieldType: "DROPDOWN_SINGLE", raw: "East", allowed: []string{"North", "South"}, wantErr: "must be one of allowed_values"},
		{name: "DROPDOWN_SINGLE rejects list", fieldType: "DROPDOWN_SINGLE", raw: []any{"North"}, wantErr: "string for DROPDOWN_SINGLE"},
		{name: "DROPDOWN_MULTIPLE accepts subset", fieldType: "DROPDOWN_MULTIPLE", raw: []any{"North", "South"}, allowed: []string{"North", "South", "East"}, want: []string{"North", "South"}},
		{name: "DROPDOWN_MULTIPLE rejects scalar", fieldType: "DROPDOWN_MULTIPLE", raw: "North", wantErr: "array of strings for DROPDOWN_MULTIPLE"},
		{name: "DROPDOWN_MULTIPLE rejects out-of-allowed item", fieldType: "DROPDOWN_MULTIPLE", raw: []any{"East"}, allowed: []string{"North", "South"}, wantErr: "must be one of allowed_values"},
		{name: "nil value passes through", fieldType: "TXT", raw: nil, want: nil},
		{name: "unsupported type errors", fieldType: "BOGUS", raw: "x", wantErr: "unsupported field_type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateCustomFieldDefaultValue(tc.fieldType, tc.raw, tc.allowed)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !equalAny(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestCreateCustomFieldForwardsWorkspaceDefaultValue exercises every
// supported field type end-to-end through CreateCustomField so the
// upstream POST body matches the per-type contract: scalar for
// TXT/LINK/DROPDOWN_SINGLE, number for NUMBER, boolean for CHECKBOX,
// list for DROPDOWN_MULTIPLE.
func TestCreateCustomFieldForwardsWorkspaceDefaultValue(t *testing.T) {
	cases := []struct {
		name       string
		fieldType  string
		allowed    []any
		defaultVal any
		wantInBody any
	}{
		{name: "TXT", fieldType: "TXT", defaultVal: "alpha", wantInBody: "alpha"},
		{name: "LINK", fieldType: "LINK", defaultVal: "https://example.com", wantInBody: "https://example.com"},
		{name: "NUMBER", fieldType: "NUMBER", defaultVal: 3.5, wantInBody: 3.5},
		{name: "CHECKBOX", fieldType: "CHECKBOX", defaultVal: true, wantInBody: true},
		{name: "DROPDOWN_SINGLE", fieldType: "DROPDOWN_SINGLE", allowed: []any{"red", "blue"}, defaultVal: "red", wantInBody: "red"},
		{name: "DROPDOWN_MULTIPLE", fieldType: "DROPDOWN_MULTIPLE", allowed: []any{"a", "b", "c"}, defaultVal: []any{"a", "c"}, wantInBody: []any{"a", "c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/custom-fields" {
					t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("decode: %v", err)
				}
				respondJSON(t, w, map[string]any{"id": "f1", "name": gotBody["name"], "type": gotBody["type"]})
			})
			defer cleanup()

			svc := New(client, "ws1")
			args := map[string]any{
				"name":                    "Test Field",
				"field_type":              tc.fieldType,
				"workspace_default_value": tc.defaultVal,
			}
			if tc.allowed != nil {
				args["allowed_values"] = tc.allowed
			}
			if _, err := svc.CreateCustomField(context.Background(), args); err != nil {
				t.Fatalf("CreateCustomField: %v", err)
			}
			got := gotBody["workspaceDefaultValue"]
			if !equalAny(got, tc.wantInBody) {
				t.Fatalf("body workspaceDefaultValue = %#v, want %#v (full body=%#v)", got, tc.wantInBody, gotBody)
			}
		})
	}
}

// TestCreateCustomFieldRejectsTypeMismatchedDefault checks that
// validation runs *before* any upstream call so a Clockify 400 turns
// into a clean MCP error envelope.
func TestCreateCustomFieldRejectsTypeMismatchedDefault(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("validation must fail before upstream POST: got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()
	svc := New(client, "ws1")

	_, err := svc.CreateCustomField(context.Background(), map[string]any{
		"name":                    "Bad",
		"field_type":              "NUMBER",
		"workspace_default_value": "not-a-number",
	})
	if err == nil || !strings.Contains(err.Error(), "number for NUMBER") {
		t.Fatalf("expected NUMBER type-mismatch error, got %v", err)
	}
}

// equalAny compares values that may include []any vs []string slice
// shapes; reflect.DeepEqual gets cranky about that pair.
func equalAny(a, b any) bool {
	switch wa := a.(type) {
	case []string:
		switch wb := b.(type) {
		case []string:
			if len(wa) != len(wb) {
				return false
			}
			for i := range wa {
				if wa[i] != wb[i] {
					return false
				}
			}
			return true
		case []any:
			if len(wa) != len(wb) {
				return false
			}
			for i := range wa {
				s, ok := wb[i].(string)
				if !ok || s != wa[i] {
					return false
				}
			}
			return true
		}
	case []any:
		switch wb := b.(type) {
		case []any:
			if len(wa) != len(wb) {
				return false
			}
			for i := range wa {
				if !equalAny(wa[i], wb[i]) {
					return false
				}
			}
			return true
		case []string:
			return equalAny(b, a)
		}
	}
	return a == b
}

func TestListCustomFields_PaginationMeta(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/custom-fields" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{
				{"id": "f1", "name": "Field 1"},
				{"id": "f2", "name": "Field 2"},
				{"id": "f3", "name": "Field 3"},
				{"id": "f4", "name": "Field 4"},
				{"id": "f5", "name": "Field 5"},
				{"id": "f6", "name": "Field 6"},
				{"id": "f7", "name": "Field 7"},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	first, err := svc.ListCustomFields(context.Background(), map[string]any{"page_size": 5})
	if err != nil {
		t.Fatalf("ListCustomFields page 1: %v", err)
	}
	items, ok := first.Data.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any, got %T", first.Data)
	}
	if len(items) != 5 {
		t.Fatalf("page 1 len=%d, want 5", len(items))
	}
	if first.Meta["page"] != 1 || first.Meta["pageSize"] != 5 || first.Meta["has_more"] != true {
		t.Fatalf("unexpected page 1 meta: %#v", first.Meta)
	}

	second, err := svc.ListCustomFields(context.Background(), map[string]any{"page": 2, "page_size": 5})
	if err != nil {
		t.Fatalf("ListCustomFields page 2: %v", err)
	}
	secondItems := second.Data.([]map[string]any)
	if len(secondItems) != 2 {
		t.Fatalf("page 2 len=%d, want 2", len(secondItems))
	}
	if items[0]["id"] == secondItems[0]["id"] {
		t.Fatalf("page 2 did not advance: page1=%#v page2=%#v", items, secondItems)
	}
}

func TestGetCustomFieldFindsFieldOnSecondPage(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/custom-fields" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			fields := make([]map[string]any, 50)
			for i := range fields {
				fields[i] = map[string]any{"id": "page1-" + strconv.Itoa(i), "name": "Page 1"}
			}
			respondJSON(t, w, fields)
		case "2":
			respondJSON(t, w, []map[string]any{{"id": "target-field", "name": "Target", "type": "TXT"}})
		default:
			respondJSON(t, w, []map[string]any{})
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	got, err := svc.GetCustomField(context.Background(), map[string]any{"field_id": "target-field"})
	if err != nil {
		t.Fatalf("GetCustomField: %v", err)
	}
	field, ok := got.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want map[string]any", got.Data)
	}
	if field["id"] != "target-field" {
		t.Fatalf("field id = %v, want target-field", field["id"])
	}
	if got.Meta["scanned"] != 51 {
		t.Fatalf("scanned = %v, want 51", got.Meta["scanned"])
	}
}
