package tools

import (
	"sync"

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
//
// The map and every envelope value inside it are built once per process
// via sync.OnceValue. The result is shared across every Service.Registry()
// invocation and therefore across every streamable-HTTP session: schemas
// are immutable per binary build, so memoising eliminates ~3000 per-session
// allocations on the reflection-heavy envelopeSchemaFor[T] path. Callers
// MUST NOT mutate the returned maps; the existing assignment-only usage
// in applyTier1OutputSchemas preserves that invariant.
var tier1OutputSchemas = sync.OnceValue(buildTier1OutputSchemas)

func buildTier1OutputSchemas() map[string]map[string]any {
	return map[string]map[string]any{
		// --- typed-data tools ---
		"clockify_whoami":                envelopeSchemaFor[IdentityData]("clockify_whoami"),
		"clockify_current_user":          envelopeSchemaFor[clockify.User]("clockify_current_user"),
		"clockify_get_workspace":         envelopeOpaque("clockify_get_workspace"),
		"clockify_list_workspaces":       envelopeSchemaFor[[]clockify.Workspace]("clockify_list_workspaces"),
		"clockify_list_users":            envelopeSchemaFor[[]clockify.User]("clockify_list_users"),
		"clockify_list_projects":         envelopeSchemaFor[[]ProjectView]("clockify_list_projects"),
		"clockify_get_project":           envelopeSchemaFor[ProjectView]("clockify_get_project"),
		"clockify_get_client":            envelopeSchemaFor[ClientView]("clockify_get_client"),
		"clockify_list_clients":          envelopeSchemaFor[[]ClientView]("clockify_list_clients"),
		"clockify_client_report":         envelopeSchemaFor[ClientReportView]("clockify_client_report"),
		"clockify_list_tags":             envelopeSchemaFor[[]clockify.Tag]("clockify_list_tags"),
		"clockify_get_tag":               envelopeSchemaFor[clockify.Tag]("clockify_get_tag"),
		"clockify_update_tag":            envelopeSchemaFor[clockify.Tag]("clockify_update_tag"),
		"clockify_list_tasks":            envelopeSchemaFor[[]TaskView]("clockify_list_tasks"),
		"clockify_get_task":              envelopeSchemaFor[TaskView]("clockify_get_task"),
		"clockify_update_task":           envelopeSchemaFor[TaskView]("clockify_update_task"),
		"clockify_list_entries":          envelopeSchemaFor[[]EntryView]("clockify_list_entries"),
		"clockify_get_entry":             envelopeSchemaFor[EntryView]("clockify_get_entry"),
		"clockify_today_entries":         envelopeSchemaFor[[]EntryView]("clockify_today_entries"),
		"clockify_summary_report":        envelopeOpaque("clockify_summary_report"),
		"clockify_weekly_summary":        envelopeOpaque("clockify_weekly_summary"),
		"clockify_quick_report":          envelopeSchemaFor[QuickReportData]("clockify_quick_report"),
		"clockify_timesheet_review":      envelopeSchemaFor[TimesheetReviewData]("clockify_timesheet_review"),
		"clockify_attendance_report":     envelopeOpaque("clockify_attendance_report"),
		"clockify_detailed_report":       envelopeOpaque("clockify_detailed_report"),
		"clockify_log_time":              envelopeSchemaFor[LogTimeData]("clockify_log_time"),
		"clockify_timesheet_fill_gap":    envelopeSchemaFor[TimesheetFillGapData]("clockify_timesheet_fill_gap"),
		"clockify_add_entry":             envelopeSchemaFor[EntryView]("clockify_add_entry"),
		"clockify_update_entry":          envelopeSchemaFor[EntryView]("clockify_update_entry"),
		"clockify_find_and_update_entry": envelopeSchemaFor[FindAndUpdateEntryData]("clockify_find_and_update_entry"),
		"clockify_create_project":        envelopeSchemaFor[ProjectView]("clockify_create_project"),
		"clockify_update_project":        envelopeSchemaFor[ProjectView]("clockify_update_project"),
		"clockify_create_client":         envelopeSchemaFor[ClientView]("clockify_create_client"),
		"clockify_update_client":         envelopeSchemaFor[ClientView]("clockify_update_client"),
		"clockify_create_tag":            envelopeSchemaFor[clockify.Tag]("clockify_create_tag"),
		"clockify_create_task":           envelopeSchemaFor[TaskView]("clockify_create_task"),
		"clockify_start_timer":           envelopeSchemaFor[EntryView]("clockify_start_timer"),

		// --- open-shape tools (helper-driven, dynamic data) ---
		"clockify_stop_timer":       envelopeSchemaFor[EntryView]("clockify_stop_timer"),
		"clockify_timer_status":     envelopeSchemaFor[TimerStatusData]("clockify_timer_status"),
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
// by Tier 2 group activation so all 104 lazy-loaded tools advertise at
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
