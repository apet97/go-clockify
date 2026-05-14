package tools

import (
	"context"
	"net/http"
	"testing"
)

func TestCustomFieldNormalizationFromEmbeddedRows(t *testing.T) {
	fields := customFieldValuesFromRaw([]any{
		map[string]any{"customFieldId": "cf1", "customFieldName": "Region", "customFieldType": "TXT", "value": "EU"},
	})
	if len(fields) != 1 {
		t.Fatalf("expected one normalized custom field, got %#v", fields)
	}
	if fields[0].CustomFieldID != "cf1" || fields[0].Name != "Region" || fields[0].Type != "TXT" || fields[0].Value != "EU" {
		t.Fatalf("unexpected normalized custom field: %#v", fields[0])
	}
}

func TestDetailedReportAddsRateBreakdownMoneyByCurrencyAndCustomFields(t *testing.T) {
	payload := map[string]any{
		"timeentries": []map[string]any{{
			"_id":               "te1",
			"description":       "Billable work",
			"rate":              10000,
			"amount":            5000,
			"earnedAmount":      5000,
			"earnedRate":        10000,
			"costAmount":        2000,
			"costRate":          4000,
			"currency":          "USD",
			"amountByCurrency":  []map[string]any{{"currency": "USD", "amount": 5000}},
			"customFieldValues": []map[string]any{{"customFieldId": "cf1", "name": "Region", "value": "EU"}},
		}},
		"totals": []map[string]any{{
			"totalTime":             1800,
			"totalAmountByCurrency": []map[string]any{{"currency": "USD", "amount": 5000}},
		}},
	}
	if got := appendDetailedReportViews(payload); got != 1 {
		t.Fatalf("appendDetailedReportViews count = %d, want 1", got)
	}
	entries := payload["entries"].([]ReportEntryView)
	if entries[0].RateBreakdown == nil || entries[0].RateBreakdown.CostAmount == nil {
		t.Fatalf("missing rate breakdown: %#v", entries[0].RateBreakdown)
	}
	if len(entries[0].MoneyByCurrency) != 1 || len(entries[0].CustomFields) != 1 {
		t.Fatalf("missing money/custom field enrichment: %#v", entries[0])
	}
	totals := payload["totals_summary"].(ReportTotalsSummary)
	if len(totals.MoneyByCurrency) != 1 {
		t.Fatalf("missing totals money_by_currency: %#v", totals)
	}
}

func TestApprovalViewAddsAuditTrailAndRollups(t *testing.T) {
	view := approvalViewFromRaw(map[string]any{
		"approvalRequest": map[string]any{
			"id": "ap1",
			"status": map[string]any{
				"state":             "APPROVED",
				"updatedBy":         "u2",
				"updatedByUserName": "Manager",
				"updatedAt":         "2026-05-13T10:00:00Z",
				"note":              "ok",
			},
			"creator": map[string]any{"userId": "u1", "userName": "Alice", "userEmail": "alice@example.test"},
		},
		"entries": []map[string]any{{
			"id":           "te1",
			"user":         map[string]any{"id": "u1", "name": "Alice"},
			"project":      map[string]any{"id": "p1", "name": "Project"},
			"task":         map[string]any{"id": "t1", "name": "Task"},
			"duration":     3600,
			"earnedAmount": 10000,
			"costAmount":   4000,
			"currency":     "USD",
		}},
	})
	if view.AuditTrail == nil || view.AuditTrail.UpdatedBy != "u2" || view.AuditTrail.Creator["email"] != "alice@example.test" {
		t.Fatalf("audit trail not normalized: %#v", view.AuditTrail)
	}
	if len(view.Rollups["users"]) != 1 || len(view.Rollups["projects"]) != 1 || len(view.Rollups["tasks"]) != 1 {
		t.Fatalf("rollups missing: %#v", view.Rollups)
	}
}

func TestInvoicePaymentsAndItemLinkage(t *testing.T) {
	item := invoiceItemViewFromRaw(map[string]any{
		"order":        2,
		"itemType":     "TIME_ENTRY_IMPORT",
		"importType":   "DETAILED",
		"amount":       12345,
		"currency":     "USD",
		"timeEntryIds": []any{"te1", "te2"},
		"expenseIds":   []any{"ex1"},
	}, "USD")
	block := item["item"].(map[string]any)
	if got := block["time_entry_ids"].([]string); len(got) != 2 || got[0] != "te1" {
		t.Fatalf("time entry ids not normalized: %#v", block)
	}
	payments := invoicePaymentViewsFromRaw([]map[string]any{{"id": "pay1", "amount": 5000, "currency": "USD", "author": map[string]any{"id": "u1", "name": "Alice"}}})
	summary := invoicePaymentSummary(payments, "USD")
	if summary["count"] != 1 || summary["amount"] == nil {
		t.Fatalf("payment summary missing: %#v", summary)
	}
}

func TestListInvoicePaymentsUsesDocumentedEndpoint(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/invoices/inv1/payments" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("page-size") != "50" {
			t.Fatalf("page-size = %s", r.URL.Query().Get("page-size"))
		}
		respondJSON(t, w, map[string]any{"payments": []map[string]any{{"id": "pay1", "amount": 5000, "currency": "USD"}}})
	})
	defer cleanup()
	svc := New(client, "ws1")
	result, err := svc.listInvoicePayments(context.Background(), map[string]any{"invoice_id": "inv1"})
	mustOK(t, result, err, "clockify_list_invoice_payments")
	if got := result.Data.([]InvoicePaymentView); len(got) != 1 {
		t.Fatalf("expected one payment, got %#v", result.Data)
	}
}

func TestListWebhookLogsUsesPostSearch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/webhooks/wh1/logs" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("size") != "50" {
			t.Fatalf("size = %s", r.URL.Query().Get("size"))
		}
		respondJSON(t, w, []map[string]any{{"id": "log1", "webhookId": "wh1", "statusCode": 500}})
	})
	defer cleanup()
	svc := New(client, "ws1")
	result, err := svc.ListWebhookLogs(context.Background(), map[string]any{"webhook_id": "wh1", "status": "FAILED"})
	mustOK(t, result, err, "clockify_list_webhook_logs")
	logs := result.Data.([]WebhookLogView)
	if logs[0]["delivery"].(map[string]any)["status"] != "FAILED" {
		t.Fatalf("delivery status not normalized: %#v", logs[0])
	}
}

func TestListInProgressTimeEntriesUsesWorkspaceEndpoint(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/time-entries/status/in-progress" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, []map[string]any{{"id": "te1", "timeInterval": map[string]any{"start": "2026-05-13T09:00:00Z"}}})
	})
	defer cleanup()
	svc := New(client, "ws1")
	result, err := svc.ListInProgressTimeEntries(context.Background(), map[string]any{})
	mustOK(t, result, err, "clockify_entries_running")
	if got := result.Data.([]EntryView); len(got) != 1 || got[0].ID != "te1" {
		t.Fatalf("unexpected entries: %#v", result.Data)
	}
}

func TestListUserManagersUsesDocumentedEndpoint(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/users/u1/managers" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, []map[string]any{{"id": "m1", "name": "Manager", "email": "m@example.test"}})
	})
	defer cleanup()
	svc := New(client, "ws1")
	result, err := svc.ListUserManagers(context.Background(), map[string]any{"user_id": "u1"})
	mustOK(t, result, err, "clockify_list_user_managers")
	if got := result.Data.([]map[string]any)[0]["manager"].(map[string]any)["email"]; got != "m@example.test" {
		t.Fatalf("manager not normalized: %#v", result.Data)
	}
}

func TestListHolidaysInPeriodUsesDocumentedEndpoint(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/holidays/in-period" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("assigned-to") != "u1" {
			t.Fatalf("assigned-to = %s", r.URL.Query().Get("assigned-to"))
		}
		respondJSON(t, w, []map[string]any{{"id": "h1", "name": "Holiday"}})
	})
	defer cleanup()
	svc := New(client, "ws1")
	result, err := svc.ListHolidaysInPeriod(context.Background(), map[string]any{"assigned_to": "u1", "start": "2026-05-01T00:00:00Z", "end": "2026-05-31T00:00:00Z"})
	mustOK(t, result, err, "clockify_list_holidays_in_period")
	if result.Meta["warning"] == "" {
		t.Fatalf("expected upstream caveat warning: %#v", result.Meta)
	}
}
