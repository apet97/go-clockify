package tools

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestEntityChangesSendsRepeatedTypesAndNormalizesTimeEntry(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/entities/updated" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query()["type"]; !reflect.DeepEqual(got, []string{"TIME_ENTRY", "PROJECTS"}) {
			t.Fatalf("type query = %#v, want repeated TIME_ENTRY/PROJECTS", got)
		}
		if r.URL.Query().Get("start") == "" || r.URL.Query().Get("end") == "" {
			t.Fatalf("missing range query: %s", r.URL.RawQuery)
		}
		respondJSON(t, w, []map[string]any{{
			"type":         "TIME_ENTRY",
			"entityId":     "e1",
			"documentCode": "entry-e1",
			"timeEntry": map[string]any{
				"id":                "e1",
				"description":       "Billable work",
				"duration":          3600,
				"approvalRequestId": "approval-1",
				"amounts":           []map[string]any{{"type": "EARNED", "value": 10000, "currency": "USD"}},
			},
		}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.entityChanges(context.Background(), map[string]any{
		"change_kind": "updated",
		"types":       []string{"TIME_ENTRY", "PROJECTS"},
		"start":       "2026-05-01T00:00:00Z",
		"end":         "2026-05-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("entity changes failed: %v", err)
	}
	data, ok := result.Data.(EntityChangesData)
	if !ok {
		t.Fatalf("data type = %T, want EntityChangesData", result.Data)
	}
	if len(data.Changes) != 1 {
		t.Fatalf("changes = %#v, want one", data.Changes)
	}
	entry, ok := data.Changes[0]["time_entry"].(ReportEntryView)
	if !ok || entry.ID != "e1" || entry.Approval == nil || entry.Approval.RequestID != "approval-1" {
		t.Fatalf("time entry view not normalized: %#v", data.Changes[0]["time_entry"])
	}
	if entry.Financials.Earned == nil || entry.Financials.Earned.AmountCents != 10000 {
		t.Fatalf("entry money not normalized: %#v", entry.Financials)
	}
}

func TestEntityChangesRejectsSingularProjectType(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()
	svc := New(client, "ws1")
	_, err := svc.entityChanges(context.Background(), map[string]any{
		"change_kind": "updated",
		"types":       []string{"PROJECT"},
		"start":       "2026-05-01T00:00:00Z",
		"end":         "2026-05-02T00:00:00Z",
	})
	if err == nil || !strings.Contains(err.Error(), "PROJECTS") {
		t.Fatalf("expected valid enum error, got %v", err)
	}
}
