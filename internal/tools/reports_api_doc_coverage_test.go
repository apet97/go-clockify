package tools

import "testing"

func TestReportRouteSchemasExposeCurrentOneUserFields(t *testing.T) {
	svc := &Service{}
	descriptors := map[string]map[string]any{}
	for _, desc := range svc.FullAccessRegistry() {
		descriptors[desc.Tool.Name] = desc.Tool.InputSchema
	}

	for _, tool := range []string{
		"clockify_reports_attendance",
		"clockify_reports_money",
		"clockify_reports_expense",
		"clockify_reports_export",
	} {
		t.Run(tool, func(t *testing.T) {
			props := schemaProperties(t, descriptors[tool], tool)
			for _, key := range []string{"date_range_start", "date_range_end", "start", "end", "export_type"} {
				if _, ok := props[key]; !ok {
					t.Fatalf("%s missing current report property %s", tool, key)
				}
			}
			for _, key := range []string{"summary_filter", "detailed_filter", "attendance_filter"} {
				if _, ok := props[key]; !ok {
					t.Fatalf("%s missing current report filter property %s", tool, key)
				}
			}
			for _, stale := range []string{"weekly_filter", "body"} {
				if _, ok := props[stale]; ok {
					t.Fatalf("%s must not expose removed legacy report filter %s", tool, stale)
				}
			}
		})
	}

	for _, tool := range []string{
		"clockify_reports_detailed",
		"clockify_reports_summary",
		"clockify_reports_weekly",
	} {
		t.Run(tool, func(t *testing.T) {
			props := schemaProperties(t, descriptors[tool], tool)
			for _, key := range []string{"start", "end", "project", "timezone"} {
				if _, ok := props[key]; !ok {
					t.Fatalf("%s missing current helper property %s", tool, key)
				}
			}
		})
	}
	weeklyProps := schemaProperties(t, descriptors["clockify_reports_weekly"], "clockify_reports_weekly")
	if _, ok := weeklyProps["week_start"]; !ok {
		t.Fatal("clockify_reports_weekly missing week_start")
	}
}

func schemaProperties(t *testing.T, schema map[string]any, tool string) map[string]any {
	t.Helper()
	if schema == nil {
		t.Fatalf("missing schema for %s", tool)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s properties = %T", tool, schema["properties"])
	}
	return props
}
