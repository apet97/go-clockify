package tools

import (
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

// TestTightenInputSchemaAddsAdditionalPropertiesFalse asserts the walker
// injects additionalProperties:false on every object schema that did not
// explicitly set one.
func TestTightenInputSchemaAddsAdditionalPropertiesFalse(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"nested": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"leaf": map[string]any{"type": "string"},
				},
			},
			"list": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	tightenInputSchema(schema)

	if schema["additionalProperties"] != false {
		t.Fatalf("top-level additionalProperties: %+v", schema["additionalProperties"])
	}
	nested := schema["properties"].(map[string]any)["nested"].(map[string]any)
	if nested["additionalProperties"] != false {
		t.Fatalf("nested additionalProperties: %+v", nested["additionalProperties"])
	}
	items := schema["properties"].(map[string]any)["list"].(map[string]any)["items"].(map[string]any)
	if items["additionalProperties"] != false {
		t.Fatalf("array items additionalProperties: %+v", items["additionalProperties"])
	}
}

func TestTightenInputSchemaPreservesExplicitAdditionalProperties(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties":           map[string]any{"x": map[string]any{"type": "string"}},
	}
	tightenInputSchema(schema)
	if schema["additionalProperties"] != true {
		t.Fatalf("explicit additionalProperties was clobbered: %+v", schema["additionalProperties"])
	}
}

func TestTightenInputSchemaPaginationBounds(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"page":      map[string]any{"type": "integer"},
			"page_size": map[string]any{"type": "integer"},
		},
	}
	tightenInputSchema(schema)
	page := schema["properties"].(map[string]any)["page"].(map[string]any)
	if page["minimum"] != 1 {
		t.Fatalf("page.minimum: %+v", page)
	}
	pageSize := schema["properties"].(map[string]any)["page_size"].(map[string]any)
	if pageSize["minimum"] != 1 || pageSize["maximum"] != 200 {
		t.Fatalf("page_size bounds: %+v", pageSize)
	}
}

func TestTightenInputSchemaRFC3339Format(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"start": map[string]any{"type": "string", "description": "RFC3339 timestamp"},
			"free":  map[string]any{"type": "string", "description": "Freeform text"},
		},
	}
	tightenInputSchema(schema)
	start := schema["properties"].(map[string]any)["start"].(map[string]any)
	if start["format"] != "date-time" {
		t.Fatalf("start.format: %+v", start)
	}
	free := schema["properties"].(map[string]any)["free"].(map[string]any)
	if _, set := free["format"]; set {
		t.Fatalf("free.format should be unset: %+v", free)
	}
}

// TestTightenInputSchemaSkipsFlexibleDateTime verifies that the format
// tightener does NOT add format:date-time to fields whose description
// advertises a flexible parser (natural language or YYYY-MM-DD). The
// jsonschema validator enforces format:date-time via strict
// time.Parse(time.RFC3339, ...), so adding the format to a flexible
// field would reject valid input like start="now" on clockify_add_entry
// before the handler's lenient parser ever runs.
func TestTightenInputSchemaSkipsFlexibleDateTime(t *testing.T) {
	cases := []struct {
		name        string
		description string
	}{
		{"natural_language_lowercase", "RFC3339 or natural language"},
		{"natural_language_with_examples", "RFC3339, or natural language: 'now', 'today 9:00'"},
		{"natural_language_capitalized", "RFC3339 or Natural Language"},
		{"yyyy_mm_dd", "Optional RFC3339 timestamp or YYYY-MM-DD date. Defaults to Monday of the current week in local time."},
		{"yyyy_mm_dd_lowercase", "RFC3339 or yyyy-mm-dd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"when": map[string]any{"type": "string", "description": tc.description},
				},
			}
			tightenInputSchema(schema)
			when := schema["properties"].(map[string]any)["when"].(map[string]any)
			if format, set := when["format"]; set {
				t.Fatalf("flexible-time field should not have format constraint, got %v (description: %q)", format, tc.description)
			}
		})
	}
}

// TestTightenInputSchemaPreservesExplicitFormat ensures the flexible
// detection does not let an explicit format set by the descriptor
// author get overwritten or ignored.
func TestTightenInputSchemaPreservesExplicitFormat(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"when": map[string]any{
				"type":        "string",
				"description": "RFC3339 or natural language",
				"format":      "date-time", // author opted in explicitly
			},
		},
	}
	tightenInputSchema(schema)
	when := schema["properties"].(map[string]any)["when"].(map[string]any)
	if when["format"] != "date-time" {
		t.Fatalf("explicit format should be preserved, got %+v", when)
	}
}

// TestRegistryFlexibleTimeFieldsHaveNoFormat asserts the property at
// the live-tool level: every field in the registry whose description
// advertises flexible parsing must NOT carry format:date-time after
// the tightener runs. This catches drift if a future descriptor
// rewords a description in a way the tightener no longer recognises.
func TestRegistryFlexibleTimeFieldsHaveNoFormat(t *testing.T) {
	type expect struct {
		toolName string
		field    string
	}
	flexible := []expect{
		{"clockify_list_entries", "start"},
		{"clockify_list_entries", "end"},
		{"clockify_add_entry", "start"},
		{"clockify_weekly_summary", "week_start"},
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

	for _, want := range flexible {
		schema, ok := descByName[want.toolName]
		if !ok {
			t.Errorf("tool %s not found in registry", want.toolName)
			continue
		}
		props, _ := schema["properties"].(map[string]any)
		field, ok := props[want.field].(map[string]any)
		if !ok {
			t.Errorf("%s.%s not found in schema", want.toolName, want.field)
			continue
		}
		// The tightener has already run via normalizeDescriptors() that
		// Registry() invokes; assert the flexible detection took effect.
		if format, set := field["format"]; set {
			t.Errorf("%s.%s carries format=%v but description advertises flexible parsing (description=%q)",
				want.toolName, want.field, format, field["description"])
		}
	}
}

func TestTightenInputSchemaColorPattern(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"color": map[string]any{"type": "string", "description": "Hex color code"},
		},
	}
	tightenInputSchema(schema)
	color := schema["properties"].(map[string]any)["color"].(map[string]any)
	if color["pattern"] != "^#[0-9a-fA-F]{6}$" {
		t.Fatalf("color.pattern: %+v", color)
	}
}

// TestRegistrySchemasAllHaveAdditionalPropertiesFalse is the property test
// required by the W1-10 plan: walk every Tier 1 tool's input schema and
// assert additionalProperties:false is present on every object.
func TestRegistrySchemasAllHaveAdditionalPropertiesFalse(t *testing.T) {
	svc := &Service{}
	for _, d := range svc.Registry() {
		if d.Tool.InputSchema == nil {
			continue
		}
		if err := assertNoOpenObjects(d.Tool.InputSchema, d.Tool.Name, "$"); err != nil {
			t.Errorf("tier1 %s: %v", d.Tool.Name, err)
		}
	}
}

// TestTier2SchemasAllHaveAdditionalPropertiesFalse walks every tier 2 group
// and makes the same assertion.
func TestTier2SchemasAllHaveAdditionalPropertiesFalse(t *testing.T) {
	svc := &Service{}
	for groupName := range Tier2Groups {
		descriptors, ok := svc.Tier2Handlers(groupName)
		if !ok {
			continue
		}
		for _, d := range descriptors {
			if d.Tool.InputSchema == nil {
				continue
			}
			if err := assertNoOpenObjects(d.Tool.InputSchema, d.Tool.Name, "$"); err != nil {
				t.Errorf("tier2/%s/%s: %v", groupName, d.Tool.Name, err)
			}
		}
	}
}

func assertNoOpenObjects(schema any, tool, path string) error {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	if typ, _ := m["type"].(string); typ == "object" {
		if ap, set := m["additionalProperties"]; !set || ap == nil {
			return newSchemaError(tool, path, "missing additionalProperties")
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		for name, raw := range props {
			if err := assertNoOpenObjects(raw, tool, path+"."+name); err != nil {
				return err
			}
		}
	}
	if items, ok := m["items"].(map[string]any); ok {
		if err := assertNoOpenObjects(items, tool, path+"[*]"); err != nil {
			return err
		}
	}
	return nil
}

type schemaError struct {
	tool string
	path string
	msg  string
}

func (e *schemaError) Error() string { return e.tool + " @ " + e.path + ": " + e.msg }

func newSchemaError(tool, path, msg string) error { return &schemaError{tool, path, msg} }

// sanity — Tier 2 catalog must actually populate the Tier2Groups map before
// the above test runs, otherwise we'd be walking zero groups and falsely
// passing. This assertion makes the precondition explicit.
func TestTier2CatalogPopulated(t *testing.T) {
	if len(Tier2Groups) == 0 {
		t.Fatal("Tier2Groups is empty — schema property tests would be vacuous")
	}
}

// TestTier1RegistryNonEmpty asserts the same precondition for Tier 1 —
// if Registry() ever returns zero tools, the property test above becomes
// a no-op and silently passes.
func TestTier1RegistryNonEmpty(t *testing.T) {
	svc := &Service{}
	if len(svc.Registry()) == 0 {
		t.Fatal("Tier1 Registry returned zero tools")
	}
}

// assertUsesMCPDescriptor keeps the mcp import alive for the file even if
// we drop every direct reference later.
var _ = mcp.ToolDescriptor{}
