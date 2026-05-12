//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

// TestLiveFullSurfaceDocCoverage proves the probe-lab allowlist is callable
// through the MCP path against the sacrificial workspace. Mutating/doc-only
// operations are unit-covered with fake upstreams; this live probe sticks to a
// stable read endpoint while also asserting that doc-only routes are present in
// the allowlist returned to clients.
func TestLiveFullSurfaceDocCoverage(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("probe_lab_api")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	ops := extractList(t, h.callOK(ctx, "clockify_list_documented_api_operations", map[string]any{
		"contains": "invoices/settings",
	}))
	if len(ops) == 0 {
		t.Fatal("probe_lab_api allowlist did not expose doc-only invoice settings route")
	}

	result := h.callOK(ctx, "clockify_call_documented_read_api", map[string]any{
		"operation": "GET /workspaces/{workspaceId}",
	})
	data := extractDataMap(t, result)
	if got, _ := data["id"].(string); got != c.WorkspaceID {
		t.Fatalf("documented read API returned workspace id %q, want %q (data=%#v)", got, c.WorkspaceID, data)
	}

	writeDryRun := h.callOK(ctx, "clockify_call_documented_write_api", map[string]any{
		"operation": "PUT /workspaces/{workspaceId}/invoices/settings",
		"json_body": map[string]any{
			"note": "live doc coverage dry-run only",
		},
		"dry_run": true,
	})
	assertDryRunTool(t, writeDryRun, "clockify_call_documented_write_api")

	deleteDryRun := h.callOK(ctx, "clockify_call_documented_delete_api", map[string]any{
		"operation":    "DELETE /workspaces/{workspaceId}/clients/{clientId}",
		"path_params":  map[string]any{"clientId": "dry-run-client-id"},
		"dry_run":      true,
		"raw_response": false,
	})
	assertDryRunTool(t, deleteDryRun, "clockify_call_documented_delete_api")

	for _, tc := range []struct {
		operation  string
		pathParams map[string]any
	}{
		{"GET /workspaces/{workspaceId}/expenses/{expenseId}/files/{fileId}", map[string]any{"expenseId": "000000000000000000000001", "fileId": "000000000000000000000002"}},
		{"GET /workspaces/{workspaceId}/time-off/balance/user/{userId}", map[string]any{"userId": c.OwnerUserID}},
		{"GET /workspaces/{workspaceId}/time-off/requests", nil},
		{"GET /workspaces/{workspaceId}/user-groups/{groupId}", map[string]any{"groupId": "000000000000000000000001"}},
		{"GET /workspaces/{workspaceId}/user-groups/{groupId}/users", map[string]any{"groupId": "000000000000000000000001"}},
		{"GET /workspaces/{workspaceId}/users/{userId}/time-off/balances", map[string]any{"userId": c.OwnerUserID}},
		{"GET /workspaces/{workspaceId}/webhooks/{webhookId}/logs", map[string]any{"webhookId": "000000000000000000000001"}},
	} {
		assertDocumentedReadCallableOrAcceptedRefusal(t, h, ctx, tc.operation, tc.pathParams)
	}

	for _, tc := range []struct {
		operation  string
		pathParams map[string]any
	}{
		{"PUT /workspaces/{workspaceId}", nil},
		{"PUT /workspaces/{workspaceId}/time-off/policies/{policyId}", map[string]any{"policyId": "000000000000000000000001"}},
		{"PATCH /workspaces/{workspaceId}/policies/{policyId}/requests/{requestId}", map[string]any{"policyId": "000000000000000000000001", "requestId": "000000000000000000000002"}},
		{"PATCH /workspaces/{workspaceId}/webhooks/{webhookId}/token", map[string]any{"webhookId": "000000000000000000000001"}},
		{"POST /workspaces/{workspaceId}/policies/{policyId}/requests", map[string]any{"policyId": "000000000000000000000001"}},
	} {
		args := map[string]any{
			"operation": tc.operation,
			"json_body": map[string]any{"note": "live doc coverage dry-run only"},
			"dry_run":   true,
		}
		if tc.pathParams != nil {
			args["path_params"] = tc.pathParams
		}
		result := h.callOK(ctx, "clockify_call_documented_write_api", args)
		assertDryRunTool(t, result, "clockify_call_documented_write_api")
	}

	deletePolicyRequest := h.callOK(ctx, "clockify_call_documented_delete_api", map[string]any{
		"operation": "DELETE /workspaces/{workspaceId}/policies/{policyId}/requests/{requestId}",
		"path_params": map[string]any{
			"policyId":  "000000000000000000000001",
			"requestId": "000000000000000000000002",
		},
		"dry_run": true,
	})
	assertDryRunTool(t, deletePolicyRequest, "clockify_call_documented_delete_api")
}

func assertDryRunTool(t *testing.T, result map[string]any, tool string) {
	t.Helper()
	data := extractDataMap(t, result)
	if v, _ := data["dry_run"].(bool); !v {
		t.Fatalf("%s dry-run result missing dry_run=true: %#v", tool, data)
	}
	if got, _ := data["tool"].(string); got != tool {
		t.Fatalf("%s dry-run result tool=%q, want %q (data=%#v)", tool, got, tool, data)
	}
}

func assertDocumentedReadCallableOrAcceptedRefusal(t *testing.T, h *liveMCPHarness, ctx context.Context, operation string, pathParams map[string]any) {
	t.Helper()
	args := map[string]any{"operation": operation}
	if pathParams != nil {
		args["path_params"] = pathParams
	}
	if _, errText := liveCallMaybe(t, h, ctx, "clockify_call_documented_read_api", args); errText != "" {
		if containsErrorText(errText, "allowlist") {
			t.Fatalf("%s was rejected by the documented API allowlist: %s", operation, errText)
		}
		if !containsErrorText(errText, "not found", "doesn't belong", "permission", "plan", "method", "not supported", "400", "403", "404", "405") {
			t.Fatalf("%s returned unexpected live read error: %s", operation, errText)
		}
	}
}
