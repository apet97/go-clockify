package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestAuditLogsSearchSchemaIsTyped(t *testing.T) {
	schema := auditLogsSearchSchema()
	required, _ := schema["required"].([]string)
	wantRequired := map[string]bool{"actions": true, "author_ids": true, "start": true, "end": true}
	if len(required) != len(wantRequired) {
		t.Fatalf("required = %v, want %d fields", required, len(wantRequired))
	}
	for _, field := range required {
		if !wantRequired[field] {
			t.Fatalf("unexpected required field %q", field)
		}
	}
	props, _ := schema["properties"].(map[string]any)
	actions, _ := props["actions"].(map[string]any)
	items, _ := actions["items"].(map[string]any)
	if enum, _ := items["enum"].([]string); len(enum) == 0 {
		t.Fatal("actions items must carry the audit-log action enum")
	}
	authorIDs, _ := props["author_ids"].(map[string]any)
	if mi, _ := authorIDs["minItems"].(int); mi != 1 {
		t.Fatalf("author_ids minItems = %v, want 1 (the field is required, so an empty array is an invalid state)", authorIDs["minItems"])
	}
}

func TestEntityChangesListSchemaIsTyped(t *testing.T) {
	schema := entityChangesListSchema()
	required, _ := schema["required"].([]string)
	wantRequired := map[string]bool{"change_type": true, "entity_types": true, "start": true, "end": true}
	if len(required) != len(wantRequired) {
		t.Fatalf("required = %v, want %v (start/end must be explicit)", required, wantRequired)
	}
	for _, field := range required {
		if !wantRequired[field] {
			t.Fatalf("unexpected required field %q", field)
		}
	}
	props, _ := schema["properties"].(map[string]any)
	changeType, _ := props["change_type"].(map[string]any)
	enum, _ := changeType["enum"].([]string)
	if len(enum) != 3 || enum[0] != "created" || enum[2] != "deleted" {
		t.Fatalf("change_type enum = %v, want created/updated/deleted", enum)
	}
	entityTypes, _ := props["entity_types"].(map[string]any)
	items, _ := entityTypes["items"].(map[string]any)
	if e, _ := items["enum"].([]string); len(e) == 0 {
		t.Fatal("entity_types items must carry the entity-type enum")
	}
}

func TestAuditLogsSearchPostsFilterBody(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"action":"CREATE_PROJECT","timestamp":"2026-05-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	svc := New(clockify.NewClient("k", ts.URL, 5*time.Second, 0), "65b382b606de527a7ee2b60e")
	svc.DefaultTimezone = time.UTC
	env, err := svc.AuditLogsSearch(context.Background(), map[string]any{
		"actions":    []any{"CREATE_PROJECT"},
		"author_ids": []any{"65b382b606de527a7ee2b622"},
		"start":      "2026-05-01",
		"end":        "2026-05-15",
	})
	if err != nil {
		t.Fatalf("AuditLogsSearch: %v", err)
	}
	if gotPath != "/workspaces/65b382b606de527a7ee2b60e/audit-log" {
		t.Fatalf("path = %s, want /workspaces/.../audit-log", gotPath)
	}
	authors, ok := gotBody["authors"].(map[string]any)
	if !ok || authors["contains"] != "CONTAINS" {
		t.Fatalf("authors body = %v, want default contains=CONTAINS", gotBody["authors"])
	}
	if gotBody["start"] != "2026-05-01T00:00:00Z" {
		t.Fatalf("start = %v, want RFC3339 UTC", gotBody["start"])
	}
	if gotBody["pageSize"] != float64(50) {
		t.Fatalf("pageSize body = %v, want 50", gotBody["pageSize"])
	}
	if !env.OK || env.Action != "clockify_audit_logs_search" {
		t.Fatalf("envelope = %+v", env)
	}
	if got, _ := env.Meta["count"].(int); got != 1 {
		t.Fatalf("meta count = %v, want 1", env.Meta["count"])
	}
	if _, exists := env.Meta["pageSize"]; exists {
		t.Fatalf("audit log meta must not report requested page_size as actual pageSize, got %+v", env.Meta)
	}
	if env.Meta["page"] != 1 || env.Meta["requested_page_size"] != 50 || env.Meta["total_is_lower_bound"] != true {
		t.Fatalf("audit log meta should expose requested_page_size and lower-bound total, got %+v", env.Meta)
	}
	note, _ := env.Meta["server_page_size_note"].(string)
	if !strings.Contains(note, "caps results") || !strings.Contains(note, "ignores page_size") {
		t.Fatalf("missing audit-log server cap note: %+v", env.Meta)
	}
}

func TestAuditLogsSearchRejectsMissingActions(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	if _, err := svc.AuditLogsSearch(context.Background(), map[string]any{
		"author_ids": []any{"u1"},
		"start":      "2026-05-01",
		"end":        "2026-05-15",
	}); err == nil {
		t.Fatal("expected error when actions is missing")
	}
}

func TestEntityChangesListBuildsTypeQuery(t *testing.T) {
	var gotPath, gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"x1","documentCode":"TIME_ENTRY"}]`))
	}))
	defer ts.Close()

	svc := New(clockify.NewClient("k", ts.URL, 5*time.Second, 0), "65b382b606de527a7ee2b60e")
	svc.DefaultTimezone = time.UTC
	env, err := svc.EntityChangesList(context.Background(), map[string]any{
		"change_type":  "created",
		"entity_types": []any{"TIME_ENTRY", "PROJECTS"},
		"start":        "2026-05-01",
		"end":          "2026-05-15",
	})
	if err != nil {
		t.Fatalf("EntityChangesList: %v", err)
	}
	if gotPath != "/workspaces/65b382b606de527a7ee2b60e/entities/created" {
		t.Fatalf("path = %s, want /workspaces/.../entities/created", gotPath)
	}
	if !strings.Contains(gotQuery, "type=TIME_ENTRY") || !strings.Contains(gotQuery, "type=PROJECTS") {
		t.Fatalf("query = %s, want repeated type params", gotQuery)
	}
	if !strings.Contains(gotQuery, "start=") || !strings.Contains(gotQuery, "end=") {
		t.Fatalf("query = %s, want explicit start/end params", gotQuery)
	}
	if !env.OK || env.Meta["changeType"] != "created" {
		t.Fatalf("envelope = %+v", env)
	}
	if got, _ := env.Meta["count"].(int); got != 1 {
		t.Fatalf("meta count = %v, want 1", env.Meta["count"])
	}
	if _, ok := env.Meta["has_more"]; !ok {
		t.Fatalf("meta missing has_more: %+v", env.Meta)
	}
	if env.Meta["page"] == nil || env.Meta["limit"] == nil {
		t.Fatalf("meta missing page/limit pagination signal: %+v", env.Meta)
	}
}

// TestAuditLogsSearchRejectsOverlongRange locks the client-side 31-day guard:
// Clockify's audit-log API hard-rejects wider windows with an opaque 400, so
// the tool fails early with a recovery instruction instead. [audit P3-1]
func TestAuditLogsSearchRejectsOverlongRange(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	svc.DefaultTimezone = time.UTC
	_, err := svc.AuditLogsSearch(context.Background(), map[string]any{
		"actions":    []any{"CREATE_PROJECT"},
		"author_ids": []any{"65b382b606de527a7ee2b622"},
		"start":      "2026-01-01",
		"end":        "2026-03-01",
	})
	if err == nil {
		t.Fatal("expected an error for a date range wider than 31 days")
	}
	if !strings.Contains(err.Error(), "31") {
		t.Fatalf("error should cite the 31-day maximum: %v", err)
	}
}

// TestEntityChangesListRequiresDateRange locks N-2: start/end are required so
// the call never silently fetches an unbounded entity-change history.
func TestEntityChangesListRequiresDateRange(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	svc.DefaultTimezone = time.UTC
	if _, err := svc.EntityChangesList(context.Background(), map[string]any{
		"change_type":  "created",
		"entity_types": []any{"TIME_ENTRY"},
	}); err == nil {
		t.Fatal("expected an error when start/end are omitted")
	}
}

func TestEntityChangesListRejectsBadChangeType(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	if _, err := svc.EntityChangesList(context.Background(), map[string]any{
		"change_type":  "modified",
		"entity_types": []any{"TIME_ENTRY"},
	}); err == nil {
		t.Fatal("expected error for invalid change_type")
	}
}
