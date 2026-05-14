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
			for _, key := range []string{"date_range_start", "date_range_end", "start", "end", "export_type", "body"} {
				if _, ok := props[key]; !ok {
					t.Fatalf("%s missing current report route property %s", tool, key)
				}
			}
			for _, stale := range []string{"summary_filter", "detailed_filter", "attendance_filter", "weekly_filter"} {
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

func TestExpenseReportSchemaCoversDetailedExpenseReportBody(t *testing.T) {
	svc := &Service{}
	var schema map[string]any
	for _, desc := range expenseHandlers(svc) {
		if desc.Tool.Name == "clockify_expense_report" {
			schema = desc.Tool.InputSchema
			break
		}
	}
	if schema == nil {
		t.Fatal("clockify_expense_report schema missing")
	}
	props := schema["properties"].(map[string]any)
	for _, key := range []string{
		"approval_state", "billable", "categories", "clients", "currency",
		"date_range_start", "date_range_end", "date_range_type", "export_type",
		"invoicing_state", "note", "page", "page_size", "projects", "sort_column",
		"sort_order", "tasks", "time_zone", "user_groups", "user_locale", "users",
		"week_start_day", "without_note", "zoom_level",
	} {
		if _, ok := props[key]; !ok {
			t.Fatalf("clockify_expense_report missing documented property %s", key)
		}
	}
	for _, wrong := range []string{"summary_filter", "detailed_filter", "attendance_filter", "weekly_filter", "amount_shown", "amounts", "tags", "custom_fields", "user_custom_fields"} {
		if _, ok := props[wrong]; ok {
			t.Fatalf("clockify_expense_report must not expose %s", wrong)
		}
	}
	assertEnumContains(t, props["sort_column"].(map[string]any), "ID", "PROJECT", "USER", "CATEGORY", "DATE", "AMOUNT")
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

func assertEnumContains(t *testing.T, schema map[string]any, wants ...string) {
	t.Helper()
	values, _ := toStringSliceAny(schema["enum"])
	have := map[string]bool{}
	for _, value := range values {
		have[value] = true
	}
	for _, want := range wants {
		if !have[want] {
			t.Fatalf("enum %v missing %s", values, want)
		}
	}
}
