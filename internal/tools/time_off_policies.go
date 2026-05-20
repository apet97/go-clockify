package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func (s *Service) listTimeOffPolicies(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	policies, page, pageSize, err := s.fetchTimeOffPolicies(ctx, wsID, args)
	if err != nil {
		return ResultEnvelope{}, err
	}

	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(policies),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	return ok("clockify_list_time_off_policies", compactTimeOffPolicyViewsFromRaw(policies), emptyListMeta(meta, "clockify_time_off_policies_create")), nil
}

func (s *Service) fetchTimeOffPolicies(ctx context.Context, wsID string, args map[string]any) ([]map[string]any, int, int, error) {
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)
	query := map[string]string{
		"page":      fmt.Sprintf("%d", page),
		"page-size": fmt.Sprintf("%d", pageSize),
	}

	var policies []map[string]any
	path, err := paths.Workspace(wsID, "time-off", "policies")
	if err != nil {
		return nil, page, pageSize, err
	}
	if err := s.Client.Get(ctx, path, query, &policies); err != nil {
		return nil, page, pageSize, err
	}
	return policies, page, pageSize, nil
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
