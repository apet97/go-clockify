package tools

import (
	"context"

	"github.com/apet97/go-clockify/internal/mcp"
)

const flexibleDatetimeDescription = "Datetime (RFC3339, YYYY-MM-DD, 'today HH:MM', 'yesterday HH:MM', or 'now')"

func timezoneInputProperty() map[string]any {
	return map[string]any{"type": "string", "description": "Optional IANA timezone; defaults to CLOCKIFY_TIMEZONE or the local/server timezone."}
}

func resolveNameInputSchema() map[string]any {
	return schemaObject(
		[]string{"entity_type", "name_or_id"},
		map[string]any{
			"entity_type": schemaEnum("Type of Clockify entity to resolve",
				"project", "client", "tag", "user", "task"),
			"name_or_id": schemaString(""),
			"project":    schemaString("Project name or ID required when entity_type is task"),
			"project_id": schemaString("Project ID required when entity_type is task unless project is supplied"),
		},
	)
}

func (s *Service) Registry() []mcp.ToolDescriptor {
	return applyTier1OutputSchemas(normalizeDescriptors([]mcp.ToolDescriptor{
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
		{Tool: toolRO("clockify_get_client", "Get a client by ID or exact name", requiredSchema("client")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetClient(ctx, stringArg(args, "client"))
		}},
		{Tool: toolRO("clockify_list_tags", "List tags in the resolved workspace", paginationSchema(nil)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListTags(ctx, args)
		}},
		{Tool: toolRO("clockify_get_tag", "Get a tag by ID or exact name", requiredSchema("tag")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetTag(ctx, args)
		}},
		{Tool: toolRO("clockify_list_tasks", "List tasks for a project", paginationSchema(map[string]any{
			"required":   []string{"project"},
			"properties": map[string]any{"project": map[string]any{"type": "string", "description": "Project name or ID"}},
		})), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListTasks(ctx, args)
		}},
		{Tool: toolRO("clockify_get_task", "Get a task by ID or exact name within a project", map[string]any{"type": "object", "required": []string{"project", "task"}, "properties": map[string]any{
			"project": map[string]any{"type": "string", "description": "Project name or ID"},
			"task":    map[string]any{"type": "string", "description": "Task ID or exact name"},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetTask(ctx, args)
		}},
		{Tool: toolRO("clockify_list_entries", "List recent time entries for the current user with optional date range, project filter, and pagination", paginationSchema(map[string]any{
			"properties": map[string]any{
				"start":    map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":      map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"timezone": timezoneInputProperty(),
				"project":  map[string]any{"type": "string", "description": "Filter by project name or ID"},
			},
		})), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListEntries(ctx, args)
		}},
		{Tool: toolRO("clockify_get_entry", "Get a single time entry by ID", requiredSchema("entry_id")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetEntry(ctx, args)
		}},
		{Tool: toolRO("clockify_today_entries", "List time entries for the current day", paginationSchema(map[string]any{"properties": map[string]any{"timezone": timezoneInputProperty()}})), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.TodayEntries(ctx, args)
		}},
		{Tool: toolRO("clockify_summary_report", "Summarize entries for a date/time range by project using the current user's time entries", map[string]any{"type": "object", "properties": map[string]any{
			"start":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":             map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"timezone":        timezoneInputProperty(),
			"project":         map[string]any{"type": "string", "description": "Optional project name or ID to push down as an upstream filter"},
			"include_entries": map[string]any{"type": "boolean"},
			"max_entries":     map[string]any{"type": "integer", "minimum": 0, "description": "Optional per-request cap override (bounded by server CLOCKIFY_REPORT_MAX_ENTRIES). 0 = unlimited (server cap still applies)."},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.SummaryReport(ctx, args)
		}},
		{Tool: toolRO("clockify_weekly_summary", "Get a weekly summary for the current user, grouped by day and project. This is a safe wrapper built over time-entry data rather than a separate reports API.", map[string]any{"type": "object", "properties": map[string]any{
			"week_start":      map[string]any{"type": "string", "description": "Optional RFC3339 timestamp or YYYY-MM-DD date. Defaults to Monday of the current week in local time."},
			"timezone":        map[string]any{"type": "string", "description": "Optional IANA timezone, defaults to local/server timezone."},
			"project":         map[string]any{"type": "string", "description": "Optional project name or ID to push down as an upstream filter"},
			"include_entries": map[string]any{"type": "boolean"},
			"max_entries":     map[string]any{"type": "integer", "minimum": 0, "description": "Optional per-request cap override (bounded by server CLOCKIFY_REPORT_MAX_ENTRIES). 0 = unlimited (server cap still applies)."},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.WeeklySummary(ctx, args)
		}},
		{Tool: toolRO("clockify_quick_report", "Quick high-signal summary for a recent period. Safe helper over the current user's time entries.", map[string]any{"type": "object", "properties": map[string]any{
			"days":            map[string]any{"type": "integer", "minimum": 1, "maximum": quickReportMaxDays},
			"timezone":        timezoneInputProperty(),
			"project":         map[string]any{"type": "string", "description": "Optional project name or ID to push down as an upstream filter"},
			"include_entries": map[string]any{"type": "boolean"},
			"max_entries":     map[string]any{"type": "integer", "minimum": 0, "description": "Optional per-request cap override (bounded by server CLOCKIFY_REPORT_MAX_ENTRIES). 0 = unlimited (server cap still applies)."},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.QuickReport(ctx, args)
		}},
		{Tool: toolRO("clockify_timesheet_review", "Review a day, week, or date range and return timesheet issues plus ready next-tool suggestions for an AI agent.", map[string]any{"type": "object", "properties": map[string]any{
			"date":            map[string]any{"type": "string", "description": "YYYY-MM-DD date to review; defaults to today in timezone."},
			"week_start":      map[string]any{"type": "string", "description": "Optional RFC3339 timestamp or YYYY-MM-DD date. Reviews the ISO week containing this date."},
			"start":           map[string]any{"type": "string", "description": flexibleDatetimeDescription + ". Must be paired with end."},
			"end":             map[string]any{"type": "string", "description": flexibleDatetimeDescription + ". Must be paired with start."},
			"timezone":        map[string]any{"type": "string", "description": "Optional IANA timezone, defaults to local/server timezone."},
			"workday_start":   map[string]any{"type": "string", "description": "HH:MM local workday start for gap detection; default 09:00."},
			"workday_end":     map[string]any{"type": "string", "description": "HH:MM local workday end for gap detection; default 17:00."},
			"min_gap_minutes": map[string]any{"type": "integer", "minimum": 0, "description": "Minimum untracked gap to report. Default 30. Set 0 to disable gap suggestions."},
			"max_suggestions": map[string]any{"type": "integer", "minimum": 0, "maximum": 50, "description": "Maximum suggested tool calls to return. Default 10."},
			"include_entries": map[string]any{"type": "boolean"},
			"max_entries":     map[string]any{"type": "integer", "minimum": 0, "description": "Optional per-request cap override (bounded by server CLOCKIFY_REPORT_MAX_ENTRIES). 0 = unlimited (server cap still applies)."},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.TimesheetReview(ctx, args)
		}},
		{Tool: toolRW("clockify_start_timer", "Start a new timer. Supports dry_run:true.", map[string]any{"type": "object", "properties": map[string]any{"project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "type": map[string]any{"type": "string", "enum": []string{"REGULAR", "BREAK"}, "description": "Time entry type. REGULAR is the default; BREAK requires the workspace to have the Break feature enabled."}, "dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.StartTimerArgs(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_stop_timer", "Stop the current running timer", map[string]any{"type": "object", "properties": map[string]any{"dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.StopTimer(ctx, args)
		}},
		{Tool: toolRW("clockify_log_time", "Create a finished time entry for a project. Preferred helper for logging past work; validates overlaps unless allow_overlap:true is passed.", map[string]any{"type": "object", "required": []string{"start", "end"}, "properties": map[string]any{"project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "start": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "end": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "timezone": timezoneInputProperty(), "tag_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional Clockify tag IDs to attach to the entry"}, "billable": map[string]any{"type": "boolean"}, "type": map[string]any{"type": "string", "enum": []string{"REGULAR", "BREAK"}, "description": "Time entry type. REGULAR is the default; BREAK requires the workspace to have the Break feature enabled."}, "allow_overlap": map[string]any{"type": "boolean", "description": "Default false. Set true only after manually confirming the overlap is intentional."}, "dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.LogTime(ctx, args)
		}},
		{Tool: toolRW("clockify_timesheet_fill_gap", "Create one finished time entry for a reviewed gap after validating that the requested interval does not overlap existing entries. Supports dry_run:true.", map[string]any{"type": "object", "required": []string{"start", "end", "description"}, "properties": map[string]any{
			"start":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"timezone":      timezoneInputProperty(),
			"project":       map[string]any{"type": "string", "description": "Project name or ID"},
			"project_id":    map[string]any{"type": "string"},
			"description":   map[string]any{"type": "string"},
			"billable":      map[string]any{"type": "boolean"},
			"type":          map[string]any{"type": "string", "enum": []string{"REGULAR", "BREAK"}, "description": "Time entry type. REGULAR is the default; BREAK requires the workspace to have the Break feature enabled."},
			"allow_overlap": map[string]any{"type": "boolean", "description": "Default false. Set true only after manually confirming the overlap is intentional."},
			"dry_run":       map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.TimesheetFillGap(ctx, args)
		}},
		{Tool: toolRW("clockify_add_entry", "Lower-level helper for creating a time entry with flexible start/end parsing. Prefer clockify_log_time for finished past work; when end is present this validates overlaps unless allow_overlap:true is passed.", map[string]any{"type": "object", "required": []string{"start"}, "properties": map[string]any{
			"start":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"timezone":      timezoneInputProperty(),
			"description":   map[string]any{"type": "string"},
			"project":       map[string]any{"type": "string", "description": "Project name or ID"},
			"project_id":    map[string]any{"type": "string"},
			"task_id":       map[string]any{"type": "string"},
			"tag_ids":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional Clockify tag IDs to attach to the entry"},
			"billable":      map[string]any{"type": "boolean"},
			"type":          map[string]any{"type": "string", "enum": []string{"REGULAR", "BREAK"}, "description": "Time entry type. REGULAR is the default; BREAK requires the workspace to have the Break feature enabled."},
			"allow_overlap": map[string]any{"type": "boolean", "description": "Default false for finished entries with end set. Set true only after manually confirming the overlap is intentional."},
			"dry_run":       map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.AddEntry(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_entry", "Update an existing time entry (fetch-then-update merge)", map[string]any{"type": "object", "required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id":    map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string"},
			"project_id":  map[string]any{"type": "string"},
			"start":       map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"timezone":    timezoneInputProperty(),
			"billable":    map[string]any{"type": "boolean"},
			"type":        map[string]any{"type": "string", "enum": []string{"REGULAR", "BREAK"}, "description": "Time entry type. REGULAR is the default; BREAK requires the workspace to have the Break feature enabled."},
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
		{Tool: toolRWIdem("clockify_find_and_update_entry", "Find one current-user entry by exact ID or safe filters, then update selected fields. Fails closed on ambiguous matches.", map[string]any{"type": "object", "properties": map[string]any{"entry_id": map[string]any{"type": "string"}, "description_contains": map[string]any{"type": "string"}, "exact_description": map[string]any{"type": "string"}, "start_after": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "start_before": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "new_description": map[string]any{"type": "string"}, "project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "start": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "end": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "timezone": timezoneInputProperty(), "billable": map[string]any{"type": "boolean"}, "dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.FindAndUpdateEntry(ctx, args)
		}},
		{Tool: toolRW("clockify_create_project", "Create a new project", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name":      map[string]any{"type": "string"},
			"client":    map[string]any{"type": "string", "description": "Client name or ID"},
			"color":     map[string]any{"type": "string", "description": "Hex color code"},
			"billable":  map[string]any{"type": "boolean"},
			"is_public": map[string]any{"type": "boolean"},
			"dry_run":   map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateProject(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_project", "Update a project by ID or exact name using a fetch-then-merge strategy. Clockify's PUT is a full replacement; caller-supplied empty strings are treated as 'do not change'. Use the dedicated archived boolean to flip archival state.", map[string]any{"type": "object", "required": []string{"project"}, "properties": map[string]any{
			"project":   map[string]any{"type": "string", "description": "Project name or ID"},
			"name":      map[string]any{"type": "string"},
			"client":    map[string]any{"type": "string", "description": "Client name or ID"},
			"color":     map[string]any{"type": "string", "description": "Hex color code"},
			"note":      map[string]any{"type": "string"},
			"billable":  map[string]any{"type": "boolean"},
			"is_public": map[string]any{"type": "boolean"},
			"archived":  map[string]any{"type": "boolean"},
			"dry_run":   map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UpdateProject(ctx, args)
		}},
		{Tool: toolDestructive("clockify_delete_project", "Delete a project by ID or exact name. Archives the project first if it is still active (Clockify rejects DELETE on active projects).", map[string]any{"type": "object", "required": []string{"project"}, "properties": map[string]any{
			"project": map[string]any{"type": "string", "description": "Project name or ID"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeleteProject(ctx, args)
		}},
		{Tool: toolRW("clockify_create_client", "Create a new client. Accepts optional address, email, and note alongside the required name.", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name":    map[string]any{"type": "string"},
			"address": map[string]any{"type": "string"},
			"email":   map[string]any{"type": "string"},
			"note":    map[string]any{"type": "string"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateClient(ctx, args)
		}},
		{Tool: toolDestructive("clockify_delete_client", "Delete a client by ID or exact name. Archives the client first if it is still active (Clockify rejects DELETE on active clients).", map[string]any{"type": "object", "required": []string{"client"}, "properties": map[string]any{
			"client":  map[string]any{"type": "string", "description": "Client name or ID"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeleteClient(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_client", "Update a client by ID or exact name using a fetch-then-merge strategy. Clockify's PUT is a full replacement; caller-supplied empty strings are treated as 'do not change'. Use the dedicated archived boolean to flip archival state.", map[string]any{"type": "object", "required": []string{"client"}, "properties": map[string]any{
			"client":   map[string]any{"type": "string", "description": "Client name or ID"},
			"name":     map[string]any{"type": "string"},
			"address":  map[string]any{"type": "string"},
			"email":    map[string]any{"type": "string"},
			"note":     map[string]any{"type": "string"},
			"archived": map[string]any{"type": "boolean"},
			"dry_run":  map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UpdateClient(ctx, args)
		}},
		{Tool: toolRW("clockify_create_tag", "Create a new tag", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name":    map[string]any{"type": "string"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateTag(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_tag", "Update a tag by ID or exact name using fetch-then-merge. Clockify's PUT is a full replacement; use the archived boolean to flip archival state.", map[string]any{"type": "object", "required": []string{"tag"}, "properties": map[string]any{
			"tag":      map[string]any{"type": "string", "description": "Tag name or ID"},
			"name":     map[string]any{"type": "string"},
			"archived": map[string]any{"type": "boolean"},
			"dry_run":  map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UpdateTag(ctx, args)
		}},
		{Tool: toolDestructive("clockify_delete_tag", "Delete a tag by ID or exact name. Clockify supports direct DELETE without an archive step for tags.", map[string]any{"type": "object", "required": []string{"tag"}, "properties": map[string]any{
			"tag":     map[string]any{"type": "string", "description": "Tag name or ID"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeleteTag(ctx, args)
		}},
		{Tool: toolRW("clockify_create_task", "Create a new task in a project", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID"},
			"project_id": map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"billable":   map[string]any{"type": "boolean"},
			"estimate":   map[string]any{"type": "string", "description": "ISO-8601 duration estimate, e.g. PT2H30M"},
			"status":     map[string]any{"type": "string", "enum": []string{"ACTIVE", "DONE"}, "description": "Task status"},
			"dry_run":    map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateTask(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_task", "Update a task by project+task reference using fetch-then-merge. Clockify's PUT is a full replacement; caller-supplied empty strings are treated as 'do not change'.", map[string]any{"type": "object", "required": []string{"project", "task"}, "properties": map[string]any{
			"project":  map[string]any{"type": "string", "description": "Project name or ID"},
			"task":     map[string]any{"type": "string", "description": "Task ID or exact name"},
			"name":     map[string]any{"type": "string"},
			"status":   map[string]any{"type": "string", "enum": []string{"ACTIVE", "DONE"}, "description": "Task status"},
			"estimate": map[string]any{"type": "string", "description": "ISO-8601 duration estimate, e.g. PT2H30M"},
			"billable": map[string]any{"type": "boolean"},
			"dry_run":  map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UpdateTask(ctx, args)
		}},
		{Tool: toolDestructive("clockify_delete_task", "Delete a task by project+task reference. Clockify supports direct DELETE without an archive step for tasks.", map[string]any{"type": "object", "required": []string{"project", "task"}, "properties": map[string]any{
			"project": map[string]any{"type": "string", "description": "Project name or ID"},
			"task":    map[string]any{"type": "string", "description": "Task ID or exact name"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeleteTask(ctx, args)
		}},

		// --- Wave 2 additions ---
		{Tool: toolRO("clockify_timer_status", "Check if a timer is currently running and show elapsed time", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			return s.TimerStatus(ctx)
		}},
		{Tool: toolRW("clockify_switch_project", "Stop the current timer if one is running, then start a new one on a different project. Response includes stop_outcome.", map[string]any{"type": "object", "required": []string{"project"}, "properties": map[string]any{
			"project":     map[string]any{"type": "string", "description": "Project name or ID to switch to"},
			"description": map[string]any{"type": "string"},
			"task_id":     map[string]any{"type": "string"},
			"billable":    map[string]any{"type": "boolean"},
			"dry_run":     map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.SwitchProject(ctx, args)
		}},
		{Tool: toolRO("clockify_detailed_report", "Detailed time entry report with filtering by project and date range", map[string]any{"type": "object", "required": []string{"start", "end"}, "properties": map[string]any{
			"start":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":             map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"timezone":        timezoneInputProperty(),
			"project":         map[string]any{"type": "string"},
			"include_entries": map[string]any{"type": "boolean"},
			"max_entries":     map[string]any{"type": "integer", "minimum": 0, "description": "Optional per-request cap override (bounded by server CLOCKIFY_REPORT_MAX_ENTRIES). 0 = unlimited (server cap still applies)."},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DetailedReport(ctx, args)
		}},
		{Tool: toolRO("clockify_resolve_name", "Resolve a project, client, tag, user, or project-scoped task name/email to a Clockify ID. Prefer this before write tools when a user gives a name.", resolveNameInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ResolveName(ctx, args)
		}},
		{Tool: toolRO("clockify_resolve_debug", "Deprecated compatibility alias for clockify_resolve_name.", resolveNameInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ResolveDebug(ctx, args)
		}},
		{Tool: toolRO("clockify_policy_info", "Display effective policy configuration", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			return s.PolicyInfo(ctx)
		}},
		{Tool: toolRO("clockify_list_tools", "Search/list the tool catalog by query string. Group results include activatable and block_reason metadata so agents can preflight whether activation will succeed under the current policy.", map[string]any{"type": "object", "properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "Search query for tools or groups"},
		}}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListTools(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_activate_group", "Activate every tool in the named Tier-2 group. The response puts only currently visible/callable names in activated_tools and reports unavailable names under activated_tools_hidden_by_bootstrap or activated_tools_blocked_by_policy.", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Tier-2 group name to activate, e.g. invoices"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ActivateToolGroup(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_activate_tool", "Activate a hidden Tier-1 tool by name, or activate the full Tier-2 group that contains the named Tier-2 tool. The side effect can bring sibling tools online; inspect activated_tools after the call.", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Tool name to activate, e.g. clockify_send_invoice"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ActivateNamedTool(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_deactivate_group", "Deactivate a previously activated Tier-2 group and remove its tools from tools/list for this session. Idempotent: deactivating an inactive group removes zero tools.", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Tier-2 group name to deactivate, e.g. invoices"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeactivateToolGroup(ctx, args)
		}},
		// clockify_search_tools is annotated as a write tool (idempotent)
		// because the activate_group / activate_tool branches mutate the
		// server's visible tool surface and emit
		// notifications/tools/list_changed. ChatGPT's audit pointed out
		// that classifying it read-only let activations bypass the
		// intent/outcome audit pipeline — read-only tools short-circuit
		// at audit.go's `hints.ReadOnly` gate. Idempotent because
		// re-activating an already-active group is a no-op.
		{Tool: toolRWIdem("clockify_search_tools", "Deprecated compatibility shim. Prefer clockify_list_tools, clockify_activate_group, clockify_activate_tool, or clockify_deactivate_group for single-purpose planning. In minimal/custom bootstrap modes, search results mark hidden Tier-1 tools with availability=tier1_hidden_by_bootstrap. Tier-2 groups remain the unit of activation.", map[string]any{"type": "object", "properties": map[string]any{
			"query":          map[string]any{"type": "string", "description": "Search query for tools"},
			"activate_group": map[string]any{"type": "string", "description": "Activate every tool in the named Tier-2 group (e.g. \"invoices\")"},
			"activate_tool":  map[string]any{"type": "string", "description": "Activate a hidden Tier-1 tool, or activate the Tier-2 group that contains the named Tier-2 tool (e.g. \"clockify_send_invoice\" activates the full \"invoices\" group)."},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.SearchTools(ctx, args)
		}},
	}))
}
