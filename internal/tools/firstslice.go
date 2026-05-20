package tools

import (
	"context"

	"github.com/apet97/go-clockify/internal/mcp"
)

func (s *Service) FirstSliceRegistry() []mcp.ToolDescriptor {
	// clockify_status, clockify_demo_seed, and clockify_demo_cleanup are
	// registered by workflowDescriptors(), which buildFullAccessRegistry
	// appends first; dedupeToolDescriptors keeps that (richer, workflow-
	// annotated) copy, so registering them here too only wasted a cold-
	// start allocation.
	descriptors := []mcp.ToolDescriptor{
		firstSliceDescriptor(20, toolRO("clockify_clients_list", "List clients in the pinned workspace, paginated via page and page_size.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"name":        map[string]any{"type": "string"},
				"archived":    map[string]any{"type": "boolean"},
				"address":     map[string]any{"type": "string"},
				"note":        map[string]any{"type": "string"},
				"sort_column": map[string]any{"type": "string", "description": "Clockify sort-column query value."},
				"sort_order":  map[string]any{"type": "string", "description": "Clockify sort-order query value."},
			},
		})), s.ClientsList),
		firstSliceDescriptor(21, toolRW("clockify_clients_create", "Create a client in the pinned workspace.", objectSchema(map[string]any{
			"required": []string{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		})), s.ClientsCreate),
		firstSliceDescriptor(30, toolRO("clockify_projects_list", "List projects in the pinned workspace, paginated via page and page_size.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"name":               map[string]any{"type": "string"},
				"strict_name_search": map[string]any{"type": "boolean"},
				"archived":           map[string]any{"type": "boolean"},
				"billable":           map[string]any{"type": "boolean"},
				"clients":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"contains_client":    map[string]any{"type": "boolean"},
				"client_status":      map[string]any{"type": "string", "enum": []string{"ACTIVE", "ARCHIVED", "ALL"}},
				"users":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"contains_user":      map[string]any{"type": "boolean"},
				"user_status":        map[string]any{"type": "string", "enum": []string{"PENDING", "ACTIVE", "DECLINED", "INACTIVE", "ALL"}},
				"is_template":        map[string]any{"type": "boolean"},
				"sort_column":        map[string]any{"type": "string", "enum": []string{"ID", "NAME", "CLIENT_NAME", "DURATION", "BUDGET", "PROGRESS"}},
				"sort_order":         map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
				"hydrated":           map[string]any{"type": "boolean", "description": "Clockify hydrated query flag; defaults to false for compact list responses."},
				"access":             map[string]any{"type": "string", "enum": []string{"PUBLIC", "PRIVATE"}},
				"expense_limit":      map[string]any{"type": "integer"},
				"expense_date":       map[string]any{"type": "string", "description": "Expense date query value, typically YYYY-MM-DD."},
				"user_groups":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User group IDs for the Clockify userGroups[] query."},
				"contains_group":     map[string]any{"type": "boolean"},
			},
		})), s.ProjectsList),
		firstSliceDescriptor(31, toolRW("clockify_projects_create", "Create a project in the pinned workspace.", objectSchema(map[string]any{
			"required": []string{"name"},
			"properties": map[string]any{
				"name":      map[string]any{"type": "string"},
				"client_id": map[string]any{"type": "string"},
				"client":    map[string]any{"type": "string", "description": "Client name or ID."},
				"color":     map[string]any{"type": "string", "description": "Hex color code."},
				"billable":  map[string]any{"type": "boolean"},
				"is_public": map[string]any{"type": "boolean"},
			},
		})), s.ProjectsCreate),
		firstSliceDescriptor(40, toolRO("clockify_tasks_list", "List tasks for a project, paginated via page and page_size.", paginationSchema(map[string]any{
			"required": []string{"project"},
			"properties": map[string]any{
				"project_id":         map[string]any{"type": "string"},
				"project":            map[string]any{"type": "string", "description": "Project name or ID."},
				"name":               map[string]any{"type": "string"},
				"strict_name_search": map[string]any{"type": "boolean"},
				"is_active":          map[string]any{"type": "boolean"},
				"sort_column":        map[string]any{"type": "string", "enum": []string{"ID", "NAME"}},
				"sort_order":         map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
			},
		})), s.TasksList),
		firstSliceDescriptor(41, toolRW("clockify_tasks_create", "Create a task under a project.", objectSchema(map[string]any{
			"required": []string{"name", "project"},
			"properties": map[string]any{
				"project_id":        map[string]any{"type": "string"},
				"project":           map[string]any{"type": "string", "description": "Project name or ID."},
				"name":              map[string]any{"type": "string"},
				"billable":          map[string]any{"type": "boolean"},
				"contains_assignee": map[string]any{"type": "boolean"},
			},
		})), s.TasksCreate),
		firstSliceDescriptor(50, toolRO("clockify_tags_list", "List tags in the pinned workspace, paginated via page and page_size.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"name":               map[string]any{"type": "string"},
				"strict_name_search": map[string]any{"type": "boolean"},
				"excluded_ids":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"archived":           map[string]any{"type": "boolean"},
				"sort_column":        map[string]any{"type": "string", "description": "Clockify sort-column query value."},
				"sort_order":         map[string]any{"type": "string", "description": "Clockify sort-order query value."},
			},
		})), s.TagsList),
		firstSliceDescriptor(51, toolRW("clockify_tags_create", "Create a tag in the pinned workspace. Retrying after a network timeout can create a duplicate tag; check with clockify_tags_list before retrying.", objectSchema(map[string]any{
			"required": []string{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		})), s.TagsCreate),
		defaultTier(firstSliceDescriptor(60, toolRO("clockify_entries_list", "List current-user time entries in the pinned workspace, paginated via page and page_size.", paginationSchema(map[string]any{
			"properties": map[string]any{
				"start":            map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":              map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"project_id":       map[string]any{"type": "string"},
				"project":          map[string]any{"type": "string", "description": "Project name or ID."},
				"description":      map[string]any{"type": "string"},
				"task":             map[string]any{"type": "string"},
				"tags":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"project_required": map[string]any{"type": "boolean"},
				"task_required":    map[string]any{"type": "boolean"},
				"hydrated":         map[string]any{"type": "boolean"},
				"in_progress":      map[string]any{"type": "string"},
				"get_week_before":  map[string]any{"type": "string"},
			},
		})), s.EntriesList)),
		firstSliceDescriptor(61, toolRW("clockify_entries_create", "Create a current-user time entry in the pinned workspace. This is a direct create with no overlap guard; use clockify_log_work for overlap-protected logging.", objectSchema(map[string]any{
			"required": []string{"start"},
			"properties": map[string]any{
				"start":       map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"end":         map[string]any{"type": "string", "description": flexibleDatetimeDescription},
				"description": map[string]any{"type": "string"},
				"project_id":  map[string]any{"type": "string"},
				"project":     map[string]any{"type": "string", "description": "Project name or ID."},
				"task_id":     map[string]any{"type": "string"},
				"task":        map[string]any{"type": "string", "description": "Task name. Requires project/project_id."},
				"tag_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"tag":         map[string]any{"type": "string", "description": "Tag name or ID."},
				"billable":    map[string]any{"type": "boolean"},
				"type":        map[string]any{"type": "string", "description": "Entry type: REGULAR (default) or BREAK."},
				"dry_run":     map[string]any{"type": "boolean", "description": "Preview the create payload without POSTing to Clockify."},
			},
		})), s.EntriesCreate),
	}
	return normalizeDescriptors(descriptors)
}

func firstSliceDescriptor(priority int, tool mcp.Tool, handler mcp.ToolHandler) mcp.ToolDescriptor {
	if tool.Annotations == nil {
		tool.Annotations = map[string]any{}
	}
	tool.Annotations["priority"] = priority
	if _, ok := tool.Annotations["handlerKind"]; !ok {
		tool.Annotations["handlerKind"] = "native handler"
	}
	tool.OutputSchema = firstSliceOutputSchema(tool.Name, firstSliceDataOutputSchema(tool.Name))
	return mcp.ToolDescriptor{Tool: tool, Handler: firstSliceHandler(tool.Name, handler)}
}

func firstSliceHandler(action string, handler mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, args map[string]any) (any, error) {
		result, err := handler(ctx, args)
		if err != nil {
			return recoverable(action, err, defaultRecovery(action, args)), nil
		}
		return result, nil
	}
}
