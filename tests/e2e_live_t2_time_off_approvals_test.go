//go:build legacy_platform && livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

func TestLiveT2TimeOffRemainingTools(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_FULL_SURFACE_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("time_off")

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	policies := extractList(t, h.callOK(ctx, "clockify_list_time_off_policies", map[string]any{"page": 1, "page_size": 50}))
	if len(policies) == 0 {
		t.Log("no time-off policies in sacrificial workspace; probing remaining tools through known error paths")
		exerciseTimeOffRequestErrorPaths(t, h, ctx, "000000000000000000000001", "000000000000000000000002")
		return
	}
	policyID := idFromRow(policies[0])
	if policyID == "" {
		t.Fatalf("first time-off policy has no id: %#v", policies[0])
	}

	_ = h.callOK(ctx, "clockify_get_time_off_policy", map[string]any{"policy_id": policyID})
	if _, errText := liveCallMaybe(t, h, ctx, "clockify_time_off_balance", map[string]any{
		"policy_id": policyID,
		"user_id":   c.OwnerUserID,
	}); errText != "" && !containsErrorText(errText, "balance", "not found", "permission", "plan", "400", "403", "404") {
		t.Fatalf("unexpected clockify_time_off_balance error: %q", errText)
	}

	requestID := createLiveTimeOffRequest(t, h, c, ctx, policyID, 760, "time-off")
	if requestID == "" {
		exerciseTimeOffRequestErrorPaths(t, h, ctx, policyID, "000000000000000000000002")
	} else {
		deleted := false
		c.RegisterCleanup("time-off-request", requestID, func(ctx context.Context) error {
			if deleted {
				return nil
			}
			return c.rawDeletePath(ctx, "/time-off/policies/"+policyID+"/requests/"+requestID)
		})

		if _, errText := liveCallMaybe(t, h, ctx, "clockify_get_time_off_request", map[string]any{
			"policy_id":  policyID,
			"request_id": requestID,
		}); errText != "" && !containsErrorText(errText, "method", "not supported", "405", "404") {
			t.Fatalf("unexpected clockify_get_time_off_request error: %q", errText)
		}

		_, updateErr := liveCallMaybe(t, h, ctx, "clockify_update_time_off_request", map[string]any{
			"policy_id":  policyID,
			"request_id": requestID,
			"note":       c.LivePrefix("time-off-updated", 0),
			"status":     "REJECTED",
		})
		if updateErr != "" && !containsErrorText(updateErr, "permission", "not found", "method", "400", "403", "404", "405") {
			t.Fatalf("unexpected clockify_update_time_off_request error: %q", updateErr)
		}

		if _, errText := liveCallMaybe(t, h, ctx, "clockify_delete_time_off_request", map[string]any{
			"policy_id":  policyID,
			"request_id": requestID,
			"dry_run":    true,
		}); errText != "" && !containsErrorText(errText, "method", "not supported", "405", "404") {
			t.Fatalf("unexpected dry-run delete_time_off_request error: %q", errText)
		}
		if _, errText := liveCallMaybe(t, h, ctx, "clockify_delete_time_off_request", map[string]any{
			"policy_id":  policyID,
			"request_id": requestID,
		}); errText != "" && !containsErrorText(errText, "not found", "permission", "400", "403", "404") {
			t.Fatalf("unexpected delete_time_off_request error: %q", errText)
		} else if errText == "" {
			deleted = true
		}
	}

	for _, tc := range []struct {
		tool string
		day  int
		args func(string) map[string]any
	}{
		{"clockify_approve_time_off", 761, func(id string) map[string]any {
			return map[string]any{"policy_id": policyID, "request_id": id, "note": c.LivePrefix("approve", 0)}
		}},
		{"clockify_deny_time_off", 762, func(id string) map[string]any {
			return map[string]any{"policy_id": policyID, "request_id": id, "note": c.LivePrefix("deny", 0)}
		}},
	} {
		reqID := createLiveTimeOffRequest(t, h, c, ctx, policyID, tc.day, tc.tool)
		if reqID == "" {
			reqID = "000000000000000000000002"
		}
		if _, errText := liveCallMaybe(t, h, ctx, tc.tool, tc.args(reqID)); errText != "" &&
			!containsErrorText(errText, "permission", "not found", "method", "400", "403", "404", "405") {
			t.Fatalf("unexpected %s error: %q", tc.tool, errText)
		}
	}

	createPolicyErr := h.callExpectError(ctx, "clockify_create_time_off_policy", map[string]any{
		"name": strings.Repeat("x", 200),
	})
	if !containsErrorText(createPolicyErr, "length", "validation", "400", "404", "method", "not supported") {
		t.Fatalf("unexpected create_time_off_policy validation error: %q", createPolicyErr)
	}
	updatePolicyErr := h.callExpectError(ctx, "clockify_update_time_off_policy", map[string]any{
		"policy_id": "000000000000000000000001",
		"name":      "mcp-live-policy",
	})
	if !containsErrorText(updatePolicyErr, "not found", "doesn't belong", "400", "404", "method", "not supported") {
		t.Fatalf("unexpected update_time_off_policy error: %q", updatePolicyErr)
	}
}

func TestLiveT2ApprovalsRemainingTools(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_FULL_SURFACE_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("approvals")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	approvalID := ""
	for _, row := range extractList(t, h.callOK(ctx, "clockify_list_approval_requests", map[string]any{"page": 1, "page_size": 25})) {
		if id := approvalIDFromRow(row); id != "" {
			approvalID = id
			break
		}
	}

	submit := map[string]any{
		"period":       "WEEKLY",
		"period_start": startOfWeek(time.Now().UTC()).Format("2006-01-02T15:04:05.000Z"),
	}
	if result, errText := liveCallMaybe(t, h, ctx, "clockify_submit_for_approval", submit); errText != "" {
		if !containsErrorText(errText, "approval", "period", "permission", "plan", "already", "400", "403", "409") {
			t.Fatalf("unexpected submit_for_approval error: %q", errText)
		}
	} else if id := approvalIDFromResult(result); id != "" {
		approvalID = id
	}

	if approvalID == "" {
		approvalID = "000000000000000000000001"
	}
	if _, errText := liveCallMaybe(t, h, ctx, "clockify_get_approval_request", map[string]any{
		"approval_id": approvalID,
	}); errText != "" && !containsErrorText(errText, "not found", "doesn't belong", "method", "not supported", "400", "404", "405") {
		t.Fatalf("unexpected get_approval_request error: %q", errText)
	}

	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"clockify_approve_timesheet", map[string]any{"approval_id": approvalID, "dry_run": true}},
		{"clockify_reject_timesheet", map[string]any{"approval_id": approvalID, "reason": c.LivePrefix("reject", 0), "dry_run": true}},
		{"clockify_withdraw_approval", map[string]any{"approval_id": approvalID}},
	} {
		if _, errText := liveCallMaybe(t, h, ctx, tc.tool, tc.args); errText != "" &&
			!containsErrorText(errText, "not found", "doesn't belong", "permission", "status", "400", "403", "404", "405") {
			t.Fatalf("unexpected %s error: %q", tc.tool, errText)
		}
	}
}

func createLiveTimeOffRequest(t *testing.T, h *liveMCPHarness, c *liveCampaignContext, ctx context.Context, policyID string, dayOffset int, noteSuffix string) string {
	t.Helper()
	day := time.Now().UTC().AddDate(2, 0, dayOffset).Format("2006-01-02")
	result, errText := liveCallMaybe(t, h, ctx, "clockify_create_time_off_request", map[string]any{
		"policy_id": policyID,
		"start":     day,
		"end":       day,
		"note":      c.LivePrefix(noteSuffix, 0),
	})
	if errText != "" {
		if !containsErrorText(errText, "time off", "balance", "policy", "permission", "plan", "already", "400", "403", "404", "409") {
			t.Fatalf("unexpected create_time_off_request error: %q", errText)
		}
		t.Logf("clockify_create_time_off_request unavailable for %s: %s", noteSuffix, errText)
		return ""
	}
	id, _ := extractDataMap(t, result)["id"].(string)
	if id == "" {
		t.Fatalf("clockify_create_time_off_request returned no id: %#v", result)
	}
	return id
}

func exerciseTimeOffRequestErrorPaths(t *testing.T, h *liveMCPHarness, ctx context.Context, policyID, requestID string) {
	t.Helper()
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"clockify_get_time_off_policy", map[string]any{"policy_id": policyID}},
		{"clockify_get_time_off_request", map[string]any{"policy_id": policyID, "request_id": requestID}},
		{"clockify_update_time_off_request", map[string]any{"policy_id": policyID, "request_id": requestID, "note": "x", "status": "REJECTED"}},
		{"clockify_delete_time_off_request", map[string]any{"policy_id": policyID, "request_id": requestID}},
		{"clockify_approve_time_off", map[string]any{"policy_id": policyID, "request_id": requestID}},
		{"clockify_deny_time_off", map[string]any{"policy_id": policyID, "request_id": requestID}},
		{"clockify_time_off_balance", map[string]any{"policy_id": policyID, "user_id": "000000000000000000000003"}},
	} {
		if _, errText := liveCallMaybe(t, h, ctx, tc.tool, tc.args); errText != "" &&
			!containsErrorText(errText, "not found", "doesn't belong", "method", "permission", "400", "403", "404", "405") {
			t.Fatalf("unexpected %s error: %q", tc.tool, errText)
		}
	}
}

func liveCallMaybe(t *testing.T, h *liveMCPHarness, ctx context.Context, tool string, args map[string]any) (map[string]any, string) {
	t.Helper()
	resp, err := h.rawCall(ctx, tool, args)
	if err != nil {
		t.Fatalf("rawCall %s: %v", tool, err)
	}
	if resp.Error != nil {
		return nil, resp.Error.Message
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("tool %s result was not a map: %T", tool, resp.Result)
	}
	if isErr, _ := result["isError"].(bool); isErr {
		content, _ := result["content"].([]any)
		var b strings.Builder
		for _, item := range content {
			if entry, ok := item.(map[string]any); ok {
				if text, ok := entry["text"].(string); ok {
					b.WriteString(text)
				}
			}
		}
		return nil, b.String()
	}
	return result, ""
}

func idFromRow(row any) string {
	obj, ok := row.(map[string]any)
	if !ok {
		return ""
	}
	id, _ := obj["id"].(string)
	return id
}

func approvalIDFromRow(row any) string {
	obj, ok := row.(map[string]any)
	if !ok {
		return ""
	}
	if id, _ := obj["id"].(string); id != "" {
		return id
	}
	if nested, ok := obj["approvalRequest"].(map[string]any); ok {
		id, _ := nested["id"].(string)
		return id
	}
	return ""
}

func approvalIDFromResult(result map[string]any) string {
	sc, _ := result["structuredContent"].(map[string]any)
	if data, ok := sc["data"].(map[string]any); ok {
		if id, _ := data["id"].(string); id != "" {
			return id
		}
	}
	return ""
}

func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := t.AddDate(0, 0, -(weekday - 1))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
}
