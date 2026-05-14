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
		{Tool: toolRO("clockify_workspace_governance", "Explain Clockify workspace governance settings: locks, approvals, rounding, required fields, creation permissions, features, subscription, and working days", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			return s.WorkspaceGovernance(ctx)
		}},
		{Tool: toolRO("clockify_list_users", "List users in the resolved workspace", paginationSchema(nil)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListUsers(ctx, args)
		}},
		{Tool: toolRO("clockify_current_user", "Get the current Clockify user", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) { return s.CurrentUser(ctx) }},
		{Tool: toolRO("clockify_list_projects", "List projects in the resolved workspace", projectListInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListProjects(ctx, args)
		}},
		{Tool: toolRO("clockify_get_project", "Get a project by ID or exact name", projectGetInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetProject(ctx, args)
		}},
		{Tool: toolRO("clockify_list_clients", "List clients in the resolved workspace", clientListInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListClients(ctx, args)
		}},
		{Tool: toolRO("clockify_get_client", "Get a client by ID or exact name with project, time, and money summary enrichment", clientGetInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetClientWithArgs(ctx, args)
		}},
		{Tool: toolRO("clockify_client_report", "Build a client-level report with projects, tracked money/time, invoice totals, approval state, and detailed-entry health", clientReportInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ClientReport(ctx, args)
		}},
		{Tool: toolRO("clockify_list_tags", "List tags in the resolved workspace", paginationSchema(nil)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListTags(ctx, args)
		}},
		{Tool: toolRO("clockify_get_tag", "Get a tag by ID or exact name", requiredSchema("tag")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.GetTag(ctx, args)
		}},
		{Tool: toolRO("clockify_list_tasks", "List tasks for a project", taskListInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
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
		{Tool: toolRO("clockify_list_in_progress_time_entries", "List workspace-wide in-progress timers with entry financial and custom-field enrichment", paginationSchema(nil)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListInProgressTimeEntries(ctx, args)
		}},
		{Tool: toolRO("clockify_summary_report", "Generate a Clockify summary report via the Reports API. Supports up to 3 groups: CLIENT, PROJECT, TASK, DATE, WEEK, MONTH, TIMEENTRY.", reportInputSchema("summary_filter", summaryFilterSchema())), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.SummaryReport(ctx, args)
		}},
		{Tool: toolRO("clockify_weekly_summary", "Generate a Clockify weekly report via the Reports API. weekly_filter.group is PROJECT or USER; subgroup is TIME.", weeklyReportInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
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
		{Tool: toolRO("clockify_monthly_brief", "Read-only month brief with money totals and entry audit health.", monthlyBriefInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.MonthlyBrief(ctx, args)
		}},
		{Tool: toolRO("clockify_money_report", "Read-only money report using Clockify Reports API earned, cost, and profit totals.", moneyReportInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.MoneyReport(ctx, args)
		}},
		{Tool: toolRO("clockify_audit_entries", "Read-only detailed report audit for missing fields, locked entries, billable state, approval, and invoicing health.", auditEntriesInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.AuditEntries(ctx, args)
		}},
		{Tool: toolRO(legacyToolTimesheetReview, "Review a day, week, or date range and return timesheet issues plus ready next-tool suggestions for an AI agent.", map[string]any{"type": "object", "properties": map[string]any{
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
		{Tool: toolRW(legacyToolStartTimer, "Start a new timer. Supports dry_run:true.", map[string]any{"type": "object", "properties": map[string]any{"project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "type": map[string]any{"type": "string", "enum": []string{"REGULAR", "BREAK"}, "description": "Time entry type. REGULAR is the default; BREAK requires the workspace to have the Break feature enabled."}, "dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.StartTimerArgs(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_stop_timer", "Stop the current running timer", map[string]any{"type": "object", "properties": map[string]any{"dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.StopTimer(ctx, args)
		}},
		{Tool: toolRW(legacyToolLogTime, "Create a finished time entry for a project. Preferred helper for logging past work; validates overlaps unless allow_overlap:true is passed. Omit billable or pass null to inherit Clockify's project/workspace default.", map[string]any{"type": "object", "required": []string{"start", "end"}, "properties": map[string]any{"project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "start": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "end": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "timezone": timezoneInputProperty(), "tag_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional Clockify tag IDs to attach to the entry"}, "billable": nullableBoolSchema("Optional billable flag. Omit or pass null to inherit the project/workspace default."), "type": map[string]any{"type": "string", "enum": []string{"REGULAR", "BREAK"}, "description": "Time entry type. REGULAR is the default; BREAK requires the workspace to have the Break feature enabled."}, "allow_overlap": map[string]any{"type": "boolean", "description": "Default false. Set true only after manually confirming the overlap is intentional."}, "dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.LogTime(ctx, args)
		}},
		{Tool: toolRW(legacyToolTimesheetFillGap, "Create one finished time entry for a reviewed gap after validating that the requested interval does not overlap existing entries. Supports dry_run:true.", map[string]any{"type": "object", "required": []string{"start", "end", "description"}, "properties": map[string]any{
			"start":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"timezone":      timezoneInputProperty(),
			"project":       map[string]any{"type": "string", "description": "Project name or ID"},
			"project_id":    map[string]any{"type": "string"},
			"description":   map[string]any{"type": "string"},
			"billable":      nullableBoolSchema("Optional billable flag. Omit or pass null to inherit the project/workspace default."),
			"type":          map[string]any{"type": "string", "enum": []string{"REGULAR", "BREAK"}, "description": "Time entry type. REGULAR is the default; BREAK requires the workspace to have the Break feature enabled."},
			"allow_overlap": map[string]any{"type": "boolean", "description": "Default false. Set true only after manually confirming the overlap is intentional."},
			"dry_run":       map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.TimesheetFillGap(ctx, args)
		}},
		{Tool: toolRW(legacyToolAddEntry, "Lower-level helper for creating a time entry with flexible start/end parsing. Prefer the one-user log-work workflow for finished past work; when end is present this validates overlaps unless allow_overlap:true is passed.", map[string]any{"type": "object", "required": []string{"start"}, "properties": map[string]any{
			"start":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"timezone":      timezoneInputProperty(),
			"description":   map[string]any{"type": "string"},
			"project":       map[string]any{"type": "string", "description": "Project name or ID"},
			"project_id":    map[string]any{"type": "string"},
			"task_id":       map[string]any{"type": "string"},
			"tag_ids":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional Clockify tag IDs to attach to the entry"},
			"billable":      nullableBoolSchema("Optional billable flag. Omit or pass null to inherit the project/workspace default."),
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
			"task_id":     map[string]any{"type": "string"},
			"tag_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Replace the entry's tag IDs"},
			"start":       map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"timezone":    timezoneInputProperty(),
			"billable":    nullableBoolSchema("Optional billable flag. Omit or pass null to leave the existing value unchanged."),
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
		{Tool: toolRWIdem(legacyToolFindAndUpdateEntry, "Find one current-user entry by exact ID or safe filters, then update selected fields. Fails closed on ambiguous matches.", map[string]any{"type": "object", "properties": map[string]any{"entry_id": map[string]any{"type": "string"}, "description_contains": map[string]any{"type": "string"}, "exact_description": map[string]any{"type": "string"}, "start_after": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "start_before": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "new_description": map[string]any{"type": "string"}, "project_id": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}, "task_id": map[string]any{"type": "string"}, "tag_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Replace the entry's tag IDs"}, "start": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "end": map[string]any{"type": "string", "description": flexibleDatetimeDescription}, "timezone": timezoneInputProperty(), "billable": nullableBoolSchema("Optional billable flag. Omit or pass null to leave the existing value unchanged."), "dry_run": map[string]any{"type": "boolean"}}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.FindAndUpdateEntry(ctx, args)
		}},
		{Tool: toolRW("clockify_create_project", "Create a new project", map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
			"name":                      map[string]any{"type": "string", "minLength": 1, "maxLength": 150},
			"client":                    map[string]any{"type": "string", "description": "Client name or ID"},
			"client_id":                 map[string]any{"type": "string", "description": "Client ID; bypasses name resolution when supplied"},
			"color":                     map[string]any{"type": "string", "description": "Hex color code"},
			"billable":                  map[string]any{"type": "boolean"},
			"is_public":                 map[string]any{"type": "boolean"},
			"note":                      map[string]any{"type": "string", "maxLength": 500},
			"cost_rate":                 rateRequestInputSchema("Project cost rate"),
			"hourly_rate":               rateRequestInputSchema("Project hourly rate"),
			"estimate":                  estimateRequestInputSchema(),
			"memberships":               map[string]any{"type": "array", "items": membershipInputSchema()},
			"tasks":                     map[string]any{"type": "array", "items": taskRequestInputSchema()},
			"budget_estimate":           estimateWithOptionsInputSchema(),
			"time_estimate":             timeEstimateInputSchema(),
			"financial_start":           map[string]any{"type": "string", "description": "Optional Reports API enrichment range start for the returned project view."},
			"financial_end":             map[string]any{"type": "string", "description": "Optional Reports API enrichment range end for the returned project view."},
			"financial_date_range_type": map[string]any{"type": "string", "enum": reportDateRangeTypeEnums, "description": "Optional Reports API dateRangeType override for returned project financials."},
			"financial_timezone":        timezoneInputProperty(),
			"dry_run":                   map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateProject(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_project", "Update a project by ID or exact name using a fetch-then-merge strategy. Clockify's PUT is a full replacement; caller-supplied empty strings are treated as 'do not change'. Use the dedicated archived boolean to flip archival state.", map[string]any{"type": "object", "required": []string{"project"}, "properties": map[string]any{
			"project":                   map[string]any{"type": "string", "description": "Project name or ID"},
			"name":                      map[string]any{"type": "string", "minLength": 1, "maxLength": 150},
			"client":                    map[string]any{"type": "string", "description": "Client name or ID"},
			"client_id":                 map[string]any{"type": "string", "description": "Client ID; bypasses name resolution when supplied"},
			"color":                     map[string]any{"type": "string", "description": "Hex color code"},
			"note":                      map[string]any{"type": "string", "maxLength": 500},
			"billable":                  map[string]any{"type": "boolean"},
			"is_public":                 map[string]any{"type": "boolean"},
			"archived":                  map[string]any{"type": "boolean"},
			"cost_rate":                 rateRequestInputSchema("Project cost rate"),
			"hourly_rate":               rateRequestInputSchema("Project hourly rate"),
			"estimate":                  estimateRequestInputSchema(),
			"memberships":               map[string]any{"type": "array", "items": membershipInputSchema()},
			"tasks":                     map[string]any{"type": "array", "items": taskRequestInputSchema()},
			"budget_estimate":           estimateWithOptionsInputSchema(),
			"estimate_reset":            estimateResetInputSchema(),
			"time_estimate":             timeEstimateInputSchema(),
			"financial_start":           map[string]any{"type": "string", "description": "Optional Reports API enrichment range start for the returned project view."},
			"financial_end":             map[string]any{"type": "string", "description": "Optional Reports API enrichment range end for the returned project view."},
			"financial_date_range_type": map[string]any{"type": "string", "enum": reportDateRangeTypeEnums, "description": "Optional Reports API dateRangeType override for returned project financials."},
			"financial_timezone":        timezoneInputProperty(),
			"dry_run":                   map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UpdateProject(ctx, args)
		}},
		{Tool: toolDestructive("clockify_delete_project", "Delete a project by ID or exact name. Archives the project first if it is still active (Clockify rejects DELETE on active projects).", map[string]any{"type": "object", "required": []string{"project"}, "properties": map[string]any{
			"project": map[string]any{"type": "string", "description": "Project name or ID"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeleteProject(ctx, args)
		}},
		{Tool: toolRW("clockify_create_client", "Create a new client. Accepts optional address, email, and note alongside the required name.", clientCreateInputSchema()), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateClient(ctx, args)
		}},
		{Tool: toolDestructive("clockify_delete_client", "Delete a client by ID or exact name. Archives the client first if it is still active (Clockify rejects DELETE on active clients).", map[string]any{"type": "object", "required": []string{"client"}, "properties": map[string]any{
			"client":  map[string]any{"type": "string", "description": "Client name or ID"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeleteClient(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_client", "Update a client by ID or exact name using a fetch-then-merge strategy. Clockify's PUT is a full replacement; caller-supplied empty strings are treated as 'do not change'. Use the dedicated archived boolean to flip archival state.", clientUpdateInputSchema()), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
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
			"project":                   map[string]any{"type": "string", "description": "Project name or ID"},
			"project_id":                map[string]any{"type": "string"},
			"name":                      map[string]any{"type": "string", "minLength": 1, "maxLength": 150},
			"assignee_id":               map[string]any{"type": "string", "description": "Deprecated upstream field"},
			"assignee_ids":              stringArraySchema("Task assignee IDs"),
			"budget_estimate":           map[string]any{"type": "integer", "minimum": 0},
			"contains_assignee":         map[string]any{"type": "boolean"},
			"billable":                  map[string]any{"type": "boolean"},
			"estimate":                  map[string]any{"type": "string", "description": "ISO-8601 duration estimate, e.g. PT2H30M"},
			"status":                    map[string]any{"type": "string", "enum": []string{"ACTIVE", "DONE", "ALL"}, "description": "Task status"},
			"user_group_ids":            stringArraySchema("Task user group IDs"),
			"financial_start":           map[string]any{"type": "string", "description": "Optional Reports API enrichment range start for the returned task view."},
			"financial_end":             map[string]any{"type": "string", "description": "Optional Reports API enrichment range end for the returned task view."},
			"financial_date_range_type": map[string]any{"type": "string", "enum": reportDateRangeTypeEnums, "description": "Optional Reports API dateRangeType override for returned task financials."},
			"financial_timezone":        timezoneInputProperty(),
			"dry_run":                   map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CreateTask(ctx, args)
		}},
		{Tool: toolRWIdem("clockify_update_task", "Update a task by project+task reference using fetch-then-merge. Clockify's PUT is a full replacement; caller-supplied empty strings are treated as 'do not change'.", map[string]any{"type": "object", "required": []string{"project", "task"}, "properties": map[string]any{
			"project":                   map[string]any{"type": "string", "description": "Project name or ID"},
			"task":                      map[string]any{"type": "string", "description": "Task ID or exact name"},
			"name":                      map[string]any{"type": "string", "minLength": 1, "maxLength": 150},
			"assignee_id":               map[string]any{"type": "string", "description": "Deprecated upstream field"},
			"assignee_ids":              stringArraySchema("Task assignee IDs"),
			"budget_estimate":           map[string]any{"type": "integer", "minimum": 0},
			"contains_assignee":         map[string]any{"type": "boolean"},
			"membership_status":         map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
			"status":                    map[string]any{"type": "string", "enum": []string{"ACTIVE", "DONE", "ALL"}, "description": "Task status"},
			"estimate":                  map[string]any{"type": "string", "description": "ISO-8601 duration estimate, e.g. PT2H30M"},
			"billable":                  map[string]any{"type": "boolean"},
			"user_group_ids":            stringArraySchema("Task user group IDs"),
			"financial_start":           map[string]any{"type": "string", "description": "Optional Reports API enrichment range start for the returned task view."},
			"financial_end":             map[string]any{"type": "string", "description": "Optional Reports API enrichment range end for the returned task view."},
			"financial_date_range_type": map[string]any{"type": "string", "enum": reportDateRangeTypeEnums, "description": "Optional Reports API dateRangeType override for returned task financials."},
			"financial_timezone":        timezoneInputProperty(),
			"dry_run":                   map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UpdateTask(ctx, args)
		}},
		{Tool: toolDestructive("clockify_delete_task", "Delete a task by project+task reference. The MCP marks active tasks DONE before DELETE because Clockify requires completed tasks for deletion.", map[string]any{"type": "object", "required": []string{"project", "task"}, "properties": map[string]any{
			"project": map[string]any{"type": "string", "description": "Project name or ID"},
			"task":    map[string]any{"type": "string", "description": "Task ID or exact name"},
			"dry_run": map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DeleteTask(ctx, args)
		}},

		// --- Wave 2 additions ---
		{Tool: toolRO(legacyToolTimerStatus, "Check if a timer is currently running and show elapsed time", map[string]any{"type": "object"}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			return s.TimerStatus(ctx)
		}},
		{Tool: toolRW(legacyToolSwitchProject, "Stop the current timer if one is running, then start a new one on a different project. Response includes previous_entry_action/stop_outcome and the stopped entry when one was finalized.", map[string]any{"type": "object", "properties": map[string]any{
			"project":     map[string]any{"type": "string", "description": "Project name or ID to switch to"},
			"project_id":  map[string]any{"type": "string", "description": "Project ID alias for project; bypasses name resolution when supplied"},
			"description": map[string]any{"type": "string"},
			"task_id":     map[string]any{"type": "string"},
			"billable":    nullableBoolSchema("Optional billable flag. Omit or pass null to inherit the project/workspace default."),
			"dry_run":     map[string]any{"type": "boolean"},
		}}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.SwitchProject(ctx, args)
		}},
		{Tool: toolRO("clockify_attendance_report", "Generate a Clockify attendance report via the Reports API", reportInputSchema("attendance_filter", attendanceFilterSchema())), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.AttendanceReport(ctx, args)
		}},
		{Tool: toolRO("clockify_detailed_report", "Generate a Clockify detailed report via the Reports API", reportInputSchema("detailed_filter", detailedFilterSchema())), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.DetailedReport(ctx, args)
		}},
		{Tool: toolRO(legacyToolResolveName, "Resolve a project, client, tag, user, or project-scoped task name/email to a Clockify ID. Prefer this before write tools when a user gives a name.", resolveNameInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ResolveName(ctx, args)
		}},
		{Tool: toolRO("clockify_resolve_debug", "Deprecated compatibility alias for the legacy name resolver.", resolveNameInputSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ResolveDebug(ctx, args)
		}},
	}))
}
