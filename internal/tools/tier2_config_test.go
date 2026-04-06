package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"goclmcp/internal/clockify"
)

// ---------------------------------------------------------------------------
// Handler count tests
// ---------------------------------------------------------------------------

func TestCustomFieldHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs := customFieldHandlers(svc)
	if got := len(descs); got != 6 {
		t.Fatalf("expected 6 custom field handlers, got %d", got)
	}
}

func TestGroupsHolidaysHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs := groupsHolidaysHandlers(svc)
	if got := len(descs); got != 8 {
		t.Fatalf("expected 8 groups/holidays handlers, got %d", got)
	}
}

func TestProjectAdminHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs := projectAdminHandlers(svc)
	if got := len(descs); got != 6 {
		t.Fatalf("expected 6 project admin handlers, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Integration-style tests with mock HTTP server
// ---------------------------------------------------------------------------

func TestListCustomFields(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/custom-fields" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{
				{"id": "cf1", "name": "Priority", "type": "DROPDOWN"},
				{"id": "cf2", "name": "Notes", "type": "TEXT"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListCustomFields(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("list custom fields failed: %v", err)
	}
	if result.Action != "clockify_list_custom_fields" {
		t.Fatalf("expected action clockify_list_custom_fields, got %s", result.Action)
	}
	items, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 custom fields, got %d", len(items))
	}
	count, _ := result.Meta["count"].(int)
	if count != 2 {
		t.Fatalf("expected meta count=2, got %d", count)
	}
}

func TestCreateHoliday(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/holidays" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "New Year" {
				t.Fatalf("expected name 'New Year', got %v", body["name"])
			}
			if body["date"] != "2026-01-01" {
				t.Fatalf("expected date '2026-01-01', got %v", body["date"])
			}
			if body["recurring"] != true {
				t.Fatalf("expected recurring=true, got %v", body["recurring"])
			}
			respondJSON(t, w, map[string]any{
				"id":        "h1",
				"name":      "New Year",
				"date":      "2026-01-01",
				"recurring": true,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateHoliday(context.Background(), map[string]any{
		"name":      "New Year",
		"date":      "2026-01-01",
		"recurring": true,
	})
	if err != nil {
		t.Fatalf("create holiday failed: %v", err)
	}
	if result.Action != "clockify_create_holiday" {
		t.Fatalf("expected action clockify_create_holiday, got %s", result.Action)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["id"] != "h1" {
		t.Fatalf("expected holiday id h1, got %v", data["id"])
	}
}

func TestArchiveProjects(t *testing.T) {
	putPaths := map[string]bool{}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects/p1" && r.Method == http.MethodPut:
			putPaths["p1"] = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["archived"] != true {
				t.Fatalf("expected archived=true for p1, got %v", body["archived"])
			}
			respondJSON(t, w, map[string]any{"id": "p1", "archived": true})
		case r.URL.Path == "/workspaces/ws1/projects/p2" && r.Method == http.MethodPut:
			putPaths["p2"] = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["archived"] != true {
				t.Fatalf("expected archived=true for p2, got %v", body["archived"])
			}
			respondJSON(t, w, map[string]any{"id": "p2", "archived": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ArchiveProjects(context.Background(), map[string]any{
		"project_ids": []any{"p1", "p2"},
	})
	if err != nil {
		t.Fatalf("archive projects failed: %v", err)
	}
	if result.Action != "clockify_archive_projects" {
		t.Fatalf("expected action clockify_archive_projects, got %s", result.Action)
	}

	items, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 archive results, got %d", len(items))
	}
	for _, item := range items {
		if item["archived"] != true {
			t.Fatalf("expected archived=true, got %v for %v", item["archived"], item["projectId"])
		}
	}
	if !putPaths["p1"] || !putPaths["p2"] {
		t.Fatalf("expected PUT calls for both p1 and p2, got %v", putPaths)
	}
}
