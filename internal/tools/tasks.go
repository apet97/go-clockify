package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
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
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
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
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"projectId":   projectID,
		"count":       len(out),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	return ok("clockify_list_tasks", out, meta), nil
}

// GetTask fetches a single task by ID or exact name within a project.
// project (name or ID) and task (ID or name) are both required.
func (s *Service) GetTask(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	taskRef := strings.TrimSpace(stringArg(args, "task"))
	if taskRef == "" {
		return ResultEnvelope{}, fmt.Errorf("task is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	taskID, err := s.resolveTaskID(ctx, wsID, projectID, taskRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	taskPath, err := paths.Workspace(wsID, "projects", projectID, "tasks", taskID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out clockify.Task
	if err := s.Client.Get(ctx, taskPath, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_task", out, map[string]any{"workspaceId": wsID, "projectId": projectID, "taskId": taskID}), nil
}

func (s *Service) CreateTask(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectID == "" && projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required (or project_id)")
	}
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if projectID == "" {
		projectID, err = s.resolveProjectID(ctx, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
	}

	payload := map[string]any{"name": name}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	if estimate := strings.TrimSpace(stringArg(args, "estimate")); estimate != "" {
		payload["estimate"] = estimate
	}
	if status := strings.TrimSpace(stringArg(args, "status")); status != "" {
		payload["status"] = status
	}
	if dryrun.Enabled(args) {
		return ok("clockify_create_task", dryrun.Preview("clockify_create_task", payload), map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
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

// UpdateTask performs a fetch-then-merge update of a task.
// Clockify's PUT /projects/{pid}/tasks/{tid} is a full replacement;
// we GET the existing task, layer caller changes on top, then PUT the
// merged shape back. Empty-string caller args are treated as "no change".
func (s *Service) UpdateTask(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	taskRef := strings.TrimSpace(stringArg(args, "task"))
	if taskRef == "" {
		return ResultEnvelope{}, fmt.Errorf("task is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	taskID, err := s.resolveTaskID(ctx, wsID, projectID, taskRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	taskPath, err := paths.Workspace(wsID, "projects", projectID, "tasks", taskID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing clockify.Task
	if err := s.Client.Get(ctx, taskPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	changedFields := make([]string, 0, 4)
	if v := stringArg(args, "name"); v != "" && v != existing.Name {
		existing.Name = v
		changedFields = append(changedFields, "name")
	}
	if v := stringArg(args, "status"); v != "" && v != existing.Status {
		existing.Status = v
		changedFields = append(changedFields, "status")
	}
	if v := stringArg(args, "estimate"); v != "" && v != existing.Estimate {
		existing.Estimate = v
		changedFields = append(changedFields, "estimate")
	}
	if v, ok := args["billable"].(bool); ok && v != existing.Billable {
		existing.Billable = v
		changedFields = append(changedFields, "billable")
	}

	meta := map[string]any{
		"workspaceId":   wsID,
		"projectId":     projectID,
		"taskId":        taskID,
		"changedFields": changedFields,
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_update_task",
			Data:   dryrun.Preview("clockify_update_task", args),
			Meta:   meta,
		}, nil
	}

	payload := taskPutPayload(existing)
	var updated clockify.Task
	if err := s.Client.Put(ctx, taskPath, payload, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_task", updated, meta), nil
}

// taskPutPayload builds the full-replacement body for PUT /projects/{pid}/tasks/{tid}.
// Clockify requires name in the body and uses full-replacement semantics.
func taskPutPayload(t clockify.Task) map[string]any {
	p := map[string]any{
		"name":     t.Name,
		"billable": t.Billable,
	}
	if t.Status != "" {
		p["status"] = t.Status
	}
	if t.Estimate != "" {
		p["estimate"] = t.Estimate
	}
	if t.AssigneeID != "" {
		p["assigneeId"] = t.AssigneeID
	}
	if len(t.AssigneeIDs) > 0 {
		p["assigneeIds"] = t.AssigneeIDs
	}
	if len(t.UserGroupIDs) > 0 {
		p["userGroupIds"] = t.UserGroupIDs
	}
	return p
}

// DeleteTask deletes a task by project + task reference (ID or name).
// Clockify's DELETE /projects/{pid}/tasks/{tid} works directly on active tasks —
// no archive step is required (unlike clients). Supports dry_run.
func (s *Service) DeleteTask(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	taskRef := strings.TrimSpace(stringArg(args, "task"))
	if taskRef == "" {
		return ResultEnvelope{}, fmt.Errorf("task is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	taskID, err := s.resolveTaskID(ctx, wsID, projectID, taskRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	taskPath, err := paths.Workspace(wsID, "projects", projectID, "tasks", taskID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing clockify.Task
	if err := s.Client.Get(ctx, taskPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_task",
			Data:   dryrun.WrapResult(existing, "clockify_delete_task"),
			Meta:   map[string]any{"workspaceId": wsID, "projectId": projectID, "taskId": taskID},
		}, nil
	}

	if err := s.Client.Delete(ctx, taskPath); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_task", map[string]any{"deleted": true, "taskId": taskID}, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}
