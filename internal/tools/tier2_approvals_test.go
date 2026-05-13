package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSubmitForApprovalBodyUsesCamelCaseKeys(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "apr1", "status": "PENDING"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.submitForApproval(context.Background(), map[string]any{
		"period":       "WEEKLY",
		"period_start": "2026-05-04T00:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("submit for approval failed: %v", err)
	}
	if result.Action != "clockify_submit_for_approval" {
		t.Fatalf("expected submit action, got %q", result.Action)
	}
	if gotBody["period"] != "WEEKLY" {
		t.Fatalf("expected period WEEKLY, got %#v", gotBody)
	}
	if gotBody["periodStart"] != "2026-05-04T00:00:00.000Z" {
		t.Fatalf("expected camelCase periodStart, got %#v", gotBody)
	}
	if _, ok := gotBody["period_start"]; ok {
		t.Fatalf("body must not send snake_case period_start: %#v", gotBody)
	}
}

func TestSubmitForApprovalLegacyStartMapsToWeekly(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "apr1", "status": "PENDING"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.submitForApproval(context.Background(), map[string]any{
		"start": "2026-05-04T00:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("submit for approval legacy start failed: %v", err)
	}
	if result.Action != "clockify_submit_for_approval" {
		t.Fatalf("expected submit action, got %q", result.Action)
	}
	if gotBody["period"] != "WEEKLY" {
		t.Fatalf("expected legacy start fallback to WEEKLY, got %#v", gotBody)
	}
	if gotBody["periodStart"] != "2026-05-04T00:00:00.000Z" {
		t.Fatalf("expected legacy start to map to periodStart, got %#v", gotBody)
	}
}

func TestSubmitForApprovalDecodesNonMapResponse(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, []map[string]any{{"id": "apr1", "status": "PENDING"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.submitForApproval(context.Background(), map[string]any{
		"period":       "WEEKLY",
		"period_start": "2026-05-04T00:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("submit for approval failed: %v", err)
	}
	if result.Action != "clockify_submit_for_approval" {
		t.Fatalf("expected submit action, got %q", result.Action)
	}
	items, ok := result.Data.([]ApprovalView)
	if !ok {
		t.Fatalf("expected non-map response to decode as []ApprovalView, got %T", result.Data)
	}
	if len(items) != 1 {
		t.Fatalf("expected one decoded response item, got %#v", items)
	}
}

func TestApprovalViewNormalizesDurationsMoneyExpensesAndClients(t *testing.T) {
	view := approvalViewFromRaw(map[string]any{
		"id":             "apr1",
		"status":         "PENDING",
		"trackedTime":    "1:30",
		"billableTime":   "PT1H",
		"pendingTime":    float64(1800),
		"billableAmount": 12500,
		"costAmount":     4000,
		"expenseTotal":   1500,
		"currencyCode":   "USD",
		"owner":          map[string]any{"id": "u1", "name": "Owner"},
		"entries": []map[string]any{{
			"_id":        "e1",
			"duration":   3600,
			"clientId":   "c1",
			"clientName": "Client A",
			"amounts":    []map[string]any{{"type": "EARNED", "value": 12500, "currency": "USD"}},
		}},
		"expenses": []map[string]any{{"amount": 1500, "currencyCode": "USD", "billable": true}},
	})
	if view.DurationTotals.Tracked == nil || view.DurationTotals.Tracked.Seconds != 5400 {
		t.Fatalf("tracked duration not normalized: %#v", view.DurationTotals.Tracked)
	}
	if view.DurationTotals.Billable == nil || view.DurationTotals.Billable.Seconds != 3600 {
		t.Fatalf("billable duration not normalized: %#v", view.DurationTotals.Billable)
	}
	if view.MoneyTotals.Source != "approval_api" || view.MoneyTotals.Earned.AmountCents != 12500 || view.MoneyTotals.Cost.AmountCents != 4000 {
		t.Fatalf("money totals not normalized: %#v", view.MoneyTotals)
	}
	if view.MoneyTotals.Profit == nil || view.MoneyTotals.Profit.AmountCents != 8500 {
		t.Fatalf("profit not derived: %#v", view.MoneyTotals.Profit)
	}
	if view.ExpenseSummary.Count != 1 || view.ExpenseSummary.BillableCount != 1 || view.ExpenseSummary.Amount.AmountCents != 1500 {
		t.Fatalf("expense summary not normalized: %#v", view.ExpenseSummary)
	}
	if view.EntrySummary.Count != 1 || len(view.ClientSummary) != 1 || view.ClientSummary[0].Client.ID != "c1" {
		t.Fatalf("entry/client summaries not normalized: entries=%#v clients=%#v", view.EntrySummary, view.ClientSummary)
	}
	if len(view.SuggestedActions) == 0 || view.SuggestedActions[0].Tool != "clockify_approve_timesheet" {
		t.Fatalf("suggested actions not populated: %#v", view.SuggestedActions)
	}
}

func TestListApprovalRequestsForwardsDocumentedSort(t *testing.T) {
	var gotQuery string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		respondJSON(t, w, []map[string]any{{"id": "apr1", "status": "PENDING"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.listApprovalRequests(context.Background(), map[string]any{
		"status":      "PENDING",
		"sort_column": "UPDATED_AT",
		"sort_order":  "DESCENDING",
	})
	if err != nil {
		t.Fatalf("list approval requests: %v", err)
	}
	if !strings.Contains(gotQuery, "sort-column=UPDATED_AT") || !strings.Contains(gotQuery, "sort-order=DESCENDING") {
		t.Fatalf("sort query not forwarded: %s", gotQuery)
	}
	items := result.Data.([]ApprovalView)
	if len(items) != 1 || items[0].ID != "apr1" {
		t.Fatalf("approval list not decoded: %#v", items)
	}
}

func TestApprovalListSchemaDocumentsSort(t *testing.T) {
	svc := New(nil, "ws1")
	handlers, ok := svc.Tier2Handlers("approvals")
	if !ok {
		t.Fatal("approvals group not registered")
	}
	for _, descriptor := range handlers {
		if descriptor.Tool.Name != "clockify_list_approval_requests" {
			continue
		}
		props := descriptor.Tool.InputSchema["properties"].(map[string]any)
		if _, ok := props["sort_column"]; !ok {
			t.Fatalf("sort_column missing from schema: %#v", props)
		}
		if _, ok := props["sort_order"]; !ok {
			t.Fatalf("sort_order missing from schema: %#v", props)
		}
		return
	}
	t.Fatal("clockify_list_approval_requests descriptor not found")
}
