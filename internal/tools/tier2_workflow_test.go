package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// TestApprovalHandlersCount verifies that the approvals group produces exactly 6 tools.
func TestApprovalHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descriptors, ok := svc.Tier2Handlers("approvals")
	if !ok {
		t.Fatal("approvals group not found in Tier2Groups")
	}
	if len(descriptors) != 6 {
		t.Fatalf("expected 6 approval tools, got %d", len(descriptors))
	}

	expected := map[string]bool{
		"clockify_list_approval_requests": true,
		"clockify_get_approval_request":   true,
		"clockify_submit_for_approval":    true,
		"clockify_approve_timesheet":      true,
		"clockify_reject_timesheet":       true,
		"clockify_withdraw_approval":      true,
	}
	for _, d := range descriptors {
		if !expected[d.Tool.Name] {
			t.Fatalf("unexpected tool %s in approvals group", d.Tool.Name)
		}
		delete(expected, d.Tool.Name)
	}
	if len(expected) > 0 {
		t.Fatalf("missing tools in approvals group: %v", expected)
	}
}

// TestSharedReportHandlersCount verifies that the shared_reports group produces exactly 6 tools.
func TestSharedReportHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descriptors, ok := svc.Tier2Handlers("shared_reports")
	if !ok {
		t.Fatal("shared_reports group not found in Tier2Groups")
	}
	if len(descriptors) != 6 {
		t.Fatalf("expected 6 shared report tools, got %d", len(descriptors))
	}

	expected := map[string]bool{
		"clockify_list_shared_reports":  true,
		"clockify_get_shared_report":    true,
		"clockify_create_shared_report": true,
		"clockify_update_shared_report": true,
		"clockify_delete_shared_report": true,
		"clockify_export_shared_report": true,
	}
	for _, d := range descriptors {
		if !expected[d.Tool.Name] {
			t.Fatalf("unexpected tool %s in shared_reports group", d.Tool.Name)
		}
		delete(expected, d.Tool.Name)
	}
	if len(expected) > 0 {
		t.Fatalf("missing tools in shared_reports group: %v", expected)
	}
}

// TestListApprovalRequests verifies the mock GET for listing approval requests.
func TestListApprovalRequests(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/approval-requests" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{
				{"id": "ar1", "status": "PENDING", "start": "2026-04-01", "end": "2026-04-07"},
				{"id": "ar2", "status": "APPROVED", "start": "2026-03-25", "end": "2026-03-31"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.listApprovalRequests(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("list approval requests failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	items, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 approval requests, got %d", len(items))
	}
	if items[0]["id"] != "ar1" {
		t.Fatalf("unexpected first approval ID: %v", items[0]["id"])
	}
}

// TestCreateSharedReport verifies the mock POST for creating a shared report.
func TestCreateSharedReport(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/shared-reports" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "Weekly Team Report" {
				t.Fatalf("expected name 'Weekly Team Report', got %v", body["name"])
			}
			if body["reportType"] != "SUMMARY" {
				t.Fatalf("expected reportType 'SUMMARY', got %v", body["reportType"])
			}
			respondJSON(t, w, map[string]any{
				"id":         "sr1",
				"name":       "Weekly Team Report",
				"reportType": "SUMMARY",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.createSharedReport(context.Background(), map[string]any{
		"name":        "Weekly Team Report",
		"report_type": "SUMMARY",
	})
	if err != nil {
		t.Fatalf("create shared report failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["id"] != "sr1" {
		t.Fatalf("expected report ID sr1, got %v", data["id"])
	}
}

// TestDeleteSharedReportDryRun verifies that dry_run=true does NOT call DELETE.
func TestDeleteSharedReportDryRun(t *testing.T) {
	var deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/shared-reports/sr1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":         "sr1",
				"name":       "Weekly Team Report",
				"reportType": "SUMMARY",
			})
		case r.URL.Path == "/workspaces/ws1/shared-reports/sr1" && r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.deleteSharedReport(context.Background(), map[string]any{
		"report_id": "sr1",
		"dry_run":   true,
	})
	if err != nil {
		t.Fatalf("delete shared report dry run failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	if deleteCalled {
		t.Fatal("DELETE should NOT be called during dry run")
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data for dry run, got %T", result.Data)
	}
	if dataMap["dry_run"] != true {
		t.Fatal("expected dry_run=true in result data")
	}
	if dataMap["note"] == nil {
		t.Fatal("expected note in dry run result")
	}
}

func TestWorkflowTier2RejectsMalformedIDs(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected for malformed ID input")
	})
	defer cleanup()

	svc := New(client, "ws1")
	ctx := context.Background()

	cases := []struct {
		name string
		fn   func() error
	}{
		{
			name: "approval request",
			fn: func() error {
				_, err := svc.getApprovalRequest(ctx, map[string]any{"approval_id": "bad/id"})
				return err
			},
		},
		{
			name: "approve timesheet",
			fn: func() error {
				_, err := svc.approveTimesheet(ctx, map[string]any{"approval_id": "bad/id"})
				return err
			},
		},
		{
			name: "shared report",
			fn: func() error {
				_, err := svc.getSharedReport(ctx, map[string]any{"report_id": "bad/id"})
				return err
			},
		},
		{
			name: "shared report export",
			fn: func() error {
				_, err := svc.exportSharedReport(ctx, map[string]any{"report_id": "bad/id"})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
