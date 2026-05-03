package tools

import (
	"context"
	"net/http"
	"testing"
)

// Note: webhook group registration is already covered by
// TestWebhookHandlersCount in tier2_admin_test.go. This file pins the
// list-webhooks shape (envelope unwrap) and the static webhook-events
// enum which had no unit coverage.

func TestListWebhookEvents(t *testing.T) {
	// Pure-static handler: must NOT issue any HTTP request. Wire a
	// mock that fails the test if it's hit.
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("ListWebhookEvents must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListWebhookEvents(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("ListWebhookEvents failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	if result.Action != "clockify_list_webhook_events" {
		t.Fatalf("expected action clockify_list_webhook_events, got %s", result.Action)
	}
	events, ok := result.Data.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", result.Data)
	}
	if len(events) < 50 {
		t.Fatalf("expected ≥50 webhook events in static enum, got %d", len(events))
	}
	// Sanity-check two well-known members are present (the values
	// the campaign and probe both verified live).
	wanted := map[string]bool{
		"NEW_TIME_ENTRY": false,
		"TIMER_STOPPED":  false,
	}
	for _, e := range events {
		if _, want := wanted[e]; want {
			wanted[e] = true
		}
	}
	for k, found := range wanted {
		if !found {
			t.Fatalf("static enum missing well-known event %q (got %d entries)", k, len(events))
		}
	}
	if result.Meta["count"] != len(events) {
		t.Fatalf("expected meta count=%d, got %v", len(events), result.Meta["count"])
	}
}

func TestListWebhooks(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks" && r.Method == http.MethodGet:
			if got := r.URL.Query().Get("page-size"); got != "50" {
				t.Fatalf("expected page-size=50, got %s", got)
			}
			respondJSON(t, w, map[string]any{
				"workspaceWebhookCount": 2,
				"webhooks": []map[string]any{
					{"id": "wh1", "url": "https://example.invalid/x", "webhookEvent": "NEW_TIME_ENTRY"},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListWebhooks(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("ListWebhooks failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	if result.Action != "clockify_list_webhooks" {
		t.Fatalf("expected action clockify_list_webhooks, got %s", result.Action)
	}
	items, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("ListWebhooks data: expected []map[string]any, got %T", result.Data)
	}
	if len(items) != 1 || items[0]["id"] != "wh1" {
		t.Fatalf("ListWebhooks items: expected [{id:wh1}], got %#v", items)
	}
	if result.Meta["count"] != 1 {
		t.Fatalf("expected meta count=1, got %v", result.Meta["count"])
	}
	if result.Meta["workspaceWebhookCount"] != 2 {
		t.Fatalf("expected meta workspaceWebhookCount=2, got %v", result.Meta["workspaceWebhookCount"])
	}
}
