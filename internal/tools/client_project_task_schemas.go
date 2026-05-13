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

func estimateRequestInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"estimate": map[string]any{"type": "string", "description": "ISO-8601 duration, e.g. PT1H30M"},
			"type":     map[string]any{"type": "string", "enum": []string{"AUTO", "MANUAL"}},
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

func taskRequestInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"name"},
		"properties": map[string]any{
			"assignee_id":     map[string]any{"type": "string", "description": "Deprecated upstream field"},
			"assignee_ids":    stringArraySchema("Task assignee IDs"),
			"billable":        map[string]any{"type": "boolean"},
			"budget_estimate": map[string]any{"type": "integer", "minimum": 0},
			"estimate":        map[string]any{"type": "string", "description": "ISO-8601 duration, e.g. PT1H30M"},
			"name":            map[string]any{"type": "string", "minLength": 1, "maxLength": 150},
			"status":          map[string]any{"type": "string", "enum": []string{"ACTIVE", "DONE", "ALL"}},
			"user_group_ids":  stringArraySchema("Task user group IDs"),
		},
	}
}

func projectListInputSchema() map[string]any {
	props := map[string]any{
		"name":               map[string]any{"type": "string"},
		"strict_name_search": map[string]any{"type": "boolean"},
		"archived":           map[string]any{"type": "boolean"},
		"billable":           map[string]any{"type": "boolean"},
		"clients":            stringArraySchema("Client IDs to include or exclude"),
		"contains_client":    map[string]any{"type": "boolean"},
		"client_status":      map[string]any{"type": "string", "enum": []string{"ACTIVE", "ARCHIVED", "ALL"}},
		"users":              stringArraySchema("User IDs to include or exclude"),
		"contains_user":      map[string]any{"type": "boolean"},
		"user_status":        map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
		"is_template":        map[string]any{"type": "boolean"},
		"sort_column":        map[string]any{"type": "string", "enum": []string{"ID", "NAME", "CLIENT_NAME", "DURATION", "BUDGET", "PROGRESS"}},
		"sort_order":         map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
		"hydrated":           map[string]any{"type": "boolean"},
		"access":             map[string]any{"type": "string", "enum": []string{"PUBLIC", "PRIVATE"}},
		"expense_limit":      map[string]any{"type": "integer"},
		"expense_date":       map[string]any{"type": "string", "description": "yyyy-MM-dd"},
		"user_groups":        stringArraySchema("User group IDs to include or exclude"),
		"contains_group":     map[string]any{"type": "boolean"},
	}
	addFinancialRangeInputProperties(props)
	return paginationSchema(map[string]any{"properties": props})
}

func projectGetInputSchema() map[string]any {
	props := map[string]any{
		"project":                  map[string]any{"type": "string", "description": "Project name or ID"},
		"hydrated":                 map[string]any{"type": "boolean", "description": "Defaults to true so rates, memberships, and estimate fields can be returned when Clockify provides them."},
		"custom_field_entity_type": map[string]any{"type": "string", "description": "Clockify custom-field-entity-type query value, e.g. TIMEENTRY"},
		"expense_limit":            map[string]any{"type": "integer"},
		"expense_date":             map[string]any{"type": "string", "description": "yyyy-MM-dd"},
	}
	addFinancialRangeInputProperties(props)
	return map[string]any{"type": "object", "required": []string{"project"}, "properties": props}
}

func addFinancialRangeInputProperties(props map[string]any) {
	props["financial_start"] = map[string]any{"type": "string", "description": "Optional Reports API enrichment range start. Defaults to three years before request time when no financial range is supplied."}
	props["financial_end"] = map[string]any{"type": "string", "description": "Optional Reports API enrichment range end. Defaults to request time when no financial range is supplied."}
	props["financial_date_range_type"] = map[string]any{"type": "string", "enum": reportDateRangeTypeEnums, "description": "Optional Reports API dateRangeType override for project/task financial enrichment."}
	props["financial_timezone"] = timezoneInputProperty()
}

func clientListInputSchema() map[string]any {
	props := map[string]any{
		"name":        map[string]any{"type": "string"},
		"sort_column": map[string]any{"type": "string", "enum": []string{"NAME"}},
		"sort_order":  map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
		"archived":    map[string]any{"type": "string", "description": "Clockify archived filter value"},
	}
	addFinancialRangeInputProperties(props)
	return paginationSchema(map[string]any{"properties": props})
}

func clientGetInputSchema() map[string]any {
	props := map[string]any{
		"client": map[string]any{"type": "string", "description": "Client name or ID"},
	}
	addFinancialRangeInputProperties(props)
	return map[string]any{"type": "object", "required": []string{"client"}, "properties": props}
}

func clientCreateInputSchema() map[string]any {
	props := map[string]any{
		"name":    map[string]any{"type": "string"},
		"address": map[string]any{"type": "string"},
		"email":   map[string]any{"type": "string"},
		"note":    map[string]any{"type": "string"},
		"dry_run": map[string]any{"type": "boolean"},
	}
	addFinancialRangeInputProperties(props)
	return map[string]any{"type": "object", "required": []string{"name"}, "properties": props}
}

func clientUpdateInputSchema() map[string]any {
	props := map[string]any{
		"client":             map[string]any{"type": "string", "description": "Client name or ID"},
		"name":               map[string]any{"type": "string", "maxLength": 100},
		"address":            map[string]any{"type": "string", "maxLength": 256},
		"email":              map[string]any{"type": "string", "format": "email"},
		"note":               map[string]any{"type": "string", "maxLength": 500},
		"cc_emails":          stringArraySchema("Additional invoice email recipients"),
		"currency_id":        map[string]any{"type": "string"},
		"archive_projects":   map[string]any{"type": "boolean"},
		"mark_tasks_as_done": map[string]any{"type": "boolean"},
		"archived":           map[string]any{"type": "boolean"},
		"dry_run":            map[string]any{"type": "boolean"},
	}
	addFinancialRangeInputProperties(props)
	return map[string]any{"type": "object", "required": []string{"client"}, "properties": props}
}

func taskListInputSchema() map[string]any {
	props := map[string]any{
		"project":            map[string]any{"type": "string", "description": "Project name or ID"},
		"name":               map[string]any{"type": "string"},
		"strict_name_search": map[string]any{"type": "boolean"},
		"is_active":          map[string]any{"type": "boolean"},
		"sort_column":        map[string]any{"type": "string", "enum": []string{"ID", "NAME"}},
		"sort_order":         map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
	}
	addFinancialRangeInputProperties(props)
	return paginationSchema(map[string]any{
		"required":   []string{"project"},
		"properties": props,
	})
}
