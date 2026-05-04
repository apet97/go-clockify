package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSubmitForApprovalBodyUsesCamelCaseKeys(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "apr1", "status": "PENDING"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.submitForApproval(context.Background(), map[string]any{
		"period":       "WEEKLY",
		"period_start": "2026-05-04T00:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("submit for approval failed: %v", err)
	}
	if result.Action != "clockify_submit_for_approval" {
		t.Fatalf("expected submit action, got %q", result.Action)
	}
	if gotBody["period"] != "WEEKLY" {
		t.Fatalf("expected period WEEKLY, got %#v", gotBody)
	}
	if gotBody["periodStart"] != "2026-05-04T00:00:00.000Z" {
		t.Fatalf("expected camelCase periodStart, got %#v", gotBody)
	}
	if _, ok := gotBody["period_start"]; ok {
		t.Fatalf("body must not send snake_case period_start: %#v", gotBody)
	}
}

func TestSubmitForApprovalLegacyStartMapsToWeekly(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "apr1", "status": "PENDING"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.submitForApproval(context.Background(), map[string]any{
		"start": "2026-05-04T00:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("submit for approval legacy start failed: %v", err)
	}
	if result.Action != "clockify_submit_for_approval" {
		t.Fatalf("expected submit action, got %q", result.Action)
	}
	if gotBody["period"] != "WEEKLY" {
		t.Fatalf("expected legacy start fallback to WEEKLY, got %#v", gotBody)
	}
	if gotBody["periodStart"] != "2026-05-04T00:00:00.000Z" {
		t.Fatalf("expected legacy start to map to periodStart, got %#v", gotBody)
	}
}

func TestSubmitForApprovalDecodesNonMapResponse(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/approval-requests" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, []map[string]any{{"id": "apr1", "status": "PENDING"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.submitForApproval(context.Background(), map[string]any{
		"period":       "WEEKLY",
		"period_start": "2026-05-04T00:00:00.000Z",
	})
	if err != nil {
		t.Fatalf("submit for approval failed: %v", err)
	}
	if result.Action != "clockify_submit_for_approval" {
		t.Fatalf("expected submit action, got %q", result.Action)
	}
	items, ok := result.Data.([]any)
	if !ok {
		t.Fatalf("expected non-map response to decode as []any, got %T", result.Data)
	}
	if len(items) != 1 {
		t.Fatalf("expected one decoded response item, got %#v", items)
	}
}
