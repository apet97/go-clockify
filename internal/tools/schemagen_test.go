package tools

import (
	"reflect"
	"testing"
	"time"
)

// TestSchemaFor_Primitives spot-checks the kind→type mapping for every
// primitive supported by the generator.
func TestSchemaFor_Primitives(t *testing.T) {
	cases := []struct {
		name string
		got  map[string]any
		want string
	}{
		{"string", schemaFor[string](), "string"},
		{"bool", schemaFor[bool](), "boolean"},
		{"int", schemaFor[int](), "integer"},
		{"int64", schemaFor[int64](), "integer"},
		{"uint32", schemaFor[uint32](), "integer"},
		{"float64", schemaFor[float64](), "number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got["type"]; got != tc.want {
				t.Fatalf("schemaFor[%s] type = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestSchemaFor_TimeIsRFC3339 verifies time.Time gets the RFC 3339 form
// rather than being walked as a struct.
func TestSchemaFor_TimeIsRFC3339(t *testing.T) {
	got := schemaFor[time.Time]()
	if got["type"] != "string" {
		t.Fatalf("time.Time type = %v, want string", got["type"])
	}
	if got["format"] != "date-time" {
		t.Fatalf("time.Time format = %v, want date-time", got["format"])
	}
}

// TestSchemaFor_Slice covers []T -> {type: array, items: ...}.
func TestSchemaFor_Slice(t *testing.T) {
	got := schemaFor[[]string]()
	if got["type"] != "array" {
		t.Fatalf("type = %v, want array", got["type"])
	}
	items, ok := got["items"].(map[string]any)
	if !ok {
		t.Fatalf("items missing or wrong type: %T", got["items"])
	}
	if items["type"] != "string" {
		t.Fatalf("items.type = %v, want string", items["type"])
	}
}

// TestSchemaFor_Map covers map[string]T -> additionalProperties.
func TestSchemaFor_Map(t *testing.T) {
	got := schemaFor[map[string]int]()
	if got["type"] != "object" {
		t.Fatalf("type = %v, want object", got["type"])
	}
	add, ok := got["additionalProperties"].(map[string]any)
	if !ok {
		t.Fatalf("additionalProperties wrong type: %T", got["additionalProperties"])
	}
	if add["type"] != "integer" {
		t.Fatalf("additionalProperties.type = %v, want integer", add["type"])
	}
}

// TestSchemaFor_Pointer verifies pointer types are unwrapped to their
// element schema.
func TestSchemaFor_Pointer(t *testing.T) {
	got := schemaFor[*string]()
	if got["type"] != "string" {
		t.Fatalf("type = %v, want string (pointer unwrap)", got["type"])
	}
}

// TestSchemaFor_Interface verifies interface{} produces no constraint.
func TestSchemaFor_Interface(t *testing.T) {
	got := schemaFor[any]()
	if len(got) != 0 {
		t.Fatalf("schemaFor[any] should be empty, got %+v", got)
	}
}

// schemaTestStruct exercises every json-tag branch in jsonFieldName.
type schemaTestStruct struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Count       int            `json:"count"`
	Tags        []string       `json:"tags,omitempty"`
	Created     time.Time      `json:"created"`
	Meta        map[string]any `json:"meta,omitempty"`
	Inner       *schemaInner   `json:"inner,omitempty"`
	NoTagField  string
	Hidden      string `json:"-"`
	unexported  string //nolint:unused
}

type schemaInner struct {
	Value int `json:"value"`
}

// TestSchemaFor_Struct walks an exhaustive struct shape and asserts the
// emitted schema matches the json tags.
func TestSchemaFor_Struct(t *testing.T) {
	got := schemaFor[schemaTestStruct]()
	if got["type"] != "object" {
		t.Fatalf("type = %v", got["type"])
	}
	if got["additionalProperties"] != false {
		t.Fatal("expected additionalProperties: false")
	}
	props, ok := got["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties wrong type: %T", got["properties"])
	}

	// Required: every field without ,omitempty should appear.
	required, _ := got["required"].([]string)
	wantRequired := map[string]bool{"name": true, "count": true, "created": true, "NoTagField": true}
	for _, name := range required {
		if !wantRequired[name] {
			t.Errorf("unexpected required field: %s", name)
		}
		delete(wantRequired, name)
	}
	for missing := range wantRequired {
		t.Errorf("missing required field: %s", missing)
	}

	// Hidden field with json:"-" must not appear at all.
	if _, exists := props["Hidden"]; exists {
		t.Error("Hidden field should be skipped")
	}
	if _, exists := props["hidden"]; exists {
		t.Error("Hidden field should be skipped")
	}
	// unexported field must not appear.
	if _, exists := props["unexported"]; exists {
		t.Error("unexported field should be skipped")
	}

	// NoTagField should keep its Go-name as the property key.
	if _, exists := props["NoTagField"]; !exists {
		t.Error("NoTagField should be present under its Go name")
	}

	// Inner should resolve through pointer unwrap to a typed object.
	inner, ok := props["inner"].(map[string]any)
	if !ok {
		t.Fatalf("inner wrong type: %T", props["inner"])
	}
	if inner["type"] != "object" {
		t.Fatalf("inner.type = %v", inner["type"])
	}
	innerProps, ok := inner["properties"].(map[string]any)
	if !ok {
		t.Fatalf("inner.properties missing")
	}
	if v, ok := innerProps["value"].(map[string]any); !ok || v["type"] != "integer" {
		t.Fatalf("inner.properties.value = %+v", innerProps["value"])
	}
}

// TestEnvelopeSchemaFor verifies the wrapper shape is stable: ok/action/data
// always present, action bound as const, data filled from schemaFor[T].
func TestEnvelopeSchemaFor(t *testing.T) {
	got := envelopeSchemaFor[SummaryData]("clockify_summary_report")
	if got["type"] != "object" {
		t.Fatalf("type = %v", got["type"])
	}
	required, _ := got["required"].([]string)
	wantRequired := []string{"ok", "action", "data"}
	if !reflect.DeepEqual(required, wantRequired) {
		t.Fatalf("required = %v, want %v", required, wantRequired)
	}
	props := got["properties"].(map[string]any)
	action := props["action"].(map[string]any)
	if action["const"] != "clockify_summary_report" {
		t.Fatalf("action const = %v", action["const"])
	}
	data := props["data"].(map[string]any)
	if data["type"] != "object" {
		t.Fatalf("data should be a typed object, got %+v", data)
	}
}

// TestEnvelopeOpaque produces a wrapper for tools whose data field is
// open-shape. Data must be present but unconstrained inside.
func TestEnvelopeOpaque(t *testing.T) {
	got := envelopeOpaque("clockify_list_invoices")
	props := got["properties"].(map[string]any)
	if props["data"] == nil {
		t.Fatal("data property missing")
	}
	data := props["data"].(map[string]any)
	if data["additionalProperties"] != true {
		t.Fatal("opaque data should allow additionalProperties")
	}
}

// TestWithOutputSchema returns a Tool copy with the schema applied.
func TestWithOutputSchema(t *testing.T) {
	base := toolRO("noop", "noop", map[string]any{"type": "object"})
	out := envelopeOpaque("noop")
	wrapped := withOutputSchema(base, out)
	if wrapped.OutputSchema == nil {
		t.Fatal("OutputSchema not set")
	}
	if base.OutputSchema != nil {
		t.Fatal("base must not be mutated")
	}
}
