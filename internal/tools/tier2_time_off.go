package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func timeOffHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. clockify_list_time_off_requests (RO)
		{
			Tool: withOutputSchema(toolRO("clockify_list_time_off_requests",
				"List time off requests with optional status filter",
				map[string]any{"type": "object", "properties": map[string]any{
					"status":    map[string]any{"type": "string", "enum": []string{"PENDING", "APPROVED", "REJECTED", "DENIED", "ALL"}, "description": "Filter by status: PENDING, APPROVED, REJECTED, ALL (default ALL). DENIED is accepted as a legacy alias for REJECTED and is translated before the upstream POST."},
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
				"Create a time off request under a policy. Changes leave balances/approval workflow.",
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
				"Update an existing time off request, including approval status when supplied.",
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
				"Permanently delete a time off request from the workspace policy. Admin scope; destructive; supports dry_run preview.",
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
				"Approve a pending time off request and update its approval state.",
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
				"Deny a pending time off request and update its approval state.",
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
				"Create a simplified owner-friendly time off policy. The handler builds a sensible default body around the current user (assigns the policy to them, applies an empty user-group filter, infers approval from auto_approve/requires_approval, and converts days_per_year into automaticAccrual). It intentionally omits Clockify's full surface — color/icon/expiration, half-day defaults, automatic time-entry creation, approval-stage configuration, and assignment filters. Use the raw API fallback (clockify_api_request POST /workspaces/{ws}/time-off/policies) when you need any of those fields.",
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
				"Update an existing time off policy by merging the supplied fields into the current upstream body. Same simplified surface as create: it preserves whatever the upstream stores for color/icon/expiration/automatic time-entry creation/approval stages and only mutates the fields you provide. Use the raw API fallback when you need to set fields outside this schema.",
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
		// 13. clockify_time_off_balance_update (RW; admin/billing impact)
		{
			Tool: toolRW("clockify_time_off_balance_update",
				"Adjust time off balances for one or more users under a policy. Admin and billing impact: balances drive future PTO accrual and approval. Supports dry_run preview.",
				map[string]any{"type": "object", "required": []string{"policy_id", "user_ids", "value", "note"}, "properties": map[string]any{
					"policy_id": map[string]any{"type": "string"},
					"user_ids":  map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}, "description": "User IDs (or names/emails) whose balance to adjust under this policy."},
					"value":     map[string]any{"type": "number", "description": "Absolute balance value to set (units determined by the policy time-unit, e.g. days or hours)."},
					"note":      map[string]any{"type": "string", "description": "Required note explaining the adjustment; surfaced in Clockify audit history."},
					"dry_run":   map[string]any{"type": "boolean", "description": "Preview the PATCH payload without calling Clockify."},
				}}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.updateTimeOffBalance(ctx, args)
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
		body["statuses"] = []string{normalizeTimeOffRequestStatus(status)}
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
	note := stringArg(args, "note")
	if note != "" {
		changed = append(changed, "note")
	}
	if status := stringArg(args, "status"); status != "" {
		if status != "APPROVED" && status != "REJECTED" {
			return ResultEnvelope{}, fmt.Errorf("status must be APPROVED or REJECTED; got %q", status)
		}
		body = timeOffRequestStatusBody(status, note)
		changed = append(changed, "status")
	}
	if _, ok := body["status"]; !ok {
		return ResultEnvelope{}, fmt.Errorf("status is required for time off request update; use APPROVED or REJECTED")
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
	for _, status := range []string{"", "PENDING", "APPROVED", "REJECTED"} {
		body := map[string]any{"page": 1, "pageSize": 200}
		if status != "" {
			body["statuses"] = []string{status}
		}
		envelope.Requests = nil
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

	payload := timeOffRequestStatusBody("APPROVED", stringArg(args, "note"))

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

	payload := timeOffRequestStatusBody("REJECTED", stringArg(args, "note"))

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

func timeOffRequestStatusBody(status, note string) map[string]any {
	return map[string]any{
		"status": status,
		"note":   note,
	}
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

	currentUser, err := s.getCurrentUser(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	unit := timeOffPolicyTimeUnit(args, nil)
	payload := map[string]any{
		"name":       name,
		"approve":    timeOffPolicyApproval(args, nil),
		"timeUnit":   unit,
		"userGroups": timeOffPolicyFilter(nil),
		"users":      timeOffPolicyFilter([]string{currentUser.ID}),
	}
	if days, ok := numberArg(args, "days_per_year"); ok {
		payload["automaticAccrual"] = map[string]any{"amount": days, "period": "YEAR", "timeUnit": unit}
	}
	if negBal, ok := optionalBoolArg(args, "negative_balance"); ok {
		applyTimeOffPolicyNegativeBalance(payload, negBal, unit)
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
	if unit := stringArg(args, "time_unit"); unit != "" {
		existing["timeUnit"] = unit
		changed = append(changed, "timeUnit")
	}
	unit := timeOffPolicyTimeUnit(args, existing)
	if days, ok := numberArg(args, "days_per_year"); ok {
		existing["automaticAccrual"] = map[string]any{"amount": days, "period": "YEAR", "timeUnit": unit}
		changed = append(changed, "automaticAccrual")
	}
	if negBal, ok := optionalBoolArg(args, "negative_balance"); ok {
		applyTimeOffPolicyNegativeBalance(existing, negBal, unit)
		changed = append(changed, "allowNegativeBalance", "negativeBalance")
	}
	if _, ok := optionalBoolArg(args, "requires_approval"); ok {
		existing["approve"] = timeOffPolicyApproval(args, existing)
		delete(existing, "requiresApproval")
		changed = append(changed, "approve")
	}
	normalizeTimeOffPolicyWriteBody(existing)

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

func timeOffPolicyTimeUnit(args map[string]any, existing map[string]any) string {
	if unit := stringArg(args, "time_unit"); unit != "" {
		return unit
	}
	if existing != nil {
		if unit, ok := existing["timeUnit"].(string); ok && unit != "" {
			return unit
		}
	}
	return "DAYS"
}

func timeOffPolicyApproval(args map[string]any, existing map[string]any) map[string]any {
	approval := map[string]any{"requiresApproval": false}
	if existing != nil {
		if existingApproval, ok := existing["approve"].(map[string]any); ok {
			approval = cloneMap(existingApproval)
		}
	}
	if requiresApproval, ok := optionalBoolArg(args, "requires_approval"); ok {
		approval["requiresApproval"] = requiresApproval
	}
	return approval
}

func applyTimeOffPolicyNegativeBalance(body map[string]any, allow bool, unit string) {
	body["allowNegativeBalance"] = allow
	if !allow {
		delete(body, "negativeBalance")
		return
	}
	body["negativeBalance"] = map[string]any{
		"amount":                 10,
		"amountValidForTimeUnit": true,
		"period":                 "YEAR",
		"shouldReset":            false,
		"timeUnit":               unit,
	}
}

func normalizeTimeOffPolicyWriteBody(body map[string]any) {
	if _, ok := body["approve"].(map[string]any); !ok {
		body["approve"] = timeOffPolicyApproval(nil, nil)
	}
	if _, ok := body["allowHalfDay"]; !ok {
		body["allowHalfDay"] = false
	}
	if _, ok := body["allowNegativeBalance"]; !ok {
		body["allowNegativeBalance"] = body["negativeBalance"] != nil
	}
	if _, ok := body["archived"]; !ok {
		body["archived"] = false
	}
	if _, ok := body["everyoneIncludingNew"]; !ok {
		body["everyoneIncludingNew"] = false
	}
	if _, ok := body["hasExpiration"]; !ok {
		body["hasExpiration"] = false
	}
	if _, ok := body["userGroups"].(map[string]any); !ok {
		body["userGroups"] = timeOffPolicyFilter(anyStringSlice(firstPresent(body, "userGroupIds", "user_group_ids")))
	}
	if _, ok := body["users"].(map[string]any); !ok {
		body["users"] = timeOffPolicyFilter(anyStringSlice(firstPresent(body, "userIds", "user_ids")))
	}
	if boolFromAny(body["allowNegativeBalance"]) {
		negativeBalance, ok := body["negativeBalance"].(map[string]any)
		if !ok {
			applyTimeOffPolicyNegativeBalance(body, true, timeOffPolicyTimeUnit(nil, body))
		} else if amount, ok := reportNumber(negativeBalance["amount"]); !ok || amount <= 0 {
			negativeBalance["amount"] = 10
		}
	}
	delete(body, "requiresApproval")
}

func timeOffPolicyFilter(ids []string) map[string]any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			out = append(out, id)
		}
	}
	return map[string]any{"contains": "CONTAINS", "ids": out, "status": "ACTIVE"}
}

func (s *Service) archiveTimeOffPolicy(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}
	archived := true
	if value, ok := optionalBoolArg(args, "archived"); ok {
		archived = value
	}
	status := "ACTIVE"
	if archived {
		status = "ARCHIVED"
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "time-off", "policies", policyID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var upstream map[string]any
	if err := s.Client.Patch(ctx, path, map[string]any{"status": status}, &upstream); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_time_off_archive", map[string]any{
		"policyId": policyID,
		"archived": archived,
		"status":   status,
		"raw":      upstream,
	}, map[string]any{"workspaceId": wsID, "policyId": policyID}), nil
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

// normalizeTimeOffRequestStatus translates the legacy DENIED alias to
// REJECTED before any upstream call. Clockify's documented status enum
// uses REJECTED; older callers and prior MCP versions exposed DENIED, so
// the alias is preserved for back-compat without polluting the wire.
func normalizeTimeOffRequestStatus(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "DENIED") {
		return "REJECTED"
	}
	return strings.TrimSpace(status)
}

// updateTimeOffBalance issues PATCH /workspaces/{ws}/time-off/balance/policy/{policyId}
// with {note, userIds, value}. The endpoint returns 204 No Content on success;
// the envelope echoes resolved IDs and the requested value so callers have a
// concrete record of the adjustment for audit logs and follow-up reads.
func (s *Service) updateTimeOffBalance(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	policyID := stringArg(args, "policy_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ResultEnvelope{}, err
	}

	userRefs := stringSliceArg(args, "user_ids")
	if len(userRefs) == 0 {
		return ResultEnvelope{}, fmt.Errorf("user_ids is required")
	}

	note := strings.TrimSpace(stringArg(args, "note"))
	if note == "" {
		return ResultEnvelope{}, fmt.Errorf("note is required")
	}

	value, hasValue := numberArg(args, "value")
	if !hasValue {
		return ResultEnvelope{}, fmt.Errorf("value is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	resolvedUsers := make([]string, 0, len(userRefs))
	for _, ref := range userRefs {
		resolved, err := s.resolveUserID(ctx, wsID, ref)
		if err != nil {
			return ResultEnvelope{}, err
		}
		resolvedUsers = append(resolvedUsers, resolved)
	}

	payload := map[string]any{
		"note":    note,
		"userIds": resolvedUsers,
		"value":   value,
	}

	path, err := paths.Workspace(wsID, "time-off", "balance", "policy", policyID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ok("clockify_time_off_balance_update", dryrunPreviewPayload("clockify_time_off_balance_update", payload), map[string]any{
			"workspaceId": wsID,
			"policyId":    policyID,
			"userIds":     resolvedUsers,
		}), nil
	}

	if err := s.Client.Patch(ctx, path, payload, nil); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_time_off_balance_update", map[string]any{
		"policyId": policyID,
		"userIds":  resolvedUsers,
		"value":    value,
		"note":     note,
	}, map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
		"userIds":     resolvedUsers,
	}), nil
}
