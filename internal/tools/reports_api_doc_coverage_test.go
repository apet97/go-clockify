package tools

import "testing"

func TestReportToolSchemasExposeOnlyTheirDocumentedFilters(t *testing.T) {
	svc := &Service{}
	tier1 := map[string]map[string]any{}
	for _, desc := range svc.Registry() {
		tier1[desc.Tool.Name] = desc.Tool.InputSchema
	}

	cases := []struct {
		tool    string
		want    string
		forbid  []string
		wantKey string
	}{
		{"clockify_attendance_report", "attendance_filter", []string{"summary_filter", "detailed_filter", "weekly_filter"}, "attendance_filter"},
		{"clockify_detailed_report", "detailed_filter", []string{"summary_filter", "attendance_filter", "weekly_filter"}, "detailed_filter"},
		{"clockify_summary_report", "summary_filter", []string{"detailed_filter", "attendance_filter", "weekly_filter"}, "summary_filter"},
		{"clockify_weekly_summary", "weekly_filter", []string{"summary_filter", "detailed_filter", "attendance_filter"}, "weekly_filter"},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			schema := tier1[tc.tool]
			if schema == nil {
				t.Fatalf("missing schema for %s", tc.tool)
			}
			props := schema["properties"].(map[string]any)
			if _, ok := props[tc.want]; !ok {
				t.Fatalf("%s missing %s in schema properties", tc.tool, tc.want)
			}
			required, _ := toStringSliceAny(schema["required"])
			if !stringSliceContains(required, tc.wantKey) {
				t.Fatalf("%s required fields = %v, want %s", tc.tool, required, tc.wantKey)
			}
			for _, key := range tc.forbid {
				if _, ok := props[key]; ok {
					t.Fatalf("%s must not expose %s", tc.tool, key)
				}
			}
		})
	}
}

func TestReportToolSchemasExposeDocumentedEnumsAndAliases(t *testing.T) {
	svc := &Service{}
	summary := schemaForTier1Tool(t, svc, "clockify_summary_report")
	summaryProps := summary["properties"].(map[string]any)
	assertEnumContains(t, summaryProps["amount_shown"].(map[string]any), "EARNED", "COST", "PROFIT")
	amountItems := summaryProps["amounts"].(map[string]any)["items"].(map[string]any)
	assertEnumContains(t, amountItems, "EARNED", "COST", "PROFIT")
	summaryFilter := summaryProps["summary_filter"].(map[string]any)["properties"].(map[string]any)
	groupItems := summaryFilter["groups"].(map[string]any)["items"].(map[string]any)
	assertEnumContains(t, groupItems, "CLIENT", "PROJECT", "TASK", "DATE", "WEEK", "MONTH", "TIMEENTRY", "USER")
	assertEnumMissing(t, groupItems, "DAY")
	assertEnumMissing(t, groupItems, "TAG")

	weekly := schemaForTier1Tool(t, svc, "clockify_weekly_summary")
	weeklyProps := weekly["properties"].(map[string]any)
	weeklyFilter := weeklyProps["weekly_filter"].(map[string]any)["properties"].(map[string]any)
	assertEnumContains(t, weeklyFilter["group"].(map[string]any), "PROJECT", "USER")
	assertEnumContains(t, weeklyFilter["subgroup"].(map[string]any), "TIME")
}

func TestExpenseReportSchemaCoversDetailedExpenseReportBody(t *testing.T) {
	svc := &Service{}
	descs, ok := svc.Tier2Handlers("expenses")
	if !ok {
		t.Fatal("expenses Tier 2 group not registered")
	}
	var schema map[string]any
	for _, desc := range descs {
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

func schemaForTier1Tool(t *testing.T, svc *Service, tool string) map[string]any {
	t.Helper()
	for _, desc := range svc.Registry() {
		if desc.Tool.Name == tool {
			return desc.Tool.InputSchema
		}
	}
	t.Fatalf("missing tool %s", tool)
	return nil
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

func assertEnumMissing(t *testing.T, schema map[string]any, wrong string) {
	t.Helper()
	values, _ := toStringSliceAny(schema["enum"])
	for _, value := range values {
		if value == wrong {
			t.Fatalf("enum %v must not contain %s", values, wrong)
		}
	}
}
