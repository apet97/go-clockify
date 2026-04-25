package tools

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

var supportedSchemaKeywords = map[string]bool{
	"type":                 true,
	"required":             true,
	"additionalProperties": true,
	"properties":           true,
	"items":                true,
	"minimum":              true,
	"maximum":              true,
	"minLength":            true,
	"maxLength":            true,
	"pattern":              true,
	"format":               true,
	"enum":                 true,
	"description":          true,
	"title":                true,
}

var forbiddenSchemaKeywords = map[string]bool{
	"$ref":              true,
	"$defs":             true,
	"allOf":             true,
	"anyOf":             true,
	"oneOf":             true,
	"not":               true,
	"if":                true,
	"then":              true,
	"else":              true,
	"dependentSchemas":  true,
	"const":             true,
	"exclusiveMinimum":  true,
	"exclusiveMaximum":  true,
	"multipleOf":        true,
	"propertyNames":     true,
	"patternProperties": true,
}

func TestSchemaSupportedKeywords(t *testing.T) {
	svc := &Service{}
	for _, d := range allToolDescriptorsForSchemaKeywordTest(svc) {
		for _, violation := range unsupportedSchemaKeywords(d.Tool.Name, d.Tool.InputSchema) {
			t.Error(violation)
		}
	}
}

func TestSchemaSupportedKeywordsRejectsForbiddenKeyword(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"oneOf": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "integer"},
				},
			},
		},
	}

	violations := unsupportedSchemaKeywords("synthetic_tool", schema)
	if len(violations) != 1 {
		t.Fatalf("violations = %d, want 1: %v", len(violations), violations)
	}
	got := violations[0]
	if got.tool != "synthetic_tool" || got.pointer != "/properties/mode/oneOf" || got.keyword != "oneOf" {
		t.Fatalf("violation = %+v, want synthetic_tool /properties/mode/oneOf oneOf", got)
	}
}

func TestSchemaSupportedKeywordsAcceptsSupportedSubset(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"title":                "Supported schema",
		"description":          "uses only supported keywords",
		"required":             []string{"name", "items"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 64,
				"pattern":   "^[a-z]+$",
			},
			"kind": map[string]any{
				"type": "string",
				"enum": []string{"safe", "strict"},
			},
			"count": map[string]any{
				"type":    "integer",
				"minimum": 1,
				"maximum": 10,
			},
			"when": map[string]any{
				"type":   "string",
				"format": "date-time",
			},
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	if violations := unsupportedSchemaKeywords("synthetic_tool", schema); len(violations) != 0 {
		t.Fatalf("supported schema had violations: %v", violations)
	}
}

func allToolDescriptorsForSchemaKeywordTest(svc *Service) []mcp.ToolDescriptor {
	out := append([]mcp.ToolDescriptor{}, svc.Registry()...)

	groupNames := make([]string, 0, len(Tier2Groups))
	for name := range Tier2Groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	for _, groupName := range groupNames {
		descriptors, ok := svc.Tier2Handlers(groupName)
		if !ok {
			continue
		}
		out = append(out, descriptors...)
	}
	return out
}

type schemaKeywordViolation struct {
	tool    string
	pointer string
	keyword string
}

func (v schemaKeywordViolation) Error() string {
	if forbiddenSchemaKeywords[v.keyword] {
		return fmt.Sprintf("%s: forbidden JSON Schema keyword %q at %s", v.tool, v.keyword, v.pointer)
	}
	return fmt.Sprintf("%s: unsupported JSON Schema keyword %q at %s", v.tool, v.keyword, v.pointer)
}

func unsupportedSchemaKeywords(tool string, schema any) []schemaKeywordViolation {
	var violations []schemaKeywordViolation
	walkSchemaKeywords(tool, schema, "", &violations)
	return violations
}

func walkSchemaKeywords(tool string, node any, pointer string, violations *[]schemaKeywordViolation) {
	m, ok := node.(map[string]any)
	if !ok {
		return
	}

	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		nextPointer := jsonPointerJoin(pointer, key)
		if !supportedSchemaKeywords[key] {
			*violations = append(*violations, schemaKeywordViolation{
				tool:    tool,
				pointer: nextPointer,
				keyword: key,
			})
			continue
		}

		switch key {
		case "properties":
			props, ok := m[key].(map[string]any)
			if !ok {
				continue
			}
			propNames := make([]string, 0, len(props))
			for propName := range props {
				propNames = append(propNames, propName)
			}
			sort.Strings(propNames)
			for _, propName := range propNames {
				walkSchemaKeywords(tool, props[propName], jsonPointerJoin(nextPointer, propName), violations)
			}
		case "items", "additionalProperties":
			walkSchemaKeywordValue(tool, m[key], nextPointer, violations)
		}
	}
}

func walkSchemaKeywordValue(tool string, node any, pointer string, violations *[]schemaKeywordViolation) {
	switch v := node.(type) {
	case map[string]any:
		walkSchemaKeywords(tool, v, pointer, violations)
	case []any:
		for i, item := range v {
			walkSchemaKeywordValue(tool, item, jsonPointerJoin(pointer, fmt.Sprintf("%d", i)), violations)
		}
	}
}

func jsonPointerJoin(base, token string) string {
	escaped := escapeJSONPointerToken(token)
	if base == "" {
		return "/" + escaped
	}
	return base + "/" + escaped
}

func escapeJSONPointerToken(token string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(token)
}
