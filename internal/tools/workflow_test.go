package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// TestScheduleWorkRequiresUserUpfront proves clockify_schedule_work rejects a
// missing user with one clear error before any upstream call — the schema
// lists only start/end/hours_per_day as required, so the handler must own the
// user/project requirement.
func TestScheduleWorkRequiresUserUpfront(t *testing.T) {
	client, cleanup := newTestClient(t, func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected upstream call before validation: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()
	svc := New(client, "ws1")
	_, err := svc.ClockifyScheduleWork(context.Background(), map[string]any{
		"start": "2026-05-18", "end": "2026-05-22", "hours_per_day": 6.0,
	})
	if err == nil || !strings.Contains(err.Error(), "needs a user") {
		t.Fatalf("err = %v, want a 'needs a user' message", err)
	}
}

func TestScheduleWorkDryRunSkipsAssignmentCreate(t *testing.T) {
	client, cleanup := newTestClient(t, func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("schedule_work dry_run must not create upstream assignments: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	out, err := svc.ClockifyScheduleWork(context.Background(), map[string]any{
		"user_id":       "000000000000000000000001",
		"project_id":    "65b382b606de527a7ee2b60d",
		"start":         "2026-01-05T09:00:00Z",
		"end":           "2026-01-09T17:00:00Z",
		"hours_per_day": float64(6),
		"dry_run":       true,
	})
	if err != nil {
		t.Fatalf("ClockifyScheduleWork dry_run: %v", err)
	}
	result, ok := out.(ToolResult)
	if !ok {
		t.Fatalf("dry_run result type = %T, want ToolResult", out)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["dry_run"] != true {
		t.Fatalf("schedule_work dry_run data = %#v, want dry_run preview", result.Data)
	}
	payload, ok := data["payload"].(map[string]any)
	if !ok || payload["userId"] != "000000000000000000000001" || payload["projectId"] != "65b382b606de527a7ee2b60d" {
		t.Fatalf("schedule_work dry_run payload did not preserve resolved IDs: %#v", data["payload"])
	}
}

func TestScheduleWorkResolvesUserAndProjectNamesDryRun(t *testing.T) {
	const (
		userID    = "000000000000000000000001"
		projectID = "65b382b606de527a7ee2b60d"
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/users":
			respondJSON(t, w, []map[string]any{{"id": userID, "name": "Ada Lovelace", "email": "ada@example.test"}})
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/projects":
			respondJSON(t, w, []map[string]any{{"id": projectID, "name": "Engine Room"}})
		default:
			t.Fatalf("unexpected schedule_work name dry_run request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	out, err := svc.ClockifyScheduleWork(context.Background(), map[string]any{
		"user":          "Ada Lovelace",
		"project":       "Engine Room",
		"start":         "2026-01-05T09:00:00Z",
		"end":           "2026-01-09T17:00:00Z",
		"hours_per_day": float64(6),
		"dry_run":       true,
	})
	if err != nil {
		t.Fatalf("ClockifyScheduleWork name dry_run: %v", err)
	}
	result := out.(ToolResult)
	data := result.Data.(map[string]any)
	payload := data["payload"].(map[string]any)
	if payload["userId"] != userID || payload["projectId"] != projectID {
		t.Fatalf("schedule_work dry-run did not resolve names: %#v", payload)
	}
}

func TestSetupWebhookWorkflowAcceptsEventAliasDryRun(t *testing.T) {
	client, cleanup := newTestClient(t, func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("setup_webhook dry_run must not create upstream webhooks: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.WebhookValidateDNS = false
	out, err := svc.ClockifySetupWebhook(context.Background(), map[string]any{
		"name":    "Alias webhook",
		"url":     "https://example.com/clockify",
		"event":   "NEW_TIME_ENTRY",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("ClockifySetupWebhook event alias dry_run: %v", err)
	}
	result, ok := out.(ToolResult)
	if !ok {
		t.Fatalf("result type = %T, want ToolResult", out)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["dry_run"] != true {
		t.Fatalf("setup_webhook dry_run data = %#v", result.Data)
	}
	payload := data["payload"].(map[string]any)
	if payload["webhookEvent"] != "NEW_TIME_ENTRY" {
		t.Fatalf("event alias did not flow to webhookEvent: %#v", payload)
	}
}

// TestApprovalHandlersCount verifies that the approvals group produces the
// documented list/create/resubmit/PATCH approval surface.
func TestApprovalHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descriptors, ok := domainHandlers(svc, "approvals")
	if !ok {
		t.Fatal("approvals group not found")
	}
	if len(descriptors) != 9 {
		t.Fatalf("expected 9 approval tools, got %d", len(descriptors))
	}

	expected := map[string]bool{
		"clockify_list_approval_requests":     true,
		"clockify_get_approval_request":       true,
		"clockify_submit_for_approval":        true,
		"clockify_resubmit_for_approval":      true,
		"clockify_submit_for_user_approval":   true,
		"clockify_resubmit_for_user_approval": true,
		"clockify_approve_timesheet":          true,
		"clockify_reject_timesheet":           true,
		"clockify_withdraw_approval":          true,
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
	items, ok := result.Data.([]ApprovalView)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 approval requests, got %d", len(items))
	}
	if items[0].ID != "ar1" {
		t.Fatalf("unexpected first approval ID: %v", items[0].ID)
	}
	if items[0].Status["state"] != "PENDING" {
		t.Fatalf("unexpected first approval status: %#v", items[0].Status)
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
