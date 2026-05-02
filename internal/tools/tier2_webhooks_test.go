package tools

import (
	"context"
	"net/http"
	"testing"
)

// Note: webhook group registration is already covered by
// TestWebhookHandlersCount in tier2_admin_test.go. This file pins the
// list-webhooks shape (envelope unwrap) which had no unit coverage.

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
