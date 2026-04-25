package tools

import (
	"testing"

	"github.com/apet97/go-clockify/internal/jsonschema"
	"github.com/apet97/go-clockify/internal/mcp"
)

// TestRegistrySchemaAcceptsNaturalLanguageDatetime is the regression
// guard for the schema/handler date-time drift. Tools that document
// flexible-time start/end (e.g. clockify_add_entry, clockify_list_entries)
// have handlers using timeparse.ParseDatetime which accepts inputs like
// "now" and "today 9:00". The schema tightener must NOT add
// format:date-time to those fields, because the jsonschema validator's
// strict time.Parse(time.RFC3339, ...) would reject them before the
// handler runs.
//
// This walks the live registry, picks the descriptors that document
// flexible parsing, and verifies the validator passes a natural-language
// argument map. If a future tightener regression re-introduces
// format:date-time on a flexible field, this test reports the offending
// tool/field and the validator error.
func TestRegistrySchemaAcceptsNaturalLanguageDatetime(t *testing.T) {
	type input struct {
		toolName string
		args     map[string]any
	}
	naturalLanguageCases := []input{
		{
			toolName: "clockify_add_entry",
			// only `start` is required for add_entry.
			args: map[string]any{"start": "now"},
		},
		{
			toolName: "clockify_list_entries",
			args:     map[string]any{"start": "today 9:00", "end": "now"},
		},
		{
			toolName: "clockify_weekly_summary",
			args:     map[string]any{"week_start": "2026-04-21"}, // YYYY-MM-DD short date
		},
	}

	svc := &Service{}
	descriptors := svc.Registry()
	descByName := make(map[string]map[string]any, len(descriptors))
	for _, d := range descriptors {
		if d.Tool.InputSchema == nil {
			continue
		}
		descByName[d.Tool.Name] = d.Tool.InputSchema
	}

	for _, tc := range naturalLanguageCases {
		t.Run(tc.toolName, func(t *testing.T) {
			schema, ok := descByName[tc.toolName]
			if !ok {
				t.Fatalf("tool %s not found in registry", tc.toolName)
			}
			if err := jsonschema.Validate(schema, tc.args); err != nil {
				t.Fatalf("%s rejected natural-language args %v: %v", tc.toolName, tc.args, err)
			}
		})
	}
}

// TestRegistrySchemasAcceptHappyPathArgs is the W2-01 regression guard:
// every Tier 1 + Tier 2 tool's InputSchema must accept a synthesized
// happy-path argument map. If a future schema tightening breaks the
// agreement between the schema walker and the handler's own inputs, this
// test fails with the tool name + JSON Pointer to the offender.
//
// The synthesizer handles the keyword subset the walker actually produces
// (type, required, format:date-time/date, minimum, additionalProperties).
// A tool whose required field has a `pattern` constraint is skipped —
// generating regex-matching strings is out of scope. Such tools should be
// exercised by their own happy-path unit test.
func TestRegistrySchemasAcceptHappyPathArgs(t *testing.T) {
	svc := &Service{}

	check := func(label string, d mcp.ToolDescriptor) {
		if d.Tool.InputSchema == nil {
			return
		}
		args, skip := synthesizeHappyArgs(d.Tool.InputSchema)
		if skip {
			return
		}
		if err := jsonschema.Validate(d.Tool.InputSchema, args); err != nil {
			t.Errorf("%s: happy-path args rejected by validator: %v", label, err)
		}
	}

	for _, d := range svc.Registry() {
		check("tier1/"+d.Tool.Name, d)
	}

	for groupName := range Tier2Groups {
		descriptors, ok := svc.Tier2Handlers(groupName)
		if !ok {
			continue
		}
		for _, d := range descriptors {
			check("tier2/"+groupName+"/"+d.Tool.Name, d)
		}
	}
}

// synthesizeHappyArgs produces a minimal arguments map that should pass
// validation for the given object schema. Returns skip=true when the
// schema is outside the synthesizer's supported shape (e.g. a pattern
// on a required field).
//
// Only required fields are populated — the additionalProperties:false
// rule demands we never emit an undeclared key, but omitting optional
// keys is always allowed. Numeric bounds are respected via minimum.
func synthesizeHappyArgs(schema map[string]any) (map[string]any, bool) {
	if typ, _ := schema["type"].(string); typ != "object" {
		return map[string]any{}, false
	}
	out := map[string]any{}
	required, _ := toStringSliceAny(schema["required"])
	props, _ := schema["properties"].(map[string]any)
	for _, name := range required {
		raw, ok := props[name].(map[string]any)
		if !ok {
			// Required field with no property definition — can't synthesize.
			return nil, true
		}
		val, skip := synthesizeValue(raw)
		if skip {
			return nil, true
		}
		out[name] = val
	}
	return out, false
}

func synthesizeValue(prop map[string]any) (any, bool) {
	if _, ok := prop["pattern"]; ok {
		return nil, true // pattern synthesis is out of scope
	}
	// If the schema declares an enum, pick the first option — it's
	// guaranteed to satisfy every other keyword by definition.
	if raw, ok := prop["enum"]; ok {
		if opts, ok := raw.([]any); ok && len(opts) > 0 {
			return opts[0], false
		}
		if opts, ok := raw.([]string); ok && len(opts) > 0 {
			return opts[0], false
		}
		return nil, true
	}
	switch prop["type"] {
	case "string":
		if format, _ := prop["format"].(string); format == "date-time" {
			return "2026-04-11T09:00:00Z", false
		} else if format == "date" {
			return "2026-04-11", false
		}
		return "x", false
	case "integer":
		if raw, ok := prop["minimum"]; ok {
			if n, ok := asInt(raw); ok {
				return n, false
			}
		}
		return 1, false
	case "number":
		if raw, ok := prop["minimum"]; ok {
			if n, ok := asFloat(raw); ok {
				return n, false
			}
		}
		return float64(1), false
	case "boolean":
		return true, false
	case "array":
		return []any{}, false
	case "object":
		// Nested required object — recurse.
		sub, skip := synthesizeHappyArgs(prop)
		if skip {
			return nil, true
		}
		return sub, false
	default:
		return "x", false
	}
}

func toStringSliceAny(v any) ([]string, bool) {
	switch x := v.(type) {
	case []string:
		return x, true
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
