package tools_test

// Dispatcher-level coverage for the Tier 2 scheduling group: recurring
// assignment CRUD (with delete dry_run) and the project / capacity read endpoints.
// Each handler is exercised through the real MCP dispatch pipeline via
// dispatchTier2 (no direct svc.* calls).
//
// The phantom clockify_get_schedule and clockify_create_schedule tools
// were removed alongside clockify_list_schedules once the probe lab
// confirmed the live API has no /scheduling/{id} or POST /scheduling
// surface (only /scheduling/assignments/... paths exist).
//
// The fake upstream serves the assignment, capacity, and totals
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

	// The bare assignments path is not a CRUD endpoint in production.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "wrong assignment CRUD path", http.StatusNotFound)
	})

	// Recurring assignment create lives at /assignments/recurring per
	// SCHEDULINGDOC.md. The endpoint returns an array of assignment rows.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/recurring", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["hoursPerDay"] == nil {
				t.Fatalf("create recurring assignment missing hoursPerDay: %#v", body)
			}
			body["id"] = "a-new"
			_ = json.NewEncoder(w).Encode([]map[string]any{body})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Assignments list — required start/end query params, hyphenated
	// page-size per SCHEDULINGDOC.md.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/all", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("start") == "" || r.URL.Query().Get("end") == "" {
			http.Error(w, `{"message":"missing range"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a-1","userId":"aaaaaaaaaaaaaaaaaaaaaaa1","projectId":"bbbbbbbbbbbbbbbbbbbbbbb1","taskId":"task-1","period":{"start":"2026-04-01T00:00:00Z","end":"2026-04-07T23:59:59Z"}}]`))
	})

	// Project totals — POST with JSON body, returns bare array per
	// SUMMARY rev 3 #18.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/projects/totals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["start"] == nil || body["end"] == nil {
			http.Error(w, `{"message":"missing range"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"projectId":"bbbbbbbbbbbbbbbbbbbbbbb1","projectName":"Active project","totalHours":36.0,"assignments":[]}]`))
	})

	// Single-project totals — GET with range query params.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/projects/totals/bbbbbbbbbbbbbbbbbbbbbbb1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("start") == "" || r.URL.Query().Get("end") == "" {
			http.Error(w, `{"message":"missing range"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projectId":"bbbbbbbbbbbbbbbbbbbbbbb1","projectName":"Active project","totalHours":36.0}`))
	})

	// Workspace user schedule totals — POST with filters, returns bare array.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/user-filter/totals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["start"] == nil || body["end"] == nil {
			http.Error(w, `{"message":"missing range"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"userId":"aaaaaaaaaaaaaaaaaaaaaaa1","userName":"Alice","totalHoursPerDay":[{"date":"2026-04-01T00:00:00Z","totalHours":7.0}]}]`))
	})

	mux.HandleFunc("/workspaces/test-workspace/reports/summary", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		filter, _ := body["summaryFilter"].(map[string]any)
		if filter["groups"] == nil {
			t.Fatalf("summary report missing grouping body: %#v", body)
		}
		if body["amountShown"] != "EARNED" {
			t.Fatalf("summary report amountShown = %#v, want EARNED", body["amountShown"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rows":[{"userId":"aaaaaaaaaaaaaaaaaaaaaaa1","projectId":"bbbbbbbbbbbbbbbbbbbbbbb1","taskId":"task-1","duration":43200,"amounts":[{"type":"EARNED","value":72000,"currency":"USD"},{"type":"COST","value":10800,"currency":"USD"},{"type":"PROFIT","value":61200,"currency":"USD"}]}]}`))
	})

	mux.HandleFunc("/workspaces/test-workspace/reports/detailed", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["detailedFilter"] == nil {
			t.Fatalf("detailed report missing filter body: %#v", body)
		}
		if body["amountShown"] != "EARNED" {
			t.Fatalf("detailed report amountShown = %#v, want EARNED", body["amountShown"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"timeEntries":[{"userId":"aaaaaaaaaaaaaaaaaaaaaaa1","projectId":"bbbbbbbbbbbbbbbbbbbbbbb1","taskId":"task-1","duration":43200,"amounts":[{"type":"EARNED","value":72000,"currency":"USD"},{"type":"COST","value":10800,"currency":"USD"},{"type":"PROFIT","value":61200,"currency":"USD"}]}]}`))
	})

	// Recurring per-assignment endpoint — PATCH update / DELETE.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/recurring/a-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPatch:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "a-1"
			_ = json.NewEncoder(w).Encode([]map[string]any{body})
		case http.MethodDelete:
			if got := r.URL.Query().Get("seriesUpdateOption"); got != "ALL" {
				t.Fatalf("delete missing seriesUpdateOption=ALL, got %q", got)
			}
			_, _ = w.Write([]byte(`[{"id":"a-1","deleted":true}]`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Per-user capacity endpoint. The probe lab proved the live shape
	// at /scheduling/assignments/users/{userId}/totals (flat object,
	// capacityPerDay in seconds, workingDays + totalHoursPerDay arrays).
	// The mux 400s when start/end are missing so the handler's required
	// range params are exercised.
	mux.HandleFunc("/workspaces/test-workspace/scheduling/assignments/users/aaaaaaaaaaaaaaaaaaaaaaa1/totals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("start") == "" || r.URL.Query().Get("end") == "" {
			http.Error(w, `{"message":"missing range"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"userId":"aaaaaaaaaaaaaaaaaaaaaaa1","userName":"Alice","capacityPerDay":25200.0,"workingDays":["MONDAY","TUESDAY","WEDNESDAY","THURSDAY","FRIDAY"],"totalHoursPerDay":[{"date":"2026-04-01T00:00:00Z","totalHours":7.0}]}`))
	})

	return testharness.NewFakeClockify(t, mux)
}

func TestTier2Dispatch_Scheduling_AssignmentsListAndGet(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_list_assignments",
		Args: map[string]any{
			"start":     "2026-04-01T00:00:00Z",
			"end":       "2026-04-07T23:59:59Z",
			"page":      1,
			"page_size": 25,
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "a-1") {
		t.Fatalf("list result missing assignment id: %q", res.ResultText)
	}
	// Pin SUMMARY rev 3 #4: the new /all suffix path must be hit.
	// The mux above 400s the no-suffix path on GET, so a regression
	// would surface as outcome != success here.
	if !strings.Contains(res.ResultText, "period") {
		t.Fatalf("list result missing period field (regression to old path?): %q", res.ResultText)
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
			"repeat":        true,
			"weeks":         1,
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
			"assignment_id":        "a-1",
			"start":                "2026-04-02T00:00:00Z",
			"end":                  "2026-04-08T23:59:59Z",
			"hours_per_day":        6.0,
			"note":                 "Reduced capacity",
			"series_update_option": "ALL",
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

	// Dry-run path: no single-assignment GET exists, so the handler
	// returns a minimal preview without reaching upstream.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_delete_assignment",
		Args:     map[string]any{"assignment_id": "a-1", "dry_run": true},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("delete(dry_run) outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if res.UpstreamHit {
		t.Fatalf("delete(dry_run) reached upstream")
	}
	if !strings.Contains(res.ResultText, "dry_run") {
		t.Fatalf("delete(dry_run) result missing dry_run marker: %q", res.ResultText)
	}

	// Live path: handler DELETEs and returns {deleted:true,...}.
	res = dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_delete_assignment",
		Args:     map[string]any{"assignment_id": "a-1", "series_update_option": "ALL"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("delete(live) outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "deleted") {
		t.Fatalf("delete(live) result missing deleted flag: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Scheduling_ProjectScheduleTotalsAndCapacity(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_get_project_schedule_totals",
		Args: map[string]any{
			"start":      "2026-04-01T00:00:00Z",
			"end":        "2026-04-30T23:59:59Z",
			"project_id": "bbbbbbbbbbbbbbbbbbbbbbb1",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("totals outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	// Pin SUMMARY rev 3 #18: the response shape changed from a wrapped
	// map ({totalSeconds, projectId}) to a bare array of project rows.
	// `totalHours` is the new per-row field; `projectName` is also
	// surfaced.
	if !strings.Contains(res.ResultText, "totalHours") {
		t.Fatalf("totals result missing totalHours: %q", res.ResultText)
	}
	if !strings.Contains(res.ResultText, "projectName") {
		t.Fatalf("totals result missing projectName: %q", res.ResultText)
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
	// Pin the per-user-totals shape: handler must hit
	// /scheduling/assignments/users/{userId}/totals and decode the flat
	// object the probe lab fixtures show. A regression to the old
	// /scheduling/capacity path will 404 against the mux above.
	for _, want := range []string{"capacityPerDay", "workingDays", "totalHoursPerDay"} {
		if !strings.Contains(res.ResultText, want) {
			t.Fatalf("capacity result missing %q: %q", want, res.ResultText)
		}
	}
	if !strings.Contains(res.ResultText, `"userId":"aaaaaaaaaaaaaaaaaaaaaaa1"`) {
		t.Fatalf("capacity result must echo resolved userId: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Scheduling_AssignmentReportAndTotalsWrappers(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_assignment_report",
		Args: map[string]any{
			"start":     "2026-04-01T00:00:00Z",
			"end":       "2026-04-30T23:59:59Z",
			"group_by":  []string{"USER", "PROJECT", "TASK"},
			"page_size": 25,
		},
		Upstream:              upstream,
		EntryFinancialReports: true,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("assignment report outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	for _, want := range []string{"amount_tracked", "cost_tracked", "realized_profit", "amount_difference", "cost_difference", "scheduled", "tracked"} {
		if !strings.Contains(res.ResultText, want) {
			t.Fatalf("assignment report result missing %q: %q", want, res.ResultText)
		}
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_get_workspace_schedule_user_totals",
		Args: map[string]any{
			"start": "2026-04-01T00:00:00Z",
			"end":   "2026-04-30T23:59:59Z",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("user totals outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, "totalHoursPerDay") {
		t.Fatalf("user totals result missing daily totals: %q", res.ResultText)
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group: "scheduling",
		Tool:  "clockify_get_single_project_schedule_totals",
		Args: map[string]any{
			"project_id": "bbbbbbbbbbbbbbbbbbbbbbb1",
			"start":      "2026-04-01T00:00:00Z",
			"end":        "2026-04-30T23:59:59Z",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("single project totals outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, "Active project") {
		t.Fatalf("single project totals result missing project: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Scheduling_SchemaValidation(t *testing.T) {
	upstream := newSchedulingUpstream(t)

	// Missing required assignment_id.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "scheduling",
		Tool:     "clockify_get_assignment",
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
