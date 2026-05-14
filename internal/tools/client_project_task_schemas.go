package tools

func stringArraySchema(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func rateRequestInputSchema(desc string) map[string]any {
	return map[string]any{
		"type":        "object",
		"description": desc,
		"required":    []string{"amount"},
		"properties": map[string]any{
			"amount": map[string]any{"type": "integer", "minimum": 0, "description": "Raw upstream integer amount; no currency scaling is applied"},
			"since":  map[string]any{"type": "string", "description": "Clockify yyyy-MM-ddThh:mm:ssZ effective timestamp"},
		},
	}
}

func estimateWithOptionsInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"active":           map[string]any{"type": "boolean"},
			"estimate":         map[string]any{"type": "integer", "minimum": 0},
			"include_expenses": map[string]any{"type": "boolean"},
			"reset_option":     map[string]any{"type": "string", "enum": []string{"WEEKLY", "MONTHLY", "YEARLY"}},
			"type":             map[string]any{"type": "string", "enum": []string{"AUTO", "MANUAL"}},
		},
	}
}

func timeEstimateInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"active":               map[string]any{"type": "boolean"},
			"estimate":             map[string]any{"type": "string", "description": "ISO-8601 duration, e.g. PT1H30M"},
			"include_non_billable": map[string]any{"type": "boolean"},
			"reset_option":         map[string]any{"type": "string", "enum": []string{"WEEKLY", "MONTHLY", "YEARLY"}},
			"type":                 map[string]any{"type": "string", "enum": []string{"AUTO", "MANUAL"}},
		},
	}
}

func estimateResetInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"active":       map[string]any{"type": "boolean"},
			"day_of_month": map[string]any{"type": "integer", "minimum": 1, "maximum": 31},
			"day_of_week":  map[string]any{"type": "string", "enum": []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY"}},
			"hour":         map[string]any{"type": "integer", "minimum": 0, "maximum": 23},
			"interval":     map[string]any{"type": "string", "enum": []string{"WEEKLY", "MONTHLY", "YEARLY"}},
			"is_active":    map[string]any{"type": "boolean"},
			"month":        map[string]any{"type": "string", "enum": []string{"JANUARY", "FEBRUARY", "MARCH", "APRIL", "MAY", "JUNE", "JULY", "AUGUST", "SEPTEMBER", "OCTOBER", "NOVEMBER", "DECEMBER"}},
		},
	}
}

func membershipInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"user_id":           map[string]any{"type": "string"},
			"hourly_rate":       rateRequestInputSchema("Per-member hourly rate"),
			"cost_rate":         rateRequestInputSchema("Per-member cost rate"),
			"membership_status": map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
			"membership_type":   map[string]any{"type": "string", "enum": []string{"WORKSPACE", "PROJECT", "USERGROUP"}},
		},
	}
}

func userGroupsInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"contains": map[string]any{"type": "string", "enum": []string{"CONTAINS", "DOES_NOT_CONTAIN"}},
			"ids":      stringArraySchema("User group IDs"),
			"status":   map[string]any{"type": "string", "enum": []string{"ALL", "ACTIVE", "INACTIVE"}},
		},
	}
}

func addFinancialRangeInputProperties(props map[string]any) {
	props["financial_start"] = map[string]any{"type": "string", "description": "Optional Reports API enrichment range start. Defaults to three years before request time when no financial range is supplied."}
	props["financial_end"] = map[string]any{"type": "string", "description": "Optional Reports API enrichment range end. Defaults to request time when no financial range is supplied."}
	props["financial_date_range_type"] = map[string]any{"type": "string", "enum": reportDateRangeTypeEnums, "description": "Optional Reports API dateRangeType override for project/task financial enrichment."}
	props["financial_timezone"] = timezoneInputProperty()
}
