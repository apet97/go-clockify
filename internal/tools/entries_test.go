package tools

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// TestListEntriesSameDayPlainDateCoversFullDay proves a bare same-day
// start==end range becomes a full-day window instead of a zero-width one that
// matches nothing.
func TestListEntriesSameDayPlainDateCoversFullDay(t *testing.T) {
	var gotQuery url.Values
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/user/user-SELF/time-entries" && r.Method == http.MethodGet:
			gotQuery = r.URL.Query()
			respondJSON(t, w, []clockify.TimeEntry{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.DefaultTimezone = time.UTC
	if _, err := svc.ListEntries(context.Background(), map[string]any{
		"start": "2026-05-17",
		"end":   "2026-05-17",
	}); err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if got := gotQuery.Get("start"); got != "2026-05-17T00:00:00Z" {
		t.Fatalf("start = %q, want 2026-05-17T00:00:00Z", got)
	}
	if got := gotQuery.Get("end"); got != "2026-05-17T23:59:59Z" {
		t.Fatalf("end = %q, want end-of-day 2026-05-17T23:59:59Z", got)
	}
}

// otherUserEntryID is a 24-char hex value the resolver treats as a
// valid Clockify ObjectID so the handler reaches the ownership guard
// rather than short-circuiting on resolve.ValidateID.
const otherUserEntryID = "6a00f6bc2568d3d293061e2a"

func TestBuildEntryPayloadForwardsCustomFields(t *testing.T) {
	svc := New(nil, "ws1")
	payload, ids, err := svc.buildEntryPayload(context.Background(), map[string]any{
		"start": "2026-05-21T09:00:00Z",
		"custom_fields": []any{
			map[string]any{"field_id": "6a00f6bc2568d3d293061e2a", "value": "Site A"},
			map[string]any{"customFieldId": "6a00f6bc2568d3d293061e2b", "value": float64(7)},
		},
	})
	if err != nil {
		t.Fatalf("buildEntryPayload: %v", err)
	}
	if ids["workspaceId"] != "ws1" {
		t.Fatalf("workspace id = %q, want ws1", ids["workspaceId"])
	}
	fields, ok := payload["customFields"].([]map[string]any)
	if !ok {
		t.Fatalf("customFields = %T %#v, want []map[string]any", payload["customFields"], payload["customFields"])
	}
	if len(fields) != 2 {
		t.Fatalf("customFields length = %d, want 2: %#v", len(fields), fields)
	}
	if fields[0]["customFieldId"] != "6a00f6bc2568d3d293061e2a" || fields[0]["value"] != "Site A" {
		t.Fatalf("first custom field not normalized: %#v", fields[0])
	}
	if fields[1]["customFieldId"] != "6a00f6bc2568d3d293061e2b" || fields[1]["value"] != float64(7) {
		t.Fatalf("second custom field not normalized: %#v", fields[1])
	}
	if _, ok := payload["custom_fields"]; ok {
		t.Fatalf("payload leaked user-facing custom_fields key: %#v", payload)
	}
}

func TestBuildEntryPayloadRejectsMalformedCustomFields(t *testing.T) {
	svc := New(nil, "ws1")
	_, _, err := svc.buildEntryPayload(context.Background(), map[string]any{
		"start":         "2026-05-21T09:00:00Z",
		"custom_fields": []any{map[string]any{"value": "Site A"}},
	})
	if err == nil || !strings.Contains(err.Error(), "custom_fields[0].field_id is required") {
		t.Fatalf("error = %v, want missing field_id guidance", err)
	}
}

func TestTimeEntryCreateSchemasExposeCustomFields(t *testing.T) {
	svc := New(nil, "ws1")
	schemas := map[string]map[string]any{
		"clockify_log_work":    logWorkSchema(),
		"clockify_start_work":  startWorkSchema(),
		"clockify_switch_work": switchWorkSchema(),
		"clockify_demo_seed":   demoSeedSchema(),
	}
	for _, descriptor := range svc.FullAccessRegistry() {
		switch descriptor.Tool.Name {
		case "clockify_entries_create", "clockify_entries_timer_start", "clockify_entries_timer_switch":
			schemas[descriptor.Tool.Name] = descriptor.Tool.InputSchema
		}
	}
	for name, schema := range schemas {
		props, _ := schema["properties"].(map[string]any)
		cf, _ := props["custom_fields"].(map[string]any)
		if cf == nil {
			t.Fatalf("%s schema missing custom_fields: %#v", name, props)
		}
		if cf["type"] != "array" {
			t.Fatalf("%s custom_fields type = %#v, want array", name, cf["type"])
		}
		items, _ := cf["items"].(map[string]any)
		itemProps, _ := items["properties"].(map[string]any)
		if itemProps["field_id"] == nil || itemProps["customFieldId"] == nil || itemProps["value"] == nil {
			t.Fatalf("%s custom_fields item schema missing id/value aliases: %#v", name, items)
		}
	}
}

func TestListEntriesForwardsOpenAPIQueryParams(t *testing.T) {
	var gotQuery url.Values
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/user/user-SELF/time-entries" && r.Method == http.MethodGet:
			gotQuery = r.URL.Query()
			respondJSON(t, w, []clockify.TimeEntry{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.ListEntries(context.Background(), map[string]any{
		"description":      "standup",
		"task":             "task-1",
		"tags":             []any{"tag-1", "tag-2"},
		"project_required": true,
		"task_required":    false,
		"hydrated":         true,
		"in_progress":      "false",
		"get_week_before":  "2026-W19",
		"page":             float64(2),
		"page_size":        float64(25),
	})
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}

	assertEntryQueryValue(t, gotQuery, "description", "standup")
	assertEntryQueryValue(t, gotQuery, "task", "task-1")
	assertEntryQueryValues(t, gotQuery, "tags", []string{"tag-1", "tag-2"})
	assertEntryQueryValue(t, gotQuery, "project-required", "true")
	assertEntryQueryValue(t, gotQuery, "task-required", "false")
	assertEntryQueryValue(t, gotQuery, "hydrated", "true")
	assertEntryQueryValue(t, gotQuery, "in-progress", "false")
	assertEntryQueryValue(t, gotQuery, "get-week-before", "2026-W19")
	assertEntryQueryValue(t, gotQuery, "page", "2")
	assertEntryQueryValue(t, gotQuery, "page-size", "25")
}

func TestGetEntryForwardsOpenAPIQueryParams(t *testing.T) {
	var gotQuery url.Values
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			gotQuery = r.URL.Query()
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-SELF",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.GetEntry(context.Background(), map[string]any{
		"entry_id":                 otherUserEntryID,
		"hydrated":                 true,
		"consider_duration_format": false,
	})
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}

	assertEntryQueryValue(t, gotQuery, "hydrated", "true")
	assertEntryQueryValue(t, gotQuery, "consider-duration-format", "false")
}

func TestUpdateEntryForwardsOpenAPIQueryParams(t *testing.T) {
	var getQuery url.Values
	var putQuery url.Values
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			getQuery = r.URL.Query()
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-SELF",
				Description:  "mine",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z", End: "2026-05-01T10:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodPut:
			putQuery = r.URL.Query()
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-SELF",
				Description:  "renamed",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z", End: "2026-05-01T10:00:00Z"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id":                 otherUserEntryID,
		"description":              "renamed",
		"hydrated":                 false,
		"consider_duration_format": true,
	})
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	for name, query := range map[string]url.Values{"GET": getQuery, "PUT": putQuery} {
		assertEntryQueryValue(t, query, "hydrated", "false")
		assertEntryQueryValue(t, query, "consider-duration-format", "true")
		if len(query) != 2 {
			t.Fatalf("%s query had unexpected extras: %v", name, query)
		}
	}
}

func assertEntryQueryValue(t *testing.T, query url.Values, key, want string) {
	t.Helper()
	got := query.Get(key)
	if got != want {
		t.Fatalf("query %q = %q, want %q; full query=%v", key, got, want, query)
	}
}

func assertEntryQueryValues(t *testing.T, query url.Values, key string, want []string) {
	t.Helper()
	got := query[key]
	if len(got) != len(want) {
		t.Fatalf("query %q = %v, want %v; full query=%v", key, got, want, query)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("query %q = %v, want %v; full query=%v", key, got, want, query)
		}
	}
}

// TestUpdateEntryRejectsOtherUserEntry pins the documented contract
// for personal time-entry mutations: docs/policy/production-tool-scope.md
// states "Mutations are constrained to the API key owner's own
// entries". `internal/tools/entries.go` UpdateEntry currently fetches
// the entry via the admin path /workspaces/{ws}/time-entries/{id}
// and never compares the returned UserID to the current user; with an
// elevated API key it would happily PUT another user's entry. This
// test seeds the fake so the fetched entry has UserID="user-OTHER"
// while /user reports "user-SELF", then asserts (1) the handler
// returns a permission-denied error and (2) no PUT is issued.
//
// Fails RED on this commit; goes GREEN when the ownership guard
// lands in UpdateEntry.
func TestUpdateEntryRejectsOtherUserEntry(t *testing.T) {
	var putCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-OTHER",
				Description:  "not mine",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodPut:
			putCalls.Add(1)
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id":    otherUserEntryID,
		"description": "rename hostile",
	})
	if err == nil {
		t.Fatal("expected ownership error; UpdateEntry permitted mutation across users")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not owned") &&
		!strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		t.Fatalf("expected ownership-flavored error, got %q", err.Error())
	}
	if got := putCalls.Load(); got != 0 {
		t.Fatalf("ownership guard must short-circuit before PUT; saw %d PUT call(s)", got)
	}
}

// TestUpdateEntryRejectsEntryWithoutUserID pins the fail-closed
// posture of requireCurrentUserEntry: an entry returned by GET
// without a userId is anomalous (the live Clockify API populates the
// field on every entry) and the handler must refuse to mutate rather
// than silently allow it. The previous shape — "skip ownership
// check when userId is empty" — would have left a quiet bypass surface
// if an upstream regression, malicious proxy, or future API change
// ever stripped the field.
func TestUpdateEntryRejectsEntryWithoutUserID(t *testing.T) {
	var putCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				Description:  "no userId on this stub",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodPut:
			putCalls.Add(1)
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id":    otherUserEntryID,
		"description": "should not apply",
	})
	if err == nil {
		t.Fatal("expected ownership error; UpdateEntry permitted mutation on entry with empty userId")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no userid") &&
		!strings.Contains(strings.ToLower(err.Error()), "ambiguous ownership") {
		t.Fatalf("expected ambiguous-ownership error, got %q", err.Error())
	}
	if got := putCalls.Load(); got != 0 {
		t.Fatalf("guard must short-circuit before PUT on missing userId; saw %d PUT call(s)", got)
	}
}

// TestUpdateEntryPermitsOwnEntryAndIssuesPUT is the positive path
// for the ownership guard: when the fetched entry's userId matches
// the current user, the guard must not interfere and the mutation
// must reach upstream. Without this test, a future "tighten
// everything" patch could break the guard into a permanent denial.
func TestUpdateEntryPermitsOwnEntryAndIssuesPUT(t *testing.T) {
	var putCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-SELF",
				Description:  "mine",
				ProjectID:    "p1",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z", End: "2026-05-01T10:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodPut:
			putCalls.Add(1)
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-SELF",
				Description:  "renamed",
				ProjectID:    "p1",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z", End: "2026-05-01T10:00:00Z"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id":    otherUserEntryID,
		"description": "renamed",
	})
	if err != nil {
		t.Fatalf("UpdateEntry on own entry must succeed, got %v", err)
	}
	if result.Action != "clockify_entries_update" {
		t.Fatalf("unexpected action %q", result.Action)
	}
	if got := putCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 PUT on own entry, got %d", got)
	}
}

// TestDeleteEntryRejectsOtherUserEntry mirrors UpdateEntry's pin for
// the destructive sibling. The DELETE must not be issued when the
// fetched entry belongs to a different user.
func TestDeleteEntryRejectsOtherUserEntry(t *testing.T) {
	var deleteCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-OTHER",
				Description:  "not mine",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodDelete:
			deleteCalls.Add(1)
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.DeleteEntry(context.Background(), map[string]any{
		"entry_id": otherUserEntryID,
	})
	if err == nil {
		t.Fatal("expected ownership error; DeleteEntry permitted mutation across users")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not owned") &&
		!strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		t.Fatalf("expected ownership-flavored error, got %q", err.Error())
	}
	if got := deleteCalls.Load(); got != 0 {
		t.Fatalf("ownership guard must short-circuit before DELETE; saw %d DELETE call(s)", got)
	}
}

// TestDeleteEntryRejectsEntryWithoutUserID mirrors UpdateEntry's
// fail-closed pin for the destructive sibling. The DELETE must not
// reach upstream when the fetched entry has no userId.
func TestDeleteEntryRejectsEntryWithoutUserID(t *testing.T) {
	var deleteCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				Description:  "no userId on this stub",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodDelete:
			deleteCalls.Add(1)
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.DeleteEntry(context.Background(), map[string]any{
		"entry_id": otherUserEntryID,
	})
	if err == nil {
		t.Fatal("expected ownership error; DeleteEntry permitted mutation on entry with empty userId")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no userid") &&
		!strings.Contains(strings.ToLower(err.Error()), "ambiguous ownership") {
		t.Fatalf("expected ambiguous-ownership error, got %q", err.Error())
	}
	if got := deleteCalls.Load(); got != 0 {
		t.Fatalf("guard must short-circuit before DELETE on missing userId; saw %d DELETE call(s)", got)
	}
}

// TestDeleteEntryPermitsOwnEntryAndIssuesDELETE is the positive path
// for the destructive guard: matching userId must allow the DELETE
// to reach upstream.
func TestDeleteEntryPermitsOwnEntryAndIssuesDELETE(t *testing.T) {
	var deleteCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-SELF",
				Description:  "mine to delete",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodDelete:
			deleteCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.DeleteEntry(context.Background(), map[string]any{
		"entry_id": otherUserEntryID,
	})
	if err != nil {
		t.Fatalf("DeleteEntry on own entry must succeed, got %v", err)
	}
	if got := deleteCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 DELETE on own entry, got %d", got)
	}
}
