package tools_test

import (
	"net/http"
	"testing"

	"github.com/apet97/go-clockify/internal/testharness"
)

func TestTier2Dispatch_NewRequiredFields_RejectedAtSchemaGate(t *testing.T) {
	tests := []struct {
		name  string
		group string
		tool  string
		args  map[string]any
	}{
		{
			name:  "create_invoice_missing_number",
			group: "invoices",
			tool:  "clockify_create_invoice",
			args: map[string]any{
				"client_id":   "client1",
				"issued_date": "2026-04-01T00:00:00Z",
				"due_date":    "2026-05-01T00:00:00Z",
			},
		},
		{
			name:  "add_invoice_item_missing_item_type",
			group: "invoices",
			tool:  "clockify_add_invoice_item",
			args: map[string]any{
				"invoice_id":  "inv1",
				"description": "Consulting",
				"quantity":    8,
				"unit_price":  150,
			},
		},
		{
			name:  "update_invoice_item_missing_item_type",
			group: "invoices",
			tool:  "clockify_update_invoice_item",
			args: map[string]any{
				"invoice_id":  "inv1",
				"item_id":     "item1",
				"description": "Consulting",
				"quantity":    8,
				"unit_price":  150,
			},
		},
		{
			name:  "create_webhook_missing_name",
			group: "webhooks",
			tool:  "clockify_create_webhook",
			args: map[string]any{
				"url":           "https://example.com/hook",
				"webhook_event": "NEW_TIME_ENTRY",
			},
		},
		{
			name:  "create_webhook_missing_event",
			group: "webhooks",
			tool:  "clockify_create_webhook",
			args: map[string]any{
				"name": "live webhook",
				"url":  "https://example.com/hook",
			},
		},
		{
			name:  "create_time_off_request_missing_note",
			group: "time_off",
			tool:  "clockify_create_time_off_request",
			args: map[string]any{
				"policy_id": "pol1",
				"start":     "2026-05-01",
				"end":       "2026-05-05",
			},
		},
		{
			name:  "submit_for_approval_missing_period_start",
			group: "approvals",
			tool:  "clockify_submit_for_approval",
			args: map[string]any{
				"period": "WEEKLY",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := testharness.NewFakeClockify(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("schema-rejected call must not reach upstream; got %s %s", r.Method, r.URL.Path)
			}))
			res := dispatchTier2(t, tier2InvokeOpts{
				Group:    tt.group,
				Tool:     tt.tool,
				Args:     tt.args,
				Upstream: upstream,
			})
			if res.Outcome != testharness.OutcomeInvalidParams {
				t.Fatalf("expected invalid_params, got %q (err=%q)", res.Outcome, res.ErrorMessage)
			}
			if res.UpstreamHit {
				t.Fatal("schema-rejected call must not reach upstream")
			}
		})
	}
}
