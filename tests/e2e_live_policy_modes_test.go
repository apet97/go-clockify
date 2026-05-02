//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

// TestLivePolicyModes parametrically exercises every policy mode the
// pipeline supports and pins the gate's allow / deny behaviour
// against a real Clockify backend through the MCP path. The matrix:
//
//	read_only           — workspace creates DENY
//	time_tracking_safe  — workspace creates DENY
//	safe_core           — workspace creates ALLOW
//	standard            — workspace creates ALLOW
//	full                — workspace creates ALLOW
//
// For each mode the test issues clockify_create_client. When ALLOWED
// the call produces a real client upstream; the cleanup registry
// archive-then-deletes it. When DENIED the call must error with
// "blocked by policy" or "<mode>" in the diagnostic, AND a
// cross-check via the raw client confirms no client materialised
// upstream — proving the policy gate executed before the upstream
// round-trip rather than firing the request and discarding the
// response.
//
// The test deliberately does NOT exercise own-entry writes
// (log_time / start_timer) because those would create real
// time-tracking entries on the workspace owner's calendar; the
// create_client path achieves the same gate-coverage goal with a
// cleaner blast radius.
//
// Tier-2 group activation policy is enforced in production at
// mcp.Server.ActivateGroup via the Activator hook (see
// internal/mcp/tools.go:308). The live test harness does not wire
// Server.Activator (only the runtime does), so the activation gate
// cannot be exercised through this MCP path without first wiring
// it — which would change the harness contract. Coverage of that
// gate lives in internal/mcp/activation_integration_test.go and
// internal/mcp/integration_test.go (unit-shaped). This file
// covers the per-tool IsAllowed gate, which is the gate that an
// AI client primarily encounters on this surface.
//
// This test pins the security contract documented in
// docs/auth-model.md and AGENTS.md:119-124. A regression that
// silently widens any of these gates fails this test.
func TestLivePolicyModes(t *testing.T) {
	cases := []struct {
		mode              policy.Mode
		clientCreateAllow bool
	}{
		{policy.ReadOnly, false},
		{policy.TimeTrackingSafe, false},
		{policy.SafeCore, true},
		{policy.Standard, true},
		{policy.Full, true},
	}

	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: tc.mode})
			c := setupLiveCampaign(t, h)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			name := c.LivePrefix("policy-"+string(tc.mode), 0)
			if tc.clientCreateAllow {
				result := h.callOK(ctx, "clockify_create_client", map[string]any{
					"name": name,
				})
				data := extractDataMap(t, result)
				id, _ := data["id"].(string)
				if id == "" {
					t.Fatalf("create_client returned no id: %#v", data)
				}
				c.RegisterCleanup("client", id, func(ctx context.Context) error {
					return c.rawArchiveAndDeleteClient(ctx, id)
				})
				return
			}

			errMsg := h.callExpectError(ctx, "clockify_create_client", map[string]any{
				"name": name,
			})
			low := strings.ToLower(errMsg)
			if !strings.Contains(low, "blocked by policy") &&
				!strings.Contains(low, string(tc.mode)) {
				t.Fatalf("expected policy-block error mentioning %q, got: %q", tc.mode, errMsg)
			}
			// Cross-check via raw client: nothing was created
			// upstream. Policy-block must execute before any
			// network round-trip.
			var clients []map[string]any
			if err := c.rawGetPath(ctx, "/clients?page-size=200&name="+name, &clients); err != nil {
				t.Fatalf("raw list clients: %v", err)
			}
			for _, cl := range clients {
				if got, _ := cl["name"].(string); got == name {
					t.Fatalf("policy regression: client %q materialised under %q despite block", name, tc.mode)
				}
			}
		})
	}
}
