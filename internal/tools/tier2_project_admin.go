package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "project_admin",
		Description: "Project templates, estimates, memberships, and archival",
		Keywords:    []string{"template", "estimate", "membership", "archive", "budget"},
		ToolNames: []string{
			"clockify_list_project_templates",
			"clockify_get_project_template",
			"clockify_create_project_template",
			"clockify_update_project_estimate",
			"clockify_set_project_memberships",
			"clockify_archive_projects",
		},
		Builder: projectAdminHandlers,
	})
}

func projectAdminHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List project templates
		{
			Tool: toolRO("clockify_list_project_templates",
				"List project templates in the workspace",
				map[string]any{"type": "object"}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, _ map[string]any) (any, error) {
				return s.ListProjectTemplates(ctx)
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
			Tool: toolRW("clockify_update_project_estimate",
				"Update a project's estimate (TIME or BUDGET)",
				map[string]any{
					"type":     "object",
					"required": []string{"project_id", "estimate_type", "estimate_value"},
					"properties": map[string]any{
						"project_id":     map[string]any{"type": "string"},
						"estimate_type":  map[string]any{"type": "string", "description": "One of: TIME, BUDGET"},
						"estimate_value": map[string]any{"type": "number", "description": "Estimate amount (seconds for TIME, currency for BUDGET)"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateProjectEstimate(ctx, args)
			},
		},
		// 5. Set project memberships
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
		// 6. Archive projects
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

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Service) ListProjectTemplates(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	query := map[string]string{"is-template": "true"}
	path, err := paths.Workspace(wsID, "projects")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out []map[string]any
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_project_templates", out, map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
	}), nil
}

func (s *Service) GetProjectTemplate(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_project_template", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) CreateProjectTemplate(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := stringArg(args, "name")
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
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
			return ResultEnvelope{}, err
		}
		body["clientId"] = clientID
	}

	path, err := paths.Workspace(wsID, "projects")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_project_template", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) UpdateProjectEstimate(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ResultEnvelope{}, err
	}
	estType := stringArg(args, "estimate_type")
	if estType == "" {
		return ResultEnvelope{}, fmt.Errorf("estimate_type is required")
	}

	estValue, estOk := args["estimate_value"].(float64)
	if !estOk {
		return ResultEnvelope{}, fmt.Errorf("estimate_value is required and must be a number")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"timeEstimate": map[string]any{
			"type":     estType,
			"estimate": estValue,
			"active":   true,
		},
	}

	path, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	s.emitResourceUpdateWithState(projectResourceURI(wsID, projectID), out)
	return ok("clockify_update_project_estimate", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) SetProjectMemberships(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectID := stringArg(args, "project_id")
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return ResultEnvelope{}, err
	}

	rawIDs, rawOk := args["user_ids"].([]any)
	if !rawOk || len(rawIDs) == 0 {
		return ResultEnvelope{}, fmt.Errorf("user_ids is required and must be a non-empty array")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	memberships := make([]map[string]any, 0, len(rawIDs))
	for _, raw := range rawIDs {
		uid, isStr := raw.(string)
		if !isStr {
			return ResultEnvelope{}, fmt.Errorf("user_ids must contain only strings")
		}
		if err := resolve.ValidateID(uid, "user_id"); err != nil {
			return ResultEnvelope{}, err
		}
		m := map[string]any{"userId": uid}
		if rate, rateOk := args["hourly_rate"].(float64); rateOk {
			m["hourlyRate"] = map[string]any{"amount": rate}
		}
		memberships = append(memberships, m)
	}

	body := map[string]any{"memberships": memberships}

	path, err := paths.Workspace(wsID, "projects", projectID, "memberships")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	s.emitResourceUpdateWithState(projectResourceURI(wsID, projectID), out)
	return ok("clockify_set_project_memberships", out, map[string]any{
		"workspaceId": wsID,
		"memberCount": len(memberships),
	}), nil
}

func (s *Service) ArchiveProjects(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	rawIDs, rawOk := args["project_ids"].([]any)
	if !rawOk || len(rawIDs) == 0 {
		return ResultEnvelope{}, fmt.Errorf("project_ids is required and must be a non-empty array")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	results := make([]map[string]any, 0, len(rawIDs))
	for _, raw := range rawIDs {
		pid, isStr := raw.(string)
		if !isStr {
			return ResultEnvelope{}, fmt.Errorf("project_ids must contain only strings")
		}
		if err := resolve.ValidateID(pid, "project_id"); err != nil {
			return ResultEnvelope{}, err
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

	return ok("clockify_archive_projects", results, map[string]any{
		"workspaceId": wsID,
		"count":       len(results),
	}), nil
}
