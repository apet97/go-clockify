package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "scheduling",
		Description: "Resource scheduling and capacity planning",
		Keywords:    []string{"schedule", "assignment", "capacity", "resource"},
		ToolNames: []string{
			"clockify_list_assignments",
			"clockify_get_assignment",
			"clockify_create_assignment",
			"clockify_update_assignment",
			"clockify_delete_assignment",
			"clockify_get_project_schedule_totals",
			"clockify_filter_schedule_capacity",
		},
		Builder: schedulingHandlers,
	})
}

func schedulingHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. clockify_list_assignments (RO)
		{
			Tool: toolRO("clockify_list_assignments",
				"List scheduling assignments within a date range",
				map[string]any{
					"type":     "object",
					"required": []string{"start", "end"},
					"properties": map[string]any{
						"start":      map[string]any{"type": "string", "description": "Range start (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
						"end":        map[string]any{"type": "string", "description": "Range end (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
						"user_id":    map[string]any{"type": "string", "description": "Filter by user ID or name/email"},
						"project_id": map[string]any{"type": "string", "description": "Filter by project ID or name"},
						"page":       map[string]any{"type": "integer", "description": "Page number (default 1)"},
						"page_size":  map[string]any{"type": "integer", "description": "Items per page (default 50)"},
					},
				}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.listAssignments(ctx, args)
			},
		},
		// 2. clockify_get_assignment (RO)
		{
			Tool: toolRO("clockify_get_assignment",
				"Get a scheduling assignment by ID by scanning the supported date-range list endpoint",
				map[string]any{"type": "object", "required": []string{"assignment_id", "start", "end"}, "properties": map[string]any{
					"assignment_id": map[string]any{"type": "string"},
					"start":         map[string]any{"type": "string", "description": "Range start (RFC3339 yyyy-MM-ddThh:mm:ssZ) used to locate the assignment"},
					"end":           map[string]any{"type": "string", "description": "Range end (RFC3339 yyyy-MM-ddThh:mm:ssZ) used to locate the assignment"},
					"page_size":     map[string]any{"type": "integer", "description": "Items to scan (default 200)"},
				}}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getAssignment(ctx, args)
			},
		},
		// 3. clockify_create_assignment (RW)
		{
			Tool: toolRW("clockify_create_assignment",
				"Create a recurring scheduling assignment for a user on a project",
				map[string]any{"type": "object", "required": []string{"user_id", "project_id", "start", "end", "hours_per_day"}, "properties": map[string]any{
					"user_id":                  map[string]any{"type": "string", "description": "User ID or name/email"},
					"project_id":               map[string]any{"type": "string", "description": "Project ID or name"},
					"start":                    map[string]any{"type": "string", "description": "Start date/time (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
					"end":                      map[string]any{"type": "string", "description": "End date/time (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
					"hours_per_day":            map[string]any{"type": "number", "description": "Hours per day"},
					"billable":                 map[string]any{"type": "boolean"},
					"include_non_working_days": map[string]any{"type": "boolean"},
					"start_time":               map[string]any{"type": "string", "description": "Optional hh:mm:ss start time"},
					"task_id":                  map[string]any{"type": "string"},
					"note":                     map[string]any{"type": "string"},
					"repeat":                   map[string]any{"type": "boolean", "description": "Whether to repeat the assignment"},
					"weeks":                    map[string]any{"type": "integer", "description": "Repeat interval in weeks when repeat is true (1-99)"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.createAssignment(ctx, args)
			},
		},
		// 4. clockify_update_assignment (RW)
		{
			Tool: toolRW("clockify_update_assignment",
				"Update a recurring scheduling assignment by ID",
				map[string]any{"type": "object", "required": []string{"assignment_id", "start", "end"}, "properties": map[string]any{
					"assignment_id":            map[string]any{"type": "string"},
					"start":                    map[string]any{"type": "string", "description": "Start date/time (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
					"end":                      map[string]any{"type": "string", "description": "End date/time (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
					"hours_per_day":            map[string]any{"type": "number"},
					"billable":                 map[string]any{"type": "boolean"},
					"include_non_working_days": map[string]any{"type": "boolean"},
					"start_time":               map[string]any{"type": "string", "description": "Optional hh:mm:ss start time"},
					"task_id":                  map[string]any{"type": "string"},
					"note":                     map[string]any{"type": "string"},
					"series_update_option":     map[string]any{"type": "string", "enum": []string{"THIS_ONE", "THIS_AND_FOLLOWING", "ALL"}, "description": "Recurring series update scope"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateAssignment(ctx, args)
			},
		},
		// 5. clockify_delete_assignment (destructive)
		{
			Tool: toolDestructive("clockify_delete_assignment",
				"Delete a recurring scheduling assignment by ID (supports dry_run preview)",
				map[string]any{"type": "object", "required": []string{"assignment_id"}, "properties": map[string]any{
					"assignment_id":        map[string]any{"type": "string"},
					"series_update_option": map[string]any{"type": "string", "enum": []string{"THIS_ONE", "THIS_AND_FOLLOWING", "ALL"}, "description": "Recurring series delete scope"},
					"dry_run":              map[string]any{"type": "boolean", "description": "Preview deletion without making changes"},
				}}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.deleteAssignment(ctx, args)
			},
		},
		// 6. clockify_get_project_schedule_totals (RO)
		{
			Tool: toolRO("clockify_get_project_schedule_totals",
				"Get scheduling totals per project across a date range",
				map[string]any{
					"type":     "object",
					"required": []string{"start", "end"},
					"properties": map[string]any{
						"start":      map[string]any{"type": "string", "description": "Range start (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
						"end":        map[string]any{"type": "string", "description": "Range end (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
						"project_id": map[string]any{"type": "string", "description": "Filter by project ID or name"},
						"page_size":  map[string]any{"type": "integer", "description": "Items per page (default 50)"},
					},
				}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getProjectScheduleTotals(ctx, args)
			},
		},
		// 7. clockify_filter_schedule_capacity (RO)
		{
			Tool: toolRO("clockify_filter_schedule_capacity",
				"Get a user's scheduling capacity totals for a date range",
				map[string]any{
					"type":     "object",
					"required": []string{"user_id", "start", "end"},
					"properties": map[string]any{
						"user_id":   map[string]any{"type": "string", "description": "User ID, name, or email"},
						"start":     map[string]any{"type": "string", "description": "Range start (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
						"end":       map[string]any{"type": "string", "description": "Range end (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
						"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
						"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
					},
				}),
			ReadOnlyHint: true, IdempotentHint: true,
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

	assignments, page, pageSize, err := s.listAssignmentsRaw(ctx, wsID, args)
	if err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_assignments", assignments, map[string]any{
		"workspaceId": wsID,
		"count":       len(assignments),
		"page":        page,
		"pageSize":    pageSize,
	}), nil
}

func (s *Service) listAssignmentsRaw(ctx context.Context, wsID string, args map[string]any) ([]map[string]any, int, int, error) {
	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return nil, 0, 0, fmt.Errorf("start and end are required")
	}

	query := map[string]string{
		"start": startRaw,
		"end":   endRaw,
	}
	if uid := stringArg(args, "user_id"); uid != "" {
		resolved, err := resolve.ResolveUserID(ctx, s.Client, wsID, uid)
		if err != nil {
			return nil, 0, 0, err
		}
		query["userId"] = resolved
	}
	if pid := stringArg(args, "project_id"); pid != "" {
		resolved, err := resolve.ResolveProjectID(ctx, s.Client, wsID, pid)
		if err != nil {
			return nil, 0, 0, err
		}
		query["projectId"] = resolved
	}

	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)
	query["page"] = fmt.Sprintf("%d", page)
	// /scheduling/assignments/all uses hyphenated page-size per
	// SCHEDULINGDOC.md; the camelCase variant is silently ignored.
	query["page-size"] = fmt.Sprintf("%d", pageSize)

	var assignments []map[string]any
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "all")
	if err != nil {
		return nil, 0, 0, err
	}
	if err := s.Client.Get(ctx, path, query, &assignments); err != nil {
		return nil, 0, 0, err
	}

	return assignments, page, pageSize, nil
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

	scanArgs := make(map[string]any, len(args)+1)
	for k, v := range args {
		scanArgs[k] = v
	}
	if _, ok := scanArgs["page_size"]; !ok {
		scanArgs["page_size"] = 200
	}
	assignments, _, _, err := s.listAssignmentsRaw(ctx, wsID, scanArgs)
	if err != nil {
		return ResultEnvelope{}, err
	}
	for _, assignment := range assignments {
		if id, _ := assignment["id"].(string); id == aID {
			return ok("clockify_get_assignment", assignment, map[string]any{"workspaceId": wsID}), nil
		}
	}

	return ResultEnvelope{}, fmt.Errorf("assignment %s not found in supplied start/end range", aID)
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
	hoursPerDay, hasHoursPerDay := args["hours_per_day"]
	if !hasHoursPerDay {
		return ResultEnvelope{}, fmt.Errorf("hours_per_day is required")
	}

	payload := map[string]any{
		"userId":      userID,
		"projectId":   projectID,
		"start":       startRaw,
		"end":         endRaw,
		"hoursPerDay": hoursPerDay,
	}

	addSchedulingOptionalFields(payload, args)
	addRecurringAssignment(payload, args)
	if note := stringArg(args, "note"); note != "" {
		payload["note"] = note
	}

	var result []map[string]any
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "recurring")
	if err != nil {
		return ResultEnvelope{}, err
	}
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

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return ResultEnvelope{}, fmt.Errorf("start and end are required")
	}

	body := map[string]any{
		"start": startRaw,
		"end":   endRaw,
	}
	changed := []string{"start", "end"}
	if hpd, ok := args["hours_per_day"]; ok {
		body["hoursPerDay"] = hpd
		changed = append(changed, "hoursPerDay")
	}
	addSchedulingOptionalFields(body, args)
	if v := stringArg(args, "note"); v != "" {
		body["note"] = v
		changed = append(changed, "note")
	}
	if v := stringArg(args, "series_update_option"); v != "" {
		body["seriesUpdateOption"] = v
		changed = append(changed, "seriesUpdateOption")
	}

	path, err := paths.Workspace(wsID, "scheduling", "assignments", "recurring", aID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result []map[string]any
	if err := s.Client.Patch(ctx, path, body, &result); err != nil {
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

	path, err := paths.Workspace(wsID, "scheduling", "assignments", "recurring", aID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_assignment",
			Data: dryrun.MinimalResult("clockify_delete_assignment", map[string]any{
				"assignment_id":        aID,
				"series_update_option": stringArg(args, "series_update_option"),
			}),
			Meta: map[string]any{"workspaceId": wsID},
		}, nil
	}

	query := map[string]string{}
	if v := stringArg(args, "series_update_option"); v != "" {
		query["seriesUpdateOption"] = v
	}
	if err := s.Client.DeleteWithQuery(ctx, path, query); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_delete_assignment", map[string]any{
		"deleted":      true,
		"assignmentId": aID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func addSchedulingOptionalFields(payload map[string]any, args map[string]any) {
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	if include, ok := args["include_non_working_days"].(bool); ok {
		payload["includeNonWorkingDays"] = include
	}
	if startTime := stringArg(args, "start_time"); startTime != "" {
		payload["startTime"] = startTime
	}
	if taskID := stringArg(args, "task_id"); taskID != "" {
		payload["taskId"] = taskID
	}
}

func addRecurringAssignment(payload map[string]any, args map[string]any) {
	_, hasRepeat := args["repeat"]
	weeks := intArg(args, "weeks", 0)
	if !hasRepeat && weeks == 0 {
		return
	}
	recurring := map[string]any{}
	if repeat, ok := args["repeat"].(bool); ok {
		recurring["repeat"] = repeat
	}
	if weeks > 0 {
		recurring["weeks"] = weeks
	}
	payload["recurringAssignment"] = recurring
}

func (s *Service) getProjectScheduleTotals(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return ResultEnvelope{}, fmt.Errorf("start and end are required")
	}
	pageSize := intArg(args, "page_size", 50)

	body := map[string]any{
		"start":    startRaw,
		"end":      endRaw,
		"pageSize": pageSize,
	}
	if pid := stringArg(args, "project_id"); pid != "" {
		resolved, err := resolve.ResolveProjectID(ctx, s.Client, wsID, pid)
		if err != nil {
			return ResultEnvelope{}, err
		}
		body["projectId"] = resolved
	}

	var totals []map[string]any
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "projects", "totals")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Post(ctx, path, body, &totals); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_get_project_schedule_totals", totals, map[string]any{
		"workspaceId": wsID,
		"count":       len(totals),
		"pageSize":    pageSize,
	}), nil
}

func (s *Service) filterScheduleCapacity(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
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

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return ResultEnvelope{}, fmt.Errorf("start and end are required")
	}

	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)
	query := map[string]string{
		"start": startRaw,
		"end":   endRaw,
		"page":  fmt.Sprintf("%d", page),
		// The per-user totals endpoint uses camelCase pageSize; the
		// assignments/all endpoint above uses hyphenated page-size.
		"pageSize": fmt.Sprintf("%d", pageSize),
	}

	var capacity map[string]any
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "users", userID, "totals")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Get(ctx, path, query, &capacity); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_filter_schedule_capacity", capacity, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"start":       startRaw,
		"end":         endRaw,
		// capacityPerDay is reported in seconds upstream (probe-lab
		// fixture: 3600 = 1 hr/day, 25200 = 7 hr/day default).
		"capacityUnit": "seconds",
	}), nil
}
