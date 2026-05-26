package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

// ---------------------------------------------------------------------------
// Time-off handler implementations
// ---------------------------------------------------------------------------

func (s *Service) listTimeOffRequests(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	page, pageSize := paginationFromArgs(args)

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
			return ToolResult{}, err
		}
		body["users"] = []string{resolved}
	}

	path, err := paths.Workspace(wsID, "time-off", "requests")
	if err != nil {
		return ToolResult{}, err
	}
	var envelope struct {
		Count    int              `json:"count"`
		Requests []map[string]any `json:"requests"`
	}
	if err := s.Client.Post(ctx, path, body, &envelope); err != nil {
		return ToolResult{}, err
	}

	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(envelope.Requests),
		"total":       envelope.Count,
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	if envelope.Count > 0 {
		meta["has_more"] = page*pageSize < envelope.Count
	}
	return ok("clockify_list_time_off_requests", timeOffRequestViewsFromRaw(envelope.Requests), emptyListMeta(meta, "clockify_request_time_off")), nil
}

func (s *Service) getTimeOffRequest(ctx context.Context, args map[string]any) (ToolResult, error) {
	policyID := stringArg(args, "policy_id")
	requestID := stringArg(args, "request_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ToolResult{}, err
	}
	if err := resolve.ValidateID(requestID, "request_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	request, err := s.fetchTimeOffRequest(ctx, wsID, policyID, requestID)
	if err != nil {
		return ToolResult{}, err
	}

	return ok("clockify_get_time_off_request", timeOffRequestViewFromRaw(request), map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
	}), nil
}

func (s *Service) createTimeOffRequest(ctx context.Context, args map[string]any) (ToolResult, error) {
	policyID := stringArg(args, "policy_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ToolResult{}, err
	}

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw == "" || endRaw == "" {
		return ToolResult{}, fmt.Errorf("start and end are required")
	}
	note := stringArg(args, "note")
	if note == "" && !boolFromAny(args["__allow_empty_note"]) {
		return ToolResult{}, fmt.Errorf("note is required")
	}
	loc, err := s.locationFromArgs(args)
	if err != nil {
		return ToolResult{}, fmt.Errorf("invalid timezone: %w", err)
	}
	days, err := timeOffRequestDays(startRaw, endRaw, loc)
	if err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	halfDay, _ := args["half_day"].(bool)
	payload := map[string]any{
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
	if note != "" {
		payload["note"] = note
	}

	var result map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests")
	if err != nil {
		return ToolResult{}, err
	}
	if dryrun.Enabled(args) {
		return ok("clockify_create_time_off_request", dryrunPreviewPayload("clockify_create_time_off_request", payload), map[string]any{
			"workspaceId": wsID,
			"policyId":    policyID,
		}), nil
	}
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
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

func (s *Service) updateTimeOffRequest(ctx context.Context, args map[string]any) (ToolResult, error) {
	policyID := stringArg(args, "policy_id")
	requestID := stringArg(args, "request_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ToolResult{}, err
	}
	if err := resolve.ValidateID(requestID, "request_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests", requestID)
	if err != nil {
		return ToolResult{}, err
	}

	body := map[string]any{}
	changed := make([]string, 0, 1)
	if status := stringArg(args, "status"); status != "" {
		if status != "APPROVED" && status != "REJECTED" {
			return ToolResult{}, fmt.Errorf("status must be APPROVED or REJECTED; got %q", status)
		}
		body = timeOffRequestStatusBody(status, "")
		changed = append(changed, "status")
	}
	if _, ok := body["status"]; !ok {
		return ToolResult{}, fmt.Errorf("status is required for time off request update; use APPROVED or REJECTED")
	}

	if dryrun.Enabled(args) {
		return ok("clockify_update_time_off_request", dryrunPreviewPayload("clockify_update_time_off_request", body), map[string]any{
			"workspaceId": wsID,
			"policyId":    policyID,
			"requestId":   requestID,
		}), nil
	}

	var result map[string]any
	if err := s.Client.Patch(ctx, path, body, &result); err != nil {
		return ToolResult{}, err
	}

	// Clockify's PATCH response is sparse (no policy/user names). Mirror
	// approveTimeOff/denyTimeOff: read the request back and return the
	// enriched body. A failed re-hydration is surfaced in meta, not fatal.
	meta := map[string]any{
		"workspaceId":   wsID,
		"policyId":      policyID,
		"changedFields": changed,
	}
	if hydrated, fetchErr := s.fetchTimeOffRequest(ctx, wsID, policyID, requestID); fetchErr == nil {
		result = hydrated
	} else {
		meta["hydration"] = "unavailable"
	}

	return ok("clockify_update_time_off_request", timeOffRequestViewFromRaw(result), meta), nil
}

func (s *Service) deleteTimeOffRequest(ctx context.Context, args map[string]any) (ToolResult, error) {
	policyID := stringArg(args, "policy_id")
	requestID := stringArg(args, "request_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ToolResult{}, err
	}
	if err := resolve.ValidateID(requestID, "request_id"); err != nil {
		return ToolResult{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests", requestID)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		request, err := s.fetchTimeOffRequest(ctx, wsID, policyID, requestID)
		if err != nil {
			return ToolResult{}, err
		}
		return ToolResult{
			OK:     true,
			Action: "clockify_delete_time_off_request",
			Data:   dryrun.WrapResult(request, "clockify_delete_time_off_request"),
			Meta:   map[string]any{"workspaceId": wsID, "policyId": policyID},
		}, nil
	}

	if err := s.Client.Delete(ctx, path); err != nil {
		return ToolResult{}, err
	}

	return ok("clockify_delete_time_off_request", map[string]any{
		"deleted":   true,
		"id":        requestID,
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
