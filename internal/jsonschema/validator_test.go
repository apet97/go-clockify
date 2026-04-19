package jsonschema

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateNilSchemaPasses(t *testing.T) {
	if err := Validate(nil, map[string]any{"anything": 1}); err != nil {
		t.Fatalf("nil schema should pass, got %v", err)
	}
}

func TestValidateTypeSuccess(t *testing.T) {
	cases := []struct {
		name   string
		schema map[string]any
		value  any
	}{
		{"string", map[string]any{"type": "string"}, "hello"},
		{"integer_float", map[string]any{"type": "integer"}, float64(42)},
		{"integer_int", map[string]any{"type": "integer"}, 42},
		{"number", map[string]any{"type": "number"}, 3.14},
		{"boolean_true", map[string]any{"type": "boolean"}, true},
		{"boolean_false", map[string]any{"type": "boolean"}, false},
		{"array", map[string]any{"type": "array"}, []any{1, 2}},
		{"object", map[string]any{"type": "object"}, map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.schema, tc.value); err != nil {
				t.Fatalf("want pass, got %v", err)
			}
		})
	}
}

func TestValidateTypeMismatch(t *testing.T) {
	cases := []struct {
		name   string
		schema map[string]any
		value  any
	}{
		{"string_got_int", map[string]any{"type": "string"}, 1},
		{"integer_got_string", map[string]any{"type": "integer"}, "x"},
		{"integer_got_float", map[string]any{"type": "integer"}, 1.5},
		{"number_got_string", map[string]any{"type": "number"}, "x"},
		{"boolean_got_string", map[string]any{"type": "boolean"}, "yes"},
		{"array_got_object", map[string]any{"type": "array"}, map[string]any{}},
		{"object_got_array", map[string]any{"type": "object"}, []any{}},
		{"object_got_nil", map[string]any{"type": "object"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.schema, tc.value)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want *ValidationError, got %T", err)
			}
			if !strings.Contains(ve.Message, "expected") {
				t.Fatalf("message should name expected type: %q", ve.Message)
			}
		})
	}
}

func TestValidateRequired(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"start", "end"},
		"properties": map[string]any{
			"start": map[string]any{"type": "string"},
			"end":   map[string]any{"type": "string"},
		},
	}
	// missing start
	err := Validate(schema, map[string]any{"end": "2026-04-11"})
	if err == nil {
		t.Fatal("expected required error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if ve.Pointer != "/start" {
		t.Errorf("pointer = %q, want /start", ve.Pointer)
	}
	if !strings.Contains(ve.Message, "required") {
		t.Errorf("message = %q, want substring 'required'", ve.Message)
	}

	// required passed as []any still works
	schemaAny := map[string]any{
		"type":     "object",
		"required": []any{"start"},
		"properties": map[string]any{
			"start": map[string]any{"type": "string"},
		},
	}
	if err := Validate(schemaAny, map[string]any{}); err == nil {
		t.Fatal("expected required error for []any shape")
	}

	// all required present → pass
	if err := Validate(schema, map[string]any{"start": "a", "end": "b"}); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
}

func TestValidateAdditionalPropertiesFalse(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"start": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}
	err := Validate(schema, map[string]any{"start": "x", "bogus": "y"})
	if err == nil {
		t.Fatal("want unknown-property error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if ve.Pointer != "/bogus" {
		t.Errorf("pointer = %q, want /bogus", ve.Pointer)
	}
}

func TestValidateAdditionalPropertiesTrueAllowsExtras(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": true,
	}
	if err := Validate(schema, map[string]any{"extra": 1}); err != nil {
		t.Fatalf("true additionalProperties should pass, got %v", err)
	}
}

func TestValidateNestedPointer(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"outer": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"inner": map[string]any{"type": "integer", "minimum": 1},
				},
			},
		},
	}
	err := Validate(schema, map[string]any{
		"outer": map[string]any{"inner": 0},
	})
	if err == nil {
		t.Fatal("expected minimum error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if ve.Pointer != "/outer/inner" {
		t.Errorf("pointer = %q, want /outer/inner", ve.Pointer)
	}
}

func TestValidateArrayItemsPointer(t *testing.T) {
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"x": map[string]any{"type": "string"},
			},
			"required": []string{"x"},
		},
	}
	err := Validate(schema, []any{
		map[string]any{"x": "ok"},
		map[string]any{},
	})
	if err == nil {
		t.Fatal("expected required error on second element")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if ve.Pointer != "/1/x" {
		t.Errorf("pointer = %q, want /1/x", ve.Pointer)
	}
}

func TestValidateNumericBounds(t *testing.T) {
	schema := map[string]any{
		"type":    "integer",
		"minimum": 1,
		"maximum": 200,
	}
	if err := Validate(schema, 50); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
	if err := Validate(schema, 0); err == nil {
		t.Fatal("want minimum error")
	}
	if err := Validate(schema, 201); err == nil {
		t.Fatal("want maximum error")
	}
}

func TestValidateStringBounds(t *testing.T) {
	schema := map[string]any{
		"type":      "string",
		"minLength": 2,
		"maxLength": 5,
	}
	if err := Validate(schema, "abc"); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
	if err := Validate(schema, "a"); err == nil {
		t.Fatal("want minLength error")
	}
	if err := Validate(schema, "abcdef"); err == nil {
		t.Fatal("want maxLength error")
	}
}

func TestValidatePatternAnchored(t *testing.T) {
	schema := map[string]any{
		"type":    "string",
		"pattern": "^#[0-9a-fA-F]{6}$",
	}
	if err := Validate(schema, "#aabbcc"); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
	if err := Validate(schema, "not-hex"); err == nil {
		t.Fatal("want pattern error")
	}
	// Caller may omit anchors; we wrap them.
	unanchored := map[string]any{
		"type":    "string",
		"pattern": "foo",
	}
	if err := Validate(unanchored, "foo"); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
	// Unanchored wrapped in ^...$ so substring-only doesn't match.
	if err := Validate(unanchored, "xfoox"); err == nil {
		t.Fatal("want pattern error — unanchored should still anchor")
	}
}

func TestValidateFormatDateTime(t *testing.T) {
	schema := map[string]any{"type": "string", "format": "date-time"}
	if err := Validate(schema, "2026-04-11T09:00:00Z"); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
	if err := Validate(schema, "2026-04-11T09:00:00+02:00"); err != nil {
		t.Fatalf("want pass with offset, got %v", err)
	}
	if err := Validate(schema, "not a date"); err == nil {
		t.Fatal("want format error")
	}
}

func TestValidateFormatDate(t *testing.T) {
	schema := map[string]any{"type": "string", "format": "date"}
	if err := Validate(schema, "2026-04-11"); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
	if err := Validate(schema, "2026-13-40"); err == nil {
		t.Fatal("want date parse error")
	}
}

func TestValidateUnknownFormatIsNoOp(t *testing.T) {
	schema := map[string]any{"type": "string", "format": "uri"}
	if err := Validate(schema, "not-a-uri"); err != nil {
		t.Fatalf("unknown format should be no-op, got %v", err)
	}
}

func TestValidateEnum(t *testing.T) {
	schema := map[string]any{"enum": []any{"read", "write", "admin"}}
	if err := Validate(schema, "read"); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
	if err := Validate(schema, "nope"); err == nil {
		t.Fatal("want enum miss")
	}
	// []string authoring shape
	schema2 := map[string]any{"enum": []string{"a", "b"}}
	if err := Validate(schema2, "a"); err != nil {
		t.Fatalf("want pass on []string enum, got %v", err)
	}
	if err := Validate(schema2, "c"); err == nil {
		t.Fatal("want enum miss on []string enum")
	}
}

func TestValidateEnumNumericEqual(t *testing.T) {
	schema := map[string]any{"enum": []any{1, 2, 3}}
	// value is float64 (like json.Unmarshal produces)
	if err := Validate(schema, float64(2)); err != nil {
		t.Fatalf("numeric enum should treat 2 and 2.0 as equal, got %v", err)
	}
	if err := Validate(schema, 4); err == nil {
		t.Fatal("want enum miss")
	}
}

func TestValidatePointerEscaping(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a/b": map[string]any{"type": "string"},
		},
	}
	err := Validate(schema, map[string]any{"a/b": 1})
	if err == nil {
		t.Fatal("expected type error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if ve.Pointer != "/a~1b" {
		t.Errorf("pointer = %q, want /a~1b (RFC 6901 escaped)", ve.Pointer)
	}
}

func TestValidatePointerEscapingMixedSpecialChars(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a~/b": map[string]any{"type": "string"},
		},
	}
	err := Validate(schema, map[string]any{"a~/b": 1})
	if err == nil {
		t.Fatal("expected type error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if ve.Pointer != "/a~0~1b" {
		t.Errorf("pointer = %q, want /a~0~1b (RFC 6901 escaped)", ve.Pointer)
	}
}

func TestValidateUnknownPropertyOnEmptyProperties(t *testing.T) {
	// additionalProperties:false with no declared properties — every key rejected.
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
	}
	err := Validate(schema, map[string]any{"k": 1})
	if err == nil {
		t.Fatal("expected unknown-property error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if ve.Pointer != "/k" {
		t.Errorf("pointer = %q, want /k", ve.Pointer)
	}
}

func TestValidationErrorMessageRoot(t *testing.T) {
	err := (&ValidationError{Pointer: "", Message: "bad"}).Error()
	if !strings.Contains(err, "invalid params: bad") {
		t.Errorf("root message format wrong: %s", err)
	}
	err2 := (&ValidationError{Pointer: "/x", Message: "bad"}).Error()
	if !strings.Contains(err2, "/x") || !strings.Contains(err2, "bad") {
		t.Errorf("pointer message format wrong: %s", err2)
	}
}

func TestValidateDoesNotMutateSchema(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"x": map[string]any{"type": "string"}},
		"additionalProperties": false,
	}
	before := len(schema)
	_ = Validate(schema, map[string]any{"x": "y"})
	if len(schema) != before {
		t.Errorf("schema was mutated (len changed)")
	}
}
