package tools

import "github.com/apet97/go-clockify/internal/mcp"

// applyOpaqueOutputSchemas gives every descriptor that lacks an
// outputSchema a generic envelopeOpaque schema keyed by tool name. Used
// by the domain-handler families so every tool advertises at least the
// envelope wrapper without hand-crafting per-tool typed schemas.
func applyOpaqueOutputSchemas(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	for i := range in {
		if in[i].Tool.OutputSchema == nil {
			in[i].Tool.OutputSchema = envelopeOpaque(in[i].Tool.Name)
		}
	}
	return in
}

func reportOutputSchema(action string, dataProps map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"ok", "action", "data"},
		"properties": map[string]any{
			"ok":     map[string]any{"type": "boolean"},
			"action": map[string]any{"type": "string", "const": action},
			"data": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"properties":           dataProps,
			},
			"meta": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}
}

func detailedReportNormalizedProperties() map[string]any {
	return map[string]any{
		"raw":              map[string]any{"type": "object", "additionalProperties": true},
		"entries":          schemaFor[[]ReportEntryView](),
		"entry_summary":    schemaFor[ReportEntrySummary](),
		"approval_summary": schemaFor[ReportApprovalSummary](),
		"totals_summary":   schemaFor[ReportTotalsSummary](),
		"entity_summary":   schemaFor[ReportEntitySummary](),
		"client_summary":   schemaFor[[]ReportClientSummary](),
		"suggestedActions": schemaFor[[]ToolSuggestion](),
	}
}

func summaryReportNormalizedProperties() map[string]any {
	return map[string]any{
		"raw":                  map[string]any{"type": "object", "additionalProperties": true},
		"summary_rollups":      openObjectArraySchema(),
		"group_totals_summary": schemaFor[ReportGroupTotalsSummary](),
		"donut_chart_summary":  schemaFor[[]DonutChartSegmentView](),
		"totals_summary":       schemaFor[ReportTotalsSummary](),
		"suggestedActions":     schemaFor[[]ToolSuggestion](),
	}
}

func weeklyReportNormalizedProperties() map[string]any {
	return map[string]any{
		"raw":               map[string]any{"type": "object", "additionalProperties": true},
		"weekly_rollups":    openObjectArraySchema(),
		"weekly_day_totals": schemaFor[[]WeeklyDayTotalView](),
		"totals_summary":    schemaFor[ReportTotalsSummary](),
		"suggestedActions":  schemaFor[[]ToolSuggestion](),
	}
}

func moneyReportOutputSchema(action string) map[string]any {
	return reportOutputSchema(action, map[string]any{
		"range":                schemaFor[FinancialRangeView](),
		"group_by":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"rollups":              openObjectArraySchema(),
		"group_totals_summary": schemaFor[ReportGroupTotalsSummary](),
		"totals_summary":       schemaFor[ReportTotalsSummary](),
		"suggestedActions":     schemaFor[[]ToolSuggestion](),
		"warnings":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"raw":                  map[string]any{"type": "object", "additionalProperties": true},
	})
}

func monthlyBriefOutputSchema() map[string]any {
	return reportOutputSchema("clockify_reports_summary", map[string]any{
		"month":            map[string]any{"type": "string"},
		"range":            schemaFor[FinancialRangeView](),
		"money":            map[string]any{"type": "object", "additionalProperties": true},
		"audit":            map[string]any{"type": "object", "additionalProperties": true},
		"suggestedActions": schemaFor[[]ToolSuggestion](),
		"warnings":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	})
}

func openObjectArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
	}
}
