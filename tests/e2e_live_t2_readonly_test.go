//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestLiveTier2ReadOnlySweep activates each Tier-2 group and exercises
// every read-only listing or reporting tool that has no required-id
// parameter. The sweep doubles as a regression detector for handler
// shape mismatches against the live Clockify API: each tool entry
// declares the expected outcome (success, or a known error substring).
// A change in upstream behaviour or a fix to a broken handler will
// flip the matching entry from one branch to the other and surface
// the swap in CI.
//
// Why each "expectErr" entry exists is documented inline. The bug
// inventory is the campaign's first surfaced finding — these fixtures
// turn each item into a tracked test case rather than an opaque
// failure: a fix to listInvoices's slice-vs-object unmarshal will
// flip the success/error path and force the maintainer to delete the
// expectErr annotation, making the sweep self-correcting.
//
// Per-tool shape and CRUD coverage live in the per-group phases that
// follow this sweep; this file is intentionally a flat smoke surface.
func TestLiveTier2ReadOnlySweep(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	// Scheduling endpoints reject calendar-day strings; RFC3339
	// "yyyy-MM-ddThh:mm:ssZ" is required (the upstream's error
	// message names that exact format).
	end := time.Now().UTC().Truncate(time.Second)
	start := end.Add(-24 * time.Hour)
	scheduleArgs := map[string]any{
		"start": start.Format("2006-01-02T15:04:05Z"),
		"end":   end.Format("2006-01-02T15:04:05Z"),
	}

	type call struct {
		tool string
		args map[string]any
		// expectErr, when non-empty, declares that the tool currently
		// returns a tool-error result containing this substring. The
		// sweep asserts callExpectError returns matching text. When the
		// underlying handler or upstream API is fixed, the substring
		// stops appearing and the sweep flips to FAIL — forcing the
		// fixer to delete this annotation. A short comment on the same
		// line records the suspected root cause and likely fix.
		expectErr string
	}

	groups := []struct {
		name  string
		tools []call
	}{
		{"invoices", []call{
			{"clockify_list_invoices", nil, ""},
			{"clockify_invoice_report", nil, ""},
		}},
		{"expenses", []call{
			{"clockify_list_expenses", nil, ""},
			{"clockify_expense_report", nil, ""},
			{"clockify_list_expense_categories", nil, ""},
		}},
		{"scheduling", []call{
			{"clockify_list_assignments", scheduleArgs, ""},
			// /scheduling (list_schedules) and /scheduling/capacity
			// (filter_schedule_capacity) still return 404. They were
			// not in the safe-batch scope (probe didn't surface the
			// correct paths); their pins stay as regression alarms.
			{"clockify_list_schedules", nil, "No static resource"},
			{"clockify_get_project_schedule_totals", scheduleArgs, ""},
			{"clockify_filter_schedule_capacity", scheduleArgs, "No static resource"},
		}},
		{"time_off", []call{
			{"clockify_list_time_off_requests", nil, ""},
			{"clockify_list_time_off_policies", nil, ""},
		}},
		{"approvals", []call{
			{"clockify_list_approval_requests", nil, ""},
		}},
		{"shared_reports", []call{
			{"clockify_list_shared_reports", nil, ""},
		}},
		{"user_admin", []call{
			{"clockify_list_user_groups", nil, ""},
		}},
		{"webhooks", []call{
			{"clockify_list_webhooks", nil, ""},
			{"clockify_list_webhook_events", nil, ""},
		}},
		{"custom_fields", []call{
			{"clockify_list_custom_fields", nil, ""},
		}},
		{"groups_holidays", []call{
			{"clockify_list_user_groups_admin", nil, ""},
			{"clockify_list_holidays", nil, ""},
		}},
		{"project_admin", []call{
			{"clockify_list_project_templates", nil, ""},
		}},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			c.activateTier2(g.name)
			for _, tc := range g.tools {
				t.Run(tc.tool, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					if tc.expectErr != "" {
						msg := h.callExpectError(ctx, tc.tool, tc.args)
						if !strings.Contains(msg, tc.expectErr) {
							t.Fatalf("%s: expected error containing %q, got %q", tc.tool, tc.expectErr, msg)
						}
						t.Logf("%s: known issue still present: %q", tc.tool, msg)
						return
					}
					result := h.callOK(ctx, tc.tool, tc.args)
					sc, ok := result["structuredContent"].(map[string]any)
					if !ok {
						t.Fatalf("%s response missing structuredContent: %#v", tc.tool, result)
					}
					if okFlag, _ := sc["ok"].(bool); !okFlag {
						t.Fatalf("%s response carried ok=false: %#v", tc.tool, sc)
					}
				})
			}
		})
	}
}
