package tools

import (
	"context"

	"github.com/apet97/go-clockify/internal/mcp"
)

func (s *Service) nativeCoreDescriptors() []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		nativeDomainTool(22, toolRO("clockify_clients_get", "Get one client by name or ID.", objectSchema(map[string]any{"required": []string{"client"}, "properties": map[string]any{
			"client":    map[string]any{"type": "string", "description": "Client name or ID."},
			"client_id": map[string]any{"type": "string", "description": "Client ID."},
		}})), "client", "", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.GetClientWithArgs(ctx, aliasArg(args, "client_id", "client"))
		}),
		nativeDomainTool(23, toolRWIdem("clockify_clients_update", "Update a client by name or ID.", objectSchema(map[string]any{"required": []string{"client"}, "properties": map[string]any{
			"client":             map[string]any{"type": "string", "description": "Client name or ID."},
			"client_id":          map[string]any{"type": "string", "description": "Client ID."},
			"name":               map[string]any{"type": "string"},
			"email":              map[string]any{"type": "string"},
			"address":            map[string]any{"type": "string"},
			"note":               map[string]any{"type": "string"},
			"currency_id":        map[string]any{"type": "string"},
			"cc_emails":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"archived":           map[string]any{"type": "boolean"},
			"archive_projects":   map[string]any{"type": "boolean"},
			"mark_tasks_as_done": map[string]any{"type": "boolean"},
		}})), "client", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.UpdateClient(ctx, aliasArg(args, "client_id", "client"))
		}),
		nativeDomainTool(24, toolDestructive("clockify_clients_delete", "Permanently delete a client by name or ID. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"client"}, "properties": map[string]any{
			"client":    map[string]any{"type": "string", "description": "Client name or ID."},
			"client_id": map[string]any{"type": "string", "description": "Client ID."},
			"dry_run":   map[string]any{"type": "boolean"},
		}})), "client", "deleted", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.DeleteClient(ctx, aliasArg(args, "client_id", "client"))
		}),

		nativeDomainTool(32, toolRO("clockify_projects_get", "Get one project by name or ID.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":                  map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id":               map[string]any{"type": "string", "description": "Project ID."},
			"hydrated":                 map[string]any{"type": "boolean", "description": "Clockify hydrated query flag; defaults to false for compact list responses."},
			"custom_field_entity_type": map[string]any{"type": "string"},
			"expense_limit":            map[string]any{"type": "integer"},
			"expense_date":             map[string]any{"type": "string", "description": "Expense date query value, typically YYYY-MM-DD."},
		}})), "project", "", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.GetProject(ctx, aliasArg(args, "project_id", "project"))
		}),
		nativeDomainTool(33, toolRWIdem("clockify_projects_update", "Update a project by name or ID.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
			"name":       map[string]any{"type": "string"},
			"client":     map[string]any{"type": "string", "description": "Client name or ID."},
			"client_id":  map[string]any{"type": "string"},
			"color":      map[string]any{"type": "string"},
			"billable":   map[string]any{"type": "boolean"},
			"is_public":  map[string]any{"type": "boolean"},
			"archived":   map[string]any{"type": "boolean"},
			"note":       map[string]any{"type": "string"},
		}})), "project", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.UpdateProject(ctx, aliasArg(args, "project_id", "project"))
		}),
		nativeDomainTool(34, toolDestructive("clockify_projects_delete", "Permanently delete a project by name or ID. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
			"dry_run":    map[string]any{"type": "boolean"},
		}})), "project", "deleted", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.DeleteProject(ctx, aliasArg(args, "project_id", "project"))
		}),
		nativeDomainTool(35, toolRWIdem("clockify_projects_archive", "Archive a project by name or ID. Destructive safety hint: archiving removes the project from active work.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
		}})), "project", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			args = aliasArg(args, "project_id", "project")
			args["archived"] = true
			return s.UpdateProject(ctx, args)
		}),
		nativeDomainTool(41, toolRWIdem("clockify_projects_rates_update", "Set a project member hourly or cost rate. Admin and billing impact: rate changes flow through every future time entry on this project.", objectSchema(map[string]any{"required": []string{"project_id", "user_id", "rate_kind", "amount"}, "properties": map[string]any{
			"project_id": map[string]any{"type": "string"},
			"user_id":    map[string]any{"type": "string"},
			"rate_kind":  map[string]any{"type": "string", "enum": []string{"hourly", "cost"}},
			"amount":     map[string]any{"type": "integer", "minimum": 0, "description": "Rate amount in minor currency units (cents): 1000 = $10.00/hr."},
			"since":      map[string]any{"type": "string"},
			"dry_run":    map[string]any{"type": "boolean"},
		}})), "project_rate", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			endpoint, err := rateKindEndpoint(args)
			if err != nil {
				return ResultEnvelope{}, err
			}
			return s.UpdateProjectUserRate(ctx, args, endpoint)
		}),

		nativeDomainTool(42, toolRO("clockify_tasks_get", "Get one task by name or ID within a project.", objectSchema(map[string]any{"required": []string{"project", "task"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
			"task":       map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":    map[string]any{"type": "string", "description": "Task ID."},
		}})), "task", "", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.GetTask(ctx, aliasArgs(args, map[string]string{"project_id": "project", "task_id": "task"}))
		}),
		nativeDomainTool(43, toolRWIdem("clockify_tasks_update", "Update a task by name or ID within a project.", objectSchema(map[string]any{"required": []string{"project", "task"}, "properties": map[string]any{
			"project":           map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id":        map[string]any{"type": "string", "description": "Project ID."},
			"task":              map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":           map[string]any{"type": "string", "description": "Task ID."},
			"name":              map[string]any{"type": "string"},
			"billable":          map[string]any{"type": "boolean"},
			"assignee_ids":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"status":            map[string]any{"type": "string", "enum": []string{"ACTIVE", "DONE"}},
			"contains_assignee": map[string]any{"type": "boolean"},
			"membership_status": map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
		}})), "task", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.UpdateTask(ctx, aliasArgs(args, map[string]string{"project_id": "project", "task_id": "task"}))
		}),
		nativeDomainTool(44, toolDestructive("clockify_tasks_delete", "Permanently delete a task by name or ID within a project. A task that is not DONE is marked DONE first (Clockify requires it), then deleted. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"project", "task"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string", "description": "Project ID."},
			"task":       map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":    map[string]any{"type": "string", "description": "Task ID."},
			"dry_run":    map[string]any{"type": "boolean"},
		}})), "task", "deleted", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.DeleteTask(ctx, aliasArgs(args, map[string]string{"project_id": "project", "task_id": "task"}))
		}),
		nativeDomainTool(45, toolRWIdem("clockify_tasks_rates_update", "Set a task hourly or cost rate. Billing impact: changes the billable amount on every future time entry charged to this task.", objectSchema(map[string]any{"required": []string{"rate_kind", "amount", "project", "task"}, "properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id": map[string]any{"type": "string"},
			"task":       map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":    map[string]any{"type": "string"},
			"rate_kind":  map[string]any{"type": "string", "enum": []string{"hourly", "cost"}},
			"amount":     map[string]any{"type": "integer", "minimum": 0, "description": "Rate amount in minor currency units (cents): 1000 = $10.00/hr."},
			"since":      map[string]any{"type": "string"},
			"dry_run":    map[string]any{"type": "boolean"},
		}})), "task_rate", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			endpoint, err := rateKindEndpoint(args)
			if err != nil {
				return ResultEnvelope{}, err
			}
			return s.UpdateTaskRate(ctx, args, endpoint)
		}),

		nativeDomainTool(52, toolRO("clockify_tags_get", "Get one tag by name or ID.", objectSchema(map[string]any{"required": []string{"tag"}, "properties": map[string]any{
			"tag":    map[string]any{"type": "string", "description": "Tag name or ID."},
			"tag_id": map[string]any{"type": "string", "description": "Tag ID."},
		}})), "tag", "", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.GetTag(ctx, aliasArg(args, "tag_id", "tag"))
		}),
		nativeDomainTool(53, toolRWIdem("clockify_tags_update", "Update a tag by name or ID.", objectSchema(map[string]any{"required": []string{"tag"}, "properties": map[string]any{
			"tag":      map[string]any{"type": "string", "description": "Tag name or ID."},
			"tag_id":   map[string]any{"type": "string", "description": "Tag ID."},
			"name":     map[string]any{"type": "string"},
			"archived": map[string]any{"type": "boolean"},
		}})), "tag", "updated", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.UpdateTag(ctx, aliasArg(args, "tag_id", "tag"))
		}),
		nativeDomainTool(54, toolDestructive("clockify_tags_delete", "Permanently delete a tag by name or ID. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"tag"}, "properties": map[string]any{
			"tag":     map[string]any{"type": "string", "description": "Tag name or ID."},
			"tag_id":  map[string]any{"type": "string", "description": "Tag ID."},
			"dry_run": map[string]any{"type": "boolean"},
		}})), "tag", "deleted", func(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
			return s.DeleteTag(ctx, aliasArg(args, "tag_id", "tag"))
		}),

		nativeDomainTool(62, toolRO("clockify_entries_get", "Get one time entry by ID.", objectSchema(map[string]any{"required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id": map[string]any{"type": "string"},
		}})), "entry", "", s.GetEntry),
		nativeDomainTool(63, toolRWIdem("clockify_entries_update", "Update a time entry by ID.", objectSchema(map[string]any{"required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id":    map[string]any{"type": "string"},
			"start":       map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"end":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
			"description": map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string", "description": "Project name or ID."},
			"project_id":  map[string]any{"type": "string"},
			"task":        map[string]any{"type": "string", "description": "Task name or ID."},
			"task_id":     map[string]any{"type": "string"},
			"tag":         map[string]any{"type": "string", "description": "Tag name or ID."},
			"tag_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"billable":    map[string]any{"type": "boolean"},
		}})), "entry", "updated", s.UpdateEntry),
		nativeDomainTool(64, toolDestructive("clockify_entries_delete", "Permanently delete a time entry by ID. Destructive; supports dry_run preview.", objectSchema(map[string]any{"required": []string{"entry_id"}, "properties": map[string]any{
			"entry_id": map[string]any{"type": "string"},
			"dry_run":  map[string]any{"type": "boolean"},
		}})), "entry", "deleted", s.DeleteEntry),

		nativeDomainTool(1100, toolRO("clockify_users_list", "List users in the pinned workspace, paginated via page and page_size.", userListSchema()), "user", "", s.ListUsers),
		nativeDomainTool(1101, toolRO("clockify_users_profile", "Get the current Clockify user.", objectSchema(nil)), "user", "", func(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
			return s.CurrentUser(ctx)
		}),
		nativeDomainTool(1105, toolRO("clockify_workspace_settings", "Read pinned workspace settings.", objectSchema(nil)), "workspace", "", func(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
			return s.GetWorkspace(ctx)
		}),
	}
}

func userListSchema() map[string]any {
	return paginationSchema(map[string]any{"properties": map[string]any{
		"email":            map[string]any{"type": "string"},
		"project_id":       map[string]any{"type": "string"},
		"status":           map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
		"account_statuses": map[string]any{"type": "string"},
		"name":             map[string]any{"type": "string"},
		"sort_column":      map[string]any{"type": "string", "enum": []string{"ID", "EMAIL", "NAME", "NAME_LOWERCASE", "ACCESS", "HOURLYRATE", "COSTRATE"}},
		"sort_order":       map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
		"memberships":      map[string]any{"type": "string", "enum": []string{"ALL", "NONE", "WORKSPACE", "PROJECT", "USERGROUP"}},
		"include_roles":    map[string]any{"type": "boolean"},
	}})
}
