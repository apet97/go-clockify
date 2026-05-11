package tools

import (
	"fmt"
	"testing"
)

// TestApplyPropertyConstraintsAddsMaxLength is a focused unit test for the
// schema tightener: feed it a tiny synthetic schema and confirm the
// configured ceilings land on the right properties. This is the quick
// signal when the central freeTextMaxLength table is edited; the
// registry-walking property test below catches descriptor-level drift.
func TestApplyPropertyConstraintsAddsMaxLength(t *testing.T) {
	for field, want := range freeTextMaxLength {
		t.Run(field, func(t *testing.T) {
			prop := map[string]any{"type": "string"}
			applyPropertyConstraints(field, prop)
			got, ok := prop["maxLength"]
			if !ok {
				t.Fatalf("expected maxLength on %s, got prop=%+v", field, prop)
			}
			if got != want {
				t.Fatalf("%s maxLength = %v, want %d", field, got, want)
			}
		})
	}
}

// TestApplyPropertyConstraintsPreservesExplicitMaxLength asserts the
// central table never clobbers a handler-side override. Handlers are
// allowed to declare a stricter limit when the field has known per-tool
// semantics (e.g. webhook name is 30 chars, not 150).
func TestApplyPropertyConstraintsPreservesExplicitMaxLength(t *testing.T) {
	prop := map[string]any{"type": "string", "maxLength": 30}
	applyPropertyConstraints("name", prop)
	if prop["maxLength"] != 30 {
		t.Fatalf("explicit maxLength=30 was clobbered: %+v", prop)
	}
}

// TestApplyPropertyConstraintsSkipsNonStringFields asserts the ceiling
// table only fires on string properties; integer/boolean siblings with
// the same name are left alone.
func TestApplyPropertyConstraintsSkipsNonStringFields(t *testing.T) {
	prop := map[string]any{"type": "integer"}
	applyPropertyConstraints("name", prop)
	if _, ok := prop["maxLength"]; ok {
		t.Fatalf("maxLength applied to non-string property: %+v", prop)
	}
}

// TestRegistryFreeTextFieldsHaveMaxLength is the registry-wide property
// test: walk every Tier 1 + Tier 2 descriptor, find every property whose
// name matches the central freeTextMaxLength table, and assert it
// carries maxLength after normalization. The test fails when a maxLength
// is absent OR when a maxLength exceeds the central ceiling, so a
// reviewer cannot silently slacken one tool by hand.
func TestRegistryFreeTextFieldsHaveMaxLength(t *testing.T) {
	svc := &Service{}
	for _, d := range svc.Registry() {
		if d.Tool.InputSchema == nil {
			continue
		}
		for _, v := range walkFreeTextViolations(d.Tool.Name, d.Tool.InputSchema) {
			t.Errorf("tier1 %s: %s", d.Tool.Name, v)
		}
	}
	for groupName := range Tier2Groups {
		descriptors, ok := svc.Tier2Handlers(groupName)
		if !ok {
			continue
		}
		for _, d := range descriptors {
			if d.Tool.InputSchema == nil {
				continue
			}
			for _, v := range walkFreeTextViolations(d.Tool.Name, d.Tool.InputSchema) {
				t.Errorf("tier2/%s/%s: %s", groupName, d.Tool.Name, v)
			}
		}
	}
}

// walkFreeTextViolations descends an MCP tool input schema and collects
// human-readable violations of the central maxLength contract: a
// free-text string field with no maxLength, or one whose maxLength
// exceeds the ceiling. Returned strings carry the JSON-pointer-ish
// path so a reviewer can locate the offender by eye.
func walkFreeTextViolations(toolName string, schema map[string]any) []string {
	var out []string
	walkSchemaFields(schema, "$", func(path, name string, prop map[string]any) {
		ceil, ok := freeTextMaxLength[name]
		if !ok {
			return
		}
		typ, _ := prop["type"].(string)
		if typ != "string" {
			return
		}
		got, set := prop["maxLength"]
		if !set {
			out = append(out, fmt.Sprintf("%s.%s missing maxLength (expected ≤ %d)", path, name, ceil))
			return
		}
		switch v := got.(type) {
		case int:
			if v > ceil {
				out = append(out, fmt.Sprintf("%s.%s maxLength=%d exceeds central ceiling %d", path, name, v, ceil))
			}
		case int64:
			if v > int64(ceil) {
				out = append(out, fmt.Sprintf("%s.%s maxLength=%d exceeds central ceiling %d", path, name, v, ceil))
			}
		case float64:
			if int(v) > ceil {
				out = append(out, fmt.Sprintf("%s.%s maxLength=%v exceeds central ceiling %d", path, name, v, ceil))
			}
		}
	})
	return out
}

// walkSchemaFields recurses into objects, arrays (items), and anyOf
// subschemas, invoking visit for every (parent path, property name,
// property schema) triple. Mirrors the recursion in tightenInputSchema.
func walkSchemaFields(schema map[string]any, path string, visit func(path, name string, prop map[string]any)) {
	if schema == nil {
		return
	}
	if typ, _ := schema["type"].(string); typ == "object" {
		if props, ok := schema["properties"].(map[string]any); ok {
			for name, raw := range props {
				prop, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				visit(path, name, prop)
				walkSchemaFields(prop, path+"."+name, visit)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		walkSchemaFields(items, path+"[]", visit)
	}
	if options, ok := schema["anyOf"].([]any); ok {
		for i, option := range options {
			if sub, ok := option.(map[string]any); ok {
				walkSchemaFields(sub, fmt.Sprintf("%s|anyOf[%d]", path, i), visit)
			}
		}
	}
}
