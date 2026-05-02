package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestSchedulingHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs := schedulingHandlers(svc)
	if len(descs) != 9 {
		t.Fatalf("expected 9 scheduling tools, got %d", len(descs))
	}

	names := make(map[string]bool, len(descs))
	for _, d := range descs {
		names[d.Tool.Name] = true
	}

	want := []string{
		"clockify_list_assignments",
		"clockify_get_assignment",
		"clockify_create_assignment",
		"clockify_update_assignment",
		"clockify_delete_assignment",
		"clockify_get_schedule",
		"clockify_create_schedule",
		"clockify_get_project_schedule_totals",
		"clockify_filter_schedule_capacity",
	}
	for _, name := range want {
		if !names[name] {
			t.Errorf("missing scheduling tool: %s", name)
		}
	}
}

func TestTimeOffHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs := timeOffHandlers(svc)
	if len(descs) != 12 {
		t.Fatalf("expected 12 time-off tools, got %d", len(descs))
	}

	names := make(map[string]bool, len(descs))
	for _, d := range descs {
		names[d.Tool.Name] = true
	}

	want := []string{
		"clockify_list_time_off_requests",
		"clockify_get_time_off_request",
		"clockify_create_time_off_request",
		"clockify_update_time_off_request",
		"clockify_delete_time_off_request",
		"clockify_approve_time_off",
		"clockify_deny_time_off",
		"clockify_list_time_off_policies",
		"clockify_get_time_off_policy",
		"clockify_create_time_off_policy",
		"clockify_update_time_off_policy",
		"clockify_time_off_balance",
	}
	for _, name := range want {
		if !names[name] {
			t.Errorf("missing time-off tool: %s", name)
		}
	}
}

func TestListAssignments(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/all" && r.Method == http.MethodGet:
			if got := r.URL.Query().Get("start"); got == "" {
				t.Fatalf("expected start query param, got empty")
			}
			if got := r.URL.Query().Get("end"); got == "" {
				t.Fatalf("expected end query param, got empty")
			}
			respondJSON(t, w, []map[string]any{
				{"id": "a1", "userId": "u1", "projectId": "p1", "start": "2026-04-01", "end": "2026-04-15"},
				{"id": "a2", "userId": "u2", "projectId": "p1", "start": "2026-04-01", "end": "2026-04-30"},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.listAssignments(context.Background(), map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-30T23:59:59Z",
	})
	if err != nil {
		t.Fatalf("list assignments failed: %v", err)
	}
	if result.Action != "clockify_list_assignments" {
		t.Fatalf("expected action clockify_list_assignments, got %s", result.Action)
	}
	items, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(items))
	}
	count, _ := result.Meta["count"].(int)
	if count != 2 {
		t.Fatalf("expected meta count=2, got %d", count)
	}
}

func TestCreateTimeOffRequest(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/time-off/policies/pol1/requests" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"id":       "req1",
				"policyId": "pol1",
				"start":    gotBody["start"],
				"end":      gotBody["end"],
				"status":   "PENDING",
				"note":     gotBody["note"],
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.createTimeOffRequest(context.Background(), map[string]any{
		"policy_id": "pol1",
		"start":     "2026-05-01",
		"end":       "2026-05-05",
		"note":      "Family vacation",
	})
	if err != nil {
		t.Fatalf("create time off request failed: %v", err)
	}
	if result.Action != "clockify_create_time_off_request" {
		t.Fatalf("expected action clockify_create_time_off_request, got %s", result.Action)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["id"] != "req1" {
		t.Fatalf("expected request id req1, got %v", data["id"])
	}
	if data["status"] != "PENDING" {
		t.Fatalf("expected status PENDING, got %v", data["status"])
	}
	// Verify POST body
	if gotBody["start"] != "2026-05-01" {
		t.Fatalf("expected start 2026-05-01 in body, got %v", gotBody["start"])
	}
	if gotBody["note"] != "Family vacation" {
		t.Fatalf("expected note in body, got %v", gotBody["note"])
	}
}

func TestDeleteAssignmentDryRun(t *testing.T) {
	var deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/abc123def456789012345678" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":        "abc123def456789012345678",
				"userId":    "u1",
				"projectId": "p1",
				"start":     "2026-04-01",
				"end":       "2026-04-15",
			})
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/abc123def456789012345678" && r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.deleteAssignment(context.Background(), map[string]any{
		"assignment_id": "abc123def456789012345678",
		"dry_run":       true,
	})
	if err != nil {
		t.Fatalf("delete assignment dry run failed: %v", err)
	}
	if result.Action != "clockify_delete_assignment" {
		t.Fatalf("expected action clockify_delete_assignment, got %s", result.Action)
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

// TestSchedulingGroupRegistered verifies the init() registered the group.
func TestSchedulingGroupRegistered(t *testing.T) {
	g, ok := Tier2Groups["scheduling"]
	if !ok {
		t.Fatal("scheduling group not registered in Tier2Groups")
	}
	if g.Description == "" {
		t.Fatal("scheduling group has empty description")
	}
	if len(g.Keywords) == 0 {
		t.Fatal("scheduling group has no keywords")
	}
}

// TestTimeOffGroupRegistered verifies the init() registered the group.
func TestTimeOffGroupRegistered(t *testing.T) {
	g, ok := Tier2Groups["time_off"]
	if !ok {
		t.Fatal("time_off group not registered in Tier2Groups")
	}
	if g.Description == "" {
		t.Fatal("time_off group has empty description")
	}
	if len(g.Keywords) == 0 {
		t.Fatal("time_off group has no keywords")
	}
}
