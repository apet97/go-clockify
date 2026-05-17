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

func TestSubmitForApprovalDryRunDoesNotPost(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("approval dry-run must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.submitForApproval(context.Background(), map[string]any{
		"period":       "WEEKLY",
		"period_start": "2026-05-04T00:00:00.000Z",
		"dry_run":      true,
	})
	if err != nil {
		t.Fatalf("submit dry-run failed: %v", err)
	}
	if result.Action != "clockify_submit_for_approval" {
		t.Fatalf("expected submit action, got %q", result.Action)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["dry_run"] != true {
		t.Fatalf("expected dry-run data, got %#v", result.Data)
	}
	payload, ok := data["payload"].(map[string]any)
	if !ok || payload["periodStart"] != "2026-05-04T00:00:00.000Z" {
		t.Fatalf("expected camelCase dry-run payload, got %#v", data["payload"])
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
	if len(view.SuggestedActions) == 0 || view.SuggestedActions[0].Tool != "clockify_approvals_approve" {
		t.Fatalf("suggested actions not populated: %#v", view.SuggestedActions)
	}
}

// TestGetApprovalRequestStripsHeavyEntries proves clockify_approvals_get drops
// the embedded time-entry array (the ~656 KB bloat) and reports the omitted
// count in meta, mirroring clockify_approvals_list.
func TestGetApprovalRequestStripsHeavyEntries(t *testing.T) {
	heavyEntries := make([]map[string]any, 50)
	for i := range heavyEntries {
		heavyEntries[i] = map[string]any{"id": "e", "duration": 3600}
	}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, []map[string]any{{
			"id":      "apr1",
			"status":  "PENDING",
			"entries": heavyEntries,
		}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.getApprovalRequest(context.Background(), map[string]any{"approval_id": "apr1"})
	mustOK(t, res, err, "clockify_get_approval_request")
	view, ok := res.Data.(ApprovalView)
	if !ok {
		t.Fatalf("data type = %T, want ApprovalView", res.Data)
	}
	if view.EntrySummary.Count != 0 {
		t.Fatalf("entries should be stripped, got EntrySummary.Count = %d", view.EntrySummary.Count)
	}
	if got, _ := res.Meta["entriesOmitted"].(int); got != 50 {
		t.Fatalf("meta.entriesOmitted = %v, want 50", res.Meta["entriesOmitted"])
	}
}

func TestApproveTimesheetDryRunStripsHeavyEntries(t *testing.T) {
	heavyEntries := make([]map[string]any, 75)
	for i := range heavyEntries {
		heavyEntries[i] = map[string]any{"id": "e", "duration": 3600}
	}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, []map[string]any{{
			"id":      "apr1",
			"status":  "PENDING",
			"entries": heavyEntries,
		}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.approveTimesheet(context.Background(), map[string]any{"approval_id": "apr1", "dry_run": true})
	mustOK(t, res, err, "clockify_approve_timesheet")
	if got, _ := res.Meta["entriesOmitted"].(int); got != 75 {
		t.Fatalf("meta.entriesOmitted = %v, want 75", res.Meta["entriesOmitted"])
	}
	data := res.Data.(map[string]any)
	preview := data["preview"].(ApprovalView)
	if preview.EntrySummary.Count != 0 {
		t.Fatalf("dry-run preview should strip entries, got EntrySummary.Count = %d", preview.EntrySummary.Count)
	}
}

func TestFindApprovalRequestScansCanonicalStatusesWhenUnfiltered(t *testing.T) {
	var statuses []string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		status := r.URL.Query().Get("status")
		statuses = append(statuses, status)
		if status == "WITHDRAWN_APPROVAL" {
			respondJSON(t, w, []map[string]any{{"id": "apr-withdrawn", "status": "WITHDRAWN_APPROVAL"}})
			return
		}
		respondJSON(t, w, []map[string]any{})
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.getApprovalRequest(context.Background(), map[string]any{"approval_id": "apr-withdrawn"})
	mustOK(t, res, err, "clockify_get_approval_request")
	if view := res.Data.(ApprovalView); view.ID != "apr-withdrawn" {
		t.Fatalf("approval ID = %q, want apr-withdrawn", view.ID)
	}
	want := []string{"PENDING", "APPROVED", "WITHDRAWN_APPROVAL"}
	if len(statuses) != len(want) {
		t.Fatalf("statuses = %#v, want %#v", statuses, want)
	}
	for i := range want {
		if statuses[i] != want[i] {
			t.Fatalf("statuses = %#v, want %#v", statuses, want)
		}
	}
}

func TestListApprovalRequestsForwardsDocumentedSort(t *testing.T) {
	var gotQuery string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		respondJSON(t, w, []map[string]any{{
			"id":      "apr1",
			"status":  "PENDING",
			"entries": []map[string]any{{"id": "entry-heavy", "duration": 3600}},
		}})
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
	if items[0].EntrySummary.Count != 0 {
		t.Fatalf("approval list should strip heavy entries before view normalization, got %#v", items[0].EntrySummary)
	}
}

func TestApprovalListSchemaDocumentsSort(t *testing.T) {
	svc := New(nil, "ws1")
	handlers, ok := tier2Handlers(svc, "approvals")
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

func TestApprovalListSchemaDocumentsCanonicalFilterStates(t *testing.T) {
	svc := New(nil, "ws1")
	handlers, ok := tier2Handlers(svc, "approvals")
	if !ok {
		t.Fatal("approvals group not registered")
	}
	for _, descriptor := range handlers {
		if descriptor.Tool.Name != "clockify_list_approval_requests" {
			continue
		}
		props := descriptor.Tool.InputSchema["properties"].(map[string]any)
		status := props["status"].(map[string]any)
		enum := status["enum"].([]string)
		// The list-filter enum is narrower than possible request states:
		// REJECTED reverts to UNSUBMITTED and WITHDRAWN_SUBMISSION is not a
		// valid filter value on Clockify's approval-requests API.
		want := []string{"PENDING", "APPROVED", "WITHDRAWN_APPROVAL"}
		if len(enum) != len(want) {
			t.Fatalf("status enum length = %d, want %d: %#v", len(enum), len(want), enum)
		}
		for i := range want {
			if enum[i] != want[i] {
				t.Fatalf("status enum = %#v, want %#v", enum, want)
			}
		}
		return
	}
	t.Fatal("clockify_list_approval_requests descriptor not found")
}

// TestResubmitApprovalPreflightGuidesIneligibleStates proves the resubmit
// state preflight blocks an already-APPROVED approval up front and turns a
// failed resubmit of a REJECTED approval into an actionable recovery hint.
func TestResubmitApprovalPreflightGuidesIneligibleStates(t *testing.T) {
	const approvalID = "000000000000000000000001"

	t.Run("approved is blocked before any POST", func(t *testing.T) {
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/approval-requests" {
				respondJSON(t, w, []map[string]any{{"id": approvalID, "status": "APPROVED"}})
				return
			}
			t.Fatalf("resubmit must not POST for an APPROVED approval: %s %s", r.Method, r.URL.Path)
		})
		defer cleanup()
		svc := New(client, "ws1")
		_, err := svc.resubmitApprovalOneUser(context.Background(), map[string]any{
			"approval_id": approvalID,
			"entry_ids":   []any{"000000000000000000000002"},
		})
		if err == nil || !strings.Contains(err.Error(), "already APPROVED") {
			t.Fatalf("err = %v, want 'already APPROVED'", err)
		}
	})

	t.Run("rejected failure yields submit-fresh recovery hint", func(t *testing.T) {
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/approval-requests":
				respondJSON(t, w, []map[string]any{{"id": approvalID, "status": "REJECTED"}})
			case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/approval-requests/resubmit-entries-for-approval":
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"no active approval request for period"}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		})
		defer cleanup()
		svc := New(client, "ws1")
		_, err := svc.resubmitApprovalOneUser(context.Background(), map[string]any{
			"approval_id": approvalID,
			"entry_ids":   []any{"000000000000000000000002"},
		})
		if err == nil || !strings.Contains(err.Error(), "REJECTED") || !strings.Contains(err.Error(), "clockify_approvals_submit") {
			t.Fatalf("err = %v, want a REJECTED + clockify_approvals_submit recovery hint", err)
		}
	})
}

func TestResubmitApprovalRequiresOnlyDocumentedRequiredFieldsAndMapsPeriodStart(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/approval-requests":
			// resubmit preflight; PENDING keeps it on the happy path
			respondJSON(t, w, []map[string]any{{"id": "000000000000000000000001", "status": "PENDING"}})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/approval-requests/resubmit-entries-for-approval":
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{"id": gotBody["approvalId"], "status": "PENDING"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.resubmitApprovalOneUser(context.Background(), map[string]any{
		"approval_id":  "000000000000000000000001",
		"entry_ids":    []any{"000000000000000000000002"},
		"period":       "WEEKLY",
		"period_start": "2026-05-04T00:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("resubmit approval: %v", err)
	}
	if gotBody["periodStart"] != "2026-05-04T00:00:00.000Z" {
		t.Fatalf("period_start must be sent as periodStart, got %#v", gotBody)
	}
	if _, ok := gotBody["period_start"]; ok {
		t.Fatalf("body must not send snake_case period_start: %#v", gotBody)
	}
	if _, ok := gotBody["expenseIds"]; ok {
		t.Fatalf("body should omit optional expenseIds when not provided: %#v", gotBody)
	}
	if _, ok := gotBody["note"]; ok {
		t.Fatalf("body should omit optional note when not provided: %#v", gotBody)
	}
}
