package tools

var assignmentReportGroups = []string{"USER", "PROJECT", "CLIENT", "TASK", "DATE", "WEEK", "MONTH"}

type AssignmentView map[string]any

type AssignmentDurationView struct {
	Seconds int64   `json:"seconds"`
	Hours   float64 `json:"hours"`
	Display string  `json:"display,omitempty"`
}

type AssignmentReportMoney struct {
	Earned *MoneyView `json:"earned,omitempty"`
	Cost   *MoneyView `json:"cost,omitempty"`
	Profit *MoneyView `json:"profit,omitempty"`
	Source string     `json:"source"`
	Reason string     `json:"reason,omitempty"`
}

type AssignmentReportRow struct {
	Title            string                 `json:"title"`
	GroupKey         map[string]string      `json:"group_key"`
	Entities         map[string]any         `json:"entities,omitempty"`
	Scheduled        AssignmentDurationView `json:"scheduled"`
	Available        AssignmentDurationView `json:"available"`
	AmountScheduled  *MoneyView             `json:"amount_scheduled,omitempty"`
	CostScheduled    *MoneyView             `json:"cost_scheduled,omitempty"`
	ExpectedProfit   *MoneyView             `json:"expected_profit,omitempty"`
	Tracked          AssignmentDurationView `json:"tracked"`
	AmountTracked    *MoneyView             `json:"amount_tracked,omitempty"`
	CostTracked      *MoneyView             `json:"cost_tracked,omitempty"`
	Difference       AssignmentDurationView `json:"difference"`
	AmountDifference *MoneyView             `json:"amount_difference,omitempty"`
	CostDifference   *MoneyView             `json:"cost_difference,omitempty"`
	RealizedProfit   *MoneyView             `json:"realized_profit,omitempty"`
	Status           string                 `json:"status,omitempty"`
	Financials       AssignmentReportMoney  `json:"financials"`
	Source           string                 `json:"source"`
	Warnings         []string               `json:"warnings,omitempty"`
	Raw              map[string]any         `json:"raw,omitempty"`
}

type AssignmentReportTotals struct {
	Scheduled        AssignmentDurationView `json:"scheduled"`
	Available        AssignmentDurationView `json:"available"`
	Tracked          AssignmentDurationView `json:"tracked"`
	Difference       AssignmentDurationView `json:"difference"`
	AmountScheduled  *MoneyView             `json:"amount_scheduled,omitempty"`
	CostScheduled    *MoneyView             `json:"cost_scheduled,omitempty"`
	ExpectedProfit   *MoneyView             `json:"expected_profit,omitempty"`
	AmountTracked    *MoneyView             `json:"amount_tracked,omitempty"`
	CostTracked      *MoneyView             `json:"cost_tracked,omitempty"`
	AmountDifference *MoneyView             `json:"amount_difference,omitempty"`
	CostDifference   *MoneyView             `json:"cost_difference,omitempty"`
	RealizedProfit   *MoneyView             `json:"realized_profit,omitempty"`
}

type AssignmentReportData struct {
	Range  FinancialRangeView     `json:"range"`
	Groups []string               `json:"groups"`
	Rows   []AssignmentReportRow  `json:"rows"`
	Totals AssignmentReportTotals `json:"totals"`
	Raw    map[string]any         `json:"raw,omitempty"`
}

type assignmentReportAccumulator struct {
	key              string
	groupKey         map[string]string
	entities         map[string]any
	scheduledSeconds int64
	availableSeconds int64
	trackedSeconds   int64
	amountScheduled  *MoneyView
	costScheduled    *MoneyView
	amountTracked    *MoneyView
	costTracked      *MoneyView
	expectedProfit   *MoneyView
	realizedProfit   *MoneyView
	status           string
	warnings         []string
	raw              map[string]any
}

func assignmentViewEnvelope(action string, slice bool) map[string]any {
	view := assignmentViewSchema()
	data := view
	if slice {
		data = map[string]any{"type": "array", "items": view}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"ok", "action", "data"},
		"properties": map[string]any{
			"ok":     map[string]any{"type": "boolean"},
			"action": map[string]any{"type": "string", "const": action},
			"data":   data,
			"meta": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}
}

func assignmentViewSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			"scheduled": map[string]any{"type": "object", "additionalProperties": true},
			"tracked":   map[string]any{"type": "object", "additionalProperties": true},
			"financials": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"properties": map[string]any{
					"source": map[string]any{"type": "string", "enum": []string{entryFinancialSourceReportsAPI, entryFinancialSourceDerivedRates, entryFinancialSourceUnavailable, "mixed"}},
				},
			},
			"variance": map[string]any{"type": "object", "additionalProperties": true},
			"entities": map[string]any{"type": "object", "additionalProperties": true},
			"warnings": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
}

func assignmentReportInputSchema() map[string]any {
	filter := func(desc string) map[string]any {
		return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}}
	}
	return map[string]any{
		"type":     "object",
		"required": []string{"start", "end"},
		"properties": map[string]any{
			"start":         map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
			"end":           map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
			"timezone":      timezoneInputProperty(),
			"page":          map[string]any{"type": "integer", "description": "Page number (default 1)"},
			"page_size":     map[string]any{"type": "integer", "description": "Items per page (default 50, max 200 upstream for user totals)"},
			"status_filter": map[string]any{"type": "string", "enum": []string{"PUBLISHED", "UNPUBLISHED", "ALL"}},
			"search":        map[string]any{"type": "string", "description": "Search users/projects where supported by scheduling endpoints"},
			"group_by":      map[string]any{"type": "array", "minItems": 1, "description": "1 to 3 grouping levels", "items": map[string]any{"type": "string", "enum": assignmentReportGroups}},
			"users":         filter("User IDs to include"),
			"user_groups":   filter("User group IDs to include"),
			"projects":      filter("Project IDs to include"),
			"clients":       filter("Client IDs to include"),
			"tasks":         filter("Task IDs to include"),
			"user_id":       map[string]any{"type": "string", "description": "Convenience single-user filter; ID, name, or email"},
			"project_id":    map[string]any{"type": "string", "description": "Convenience single-project filter; ID or name"},
		},
	}
}

func userScheduleTotalsInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"start", "end"},
		"properties": map[string]any{
			"start":         map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
			"end":           map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
			"timezone":      timezoneInputProperty(),
			"page":          map[string]any{"type": "integer", "description": "Page number (default 1)"},
			"page_size":     map[string]any{"type": "integer", "description": "Items per page (default 50, max 200 upstream)"},
			"search":        map[string]any{"type": "string"},
			"status_filter": map[string]any{"type": "string", "enum": []string{"PUBLISHED", "UNPUBLISHED", "ALL"}},
			"users":         stringArraySchema("User IDs to include"),
			"user_groups":   stringArraySchema("User group IDs to include"),
		},
	}
}

func singleProjectScheduleTotalsInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"project_id", "start", "end"},
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or name"},
			"start":      map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
			"end":        map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
			"timezone":   timezoneInputProperty(),
		},
	}
}
