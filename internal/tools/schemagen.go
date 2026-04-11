package tools

import (
	"reflect"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
)

// schemaFor produces a JSON Schema (Draft 2020-12 subset) describing the
// shape of T. The generator is reflection-based and stdlib-only — no
// third-party dependencies — so the server core stays self-contained.
//
// Coverage:
//   - string                    {type: string}
//   - bool                      {type: boolean}
//   - int / int64 / uint*       {type: integer}
//   - float32 / float64         {type: number}
//   - time.Time                 {type: string, format: date-time}
//   - struct{ ... }             {type: object, properties: {...},
//     additionalProperties: false}
//   - *T (pointer)              schemaFor[T]
//   - []T                       {type: array, items: schemaFor[T]}
//   - map[string]T              {type: object, additionalProperties: schemaFor[T]}
//   - any / interface{}         {} (no constraint)
//
// The generator honours the `json:"name,omitempty"` tag for field naming
// and treats fields without `omitempty` as required. Unexported fields
// and fields with `json:"-"` are skipped.
func schemaFor[T any]() map[string]any {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		// T is interface{} — no constraint we can safely express.
		return map[string]any{}
	}
	return schemaForType(t)
}

// schemaForType is the reflection workhorse. Exported only for tests in
// the same package; not part of the public API.
func schemaForType(t reflect.Type) map[string]any {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// time.Time is the one named struct that is treated as a primitive.
	if t == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{
			"type":  "array",
			"items": schemaForType(t.Elem()),
		}
	case reflect.Map:
		// Only string-keyed maps round-trip cleanly through JSON.
		return map[string]any{
			"type":                 "object",
			"additionalProperties": schemaForType(t.Elem()),
		}
	case reflect.Interface:
		// Untyped value — no constraint.
		return map[string]any{}
	case reflect.Struct:
		return structSchema(t)
	default:
		return map[string]any{}
	}
}

// structSchema walks a struct's exported fields and emits properties +
// required arrays based on the json tags.
func structSchema(t reflect.Type) map[string]any {
	props := map[string]any{}
	required := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name, optional, skip := jsonFieldName(field)
		if skip {
			continue
		}
		props[name] = schemaForType(field.Type)
		if !optional {
			required = append(required, name)
		}
	}

	out := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

// jsonFieldName parses the json tag on a struct field and returns the
// effective field name, whether ,omitempty is set (treated as optional),
// and whether the field should be skipped entirely (json:"-" or no tag
// + lowercase Go name).
func jsonFieldName(field reflect.StructField) (name string, optional, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	if tag == "" {
		// No tag: fall back to the Go field name. encoding/json would
		// use the field name verbatim for exported fields.
		return field.Name, false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = field.Name
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			optional = true
		}
	}
	return name, optional, false
}

// envelopeSchemaFor produces an outputSchema for a tool whose Data field
// is a typed struct T. The shape mirrors ResultEnvelope verbatim so MCP
// clients can validate every tool result against a strongly-typed schema.
//
// `action` is bound as a JSON Schema `const` so the schema doubles as a
// dispatch hint — clients that branch on action no longer need to scan
// the value at runtime.
func envelopeSchemaFor[T any](action string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"ok", "action", "data"},
		"properties": map[string]any{
			"ok":     map[string]any{"type": "boolean"},
			"action": map[string]any{"type": "string", "const": action},
			"data":   schemaFor[T](),
			"meta": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}
}

// envelopeOpaque produces an outputSchema for tools whose Data field is
// an open-shape map[string]any (most Tier 2 CRUD wrappers). It still
// pins the action field as a const and validates the envelope wrapper,
// while leaving the data payload unconstrained.
func envelopeOpaque(action string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"ok", "action"},
		"properties": map[string]any{
			"ok":     map[string]any{"type": "boolean"},
			"action": map[string]any{"type": "string", "const": action},
			"data": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
			"meta": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}
}

// withOutputSchema returns a copy of t with OutputSchema set. Used by
// tool registrations that want to attach a schema without changing the
// existing toolRO/toolRW/toolDestructive helper signatures.
func withOutputSchema(t mcp.Tool, schema map[string]any) mcp.Tool {
	t.OutputSchema = schema
	return t
}
