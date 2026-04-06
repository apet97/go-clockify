package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"goclmcp/internal/clockify"
)

func TestRegistryContainsCoreAndReportWorkflowTools(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	reg := svc.Registry()
	if len(reg) < 27 {
		t.Fatalf("expected at least 27 tools, got %d", len(reg))
	}

	names := map[string]bool{}
	for _, d := range reg {
		names[d.Tool.Name] = true
	}

	for _, want := range []string{
		"clockify_whoami",
		"clockify_list_workspaces",
		"clockify_get_workspace",
		"clockify_list_users",
		"clockify_current_user",
		"clockify_list_projects",
		"clockify_get_project",
		"clockify_list_clients",
		"clockify_list_tags",
		"clockify_list_tasks",
		"clockify_list_entries",
		"clockify_get_entry",
		"clockify_today_entries",
		"clockify_summary_report",
		"clockify_weekly_summary",
		"clockify_quick_report",
		"clockify_start_timer",
		"clockify_stop_timer",
		"clockify_log_time",
		"clockify_add_entry",
		"clockify_update_entry",
		"clockify_delete_entry",
		"clockify_find_and_update_entry",
		"clockify_create_project",
		"clockify_create_client",
		"clockify_create_tag",
		"clockify_create_task",
	} {
		if !names[want] {
			t.Fatalf("missing tool %s", want)
		}
	}
}

func TestSummaryReportAggregatesEntries(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			if got := r.URL.Query().Get("start"); got == "" {
				t.Fatalf("expected start filter")
			}
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "Build", ProjectID: "p1", ProjectName: "Project A", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T11:00:00Z"}},
				{ID: "e2", Description: "Review", ProjectID: "p1", ProjectName: "Project A", TimeInterval: clockify.TimeInterval{Start: "2026-04-02T09:00:00Z", End: "2026-04-02T10:30:00Z"}},
				{ID: "e3", Description: "Ops", ProjectID: "p2", ProjectName: "Project B", TimeInterval: clockify.TimeInterval{Start: "2026-04-03T12:00:00Z", End: "2026-04-03T13:00:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.SummaryReport(context.Background(), map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("summary report failed: %v", err)
	}

	data, ok := result.Data.(SummaryData)
	if !ok {
		t.Fatalf("unexpected summary data type: %T", result.Data)
	}
	if data.Totals.Entries != 3 {
		t.Fatalf("expected 3 entries, got %d", data.Totals.Entries)
	}
	if len(data.ByProject) != 2 {
		t.Fatalf("expected 2 project groups, got %d", len(data.ByProject))
	}
	if data.ByProject[0].ProjectName != "Project A" {
		t.Fatalf("expected top project Project A, got %+v", data.ByProject[0])
	}
	if data.ByProject[0].TotalSeconds != 12600 {
		t.Fatalf("expected 12600 seconds for Project A, got %d", data.ByProject[0].TotalSeconds)
	}
}

func TestFindAndUpdateEntryFailsOnAmbiguousMatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "standup", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T09:15:00Z"}},
				{ID: "e2", Description: "standup notes", TimeInterval: clockify.TimeInterval{Start: "2026-04-02T09:00:00Z", End: "2026-04-02T09:20:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"description_contains": "standup",
		"new_description":      "Daily standup",
	})
	if err == nil || !strings.Contains(err.Error(), "multiple entries matched") {
		t.Fatalf("expected ambiguous match error, got %v", err)
	}
}

func TestLogTimeCreatesFinishedEntry(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/time-entries":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["start"] != "2026-04-01T09:00:00Z" || body["end"] != "2026-04-01T10:30:00Z" {
				t.Fatalf("unexpected body: %+v", body)
			}
			respondJSON(t, w, clockify.TimeEntry{ID: "e1", Description: "Focus", ProjectID: "p1", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:30:00Z"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.LogTime(context.Background(), map[string]any{
		"project_id":  "p1",
		"description": "Focus",
		"start":       "2026-04-01T09:00:00Z",
		"end":         "2026-04-01T10:30:00Z",
		"billable":    true,
	})
	if err != nil {
		t.Fatalf("log time failed: %v", err)
	}
	env, ok := result.(ResultEnvelope)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	data, ok := env.Data.(LogTimeData)
	if !ok {
		t.Fatalf("unexpected data type: %T", env.Data)
	}
	if data.Entry.ID != "e1" {
		t.Fatalf("unexpected entry: %+v", data.Entry)
	}
}

func TestGetEntry(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/time-entries/abc123def456789012345678":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "abc123def456789012345678",
				Description: "Meeting",
				ProjectID:   "p1",
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-01T09:00:00Z",
					End:   "2026-04-01T10:00:00Z",
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.GetEntry(context.Background(), map[string]any{
		"entry_id": "abc123def456789012345678",
	})
	if err != nil {
		t.Fatalf("get entry failed: %v", err)
	}
	if result.Action != "clockify_get_entry" {
		t.Fatalf("expected action clockify_get_entry, got %s", result.Action)
	}
	entry, ok := result.Data.(clockify.TimeEntry)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if entry.ID != "abc123def456789012345678" {
		t.Fatalf("unexpected entry ID: %s", entry.ID)
	}
	if entry.Description != "Meeting" {
		t.Fatalf("unexpected description: %s", entry.Description)
	}
}

func TestTodayEntries(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			// Verify date range parameters are present
			if r.URL.Query().Get("start") == "" {
				t.Fatalf("expected start parameter for today range")
			}
			if r.URL.Query().Get("end") == "" {
				t.Fatalf("expected end parameter for today range")
			}
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "Morning standup", TimeInterval: clockify.TimeInterval{Start: "2026-04-06T09:00:00Z", End: "2026-04-06T09:15:00Z"}},
				{ID: "e2", Description: "Dev work", TimeInterval: clockify.TimeInterval{Start: "2026-04-06T09:30:00Z", End: "2026-04-06T12:00:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TodayEntries(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("today entries failed: %v", err)
	}
	if result.Action != "clockify_today_entries" {
		t.Fatalf("expected action clockify_today_entries, got %s", result.Action)
	}
	entries, ok := result.Data.([]clockify.TimeEntry)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestAddEntry(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/time-entries":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["start"] == nil || body["start"] == "" {
				t.Fatalf("expected start in payload, got: %+v", body)
			}
			if body["end"] == nil || body["end"] == "" {
				t.Fatalf("expected end in payload, got: %+v", body)
			}
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "new1",
				Description: "New task",
				ProjectID:   "p1",
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-06T09:00:00Z",
					End:   "2026-04-06T10:00:00Z",
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.AddEntry(context.Background(), map[string]any{
		"start":       "2026-04-06T09:00:00Z",
		"end":         "2026-04-06T10:00:00Z",
		"description": "New task",
		"project_id":  "p1",
		"billable":    true,
	})
	if err != nil {
		t.Fatalf("add entry failed: %v", err)
	}
	if result.Action != "clockify_add_entry" {
		t.Fatalf("expected action clockify_add_entry, got %s", result.Action)
	}
	entry, ok := result.Data.(clockify.TimeEntry)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if entry.ID != "new1" {
		t.Fatalf("unexpected entry ID: %s", entry.ID)
	}
}

func TestUpdateEntryFetchThenPut(t *testing.T) {
	var gotPutBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/time-entries/abc123def456789012345678" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "abc123def456789012345678",
				Description: "Old description",
				ProjectID:   "p1",
				Billable:    false,
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-01T09:00:00Z",
					End:   "2026-04-01T10:00:00Z",
				},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/abc123def456789012345678" && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&gotPutBody); err != nil {
				t.Fatalf("decode PUT body: %v", err)
			}
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "abc123def456789012345678",
				Description: "Updated description",
				ProjectID:   "p1",
				Billable:    true,
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-01T09:00:00Z",
					End:   "2026-04-01T10:00:00Z",
				},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id":    "abc123def456789012345678",
		"description": "Updated description",
		"billable":    true,
	})
	if err != nil {
		t.Fatalf("update entry failed: %v", err)
	}
	if result.Action != "clockify_update_entry" {
		t.Fatalf("expected action clockify_update_entry, got %s", result.Action)
	}
	// Verify the PUT payload includes merged fields from the fetched entry
	if gotPutBody == nil {
		t.Fatal("expected PUT to be called")
	}
	if gotPutBody["start"] != "2026-04-01T09:00:00Z" {
		t.Fatalf("PUT should carry original start, got %v", gotPutBody["start"])
	}
	if gotPutBody["description"] != "Updated description" {
		t.Fatalf("PUT should carry new description, got %v", gotPutBody["description"])
	}
	if gotPutBody["billable"] != true {
		t.Fatalf("PUT should carry new billable=true, got %v", gotPutBody["billable"])
	}
	// Verify changedFields in meta
	changedFields, ok := result.Meta["changedFields"].([]string)
	if !ok {
		t.Fatalf("expected changedFields in meta, got %T", result.Meta["changedFields"])
	}
	if len(changedFields) != 2 {
		t.Fatalf("expected 2 changed fields, got %d: %v", len(changedFields), changedFields)
	}
}

func TestDeleteEntryDryRun(t *testing.T) {
	var deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/time-entries/abc123def456789012345678" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "abc123def456789012345678",
				Description: "Entry to delete",
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-01T09:00:00Z",
					End:   "2026-04-01T10:00:00Z",
				},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/abc123def456789012345678" && r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteEntry(context.Background(), map[string]any{
		"entry_id": "abc123def456789012345678",
		"dry_run":  true,
	})
	if err != nil {
		t.Fatalf("delete entry dry run failed: %v", err)
	}
	if result.Action != "clockify_delete_entry" {
		t.Fatalf("expected action clockify_delete_entry, got %s", result.Action)
	}
	if deleteCalled {
		t.Fatal("DELETE should NOT be called during dry run")
	}
	// The data should be a dry-run wrapper
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data for dry run, got %T", result.Data)
	}
	if dataMap["dry_run"] != true {
		t.Fatalf("expected dry_run=true in result data")
	}
	if dataMap["note"] == nil {
		t.Fatal("expected note in dry run result")
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*clockify.Client, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	client := clockify.NewClient("test-key", ts.URL, 5*time.Second, 0)
	return client, ts.Close
}

func respondJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
