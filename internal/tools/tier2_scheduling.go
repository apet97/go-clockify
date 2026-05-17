package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func schedulingHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. clockify_assignment_report (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_assignment_report",
				"Generate a scheduled-vs-tracked assignment report with scheduling capacity, tracked hours, earned amount, cost, profit, and variance. group_by accepts USER, PROJECT, CLIENT, TASK, DATE, WEEK, MONTH and defaults to USER,PROJECT,TASK.",
				assignmentReportInputSchema()), envelopeSchemaFor[AssignmentReportData]("clockify_assignment_report")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.AssignmentReport(ctx, args)
			},
		},
		// 2. clockify_list_assignments (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_list_assignments",
				"List scheduling assignments within a date range",
				map[string]any{
					"type":     "object",
					"required": []string{"start", "end"},
					"properties": map[string]any{
						"start":      map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
						"end":        map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
						"user_id":    map[string]any{"type": "string", "description": "Filter by user ID or name/email"},
						"project_id": map[string]any{"type": "string", "description": "Filter by project ID or name"},
						"page":       map[string]any{"type": "integer", "description": "Page number (default 1)"},
						"page_size":  map[string]any{"type": "integer", "description": "Items per page (default 50)"},
						"timezone":   timezoneInputProperty(),
					},
				}), assignmentViewEnvelope("clockify_list_assignments", true)),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.listAssignments(ctx, args)
			},
		},
		// 3. clockify_get_assignment (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_get_assignment",
				"Get a scheduling assignment by ID by paging through the supported date-range list endpoint until found. If start/end are omitted, scans one year back through one year forward.",
				map[string]any{"type": "object", "required": []string{"assignment_id"}, "properties": map[string]any{
					"assignment_id": map[string]any{"type": "string"},
					"start":         map[string]any{"type": "string", "description": "Optional range start used to locate the assignment. " + flexibleDatetimeDescription + ". Defaults to now minus 1 year"},
					"end":           map[string]any{"type": "string", "description": "Optional range end used to locate the assignment. " + flexibleDatetimeDescription + ". Defaults to now plus 1 year"},
					"page_size":     map[string]any{"type": "integer", "description": "Upstream page size to scan per request (default 200); response meta includes pagesFetched and entriesScanned"},
					"timezone":      timezoneInputProperty(),
				}}), assignmentViewEnvelope("clockify_get_assignment", false)),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getAssignment(ctx, args)
			},
		},
		// 4. clockify_create_assignment (RW)
		{
			Tool: toolRW("clockify_create_assignment",
				"Create a recurring scheduling assignment for a user on a project. Retrying after a network timeout can create a duplicate assignment; check with clockify_list_assignments before retrying.",
				map[string]any{"type": "object", "required": []string{"user_id", "project_id", "start", "end", "hours_per_day"}, "properties": map[string]any{
					"user_id":                  map[string]any{"type": "string", "description": "User ID or name/email"},
					"project_id":               map[string]any{"type": "string", "description": "Project ID or name"},
					"start":                    map[string]any{"type": "string", "description": "Start date/time. " + flexibleDatetimeDescription},
					"end":                      map[string]any{"type": "string", "description": "End date/time. " + flexibleDatetimeDescription},
					"hours_per_day":            map[string]any{"type": "number", "minimum": 0.5, "maximum": 24, "description": "Work hours per day (0.5-24)."},
					"billable":                 map[string]any{"type": "boolean"},
					"include_non_working_days": map[string]any{"type": "boolean"},
					"start_time":               map[string]any{"type": "string", "description": "Optional hh:mm:ss start time"},
					"task_id":                  map[string]any{"type": "string"},
					"note":                     map[string]any{"type": "string"},
					"repeat":                   map[string]any{"type": "boolean", "description": "Whether to repeat the assignment"},
					"weeks":                    map[string]any{"type": "integer", "minimum": 1, "maximum": 99, "description": "Repeat interval in weeks when repeat is true. Default: 1."},
					"timezone":                 timezoneInputProperty(),
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.createAssignment(ctx, args)
			},
		},
		// 5. clockify_update_assignment (RW)
		{
			Tool: toolRW("clockify_update_assignment",
				"Update a recurring scheduling assignment by ID",
				map[string]any{"type": "object", "required": []string{"assignment_id", "start", "end"}, "properties": map[string]any{
					"assignment_id":            map[string]any{"type": "string"},
					"start":                    map[string]any{"type": "string", "description": "Start date/time. " + flexibleDatetimeDescription},
					"end":                      map[string]any{"type": "string", "description": "End date/time. " + flexibleDatetimeDescription},
					"hours_per_day":            map[string]any{"type": "number", "minimum": 0.5, "maximum": 24, "description": "Work hours per day (0.5-24)."},
					"billable":                 map[string]any{"type": "boolean"},
					"include_non_working_days": map[string]any{"type": "boolean"},
					"start_time":               map[string]any{"type": "string", "description": "Optional hh:mm:ss start time"},
					"task_id":                  map[string]any{"type": "string"},
					"note":                     map[string]any{"type": "string"},
					"series_update_option":     map[string]any{"type": "string", "enum": []string{"THIS_ONE", "THIS_AND_FOLLOWING", "ALL"}, "description": "Recurring series update scope"},
					"timezone":                 timezoneInputProperty(),
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateAssignment(ctx, args)
			},
		},
		// 6. clockify_delete_assignment (destructive)
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
		// 7. clockify_get_project_schedule_totals (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_get_project_schedule_totals",
				"Get scheduling totals per project across a date range, with tracked amount/cost/profit comparison when Reports API enrichment is available",
				map[string]any{
					"type":     "object",
					"required": []string{"start", "end"},
					"properties": map[string]any{
						"start":      map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
						"end":        map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
						"project_id": map[string]any{"type": "string", "description": "Filter by project ID or name"},
						"page_size":  map[string]any{"type": "integer", "description": "Items per page (default 50)"},
						"timezone":   timezoneInputProperty(),
					},
				}), envelopeOpenMapSlice("clockify_get_project_schedule_totals")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getProjectScheduleTotals(ctx, args)
			},
		},
		// 8. clockify_get_single_project_schedule_totals (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_get_single_project_schedule_totals",
				"Get scheduling totals for one project across a date range, enriched with tracked amount/cost/profit comparison when available",
				singleProjectScheduleTotalsInputSchema()), envelopeOpenMap("clockify_get_single_project_schedule_totals")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getSingleProjectScheduleTotals(ctx, args)
			},
		},
		// 9. clockify_get_workspace_schedule_user_totals (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_get_workspace_schedule_user_totals",
				"Get workspace scheduling totals per user across a date range",
				userScheduleTotalsInputSchema()), envelopeOpenMapSlice("clockify_get_workspace_schedule_user_totals")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getWorkspaceScheduleUserTotals(ctx, args)
			},
		},
		// 10. clockify_filter_schedule_capacity (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_filter_schedule_capacity",
				"Get a user's scheduling capacity totals for a date range, enriched with tracked amount/cost/profit comparison when available",
				map[string]any{
					"type":     "object",
					"required": []string{"user_id", "start", "end"},
					"properties": map[string]any{
						"user_id":   map[string]any{"type": "string", "description": "User ID, name, or email"},
						"start":     map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
						"end":       map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
						"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
						"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
						"timezone":  timezoneInputProperty(),
					},
				}), envelopeOpenMap("clockify_filter_schedule_capacity")),
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

	views, enrichment := s.enrichAssignmentViews(ctx, wsID, assignments, args)
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(assignments),
		"page":        page,
		"pageSize":    pageSize,
		"enrichment":  enrichment,
	}, args, page, pageSize)
	return ok("clockify_list_assignments", views, emptyListMeta(meta, "clockify_schedule_work")), nil
}

func (s *Service) listAssignmentsRaw(ctx context.Context, wsID string, args map[string]any) ([]map[string]any, int, int, error) {
	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return nil, 0, 0, err
	}

	query := map[string]string{
		"start": start,
		"end":   end,
	}
	if uid := stringArg(args, "user_id"); uid != "" {
		resolved, err := s.resolveUserID(ctx, wsID, uid)
		if err != nil {
			return nil, 0, 0, err
		}
		query["userId"] = resolved
	}
	if pid := stringArg(args, "project_id"); pid != "" {
		resolved, err := s.resolveProjectID(ctx, wsID, pid)
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
	scanRangeDefaulted := false
	if stringArg(scanArgs, "start") == "" || stringArg(scanArgs, "end") == "" {
		start, end := defaultAssignmentScanRange(time.Now().UTC())
		if stringArg(scanArgs, "start") == "" {
			scanArgs["start"] = start
		}
		if stringArg(scanArgs, "end") == "" {
			scanArgs["end"] = end
		}
		scanRangeDefaulted = true
	}
	if _, ok := scanArgs["page_size"]; !ok {
		scanArgs["page_size"] = 200
	}
	pageSize := intArg(scanArgs, "page_size", 200)
	if pageSize <= 0 {
		pageSize = 200
		scanArgs["page_size"] = pageSize
	}

	entriesScanned := 0
	pagesFetched := 0
	for page := 1; page <= aggregatePageSafetyStop; page++ {
		scanArgs["page"] = page
		assignments, _, _, err := s.listAssignmentsRaw(ctx, wsID, scanArgs)
		if err != nil {
			return ResultEnvelope{}, err
		}
		pagesFetched++
		entriesScanned += len(assignments)
		for _, assignment := range assignments {
			if id, _ := assignment["id"].(string); id == aID {
				views, enrichment := s.enrichAssignmentViews(ctx, wsID, []map[string]any{assignment}, scanArgs)
				data := any(assignment)
				if len(views) > 0 {
					data = views[0]
				}
				return ok("clockify_get_assignment", data, map[string]any{
					"workspaceId":        wsID,
					"pagesFetched":       pagesFetched,
					"entriesScanned":     entriesScanned,
					"assignmentId":       aID,
					"scanPageSize":       pageSize,
					"scanSafetyLimit":    aggregatePageSafetyStop,
					"scanStart":          stringArg(scanArgs, "start"),
					"scanEnd":            stringArg(scanArgs, "end"),
					"scanRangeDefaulted": scanRangeDefaulted,
					"enrichment":         enrichment,
				}), nil
			}
		}
		if len(assignments) < pageSize {
			return ResultEnvelope{}, fmt.Errorf("assignment %s not found in scan range after scanning %d entries across %d pages; pass start/end to narrow or shift the search window", aID, entriesScanned, pagesFetched)
		}
	}

	return ResultEnvelope{}, fmt.Errorf("assignment search pagination safety stop reached at %d pages; narrow the date range", aggregatePageSafetyStop)
}

func defaultAssignmentScanRange(now time.Time) (string, string) {
	now = now.UTC()
	return now.AddDate(-1, 0, 0).Format(time.RFC3339), now.AddDate(1, 0, 0).Format(time.RFC3339)
}

// schedulingRangeArgs parses the required start/end pair from args using
// loc as the default location for naked timestamps (e.g. "2026-04-01
// 09:00"). Pass a non-UTC location when the caller supplied a `timezone`
// argument or the service has a non-UTC DefaultTimezone. It delegates to
// parseRangeInLocation so scheduling shares one range parser: start/end
// are required, a bare end date counts as the end of that day, and an
// inverted range (end <= start) is rejected. The result is the RFC3339
// UTC pair the Clockify scheduling API expects.
func schedulingRangeArgs(args map[string]any, loc *time.Location) (string, string, error) {
	start, end, err := parseRangeInLocation(args, loc)
	if err != nil {
		return "", "", err
	}
	if !end.After(start) {
		return "", "", fmt.Errorf("end must be after start (got start=%s end=%s)", stringArg(args, "start"), stringArg(args, "end"))
	}
	return start.Format(time.RFC3339), end.Format(time.RFC3339), nil
}

// schedulingPublishSchema is the input schema for clockify_scheduling_publish.
func schedulingPublishSchema() map[string]any {
	return objectSchema(map[string]any{
		"required": []string{"start", "end"},
		"properties": map[string]any{
			"start":        map[string]any{"type": "string", "description": "Publish-window start. " + flexibleDatetimeDescription},
			"end":          map[string]any{"type": "string", "description": "Publish-window end; must be after start. " + flexibleDatetimeDescription},
			"notify_users": map[string]any{"type": "boolean", "description": "Whether Clockify emails affected users about the published schedule. Default false."},
			"dry_run":      map[string]any{"type": "boolean", "description": "Preview the publish request without applying it."},
		},
	})
}

// SchedulingPublish backs clockify_scheduling_publish. Scheduling assignment
// create/update produce drafts; this is the separate step that publishes
// every draft assignment in the start..end window. The Clockify endpoint
// returns no body, so the receipt is synthesised from the request (Axiom 8).
func (s *Service) SchedulingPublish(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return ResultEnvelope{}, err
	}
	notify := false
	if v, ok := args["notify_users"].(bool); ok {
		notify = v
	}
	payload := map[string]any{"start": start, "end": end, "notifyUsers": notify}

	if dryrun.Enabled(args) {
		return ok("clockify_scheduling_publish", dryrunPreviewPayload("clockify_scheduling_publish", payload), map[string]any{
			"workspaceId": wsID,
			"start":       start,
			"end":         end,
		}), nil
	}

	path, err := paths.Workspace(wsID, "scheduling", "assignments", "publish")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Put(ctx, path, payload, nil); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_scheduling_publish", map[string]any{
		"published":   true,
		"start":       start,
		"end":         end,
		"notifyUsers": notify,
	}, map[string]any{"workspaceId": wsID, "start": start, "end": end}), nil
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
	userID, err := s.resolveUserID(ctx, wsID, userRef)
	if err != nil {
		return ResultEnvelope{}, err
	}

	projectRef := stringArg(args, "project_id")
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project_id is required")
	}
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}

	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return ResultEnvelope{}, err
	}
	hoursPerDay, hasHoursPerDay := args["hours_per_day"]
	if !hasHoursPerDay {
		return ResultEnvelope{}, fmt.Errorf("hours_per_day is required")
	}

	payload := map[string]any{
		"userId":      userID,
		"projectId":   projectID,
		"start":       start,
		"end":         end,
		"hoursPerDay": hoursPerDay,
	}

	addSchedulingOptionalFields(payload, args)
	weeksApplied := addRecurringAssignment(payload, args)
	if note := stringArg(args, "note"); note != "" {
		payload["note"] = note
	}

	var result []map[string]any
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "recurring")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		var apiErr *clockify.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 500 {
			return ResultEnvelope{}, fmt.Errorf("scheduling_capacity_unavailable: scheduling assignment create returned upstream %d; verify the workspace subscription/scheduling capacity and try clockify_list_assignments as a preflight: %w", apiErr.StatusCode, err)
		}
		return ResultEnvelope{}, err
	}

	return ok("clockify_create_assignment", result, map[string]any{
		"workspaceId":  wsID,
		"userId":       userID,
		"projectId":    projectID,
		"weeksApplied": weeksApplied,
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

	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"start": start,
		"end":   end,
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

// addRecurringAssignment sets the recurringAssignment payload field and
// returns the weeks interval actually applied. A missing or sub-1 value
// defaults to 1; the schema's minimum:1 keeps 0 from reaching here, but
// the clamp stays as defence so the API never sees weeks:0.
func addRecurringAssignment(payload map[string]any, args map[string]any) int {
	weeks := intArg(args, "weeks", 0)
	if weeks < 1 {
		weeks = 1
	}
	payload["recurringAssignment"] = map[string]any{"weeks": weeks}
	return weeks
}

func (s *Service) getProjectScheduleTotals(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	totals, pageSize, err := s.getProjectScheduleTotalsRaw(ctx, wsID, args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	enriched, enrichment := s.enrichProjectScheduleTotals(ctx, wsID, totals, args)

	return ok("clockify_get_project_schedule_totals", enriched, map[string]any{
		"workspaceId": wsID,
		"count":       len(totals),
		"pageSize":    pageSize,
		"enrichment":  enrichment,
	}), nil
}

func (s *Service) getProjectScheduleTotalsRaw(ctx context.Context, wsID string, args map[string]any) ([]map[string]any, int, error) {
	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return nil, 0, err
	}
	pageSize := intArg(args, "page_size", 50)

	body := map[string]any{
		"start":    start,
		"end":      end,
		"pageSize": pageSize,
	}
	if pid := stringArg(args, "project_id"); pid != "" {
		resolved, err := s.resolveProjectID(ctx, wsID, pid)
		if err != nil {
			return nil, 0, err
		}
		body["projectId"] = resolved
	}

	var totals []map[string]any
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "projects", "totals")
	if err != nil {
		return nil, 0, err
	}
	if err := s.Client.Post(ctx, path, body, &totals); err != nil {
		return nil, 0, err
	}

	return totals, pageSize, nil
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
	userID, err := s.resolveUserID(ctx, wsID, userRef)
	if err != nil {
		return ResultEnvelope{}, err
	}

	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return ResultEnvelope{}, err
	}

	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)
	query := map[string]string{
		"start": start,
		"end":   end,
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
	enriched, enrichment := s.enrichScheduleCapacity(ctx, wsID, capacity, args)

	return ok("clockify_filter_schedule_capacity", enriched, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"start":       start,
		"end":         end,
		// capacityPerDay is reported in seconds upstream (probe-lab
		// fixture: 3600 = 1 hr/day, 25200 = 7 hr/day default).
		"capacityUnit": "seconds",
		"enrichment":   enrichment,
	}), nil
}
