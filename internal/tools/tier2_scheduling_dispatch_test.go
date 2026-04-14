package tools_test

// Dispatcher-level coverage for the Tier 2 scheduling group: assignment
// CRUD (with delete dry_run), schedule CRUD-lite, and the project / capacity
// read endpoints. Each handler is exercised through the real MCP dispatch
// pipeline via dispatchTier2 (no direct svc.* calls).
//
// The fake upstream serves the assignment, schedule, capacity, and totals
// endpoints, plus the workspace-level users + projects collections that
// resolve.ResolveUserID / resolve.ResolveProjectID hit when the create path
// runs. Without those resolution endpoints the create handler errors before
// it issues its POST, so the resolve helpers are part of the surface this
// test file deliberately covers.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/testharness"
)

func newSchedulingUpstream(t *testing.T) *testharness.FakeClockify {
	t.Helper()
	mux := http.NewServeMux()

	// Workspace user list — used by resolve.ResolveUserID when the create
	// handler is given a non-ID user reference.
	mux.HandleFunc("/workspaces/test-workspace/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"aaaaaaaaaaaaaaaaaaaaaaa1","name":"Alice","email":"alice@example.com"}]`))
	})

	// Project list — used by resolve.ResolveProjectID similarly.
	mux.HandleFunc("/workspaces/test-workspace/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"bbbbbbbbbbbbbbbbbbbbbbb1","name":"Active project","archived":false}]`))
	})

	// Assignments collection — list + create.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`[{"id":"a-1","userId":"aaaaaaaaaaaaaaaaaaaaaaa1","projectId":"bbbbbbbbbbbbbbbbbbbbbbb1"}]`))
		case http.MethodPost:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "a-new"
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Project totals.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/project-totals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalSeconds":36000,"projectId":"bbbbbbbbbbbbbbbbbbbbbbb1"}`))
	})

	// Per-assignment endpoint — get / put (update merge) / delete (live + dry-run preview).
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/a-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"a-1","userId":"aaaaaaaaaaaaaaaaaaaaaaa1","projectId":"bbbbbbbbbbbbbbbbbbbbbbb1","start":"2026-04-01","end":"2026-04-07"}`))
		case http.MethodPut:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "a-1"
			_ = json.NewEncoder(w).Encode(body)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Schedules collection — list + create.
	mux.HandleFunc("/workspaces/test-workspace/scheduling", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`[{"id":"s-1","name":"Q2"}]`))
		case http.MethodPost:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "s-new"
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Per-schedule endpoint — get only (no update/delete tools registered).
	mux.HandleFunc("/workspaces/test-workspace/scheduling/s-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"s-1","name":"Q2"}`))
	})

	// Capacity endpoint.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/capacity", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalHours":160,"users":[]}`))
	})

	return testharness.NewFakeClockify(t, mux)
}

func TestTier2Dispatch_Scheduling_AssignmentsListAndGet(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_list_assignments",
		Args:     map[string]any{"page": 1, "page_size": 25},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "a-1") {
		t.Fatalf("list result missing assignment id: %q", res.ResultText)
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_get_assignment",
		Args:     map[string]any{"assignment_id": "a-1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("get outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
}

func TestTier2Dispatch_Scheduling_CreateAssignment(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_create_assignment",
		Args: map[string]any{
			"user_id":       "aaaaaaaaaaaaaaaaaaaaaaa1",
			"project_id":    "bbbbbbbbbbbbbbbbbbbbbbb1",
			"start":         "2026-04-01T00:00:00Z",
			"end":           "2026-04-07T23:59:59Z",
			"hours_per_day": 8.0,
			"note":          "Sprint 14 capacity",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("create outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("create did not reach upstream")
	}
	if !strings.Contains(res.ResultText, "a-new") {
		t.Fatalf("create result missing new id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Scheduling_UpdateAssignment(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_update_assignment",
		Args: map[string]any{
			"assignment_id": "a-1",
			"start":         "2026-04-02T00:00:00Z",
			"end":           "2026-04-08T23:59:59Z",
			"hours_per_day": 6.0,
			"note":          "Reduced capacity",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("update outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, "a-1") {
		t.Fatalf("update result missing id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Scheduling_DeleteAssignmentDryRunAndLive(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	// Dry-run path: handler does a GET then returns a preview, no DELETE.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_delete_assignment",
		Args:     map[string]any{"assignment_id": "a-1", "dry_run": true},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("delete(dry_run) outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "a-1") {
		t.Fatalf("delete(dry_run) result missing id: %q", res.ResultText)
	}

	// Live path: handler DELETEs and returns {deleted:true,...}.
	res = dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_delete_assignment",
		Args:     map[string]any{"assignment_id": "a-1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("delete(live) outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "deleted") {
		t.Fatalf("delete(live) result missing deleted flag: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Scheduling_SchedulesListGetCreate(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_list_schedules",
		Args:     map[string]any{},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list_schedules outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "s-1") {
		t.Fatalf("list_schedules result missing id: %q", res.ResultText)
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_get_schedule",
		Args:     map[string]any{"schedule_id": "s-1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("get_schedule outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_create_schedule",
		Args: map[string]any{
			"name":          "Q3 plan",
			"start":         "2026-07-01T00:00:00Z",
			"end":           "2026-09-30T23:59:59Z",
			"hours_per_day": 8.0,
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("create_schedule outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, "s-new") {
		t.Fatalf("create_schedule result missing new id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Scheduling_ProjectScheduleTotalsAndCapacity(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_get_project_schedule_totals",
		Args:     map[string]any{"project_id": "bbbbbbbbbbbbbbbbbbbbbbb1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("totals outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "totalSeconds") {
		t.Fatalf("totals result missing field: %q", res.ResultText)
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_filter_schedule_capacity",
		Args: map[string]any{
			"start":   "2026-04-01T00:00:00Z",
			"end":     "2026-04-30T23:59:59Z",
			"user_id": "aaaaaaaaaaaaaaaaaaaaaaa1",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("capacity outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "totalHours") {
		t.Fatalf("capacity result missing field: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Scheduling_SchemaValidation(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	// Missing required schedule_id.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_get_schedule",
		Args:     map[string]any{},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeInvalidParams {
		t.Fatalf("expected invalid_params, got %q (err=%q)", res.Outcome, res.ErrorMessage)
	}
	if res.UpstreamHit {
		t.Fatalf("schema-rejected call must not reach upstream")
	}
}
