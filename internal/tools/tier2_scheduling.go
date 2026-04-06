package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "scheduling",
		Description: "Resource scheduling and capacity planning",
		Keywords:    []string{"schedule", "assignment", "capacity", "resource"},
		Builder:     schedulingHandlers,
	})
}

func schedulingHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. clockify_list_assignments (RO)
		{
			Tool: toolRO("clockify_list_assignments",
				"List scheduling assignments with optional user_id and project_id filters",
				map[string]any{"type": "object", "properties": map[string]any{
					"user_id":    map[string]any{"type": "string", "description": "Filter by user ID or name/email"},
					"project_id": map[string]any{"type": "string", "description": "Filter by project ID or name"},
					"page":       map[string]any{"type": "integer", "description": "Page number (default 1)"},
					"page_size":  map[string]any{"type": "integer", "description": "Items per page (default 50)"},
				}}),
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.listAssignments(ctx, args)
			},
		},
		// 2. clockify_get_assignment (RO)
		{
			Tool: toolRO("clockify_get_assignment",
				"Get a scheduling assignment by ID",
				map[string]any{"type": "object", "required": []string{"assignment_id"}, "properties": map[string]any{
					"assignment_id": map[string]any{"type": "string"},
				}}),
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getAssignment(ctx, args)
			},
		},
		// 3. clockify_create_assignment (RW)
		{
			Tool: toolRW("clockify_create_assignment",
				"Create a scheduling assignment for a user on a project",
				map[string]any{"type": "object", "required": []string{"user_id", "project_id", "start", "end"}, "properties": map[string]any{
					"user_id":       map[string]any{"type": "string", "description": "User ID or name/email"},
					"project_id":    map[string]any{"type": "string", "description": "Project ID or name"},
					"start":         map[string]any{"type": "string", "description": "Start date (YYYY-MM-DD or RFC3339)"},
					"end":           map[string]any{"type": "string", "description": "End date (YYYY-MM-DD or RFC3339)"},
					"hours_per_day": map[string]any{"type": "number", "description": "Hours per day (default 8)"},
					"note":          map[string]any{"type": "string"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.createAssignment(ctx, args)
			},
		},
		// 4. clockify_update_assignment (RW)
		{
			Tool: toolRW("clockify_update_assignment",
				"Update a scheduling assignment by ID",
				map[string]any{"type": "object", "required": []string{"assignment_id"}, "properties": map[string]any{
					"assignment_id": map[string]any{"type": "string"},
					"start":         map[string]any{"type": "string", "description": "Start date (YYYY-MM-DD or RFC3339)"},
					"end":           map[string]any{"type": "string", "description": "End date (YYYY-MM-DD or RFC3339)"},
					"hours_per_day": map[string]any{"type": "number"},
					"note":          map[string]any{"type": "string"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateAssignment(ctx, args)
			},
		},
		// 5. clockify_delete_assignment (destructive)
		{
			Tool: toolDestructive("clockify_delete_assignment",
				"Delete a scheduling assignment by ID (supports dry_run preview)",
				map[string]any{"type": "object", "required": []string{"assignment_id"}, "properties": map[string]any{
					"assignment_id": map[string]any{"type": "string"},
					"dry_run":       map[string]any{"type": "boolean", "description": "Preview deletion without making changes"},
				}}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.deleteAssignment(ctx, args)
			},
		},
		// 6. clockify_list_schedules (RO)
		{
			Tool: toolRO("clockify_list_schedules",
				"List scheduling schedules for the workspace",
				map[string]any{"type": "object", "properties": map[string]any{
					"page":      map[string]any{"type": "integer"},
					"page_size": map[string]any{"type": "integer"},
				}}),
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.listSchedules(ctx, args)
			},
		},
		// 7. clockify_get_schedule (RO)
		{
			Tool: toolRO("clockify_get_schedule",
				"Get a schedule by ID",
				map[string]any{"type": "object", "required": []string{"schedule_id"}, "properties": map[string]any{
					"schedule_id": map[string]any{"type": "string"},
				}}),
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getSchedule(ctx, args)
			},
		},
		// 8. clockify_create_schedule (RW)
		{
			Tool: toolRW("clockify_create_schedule",
				"Create a new schedule in the workspace",
				map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
					"name":          map[string]any{"type": "string", "description": "Schedule name"},
					"start":         map[string]any{"type": "string", "description": "Start date (YYYY-MM-DD or RFC3339)"},
					"end":           map[string]any{"type": "string", "description": "End date (YYYY-MM-DD or RFC3339)"},
					"hours_per_day": map[string]any{"type": "number"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.createSchedule(ctx, args)
			},
		},
		// 9. clockify_get_project_schedule_totals (RO)
		{
			Tool: toolRO("clockify_get_project_schedule_totals",
				"Get project scheduling assignment totals",
				map[string]any{"type": "object", "properties": map[string]any{
					"project_id": map[string]any{"type": "string", "description": "Filter by project ID or name"},
				}}),
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getProjectScheduleTotals(ctx, args)
			},
		},
		// 10. clockify_filter_schedule_capacity (RO)
		{
			Tool: toolRO("clockify_filter_schedule_capacity",
				"Get scheduling capacity data for a date range",
				map[string]any{"type": "object", "required": []string{"start", "end"}, "properties": map[string]any{
					"start":   map[string]any{"type": "string", "description": "Start date (YYYY-MM-DD or RFC3339)"},
					"end":     map[string]any{"type": "string", "description": "End date (YYYY-MM-DD or RFC3339)"},
					"user_id": map[string]any{"type": "string", "description": "Filter by user ID"},
				}}),
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.filterScheduleCapacity(ctx, args)
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Scheduling handler implementations
// ---------------------------------------------------------------------------

func (s *Service) listAssignments(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	query := map[string]string{}
	if uid := stringArg(args, "user_id"); uid != "" {
		resolved, err := resolve.ResolveUserID(ctx, s.Client, wsID, uid)
		if err != nil {
			return ResultEnvelope{}, err
		}
		query["userId"] = resolved
	}
	if pid := stringArg(args, "project_id"); pid != "" {
		resolved, err := resolve.ResolveProjectID(ctx, s.Client, wsID, pid)
		if err != nil {
			return ResultEnvelope{}, err
		}
		query["projectId"] = resolved
	}

	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)
	query["page"] = fmt.Sprintf("%d", page)
	query["page-size"] = fmt.Sprintf("%d", pageSize)

	var assignments []map[string]any
	path := "/workspaces/" + wsID + "/scheduling/assignments"
	if err := s.Client.Get(ctx, path, query, &assignments); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_assignments", assignments, map[string]any{
		"workspaceId": wsID,
		"count":       len(assignments),
		"page":        page,
		"pageSize":    pageSize,
	}), nil
}

func (s *Service) getAssignment(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	aID := stringArg(args, "assignment_id")
	if err := resolve.ValidateID(aID, "assignment_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var assignment map[string]any
	path := "/workspaces/" + wsID + "/scheduling/assignments/" + aID
	if err := s.Client.Get(ctx, path, nil, &assignment); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_get_assignment", assignment, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) createAssignment(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	userRef := stringArg(args, "user_id")
	if userRef == "" {
		return ResultEnvelope{}, fmt.Errorf("user_id is required")
	}
	userID, err := resolve.ResolveUserID(ctx, s.Client, wsID, userRef)
	if err != nil {
		return ResultEnvelope{}, err
	}

	projectRef := stringArg(args, "project_id")
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project_id is required")
	}
	projectID, err := resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return ResultEnvelope{}, fmt.Errorf("start and end are required")
	}

	payload := map[string]any{
		"userId":    userID,
		"projectId": projectID,
		"start":     startRaw,
		"end":       endRaw,
	}

	if hpd, ok := args["hours_per_day"]; ok {
		payload["hoursPerDay"] = hpd
	}
	if note := stringArg(args, "note"); note != "" {
		payload["note"] = note
	}

	var result map[string]any
	path := "/workspaces/" + wsID + "/scheduling/assignments"
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_create_assignment", result, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"projectId":   projectID,
	}), nil
}

func (s *Service) updateAssignment(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	aID := stringArg(args, "assignment_id")
	if err := resolve.ValidateID(aID, "assignment_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Fetch existing assignment for merge
	var existing map[string]any
	path := "/workspaces/" + wsID + "/scheduling/assignments/" + aID
	if err := s.Client.Get(ctx, path, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	changed := make([]string, 0, 4)
	if v := stringArg(args, "start"); v != "" {
		existing["start"] = v
		changed = append(changed, "start")
	}
	if v := stringArg(args, "end"); v != "" {
		existing["end"] = v
		changed = append(changed, "end")
	}
	if hpd, ok := args["hours_per_day"]; ok {
		existing["hoursPerDay"] = hpd
		changed = append(changed, "hoursPerDay")
	}
	if v := stringArg(args, "note"); v != "" {
		existing["note"] = v
		changed = append(changed, "note")
	}

	var result map[string]any
	if err := s.Client.Put(ctx, path, existing, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_update_assignment", result, map[string]any{
		"workspaceId":   wsID,
		"changedFields": changed,
	}), nil
}

func (s *Service) deleteAssignment(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	aID := stringArg(args, "assignment_id")
	if err := resolve.ValidateID(aID, "assignment_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path := "/workspaces/" + wsID + "/scheduling/assignments/" + aID

	if dryrun.Enabled(args) {
		var assignment map[string]any
		if err := s.Client.Get(ctx, path, nil, &assignment); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_assignment",
			Data:   dryrun.WrapResult(assignment, "clockify_delete_assignment"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_delete_assignment", map[string]any{
		"deleted":      true,
		"assignmentId": aID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) listSchedules(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)
	query := map[string]string{
		"page":      fmt.Sprintf("%d", page),
		"page-size": fmt.Sprintf("%d", pageSize),
	}

	var schedules []map[string]any
	path := "/workspaces/" + wsID + "/scheduling"
	if err := s.Client.Get(ctx, path, query, &schedules); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_schedules", schedules, map[string]any{
		"workspaceId": wsID,
		"count":       len(schedules),
	}), nil
}

func (s *Service) getSchedule(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	sID := stringArg(args, "schedule_id")
	if err := resolve.ValidateID(sID, "schedule_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var schedule map[string]any
	path := "/workspaces/" + wsID + "/scheduling/" + sID
	if err := s.Client.Get(ctx, path, nil, &schedule); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_get_schedule", schedule, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) createSchedule(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := stringArg(args, "name")
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{
		"name": name,
	}
	if v := stringArg(args, "start"); v != "" {
		payload["start"] = v
	}
	if v := stringArg(args, "end"); v != "" {
		payload["end"] = v
	}
	if hpd, ok := args["hours_per_day"]; ok {
		payload["hoursPerDay"] = hpd
	}

	var result map[string]any
	path := "/workspaces/" + wsID + "/scheduling"
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_create_schedule", result, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) getProjectScheduleTotals(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	query := map[string]string{}
	if pid := stringArg(args, "project_id"); pid != "" {
		resolved, err := resolve.ResolveProjectID(ctx, s.Client, wsID, pid)
		if err != nil {
			return ResultEnvelope{}, err
		}
		query["projectId"] = resolved
	}

	var totals map[string]any
	path := "/workspaces/" + wsID + "/scheduling/assignments/project-totals"
	if err := s.Client.Get(ctx, path, query, &totals); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_get_project_schedule_totals", totals, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) filterScheduleCapacity(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return ResultEnvelope{}, fmt.Errorf("start and end are required")
	}

	query := map[string]string{
		"start": startRaw,
		"end":   endRaw,
	}
	if uid := stringArg(args, "user_id"); uid != "" {
		query["userId"] = uid
	}

	var capacity map[string]any
	path := "/workspaces/" + wsID + "/scheduling/capacity"
	if err := s.Client.Get(ctx, path, query, &capacity); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_filter_schedule_capacity", capacity, map[string]any{
		"workspaceId": wsID,
		"start":       startRaw,
		"end":         endRaw,
	}), nil
}
