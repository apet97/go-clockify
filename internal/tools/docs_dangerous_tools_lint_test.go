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
	catalog := loadDangerousCatalog(t)
	dangerousText := string(readDangerousToolsDoc(t))

	for _, tool := range catalog.Tools {
		if !catalogToolIsHighRisk(tool.RiskClass) {
			continue
		}
		if !strings.Contains(dangerousText, tool.Name) {
			t.Errorf("high-risk tool %q (risk_class %v) has no docs/dangerous-tools.md entry", tool.Name, tool.RiskClass)
		}
	}
}

func TestDangerousToolsDryRunColumnMatchesCatalog(t *testing.T) {
	rows := parseDangerousToolRows(t, readDangerousToolsDoc(t))
	svc := &Service{}

	for _, descriptor := range mustRegistry(t, svc) {
		if !descriptor.RiskClass.IsHighRisk() {
			continue
		}
		row, ok := rows[descriptor.Tool.Name]
		if !ok {
			continue
		}
		want := "no"
		if annotationBoolForDangerousDoc(descriptor.Tool.Annotations, "dryRun") {
			want = "yes"
		}
		if row.DryRun != want {
			t.Errorf("%s dry-run docs = %q, want %q from runtime registry metadata", descriptor.Tool.Name, row.DryRun, want)
		}
	}
}

type dangerousCatalog struct {
	Tools []struct {
		Name      string   `json:"name"`
		RiskClass []string `json:"risk_class"`
		DryRun    bool     `json:"dry_run"`
	} `json:"tools"`
}

type dangerousDocRow struct {
	DryRun string
}

func loadDangerousCatalog(t *testing.T) dangerousCatalog {
	t.Helper()
	raw, err := os.ReadFile("../../docs/tool-catalog.json")
	if err != nil {
		t.Fatalf("read tool catalog: %v", err)
	}
	var catalog dangerousCatalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatalf("parse tool catalog: %v", err)
	}
	return catalog
}

func readDangerousToolsDoc(t *testing.T) []byte {
	t.Helper()
	dangerousDoc, err := os.ReadFile("../../docs/dangerous-tools.md")
	if err != nil {
		t.Fatalf("read dangerous-tools.md: %v", err)
	}
	return dangerousDoc
}

func catalogToolIsHighRisk(risks []string) bool {
	for _, rc := range risks {
		if highRiskRiskClasses[rc] {
			return true
		}
	}
	return false
}

func parseDangerousToolRows(t *testing.T, raw []byte) map[string]dangerousDocRow {
	t.Helper()
	rows := map[string]dangerousDocRow{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `clockify_") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 6 {
			t.Fatalf("malformed dangerous-tools row: %s", line)
		}
		name := strings.Trim(strings.TrimSpace(parts[1]), "`")
		rows[name] = dangerousDocRow{DryRun: strings.ToLower(strings.TrimSpace(parts[4]))}
	}
	return rows
}

func annotationBoolForDangerousDoc(annotations map[string]any, key string) bool {
	value, _ := annotations[key].(bool)
	return value
}
