package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestFullAccessRegistryContainsCoreOneUserTools(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	reg := svc.FullAccessRegistry()
	if len(reg) != 152 {
		t.Fatalf("registry size=%d, want 152", len(reg))
	}

	names := map[string]bool{}
	for _, d := range reg {
		names[d.Tool.Name] = true
	}

	for _, want := range []string{
		"clockify_status",
		"clockify_tools_guide",
		"clockify_create_work_package",
		"clockify_log_work",
		"clockify_start_work",
		"clockify_stop_work",
		"clockify_switch_work",
		"clockify_review_day",
		"clockify_fix_entry",
		"clockify_clients_list",
		"clockify_clients_create",
		"clockify_projects_list",
		"clockify_projects_create",
		"clockify_tasks_list",
		"clockify_tasks_create",
		"clockify_tags_list",
		"clockify_tags_create",
		"clockify_entries_list",
		"clockify_entries_create",
		"clockify_entries_timer_start",
		"clockify_entries_timer_stop",
		"clockify_reports_summary",
		"clockify_workspace_settings",
		"clockify_api_request",
	} {
		if !names[want] {
			t.Fatalf("missing tool %s", want)
		}
	}
}

func TestSummaryReportUsesReportsAPI(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{"id": "u1", "settings": map[string]any{}})
		case r.URL.Path == "/workspaces/ws1/reports/summary" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode summary body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"totals": []map[string]any{{"entriesCount": 3}},
				"groupOne": []map[string]any{
					{"id": "p1", "name": "Project A", "duration": 12600},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.SummaryReport(context.Background(), map[string]any{
		"start":        "2026-04-01T00:00:00Z",
		"end":          "2026-04-08T00:00:00Z",
		"amount_shown": "PROFIT",
		"amounts":      []any{"EARNED", "COST", "PROFIT"},
		"summary_filter": map[string]any{
			"groups":             []any{"CLIENT", "PROJECT", "DATE"},
			"sort_column":        "PROFIT",
			"summary_chart_type": "PROJECT",
		},
	})
	if err != nil {
		t.Fatalf("summary report failed: %v", err)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected summary data type: %T", result.Data)
	}
	if _, ok := data["totals"]; !ok {
		t.Fatalf("expected upstream totals in data, got %#v", data)
	}
	if gotBody["dateRangeStart"] != "2026-04-01T00:00:00Z" || gotBody["dateRangeEnd"] != "2026-04-08T00:00:00Z" {
		t.Fatalf("date aliases not forwarded as dateRangeStart/dateRangeEnd: %#v", gotBody)
	}
	if gotBody["amountShown"] != "PROFIT" {
		t.Fatalf("amountShown = %v, want PROFIT", gotBody["amountShown"])
	}
	filter, _ := gotBody["summaryFilter"].(map[string]any)
	groups, _ := filter["groups"].([]any)
	if len(groups) != 3 || groups[2] != "DATE" {
		t.Fatalf("summary groups should send DATE upstream, got %#v", filter["groups"])
	}
	if _, has := gotBody["detailedFilter"]; has {
		t.Fatalf("summary report must not send detailedFilter: %#v", gotBody)
	}
	if result.Meta["source"] != "reports-api" {
		t.Fatalf("source meta = %v, want reports-api", result.Meta["source"])
	}
}

func TestSummaryReportDefaultsMoneyColumns(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" && r.Method == http.MethodGet {
			respondJSON(t, w, map[string]any{"id": "u1", "settings": map[string]any{}})
			return
		}
		if r.URL.Path != "/workspaces/ws1/reports/summary" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode summary body: %v", err)
		}
		respondJSON(t, w, map[string]any{"totals": []map[string]any{{"entriesCount": 1}}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.SummaryReport(context.Background(), map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-08T00:00:00Z",
		"summary_filter": map[string]any{
			"groups": []any{"CLIENT", "PROJECT", "DATE"},
		},
	})
	if err != nil {
		t.Fatalf("summary report failed: %v", err)
	}
	assertReportMoneyDefaults(t, gotBody)
}

func TestSummaryReportUsesCurrentUserSettingsAsDefaults(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{"id": "u1", "settings": map[string]any{"timeZone": "Europe/Belgrade", "weekStart": "MONDAY"}})
		case r.URL.Path == "/workspaces/ws1/reports/summary" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode summary body: %v", err)
			}
			respondJSON(t, w, map[string]any{"totals": []map[string]any{{"entriesCount": 1}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.SummaryReport(context.Background(), map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-08T00:00:00Z",
		"summary_filter": map[string]any{
			"groups": []any{"CLIENT"},
		},
	})
	if err != nil {
		t.Fatalf("summary report failed: %v", err)
	}
	if gotBody["timeZone"] != "Europe/Belgrade" || gotBody["weekStart"] != "MONDAY" {
		t.Fatalf("user defaults not applied: %#v", gotBody)
	}
	defaults, _ := result.Meta["defaults"].(map[string]any)
	if defaults["default_source"] != "user_settings" {
		t.Fatalf("defaults meta missing user_settings source: %#v", result.Meta)
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
	if !strings.Contains(err.Error(), "clockify_tools_guide") {
		t.Fatalf("expected tools guide recovery, got %v", err)
	}
}

func TestFindAndUpdateEntryDryRunIncludesMatchedIdentityAndProposedChanges(t *testing.T) {
	var putCount int
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u-self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/entry-1" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "entry-1",
				UserID:      "u-self",
				Description: "Old description",
				ProjectID:   "p-old",
				TaskID:      "t-old",
				TagIDs:      []string{"tag-old"},
				Billable:    false,
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-01T09:00:00Z",
					End:   "2026-04-01T10:00:00Z",
				},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/entry-1" && r.Method == http.MethodPut:
			putCount++
			t.Fatal("PUT must not run for find-and-update dry-run")
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"entry_id":        "entry-1",
		"new_description": "New description",
		"project_id":      "p-new",
		"task_id":         "t-new",
		"tag_ids":         []any{"tag-new"},
		"start":           "2026-04-01T09:15:00Z",
		"end":             "2026-04-01T10:15:00Z",
		"billable":        true,
		"dry_run":         true,
	})
	if err != nil {
		t.Fatalf("find-and-update dry-run failed: %v", err)
	}
	if putCount != 0 {
		t.Fatalf("PUT must not run for dry-run, got %d calls", putCount)
	}
	env, ok := result.(ResultEnvelope)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	data, ok := env.Data.(FindAndUpdateEntryData)
	if !ok {
		t.Fatalf("unexpected data type: %T", env.Data)
	}
	if data.MatchedEntryID != "entry-1" {
		t.Fatalf("matched_entry_id = %q", data.MatchedEntryID)
	}
	if data.Current == nil || data.Current.Description != "Old description" || data.Current.ProjectID != "p-old" ||
		data.Current.TaskID != "t-old" || len(data.Current.TagIDs) != 1 || data.Current.TagIDs[0] != "tag-old" ||
		data.Current.Start != "2026-04-01T09:00:00Z" || data.Current.End != "2026-04-01T10:00:00Z" {
		t.Fatalf("unexpected current preview: %+v", data.Current)
	}
	if data.Proposed["description"] != "New description" || data.Proposed["project_id"] != "p-new" ||
		data.Proposed["task_id"] != "t-new" ||
		data.Proposed["start"] != "2026-04-01T09:15:00Z" || data.Proposed["end"] != "2026-04-01T10:15:00Z" ||
		data.Proposed["billable"] != true {
		t.Fatalf("unexpected proposed changes: %+v", data.Proposed)
	}
	if proposedTags, ok := data.Proposed["tag_ids"].([]string); !ok || len(proposedTags) != 1 || proposedTags[0] != "tag-new" {
		t.Fatalf("unexpected proposed tag_ids: %+v", data.Proposed)
	}
	if !data.DryRun || data.Note == "" {
		t.Fatalf("expected dry-run note, got %+v", data)
	}
	if data.Entry.Description != "New description" || data.Entry.ProjectID != "p-new" ||
		data.Entry.TaskID != "t-new" || len(data.Entry.TagIDs) != 1 || data.Entry.TagIDs[0] != "tag-new" ||
		data.Entry.TimeInterval.Start != "2026-04-01T09:15:00Z" || data.Entry.TimeInterval.End != "2026-04-01T10:15:00Z" ||
		!data.Entry.Billable {
		t.Fatalf("entry should show proposed state, got %+v", data.Entry)
	}
}

func TestLogTimeCreatesFinishedEntry(t *testing.T) {
	var postBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/time-entries":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if postBody["start"] != "2026-04-01T09:00:00Z" || postBody["end"] != "2026-04-01T10:30:00Z" {
				t.Fatalf("unexpected body: %+v", postBody)
			}
			respondJSON(t, w, clockify.TimeEntry{ID: "e1", Description: "Focus", ProjectID: "p1", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:30:00Z"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.LogTime(context.Background(), map[string]any{
		"project_id":    "p1",
		"description":   "Focus",
		"start":         "2026-04-01T09:00:00Z",
		"end":           "2026-04-01T10:30:00Z",
		"tag_ids":       []any{"tag1", "tag2"},
		"billable":      true,
		"allow_overlap": true,
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
	tagIDs, ok := postBody["tagIds"].([]any)
	if !ok || len(tagIDs) != 2 || tagIDs[0] != "tag1" || tagIDs[1] != "tag2" {
		t.Fatalf("expected tagIds in log-time payload, got %#v", postBody["tagIds"])
	}
}

func TestLogTimeUsesServiceDefaultTimezoneForFlexibleTimes(t *testing.T) {
	var postBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/time-entries":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, clockify.TimeEntry{ID: "e1"})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	loc, err := time.LoadLocation("Europe/Belgrade")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	svc := New(client, "ws1")
	svc.DefaultTimezone = loc
	_, err = svc.LogTime(context.Background(), map[string]any{
		"description":   "Focus",
		"start":         "2026-04-01 09:00",
		"end":           "2026-04-01 10:00",
		"allow_overlap": true,
	})
	if err != nil {
		t.Fatalf("log time failed: %v", err)
	}
	if postBody["start"] != "2026-04-01T07:00:00Z" || postBody["end"] != "2026-04-01T08:00:00Z" {
		t.Fatalf("expected Europe/Belgrade local times to convert to UTC, got %+v", postBody)
	}
}

func TestLogTimeAcceptsFlexibleTimesAndScansForOverlap(t *testing.T) {
	var postBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			q := r.URL.Query()
			if q.Get("start") != "2026-03-31T09:00:00Z" || q.Get("end") != "2026-04-01T10:00:00Z" {
				t.Fatalf("unexpected overlap scan query: %s", r.URL.RawQuery)
			}
			if q.Get("page") != "1" || q.Get("page-size") != "200" {
				t.Fatalf("unexpected overlap scan pagination: %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []clockify.TimeEntry{})
		case r.URL.Path == "/workspaces/ws1/time-entries" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "e1",
				Description: "Focus",
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
	_, err := svc.LogTime(context.Background(), map[string]any{
		"description": "Focus",
		"start":       "2026-04-01 09:00",
		"end":         "2026-04-01 10:00",
		"timezone":    "UTC",
	})
	if err != nil {
		t.Fatalf("log time failed: %v", err)
	}
	if postBody["start"] != "2026-04-01T09:00:00Z" || postBody["end"] != "2026-04-01T10:00:00Z" {
		t.Fatalf("unexpected normalized body: %+v", postBody)
	}
}

func TestLogTimeRejectsOverlapUnlessAllowed(t *testing.T) {
	var postCount int
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{{
				ID:          "existing1",
				Description: "Existing",
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-01T09:30:00Z",
					End:   "2026-04-01T10:30:00Z",
				},
			}})
		case r.URL.Path == "/workspaces/ws1/time-entries" && r.Method == http.MethodPost:
			postCount++
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "new1",
				Description: "Focus",
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
	args := map[string]any{
		"description": "Focus",
		"start":       "2026-04-01T09:00:00Z",
		"end":         "2026-04-01T10:00:00Z",
	}
	_, err := svc.LogTime(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "overlaps 1 existing entry") {
		t.Fatalf("expected overlap rejection, got %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST must not run after overlap rejection, got %d calls", postCount)
	}

	args["allow_overlap"] = true
	if _, err := svc.LogTime(context.Background(), args); err != nil {
		t.Fatalf("log time with allow_overlap failed: %v", err)
	}
	if postCount != 1 {
		t.Fatalf("expected one POST after allow_overlap, got %d", postCount)
	}
}

func TestLogTimeDryRunReportsOverlapWithoutPosting(t *testing.T) {
	var postCount int
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{{
				ID:          "existing1",
				Description: "Existing",
				ProjectID:   "p1",
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-01T09:30:00Z",
					End:   "2026-04-01T10:30:00Z",
				},
			}})
		case r.URL.Path == "/workspaces/ws1/time-entries" && r.Method == http.MethodPost:
			postCount++
			t.Fatal("POST must not run for an overlapping dry-run")
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.LogTime(context.Background(), map[string]any{
		"project_id":  "p1",
		"description": "Focus",
		"start":       "2026-04-01T09:00:00Z",
		"end":         "2026-04-01T10:00:00Z",
		"dry_run":     true,
	})
	if err != nil {
		t.Fatalf("dry-run log time should return a block preview, got error: %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST must not run for dry-run, got %d calls", postCount)
	}
	env, ok := result.(ResultEnvelope)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected dry-run data type: %T", env.Data)
	}
	if data["blocked"] != true {
		t.Fatalf("expected blocked dry-run preview, got %+v", data)
	}
	if warning, _ := data["warning"].(string); !strings.Contains(warning, "overlaps 1 existing entry") {
		t.Fatalf("expected overlap warning, got %q", warning)
	}
	overlaps, ok := data["overlaps"].([]TimeEntryRef)
	if !ok || len(overlaps) != 1 || overlaps[0].ID != "existing1" {
		t.Fatalf("unexpected overlaps in dry-run preview: %#v", data["overlaps"])
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
	if result.Action != "clockify_entries_get" {
		t.Fatalf("expected action clockify_entries_get, got %s", result.Action)
	}
	entry, ok := result.Data.(EntryView)
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
	if result.Action != "clockify_entries_list" {
		t.Fatalf("expected action clockify_entries_list, got %s", result.Action)
	}
	entries, ok := result.Data.([]EntryView)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestTodayEntriesPaginationSanitizesBounds(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		wantPage     string
		wantPageSize string
		wantMetaPage int
		wantMetaSize int
	}{
		{
			name:         "floors invalid values",
			args:         map[string]any{"page": 0, "page_size": 0},
			wantPage:     "1",
			wantPageSize: "50",
			wantMetaPage: 1,
			wantMetaSize: 50,
		},
		{
			name:         "caps page size",
			args:         map[string]any{"page": 3, "page_size": 9999},
			wantPage:     "3",
			wantPageSize: "200",
			wantMetaPage: 3,
			wantMetaSize: 200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/user":
					respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
				case "/workspaces/ws1/user/u1/time-entries":
					q := r.URL.Query()
					if q.Get("page") != tt.wantPage || q.Get("page-size") != tt.wantPageSize {
						t.Fatalf("expected page=%s page-size=%s, got %s", tt.wantPage, tt.wantPageSize, r.URL.RawQuery)
					}
					respondJSON(t, w, []clockify.TimeEntry{})
				default:
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
			})
			defer cleanup()

			svc := New(client, "ws1")
			result, err := svc.TodayEntries(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("today entries failed: %v", err)
			}
			if result.Meta["page"] != tt.wantMetaPage || result.Meta["pageSize"] != tt.wantMetaSize {
				t.Fatalf("unexpected pagination meta: %+v", result.Meta)
			}
			pagination, ok := result.Meta["pagination"].(map[string]any)
			if !ok {
				t.Fatalf("expected structured pagination meta, got %+v", result.Meta)
			}
			if pagination["requested_page_size"] != tt.args["page_size"] || pagination["applied_page_size"] != tt.wantMetaSize || pagination["clamped"] != true {
				t.Fatalf("unexpected structured pagination meta: %+v", pagination)
			}
		})
	}
}

func TestEntriesCreate(t *testing.T) {
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
	res, err := svc.EntriesCreate(context.Background(), map[string]any{
		"start":       "2026-04-06T09:00:00Z",
		"end":         "2026-04-06T10:00:00Z",
		"description": "New task",
		"project_id":  "p1",
		"billable":    true,
	})
	if err != nil {
		t.Fatalf("EntriesCreate failed: %v", err)
	}
	tr, ok := res.(ToolResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", res)
	}
	if !tr.OK || tr.Action != oneUserToolEntriesCreate {
		t.Fatalf("unexpected envelope: ok=%v action=%s", tr.OK, tr.Action)
	}
	if tr.IDs["entryId"] != "new1" {
		t.Fatalf("unexpected entryId: %q", tr.IDs["entryId"])
	}
	entry, ok := tr.Data.(clockify.TimeEntry)
	if !ok {
		t.Fatalf("unexpected data type: %T", tr.Data)
	}
	if entry.ID != "new1" {
		t.Fatalf("unexpected entry ID: %s", entry.ID)
	}
}

// TestEntriesCreatePassesType pins the QA-agent-18 fix on the live tool:
// when `type` is supplied (REGULAR or BREAK) the upstream POST body must
// carry it. Without the wiring the parameter is silently ignored and the
// API defaults every entry to REGULAR, leaving callers unable to record
// breaks. The fake upstream fails if the field is missing.
func TestEntriesCreatePassesType(t *testing.T) {
	var gotType any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/time-entries" || r.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		gotType = body["type"]
		respondJSON(t, w, clockify.TimeEntry{ID: "br1", Type: "BREAK"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.EntriesCreate(context.Background(), map[string]any{
		"start":       "2026-04-06T09:00:00Z",
		"end":         "2026-04-06T09:15:00Z",
		"description": "Coffee",
		"project_id":  "p1",
		"type":        "BREAK",
	})
	if err != nil {
		t.Fatalf("EntriesCreate: %v", err)
	}
	if gotType != "BREAK" {
		t.Fatalf("upstream POST body.type = %#v, want \"BREAK\"", gotType)
	}
}

func TestUpdateEntryFetchThenPut(t *testing.T) {
	var gotPutBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u-self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/abc123def456789012345678" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "abc123def456789012345678",
				UserID:      "u-self",
				Description: "Old description",
				ProjectID:   "p1",
				TaskID:      "task1",
				TagIDs:      []string{"tag1", "tag2"},
				CustomFieldValues: []map[string]any{{
					"customFieldId": "cf1",
					"value":         "kept",
				}},
				Billable: false,
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
		"task_id":     "task2",
		"tag_ids":     []any{"tag3"},
		"billable":    true,
	})
	if err != nil {
		t.Fatalf("update entry failed: %v", err)
	}
	if result.Action != "clockify_entries_update" {
		t.Fatalf("expected action clockify_entries_update, got %s", result.Action)
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
	if gotPutBody["taskId"] != "task2" {
		t.Fatalf("PUT should update taskId, got %v", gotPutBody["taskId"])
	}
	tagIDs, ok := gotPutBody["tagIds"].([]any)
	if !ok || len(tagIDs) != 1 || tagIDs[0] != "tag3" {
		t.Fatalf("PUT should replace tagIds, got %#v", gotPutBody["tagIds"])
	}
	if _, ok := gotPutBody["customFields"]; !ok {
		t.Fatalf("PUT should preserve custom fields, got body %#v", gotPutBody)
	}
	// Verify changedFields in meta
	changedFields, ok := result.Meta["changedFields"].([]string)
	if !ok {
		t.Fatalf("expected changedFields in meta, got %T", result.Meta["changedFields"])
	}
	if len(changedFields) != 4 {
		t.Fatalf("expected 4 changed fields, got %d: %v", len(changedFields), changedFields)
	}
}

func TestUpdateEntryProjectResolutionGuidesResolveName(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u-self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/abc123def456789012345678" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "abc123def456789012345678",
				UserID:      "u-self",
				Description: "Old description",
				ProjectID:   "p1",
				TimeInterval: clockify.TimeInterval{
					Start: "2026-04-01T09:00:00Z",
					End:   "2026-04-01T10:00:00Z",
				},
			})
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{
				{"id": "p1", "name": "Alpha"},
				{"id": "p2", "name": "Alpha"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/abc123def456789012345678" && r.Method == http.MethodPut:
			t.Fatalf("PUT should not run when project resolution is ambiguous")
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id": "abc123def456789012345678",
		"project":  "Alpha",
	})
	if err == nil || !strings.Contains(err.Error(), "multiple projects match") {
		t.Fatalf("expected ambiguous project error, got %v", err)
	}
	if !strings.Contains(err.Error(), "clockify_tools_guide") {
		t.Fatalf("expected tools guide recovery, got %v", err)
	}
}

func TestDeleteEntryDryRun(t *testing.T) {
	var deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u-self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/abc123def456789012345678" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:          "abc123def456789012345678",
				UserID:      "u-self",
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
	if result.Action != "clockify_entries_delete" {
		t.Fatalf("expected action clockify_entries_delete, got %s", result.Action)
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

func TestWhoAmI(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Alice Smith", Email: "alice@example.com"})
		case "/workspaces":
			respondJSON(t, w, []clockify.Workspace{{ID: "ws1", Name: "My Workspace"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.WhoAmI(context.Background())
	if err != nil {
		t.Fatalf("WhoAmI failed: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok=true, got ok=false")
	}
	if result.Action != "clockify_status" {
		t.Fatalf("expected action clockify_whoami, got %s", result.Action)
	}
	data, ok := result.Data.(IdentityData)
	if !ok {
		t.Fatalf("expected IdentityData, got %T", result.Data)
	}
	if data.User.ID != "u1" {
		t.Fatalf("expected user ID u1, got %s", data.User.ID)
	}
	if data.User.Name != "Alice Smith" {
		t.Fatalf("expected user name Alice Smith, got %s", data.User.Name)
	}
	if data.User.Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", data.User.Email)
	}
	if data.WorkspaceID != "ws1" {
		t.Fatalf("expected workspace ID ws1, got %s", data.WorkspaceID)
	}
}

func TestListProjects(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws123/projects":
			respondJSON(t, w, []clockify.Project{
				{ID: "p1", Name: "Backend API", Color: "#0000FF", Archived: false},
				{ID: "p2", Name: "Frontend App", Color: "#FF0000", Archived: false},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws123")
	result, err := svc.ListProjects(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok=true")
	}
	if result.Action != "clockify_projects_list" {
		t.Fatalf("expected action clockify_projects_list, got %s", result.Action)
	}
	projects, ok := result.Data.([]ProjectView)
	if !ok {
		t.Fatalf("expected []ProjectView, got %T", result.Data)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].Name != "Backend API" {
		t.Fatalf("expected first project Backend API, got %s", projects[0].Name)
	}
	if projects[1].Name != "Frontend App" {
		t.Fatalf("expected second project Frontend App, got %s", projects[1].Name)
	}
	count, ok := result.Meta["count"].(int)
	if !ok || count != 2 {
		t.Fatalf("expected meta count=2, got %v", result.Meta["count"])
	}
}

func TestTimerStatus_NoRunning(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			// Return an entry with a non-empty End (finished, not running)
			respondJSON(t, w, []clockify.TimeEntry{
				{
					ID:          "e1",
					Description: "Finished task",
					TimeInterval: clockify.TimeInterval{
						Start: "2026-04-06T09:00:00Z",
						End:   "2026-04-06T10:00:00Z",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimerStatus(context.Background())
	if err != nil {
		t.Fatalf("TimerStatus failed: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok=true")
	}
	if result.Action != oneUserToolEntriesTimerStatus {
		t.Fatalf("expected action clockify_"+"timer_status, got %s", result.Action)
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	running, ok := dataMap["running"].(bool)
	if !ok || running {
		t.Fatalf("expected running=false, got %v", dataMap["running"])
	}
	elapsed, ok := dataMap["elapsed"].(string)
	if !ok || elapsed != "" {
		t.Fatalf("expected empty elapsed string, got %q", elapsed)
	}
}

func TestTimerStatus_Running(t *testing.T) {
	// Use a start time close to "now" so we get a valid elapsed calculation
	startTime := time.Now().UTC().Add(-35 * time.Minute).Format(time.RFC3339)

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			// Return an entry with empty End (running)
			respondJSON(t, w, []clockify.TimeEntry{
				{
					ID:          "e1",
					Description: "Active task",
					TimeInterval: clockify.TimeInterval{
						Start: startTime,
						End:   "", // empty = running
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TimerStatus(context.Background())
	if err != nil {
		t.Fatalf("TimerStatus failed: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok=true")
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	running, ok := dataMap["running"].(bool)
	if !ok || !running {
		t.Fatalf("expected running=true, got %v", dataMap["running"])
	}
	elapsed, ok := dataMap["elapsed"].(string)
	if !ok || elapsed == "" {
		t.Fatalf("expected non-empty elapsed string, got %q", elapsed)
	}
	// With 35 minutes elapsed, it should show something like "35m Xs"
	if !strings.Contains(elapsed, "m") {
		t.Fatalf("expected elapsed to contain minutes, got %q", elapsed)
	}
	// Verify the entry is in the result
	entry, ok := dataMap["entry"].(EntryView)
	if !ok {
		t.Fatalf("expected EntryView for entry field, got %T", dataMap["entry"])
	}
	if entry.ID != "e1" {
		t.Fatalf("expected entry ID e1, got %s", entry.ID)
	}
}

func TestHandlerAPIError(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message":"internal server error"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.WhoAmI(context.Background())
	if err == nil {
		t.Fatal("expected error from WhoAmI when API returns 500, got nil")
	}
	// Verify the error message includes the status info
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error to contain status code 500, got: %s", err.Error())
	}
}

// TestEntriesCreateDryRun verifies that dry_run:true returns a validated
// preview envelope without issuing any request to the Clockify API.
func TestEntriesCreateDryRun(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry_run must not issue any request; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.EntriesCreate(context.Background(), map[string]any{
		"start":       "2026-04-06T09:00:00Z",
		"end":         "2026-04-06T10:00:00Z",
		"description": "Planned work",
		"dry_run":     true,
	})
	if err != nil {
		t.Fatalf("EntriesCreate dry run failed: %v", err)
	}
	tr, ok := res.(ToolResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", res)
	}
	if !tr.OK || tr.Action != oneUserToolEntriesCreate {
		t.Fatalf("unexpected envelope: ok=%v action=%s", tr.OK, tr.Action)
	}
	if _, created := tr.IDs["entryId"]; created {
		t.Fatalf("dry run must not report a created entryId: %v", tr.IDs)
	}
	dataMap, ok := tr.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data for dry run, got %T", tr.Data)
	}
	if dataMap["dry_run"] != true {
		t.Fatalf("expected dry_run=true marker, got %+v", dataMap)
	}
	if dataMap["note"] == nil {
		t.Fatal("expected note in dry run preview")
	}
	validation, ok := dataMap["validation"].(ValidationView)
	if !ok || validation.Status != validationStatusOK {
		t.Fatalf("expected validation ok, got %#v", dataMap["validation"])
	}
}

// TestFindAndUpdateEntryHappyPath covers a single matching entry being updated
// via PUT, including verification of the updatedFields metadata.
func TestFindAndUpdateEntryHappyPath(t *testing.T) {
	var gotPutBody map[string]any
	var putCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "draft docs", ProjectID: "p1", ProjectName: "Docs", TaskID: "task1", TagIDs: []string{"tag1"}, TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}, Billable: false},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/e1" && r.Method == http.MethodPut:
			putCalled = true
			if err := json.NewDecoder(r.Body).Decode(&gotPutBody); err != nil {
				t.Fatalf("decode PUT body: %v", err)
			}
			respondJSON(t, w, clockify.TimeEntry{ID: "e1", Description: "Write docs", ProjectID: "p1", ProjectName: "Docs", Billable: true, TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"description_contains": "draft docs",
		"new_description":      "Write docs",
		"billable":             true,
	})
	if err != nil {
		t.Fatalf("find and update failed: %v", err)
	}
	if !putCalled {
		t.Fatal("expected PUT to be called")
	}

	env, ok := result.(ResultEnvelope)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	data, ok := env.Data.(FindAndUpdateEntryData)
	if !ok {
		t.Fatalf("unexpected data type: %T", env.Data)
	}
	if data.Entry.Description != "Write docs" {
		t.Fatalf("expected updated description, got %s", data.Entry.Description)
	}
	hasDesc, hasBillable := false, false
	for _, f := range data.UpdatedFields {
		switch f {
		case "description":
			hasDesc = true
		case "billable":
			hasBillable = true
		}
	}
	if !hasDesc || !hasBillable {
		t.Fatalf("expected updatedFields to include description and billable, got %v", data.UpdatedFields)
	}
	// PUT body must carry the merged fields
	if gotPutBody["description"] != "Write docs" {
		t.Fatalf("expected description in PUT body, got %+v", gotPutBody)
	}
	if gotPutBody["billable"] != true {
		t.Fatalf("expected billable=true in PUT body, got %+v", gotPutBody)
	}
	if gotPutBody["taskId"] != "task1" {
		t.Fatalf("PUT should preserve taskId, got %v", gotPutBody["taskId"])
	}
	findTagIDs, ok := gotPutBody["tagIds"].([]any)
	if !ok || len(findTagIDs) != 1 || findTagIDs[0] != "tag1" {
		t.Fatalf("PUT should preserve tagIds, got %#v", gotPutBody["tagIds"])
	}
}

func TestHandlerDryRunsUseResultEnvelope(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{})
		default:
			t.Fatalf("dry-run should not call upstream mutation or unknown read: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	cases := []struct {
		name string
		call func() (any, error)
	}{
		{
			name: oneUserToolLogWork,
			call: func() (any, error) {
				return svc.LogTime(context.Background(), map[string]any{
					"start":         "2026-04-01T09:00:00Z",
					"end":           "2026-04-01T10:00:00Z",
					"allow_overlap": true,
					"dry_run":       true,
				})
			},
		},
		{
			name: "clockify_entries_timer_stop",
			call: func() (any, error) {
				return svc.StopTimer(context.Background(), map[string]any{"dry_run": true})
			},
		},
		{
			name: oneUserToolEntriesTimerStart,
			call: func() (any, error) {
				return svc.StartTimerArgs(context.Background(), map[string]any{
					"project_id":  "123456789012345678901234",
					"description": "planned timer",
					"dry_run":     true,
				})
			},
		},
		{
			name: "clockify_projects_create",
			call: func() (any, error) {
				return svc.CreateProject(context.Background(), map[string]any{
					"name":    "Dry project",
					"dry_run": true,
				})
			},
		},
		{
			name: "clockify_clients_create",
			call: func() (any, error) {
				return svc.CreateClient(context.Background(), map[string]any{
					"name":    "Dry client",
					"dry_run": true,
				})
			},
		},
		{
			name: "clockify_tags_create",
			call: func() (any, error) {
				return svc.CreateTag(context.Background(), map[string]any{
					"name":    "Dry tag",
					"dry_run": true,
				})
			},
		},
		{
			name: "clockify_tasks_create",
			call: func() (any, error) {
				return svc.CreateTask(context.Background(), map[string]any{
					"project_id": "123456789012345678901234",
					"name":       "Dry task",
					"dry_run":    true,
				})
			},
		},
		{
			name: oneUserToolSwitchWork,
			call: func() (any, error) {
				return svc.SwitchProject(context.Background(), map[string]any{
					"project": "123456789012345678901234",
					"dry_run": true,
				})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.call()
			if err != nil {
				t.Fatalf("dry-run: %v", err)
			}
			env, ok := got.(ResultEnvelope)
			if !ok {
				t.Fatalf("dry-run result type = %T, want ResultEnvelope", got)
			}
			if !env.OK || env.Action != tc.name {
				t.Fatalf("dry-run envelope = %+v, want OK action %s", env, tc.name)
			}
		})
	}
}

func TestFindEntryOverlapsPadsLookupWindow(t *testing.T) {
	var gotStart string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1"})
		case "/workspaces/ws1/user/u1/time-entries":
			gotStart = r.URL.Query().Get("start")
			respondJSON(t, w, []clockify.TimeEntry{})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	start := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	if _, _, err := svc.findEntryOverlaps(context.Background(), start, end); err != nil {
		t.Fatalf("findEntryOverlaps: %v", err)
	}
	want := start.Add(-24 * time.Hour).Format(time.RFC3339)
	if gotStart != want {
		t.Fatalf("overlap lookup start = %q, want padded %q", gotStart, want)
	}
}

func TestAggregateEntriesRangeRequestsHydratedEntries(t *testing.T) {
	var gotHydrated string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1"})
		case "/workspaces/ws1/user/u1/time-entries":
			gotHydrated = r.URL.Query().Get("hydrated")
			respondJSON(t, w, []clockify.TimeEntry{})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, _, _, err := svc.aggregateEntriesRange(context.Background(), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), time.UTC, aggregateOptions{})
	if err != nil {
		t.Fatalf("aggregateEntriesRange: %v", err)
	}
	if gotHydrated != "true" {
		t.Fatalf("hydrated query = %q, want true", gotHydrated)
	}
}

func TestParseRangeSameBareDateEndMeansNextMidnight(t *testing.T) {
	start, end, err := parseRange(map[string]any{
		"start": "2026-05-04",
		"end":   "2026-05-04",
	})
	if err != nil {
		t.Fatalf("parseRange: %v", err)
	}
	if want := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Fatalf("start = %s, want %s", start, want)
	}
	if want := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC); !end.Equal(want) {
		t.Fatalf("end = %s, want %s", end, want)
	}
}

func TestParseRangeInLocationUsesLocalBareDateBoundary(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Belgrade")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	start, end, err := parseRangeInLocation(map[string]any{
		"start": "2026-05-04",
		"end":   "2026-05-04",
	}, loc)
	if err != nil {
		t.Fatalf("parseRangeInLocation: %v", err)
	}
	if want := time.Date(2026, 5, 3, 22, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Fatalf("start = %s, want %s", start, want)
	}
	if want := time.Date(2026, 5, 4, 22, 0, 0, 0, time.UTC); !end.Equal(want) {
		t.Fatalf("end = %s, want %s", end, want)
	}
}

func TestFindAndUpdateEntryPushesDescriptionFilter(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		wantQuery   string
		description string
	}{
		{
			name: "description_contains",
			args: map[string]any{
				"description_contains": " draft docs ",
				"new_description":      "Write docs",
			},
			wantQuery:   "draft docs",
			description: "draft docs",
		},
		{
			name: "exact_description",
			args: map[string]any{
				"exact_description": "large workspace target",
				"new_description":   "updated target",
			},
			wantQuery:   "large workspace target",
			description: "large workspace target",
		},
		{
			name: "exact_wins_when_both_set",
			args: map[string]any{
				"exact_description":    "broader exact target",
				"description_contains": "broader",
				"new_description":      "updated target",
			},
			wantQuery:   "broader exact target",
			description: "broader exact target",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery := ""
			client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/user":
					respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
				case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
					gotQuery = r.URL.Query().Get("description")
					respondJSON(t, w, []clockify.TimeEntry{
						{ID: "e1", Description: tt.description, ProjectID: "p1", ProjectName: "Docs", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}},
					})
				case r.URL.Path == "/workspaces/ws1/time-entries/e1" && r.Method == http.MethodPut:
					updated := clockify.TimeEntry{ID: "e1", Description: "updated target", ProjectID: "p1", ProjectName: "Docs", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}}
					if v, _ := tt.args["new_description"].(string); v != "" {
						updated.Description = v
					}
					respondJSON(t, w, updated)
				default:
					t.Fatalf("unexpected %s %s", r.Method, r.URL.String())
				}
			})
			defer cleanup()

			svc := New(client, "ws1")
			if _, err := svc.FindAndUpdateEntry(context.Background(), tt.args); err != nil {
				t.Fatalf("find and update failed: %v", err)
			}
			if gotQuery != tt.wantQuery {
				t.Fatalf("description query=%q, want %q", gotQuery, tt.wantQuery)
			}
		})
	}
}

func TestFindAndUpdateEntryAcceptsFlexibleDateFiltersAndUpdateTimes(t *testing.T) {
	var gotQueryStart, gotQueryEnd string
	var gotPutBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			gotQueryStart = r.URL.Query().Get("start")
			gotQueryEnd = r.URL.Query().Get("end")
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", Description: "move me", ProjectID: "p1", ProjectName: "Docs", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/e1" && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&gotPutBody); err != nil {
				t.Fatalf("decode PUT body: %v", err)
			}
			respondJSON(t, w, clockify.TimeEntry{ID: "e1", Description: "move me", ProjectID: "p1", ProjectName: "Docs", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T10:00:00Z", End: "2026-04-01T11:00:00Z"}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"exact_description": "move me",
		"start_after":       "2026-04-01",
		"start_before":      "2026-04-02",
		"start":             "2026-04-01 10:00",
		"end":               "2026-04-01 11:00",
		"timezone":          "UTC",
	})
	if err != nil {
		t.Fatalf("find and update failed: %v", err)
	}
	if gotQueryStart != "2026-04-01T00:00:00Z" || gotQueryEnd != "2026-04-02T00:00:00Z" {
		t.Fatalf("unexpected normalized search query start=%q end=%q", gotQueryStart, gotQueryEnd)
	}
	if gotPutBody["start"] != "2026-04-01T10:00:00Z" || gotPutBody["end"] != "2026-04-01T11:00:00Z" {
		t.Fatalf("unexpected normalized PUT body: %+v", gotPutBody)
	}
}

func TestFindAndUpdateEntryFindsMatchBeyondFirstPage(t *testing.T) {
	firstPage := make([]clockify.TimeEntry, 200)
	for i := range firstPage {
		firstPage[i] = clockify.TimeEntry{
			ID:          "filler",
			Description: "ordinary work",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-01T09:00:00Z",
				End:   "2026-04-01T09:05:00Z",
			},
		}
	}
	match := clockify.TimeEntry{
		ID:          "target",
		Description: "large workspace target",
		ProjectID:   "p1",
		ProjectName: "Project",
		TimeInterval: clockify.TimeInterval{
			Start: "2026-04-02T09:00:00Z",
			End:   "2026-04-02T10:00:00Z",
		},
	}
	var putCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			if got := r.URL.Query().Get("page-size"); got != "200" {
				t.Fatalf("expected page-size=200, got %q", got)
			}
			switch r.URL.Query().Get("page") {
			case "1":
				respondJSON(t, w, firstPage)
			case "2":
				respondJSON(t, w, []clockify.TimeEntry{match})
			default:
				respondJSON(t, w, []clockify.TimeEntry{})
			}
		case r.URL.Path == "/workspaces/ws1/time-entries/target" && r.Method == http.MethodPut:
			putCalled = true
			updated := match
			updated.Description = "updated target"
			respondJSON(t, w, updated)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"exact_description": "large workspace target",
		"new_description":   "updated target",
	})
	if err != nil {
		t.Fatalf("find and update failed: %v", err)
	}
	if !putCalled {
		t.Fatal("expected PUT to be called")
	}
	env := result.(ResultEnvelope)
	data := env.Data.(FindAndUpdateEntryData)
	if data.MatchedBy["pagesFetched"] != 2 || data.MatchedBy["entriesScanned"] != 201 {
		t.Fatalf("unexpected paginated match metadata: %+v", data.MatchedBy)
	}
}

func TestFindAndUpdateEntryDetectsAmbiguousMatchAcrossPages(t *testing.T) {
	firstPage := make([]clockify.TimeEntry, 200)
	firstPage[0] = clockify.TimeEntry{
		ID:          "match-1",
		Description: "duplicate target",
		TimeInterval: clockify.TimeInterval{
			Start: "2026-04-01T09:00:00Z",
			End:   "2026-04-01T09:30:00Z",
		},
	}
	for i := 1; i < len(firstPage); i++ {
		firstPage[i] = clockify.TimeEntry{
			ID:          "filler",
			Description: "ordinary work",
			TimeInterval: clockify.TimeInterval{
				Start: "2026-04-01T10:00:00Z",
				End:   "2026-04-01T10:05:00Z",
			},
		}
	}
	secondMatch := clockify.TimeEntry{
		ID:          "match-2",
		Description: "duplicate target",
		TimeInterval: clockify.TimeInterval{
			Start: "2026-04-02T09:00:00Z",
			End:   "2026-04-02T09:30:00Z",
		},
	}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			switch r.URL.Query().Get("page") {
			case "1":
				respondJSON(t, w, firstPage)
			case "2":
				respondJSON(t, w, []clockify.TimeEntry{secondMatch})
			default:
				respondJSON(t, w, []clockify.TimeEntry{})
			}
		case strings.HasPrefix(r.URL.Path, "/workspaces/ws1/time-entries/") && r.Method == http.MethodPut:
			t.Fatalf("PUT should not run for ambiguous matches")
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"exact_description": "duplicate target",
		"new_description":   "updated",
	})
	if err == nil || !strings.Contains(err.Error(), "multiple entries matched") {
		t.Fatalf("expected ambiguous match error, got %v", err)
	}
	if !strings.Contains(err.Error(), "clockify_tools_guide") {
		t.Fatalf("expected tools guide recovery, got %v", err)
	}
}

func TestFindAndUpdateEntryWithEntryIDFetchesDirectly(t *testing.T) {
	entryID := "abc123def456789012345678"
	entry := clockify.TimeEntry{
		ID:          entryID,
		UserID:      "u-self",
		Description: "direct target",
		ProjectID:   "p1",
		TimeInterval: clockify.TimeInterval{
			Start: "2026-04-01T09:00:00Z",
			End:   "2026-04-01T10:00:00Z",
		},
	}
	var gotDirectGet, gotPut bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/time-entries/"+entryID && r.Method == http.MethodGet:
			gotDirectGet = true
			respondJSON(t, w, entry)
		case r.URL.Path == "/workspaces/ws1/time-entries/"+entryID && r.Method == http.MethodPut:
			gotPut = true
			updated := entry
			updated.Description = "direct update"
			respondJSON(t, w, updated)
		case strings.Contains(r.URL.Path, "/user/"):
			t.Fatalf("entry_id lookup should not scan current-user list pages: %s", r.URL.Path)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	// Pre-prime the current-user cache so the ownership guard does not
	// HTTP-fetch /user (which is allowed but unnecessary here). The
	// test's intent is that the entry_id branch must not paginate
	// /workspaces/{ws}/user/{userID}/time-entries — the bare /user
	// lookup is orthogonal.
	svc.cachedUser = &clockify.User{ID: "u-self"}
	result, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"entry_id":        entryID,
		"new_description": "direct update",
	})
	if err != nil {
		t.Fatalf("find and update by entry_id failed: %v", err)
	}
	if !gotDirectGet || !gotPut {
		t.Fatalf("expected direct GET and PUT, got get=%v put=%v", gotDirectGet, gotPut)
	}
	env := result.(ResultEnvelope)
	data := env.Data.(FindAndUpdateEntryData)
	if data.MatchedBy["entryId"] != entryID {
		t.Fatalf("expected matched entryId, got %+v", data.MatchedBy)
	}
}

// TestListClientsPagination verifies that page and page_size args are forwarded
// to the Clockify API as query parameters.
func TestListClientsPagination(t *testing.T) {
	var gotPage, gotPageSize string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/clients" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotPage = r.URL.Query().Get("page")
		gotPageSize = r.URL.Query().Get("page-size")
		respondJSON(t, w, []clockify.ClientEntity{{ID: "c1", Name: "Acme"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListClients(context.Background(), map[string]any{
		"page":      2,
		"page_size": 100,
	})
	if err != nil {
		t.Fatalf("list clients failed: %v", err)
	}
	if gotPage != "2" || gotPageSize != "100" {
		t.Fatalf("expected page=2 page-size=100, got page=%s page-size=%s", gotPage, gotPageSize)
	}
	meta := result.Meta
	if meta["page"] != 2 || meta["pageSize"] != 100 {
		t.Fatalf("expected meta page=2 pageSize=100, got %+v", meta)
	}
}

// TestListClientsPageSizeCap ensures page_size is capped at 200.
func TestListClientsPageSizeCap(t *testing.T) {
	var gotPageSize string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPageSize = r.URL.Query().Get("page-size")
		respondJSON(t, w, []clockify.ClientEntity{})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListClients(context.Background(), map[string]any{"page_size": 9999})
	if err != nil {
		t.Fatalf("list clients failed: %v", err)
	}
	if gotPageSize != "200" {
		t.Fatalf("expected page-size capped at 200, got %s", gotPageSize)
	}
	pagination, ok := result.Meta["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured pagination meta, got %+v", result.Meta)
	}
	if pagination["requested_page_size"] != 9999 || pagination["applied_page_size"] != 200 || pagination["clamped"] != true {
		t.Fatalf("unexpected structured pagination meta: %+v", pagination)
	}
}

// TestListTags verifies default pagination (page=1, page_size=50).
func TestListTags(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("page-size") != "50" {
			t.Fatalf("expected default pagination, got %s", r.URL.RawQuery)
		}
		respondJSON(t, w, []clockify.Tag{{ID: "t1", Name: "urgent"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListTags(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tags failed: %v", err)
	}
	tags, ok := result.Data.([]clockify.Tag)
	if !ok || len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %+v", result.Data)
	}
}

// TestListTasks verifies that project ref is resolved and pagination applied.
func TestListTasks(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/ws1/projects":
			respondJSON(t, w, []map[string]any{{"id": "p1", "name": "MyProj"}})
		case "/workspaces/ws1/projects/p1/tasks":
			respondJSON(t, w, []clockify.Task{{ID: "tk1", Name: "Task A", ProjectID: "p1"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListTasks(context.Background(), map[string]any{"project": "MyProj"})
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if result.Action != "clockify_tasks_list" {
		t.Fatalf("unexpected action: %s", result.Action)
	}
	tasks, ok := result.Data.([]TaskView)
	if !ok || len(tasks) != 1 || tasks[0].ID != "tk1" {
		t.Fatalf("unexpected tasks: %+v", result.Data)
	}
}

// TestListTasksMissingProject verifies fail-closed on missing project arg.
func TestListTasksMissingProject(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected")
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.ListTasks(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

// TestListEntries verifies basic listing with date filters and pagination.
func TestListEntries(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/user/u1/time-entries":
			q := r.URL.Query()
			if q.Get("start") == "" || q.Get("end") == "" {
				t.Fatalf("expected start/end filters, got %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", ProjectID: "p1", ProjectName: "Alpha", TimeInterval: clockify.TimeInterval{Start: "2026-04-01T09:00:00Z", End: "2026-04-01T10:00:00Z"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListEntries(context.Background(), map[string]any{
		"start": "2026-04-01T00:00:00Z",
		"end":   "2026-04-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("list entries failed: %v", err)
	}
	if result.Action != "clockify_entries_list" {
		t.Fatalf("unexpected action: %s", result.Action)
	}
}

func TestListEntriesPaginationSanitizesBounds(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]any
		wantPage     string
		wantPageSize string
		wantMetaPage int
		wantMetaSize int
	}{
		{
			name:         "floors invalid values",
			args:         map[string]any{"page": -4, "page_size": -1},
			wantPage:     "1",
			wantPageSize: "50",
			wantMetaPage: 1,
			wantMetaSize: 50,
		},
		{
			name:         "caps page size",
			args:         map[string]any{"page": 2, "page_size": 9999},
			wantPage:     "2",
			wantPageSize: "200",
			wantMetaPage: 2,
			wantMetaSize: 200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/user":
					respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
				case "/workspaces/ws1/user/u1/time-entries":
					q := r.URL.Query()
					if q.Get("page") != tt.wantPage || q.Get("page-size") != tt.wantPageSize {
						t.Fatalf("expected page=%s page-size=%s, got %s", tt.wantPage, tt.wantPageSize, r.URL.RawQuery)
					}
					respondJSON(t, w, []clockify.TimeEntry{})
				default:
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
			})
			defer cleanup()

			svc := New(client, "ws1")
			result, err := svc.ListEntries(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("list entries failed: %v", err)
			}
			if result.Meta["page"] != tt.wantMetaPage || result.Meta["pageSize"] != tt.wantMetaSize {
				t.Fatalf("unexpected pagination meta: %+v", result.Meta)
			}
		})
	}
}

func TestListEntriesProjectFilterPushesResolvedName(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/projects":
			q := r.URL.Query()
			if q.Get("name") != "Alpha" || q.Get("strict-name-search") != "true" {
				t.Fatalf("expected strict project-name lookup for Alpha, got %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []map[string]any{{"id": "p-alpha", "name": "Alpha"}})
		case "/workspaces/ws1/user/u1/time-entries":
			q := r.URL.Query()
			if q.Get("page-size") != "200" {
				t.Fatalf("expected upstream page-size=200 for filtered scan, got %s", r.URL.RawQuery)
			}
			if q.Get("project") != "p-alpha" {
				t.Fatalf("expected resolved project=p-alpha upstream, got %s", r.URL.RawQuery)
			}
			if q.Get("page") != "1" {
				t.Fatalf("unexpected page for filtered scan: %s", r.URL.RawQuery)
			}
			respondJSON(t, w, []clockify.TimeEntry{{ID: "target", ProjectID: "p-alpha", ProjectName: "Alpha"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListEntries(context.Background(), map[string]any{"project": "Alpha"})
	if err != nil {
		t.Fatalf("list entries failed: %v", err)
	}
	entries, ok := result.Data.([]EntryView)
	if !ok {
		t.Fatalf("expected []EntryView, got %T", result.Data)
	}
	if len(entries) != 1 || entries[0].ID != "target" {
		t.Fatalf("expected target entry from resolved project filter, got %+v", entries)
	}
	if result.Meta["filteredCount"] != 1 || result.Meta["pagesFetched"] != 1 || result.Meta["entriesScanned"] != 1 || result.Meta["projectFilterResolvedId"] != "p-alpha" {
		t.Fatalf("unexpected filter metadata: %+v", result.Meta)
	}
}

func TestListEntriesProjectFilterPaginatesFilteredResults(t *testing.T) {
	const projectID = "abc123def456789012345678"
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case "/workspaces/ws1/projects":
			t.Fatalf("project ID filter should not resolve through /projects")
		case "/workspaces/ws1/user/u1/time-entries":
			if r.URL.Query().Get("page") != "1" {
				t.Fatalf("unexpected extra page for short filtered result: %s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("project") != projectID {
				t.Fatalf("expected project=%s upstream, got %s", projectID, r.URL.RawQuery)
			}
			respondJSON(t, w, []clockify.TimeEntry{
				{ID: "e1", ProjectID: projectID, ProjectName: "Alpha"},
				{ID: "e2", ProjectID: projectID, ProjectName: "Alpha"},
				{ID: "e3", ProjectID: projectID, ProjectName: "Alpha"},
				{ID: "e4", ProjectID: projectID, ProjectName: "Alpha"},
				{ID: "e5", ProjectID: projectID, ProjectName: "Alpha"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListEntries(context.Background(), map[string]any{"project": projectID, "page": 2, "page_size": 2})
	if err != nil {
		t.Fatalf("list entries failed: %v", err)
	}
	entries, ok := result.Data.([]EntryView)
	if !ok {
		t.Fatalf("expected []EntryView, got %T", result.Data)
	}
	if len(entries) != 2 || entries[0].ID != "e3" || entries[1].ID != "e4" {
		t.Fatalf("expected second page of filtered entries, got %+v", entries)
	}
	if result.Meta["count"] != 2 || result.Meta["filteredCount"] != 5 || result.Meta["page"] != 2 || result.Meta["pageSize"] != 2 {
		t.Fatalf("unexpected pagination metadata: %+v", result.Meta)
	}
	if result.Meta["projectFilterResolvedId"] != projectID {
		t.Fatalf("expected projectFilterResolvedId=%s, got %+v", projectID, result.Meta)
	}
}

// TestListUsersPagination covers the users handler and its pagination contract.
func TestListUsersPagination(t *testing.T) {
	var gotQuery url.Values
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/users" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotQuery = r.URL.Query()
		respondJSON(t, w, []clockify.User{{ID: "u1", Name: "Alice"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.ListUsers(context.Background(), map[string]any{
		"page":             3,
		"page_size":        25,
		"email":            "alice@example.test",
		"project_id":       "p1",
		"status":           "ACTIVE",
		"account_statuses": "ACTIVE,PENDING_EMAIL_VERIFICATION",
		"name":             "Alice",
		"sort_column":      "NAME",
		"sort_order":       "ASCENDING",
		"memberships":      "ALL",
		"include_roles":    false,
	})
	if err != nil {
		t.Fatalf("list users failed: %v", err)
	}
	want := map[string]string{
		"page":             "3",
		"page-size":        "25",
		"email":            "alice@example.test",
		"project-id":       "p1",
		"status":           "ACTIVE",
		"account-statuses": "ACTIVE,PENDING_EMAIL_VERIFICATION",
		"name":             "Alice",
		"sort-column":      "NAME",
		"sort-order":       "ASCENDING",
		"memberships":      "ALL",
		"include-roles":    "false",
	}
	for key, value := range want {
		if got := gotQuery.Get(key); got != value {
			t.Fatalf("expected %s=%q, got %q in query %s", key, value, got, gotQuery.Encode())
		}
	}
}

func TestListUsersSchemaExposesOpenAPIFilters(t *testing.T) {
	schema := userListSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected schema properties, got %T", schema["properties"])
	}
	for _, name := range []string{
		"email",
		"project_id",
		"status",
		"account_statuses",
		"name",
		"sort_column",
		"sort_order",
		"memberships",
		"include_roles",
		"page",
		"page_size",
	} {
		if _, ok := props[name]; !ok {
			t.Fatalf("expected schema property %q in %+v", name, props)
		}
	}
	for _, name := range []string{"include_roles"} {
		prop, ok := props[name].(map[string]any)
		if !ok {
			t.Fatalf("expected %s property map, got %T", name, props[name])
		}
		if prop["type"] != "boolean" {
			t.Fatalf("expected %s boolean schema, got %+v", name, prop)
		}
	}
}

func newTestClient(t testing.TB, handler http.HandlerFunc) (*clockify.Client, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	client := clockify.NewClient("test-key", ts.URL, 5*time.Second, 0)
	return client, ts.Close
}

func respondJSON(t testing.TB, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
