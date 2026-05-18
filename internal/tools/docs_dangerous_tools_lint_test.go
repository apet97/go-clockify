package tools

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// highRiskRiskClasses are the catalog risk_class values that mark a tool as
// dangerous and therefore requiring a docs/dangerous-tools.md row.
var highRiskRiskClasses = map[string]bool{
	"destructive":          true,
	"billing":              true,
	"admin":                true,
	"permission_change":    true,
	"external_side_effect": true,
}

func TestEveryHighRiskToolIsDocumentedAsDangerous(t *testing.T) {
	raw, err := os.ReadFile("../../docs/tool-catalog.json")
	if err != nil {
		t.Fatalf("read tool catalog: %v", err)
	}
	var catalog struct {
		Tools []struct {
			Name      string   `json:"name"`
			RiskClass []string `json:"risk_class"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatalf("parse tool catalog: %v", err)
	}

	dangerousDoc, err := os.ReadFile("../../docs/dangerous-tools.md")
	if err != nil {
		t.Fatalf("read dangerous-tools.md: %v", err)
	}
	dangerousText := string(dangerousDoc)

	for _, tool := range catalog.Tools {
		high := false
		for _, rc := range tool.RiskClass {
			if highRiskRiskClasses[rc] {
				high = true
				break
			}
		}
		if !high {
			continue
		}
		if !strings.Contains(dangerousText, tool.Name) {
			t.Errorf("high-risk tool %q (risk_class %v) has no docs/dangerous-tools.md entry", tool.Name, tool.RiskClass)
		}
	}
}
