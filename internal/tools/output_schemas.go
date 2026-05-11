package tools

import (
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// tier1OutputSchemas returns the outputSchema map keyed by tool name for
// every Tier 1 tool. Splitting the schema lookup out of registry.go keeps
// the inline tool table compact and lets the schema sweep be reviewed in
// isolation.
//
// Tools whose handlers return a typed Go struct (SummaryData, LogTimeData,
// clockify.TimeEntry, etc.) get a schemaFor[T]-driven envelope. Tools that
// return open shapes (map[string]any from helper builders, internal status
// structs, etc.) get envelopeOpaque, which still pins ok/action/data and
// makes the action a JSON Schema const so MCP clients can dispatch on it.
func tier1OutputSchemas() map[string]map[string]any {
	return map[string]map[string]any{
		// --- typed-data tools ---
		"clockify_whoami":                envelopeSchemaFor[IdentityData]("clockify_whoami"),
		"clockify_current_user":          envelopeSchemaFor[clockify.User]("clockify_current_user"),
		"clockify_get_workspace":         envelopeOpaque("clockify_get_workspace"),
		"clockify_list_workspaces":       envelopeSchemaFor[[]clockify.Workspace]("clockify_list_workspaces"),
		"clockify_list_users":            envelopeSchemaFor[[]clockify.User]("clockify_list_users"),
		"clockify_list_projects":         envelopeSchemaFor[[]clockify.Project]("clockify_list_projects"),
		"clockify_get_project":           envelopeSchemaFor[clockify.Project]("clockify_get_project"),
		"clockify_get_client":            envelopeSchemaFor[clockify.ClientEntity]("clockify_get_client"),
		"clockify_list_clients":          envelopeSchemaFor[[]clockify.ClientEntity]("clockify_list_clients"),
		"clockify_list_tags":             envelopeSchemaFor[[]clockify.Tag]("clockify_list_tags"),
		"clockify_get_tag":               envelopeSchemaFor[clockify.Tag]("clockify_get_tag"),
		"clockify_update_tag":            envelopeSchemaFor[clockify.Tag]("clockify_update_tag"),
		"clockify_list_tasks":            envelopeSchemaFor[[]clockify.Task]("clockify_list_tasks"),
		"clockify_get_task":              envelopeSchemaFor[clockify.Task]("clockify_get_task"),
		"clockify_update_task":           envelopeSchemaFor[clockify.Task]("clockify_update_task"),
		"clockify_list_entries":          envelopeSchemaFor[[]clockify.TimeEntry]("clockify_list_entries"),
		"clockify_get_entry":             envelopeSchemaFor[clockify.TimeEntry]("clockify_get_entry"),
		"clockify_today_entries":         envelopeSchemaFor[[]clockify.TimeEntry]("clockify_today_entries"),
		"clockify_summary_report":        envelopeSchemaFor[SummaryData]("clockify_summary_report"),
		"clockify_weekly_summary":        envelopeSchemaFor[WeeklySummaryData]("clockify_weekly_summary"),
		"clockify_quick_report":          envelopeSchemaFor[QuickReportData]("clockify_quick_report"),
		"clockify_timesheet_review":      envelopeSchemaFor[TimesheetReviewData]("clockify_timesheet_review"),
		"clockify_detailed_report":       envelopeSchemaFor[SummaryData]("clockify_detailed_report"),
		"clockify_log_time":              envelopeSchemaFor[LogTimeData]("clockify_log_time"),
		"clockify_timesheet_fill_gap":    envelopeSchemaFor[TimesheetFillGapData]("clockify_timesheet_fill_gap"),
		"clockify_add_entry":             envelopeSchemaFor[clockify.TimeEntry]("clockify_add_entry"),
		"clockify_update_entry":          envelopeSchemaFor[clockify.TimeEntry]("clockify_update_entry"),
		"clockify_find_and_update_entry": envelopeSchemaFor[FindAndUpdateEntryData]("clockify_find_and_update_entry"),
		"clockify_create_project":        envelopeSchemaFor[clockify.Project]("clockify_create_project"),
		"clockify_update_project":        envelopeSchemaFor[clockify.Project]("clockify_update_project"),
		"clockify_create_client":         envelopeSchemaFor[clockify.ClientEntity]("clockify_create_client"),
		"clockify_update_client":         envelopeSchemaFor[clockify.ClientEntity]("clockify_update_client"),
		"clockify_create_tag":            envelopeSchemaFor[clockify.Tag]("clockify_create_tag"),
		"clockify_create_task":           envelopeSchemaFor[clockify.Task]("clockify_create_task"),
		"clockify_start_timer":           envelopeSchemaFor[clockify.TimeEntry]("clockify_start_timer"),

		// --- open-shape tools (helper-driven, dynamic data) ---
		"clockify_stop_timer":       envelopeOpaque("clockify_stop_timer"),
		"clockify_timer_status":     envelopeOpaque("clockify_timer_status"),
		"clockify_switch_project":   envelopeOpaque("clockify_switch_project"),
		"clockify_delete_entry":     envelopeOpaque("clockify_delete_entry"),
		"clockify_delete_project":   envelopeOpaque("clockify_delete_project"),
		"clockify_delete_client":    envelopeOpaque("clockify_delete_client"),
		"clockify_delete_tag":       envelopeOpaque("clockify_delete_tag"),
		"clockify_delete_task":      envelopeOpaque("clockify_delete_task"),
		"clockify_resolve_name":     envelopeOpaque("clockify_resolve_name"),
		"clockify_resolve_debug":    envelopeOpaque("clockify_resolve_debug"),
		"clockify_policy_info":      envelopeOpaque("clockify_policy_info"),
		"clockify_list_tools":       envelopeOpaque("clockify_list_tools"),
		"clockify_activate_group":   envelopeOpaque("clockify_activate_group"),
		"clockify_activate_tool":    envelopeOpaque("clockify_activate_tool"),
		"clockify_deactivate_group": envelopeOpaque("clockify_deactivate_group"),
		"clockify_search_tools":     envelopeOpaque("clockify_search_tools"),
	}
}

// applyTier1OutputSchemas attaches an outputSchema to every Tier 1 tool
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
// by Tier 2 group activation so all 91 lazy-loaded tools advertise at
// least the envelope wrapper to clients without the maintenance burden
// of hand-crafting per-tool typed schemas.
func applyOpaqueOutputSchemas(in []mcp.ToolDescriptor) []mcp.ToolDescriptor {
	for i := range in {
		if in[i].Tool.OutputSchema == nil {
			in[i].Tool.OutputSchema = envelopeOpaque(in[i].Tool.Name)
		}
	}
	return in
}
