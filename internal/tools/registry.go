package tools

import (
	"context"

	"github.com/apet97/go-clockify/internal/mcp"
)

func (s *Service) Registry() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		{Tool: toolRO("clockify_whoami", "Get current user and resolved workspace", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) { return s.WhoAmI(ctx) }},
		{Tool: toolRO("clockify_list_workspaces", "List available Clockify workspaces", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) { return s.ListWorkspaces(ctx) }},
		{Tool: toolRO("clockify_get_workspace", "Get the resolved workspace", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) { return s.GetWorkspace(ctx) }},
		{Tool: toolRO("clockify_list_users", "List users in the resolved workspace", paginationSchema(nil)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListUsers(ctx, args)
		}},
		{Tool: toolRO("clockify_current_user", "Get the current Clockify user", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) { return s.CurrentUser(ctx) }},
		{Tool: toolRO("clockify_list_projects", "List projects in the resolved workspace", paginationSchema(nil)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListProjects(ctx, args)
		}},
		{Tool: toolRO("clockify_get_project", "Get a project by ID or exact name", requiredSchema("project")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetProject(ctx, stringArg(args, "project"))
		}},
		{Tool: toolRO("clockify_list_clients", "List clients in the resolved workspace", paginationSchema(nil)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListClients(ctx, args)
		}},
		{Tool: toolRO("clockify_list_tags", "List tags in the resolved workspace", paginationSchema(nil)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListTags(ctx, args)
		}},
		{Tool: toolRO("clockify_list_tasks", "List tasks for a project", paginationSchema(map[string]any{
			"required":   []string{"project"},
			"properties": map[string]any{"project": map[string]any{"type": "string", "description": "Project name or ID"}},
		})), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListTasks(ctx, args)
		}},
		{Tool: toolRO("clockify_list_entries", "List recent time entries for the current user with optional date range, project filter, and pagination", map[string]any{"type": "object", "properties": map[string]any{
			"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
			"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50, max 200)"},
			"start":     map[string]any{"type": "string", "description": "Start time (RFC3339 or natural language)"},
			"end":       map[string]any{"type": "string", "description": "End time (RFC3339 or natural language)"},
			"project":   map[string]any{"type": "string", "description": "Filter by project name or ID"},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListEntries(ctx, args)
		}},
		{Tool: toolRO("clockify_get_entry", "Get a single time entry by ID", requiredSchema("entry_id")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetEntry(ctx, args)
		}},
		{Tool: toolRO("clockify_today_entries", "List time entries for the current day", map[string]any{"type": "object", "properties": map[string]any{
			"page":      map[string]any{"type": "integer"},
			"page_size": map[string]any{"type": "integer"},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.TodayEntries(ctx, args)
		}},
		{Tool: toolRO("clockify_summary_report", "Summarize entries for a date/time range by project using the current user's time entries", map[string]any{"type": "object", "properties": map[string]any{"start": map[string]any{"type": "string", "description": "RFC3339 timestamp"}, "end": map[string]any{"type": "string", "description": "RFC3339 timestamp"}, "include_entries": map[string]any{"type": "boolean"}}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.SummaryReport(ctx, args)
		}},
		{Tool: toolRO("clockify_weekly_summary", "Get a weekly summary for the current user, grouped by day and project. This is a safe wrapper built over time-entry data rather than a separate reports API.", map[string]any{"type": "object", "properties": map[string]any{"week_start": map[string]any{"type": "string", "description": "Optional RFC3339 timestamp or YYYY-MM-DD date. Defaults to Monday of the current week in local time."}, "timezone": map[string]any{"type": "string", "description": "Optional IANA timezone, defaults to local/server timezone."}, "include_entries": map[string]any{"type": "boolean"}}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.WeeklySummary(ctx, args)
		}},
		{Tool: toolRO("clockify_quick_report", "Quick high-signal summary for a recent period. Safe helper over the current user's time entries.", map[string]any{"type": "object", "properties": map[string]any{"days": map[string]any{"type": "integer", "minimum": 1, "maximum": 31}, "include_entries": map[string]any{"type": "boolean"}}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.QuickReport(ctx, args)
		}},
		{Tool: toolRW("clockify_start_timer", "Start a new timer", map[string]any{"type": "object", "properties": map[string]any{"project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.StartTimer(ctx, stringArg(args, "project_id"), stringArg(args, "project"), stringArg(args, "description"))
		}},
		{Tool: toolRWIdem("clockify_stop_timer", "Stop the current running timer", map[string]any{"type": "object", "properties": map[string]any{"dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.StopTimer(ctx, args)
		}},
		{Tool: toolRW("clockify_log_time", "Create a finished time entry for a project. Safe workflow helper for logging time without starting a live timer.", map[string]any{"type": "object", "required": []string{"start", "end"}, "properties": map[string]any{"project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "start": map[string]any{"type": "string", "description": "RFC3339 timestamp"}, "end": map[string]any{"type": "string", "description": "RFC3339 timestamp"}, "billable": map[string]any{"type": "boolean"}, "dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.LogTime(ctx, args)
		}},
		{Tool: toolRW("clockify_add_entry", "Create a new time entry with flexible start/end parsing", map[string]any{"type": "object", "required": []string{"start"}, "properties": map[string]any{
			"start":       map[string]any{"type": "string", "description": "Start time (RFC3339, or natural language: 'now', 'today 9:00')"},
			"end":         map[string]any{"type": "string", "description": "End time"},
			"description": map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string", "description": "Project name or ID"},
			"project_id":  map[string]any{"type": "string"},
			"task_id":     map[string]any{"type": "string"},
			"billable":    map[string]any{"type": "boolean"},
			"dry_run":     map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.AddEntry(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_entry", "Update an existing time entry (fetch-then-update merge)", map[string]any{"type": "object", "required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id":    map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string"},
			"project_id":  map[string]any{"type": "string"},
			"start":       map[string]any{"type": "string"},
			"end":         map[string]any{"type": "string"},
			"billable":    map[string]any{"type": "boolean"},
			"dry_run":     map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UpdateEntry(ctx, args)
		}},
		{Tool: toolDestructive("clockify_delete_entry", "Delete a time entry by ID", map[string]any{"type": "object", "required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id": map[string]any{"type": "string"},
			"dry_run":  map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeleteEntry(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_find_and_update_entry", "Find one current-user entry by exact ID or safe filters, then update selected fields. Fails closed on ambiguous matches.", map[string]any{"type": "object", "properties": map[string]any{"entry_id": map[string]any{"type": "string"}, "description_contains": map[string]any{"type": "string"}, "exact_description": map[string]any{"type": "string"}, "start_after": map[string]any{"type": "string", "description": "RFC3339 timestamp"}, "start_before": map[string]any{"type": "string", "description": "RFC3339 timestamp"}, "new_description": map[string]any{"type": "string"}, "project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "start": map[string]any{"type": "string", "description": "RFC3339 timestamp"}, "end": map[string]any{"type": "string", "description": "RFC3339 timestamp"}, "billable": map[string]any{"type": "boolean"}, "dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.FindAndUpdateEntry(ctx, args)
		}},
		{Tool: toolRW("clockify_create_project", "Create a new project", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name":      map[string]any{"type": "string"},
			"client":    map[string]any{"type": "string", "description": "Client name or ID"},
			"color":     map[string]any{"type": "string", "description": "Hex color code"},
			"billable":  map[string]any{"type": "boolean"},
			"is_public": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateProject(ctx, args)
		}},
		{Tool: toolRW("clockify_create_client", "Create a new client", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name": map[string]any{"type": "string"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateClient(ctx, args)
		}},
		{Tool: toolRW("clockify_create_tag", "Create a new tag", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name": map[string]any{"type": "string"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateTag(ctx, args)
		}},
		{Tool: toolRW("clockify_create_task", "Create a new task in a project", map[string]any{"type": "object", "required": []string{"project", "name"}, "properties": map[string]any{
			"project":  map[string]any{"type": "string", "description": "Project name or ID"},
			"name":     map[string]any{"type": "string"},
			"billable": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateTask(ctx, args)
		}},

		// --- Wave 2 additions ---
		{Tool: toolRO("clockify_timer_status", "Check if a timer is currently running and show elapsed time", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			return s.TimerStatus(ctx)
		}},
		{Tool: toolRW("clockify_switch_project", "Stop the current timer and start a new one on a different project", map[string]any{"type": "object", "required": []string{"project"}, "properties": map[string]any{
			"project":     map[string]any{"type": "string", "description": "Project name or ID to switch to"},
			"description": map[string]any{"type": "string"},
			"task_id":     map[string]any{"type": "string"},
			"billable":    map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.SwitchProject(ctx, args)
		}},
		{Tool: toolRO("clockify_detailed_report", "Detailed time entry report with filtering by project and date range", map[string]any{"type": "object", "required": []string{"start", "end"}, "properties": map[string]any{
			"start":           map[string]any{"type": "string", "description": "RFC3339 timestamp"},
			"end":             map[string]any{"type": "string", "description": "RFC3339 timestamp"},
			"project":         map[string]any{"type": "string"},
			"include_entries": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DetailedReport(ctx, args)
		}},
		{Tool: toolRO("clockify_resolve_debug", "Debug name-to-ID resolution for projects, clients, tags, or users", map[string]any{"type": "object", "required": []string{"entity_type", "name_or_id"}, "properties": map[string]any{
			"entity_type": map[string]any{"type": "string", "description": "project, client, tag, or user"},
			"name_or_id":  map[string]any{"type": "string"},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ResolveDebug(ctx, args)
		}},
		{Tool: toolRO("clockify_policy_info", "Display effective policy configuration", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			return s.PolicyInfo(ctx)
		}},
		{Tool: toolRO("clockify_search_tools", "Search and discover available tools by keyword", map[string]any{"type": "object", "properties": map[string]any{
			"query":          map[string]any{"type": "string", "description": "Search query for tools"},
			"activate_group": map[string]any{"type": "string"},
			"activate_tool":  map[string]any{"type": "string"},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.SearchTools(ctx, args)
		}},
	}
}
