package tools

var reportAmountEnums = []string{"EARNED", "COST", "PROFIT", "HIDE_AMOUNT", "EXPORT"}
var reportApprovalStateEnums = []string{"APPROVED", "UNAPPROVED", "ALL"}
var reportDateRangeTypeEnums = []string{"ABSOLUTE", "TODAY", "YESTERDAY", "THIS_WEEK", "LAST_WEEK", "PAST_TWO_WEEKS", "THIS_MONTH", "LAST_MONTH", "THIS_YEAR", "LAST_YEAR"}
var reportExportTypeEnums = []string{"JSON", "JSON_V1", "PDF", "CSV", "XLSX", "ZIP"}
var reportInvoicingStateEnums = []string{"INVOICED", "UNINVOICED", "ALL"}
var reportSortOrderEnums = []string{"ASCENDING", "DESCENDING"}
var reportWeekdayEnums = []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY"}
var reportZoomLevelEnums = []string{"WEEK", "MONTH", "YEAR"}
var reportContainsEnums = []string{"CONTAINS", "DOES_NOT_CONTAIN", "CONTAINS_ONLY"}
var reportArchivedStatusEnums = []string{"ACTIVE", "ARCHIVED", "ALL"}
var reportUserStatusEnums = []string{"ALL", "ACTIVE_WITH_PENDING", "ACTIVE", "PENDING", "INACTIVE"}
var reportCustomFieldTypeEnums = []string{"TXT", "NUMBER", "DROPDOWN_SINGLE", "DROPDOWN_MULTIPLE", "CHECKBOX", "LINK"}
var reportNumberConditionEnums = []string{"EQUAL", "GREATER_THAN", "LESS_THAN"}

func reportInputSchema(reportFilterKey string, reportFilterSchema map[string]any) map[string]any {
	props := reportCommonProperties()
	props[reportFilterKey] = reportFilterSchema
	return map[string]any{
		"type":       "object",
		"required":   []string{reportFilterKey},
		"properties": props,
		"anyOf": []any{
			map[string]any{"required": []string{"start", "end"}},
			map[string]any{"required": []string{"date_range_start", "date_range_end"}},
		},
	}
}

func weeklyReportInputSchema() map[string]any {
	props := reportCommonProperties()
	props["weekly_filter"] = weeklyFilterSchema()
	return map[string]any{
		"type":       "object",
		"required":   []string{"weekly_filter"},
		"properties": props,
	}
}

func expenseReportInputSchema() map[string]any {
	props := expenseReportProperties()
	return map[string]any{
		"type":       "object",
		"properties": props,
		"anyOf": []any{
			map[string]any{"required": []string{"start", "end"}},
			map[string]any{"required": []string{"date_range_start", "date_range_end"}},
		},
	}
}

func reportCommonProperties() map[string]any {
	return map[string]any{
		"amount_shown":        map[string]any{"type": "string", "enum": reportAmountEnums},
		"amounts":             map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": reportAmountEnums}},
		"approval_state":      map[string]any{"type": "string", "enum": reportApprovalStateEnums},
		"archived":            map[string]any{"type": "boolean"},
		"billable":            map[string]any{"type": "boolean"},
		"clients":             containsArchivedReportFilterSchema(),
		"currency":            containsArchivedReportFilterSchema(),
		"custom_fields":       map[string]any{"type": "array", "items": customFieldReportFilterSchema()},
		"date_format":         map[string]any{"type": "string", "description": "Date format, e.g. YYYY-MM-DD"},
		"date_range_start":    map[string]any{"type": "string", "description": "Reports API dateRangeStart, e.g. 2026-05-01T00:00:00.000"},
		"date_range_end":      map[string]any{"type": "string", "description": "Reports API dateRangeEnd, e.g. 2026-05-12T23:59:59.999"},
		"date_range_type":     map[string]any{"type": "string", "enum": reportDateRangeTypeEnums},
		"description":         map[string]any{"type": "string"},
		"end":                 map[string]any{"type": "string", "description": flexibleDatetimeDescription + ". Alias for date_range_end."},
		"export_type":         map[string]any{"type": "string", "enum": reportExportTypeEnums},
		"invoicing_state":     map[string]any{"type": "string", "enum": reportInvoicingStateEnums},
		"projects":            containsArchivedReportFilterSchema(),
		"rounding":            map[string]any{"type": "boolean"},
		"sort_order":          map[string]any{"type": "string", "enum": reportSortOrderEnums},
		"start":               map[string]any{"type": "string", "description": flexibleDatetimeDescription + ". Alias for date_range_start."},
		"tags":                tagReportFilterSchema(),
		"tasks":               containsArchivedReportFilterSchema(),
		"time_format":         map[string]any{"type": "string", "description": "Time format, e.g. THH:MM:SS.ssssss"},
		"time_zone":           timezoneInputProperty(),
		"timezone":            timezoneInputProperty(),
		"user_custom_fields":  map[string]any{"type": "array", "items": customFieldReportFilterSchema()},
		"user_groups":         containsUsersReportFilterSchema(),
		"user_locale":         map[string]any{"type": "string"},
		"users":               containsUsersReportFilterSchema(),
		"week_start":          map[string]any{"type": "string", "description": "Flexible date used to derive the exact weekly Reports API range."},
		"week_start_day":      map[string]any{"type": "string", "enum": reportWeekdayEnums, "description": "Reports API weekStart enum."},
		"without_description": map[string]any{"type": "boolean"},
		"zoom_level":          map[string]any{"type": "string", "enum": reportZoomLevelEnums},
	}
}

func expenseReportProperties() map[string]any {
	return map[string]any{
		"approval_state":   map[string]any{"type": "string", "enum": reportApprovalStateEnums},
		"billable":         map[string]any{"type": "boolean"},
		"categories":       containsArchivedReportFilterSchema(),
		"clients":          containsArchivedReportFilterSchema(),
		"currency":         containsArchivedReportFilterSchema(),
		"date_range_start": map[string]any{"type": "string", "description": "Reports API dateRangeStart, e.g. 2026-05-01T00:00:00.000"},
		"date_range_end":   map[string]any{"type": "string", "description": "Reports API dateRangeEnd, e.g. 2026-05-12T23:59:59.999"},
		"date_range_type":  map[string]any{"type": "string", "enum": reportDateRangeTypeEnums},
		"end":              map[string]any{"type": "string", "description": flexibleDatetimeDescription + ". Alias for date_range_end."},
		"export_type":      map[string]any{"type": "string", "enum": reportExportTypeEnums},
		"invoicing_state":  map[string]any{"type": "string", "enum": reportInvoicingStateEnums},
		"note":             map[string]any{"type": "string"},
		"page":             map[string]any{"type": "integer", "minimum": 1},
		"page_size":        map[string]any{"type": "integer", "minimum": 1},
		"projects":         containsArchivedReportFilterSchema(),
		"sort_column":      map[string]any{"type": "string", "enum": []string{"ID", "PROJECT", "USER", "CATEGORY", "DATE", "AMOUNT"}},
		"sort_order":       map[string]any{"type": "string", "enum": reportSortOrderEnums},
		"start":            map[string]any{"type": "string", "description": flexibleDatetimeDescription + ". Alias for date_range_start."},
		"tasks":            containsArchivedReportFilterSchema(),
		"time_zone":        timezoneInputProperty(),
		"timezone":         timezoneInputProperty(),
		"user_groups":      containsUsersReportFilterSchema(),
		"user_locale":      map[string]any{"type": "string"},
		"users":            containsUsersReportFilterSchema(),
		"week_start_day":   map[string]any{"type": "string", "enum": reportWeekdayEnums, "description": "Reports API weekStart enum."},
		"without_note":     map[string]any{"type": "boolean"},
		"zoom_level":       map[string]any{"type": "string", "enum": reportZoomLevelEnums},
	}
}

func attendanceFilterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"break_filters":    compareReportFiltersSchema(),
			"capacity_filters": compareReportFiltersSchema(),
			"end_filters":      compareReportFiltersSchema(),
			"has_time_off":     map[string]any{"type": "boolean"},
			"overtime_filters": compareReportFiltersSchema(),
			"page":             map[string]any{"type": "integer", "minimum": 1},
			"page_size":        map[string]any{"type": "integer", "minimum": 1},
			"sort_column":      map[string]any{"type": "string", "enum": []string{"USER", "DATE", "START", "END", "BREAK", "WORK", "CAPACITY", "OVERTIME", "TIME_OFF"}},
			"start_filters":    compareReportFiltersSchema(),
			"work_filters":     compareReportFiltersSchema(),
		},
	}
}

func detailedFilterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"audit_filter": auditReportFilterSchema(),
			"options":      detailedOptionsSchema(),
			"page":         map[string]any{"type": "integer", "minimum": 1},
			"page_size":    map[string]any{"type": "integer", "minimum": 1},
			"sort_column":  map[string]any{"type": "string", "enum": []string{"ID", "DESCRIPTION", "USER", "DURATION", "DATE", "ZONED_DATE", "NATURAL", "USER_DATE"}},
		},
	}
}

func summaryFilterSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"groups"},
		"properties": map[string]any{
			"groups":             map[string]any{"type": "array", "minItems": 1, "description": "1 to 3 values. DATE is the upstream group; legacy DAY input is still accepted and sent as DATE.", "items": map[string]any{"type": "string", "enum": []string{"CLIENT", "PROJECT", "TASK", "DATE", "WEEK", "MONTH", "TIMEENTRY", "USER"}}},
			"sort_column":        map[string]any{"type": "string", "enum": []string{"GROUP", "DURATION", "AMOUNT", "EARNED", "COST", "PROFIT"}},
			"summary_chart_type": map[string]any{"type": "string", "enum": []string{"BILLABILITY", "PROJECT"}},
		},
	}
}

func weeklyFilterSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"group", "subgroup"},
		"properties": map[string]any{
			"group":    map[string]any{"type": "string", "enum": []string{"PROJECT", "USER"}},
			"subgroup": map[string]any{"type": "string", "enum": []string{"TIME"}},
		},
	}
}

func containsArchivedReportFilterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"contains": map[string]any{"type": "string", "enum": reportContainsEnums},
			"ids":      stringArraySchema("Clockify IDs"),
			"status":   map[string]any{"type": "string", "enum": reportArchivedStatusEnums},
		},
	}
}

func containsUsersReportFilterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"contains": map[string]any{"type": "string", "enum": reportContainsEnums},
			"ids":      stringArraySchema("User or user group IDs"),
			"status":   map[string]any{"type": "string", "enum": reportUserStatusEnums},
		},
	}
}

func tagReportFilterSchema() map[string]any {
	props := containsArchivedReportFilterSchema()["properties"].(map[string]any)
	props["contained_in_timeentry"] = map[string]any{"type": "string", "enum": reportContainsEnums}
	return map[string]any{"type": "object", "properties": props}
}

func customFieldReportFilterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":               map[string]any{"type": "string"},
			"is_empty":         map[string]any{"type": "boolean"},
			"number_condition": map[string]any{"type": "string", "enum": reportNumberConditionEnums},
			"type":             map[string]any{"type": "string", "enum": reportCustomFieldTypeEnums},
			"value":            map[string]any{"type": "object", "additionalProperties": true},
		},
	}
}

func compareReportFiltersSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filtration_type": map[string]any{"type": "string", "enum": []string{"EXACTLY", "LARGER_THAN", "SMALLER_THAN"}},
				"value":           map[string]any{"type": "string"},
			},
		},
	}
}

func auditReportFilterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"duration":         map[string]any{"type": "integer", "minimum": 0},
			"duration_shorter": map[string]any{"type": "boolean"},
			"without_project":  map[string]any{"type": "boolean"},
			"without_task":     map[string]any{"type": "boolean"},
		},
	}
}

func detailedOptionsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"totals": map[string]any{"type": "string", "enum": []string{"CALCULATE", "EXCLUDE"}},
		},
	}
}
