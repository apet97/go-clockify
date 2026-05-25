package tools

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

var supportedSchemaKeywordFixtures = map[string]any{
	"additionalProperties": map[string]any{"type": "object", "additionalProperties": false},
	"anyOf":                map[string]any{"anyOf": []any{map[string]any{"required": []string{"name"}}}},
	"description":          map[string]any{"description": "accepted schema description"},
	"enum":                 map[string]any{"enum": []string{"safe", "strict"}},
	"format":               map[string]any{"type": "string", "format": "date-time"},
	"items":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	"maximum":              map[string]any{"type": "integer", "maximum": 10},
	"maxLength":            map[string]any{"type": "string", "maxLength": 64},
	"minimum":              map[string]any{"type": "integer", "minimum": 1},
	"minItems":             map[string]any{"type": "array", "minItems": 1},
	"minLength":            map[string]any{"type": "string", "minLength": 1},
	"pattern":              map[string]any{"type": "string", "pattern": "^[a-z]+$"},
	"properties":           map[string]any{"properties": map[string]any{"name": map[string]any{"type": "string"}}},
	"required":             map[string]any{"required": []string{"name"}},
	"title":                map[string]any{"title": "Accepted schema title"},
	"type":                 map[string]any{"type": "object"},
}

var supportedSchemaKeywords = schemaKeywordSet(supportedSchemaKeywordFixtures)

var forbiddenSchemaKeywords = map[string]bool{
	"$ref":              true,
	"$defs":             true,
	"allOf":             true,
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
	for _, d := range allToolDescriptorsForSchemaKeywordTest(t, svc) {
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
	for _, keyword := range supportedSchemaKeywordNames() {
		t.Run(keyword, func(t *testing.T) {
			schema := supportedSchemaKeywordFixtures[keyword]
			if violations := unsupportedSchemaKeywords("synthetic_tool", schema); len(violations) != 0 {
				t.Fatalf("supported keyword %q had violations: %v", keyword, violations)
			}
		})
	}
}

func allToolDescriptorsForSchemaKeywordTest(t *testing.T, svc *Service) []mcp.ToolDescriptor {
	t.Helper()
	return mustRegistry(t, svc)
}

func schemaKeywordSet(fixtures map[string]any) map[string]bool {
	out := make(map[string]bool, len(fixtures))
	for keyword := range fixtures {
		out[keyword] = true
	}
	return out
}

func supportedSchemaKeywordNames() []string {
	names := make([]string, 0, len(supportedSchemaKeywordFixtures))
	for keyword := range supportedSchemaKeywordFixtures {
		names = append(names, keyword)
	}
	sort.Strings(names)
	return names
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
		case "items", "additionalProperties", "anyOf":
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
