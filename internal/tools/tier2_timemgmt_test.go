package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestSchedulingHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs := schedulingHandlers(svc)
	if len(descs) != 10 {
		t.Fatalf("expected 10 scheduling tools, got %d", len(descs))
	}

	names := make(map[string]bool, len(descs))
	for _, d := range descs {
		names[d.Tool.Name] = true
	}

	want := []string{
		"clockify_assignment_report",
		"clockify_list_assignments",
		"clockify_get_assignment",
		"clockify_create_assignment",
		"clockify_update_assignment",
		"clockify_delete_assignment",
		"clockify_get_project_schedule_totals",
		"clockify_get_single_project_schedule_totals",
		"clockify_get_workspace_schedule_user_totals",
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
	items, ok := result.Data.([]AssignmentView)
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

func TestSchedulingRangeArgsNormalizesFlexibleDatetimes(t *testing.T) {
	start, end, err := schedulingRangeArgs(map[string]any{
		"start": "2026-04-01 09:00",
		"end":   "2026-04-02T17:30",
	}, time.UTC)
	if err != nil {
		t.Fatalf("schedulingRangeArgs failed: %v", err)
	}
	if start != "2026-04-01T09:00:00Z" || end != "2026-04-02T17:30:00Z" {
		t.Fatalf("normalized range = %q..%q", start, end)
	}
}

// TestSchedulingRangeArgsHonoursCallerLocation pins the QA-agent-24
// fix: a naked timestamp like "2026-04-01 09:00" must be interpreted
// in the caller's location, not silently bucketed in UTC. Before the
// fix a user in America/Los_Angeles asking for a 9am assignment got
// the slot recorded as 9am UTC (2am local), which is a four-hour
// drift across all scheduling/time-off tools.
func TestSchedulingRangeArgsHonoursCallerLocation(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load LA timezone: %v", err)
	}
	start, end, err := schedulingRangeArgs(map[string]any{
		"start": "2026-04-01 09:00",
		"end":   "2026-04-01 17:00",
	}, la)
	if err != nil {
		t.Fatalf("schedulingRangeArgs failed: %v", err)
	}
	// PDT is UTC-7 on 2026-04-01 (DST in effect).
	if start != "2026-04-01T16:00:00Z" {
		t.Fatalf("start = %q, want 2026-04-01T16:00:00Z (9am PDT)", start)
	}
	if end != "2026-04-02T00:00:00Z" {
		t.Fatalf("end = %q, want 2026-04-02T00:00:00Z (5pm PDT)", end)
	}
}

func TestListAssignmentsNormalizesFlexibleRange(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/all" && r.Method == http.MethodGet:
			if got := r.URL.Query().Get("start"); got != "2026-04-01T09:00:00Z" {
				t.Fatalf("start query = %q, want normalized RFC3339", got)
			}
			if got := r.URL.Query().Get("end"); got != "2026-04-02T17:30:00Z" {
				t.Fatalf("end query = %q, want normalized RFC3339", got)
			}
			respondJSON(t, w, []map[string]any{{"id": "a1"}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.DefaultTimezone = time.UTC
	if _, err := svc.listAssignments(context.Background(), map[string]any{
		"start": "2026-04-01 09:00",
		"end":   "2026-04-02T17:30",
	}); err != nil {
		t.Fatalf("list assignments failed: %v", err)
	}
}

func TestCreateAssignmentNormalizesFlexibleRange(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/scheduling/assignments/recurring" && r.Method == http.MethodPost:
			body := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["start"] != "2026-04-01T09:00:00Z" || body["end"] != "2026-04-02T17:30:00Z" {
				t.Fatalf("body start/end not normalized: %#v", body)
			}
			body["id"] = "a1"
			respondJSON(t, w, []map[string]any{body})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.DefaultTimezone = time.UTC
	if _, err := svc.createAssignment(context.Background(), map[string]any{
		"user_id":       "aaaaaaaaaaaaaaaaaaaaaaaa",
		"project_id":    "bbbbbbbbbbbbbbbbbbbbbbbb",
		"start":         "2026-04-01 09:00",
		"end":           "2026-04-02T17:30",
		"hours_per_day": 8.0,
	}); err != nil {
		t.Fatalf("create assignment failed: %v", err)
	}
}

func TestGetAssignmentScansPastFirstPage(t *testing.T) {
	pages := []string{}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/scheduling/assignments/all" || r.Method != http.MethodGet {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("page-size"); got != "2" {
			t.Fatalf("expected page-size=2, got %s", got)
		}
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		switch page {
		case "1":
			respondJSON(t, w, []map[string]any{
				{"id": "a-1", "userId": "u1"},
				{"id": "a-2", "userId": "u2"},
			})
		case "2":
			respondJSON(t, w, []map[string]any{
				{"id": "a-3", "userId": "u3"},
			})
		default:
			t.Fatalf("unexpected page %q", page)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.getAssignment(context.Background(), map[string]any{
		"assignment_id": "a-3",
		"start":         "2026-04-01T00:00:00Z",
		"end":           "2026-04-30T23:59:59Z",
		"page_size":     2,
	})
	if err != nil {
		t.Fatalf("get assignment failed: %v", err)
	}
	if result.Action != "clockify_get_assignment" {
		t.Fatalf("expected action clockify_get_assignment, got %s", result.Action)
	}
	data, ok := result.Data.(AssignmentView)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["id"] != "a-3" {
		t.Fatalf("expected assignment a-3, got %#v", data)
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("expected pages [1 2], got %#v", pages)
	}
	if result.Meta["pagesFetched"] != 2 {
		t.Fatalf("expected pagesFetched=2, got %#v", result.Meta)
	}
	if result.Meta["entriesScanned"] != 3 {
		t.Fatalf("expected entriesScanned=3, got %#v", result.Meta)
	}
}

func TestGetAssignmentDefaultsScanRange(t *testing.T) {
	var gotStart string
	var gotEnd string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/scheduling/assignments/all" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotStart = r.URL.Query().Get("start")
		gotEnd = r.URL.Query().Get("end")
		if gotStart == "" || gotEnd == "" {
			t.Fatalf("expected default start/end query params, got %q %q", gotStart, gotEnd)
		}
		if _, err := time.Parse(time.RFC3339, gotStart); err != nil {
			t.Fatalf("default start is not RFC3339: %q (%v)", gotStart, err)
		}
		if _, err := time.Parse(time.RFC3339, gotEnd); err != nil {
			t.Fatalf("default end is not RFC3339: %q (%v)", gotEnd, err)
		}
		if r.URL.Query().Get("page-size") != "200" {
			t.Fatalf("expected default page-size=200, got %q", r.URL.Query().Get("page-size"))
		}
		respondJSON(t, w, []map[string]any{{"id": "a-1", "userId": "u1"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.getAssignment(context.Background(), map[string]any{
		"assignment_id": "a-1",
	})
	if err != nil {
		t.Fatalf("get assignment failed: %v", err)
	}
	if result.Meta["scanRangeDefaulted"] != true {
		t.Fatalf("expected scanRangeDefaulted=true, got %#v", result.Meta)
	}
	if result.Meta["scanStart"] != gotStart || result.Meta["scanEnd"] != gotEnd {
		t.Fatalf("scan range meta does not match query: meta=%#v query=%s..%s", result.Meta, gotStart, gotEnd)
	}
}

func TestGetAssignmentInputSchemaRequiresOnlyAssignmentID(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs := schedulingHandlers(svc)
	for _, d := range descs {
		if d.Tool.Name != "clockify_get_assignment" {
			continue
		}
		required, ok := d.Tool.InputSchema["required"].([]string)
		if !ok {
			t.Fatalf("unexpected required type: %T", d.Tool.InputSchema["required"])
		}
		if len(required) != 1 || required[0] != "assignment_id" {
			t.Fatalf("expected only assignment_id required, got %#v", required)
		}
		return
	}
	t.Fatal("clockify_get_assignment descriptor not found")
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
				"period":   gotBody["timeOffPeriod"],
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
	data, ok := result.Data.(TimeOffRequestView)
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
	periodEnvelope, _ := gotBody["timeOffPeriod"].(map[string]any)
	period, _ := periodEnvelope["period"].(map[string]any)
	if period["start"] != "2026-05-01" {
		t.Fatalf("expected nested period.start 2026-05-01 in body, got %#v", gotBody)
	}
	if period["days"] != float64(5) {
		t.Fatalf("expected nested period.days 5 in body, got %#v", gotBody)
	}
	if gotBody["note"] != "Family vacation" {
		t.Fatalf("expected note in body, got %v", gotBody["note"])
	}
	if periodEnvelope["isHalfDay"] != false {
		t.Fatalf("expected default isHalfDay=false in body, got %#v", gotBody)
	}
	if periodEnvelope["halfDayPeriod"] != "NOT_DEFINED" {
		t.Fatalf("expected default halfDayPeriod=NOT_DEFINED in body, got %#v", gotBody)
	}
	if periodEnvelope["timeOffHalfDayPeriod"] != "NOT_DEFINED" {
		t.Fatalf("expected default timeOffHalfDayPeriod=NOT_DEFINED in body, got %#v", gotBody)
	}
}

func TestCreateTimeOffRequestRejectsMissingNote(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("create time-off missing note must not reach upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.createTimeOffRequest(context.Background(), map[string]any{
		"policy_id": "pol1",
		"start":     "2026-05-01",
		"end":       "2026-05-05",
	})
	if err == nil {
		t.Fatal("expected missing note error")
	}
}

func TestTimeOffRequestDaysBoundaryCases(t *testing.T) {
	tests := []struct {
		name    string
		start   string
		end     string
		want    int
		wantErr string
	}{
		{name: "single_day", start: "2026-05-01", end: "2026-05-01", want: 1},
		{name: "five_days", start: "2026-05-01", end: "2026-05-05", want: 5},
		{name: "end_before_start", start: "2026-05-05", end: "2026-05-01", wantErr: "end must be on or after start"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timeOffRequestDays(tt.start, tt.end, time.UTC)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("timeOffRequestDays failed: %v", err)
			}
			if got != tt.want {
				t.Fatalf("days=%d, want %d", got, tt.want)
			}
		})
	}
}

func TestUpdateTimeOffRequestPatchesBareRequestPath(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
		wantPath  = "/workspaces/ws1/time-off/policies/" + policyID + "/requests/" + requestID
	)
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != wantPath {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{
			"id":     requestID,
			"status": gotBody["status"],
			"note":   gotBody["note"],
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.updateTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
		"status":     "APPROVED",
		"note":       "Approved after manager review",
	})
	if err != nil {
		t.Fatalf("update time off request failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected ok result")
	}
	if result.Action != "clockify_update_time_off_request" {
		t.Fatalf("expected action clockify_update_time_off_request, got %s", result.Action)
	}
	if gotBody["status"] != "APPROVED" {
		t.Fatalf("expected status APPROVED in body, got %#v", gotBody)
	}
	if gotBody["note"] != "Approved after manager review" {
		t.Fatalf("expected note in body, got %#v", gotBody)
	}
}

func TestUpdateTimeOffRequestRejectsBadStatus(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("bad time-off status must not reach upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  "abc123def456789012345678",
		"request_id": "abc123def456789012345679",
		"status":     "DENIED",
	})
	if err == nil || !strings.Contains(err.Error(), "status must be APPROVED or REJECTED") {
		t.Fatalf("expected bad status error, got %v", err)
	}
}

func TestUpdateTimeOffRequestRejectsEmptyBody(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("empty time-off update must not reach upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  "abc123def456789012345678",
		"request_id": "abc123def456789012345679",
	})
	if err == nil || !strings.Contains(err.Error(), "at least one field") {
		t.Fatalf("expected empty update body error, got %v", err)
	}
}

func TestApproveTimeOffPatchesBareRequestPathWithStatusApproved(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
		wantPath  = "/workspaces/ws1/time-off/policies/" + policyID + "/requests/" + requestID
	)
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != wantPath {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{
			"id":     requestID,
			"status": gotBody["status"],
			"note":   gotBody["note"],
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.approveTimeOff(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
		"note":       "Approved",
	})
	if err != nil {
		t.Fatalf("approve time off failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected ok result")
	}
	if result.Action != "clockify_approve_time_off" {
		t.Fatalf("expected action clockify_approve_time_off, got %s", result.Action)
	}
	if gotBody["status"] != "APPROVED" {
		t.Fatalf("expected status APPROVED in body, got %#v", gotBody)
	}
	if gotBody["note"] != "Approved" {
		t.Fatalf("expected note in body, got %#v", gotBody)
	}
}

func TestDenyTimeOffPatchesBareRequestPathWithStatusRejected(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
		wantPath  = "/workspaces/ws1/time-off/policies/" + policyID + "/requests/" + requestID
	)
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != wantPath {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{
			"id":     requestID,
			"status": gotBody["status"],
			"note":   gotBody["note"],
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.denyTimeOff(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
		"note":       "Insufficient balance",
	})
	if err != nil {
		t.Fatalf("deny time off failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected ok result")
	}
	if result.Action != "clockify_deny_time_off" {
		t.Fatalf("expected action clockify_deny_time_off, got %s", result.Action)
	}
	if gotBody["status"] != "REJECTED" {
		t.Fatalf("expected status REJECTED in body, got %#v", gotBody)
	}
	if gotBody["note"] != "Insufficient balance" {
		t.Fatalf("expected note in body, got %#v", gotBody)
	}
}

func TestDeleteAssignmentDryRun(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run delete assignment should not reach upstream, got %s %s", r.Method, r.URL.Path)
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
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data for dry run, got %T", result.Data)
	}
	if dataMap["dry_run"] != true {
		t.Fatal("expected dry_run=true in result data")
	}
	if dataMap["resource"] != nil {
		t.Fatalf("expected minimal dry-run resource nil, got %#v", dataMap["resource"])
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
