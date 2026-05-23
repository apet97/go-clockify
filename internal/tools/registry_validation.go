package tools

import (
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
)

func ValidateRegistry(reg []mcp.ToolDescriptor) error {
	seen := map[string]int{}
	rawStarted := false
	for i, d := range reg {
		name := strings.TrimSpace(d.Tool.Name)
		if name == "" {
			return fmt.Errorf("tool at index %d has empty name", i)
		}
		if prev, ok := seen[name]; ok {
			return fmt.Errorf("duplicate tool name %q at index %d, first seen at %d", name, i, prev)
		}
		seen[name] = i
		if strings.TrimSpace(d.Tool.Title) == "" {
			return fmt.Errorf("%s missing title", name)
		}
		if strings.TrimSpace(d.Tool.Description) == "" {
			return fmt.Errorf("%s missing description", name)
		}
		if !objectSchemaType(d.Tool.InputSchema) {
			return fmt.Errorf("%s missing object input schema", name)
		}
		if !objectSchemaType(d.Tool.OutputSchema) {
			return fmt.Errorf("%s missing object output schema", name)
		}
		if d.Handler == nil {
			return fmt.Errorf("%s missing handler", name)
		}
		if !validRiskClass(d.RiskClass) {
			return fmt.Errorf("%s missing or invalid risk metadata", name)
		}
		if isRawToolName(name) {
			rawStarted = true
		} else if rawStarted {
			return fmt.Errorf("raw tools must be last: %s appears after raw bucket", name)
		}
		for _, tier := range d.Tiers {
			if !validToolTier(tier) {
				return fmt.Errorf("%s has invalid tier %q", name, tier)
			}
		}
	}
	return nil
}

func objectSchemaType(schema map[string]any) bool {
	if schema == nil {
		return false
	}
	return schema["type"] == "object"
}

func isRawToolName(name string) bool {
	return name == "clockify_api_get" || name == "clockify_api_request"
}

func validToolTier(tier string) bool {
	switch tier {
	case "default", "core", "business", "admin":
		return true
	default:
		return false
	}
}

func validRiskClass(risk mcp.RiskClass) bool {
	const known = mcp.RiskRead |
		mcp.RiskWrite |
		mcp.RiskSensitiveRead |
		mcp.RiskBilling |
		mcp.RiskAdmin |
		mcp.RiskPermissionChange |
		mcp.RiskExternalSideEffect |
		mcp.RiskDestructive
	return risk != 0 && risk&^known == 0
}
