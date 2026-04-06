package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/resolve"
)

func (s *Service) ListProjects(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var projects []clockify.Project
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/projects", map[string]string{"page-size": "50"}, &projects); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_projects", projects, map[string]any{"workspaceId": wsID, "count": len(projects)}), nil
}

func (s *Service) GetProject(ctx context.Context, projectRef string) (ResultEnvelope, error) {
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out clockify.Project
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/projects/"+projectID, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_project", out, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}

func (s *Service) CreateProject(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{"name": name}

	clientRef := stringArg(args, "client")
	if clientRef != "" {
		clientID, err := resolve.ResolveClientID(ctx, s.Client, wsID, clientRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
		payload["clientId"] = clientID
	}
	if color := stringArg(args, "color"); color != "" {
		payload["color"] = color
	}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	if isPublic, ok := args["is_public"].(bool); ok {
		payload["isPublic"] = isPublic
	}

	var project clockify.Project
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/projects", payload, &project); err != nil {
		return ResultEnvelope{}, err
	}

	meta := map[string]any{"workspaceId": wsID}
	return ok("clockify_create_project", project, meta), nil
}
