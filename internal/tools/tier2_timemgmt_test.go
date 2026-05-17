package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/jsonschema"
)

// TestTimeOffRequestViewOmitsEmptyNames proves the time-off view omits
// user.name/user.email/policy.name when the upstream approve/deny/update
// response does not include them, instead of emitting empty strings.
func TestTimeOffRequestViewOmitsEmptyNames(t *testing.T) {
	view := timeOffRequestViewFromRaw(map[string]any{
		"id": "req1", "status": "APPROVED", "userId": "u1", "policyId": "p1",
	})
	user, ok := view["user"].(map[string]any)
	if !ok {
		t.Fatalf("user = %T, want map", view["user"])
	}
	if _, has := user["name"]; has {
		t.Errorf("user.name should be omitted when empty, got %v", user["name"])
	}
	if _, has := user["email"]; has {
		t.Errorf("user.email should be omitted when empty, got %v", user["email"])
	}
	if user["id"] != "u1" {
		t.Errorf("user.id = %v, want u1", user["id"])
	}
	policy, ok := view["policy"].(map[string]any)
	if !ok {
		t.Fatalf("policy = %T, want map", view["policy"])
	}
	if _, has := policy["name"]; has {
		t.Errorf("policy.name should be omitted when empty, got %v", policy["name"])
	}

	// When the upstream does return names, they pass through unchanged.
	full := timeOffRequestViewFromRaw(map[string]any{
		"id": "req2", "status": "APPROVED", "userId": "u2", "policyId": "p2",
		"userName": "Ada", "userEmail": "ada@example.com", "policyName": "PTO",
	})
	fu := full["user"].(map[string]any)
	if fu["name"] != "Ada" || fu["email"] != "ada@example.com" {
		t.Errorf("populated user = %#v, want name/email present", fu)
	}
	if full["policy"].(map[string]any)["name"] != "PTO" {
		t.Errorf("populated policy.name missing: %#v", full["policy"])
	}
}

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
	if len(descs) != 13 {
		t.Fatalf("expected 13 time-off tools, got %d", len(descs))
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
		"clockify_time_off_balance_update",
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
			recurring, ok := body["recurringAssignment"].(map[string]any)
			if !ok {
				t.Fatalf("expected recurringAssignment defaults, got %#v", body["recurringAssignment"])
			}
			if _, ok := recurring["repeat"]; ok {
				t.Fatalf("recurringAssignment must omit repeat per canonical OpenAPI, got %#v", recurring)
			}
			if weeks, ok := reportNumber(recurring["weeks"]); !ok || weeks != 1 {
				t.Fatalf("expected one-off weeks=1, got %#v", recurring)
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

func TestCreateAssignmentOmitsRecurringRepeatWhenRequested(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/scheduling/assignments/recurring" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, []map[string]any{{"id": "a1"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.createAssignment(context.Background(), map[string]any{
		"user_id":       "aaaaaaaaaaaaaaaaaaaaaaaa",
		"project_id":    "bbbbbbbbbbbbbbbbbbbbbbbb",
		"start":         "2026-04-01T09:00:00Z",
		"end":           "2026-04-02T17:30:00Z",
		"hours_per_day": 8.0,
		"repeat":        true,
		"weeks":         2,
	}); err != nil {
		t.Fatalf("create assignment failed: %v", err)
	}
	recurring, ok := gotBody["recurringAssignment"].(map[string]any)
	if !ok {
		t.Fatalf("expected recurringAssignment body, got %#v", gotBody)
	}
	if _, ok := recurring["repeat"]; ok {
		t.Fatalf("recurringAssignment must omit repeat per canonical OpenAPI, got %#v", recurring)
	}
	if weeks, ok := reportNumber(recurring["weeks"]); !ok || weeks != 2 {
		t.Fatalf("expected weeks=2, got %#v", recurring)
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

func TestCreateTimeOffRequestDryRunDoesNotPost(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("create time-off dry-run must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.createTimeOffRequest(context.Background(), map[string]any{
		"policy_id": "pol1",
		"start":     "2026-05-01",
		"end":       "2026-05-05",
		"note":      "Family vacation",
		"dry_run":   true,
	})
	if err != nil {
		t.Fatalf("create time off dry-run failed: %v", err)
	}
	if result.Action != "clockify_create_time_off_request" {
		t.Fatalf("expected action clockify_create_time_off_request, got %s", result.Action)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["dry_run"] != true {
		t.Fatalf("expected dry-run payload, got %#v", result.Data)
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

func TestGetTimeOffRequestUsesBareRequestEndpointAndNormalizesStructuredStatus(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/time-off/requests/"+requestID {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{
			"id":       requestID,
			"policyId": policyID,
			"status":   map[string]any{"statusType": "pending"},
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.getTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
	})
	if err != nil {
		t.Fatalf("get time off request failed: %v", err)
	}
	data, ok := result.Data.(TimeOffRequestView)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["status"] != "PENDING" {
		t.Fatalf("expected normalized request status PENDING, got %#v", data["status"])
	}
}

func TestTimeOffRequestViewFlagsPastPendingPeriodAsStale(t *testing.T) {
	view := timeOffRequestViewFromRaw(map[string]any{
		"id":     "req1",
		"status": "PENDING",
		"period": map[string]any{
			"period": map[string]any{
				"end": "2020-01-01T00:00:00Z",
			},
		},
	})
	stale, ok := view["stale"].(map[string]any)
	if !ok {
		t.Fatalf("expected stale marker, got %#v", view["stale"])
	}
	if stale["reason"] != "pending_period_in_past" {
		t.Fatalf("unexpected stale reason: %#v", stale["reason"])
	}
}

func TestTimeOffRequestViewDoesNotFlagFuturePendingPeriodAsStale(t *testing.T) {
	view := timeOffRequestViewFromRaw(map[string]any{
		"id":     "req1",
		"status": "PENDING",
		"period": map[string]any{
			"period": map[string]any{
				"end": "2999-01-01T00:00:00Z",
			},
		},
	})
	if _, ok := view["stale"]; ok {
		t.Fatalf("did not expect stale marker: %#v", view["stale"])
	}
}

func TestListTimeOffRequestsForwardsRejectedStatus(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantStat string
	}{
		{name: "REJECTED stays REJECTED", input: "REJECTED", wantStat: "REJECTED"},
		{name: "DENIED maps to REJECTED", input: "DENIED", wantStat: "REJECTED"},
		{name: "denied lower-case still maps", input: "denied", wantStat: "REJECTED"},
		{name: "PENDING passes through", input: "PENDING", wantStat: "PENDING"},
		{name: "APPROVED passes through", input: "APPROVED", wantStat: "APPROVED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotStatuses []any
			client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/time-off/requests" {
					t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				gotStatuses, _ = body["statuses"].([]any)
				respondJSON(t, w, map[string]any{"count": 0, "requests": []map[string]any{}})
			})
			defer cleanup()

			svc := New(client, "ws1")
			if _, err := svc.listTimeOffRequests(context.Background(), map[string]any{"status": tc.input}); err != nil {
				t.Fatalf("listTimeOffRequests: %v", err)
			}
			if len(gotStatuses) != 1 || gotStatuses[0] != tc.wantStat {
				t.Fatalf("statuses = %v, want [%s]", gotStatuses, tc.wantStat)
			}
		})
	}
}

func TestGetTimeOffRequestSearchesApprovedRequestsWhenBareEndpointMisses(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/time-off/requests/"+requestID:
			http.Error(w, "not found", http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/time-off/requests":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			statuses, _ := body["statuses"].([]any)
			if len(statuses) == 1 && statuses[0] == "APPROVED" {
				respondJSON(t, w, map[string]any{
					"requests": []map[string]any{{
						"id":       requestID,
						"policyId": policyID,
						"status":   map[string]any{"statusType": "APPROVED"},
					}},
				})
				return
			}
			respondJSON(t, w, map[string]any{"requests": []map[string]any{}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.getTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
	})
	if err != nil {
		t.Fatalf("get time off request failed: %v", err)
	}
	data, ok := result.Data.(TimeOffRequestView)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["status"] != "APPROVED" {
		t.Fatalf("expected fallback-approved request, got %#v", data["status"])
	}
}

func TestDeleteTimeOffRequestDryRunUsesBareRequestEndpoint(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/time-off/requests/"+requestID {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": requestID, "policyId": policyID, "status": "PENDING"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.deleteTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
		"dry_run":    true,
	})
	if err != nil {
		t.Fatalf("delete time off dry-run failed: %v", err)
	}
	if result.Action != "clockify_delete_time_off_request" {
		t.Fatalf("expected action clockify_delete_time_off_request, got %s", result.Action)
	}
}

func TestTimeOffBalanceUsesUserBalanceEndpointAndFiltersPolicy(t *testing.T) {
	const (
		userID   = "abc123def456789012345678"
		policyID = "abc123def456789012345679"
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/time-off/balance/user/"+userID {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("page-size") != "200" {
			t.Fatalf("expected page-size=200, got %s", r.URL.RawQuery)
		}
		respondJSON(t, w, map[string]any{
			"count": 2,
			"balances": []map[string]any{
				{"policyId": "abc123def456789012345670", "available": 1},
				{"policyId": policyID, "available": 5, "timeUnit": "DAYS"},
			},
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.timeOffBalance(context.Background(), map[string]any{
		"policy_id": policyID,
		"user_id":   userID,
	})
	if err != nil {
		t.Fatalf("time off balance failed: %v", err)
	}
	data, ok := result.Data.(TimeOffBalanceView)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	policy, ok := data["policy"].(map[string]any)
	if !ok || policy["id"] != policyID {
		t.Fatalf("expected selected policy %s, got %#v", policyID, data["policy"])
	}
}

func TestUpdateTimeOffBalancePatchesPolicyEndpoint(t *testing.T) {
	const (
		policyID = "abc123def456789012345679"
		userA    = "abc123def456789012345670"
		userB    = "abc123def456789012345671"
	)
	var (
		gotPath   string
		gotMethod string
		gotBody   map[string]any
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.updateTimeOffBalance(context.Background(), map[string]any{
		"policy_id": policyID,
		"user_ids":  []any{userA, userB},
		"value":     float64(22),
		"note":      "Bonus days added.",
	})
	if err != nil {
		t.Fatalf("updateTimeOffBalance: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %s, want PATCH", gotMethod)
	}
	wantPath := "/workspaces/ws1/time-off/balance/policy/" + policyID
	if gotPath != wantPath {
		t.Fatalf("path = %s, want %s", gotPath, wantPath)
	}
	if gotBody["note"] != "Bonus days added." {
		t.Fatalf("body note = %v, want bonus", gotBody["note"])
	}
	if gotBody["value"] != float64(22) {
		t.Fatalf("body value = %v, want 22", gotBody["value"])
	}
	users, ok := gotBody["userIds"].([]any)
	if !ok || len(users) != 2 || users[0] != userA || users[1] != userB {
		t.Fatalf("body userIds = %v, want [%s %s]", gotBody["userIds"], userA, userB)
	}
	if !result.OK {
		t.Fatalf("envelope ok=%v, want true", result.OK)
	}
	if result.Action != "clockify_time_off_balance_update" {
		t.Fatalf("envelope action = %s, want clockify_time_off_balance_update", result.Action)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["policyId"] != policyID || data["value"] != float64(22) {
		t.Fatalf("envelope data = %#v", result.Data)
	}
}

// TestUpdateTimeOffBalanceRejectsDuplicateLiteralUserIDs pins the
// BALANCEDOC.md "userIds non-empty unique" contract: the same literal
// user ID listed twice must fail closed before any HTTP call.
func TestUpdateTimeOffBalanceRejectsDuplicateLiteralUserIDs(t *testing.T) {
	const (
		policyID = "abc123def456789012345679"
		userX    = "abc123def456789012345670"
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("duplicate user_ids must be rejected before any HTTP call; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateTimeOffBalance(context.Background(), map[string]any{
		"policy_id": policyID,
		"user_ids":  []any{userX, userX},
		"value":     float64(8),
		"note":      "Manual adjustment",
	})
	if err == nil {
		t.Fatal("expected duplicate user_ids to be rejected")
	}
	if !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), userX) {
		t.Fatalf("error = %q, want it to name the duplicate user %s", err, userX)
	}
}

// TestUpdateTimeOffBalanceRejectsRefsResolvingToSameUser proves the
// duplicate check runs after ref resolution: a literal ID and a name
// that resolve to the same user must be rejected before the PATCH.
func TestUpdateTimeOffBalanceRejectsRefsResolvingToSameUser(t *testing.T) {
	const (
		policyID = "abc123def456789012345679"
		userX    = "abc123def456789012345670"
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/users" {
			respondJSON(t, w, []map[string]any{{"id": userX, "name": "Alice"}})
			return
		}
		t.Fatalf("refs resolving to the same user must be rejected before PATCH; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateTimeOffBalance(context.Background(), map[string]any{
		"policy_id": policyID,
		"user_ids":  []any{userX, "Alice"},
		"value":     float64(8),
		"note":      "Manual adjustment",
	})
	if err == nil {
		t.Fatal("expected refs resolving to the same user to be rejected")
	}
	if !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), userX) {
		t.Fatalf("error = %q, want it to name the duplicate user %s", err, userX)
	}
}

// TestUpdateTimeOffBalanceUniqueRefsPatchResolvedBody confirms the
// duplicate guard does not over-reject: distinct refs (a literal ID and
// a name resolving to a different user) still PATCH the resolved body.
func TestUpdateTimeOffBalanceUniqueRefsPatchResolvedBody(t *testing.T) {
	const (
		policyID = "abc123def456789012345679"
		userX    = "abc123def456789012345670"
		userY    = "abc123def456789012345671"
	)
	var (
		gotMethod string
		gotPath   string
		gotBody   map[string]any
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/users" {
			respondJSON(t, w, []map[string]any{{"id": userY, "name": "Bob"}})
			return
		}
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.updateTimeOffBalance(context.Background(), map[string]any{
		"policy_id": policyID,
		"user_ids":  []any{userX, "Bob"},
		"value":     float64(12),
		"note":      "Quarterly grant",
	})
	if err != nil {
		t.Fatalf("updateTimeOffBalance: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %s, want PATCH", gotMethod)
	}
	if gotPath != "/workspaces/ws1/time-off/balance/policy/"+policyID {
		t.Fatalf("path = %s", gotPath)
	}
	users, ok := gotBody["userIds"].([]any)
	if !ok || len(users) != 2 || users[0] != userX || users[1] != userY {
		t.Fatalf("body userIds = %v, want [%s %s]", gotBody["userIds"], userX, userY)
	}
	if gotBody["note"] != "Quarterly grant" || gotBody["value"] != float64(12) {
		t.Fatalf("body note/value = %v / %v", gotBody["note"], gotBody["value"])
	}
	if !result.OK || result.Action != "clockify_time_off_balance_update" {
		t.Fatalf("envelope = %#v", result)
	}
}

func TestUpdateTimeOffBalanceDryRunSkipsHTTP(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry_run should not hit Clockify; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.updateTimeOffBalance(context.Background(), map[string]any{
		"policy_id": "abc123def456789012345679",
		"user_ids":  []any{"abc123def456789012345670"},
		"value":     float64(8),
		"note":      "Manual adjustment",
		"dry_run":   true,
	})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if !result.OK || result.Action != "clockify_time_off_balance_update" {
		t.Fatalf("envelope = %#v", result)
	}
}

func TestUpdateTimeOffBalanceValidatesRequiredArgs(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("should not reach HTTP layer; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()
	svc := New(client, "ws1")

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "missing user_ids",
			args: map[string]any{"policy_id": "abc123def456789012345679", "value": float64(1), "note": "x"},
			want: "user_ids",
		},
		{
			name: "missing note",
			args: map[string]any{"policy_id": "abc123def456789012345679", "user_ids": []any{"abc123def456789012345670"}, "value": float64(1)},
			want: "note",
		},
		{
			name: "missing value",
			args: map[string]any{"policy_id": "abc123def456789012345679", "user_ids": []any{"abc123def456789012345670"}, "note": "x"},
			want: "value",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.updateTimeOffBalance(context.Background(), tc.args)
			if err == nil {
				t.Fatalf("expected error mentioning %s", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want contains %q", err, tc.want)
			}
		})
	}
}

// TestUpdateTimeOffBalanceSchemaBoundsValue pins the Clockify-documented
// [-10000, 10000] range on the value input. The MCP dispatch layer
// validates arguments against the tool's InputSchema before the handler
// runs, so an out-of-range value must be rejected as a protocol error;
// this test exercises that schema directly.
func TestUpdateTimeOffBalanceSchemaBoundsValue(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	var schema map[string]any
	for _, d := range timeOffHandlers(svc) {
		if d.Tool.Name == "clockify_time_off_balance_update" {
			schema = d.Tool.InputSchema
			break
		}
	}
	if schema == nil {
		t.Fatal("clockify_time_off_balance_update descriptor not found")
	}

	base := func(value float64) map[string]any {
		return map[string]any{
			"policy_id": "abc123def456789012345679",
			"user_ids":  []any{"abc123def456789012345670"},
			"value":     value,
			"note":      "schema bounds probe",
		}
	}
	cases := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{name: "zero is valid", value: 0},
		{name: "max boundary valid", value: 10000},
		{name: "min boundary valid", value: -10000},
		{name: "above max rejected", value: 10001, wantErr: true},
		{name: "below min rejected", value: -10001, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := jsonschema.Validate(schema, base(tc.value))
			if tc.wantErr && err == nil {
				t.Fatalf("value %v: expected schema rejection", tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("value %v: unexpected schema error: %v", tc.value, err)
			}
		})
	}
}

func TestCreateTimeOffPolicyUsesClockifyPolicyBodyShape(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/user" {
			respondJSON(t, w, map[string]any{"id": "user1", "name": "Owner"})
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/time-off/policies" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "pol1", "name": gotBody["name"]})
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.createTimeOffPolicy(context.Background(), map[string]any{
		"name":              "Vacation",
		"time_unit":         "DAYS",
		"days_per_year":     5,
		"negative_balance":  true,
		"requires_approval": false,
	})
	if err != nil {
		t.Fatalf("create time off policy failed: %v", err)
	}
	if gotBody["allowNegativeBalance"] != true {
		t.Fatalf("expected allowNegativeBalance=true, got %#v", gotBody)
	}
	negativeBalance, ok := gotBody["negativeBalance"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured negativeBalance, got %#v", gotBody["negativeBalance"])
	}
	if negativeBalance["timeUnit"] != "DAYS" || negativeBalance["period"] != "YEAR" {
		t.Fatalf("unexpected negativeBalance defaults: %#v", negativeBalance)
	}
	if negativeBalance["amount"] != float64(10) {
		t.Fatalf("expected negativeBalance.amount=10, got %#v", negativeBalance)
	}
	approve, ok := gotBody["approve"].(map[string]any)
	if !ok || approve["requiresApproval"] != false {
		t.Fatalf("expected approve.requiresApproval=false, got %#v", gotBody["approve"])
	}
	if _, exists := gotBody["requiresApproval"]; exists {
		t.Fatalf("requiresApproval must be nested under approve, got %#v", gotBody)
	}
	users, ok := gotBody["users"].(map[string]any)
	if !ok {
		t.Fatalf("expected current-user filter, got %#v", gotBody["users"])
	}
	ids, ok := users["ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "user1" {
		t.Fatalf("expected current user ID in users filter, got %#v", users)
	}
}

func TestUpdateTimeOffPolicyMapsNegativeBalanceAndApprovalShape(t *testing.T) {
	const policyID = "abc123def456789012345678"
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/time-off/policies/"+policyID {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":                   policyID,
				"name":                 "Existing",
				"allowNegativeBalance": false,
				"approve":              map[string]any{"requiresApproval": true},
				"archived":             false,
				"everyoneIncludingNew": false,
				"hasExpiration":        false,
				"timeUnit":             "DAYS",
				"userGroups":           map[string]any{"contains": "CONTAINS", "ids": []any{}, "status": "ACTIVE"},
				"users":                map[string]any{"contains": "CONTAINS", "ids": []any{}, "status": "ACTIVE"},
			})
		case http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{"id": policyID, "name": gotBody["name"]})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateTimeOffPolicy(context.Background(), map[string]any{
		"policy_id":         policyID,
		"negative_balance":  true,
		"requires_approval": false,
	})
	if err != nil {
		t.Fatalf("update time off policy failed: %v", err)
	}
	if gotBody["allowNegativeBalance"] != true {
		t.Fatalf("expected allowNegativeBalance=true, got %#v", gotBody)
	}
	if _, ok := gotBody["negativeBalance"].(map[string]any); !ok {
		t.Fatalf("expected structured negativeBalance, got %#v", gotBody["negativeBalance"])
	}
	if amount, ok := reportNumber(gotBody["negativeBalance"].(map[string]any)["amount"]); !ok || amount != 10 {
		t.Fatalf("expected update to coerce reusable negativeBalance.amount=10, got %#v", gotBody["negativeBalance"])
	}
	approve, ok := gotBody["approve"].(map[string]any)
	if !ok || approve["requiresApproval"] != false {
		t.Fatalf("expected approve.requiresApproval=false, got %#v", gotBody["approve"])
	}
	if _, exists := gotBody["requiresApproval"]; exists {
		t.Fatalf("requiresApproval must be nested under approve, got %#v", gotBody)
	}
}

func TestArchiveTimeOffPolicyUsesStatusPatch(t *testing.T) {
	const policyID = "abc123def456789012345678"
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/workspaces/ws1/time-off/policies/"+policyID {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": policyID, "status": gotBody["status"]})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.archiveTimeOffPolicy(context.Background(), map[string]any{
		"policy_id": policyID,
		"archived":  true,
	})
	if err != nil {
		t.Fatalf("archive time off policy failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected ok result")
	}
	if gotBody["status"] != "ARCHIVED" {
		t.Fatalf("expected status ARCHIVED, got %#v", gotBody)
	}
	if _, exists := gotBody["archived"]; exists {
		t.Fatalf("archive endpoint expects status, not archived boolean: %#v", gotBody)
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
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/time-off/requests/"+requestID {
			respondJSON(t, w, map[string]any{"id": requestID, "policyId": policyID, "status": "PENDING"})
			return
		}
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
	if _, ok := gotBody["note"]; ok {
		t.Fatalf("time-off status patch body must not include removed note field, got %#v", gotBody)
	}
}

func TestUpdateTimeOffRequestIncludesEmptyNoteForStatusPatch(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
		wantPath  = "/workspaces/ws1/time-off/policies/" + policyID + "/requests/" + requestID
	)
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/time-off/requests/"+requestID {
			respondJSON(t, w, map[string]any{"id": requestID, "policyId": policyID, "status": "PENDING"})
			return
		}
		if r.Method != http.MethodPatch || r.URL.Path != wantPath {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": requestID, "status": gotBody["status"], "note": gotBody["note"]})
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
		"status":     "APPROVED",
	})
	if err != nil {
		t.Fatalf("update time off request failed: %v", err)
	}
	if _, ok := gotBody["note"]; ok {
		t.Fatalf("status patch body must omit removed note field, got %#v", gotBody)
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
	if err == nil || !strings.Contains(err.Error(), "status is required") {
		t.Fatalf("expected empty update body error, got %v", err)
	}
}

func TestUpdateTimeOffRequestRejectsNoteOnly(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("note-only time-off update must not reach upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateTimeOffRequest(context.Background(), map[string]any{
		"policy_id":  "abc123def456789012345678",
		"request_id": "abc123def456789012345679",
		"note":       "status endpoint requires a status",
	})
	if err == nil || !strings.Contains(err.Error(), "status is required") {
		t.Fatalf("expected status required error, got %v", err)
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
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/time-off/requests/"+requestID {
			respondJSON(t, w, map[string]any{"id": requestID, "policyId": policyID, "status": "PENDING"})
			return
		}
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

func TestApproveTimeOffIncludesEmptyNoteForCanonicalStatusBody(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
		wantPath  = "/workspaces/ws1/time-off/policies/" + policyID + "/requests/" + requestID
	)
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/time-off/requests/"+requestID {
			respondJSON(t, w, map[string]any{
				"id":       requestID,
				"policyId": policyID,
				"status":   "PENDING",
			})
			return
		}
		if r.Method != http.MethodPatch || r.URL.Path != wantPath {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": requestID, "status": gotBody["status"], "note": gotBody["note"]})
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.approveTimeOff(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
	})
	if err != nil {
		t.Fatalf("approve time off failed: %v", err)
	}
	if _, ok := gotBody["note"]; ok {
		t.Fatalf("status patch body must omit removed note field, got %#v", gotBody)
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
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/time-off/requests/"+requestID {
			respondJSON(t, w, map[string]any{"id": requestID, "policyId": policyID, "status": "PENDING"})
			return
		}
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

func TestDenyTimeOffRejectsNonPendingRequest(t *testing.T) {
	const (
		policyID  = "abc123def456789012345678"
		requestID = "abc123def456789012345679"
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/time-off/requests/"+requestID {
			respondJSON(t, w, map[string]any{"id": requestID, "policyId": policyID, "status": "APPROVED"})
			return
		}
		if r.Method == http.MethodPatch {
			t.Fatalf("non-pending request must not be patched")
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.denyTimeOff(context.Background(), map[string]any{
		"policy_id":  policyID,
		"request_id": requestID,
	})
	if err == nil || !strings.Contains(err.Error(), "must be PENDING") {
		t.Fatalf("expected pending-state guard, got %v", err)
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

func TestTimeOffBalanceDefaultsToCurrentUser(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u-current", Name: "Tester"})
		case r.URL.Path == "/workspaces/ws1/time-off/balance/user/u-current" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{"count": 956, "balances": []map[string]any{
				{"policyId": "pol1", "policyName": "PTO", "userId": "u-current", "balance": 3, "policyTimeUnit": "DAYS"},
				{"policyId": "pol2", "policyName": "Sick", "userId": "u-current", "balance": 1, "policyTimeUnit": "DAYS"},
			}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.timeOffBalance(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("timeOffBalance no-arg failed: %v", err)
	}
	if res.Meta["userId"] != "u-current" {
		t.Fatalf("userId meta = %v, want u-current", res.Meta["userId"])
	}
	if _, ok := res.Data.([]TimeOffBalanceView); !ok {
		t.Fatalf("data type = %T, want []TimeOffBalanceView", res.Data)
	}
	if res.Meta["count"] != 2 || res.Meta["total"] != 956 || res.Meta["has_more"] != true || res.Meta["dropped"] != 954 {
		t.Fatalf("time_off_balances meta should describe returned collection, got %#v", res.Meta)
	}
}

func TestTimeOffPoliciesListPaginationMeta(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/time-off/policies" && r.Method == http.MethodGet:
			if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("page-size") != "1" {
				t.Fatalf("unexpected pagination query: %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []map[string]any{{
				"id":           "pol2",
				"name":         "Second",
				"timeUnit":     "DAYS",
				"userIds":      []string{"u1", "u2"},
				"userGroupIds": []string{"g1"},
			}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.listTimeOffPolicies(context.Background(), map[string]any{"page": 2, "page_size": 1})
	if err != nil {
		t.Fatalf("listTimeOffPolicies: %v", err)
	}
	if res.Meta["page"] != 2 || res.Meta["pageSize"] != 1 || res.Meta["has_more"] != true {
		t.Fatalf("unexpected pagination meta: %#v", res.Meta)
	}
	policies, ok := res.Data.([]CompactTimeOffPolicyView)
	if !ok {
		t.Fatalf("data type = %T, want []CompactTimeOffPolicyView", res.Data)
	}
	if len(policies) != 1 || policies[0].ID != "pol2" || policies[0].TimeUnit != "DAYS" || policies[0].UserCount != 2 || policies[0].UserGroupCount != 1 {
		t.Fatalf("compact policy not preserved: %+v", policies)
	}
}

func TestCreateAssignment_RejectsGarbageDate(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "ws1")
	_, err := svc.createAssignment(context.Background(), map[string]any{
		"user_id":       "5e1b2c3d4e5f6a7b8c9d0e1f",
		"project_id":    "6e1b2c3d4e5f6a7b8c9d0e1f",
		"start":         "last tuesday",
		"end":           "2026-05-17",
		"hours_per_day": 8.0,
	})
	if err == nil || !strings.Contains(err.Error(), "could not parse date") {
		t.Fatalf("expected could not parse date error, got %v", err)
	}
}
