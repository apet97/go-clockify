package tools

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// catalogToolNames loads the set of tool names from docs/tool-catalog.json.
func catalogToolNames(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("../../docs/tool-catalog.json")
	if err != nil {
		t.Fatalf("read tool catalog: %v", err)
	}
	var catalog struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatalf("parse tool catalog: %v", err)
	}
	names := make(map[string]bool, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		names[tool.Name] = true
	}
	if len(names) == 0 {
		t.Fatal("tool catalog has no tools")
	}
	return names
}

var clockifyToolToken = regexp.MustCompile(`clockify_[a-z0-9_]+`)

// docsToolNameExceptions are clockify_* tokens that appear in docs as prose
// patterns or families, not as a single concrete tool name.
var docsToolNameExceptions = map[string]bool{
	"clockify_users_role":                     true, // also a real tool — kept for clarity
	"clockify_invoices_":                      true, // family pattern in prose
	"clockify_expenses_":                      true, // family pattern in prose
	"clockify_groups_":                        true, // family pattern in prose
	"clockify_entries_":                       true, // family pattern in prose
	"clockify_projects_":                      true, // family pattern in prose
	"clockify_tasks_":                         true, // family pattern in prose
	"clockify_tags_":                          true, // family pattern in prose
	"clockify_reports_":                       true, // family pattern in prose
	"clockify_review_":                        true, // family pattern in prose
	"clockify_scheduling_":                    true, // family pattern in prose
	"clockify_approvals_":                     true, // family pattern in prose
	"clockify_webhooks_":                      true, // family pattern in prose
	"clockify_users_":                         true, // family pattern in prose
	"clockify_custom_fields_":                 true, // family pattern in prose
	"clockify_holidays_":                      true, // family pattern in prose
	"clockify_time_off_":                      true, // family pattern in prose
	"clockify_workspace_settings":             true,
	"clockify_upstream_requests_total":        true, // metric name
	"clockify_upstream_retries_total":         true, // metric name
	"clockify_mcp_tool_call_duration_seconds": true, // metric name
}

func TestActiveDocsToolNamesExistInCatalog(t *testing.T) {
	names := catalogToolNames(t)
	walkActiveDocs(t, func(path string, content []byte) {
		for _, token := range clockifyToolToken.FindAllString(string(content), -1) {
			if names[token] || docsToolNameExceptions[token] {
				continue
			}
			t.Errorf("%s references unknown tool %q (not in docs/tool-catalog.json)", path, token)
		}
	})
}
