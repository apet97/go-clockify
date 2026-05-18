package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
)

func taskListQuery(args map[string]any, page, pageSize int) map[string]string {
	query := pageQuery(page, pageSize)
	addStringQuery(query, args, "name", "name")
	addBoolQuery(query, args, "strict_name_search", "strict-name-search")
	addBoolQuery(query, args, "is_active", "is-active")
	addStringQuery(query, args, "sort_column", "sort-column")
	addStringQuery(query, args, "sort_order", "sort-order")
	return query
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
	view, financialMeta := s.enrichTaskView(ctx, wsID, projectID, out, args)
	return ok("clockify_tasks_get", view, withFinancialMeta(map[string]any{"workspaceId": wsID, "projectId": projectID, "taskId": taskID}, financialMeta)), nil
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
	if v := stringArg(args, "assignee_id"); v != "" && v != existing.AssigneeID {
		existing.AssigneeID = v
		changedFields = append(changedFields, "assignee_id")
	}
	if assigneeIDs, ok, err := strictStringSliceArg(args, "assignee_ids"); err != nil {
		return ResultEnvelope{}, err
	} else if ok {
		existing.AssigneeIDs = assigneeIDs
		changedFields = append(changedFields, "assignee_ids")
	}
	if budget, ok := int64Arg(args, "budget_estimate"); ok && budget != existing.BudgetEstimate {
		existing.BudgetEstimate = budget
		changedFields = append(changedFields, "budget_estimate")
	}
	if groupIDs, ok, err := strictStringSliceArg(args, "user_group_ids"); err != nil {
		return ResultEnvelope{}, err
	} else if ok {
		existing.UserGroupIDs = groupIDs
		changedFields = append(changedFields, "user_group_ids")
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
			Action: "clockify_tasks_update",
			Data:   dryrun.Preview("clockify_tasks_update", args),
			Meta:   meta,
		}, nil
	}

	payload := taskPutPayload(existing)
	if err := applyTaskRequestFields(payload, args); err != nil {
		return ResultEnvelope{}, err
	}
	var updated clockify.Task
	query := map[string]string{}
	addBoolQuery(query, args, "contains_assignee", "contains-assignee")
	addStringQuery(query, args, "membership_status", "membership-status")
	if err := s.Client.PutWithQuery(ctx, taskPath, query, payload, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	view, financialMeta := s.enrichTaskView(ctx, wsID, projectID, updated, args)
	return ok("clockify_tasks_update", view, withFinancialMeta(meta, financialMeta)), nil
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
	if t.BudgetEstimate > 0 {
		p["budgetEstimate"] = t.BudgetEstimate
	}
	return p
}

func applyTaskRequestFields(payload map[string]any, args map[string]any) error {
	setIfString(payload, args, "assignee_id", "assigneeId")
	if err := setIfStringSlice(payload, args, "assignee_ids", "assigneeIds"); err != nil {
		return err
	}
	setIfBool(payload, args, "billable", "billable")
	setIfInt64(payload, args, "budget_estimate", "budgetEstimate")
	setIfString(payload, args, "estimate", "estimate")
	setIfString(payload, args, "status", "status")
	if err := setIfStringSlice(payload, args, "user_group_ids", "userGroupIds"); err != nil {
		return err
	}
	return nil
}

// DeleteTask deletes a task by project + task reference (ID or name).
// Clockify requires tasks to be DONE before DELETE, so active tasks are first
// updated with the existing full-replacement shape plus status=DONE.
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
		steps := []string{"GET existing task"}
		if !strings.EqualFold(existing.Status, "DONE") {
			steps = append(steps, "PUT status=DONE")
		}
		steps = append(steps, "DELETE task")
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_tasks_delete",
			Data:   dryrun.WrapResult(existing, "clockify_tasks_delete"),
			Meta: map[string]any{
				"workspaceId":               wsID,
				"projectId":                 projectID,
				"taskId":                    taskID,
				"wouldMarkDoneBeforeDelete": !strings.EqualFold(existing.Status, "DONE"),
				"dryRunSteps":               steps,
			},
		}, nil
	}

	if !strings.EqualFold(existing.Status, "DONE") {
		existing.Status = "DONE"
		payload := taskPutPayload(existing)
		var updated clockify.Task
		if err := s.Client.Put(ctx, taskPath, payload, &updated); err != nil {
			return ResultEnvelope{}, err
		}
	}

	if err := s.Client.Delete(ctx, taskPath); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_tasks_delete", map[string]any{"deleted": true, "taskId": taskID}, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}
