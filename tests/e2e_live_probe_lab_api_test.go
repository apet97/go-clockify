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
}
