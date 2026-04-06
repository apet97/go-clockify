package tools

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestTimerStatusRunning(t *testing.T) {
	now := time.Now().UTC().Add(-2*time.Hour - 14*time.Minute)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			if r.URL.Query().Get("page-size") != "1" {
				t.Fatalf("expected page-size=1, got %s", r.URL.Query().Get("page-size"))
			}
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "Working", TimeInterval: clockify.TimeInterval{Start: now.Format(time.RFC3339)}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimerStatus(context.Background())
	if err != nil {
		t.Fatalf("timer status failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["running"] != true {
		t.Fatalf("expected running=true, got %v", data["running"])
	}
	if data["entry"] == nil {
		t.Fatal("expected entry to be non-nil")
	}
	elapsed, _ := data["elapsed"].(string)
	if elapsed == "" {
		t.Fatal("expected non-empty elapsed string")
	}
}

func TestTimerStatusNotRunning(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "Done", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T11:00:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimerStatus(context.Background())
	if err != nil {
		t.Fatalf("timer status failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["running"] != false {
		t.Fatalf("expected running=false, got %v", data["running"])
	}
	if data["entry"] != nil {
		t.Fatalf("expected entry to be nil, got %v", data["entry"])
	}
}

func TestSwitchProject(t *testing.T) {
	callCount := map[string]int{}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		callCount[key]++
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.Method == http.MethodPatch && r.URL.Path == "/workspaces/ws1/user/u1/time-entries":
			// Stop timer
			respondJSON(t, w, clockify.TimeEntry{ID: "stopped1", Description: "Old task", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T11:00:00Z"}})
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/projects":
			respondJSON(t, w, []map[string]any{
				{"id": "p2", "name": "New Project"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/time-entries":
			// Start timer
			respondJSON(t, w, clockify.TimeEntry{ID: "started1", Description: "Switched", ProjectID: "p2", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T11:00:00Z"}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.SwitchProject(context.Background(), map[string]any{
		"project":     "New Project",
		"description": "Switched",
	})
	if err != nil {
		t.Fatalf("switch project failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["stopped"] == nil {
		t.Fatal("expected stopped to be non-nil")
	}
	if data["started"] == nil {
		t.Fatal("expected started to be non-nil")
	}
}

func TestResolveDebugExactMatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/projects":
			respondJSON(t, w, []map[string]any{
				{"id": "p1", "name": "Alpha"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ResolveDebug(context.Background(), map[string]any{
		"entity_type": "project",
		"name_or_id":  "Alpha",
	})
	if err != nil {
		t.Fatalf("resolve debug failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["status"] != "exact_match" {
		t.Fatalf("expected exact_match, got %v", data["status"])
	}
	if data["resolved_id"] != "p1" {
		t.Fatalf("expected resolved_id=p1, got %v", data["resolved_id"])
	}
	if data["error"] != "" {
		t.Fatalf("expected empty error, got %v", data["error"])
	}
}

func TestSearchToolsByQuery(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	result, err := svc.SearchTools(context.Background(), map[string]any{
		"query": "timer",
	})
	if err != nil {
		t.Fatalf("search tools failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	count, _ := data["count"].(int)
	if count == 0 {
		t.Fatal("expected at least one result for 'timer' query")
	}
	byDomain, _ := data["by_domain"].(map[string][]interface{})
	_ = byDomain // grouped results present
}

func TestPolicyInfo(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")

	// Without PolicyDescribe set, should return "not available" message.
	result, err := svc.PolicyInfo(context.Background())
	if err != nil {
		t.Fatalf("policy info failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["message"] != "policy info not available" {
		t.Fatalf("expected 'policy info not available', got %v", data["message"])
	}

	// With PolicyDescribe set, should return the callback result.
	svc.PolicyDescribe = func() map[string]any {
		return map[string]any{
			"mode":         "standard",
			"denied_tools": []string{},
		}
	}
	result2, err := svc.PolicyInfo(context.Background())
	if err != nil {
		t.Fatalf("policy info with callback failed: %v", err)
	}
	data2, ok := result2.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result2.Data)
	}
	if data2["mode"] != "standard" {
		t.Fatalf("expected mode=standard, got %v", data2["mode"])
	}
}
