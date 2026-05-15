package tools

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// TestApprovalHandlersCount verifies that the approvals group produces the
// documented list/create/resubmit/PATCH approval surface.
func TestApprovalHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descriptors, ok := tier2Handlers(svc, "approvals")
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
