package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "time_off",
		Description: "Time off policies, requests, and balances",
		Keywords:    []string{"time off", "time-off", "vacation", "leave", "pto", "policy", "balance"},
		ToolNames: []string{
			"clockify_list_time_off_requests",
			"clockify_get_time_off_request",
			"clockify_create_time_off_request",
			"clockify_update_time_off_request",
			"clockify_delete_time_off_request",
			"clockify_approve_time_off",
			"clockify_deny_time_off",
			"clockify_list_time_off_policies",
			"clockify_get_time_off_policy",
			"clockify_create_time_off_policy",
			"clockify_update_time_off_policy",
			"clockify_time_off_balance",
		},
		Builder: timeOffHandlers,
	})
}

func timeOffHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. clockify_list_time_off_requests (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_list_time_off_requests",
				"List time off requests with optional status filter",
				map[string]any{"type": "object", "properties": map[string]any{
					"status":    map[string]any{"type": "string", "description": "Filter by status: PENDING, APPROVED, DENIED, ALL (default ALL)"},
					"user_id":   map[string]any{"type": "string", "description": "Filter by user ID or name/email"},
					"page":      map[string]any{"type": "integer"},
					"page_size": map[string]any{"type": "integer"},
				}}), envelopeSchemaFor[[]TimeOffRequestView]("clockify_list_time_off_requests")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.listTimeOffRequests(ctx, args)
			},
		},
		// 2. clockify_get_time_off_request (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_get_time_off_request",
				"Get a time off request by policy ID and request ID",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
				}}), envelopeSchemaFor[TimeOffRequestView]("clockify_get_time_off_request")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getTimeOffRequest(ctx, args)
			},
		},
		// 3. clockify_create_time_off_request (RW)
		{
			Tool: toolRW("clockify_create_time_off_request",
				"Create a time off request under a policy",
				map[string]any{"type": "object", "required": []string{"policy_id", "start", "end", "note"}, "properties": map[string]any{
					"policy_id": map[string]any{"type": "string"},
					"start":     map[string]any{"type": "string", "description": "Start date (YYYY-MM-DD or RFC3339)"},
					"end":       map[string]any{"type": "string", "description": "End date (YYYY-MM-DD or RFC3339)"},
					"note":      map[string]any{"type": "string", "description": "Required note/reason"},
					"half_day":  map[string]any{"type": "boolean", "description": "Request half day"},
					"timezone":  timezoneInputProperty(),
					"dry_run":   map[string]any{"type": "boolean", "description": "Preview request creation without making changes"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.createTimeOffRequest(ctx, args)
			},
		},
		// 4. clockify_update_time_off_request (RW)
		{
			Tool: toolRW("clockify_update_time_off_request",
				"Update an existing time off request",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"note":       map[string]any{"type": "string"},
					"status": map[string]any{
						"type":        "string",
						"description": "Status to set via Clockify's PATCH route",
						"enum":        []string{"APPROVED", "REJECTED"},
					},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateTimeOffRequest(ctx, args)
			},
		},
		// 5. clockify_delete_time_off_request (destructive)
		{
			Tool: toolDestructive("clockify_delete_time_off_request",
				"Delete a time off request (supports dry_run preview)",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"dry_run":    map[string]any{"type": "boolean", "description": "Preview deletion without making changes"},
				}}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.deleteTimeOffRequest(ctx, args)
			},
		},
		// 6. clockify_approve_time_off (RW)
		{
			Tool: toolRW("clockify_approve_time_off",
				"Approve a pending time off request",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"note":       map[string]any{"type": "string", "description": "Optional approval note"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.approveTimeOff(ctx, args)
			},
		},
		// 7. clockify_deny_time_off (RW)
		{
			Tool: toolRW("clockify_deny_time_off",
				"Deny a pending time off request",
				map[string]any{"type": "object", "required": []string{"policy_id", "request_id"}, "properties": map[string]any{
					"policy_id":  map[string]any{"type": "string"},
					"request_id": map[string]any{"type": "string"},
					"note":       map[string]any{"type": "string", "description": "Optional denial reason"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.denyTimeOff(ctx, args)
			},
		},
		// 8. clockify_list_time_off_policies (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_list_time_off_policies",
				"List time off policies for the workspace",
				map[string]any{"type": "object", "properties": map[string]any{
					"page":      map[string]any{"type": "integer"},
					"page_size": map[string]any{"type": "integer"},
				}}), envelopeSchemaFor[[]TimeOffPolicyView]("clockify_list_time_off_policies")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.listTimeOffPolicies(ctx, args)
			},
		},
		// 9. clockify_get_time_off_policy (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_get_time_off_policy",
				"Get a time off policy by ID",
				map[string]any{"type": "object", "required": []string{"policy_id"}, "properties": map[string]any{
					"policy_id": map[string]any{"type": "string"},
				}}), envelopeSchemaFor[TimeOffPolicyView]("clockify_get_time_off_policy")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.getTimeOffPolicy(ctx, args)
			},
		},
		// 10. clockify_create_time_off_policy (RW)
		{
			Tool: toolRW("clockify_create_time_off_policy",
				"Create a new time off policy",
				map[string]any{"type": "object", "required": []string{"name"}, "properties": map[string]any{
					"name":              map[string]any{"type": "string"},
					"accrual":           map[string]any{"type": "boolean", "description": "Whether the policy uses accrual"},
					"auto_approve":      map[string]any{"type": "boolean", "description": "Auto-approve requests"},
					"days_per_year":     map[string]any{"type": "number"},
					"negative_balance":  map[string]any{"type": "boolean", "description": "Allow negative balances"},
					"requires_approval": map[string]any{"type": "boolean"},
					"time_unit":         map[string]any{"type": "string", "description": "DAYS or HOURS"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.createTimeOffPolicy(ctx, args)
			},
		},
		// 11. clockify_update_time_off_policy (RW)
		{
			Tool: toolRW("clockify_update_time_off_policy",
				"Update an existing time off policy",
				map[string]any{"type": "object", "required": []string{"policy_id"}, "properties": map[string]any{
					"policy_id":         map[string]any{"type": "string"},
					"name":              map[string]any{"type": "string"},
					"accrual":           map[string]any{"type": "boolean"},
					"auto_approve":      map[string]any{"type": "boolean"},
					"days_per_year":     map[string]any{"type": "number"},
					"negative_balance":  map[string]any{"type": "boolean"},
					"requires_approval": map[string]any{"type": "boolean"},
					"time_unit":         map[string]any{"type": "string"},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateTimeOffPolicy(ctx, args)
			},
		},
		// 12. clockify_time_off_balance (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_time_off_balance",
				"Get time off balance for a user under a specific policy",
				map[string]any{"type": "object", "required": []string{"policy_id", "user_id"}, "properties": map[string]any{
					"policy_id": map[string]any{"type": "string"},
					"user_id":   map[string]any{"type": "string", "description": "User ID or name/email"},
				}}), envelopeSchemaFor[TimeOffBalanceView]("clockify_time_off_balance")),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.timeOffBalance(ctx, args)
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Time-off handler implementations
// ---------------------------------------------------------------------------

func (s *Service) listTimeOffRequests(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	// /time-off/requests is POST-only with a JSON search body. Filters
	// that were query params before now go inside the body — `statuses`
	// and `users` are array-shaped per TIMEOFFDOC.md.
	body := map[string]any{
		"page":     page,
		"pageSize": pageSize,
	}
	if status := stringArg(args, "status"); status != "" {
		body["statuses"] = []string{status}
	}
	if uid := stringArg(args, "user_id"); uid != "" {
		resolved, err := s.resolveUserID(ctx, wsID, uid)
		if err != nil {
			return ResultEnvelope{}, err
		}
		body["users"] = []string{resolved}
	}

	path, err := paths.Workspace(wsID, "time-off", "requests")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var envelope struct {
		Count    int              `json:"count"`
		Requests []map[string]any `json:"requests"`
	}
	if err := s.Client.Post(ctx, path, body, &envelope); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_time_off_requests", timeOffRequestViewsFromRaw(envelope.Requests), map[string]any{
		"workspaceId": wsID,
		"count":       len(envelope.Requests),
		"total":       envelope.Count,
		"page":        page,
		"pageSize":    pageSize,
	}), nil
}

func (s *Service) getTimeOffRequest(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	requestID := stringArg(args, "request_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}
	if err := resolve.ValidateID(requestID, "request_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	request, err := s.fetchTimeOffRequest(ctx, wsID, policyID, requestID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_get_time_off_request", timeOffRequestViewFromRaw(request), map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
	}), nil
}

func (s *Service) createTimeOffRequest(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return ResultEnvelope{}, fmt.Errorf("start and end are required")
	}
	note := stringArg(args, "note")
	if note == "" {
		return ResultEnvelope{}, fmt.Errorf("note is required")
	}
	loc, _ := s.locationFromArgs(args)
	days, err := timeOffRequestDays(startRaw, endRaw, loc)
	if err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	halfDay, _ := args["half_day"].(bool)
	payload := map[string]any{
		"note": note,
		"timeOffPeriod": map[string]any{
			"period": map[string]any{
				"start": startRaw,
				"end":   endRaw,
				"days":  days,
			},
			"isHalfDay":            halfDay,
			"halfDayPeriod":        "NOT_DEFINED",
			"timeOffHalfDayPeriod": "NOT_DEFINED",
		},
	}

	var result map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if dryrun.Enabled(args) {
		return ok("clockify_create_time_off_request", dryrunPreviewPayload("clockify_create_time_off_request", payload), map[string]any{
			"workspaceId": wsID,
			"policyId":    policyID,
		}), nil
	}
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_create_time_off_request", timeOffRequestViewFromRaw(result), map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
	}), nil
}

// timeOffRequestDays computes the inclusive day span between two
// flexible date inputs interpreted in loc. Bucketing both endpoints in
// the caller's location prevents a request from a non-UTC user from
// collapsing or expanding by one day because midnight local maps to a
// different UTC date. Nil loc falls back to time.UTC.
func timeOffRequestDays(startRaw, endRaw string, loc *time.Location) (int, error) {
	if loc == nil {
		loc = time.UTC
	}
	start, err := parseFlexibleDateTime(startRaw, loc)
	if err != nil {
		return 0, fmt.Errorf("start: %w", err)
	}
	end, err := parseFlexibleDateTime(endRaw, loc)
	if err != nil {
		return 0, fmt.Errorf("end: %w", err)
	}
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, loc)
	if endDay.Before(startDay) {
		return 0, fmt.Errorf("end must be on or after start")
	}
	return int(endDay.Sub(startDay).Hours()/24) + 1, nil
}

func (s *Service) updateTimeOffRequest(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	requestID := stringArg(args, "request_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}
	if err := resolve.ValidateID(requestID, "request_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests", requestID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	changed := make([]string, 0, 2)
	if v := stringArg(args, "note"); v != "" {
		body["note"] = v
		changed = append(changed, "note")
	}
	if status := stringArg(args, "status"); status != "" {
		if status != "APPROVED" && status != "REJECTED" {
			return ResultEnvelope{}, fmt.Errorf("status must be APPROVED or REJECTED; got %q", status)
		}
		body["status"] = status
		changed = append(changed, "status")
	}
	if len(body) == 0 {
		return ResultEnvelope{}, fmt.Errorf("at least one field (note, status) must be provided for update")
	}

	var result map[string]any
	if err := s.Client.Patch(ctx, path, body, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_update_time_off_request", timeOffRequestViewFromRaw(result), map[string]any{
		"workspaceId":   wsID,
		"policyId":      policyID,
		"changedFields": changed,
	}), nil
}

func (s *Service) deleteTimeOffRequest(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	requestID := stringArg(args, "request_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}
	if err := resolve.ValidateID(requestID, "request_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests", requestID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		request, err := s.fetchTimeOffRequest(ctx, wsID, policyID, requestID)
		if err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_time_off_request",
			Data:   dryrun.WrapResult(request, "clockify_delete_time_off_request"),
			Meta:   map[string]any{"workspaceId": wsID, "policyId": policyID},
		}, nil
	}

	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_delete_time_off_request", map[string]any{
		"deleted":   true,
		"requestId": requestID,
		"policyId":  policyID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) fetchTimeOffRequest(ctx context.Context, wsID, policyID, requestID string) (map[string]any, error) {
	path, err := paths.Workspace(wsID, "time-off", "requests", requestID)
	if err != nil {
		return nil, err
	}
	var request map[string]any
	if err := s.Client.Get(ctx, path, nil, &request); err == nil {
		return request, nil
	}

	searchPath, err := paths.Workspace(wsID, "time-off", "requests")
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Requests []map[string]any `json:"requests"`
	}
	body := map[string]any{"page": 1, "pageSize": 200}
	if err := s.Client.Post(ctx, searchPath, body, &envelope); err != nil {
		return nil, err
	}
	for _, candidate := range envelope.Requests {
		if firstReportString(candidate, "id", "_id", "requestId", "request_id") != requestID {
			continue
		}
		if policyID == "" || firstReportString(candidate, "policyId", "policy_id") == policyID {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("time off request %s not found", requestID)
}

func (s *Service) approveTimeOff(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	requestID := stringArg(args, "request_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}
	if err := resolve.ValidateID(requestID, "request_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{
		"status": "APPROVED",
	}
	if note := stringArg(args, "note"); note != "" {
		payload["note"] = note
	}

	var result map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests", requestID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Patch(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_approve_time_off", timeOffRequestViewFromRaw(result), map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
		"requestId":   requestID,
	}), nil
}

func (s *Service) denyTimeOff(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	requestID := stringArg(args, "request_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}
	if err := resolve.ValidateID(requestID, "request_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{
		"status": "REJECTED",
	}
	if note := stringArg(args, "note"); note != "" {
		payload["note"] = note
	}

	var result map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests", requestID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Patch(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_deny_time_off", timeOffRequestViewFromRaw(result), map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
		"requestId":   requestID,
	}), nil
}

func (s *Service) listTimeOffPolicies(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
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

	var policies []map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Get(ctx, path, query, &policies); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_time_off_policies", timeOffPolicyViewsFromRaw(policies), map[string]any{
		"workspaceId": wsID,
		"count":       len(policies),
	}), nil
}

func (s *Service) getTimeOffPolicy(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var policy map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies", policyID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Get(ctx, path, nil, &policy); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_get_time_off_policy", timeOffPolicyViewFromRaw(policy), map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
	}), nil
}

func (s *Service) createTimeOffPolicy(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
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
	if accrual, ok := args["accrual"].(bool); ok {
		payload["accrual"] = accrual
	}
	if autoApprove, ok := args["auto_approve"].(bool); ok {
		payload["autoApprove"] = autoApprove
	}
	if dpy, ok := args["days_per_year"]; ok {
		payload["daysPerYear"] = dpy
	}
	if negBal, ok := args["negative_balance"].(bool); ok {
		payload["negativeBalance"] = negBal
	}
	if reqApproval, ok := args["requires_approval"].(bool); ok {
		payload["requiresApproval"] = reqApproval
	}
	if unit := stringArg(args, "time_unit"); unit != "" {
		payload["timeUnit"] = unit
	}

	var result map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_create_time_off_policy", timeOffPolicyViewFromRaw(result), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) updateTimeOffPolicy(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Fetch existing for merge
	var existing map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies", policyID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Get(ctx, path, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	changed := make([]string, 0, 8)
	if v := stringArg(args, "name"); v != "" {
		existing["name"] = v
		changed = append(changed, "name")
	}
	if accrual, ok := args["accrual"].(bool); ok {
		existing["accrual"] = accrual
		changed = append(changed, "accrual")
	}
	if autoApprove, ok := args["auto_approve"].(bool); ok {
		existing["autoApprove"] = autoApprove
		changed = append(changed, "autoApprove")
	}
	if dpy, ok := args["days_per_year"]; ok {
		existing["daysPerYear"] = dpy
		changed = append(changed, "daysPerYear")
	}
	if negBal, ok := args["negative_balance"].(bool); ok {
		existing["negativeBalance"] = negBal
		changed = append(changed, "negativeBalance")
	}
	if reqApproval, ok := args["requires_approval"].(bool); ok {
		existing["requiresApproval"] = reqApproval
		changed = append(changed, "requiresApproval")
	}
	if unit := stringArg(args, "time_unit"); unit != "" {
		existing["timeUnit"] = unit
		changed = append(changed, "timeUnit")
	}

	var result map[string]any
	if err := s.Client.Put(ctx, path, existing, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_update_time_off_policy", timeOffPolicyViewFromRaw(result), map[string]any{
		"workspaceId":   wsID,
		"policyId":      policyID,
		"changedFields": changed,
	}), nil
}

func (s *Service) timeOffBalance(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}

	userRef := stringArg(args, "user_id")
	if userRef == "" {
		return ResultEnvelope{}, fmt.Errorf("user_id is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	userID, err := s.resolveUserID(ctx, wsID, userRef)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var envelope struct {
		Count    int              `json:"count"`
		Balances []map[string]any `json:"balances"`
	}
	path, err := paths.Workspace(wsID, "time-off", "balance", "user", userID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	query := map[string]string{"page": "1", "page-size": "200"}
	if err := s.Client.Get(ctx, path, query, &envelope); err != nil {
		return ResultEnvelope{}, err
	}
	var balance map[string]any
	for _, candidate := range envelope.Balances {
		if timeOffBalancePolicyID(candidate) == policyID {
			balance = candidate
			break
		}
	}
	if balance == nil {
		return ResultEnvelope{}, fmt.Errorf("time off balance for policy_id %s and user_id %s not found", policyID, userID)
	}

	return ok("clockify_time_off_balance", timeOffBalanceViewFromRaw(balance), map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
		"userId":      userID,
		"total":       envelope.Count,
	}), nil
}

func timeOffBalancePolicyID(raw map[string]any) string {
	if id := firstReportString(raw, "policyId", "policy_id"); id != "" {
		return id
	}
	if policy, ok := raw["policy"].(map[string]any); ok {
		return firstReportString(policy, "id", "_id", "policyId", "policy_id")
	}
	return ""
}
