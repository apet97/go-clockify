package tools

import (
	"context"
	"fmt"
	"strings"

	"goclmcp/internal/clockify"
	"goclmcp/internal/resolve"
)

func (s *Service) ListTasks(ctx context.Context, projectRef string) (ResultEnvelope, error) {
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
	var out []clockify.Task
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/projects/"+projectID+"/tasks", map[string]string{"page-size": "50"}, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_tasks", out, map[string]any{"workspaceId": wsID, "projectId": projectID, "count": len(out)}), nil
}

func (s *Service) CreateTask(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{"name": name}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}

	var task clockify.Task
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/projects/"+projectID+"/tasks", payload, &task); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_task", task, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}
