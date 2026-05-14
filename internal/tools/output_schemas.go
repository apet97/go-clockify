package tools

import (
	"sync"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// tier1OutputSchemas returns the outputSchema map keyed by tool name for
// the first-slice tools. Splitting the schema lookup out of registry.go
// keeps the inline tool table compact and lets the schema sweep be
// reviewed in isolation.
//
// Tools whose handlers return a typed Go struct (SummaryData, LogTimeData,
// clockify.TimeEntry, etc.) get a schemaFor[T]-driven envelope. Tools that
// return open shapes (map[string]any from helper builders, internal status
// structs, etc.) get envelopeOpaque, which still pins ok/action/data and
// makes the action a JSON Schema const so MCP clients can dispatch on it.
//
// The map and every envelope value inside it are built once per process
// via sync.OnceValue so the reflection-heavy envelopeSchemaFor[T] path
// runs at most once. Callers MUST NOT mutate the returned maps; the
// existing assignment-only usage in applyTier1OutputSchemas preserves
// that invariant.
var tier1OutputSchemas = sync.OnceValue(buildTier1OutputSchemas)

func buildTier1OutputSchemas() map[string]map[string]any {
	return map[string]map[string]any{
		// --- typed-data tools ---
		"clockify_whoami":                        envelopeSchemaFor[IdentityData]("clockify_whoami"),
		"clockify_current_user":                  envelopeSchemaFor[UserView]("clockify_current_user"),
		"clockify_get_workspace":                 envelopeSchemaFor[WorkspaceView]("clockify_get_workspace"),
		"clockify_list_workspaces":               envelopeSchemaFor[[]WorkspaceView]("clockify_list_workspaces"),
		"clockify_list_users":                    envelopeSchemaFor[[]UserView]("clockify_list_users"),
		"clockify_list_projects":                 envelopeSchemaFor[[]ProjectView]("clockify_list_projects"),
		"clockify_get_project":                   envelopeSchemaFor[ProjectView]("clockify_get_project"),
		"clockify_get_client":                    envelopeSchemaFor[ClientView]("clockify_get_client"),
		"clockify_list_clients":                  envelopeSchemaFor[[]ClientView]("clockify_list_clients"),
		"clockify_client_report":                 envelopeSchemaFor[ClientReportView]("clockify_client_report"),
		"clockify_list_tags":                     envelopeSchemaFor[[]clockify.Tag]("clockify_list_tags"),
		"clockify_get_tag":                       envelopeSchemaFor[clockify.Tag]("clockify_get_tag"),
		"clockify_update_tag":                    envelopeSchemaFor[clockify.Tag]("clockify_update_tag"),
		"clockify_list_tasks":                    envelopeSchemaFor[[]TaskView]("clockify_list_tasks"),
		"clockify_get_task":                      envelopeSchemaFor[TaskView]("clockify_get_task"),
		"clockify_update_task":                   envelopeSchemaFor[TaskView]("clockify_update_task"),
		"clockify_list_entries":                  envelopeSchemaFor[[]EntryView]("clockify_list_entries"),
		"clockify_get_entry":                     envelopeSchemaFor[EntryView]("clockify_get_entry"),
		"clockify_today_entries":                 envelopeSchemaFor[[]EntryView]("clockify_today_entries"),
		"clockify_list_in_progress_time_entries": envelopeSchemaFor[[]EntryView]("clockify_list_in_progress_time_entries"),
		"clockify_summary_report":                reportOutputSchema("clockify_summary_report", summaryReportNormalizedProperties()),
		"clockify_weekly_summary":                reportOutputSchema("clockify_weekly_summary", weeklyReportNormalizedProperties()),
		"clockify_quick_report":                  envelopeSchemaFor[QuickReportData]("clockify_quick_report"),
		legacyToolTimesheetReview:                envelopeSchemaFor[TimesheetReviewData](legacyToolTimesheetReview),
		"clockify_attendance_report":             envelopeOpaque("clockify_attendance_report"),
		"clockify_detailed_report":               reportOutputSchema("clockify_detailed_report", detailedReportNormalizedProperties()),
		legacyToolLogTime:                        envelopeSchemaFor[LogTimeData](legacyToolLogTime),
		legacyToolTimesheetFillGap:               envelopeSchemaFor[TimesheetFillGapData](legacyToolTimesheetFillGap),
		legacyToolAddEntry:                       envelopeSchemaFor[EntryView](legacyToolAddEntry),
		"clockify_update_entry":                  envelopeSchemaFor[EntryView]("clockify_update_entry"),
		legacyToolFindAndUpdateEntry:             envelopeSchemaFor[FindAndUpdateEntryData](legacyToolFindAndUpdateEntry),
		"clockify_create_project":                envelopeSchemaFor[ProjectView]("clockify_create_project"),
		"clockify_update_project":                envelopeSchemaFor[ProjectView]("clockify_update_project"),
		"clockify_create_client":                 envelopeSchemaFor[ClientView]("clockify_create_client"),
		"clockify_update_client":                 envelopeSchemaFor[ClientView]("clockify_update_client"),
		"clockify_create_tag":                    envelopeSchemaFor[clockify.Tag]("clockify_create_tag"),
		"clockify_create_task":                   envelopeSchemaFor[TaskView]("clockify_create_task"),
		legacyToolStartTimer:                     envelopeSchemaFor[EntryView](legacyToolStartTimer),

		// --- open-shape tools (helper-driven, dynamic data) ---
		"clockify_stop_timer":           envelopeSchemaFor[EntryView]("clockify_stop_timer"),
		legacyToolTimerStatus:           envelopeSchemaFor[TimerStatusData](legacyToolTimerStatus),
		legacyToolSwitchProject:         envelopeOpaque(legacyToolSwitchProject),
		"clockify_delete_entry":         envelopeOpaque("clockify_delete_entry"),
		"clockify_delete_project":       envelopeOpaque("clockify_delete_project"),
		"clockify_delete_client":        envelopeOpaque("clockify_delete_client"),
		"clockify_delete_tag":           envelopeOpaque("clockify_delete_tag"),
		"clockify_delete_task":          envelopeOpaque("clockify_delete_task"),
		legacyToolResolveName:           envelopeOpaque(legacyToolResolveName),
		"clockify_resolve_debug":        envelopeOpaque("clockify_resolve_debug"),
		"clockify_workspace_governance": envelopeSchemaFor[WorkspaceGovernanceView]("clockify_workspace_governance"),
		"clockify_monthly_brief":        monthlyBriefOutputSchema(),
		"clockify_money_report":         moneyReportOutputSchema("clockify_money_report"),
		"clockify_audit_entries":        envelopeSchemaFor[AuditEntriesView]("clockify_audit_entries"),
	}
}

// applyTier1OutputSchemas attaches an outputSchema to every descriptor
// that has an entry in the lookup. Tools missing from the lookup are
// left untouched (their OutputSchema stays nil) so partial coverage
// during the sweep is safe.
func applyTier1OutputSchemas(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	schemas := tier1OutputSchemas()
	for i := range in {
		if s, ok := schemas[in[i].Tool.Name]; ok && in[i].Tool.OutputSchema == nil {
			in[i].Tool.OutputSchema = s
		}
	}
	return in
}

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
	return reportOutputSchema("clockify_monthly_brief", map[string]any{
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
