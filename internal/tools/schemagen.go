package tools

import (
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
)

// schemaTypeCache memoises schemaForType so the reflection walk runs
// at most once per distinct (post-deref) reflect.Type per process. Two
// observations make this safe:
//   - The output JSON Schema is a pure function of the post-deref type;
//     given the same reflect.Type the result is byte-identical across
//     every call.
//   - All consumers embed the returned map as a property of a fresh outer map
//     and never mutate it afterwards.
//
// This collapses repeated typed-schema generation to a single
// per-binary cost; subsequent sessions inherit the result with zero
// allocations.
//
// Returned maps are shared and MUST NOT be mutated; the comment is
// load-bearing because the only way to violate the invariant is a
// future caller adding a post-construction edit.
var schemaTypeCache sync.Map // reflect.Type → map[string]any

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
//
// Results are memoised in schemaTypeCache keyed by the post-deref type
// so *T and T share a cache entry. Recursive callers (slice, map,
// struct) re-enter through schemaForType and inherit the cache.
func schemaForType(t reflect.Type) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if cached, ok := schemaTypeCache.Load(t); ok {
		if schema, ok := cached.(map[string]any); ok {
			return schema
		}
	}
	schema := computeSchemaForType(t)
	actual, _ := schemaTypeCache.LoadOrStore(t, schema)
	if cached, ok := actual.(map[string]any); ok {
		return cached
	}
	return schema
}

// computeSchemaForType performs the actual reflection walk. Callers
// must dereference pointers and consult schemaTypeCache first; this
// helper exists so the cache-miss branch stays a single expression.
func computeSchemaForType(t reflect.Type) map[string]any {
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

func envelopeOpenMap(action string) map[string]any {
	return envelopeSchemaFor[map[string]any](action)
}

func envelopeOpenMapSlice(action string) map[string]any {
	return envelopeSchemaFor[[]map[string]any](action)
}

// withOutputSchema returns a copy of t with OutputSchema set. Used by
// tool registrations that want to attach a schema without changing the
// existing toolRO/toolRW/toolDestructive helper signatures.
func withOutputSchema(t mcp.Tool, schema map[string]any) mcp.Tool {
	t.OutputSchema = schema
	return t
}
