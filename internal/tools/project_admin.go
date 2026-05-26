package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

const (
	projectTemplateListNoteLimit  = 280
	projectTemplateListNoteSuffix = "..."
)

func projectAdminHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List project templates
		{
			Tool: toolRO("clockify_list_project_templates",
				"List project templates in the workspace with pagination",
				paginationSchema(nil)),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListProjectTemplates(ctx, args)
			},
		},
		// 2. Get project template by ID
		{
			Tool: toolRO("clockify_get_project_template",
				"Get a project template by ID",
				requiredSchema("project_id")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.GetProjectTemplate(ctx, args)
			},
		},
		// 3. Create project template
		{
			Tool: toolRW("clockify_create_project_template",
				"Create a new project template",
				map[string]any{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]any{
						"name":      map[string]any{"type": "string", "description": "Template project name"},
						"color":     map[string]any{"type": "string", "description": "Hex color code"},
						"billable":  map[string]any{"type": "boolean"},
						"is_public": map[string]any{"type": "boolean"},
						"client_id": map[string]any{"type": "string", "description": "Associated client ID"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateProjectTemplate(ctx, args)
			},
		},
		// 4. Update project estimate
		{
			Tool: toolRW("clockify_create_project_from_template",
				"Create a project from an existing project template",
				map[string]any{
					"type":     "object",
					"required": []string{"name", "template_project_id"},
					"properties": map[string]any{
						"name":                map[string]any{"type": "string", "minLength": 1, "maxLength": 150},
						"template_project_id": map[string]any{"type": "string"},
						"client_id":           map[string]any{"type": "string"},
						"color":               map[string]any{"type": "string", "description": "Hex color code"},
						"is_public":           map[string]any{"type": "boolean"},
						"dry_run":             map[string]any{"type": "boolean"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateProjectFromTemplate(ctx, args)
			},
		},
		{
			Tool: toolRW("clockify_update_project_estimate",
				"Update a project's documented estimate fields",
				map[string]any{
					"type":     "object",
					"required": []string{"project_id"},
					"properties": map[string]any{
						"project_id":      map[string]any{"type": "string"},
						"budget_estimate": estimateWithOptionsInputSchema(),
						"estimate_reset":  estimateResetInputSchema(),
						"time_estimate":   timeEstimateInputSchema(),
						"estimate_type":   map[string]any{"type": "string", "enum": []string{"TIME", "BUDGET"}, "description": "Deprecated alias; use time_estimate or budget_estimate"},
						"estimate_value":  map[string]any{"type": "number", "description": "Deprecated alias; seconds for TIME, raw amount for BUDGET"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateProjectEstimate(ctx, args)
			},
		},
		// 5. Update project memberships (documented PATCH /memberships)
		{
			Tool: toolRWIdem("clockify_update_project_memberships",
				"Replace project memberships and optional user-group filter/rate metadata",
				map[string]any{
					"type":     "object",
					"required": []string{"project_id", "memberships"},
					"properties": map[string]any{
						"project_id":  map[string]any{"type": "string"},
						"memberships": map[string]any{"type": "array", "items": membershipInputSchema()},
						"user_groups": userGroupsInputSchema(),
						"dry_run":     map[string]any{"type": "boolean"},
					},
				}),
			ReadOnlyHint: false, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateProjectMemberships(ctx, args)
			},
		},
		// 6. Backward-compatible alias for update_project_memberships.
		{
			Tool: toolRW("clockify_set_project_memberships",
				"Set project memberships with optional hourly rates",
				map[string]any{
					"type":     "object",
					"required": []string{"project_id", "user_ids"},
					"properties": map[string]any{
						"project_id":  map[string]any{"type": "string"},
						"user_ids":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User IDs to set as members"},
						"hourly_rate": map[string]any{"type": "number", "description": "Hourly rate for all members (optional)"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.SetProjectMemberships(ctx, args)
			},
		},
		// 7. Assign/remove memberships (documented POST /memberships)
		{
			Tool: toolRW("clockify_assign_project_memberships",
				"Assign or remove users and user groups on a project",
				map[string]any{
					"type":     "object",
					"required": []string{"project_id"},
					"properties": map[string]any{
						"project_id":  map[string]any{"type": "string"},
						"remove":      map[string]any{"type": "boolean"},
						"user_ids":    stringArraySchema("User IDs to assign or remove"),
						"user_groups": userGroupsInputSchema(),
						"dry_run":     map[string]any{"type": "boolean"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.AssignProjectMemberships(ctx, args)
			},
		},
		// 8. Toggle project template flag
		{
			Tool: toolRWIdem("clockify_update_project_template",
				"Update whether a project is a template",
				map[string]any{
					"type":     "object",
					"required": []string{"project_id", "is_template"},
					"properties": map[string]any{
						"project_id":  map[string]any{"type": "string"},
						"is_template": map[string]any{"type": "boolean"},
						"dry_run":     map[string]any{"type": "boolean"},
					},
				}),
			ReadOnlyHint: false, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateProjectTemplate(ctx, args)
			},
		},
		// 9. Project user rates
		projectUserRateDescriptor("clockify_update_project_user_cost_rate", "Update a project user's cost rate", "cost-rate", func(ctx context.Context, args map[string]any) (ToolResult, error) {
			return s.UpdateProjectUserRate(ctx, args, "cost-rate")
		}),
		projectUserRateDescriptor("clockify_update_project_user_hourly_rate", "Update a project user's billable hourly rate", "hourly-rate", func(ctx context.Context, args map[string]any) (ToolResult, error) {
			return s.UpdateProjectUserRate(ctx, args, "hourly-rate")
		}),
		// 10. Task rates
		taskRateDescriptor("clockify_update_task_cost_rate", "Update a task cost rate", "cost-rate", func(ctx context.Context, args map[string]any) (ToolResult, error) {
			return s.UpdateTaskRate(ctx, args, "cost-rate")
		}),
		taskRateDescriptor("clockify_update_task_hourly_rate", "Update a task billable hourly rate", "hourly-rate", func(ctx context.Context, args map[string]any) (ToolResult, error) {
			return s.UpdateTaskRate(ctx, args, "hourly-rate")
		}),
		// 11. Archive projects
		{
			Tool: toolRW("clockify_archive_projects",
				"Archive multiple projects by setting them to archived state",
				map[string]any{
					"type":     "object",
					"required": []string{"project_ids"},
					"properties": map[string]any{
						"project_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Array of project IDs to archive"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ArchiveProjects(ctx, args)
			},
		},
	}
}

func projectUserRateDescriptor(name, desc, endpoint string, handler func(context.Context, map[string]any) (ToolResult, error)) mcp.ToolDescriptor {
	return mcp.ToolDescriptor{
		Tool: toolRWIdem(name, desc, map[string]any{
			"type":     "object",
			"required": []string{"project_id", "user_id", "amount"},
			"properties": map[string]any{
				"project_id": map[string]any{"type": "string"},
				"user_id":    map[string]any{"type": "string"},
				"amount":     map[string]any{"type": "integer", "minimum": 0, "description": "Raw upstream integer amount; no currency scaling is applied"},
				"since":      map[string]any{"type": "string", "description": "Clockify yyyy-MM-ddThh:mm:ssZ effective timestamp"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}),
		ReadOnlyHint: false, IdempotentHint: true,
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			_ = endpoint
			return handler(ctx, args)
		},
	}
}

func taskRateDescriptor(name, desc, endpoint string, handler func(context.Context, map[string]any) (ToolResult, error)) mcp.ToolDescriptor {
	return mcp.ToolDescriptor{
		Tool: toolRWIdem(name, desc, map[string]any{
			"type":     "object",
			"required": []string{"amount"},
			"properties": map[string]any{
				"project_id": map[string]any{"type": "string", "description": "Project ID; required unless project is supplied"},
				"project":    map[string]any{"type": "string", "description": "Project name or ID"},
				"task_id":    map[string]any{"type": "string", "description": "Task ID; required unless task is supplied"},
				"task":       map[string]any{"type": "string", "description": "Task ID or exact name"},
				"amount":     map[string]any{"type": "integer", "minimum": 0, "description": "Raw upstream integer amount; no currency scaling is applied"},
				"since":      map[string]any{"type": "string", "description": "Clockify yyyy-MM-ddThh:mm:ssZ effective timestamp"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}),
		ReadOnlyHint: false, IdempotentHint: true,
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			_ = endpoint
			return handler(ctx, args)
		},
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Service) ListProjectTemplates(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"is-template": "true",
		"page":        strconv.Itoa(page),
		"page-size":   strconv.Itoa(pageSize),
	}
	path, err := paths.Workspace(wsID, "projects")
	if err != nil {
		return ToolResult{}, err
	}
	var out []map[string]any
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ToolResult{}, err
	}
	out = compactProjectTemplateListNotes(out)
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
	}, args, page, pageSize)
	return ok("clockify_list_project_templates", out, emptyListMeta(meta, "clockify_projects_templates_create")), nil
}

func compactProjectTemplateListNotes(items []map[string]any) []map[string]any {
	compact := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := make(map[string]any, len(item)+1)
		for key, value := range item {
			row[key] = value
		}
		if note, ok := row["note"].(string); ok {
			if truncated, changed := truncateProjectTemplateListNote(note); changed {
				row["note"] = truncated
				row["noteTruncated"] = true
			}
		}
		compact = append(compact, row)
	}
	return compact
}

func truncateProjectTemplateListNote(note string) (string, bool) {
	runes := []rune(note)
	if len(runes) <= projectTemplateListNoteLimit {
		return note, false
	}
	keep := projectTemplateListNoteLimit - len([]rune(projectTemplateListNoteSuffix))
	if keep < 0 {
		keep = 0
	}
	return string(runes[:keep]) + projectTemplateListNoteSuffix, true
}

func (s *Service) GetProjectTemplate(ctx context.Context, args map[string]any) (ToolResult, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_get_project_template", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) CreateProjectTemplate(ctx context.Context, args map[string]any) (ToolResult, error) {
	name := stringArg(args, "name")
	if name == "" {
		return ToolResult{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	body := map[string]any{
		"name":       name,
		"isTemplate": true,
	}
	if color := stringArg(args, "color"); color != "" {
		body["color"] = color
	}
	if billable, ok := args["billable"].(bool); ok {
		body["billable"] = billable
	}
	if isPublic, ok := args["is_public"].(bool); ok {
		body["isPublic"] = isPublic
	}
	if clientID := stringArg(args, "client_id"); clientID != "" {
		if err := resolve.ValidateID(clientID, "client_id"); err != nil {
			return ToolResult{}, err
		}
		body["clientId"] = clientID
	}

	path, err := paths.Workspace(wsID, "projects")
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_create_project_template", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) CreateProjectFromTemplate(ctx context.Context, args map[string]any) (ToolResult, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ToolResult{}, fmt.Errorf("name is required")
	}
	templateID := strings.TrimSpace(stringArg(args, "template_project_id"))
	if err := resolve.ValidateID(templateID, "template_project_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	body := map[string]any{
		"name":              name,
		"templateProjectId": templateID,
	}
	setIfString(body, args, "client_id", "clientId")
	setIfString(body, args, "color", "color")
	setIfBool(body, args, "is_public", "isPublic")
	if dryrun.Enabled(args) {
		return ok("clockify_create_project_from_template", dryrun.Preview("clockify_create_project_from_template", body), map[string]any{"workspaceId": wsID}), nil
	}
	path, err := paths.Workspace(wsID, "projects", "from-template")
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ToolResult{}, err
	}
	if id, _ := out["id"].(string); id != "" {
		s.emitResourceUpdate(ctx, projectResourceURI(wsID, id))
	}
	return ok("clockify_create_project_from_template", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) UpdateProjectEstimate(ctx context.Context, args map[string]any) (ToolResult, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	body := map[string]any{}
	if budgetEstimate, ok, err := mapArg(args, "budget_estimate"); err != nil {
		return ToolResult{}, err
	} else if ok {
		body["budgetEstimate"] = normalizeEstimateWithOptionsRequest(budgetEstimate)
	}
	if estimateReset, ok, err := mapArg(args, "estimate_reset"); err != nil {
		return ToolResult{}, err
	} else if ok {
		body["estimateReset"] = normalizeEstimateResetRequest(estimateReset)
	}
	if timeEstimate, ok, err := mapArg(args, "time_estimate"); err != nil {
		return ToolResult{}, err
	} else if ok {
		body["timeEstimate"] = normalizeTimeEstimateRequest(timeEstimate)
	}
	if estType := strings.ToUpper(strings.TrimSpace(stringArg(args, "estimate_type"))); estType != "" {
		estValue, estOK := numberArg(args, "estimate_value")
		if !estOK {
			return ToolResult{}, fmt.Errorf("estimate_value is required when estimate_type is supplied")
		}
		switch estType {
		case "TIME":
			body["timeEstimate"] = map[string]any{
				"active":   true,
				"estimate": isoDurationFromSeconds(int64(estValue)),
				"type":     "MANUAL",
			}
		case "BUDGET":
			body["budgetEstimate"] = map[string]any{
				"active":   true,
				"estimate": estValue,
				"type":     "MANUAL",
			}
		default:
			return ToolResult{}, fmt.Errorf("estimate_type must be TIME or BUDGET")
		}
	}
	if len(body) == 0 {
		return ToolResult{}, fmt.Errorf("at least one estimate field is required")
	}
	if dryrun.Enabled(args) {
		return ok("clockify_update_project_estimate", dryrun.Preview("clockify_update_project_estimate", body), map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
	}

	path, err := paths.Workspace(wsID, "projects", projectID, "estimate")
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Patch(ctx, path, body, &out); err != nil {
		return ToolResult{}, err
	}
	s.emitResourceUpdateWithState(projectResourceURI(wsID, projectID), out)
	return ok("clockify_update_project_estimate", out, map[string]any{"workspaceId": wsID}), nil
}

func isoDurationFromSeconds(seconds int64) string {
	if seconds <= 0 {
		return "PT0S"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	out := "PT"
	if h > 0 {
		out += strconv.FormatInt(h, 10) + "H"
	}
	if m > 0 {
		out += strconv.FormatInt(m, 10) + "M"
	}
	if s > 0 || out == "PT" {
		out += strconv.FormatInt(s, 10) + "S"
	}
	return out
}

func (s *Service) UpdateProjectMemberships(ctx context.Context, args map[string]any) (ToolResult, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	body := map[string]any{}
	memberships, hasMemberships, err := mapSliceArg(args, "memberships")
	if err != nil {
		return ToolResult{}, err
	}
	if !hasMemberships || len(memberships) == 0 {
		return ToolResult{}, fmt.Errorf("memberships is required and must be a non-empty array")
	}
	outMemberships := make([]map[string]any, 0, len(memberships))
	for _, item := range memberships {
		outMemberships = append(outMemberships, normalizeMembershipRequest(item))
	}
	body["memberships"] = outMemberships
	if userGroups, hasUserGroups, err := mapArg(args, "user_groups"); err != nil {
		return ToolResult{}, err
	} else if hasUserGroups {
		body["userGroups"] = normalizeUserGroupsRequest(userGroups)
	}
	if dryrun.Enabled(args) {
		return ok("clockify_update_project_memberships", dryrun.Preview("clockify_update_project_memberships", body), map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
	}
	path, err := paths.Workspace(wsID, "projects", projectID, "memberships")
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Patch(ctx, path, body, &out); err != nil {
		return ToolResult{}, err
	}
	s.emitResourceUpdateWithState(projectResourceURI(wsID, projectID), out)
	return ok("clockify_update_project_memberships", out, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}

func (s *Service) SetProjectMemberships(ctx context.Context, args map[string]any) (ToolResult, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ToolResult{}, err
	}

	rawIDs, rawOk := args["user_ids"].([]any)
	if !rawOk || len(rawIDs) == 0 {
		return ToolResult{}, fmt.Errorf("user_ids is required and must be a non-empty array")
	}

	memberships := make([]map[string]any, 0, len(rawIDs))
	for _, raw := range rawIDs {
		uid, isStr := raw.(string)
		if !isStr {
			return ToolResult{}, fmt.Errorf("user_ids must contain only strings")
		}
		if err := resolve.ValidateID(uid, "user_id"); err != nil {
			return ToolResult{}, err
		}
		m := map[string]any{"userId": uid}
		if rate, rateOk := numberArg(args, "hourly_rate"); rateOk {
			m["hourlyRate"] = map[string]any{"amount": rate}
		}
		memberships = append(memberships, m)
	}

	aliasArgs := map[string]any{
		"project_id":  projectID,
		"memberships": mapsFromAny(memberships),
	}
	if dryrun.Enabled(args) {
		aliasArgs["dry_run"] = true
	}
	result, err := s.UpdateProjectMemberships(ctx, aliasArgs)
	if err != nil {
		return ToolResult{}, err
	}
	data := result.Data
	if out, ok := result.Data.(map[string]any); ok {
		if m, ok := out["memberships"]; ok {
			data = m
		}
	}
	return ok("clockify_set_project_memberships", data, map[string]any{
		"workspaceId": result.Meta["workspaceId"],
		"memberCount": len(memberships),
	}), nil
}

func mapsFromAny(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func (s *Service) AssignProjectMemberships(ctx context.Context, args map[string]any) (ToolResult, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	body := map[string]any{}
	setIfBool(body, args, "remove", "remove")
	if err := setIfStringSlice(body, args, "user_ids", "userIds"); err != nil {
		return ToolResult{}, err
	}
	if userGroups, ok, err := mapArg(args, "user_groups"); err != nil {
		return ToolResult{}, err
	} else if ok {
		body["userGroups"] = normalizeUserGroupsRequest(userGroups)
	}
	if len(body) == 0 {
		return ToolResult{}, fmt.Errorf("user_ids or user_groups is required")
	}
	if dryrun.Enabled(args) {
		return ok("clockify_assign_project_memberships", dryrun.Preview("clockify_assign_project_memberships", body), map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
	}
	path, err := paths.Workspace(wsID, "projects", projectID, "memberships")
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ToolResult{}, err
	}
	s.emitResourceUpdateWithState(projectResourceURI(wsID, projectID), out)
	return ok("clockify_assign_project_memberships", out, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}

func (s *Service) UpdateProjectTemplate(ctx context.Context, args map[string]any) (ToolResult, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ToolResult{}, err
	}
	isTemplate, hasTemplate := optionalBoolArg(args, "is_template")
	if !hasTemplate {
		return ToolResult{}, fmt.Errorf("is_template is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	body := map[string]any{"isTemplate": isTemplate}
	if dryrun.Enabled(args) {
		return ok("clockify_update_project_template", dryrun.Preview("clockify_update_project_template", body), map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
	}
	path, err := paths.Workspace(wsID, "projects", projectID, "template")
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Patch(ctx, path, body, &out); err != nil {
		return ToolResult{}, err
	}
	s.emitResourceUpdateWithState(projectResourceURI(wsID, projectID), out)
	return ok("clockify_update_project_template", out, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}

func (s *Service) UpdateProjectUserRate(ctx context.Context, args map[string]any, endpoint string) (ToolResult, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ToolResult{}, err
	}
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	body, err := rateBodyFromArgs(args)
	if err != nil {
		return ToolResult{}, err
	}
	action := "clockify_update_project_user_" + strings.ReplaceAll(endpoint, "-", "_")
	if dryrun.Enabled(args) {
		return ok(action, dryrun.Preview(action, body), map[string]any{"workspaceId": wsID, "projectId": projectID, "userId": userID}), nil
	}
	path, err := paths.Workspace(wsID, "projects", projectID, "users", userID, endpoint)
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ToolResult{}, err
	}
	s.emitResourceUpdateWithState(projectResourceURI(wsID, projectID), out)
	return ok(action, out, map[string]any{"workspaceId": wsID, "projectId": projectID, "userId": userID}), nil
}

func (s *Service) UpdateTaskRate(ctx context.Context, args map[string]any, endpoint string) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	if projectID == "" {
		projectRef := strings.TrimSpace(stringArg(args, "project"))
		if projectRef == "" {
			return ToolResult{}, fmt.Errorf("project_id or project is required")
		}
		projectID, err = s.resolveProjectID(ctx, wsID, projectRef)
		if err != nil {
			return ToolResult{}, err
		}
	}
	taskID := strings.TrimSpace(stringArg(args, "task_id"))
	if taskID == "" {
		taskRef := strings.TrimSpace(stringArg(args, "task"))
		if taskRef == "" {
			return ToolResult{}, fmt.Errorf("task_id or task is required")
		}
		taskID, err = s.resolveTaskID(ctx, wsID, projectID, taskRef)
		if err != nil {
			return ToolResult{}, err
		}
	}
	body, err := rateBodyFromArgs(args)
	if err != nil {
		return ToolResult{}, err
	}
	action := "clockify_update_task_" + strings.ReplaceAll(endpoint, "-", "_")
	if dryrun.Enabled(args) {
		return ok(action, dryrun.Preview(action, body), map[string]any{"workspaceId": wsID, "projectId": projectID, "taskId": taskID}), nil
	}
	path, err := paths.Workspace(wsID, "projects", projectID, "tasks", taskID, endpoint)
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ToolResult{}, err
	}
	return ok(action, out, map[string]any{"workspaceId": wsID, "projectId": projectID, "taskId": taskID}), nil
}

func rateBodyFromArgs(args map[string]any) (map[string]any, error) {
	amount, ok := int64Arg(args, "amount")
	if !ok {
		return nil, fmt.Errorf("amount is required")
	}
	body := map[string]any{"amount": amount}
	setIfString(body, args, "since", "since")
	return body, nil
}

func (s *Service) ArchiveProjects(ctx context.Context, args map[string]any) (ToolResult, error) {
	rawIDs, rawOk := args["project_ids"].([]any)
	if !rawOk || len(rawIDs) == 0 {
		return ToolResult{}, fmt.Errorf("project_ids is required and must be a non-empty array")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	results := make([]map[string]any, 0, len(rawIDs))
	for _, raw := range rawIDs {
		pid, isStr := raw.(string)
		if !isStr {
			return ToolResult{}, fmt.Errorf("project_ids must contain only strings")
		}
		if err := resolve.ValidateID(pid, "project_id"); err != nil {
			return ToolResult{}, err
		}

		body := map[string]any{"archived": true}
		path, err := paths.Workspace(wsID, "projects", pid)
		if err != nil {
			results = append(results, map[string]any{
				"projectId": pid,
				"archived":  false,
				"error":     err.Error(),
			})
			continue
		}
		var out map[string]any
		if err := s.Client.Put(ctx, path, body, &out); err != nil {
			results = append(results, map[string]any{
				"projectId": pid,
				"archived":  false,
				"error":     err.Error(),
			})
			continue
		}
		s.emitResourceUpdateWithState(projectResourceURI(wsID, pid), out)
		results = append(results, map[string]any{
			"projectId": pid,
			"archived":  true,
		})
	}

	failed := 0
	for _, r := range results {
		if archived, _ := r["archived"].(bool); !archived {
			failed++
		}
	}
	return ok("clockify_archive_projects", results, map[string]any{
		"workspaceId":    wsID,
		"count":          len(results),
		"failedCount":    failed,
		"allSucceeded":   failed == 0,
		"partialFailure": failed > 0 && failed < len(results),
	}), nil
}
