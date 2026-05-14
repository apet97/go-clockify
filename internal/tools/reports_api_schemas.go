package tools

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

func containsArchivedReportFilterSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": archivedReportFilterProperties(),
	}
}

func archivedReportFilterProperties() map[string]any {
	return map[string]any{
		"contains": map[string]any{"type": "string", "enum": reportContainsEnums},
		"ids":      stringArraySchema("Clockify IDs"),
		"status":   map[string]any{"type": "string", "enum": reportArchivedStatusEnums},
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
