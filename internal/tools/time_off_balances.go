package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func (s *Service) approveTimeOff(ctx context.Context, args map[string]any) (ToolResult, error) {
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

	payload := timeOffRequestStatusBody("APPROVED", stringArg(args, "note"))

	return s.patchTimeOffStatusAndHydrate(ctx, wsID, policyID, requestID, payload, "clockify_approve_time_off")
}

func (s *Service) denyTimeOff(ctx context.Context, args map[string]any) (ToolResult, error) {
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

	req, err := s.fetchTimeOffRequest(ctx, wsID, policyID, requestID)
	if err != nil {
		return ToolResult{}, err
	}
	if status := timeOffRequestStatus(req); status != "PENDING" {
		return ToolResult{}, fmt.Errorf("cannot deny request in %s state, must be PENDING", status)
	}

	payload := timeOffRequestStatusBody("REJECTED", stringArg(args, "note"))

	return s.patchTimeOffStatusAndHydrate(ctx, wsID, policyID, requestID, payload, "clockify_deny_time_off")
}

func (s *Service) patchTimeOffStatusAndHydrate(ctx context.Context, wsID, policyID, requestID string, payload map[string]any, action string) (ToolResult, error) {
	var result map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies", policyID, "requests", requestID)
	if err != nil {
		return ToolResult{}, err
	}
	if err := s.Client.Patch(ctx, path, payload, &result); err != nil {
		return ToolResult{}, err
	}

	meta := map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
		"requestId":   requestID,
	}
	hydrated, fetchErr := s.fetchTimeOffRequest(ctx, wsID, policyID, requestID)
	if fetchErr == nil {
		result = hydrated
	} else {
		meta["hydration"] = "unavailable"
	}

	return ok(action, timeOffRequestViewFromRaw(result), meta), nil
}

func timeOffRequestStatusBody(status, note string) map[string]any {
	body := map[string]any{"status": status}
	if strings.TrimSpace(note) != "" {
		body["note"] = note
	}
	return body
}

func (s *Service) timeOffBalance(ctx context.Context, args map[string]any) (ToolResult, error) {
	policyID := stringArg(args, "policy_id")
	if policyID != "" {
		if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
			return ToolResult{}, err
		}
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	userRef := stringArg(args, "user_id")
	var userID string
	if userRef == "" {
		user, err := s.getCurrentUser(ctx)
		if err != nil {
			return ToolResult{}, err
		}
		userID = user.ID
	} else {
		var err error
		userID, err = s.resolveUserID(ctx, wsID, userRef)
		if err != nil {
			return ToolResult{}, err
		}
	}

	var envelope struct {
		Count    int              `json:"count"`
		Balances []map[string]any `json:"balances"`
	}
	path, err := paths.Workspace(wsID, "time-off", "balance", "user", userID)
	if err != nil {
		return ToolResult{}, err
	}
	page, pageSize := 1, 200
	if policyID == "" {
		page, pageSize = paginationFromArgs(args)
	}
	query := map[string]string{"page": fmt.Sprintf("%d", page), "page-size": fmt.Sprintf("%d", pageSize)}
	if err := s.Client.Get(ctx, path, query, &envelope); err != nil {
		return ToolResult{}, err
	}
	if policyID == "" {
		views := make([]TimeOffBalanceView, 0, len(envelope.Balances))
		policyIDs := make([]string, 0, len(envelope.Balances))
		for _, raw := range envelope.Balances {
			views = append(views, timeOffBalanceViewFromRaw(raw))
			if id := timeOffBalancePolicyID(raw); id != "" {
				policyIDs = append(policyIDs, id)
			}
		}
		count := len(views)
		total := max(envelope.Count, count)
		meta := map[string]any{
			"workspaceId": wsID,
			"userId":      userID,
			"policyIds":   policyIDs,
			"count":       count,
			"total":       total,
			"page":        page,
			"pageSize":    pageSize,
			"has_more":    page*pageSize < total,
		}
		if envelope.Count > count {
			meta["upstream_count_ignored"] = envelope.Count
			meta["dropped"] = envelope.Count - count
			meta["nextAction"] = fmt.Sprintf("Upstream reports %d balances; this page returned %d. Pass a higher page_size or request page %d for the rest.", envelope.Count, count, page+1)
		}
		return ok("clockify_time_off_balance", views, meta), nil
	}

	var balance map[string]any
	for _, candidate := range envelope.Balances {
		if timeOffBalancePolicyID(candidate) == policyID {
			balance = candidate
			break
		}
	}
	if balance == nil {
		return ToolResult{}, fmt.Errorf("time off balance for policy_id %s and user_id %s not found", policyID, userID)
	}

	return ok("clockify_time_off_balance", timeOffBalanceViewFromRaw(balance), map[string]any{
		"workspaceId": wsID,
		"policyId":    policyID,
		"userId":      userID,
		"policyIds":   []string{policyID},
		"count":       1,
		"total":       1,
		"page":        page,
		"pageSize":    pageSize,
		"has_more":    false,
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

// updateTimeOffBalance issues PATCH /workspaces/{ws}/time-off/balance/policy/{policyId}
// with {note, userIds, value}. The endpoint returns 204 No Content on success;
// the envelope echoes resolved IDs and the requested value so callers have a
// concrete record of the adjustment for audit logs and follow-up reads.
func (s *Service) updateTimeOffBalance(ctx context.Context, args map[string]any) (ToolResult, error) {
	policyID := stringArg(args, "policy_id")
	if err := resolve.ValidateID(policyID, "policy_id"); err != nil {
		return ToolResult{}, err
	}

	userRefs, _, err := strictStringSliceArg(args, "user_ids")
	if err != nil {
		return ToolResult{}, err
	}
	if len(userRefs) == 0 {
		return ToolResult{}, fmt.Errorf("user_ids is required")
	}

	note := strings.TrimSpace(stringArg(args, "note"))
	if note == "" {
		return ToolResult{}, fmt.Errorf("note is required")
	}

	value, hasValue := numberArg(args, "value")
	if !hasValue {
		return ToolResult{}, fmt.Errorf("value is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	resolvedUsers := make([]string, 0, len(userRefs))
	for _, ref := range userRefs {
		resolved, err := s.resolveUserID(ctx, wsID, ref)
		if err != nil {
			return ToolResult{}, err
		}
		resolvedUsers = append(resolvedUsers, resolved)
	}

	// Clockify's PATCH /time-off/balance/policy/{policyId} requires
	// userIds to be unique (see BALANCEDOC.md). Two literal IDs that
	// match, or distinct refs that resolve to the same user, would
	// otherwise produce a malformed body — fail closed before the
	// dry-run preview or the PATCH.
	seen := make(map[string]bool, len(resolvedUsers))
	for _, id := range resolvedUsers {
		if seen[id] {
			return ToolResult{}, fmt.Errorf("user_ids resolves to duplicate user %s; Clockify requires unique userIds", id)
		}
		seen[id] = true
	}

	payload := map[string]any{
		"note":    note,
		"userIds": resolvedUsers,
		"value":   value,
	}

	path, err := paths.Workspace(wsID, "time-off", "balance", "policy", policyID)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		return ok("clockify_time_off_balance_update", dryrunPreviewPayload("clockify_time_off_balance_update", payload), map[string]any{
			"workspaceId": wsID,
			"policyId":    policyID,
			"userIds":     resolvedUsers,
		}), nil
	}

	if err := s.Client.Patch(ctx, path, payload, nil); err != nil {
		return ToolResult{}, err
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
