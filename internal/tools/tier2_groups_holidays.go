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

func groupsHolidaysHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List user groups (admin)
		{
			Tool: toolRO("clockify_list_user_groups_admin",
				"List user groups in the workspace (admin view) with pagination",
				userGroupListSchema()),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListUserGroupsAdmin(ctx, args)
			},
		},
		// 2. Get user group by ID
		{
			Tool: toolRO("clockify_get_user_group",
				"Get a user group by ID",
				requiredSchema("group_id")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.GetUserGroup(ctx, args)
			},
		},
		// 3. Create user group (admin)
		{
			Tool: toolRW("clockify_create_user_group_admin",
				"Create a new user group with optional member user IDs",
				map[string]any{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]any{
						"name":     map[string]any{"type": "string", "description": "Group name"},
						"user_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "User IDs to add as members"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateUserGroupAdmin(ctx, args)
			},
		},
		// 4. Update user group (admin)
		{
			Tool: toolRW("clockify_update_user_group_admin",
				"Update an existing user group by ID",
				map[string]any{
					"type":     "object",
					"required": []string{"group_id"},
					"properties": map[string]any{
						"group_id": map[string]any{"type": "string"},
						"name":     map[string]any{"type": "string"},
						"user_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Replace member list with these user IDs"},
					},
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateUserGroupAdmin(ctx, args)
			},
		},
		// 5. Delete user group (destructive, dry-run minimal)
		{
			Tool: toolDestructive("clockify_delete_user_group_admin",
				"Delete a user group by ID (supports dry_run preview)",
				map[string]any{
					"type":     "object",
					"required": []string{"group_id"},
					"properties": map[string]any{
						"group_id": map[string]any{"type": "string"},
						"dry_run":  map[string]any{"type": "boolean", "description": "Preview without deleting"},
					},
				}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeleteUserGroupAdmin(ctx, args)
			},
		},
		// 6. List holidays
		{
			Tool: toolRO("clockify_list_holidays",
				"List holidays configured in the workspace",
				map[string]any{"type": "object"}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, _ map[string]any) (any, error) {
				return s.ListHolidays(ctx)
			},
		},
		// 7. List holidays in period
		{
			Tool: toolRO("clockify_list_holidays_in_period",
				"List holidays assigned to a user in a date period",
				map[string]any{
					"type": "object",
					// assigned_to is not in required: user_id is an accepted
					// alias, and "assigned_to OR user_id" cannot be expressed
					// in a plain-object schema. ListHolidaysInPeriod enforces
					// that exactly one is present before any API call.
					"required": []string{"start", "end"},
					"properties": map[string]any{
						"assigned_to": map[string]any{"type": "string", "description": "User ID the holidays are assigned to (or pass user_id)"},
						"user_id":     map[string]any{"type": "string", "description": "Alias for assigned_to"},
						"start":       map[string]any{"type": "string", "description": "Range start timestamp/date"},
						"end":         map[string]any{"type": "string", "description": "Range end timestamp/date"},
					},
				}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListHolidaysInPeriod(ctx, args)
			},
		},
		// 8. Create holiday
		{
			Tool: toolRW("clockify_create_holiday",
				"Create a new holiday in the workspace. Requires name + start_date and at least one user_ids or user_group_ids entry; the upstream rejects holidays with no assignment.",
				map[string]any{
					"type":     "object",
					"required": []string{"name", "start_date"},
					"properties": map[string]any{
						"name":            map[string]any{"type": "string", "description": "Holiday name (2–100 chars)"},
						"start_date":      map[string]any{"type": "string", "description": "Range start in yyyy-MM-dd format"},
						"end_date":        map[string]any{"type": "string", "description": "Range end in yyyy-MM-dd format; defaults to start_date for single-day holidays"},
						"occurs_annually": map[string]any{"type": "boolean", "description": "Whether the holiday recurs annually"},
						"user_ids":        map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}, "description": "User IDs the holiday applies to (at least one user_ids or user_group_ids entry is required)"},
						"user_group_ids":  map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}, "description": "User-group IDs the holiday applies to"},
					},
					// The "at least one of user_ids/user_group_ids" rule is
					// enforced in CreateHoliday. It is intentionally not a
					// top-level anyOf: Anthropic tool input schemas must be a
					// plain object, and a sibling anyOf breaks subagent launches.
				}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateHoliday(ctx, args)
			},
		},
		// 9. Delete holiday (destructive, dry-run minimal)
		{
			Tool: toolDestructive("clockify_delete_holiday",
				"Delete a holiday by ID (supports dry_run preview)",
				map[string]any{
					"type":     "object",
					"required": []string{"holiday_id"},
					"properties": map[string]any{
						"holiday_id": map[string]any{"type": "string"},
						"dry_run":    map[string]any{"type": "boolean", "description": "Preview without deleting"},
					},
				}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeleteHoliday(ctx, args)
			},
		},
	}
}

// ---------------------------------------------------------------------------
// User Group Handlers
// ---------------------------------------------------------------------------

func userGroupListSchema() map[string]any {
	return paginationSchema(map[string]any{
		"properties": map[string]any{
			"project_id":            map[string]any{"type": "string", "description": "Filter groups assigned to this project ID"},
			"name":                  map[string]any{"type": "string", "description": "Filter groups by name"},
			"sort_column":           map[string]any{"type": "string", "enum": []string{"ID", "NAME"}},
			"sort_order":            map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
			"include_team_managers": map[string]any{"type": "boolean", "description": "Include team-manager data in the returned groups"},
		},
	})
}

func userGroupListQuery(args map[string]any) (map[string]string, int, int) {
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	if v := strings.TrimSpace(stringArg(args, "project_id")); v != "" {
		query["project-id"] = v
	}
	if v := strings.TrimSpace(stringArg(args, "name")); v != "" {
		query["name"] = v
	}
	if v := strings.ToUpper(strings.TrimSpace(stringArg(args, "sort_column"))); v != "" {
		query["sort-column"] = v
	}
	if v := strings.ToUpper(strings.TrimSpace(stringArg(args, "sort_order"))); v != "" {
		query["sort-order"] = v
	}
	if v, ok := args["include_team_managers"].(bool); ok {
		query["includeTeamManagers"] = strconv.FormatBool(v)
	}
	return query, page, pageSize
}

func (s *Service) ListUserGroupsAdmin(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	query, page, pageSize := userGroupListQuery(args)

	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out []map[string]any
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ResultEnvelope{}, err
	}
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
	}, args, page, pageSize)
	return ok("clockify_list_user_groups_admin", out, meta), nil
}

func (s *Service) GetUserGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var groups []map[string]any
	if err := s.Client.Get(ctx, path, nil, &groups); err != nil {
		return ResultEnvelope{}, err
	}
	for _, out := range groups {
		if id, _ := out["id"].(string); id == groupID {
			return ok("clockify_get_user_group", out, map[string]any{"workspaceId": wsID}), nil
		}
	}
	return ResultEnvelope{}, fmt.Errorf("user group %s not found", groupID)
}

func (s *Service) CreateUserGroupAdmin(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	requestedUserIDs := createGroupUserIDs(args)

	body := map[string]any{"name": name}
	if len(requestedUserIDs) > 0 {
		body["userIds"] = requestedUserIDs
	}

	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}

	// Clockify's group-create endpoint does not reliably attach members, so
	// add each requested user explicitly. Members that were already attached
	// by the create call simply succeed again or are skipped by Clockify.
	groupID := firstReportString(out, "id", "_id")
	addedUserIDs := []string{}
	memberFailures := []map[string]any{}
	if groupID != "" {
		for _, uid := range requestedUserIDs {
			memberPath, perr := paths.Workspace(wsID, "user-groups", groupID, "users")
			if perr != nil {
				memberFailures = append(memberFailures, map[string]any{"userId": uid, "error": perr.Error()})
				continue
			}
			var memberOut map[string]any
			if aerr := s.Client.Post(ctx, memberPath, map[string]any{"userId": uid}, &memberOut); aerr != nil {
				memberFailures = append(memberFailures, map[string]any{"userId": uid, "error": aerr.Error()})
				continue
			}
			addedUserIDs = append(addedUserIDs, uid)
		}
	}

	// Clockify's group-create response carries userIds:[] because the create
	// endpoint does not echo members. Surface the members that were actually
	// attached so the receipt reflects reality.
	if out == nil {
		out = map[string]any{}
	}
	out["userIds"] = addedUserIDs
	meta := map[string]any{
		"workspaceId":  wsID,
		"addedUserIds": addedUserIDs,
	}
	if len(memberFailures) > 0 {
		meta["memberAddFailures"] = memberFailures
	}
	return ok("clockify_create_user_group_admin", out, meta), nil
}

// createGroupUserIDs reads the user_ids argument tolerantly: the MCP layer may
// decode a JSON array as []any or []string depending on the call path.
func createGroupUserIDs(args map[string]any) []string {
	out := []string{}
	switch v := args["user_ids"].(type) {
	case []string:
		for _, s := range v {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if t := strings.TrimSpace(s); t != "" {
					out = append(out, t)
				}
			}
		}
	}
	return out
}

func (s *Service) UpdateUserGroupAdmin(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if name := stringArg(args, "name"); name != "" {
		body["name"] = name
	}
	if userIDs, ok := args["user_ids"].([]any); ok {
		body["userIds"] = userIDs
	}

	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_user_group_admin", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) DeleteUserGroupAdmin(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_user_group_admin",
			Data:   dryrun.MinimalResult("clockify_delete_user_group_admin", map[string]any{"group_id": groupID}),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_user_group_admin", map[string]any{
		"deleted": true,
		"groupId": groupID,
	}, map[string]any{"workspaceId": wsID}), nil
}

// ---------------------------------------------------------------------------
// Holiday Handlers
// ---------------------------------------------------------------------------

func (s *Service) ListHolidays(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "holidays")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out []map[string]any
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_holidays", out, emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
		"total":       len(out),
		"page":        1,
		"pageSize":    len(out),
		"has_more":    false,
	}, "clockify_holidays_create")), nil
}

func (s *Service) GetHoliday(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	holidayID, err := requiredIDArg(args, "holiday_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	out, scanned, err := s.findHolidayByID(ctx, wsID, holidayID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_holidays_get", out, map[string]any{
		"workspaceId": wsID,
		"holidayId":   holidayID,
		"scanned":     scanned,
	}), nil
}

func (s *Service) ListHolidaysInPeriod(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	assignedTo := firstNonEmptyString(strings.TrimSpace(stringArg(args, "assigned_to")), strings.TrimSpace(stringArg(args, "user_id")))
	if assignedTo == "" {
		return ResultEnvelope{}, fmt.Errorf("assigned_to is required")
	}
	start := normalizeHolidayPeriodStart(strings.TrimSpace(stringArg(args, "start")))
	end := normalizeHolidayPeriodEnd(strings.TrimSpace(stringArg(args, "end")))
	if start == "" || end == "" {
		return ResultEnvelope{}, fmt.Errorf("start and end are required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "holidays", "in-period")
	if err != nil {
		return ResultEnvelope{}, err
	}
	query := map[string]string{
		"assigned-to": assignedTo,
		"start":       start,
		"end":         end,
	}
	var out []map[string]any
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_holidays_in_period", out, emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"assignedTo":  assignedTo,
		"count":       len(out),
		"warning":     "Clockify may return a server error for valid-looking but unknown assigned_to users; pass a live user ID from this workspace.",
	}, "")), nil
}

func (s *Service) CreateHoliday(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	startDate := strings.TrimSpace(stringArg(args, "start_date"))
	if startDate == "" {
		return ResultEnvelope{}, fmt.Errorf("start_date is required")
	}
	endDate := strings.TrimSpace(stringArg(args, "end_date"))
	if endDate == "" {
		// Single-day holidays use the same value for both bounds.
		endDate = startDate
	}

	userIDs := stringSliceArg(args, "user_ids")
	groupIDs := stringSliceArg(args, "user_group_ids")
	if len(userIDs) == 0 && len(groupIDs) == 0 {
		return ResultEnvelope{}, fmt.Errorf("at least one of user_ids or user_group_ids is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{
		"name": name,
		"datePeriod": map[string]any{
			"startDate": startDate,
			"endDate":   endDate,
		},
	}
	if occurs, ok := args["occurs_annually"].(bool); ok {
		body["occursAnnually"] = occurs
	}
	if len(userIDs) > 0 {
		body["users"] = map[string]any{
			"contains": "CONTAINS",
			"ids":      userIDs,
			"status":   "ACTIVE",
		}
	}
	if len(groupIDs) > 0 {
		body["userGroups"] = map[string]any{
			"contains": "CONTAINS",
			"ids":      groupIDs,
			"status":   "ALL",
		}
	}

	path, err := paths.Workspace(wsID, "holidays")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Post(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	out["date_period"] = holidayDatePeriodSnake(out, startDate, endDate)
	return ok("clockify_create_holiday", out, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) UpdateHoliday(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	holidayID, err := requiredIDArg(args, "holiday_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	existing, _, err := s.findHolidayByID(ctx, wsID, holidayID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	body := holidayUpdateBodyFromExisting(existing)
	name := strings.TrimSpace(stringArg(args, "name"))
	if name != "" {
		body["name"] = name
	}
	startDate := strings.TrimSpace(stringArg(args, "start_date"))
	endDate := strings.TrimSpace(stringArg(args, "end_date"))
	if startDate != "" || endDate != "" {
		currentPeriod, _ := body["datePeriod"].(map[string]any)
		if startDate == "" {
			startDate = firstReportString(currentPeriod, "startDate", "start_date")
		}
		if endDate == "" {
			endDate = firstReportString(currentPeriod, "endDate", "end_date")
		}
		body["datePeriod"] = map[string]any{
			"startDate": startDate,
			"endDate":   firstNonEmptyString(endDate, startDate),
		}
	}
	if occurs, ok := optionalBoolArg(args, "occurs_annually"); ok {
		body["occursAnnually"] = occurs
	}
	if userIDs, found, err := strictStringSliceArg(args, "user_ids"); err != nil {
		return ResultEnvelope{}, err
	} else if found && len(userIDs) > 0 {
		body["users"] = map[string]any{
			"contains": "CONTAINS",
			"ids":      userIDs,
			"status":   "ACTIVE",
		}
	}
	if groupIDs, found, err := strictStringSliceArg(args, "user_group_ids"); err != nil {
		return ResultEnvelope{}, err
	} else if found && len(groupIDs) > 0 {
		body["userGroups"] = map[string]any{
			"contains": "CONTAINS",
			"ids":      groupIDs,
			"status":   "ALL",
		}
	}

	path, err := paths.Workspace(wsID, "holidays", holidayID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Put(ctx, path, body, &out); err != nil {
		return ResultEnvelope{}, err
	}
	fallbackPeriod, _ := body["datePeriod"].(map[string]any)
	out["date_period"] = holidayDatePeriodSnake(out, firstReportString(fallbackPeriod, "startDate"), firstReportString(fallbackPeriod, "endDate"))
	return ok("clockify_holidays_update", out, map[string]any{
		"workspaceId": wsID,
		"holidayId":   holidayID,
	}), nil
}

func (s *Service) findHolidayByID(ctx context.Context, wsID, holidayID string) (map[string]any, int, error) {
	path, err := paths.Workspace(wsID, "holidays")
	if err != nil {
		return nil, 0, err
	}
	var rows []map[string]any
	if err := s.Client.Get(ctx, path, nil, &rows); err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		if firstReportString(row, "id", "_id", "holidayId", "holiday_id") == holidayID {
			return row, len(rows), nil
		}
	}
	return nil, len(rows), fmt.Errorf("holiday %s not found in workspace holiday list; call clockify_holidays_list and retry with a returned holiday_id", holidayID)
}

func holidayUpdateBodyFromExisting(existing map[string]any) map[string]any {
	body := map[string]any{}
	if name := firstReportString(existing, "name"); name != "" {
		body["name"] = name
	}
	if period, ok := firstPresent(existing, "datePeriod", "date_period").(map[string]any); ok {
		body["datePeriod"] = map[string]any{
			"startDate": firstReportString(period, "startDate", "start_date"),
			"endDate":   firstReportString(period, "endDate", "end_date"),
		}
	} else {
		start := firstReportString(existing, "startDate", "start_date", "start")
		end := firstReportString(existing, "endDate", "end_date", "end")
		if start != "" || end != "" {
			body["datePeriod"] = map[string]any{"startDate": start, "endDate": firstNonEmptyString(end, start)}
		}
	}
	if occurs, ok := optionalBoolArg(existing, "occursAnnually"); ok {
		body["occursAnnually"] = occurs
	} else if occurs, ok := optionalBoolArg(existing, "occurs_annually"); ok {
		body["occursAnnually"] = occurs
	}
	if users, ok := firstPresent(existing, "users").(map[string]any); ok {
		body["users"] = users
	} else if ids := anyStringSlice(firstPresent(existing, "userIds", "user_ids")); len(ids) > 0 {
		body["users"] = map[string]any{"contains": "CONTAINS", "ids": ids, "status": "ACTIVE"}
	}
	if groups, ok := firstPresent(existing, "userGroups", "user_groups").(map[string]any); ok {
		body["userGroups"] = groups
	} else if ids := anyStringSlice(firstPresent(existing, "userGroupIds", "user_group_ids")); len(ids) > 0 {
		body["userGroups"] = map[string]any{"contains": "CONTAINS", "ids": ids, "status": "ALL"}
	}
	return body
}

func normalizeHolidayPeriodStart(raw string) string {
	if raw == "" || strings.Contains(raw, "T") {
		return raw
	}
	return raw + "T00:00:00Z"
}

func normalizeHolidayPeriodEnd(raw string) string {
	if raw == "" || strings.Contains(raw, "T") {
		return raw
	}
	return raw + "T23:59:59.999Z"
}

func holidayDatePeriodSnake(out map[string]any, fallbackStart, fallbackEnd string) map[string]any {
	start := fallbackStart
	end := fallbackEnd
	if raw, ok := out["datePeriod"].(map[string]any); ok {
		if v := firstReportString(raw, "startDate", "start_date"); v != "" {
			start = v
		}
		if v := firstReportString(raw, "endDate", "end_date"); v != "" {
			end = v
		}
	}
	return map[string]any{"start_date": start, "end_date": end}
}

func (s *Service) DeleteHoliday(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	holidayID := stringArg(args, "holiday_id")
	if err := resolve.ValidateID(holidayID, "holiday_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_holiday",
			Data:   dryrun.MinimalResult("clockify_delete_holiday", map[string]any{"holiday_id": holidayID}),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "holidays", holidayID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_holiday", map[string]any{
		"deleted":   true,
		"holidayId": holidayID,
	}, map[string]any{"workspaceId": wsID}), nil
}
