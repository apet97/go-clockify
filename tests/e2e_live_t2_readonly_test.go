//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestLiveOneUserOptionalDomainReadOnlySweep exercises read-only optional
// domain tools through the one-user startup registry. Each tool is already
// loaded at initialize time; this sweep exists to keep live optional coverage
// aligned with the current tool names.
func TestLiveOneUserOptionalDomainReadOnlySweep(t *testing.T) {
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
	userTotalsArgs := map[string]any{
		"start":   scheduleArgs["start"],
		"end":     scheduleArgs["end"],
		"user_id": c.OwnerUserID,
	}
	capacityArgs := map[string]any{
		"start":    scheduleArgs["start"],
		"end":      scheduleArgs["end"],
		"user_ids": []any{c.OwnerUserID},
	}
	expenseReportArgs := map[string]any{
		"start":       start.Format("2006-01-02T15:04:05.000"),
		"end":         end.Format("2006-01-02T15:04:05.000"),
		"page":        1,
		"page_size":   10,
		"sort_column": "ID",
		"sort_order":  "ASCENDING",
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
			{"clockify_invoices_list", nil, ""},
		}},
		{"expenses", []call{
			{"clockify_expenses_list", nil, ""},
			{"clockify_reports_expense", expenseReportArgs, ""},
			{"clockify_expenses_categories_list", nil, ""},
		}},
		{"scheduling", []call{
			{"clockify_scheduling_assignments_list", scheduleArgs, ""},
			{"clockify_scheduling_project_totals", scheduleArgs, ""},
			{"clockify_scheduling_user_totals", userTotalsArgs, ""},
			{"clockify_scheduling_capacity", capacityArgs, ""},
		}},
		{"time_off", []call{
			{"clockify_time_off_requests_list", nil, ""},
			{"clockify_time_off_policies_list", nil, ""},
		}},
		{"approvals", []call{
			{"clockify_approvals_list", nil, ""},
		}},
		{"user_admin", []call{
			{"clockify_groups_list", nil, ""},
			{"clockify_users_profile", nil, ""},
		}},
		{"webhooks", []call{
			{"clockify_webhooks_list", nil, ""},
			{"clockify_webhooks_events", nil, ""},
		}},
		{"custom_fields", []call{
			{"clockify_custom_fields_list", nil, ""},
		}},
		{"groups_holidays", []call{
			{"clockify_groups_list", nil, ""},
			{"clockify_holidays_list", nil, ""},
		}},
		{"project_admin", []call{
			{"clockify_projects_templates_list", nil, ""},
		}},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
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
					sc := structuredContentMap(t, result)
					if okFlag, _ := sc["ok"].(bool); !okFlag {
						t.Fatalf("%s response carried ok=false: %#v", tc.tool, sc)
					}
				})
			}
		})
	}
}
