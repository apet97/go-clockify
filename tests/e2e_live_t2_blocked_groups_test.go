//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestLiveT2BlockedGroups pins the current per-tool status of the two
// Tier-2 groups whose entire surface is unreachable on the standard
// Clockify API host (api.clockify.me). The read-only sweep already
// pinned the list-class tools; this file extends to the create /
// update / delete / get / export / capacity tools so the bug
// inventory is complete and any handler reroute (or scheduling
// licence change) flips the relevant assertions and forces this
// file to be reviewed.
//
// Both groups appear to live on a separate Clockify host
// (reports.api.clockify.me for shared reports; a Clockify Scheduling
// API host for scheduling). The likely fix in
// internal/clockify/client.go and the relevant tier2 handlers is to
// thread additional base URLs through and route accordingly.
//
// Until that lands, every tool in these groups returns the same
// upstream "No static resource" 404 — the assertion is simply that
// the error string is what we expect today.
func TestLiveT2BlockedGroups(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	t.Run("shared_reports", func(t *testing.T) {
		c.activateTier2("shared_reports")

		// Common bogus 24-char hex id — all id-bearing tools accept
		// it without complaint at the descriptor layer; the upstream
		// 404 fires before any request validation could.
		const bogusID = "000000000000000000000001"

		cases := []struct {
			tool string
			args map[string]any
		}{
			{"clockify_get_shared_report", map[string]any{"report_id": bogusID}},
			{"clockify_create_shared_report", map[string]any{"name": c.LivePrefix("sr", 0), "report_type": "DETAILED"}},
			{"clockify_update_shared_report", map[string]any{"report_id": bogusID, "name": c.LivePrefix("sr", 1)}},
			{"clockify_delete_shared_report", map[string]any{"report_id": bogusID}},
			{"clockify_export_shared_report", map[string]any{"report_id": bogusID, "format": "json"}},
		}
		for _, tc := range cases {
			t.Run(tc.tool, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				msg := h.callExpectError(ctx, tc.tool, tc.args)
				if !strings.Contains(msg, "No static resource") {
					t.Fatalf("%s: expected 404 'No static resource', got: %q", tc.tool, msg)
				}
			})
		}
	})

	t.Run("scheduling", func(t *testing.T) {
		c.activateTier2("scheduling")

		const bogusID = "000000000000000000000001"
		end := time.Now().UTC()
		start := end.Add(-24 * time.Hour)

		cases := []struct {
			tool string
			args map[string]any
		}{
			{"clockify_get_assignment", map[string]any{"assignment_id": bogusID}},
			{"clockify_create_assignment", map[string]any{
				"user_id":       c.OwnerUserID,
				"project_id":    bogusID,
				"start":         start.Format("2006-01-02"),
				"end":           end.Format("2006-01-02"),
				"hours_per_day": 4,
			}},
			{"clockify_update_assignment", map[string]any{"assignment_id": bogusID}},
			{"clockify_delete_assignment", map[string]any{"assignment_id": bogusID}},
			{"clockify_get_schedule", map[string]any{"schedule_id": bogusID}},
			{"clockify_create_schedule", map[string]any{"name": c.LivePrefix("sched", 0)}},
		}
		for _, tc := range cases {
			t.Run(tc.tool, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				msg := h.callExpectError(ctx, tc.tool, tc.args)
				if !strings.Contains(msg, "No static resource") {
					t.Fatalf("%s: expected 404 'No static resource', got: %q", tc.tool, msg)
				}
			})
		}
	})
}
