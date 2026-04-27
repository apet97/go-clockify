package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func (s *Service) ListTasks(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectRef := strings.TrimSpace(stringArg(args, "project"))
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
	path, err := paths.Workspace(wsID, "projects", projectID, "tasks")
	if err != nil {
		return ResultEnvelope{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	var out []clockify.Task
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_tasks", out, map[string]any{
		"workspaceId": wsID,
		"projectId":   projectID,
		"count":       len(out),
		"page":        page,
		"pageSize":    pageSize,
	}), nil
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

	path, err := paths.Workspace(wsID, "projects", projectID, "tasks")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var task clockify.Task
	if err := s.Client.Post(ctx, path, payload, &task); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_task", task, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}
