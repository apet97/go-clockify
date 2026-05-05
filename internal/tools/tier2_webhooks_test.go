package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
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
					{"id": "wh1", "url": "https://example.invalid/x", "webhookEvent": "NEW_TIME_ENTRY", "authToken": "abcdefghijklmnopqrstuvwxyz"},
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
	if got := items[0]["authToken"]; got != "********wxyz" {
		t.Fatalf("expected masked authToken, got %#v", got)
	}
	if result.Meta["count"] != 1 {
		t.Fatalf("expected meta count=1, got %v", result.Meta["count"])
	}
	if result.Meta["workspaceWebhookCount"] != 2 {
		t.Fatalf("expected meta workspaceWebhookCount=2, got %v", result.Meta["workspaceWebhookCount"])
	}
}

func TestGetWebhookMasksAuthToken(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/abc123def456789012345678" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":           "abc123def456789012345678",
				"authToken":    "secret-token-1234",
				"webhookEvent": "NEW_TIME_ENTRY",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.GetWebhook(context.Background(), map[string]any{"webhook_id": "abc123def456789012345678"})
	if err != nil {
		t.Fatalf("GetWebhook failed: %v", err)
	}
	item, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("GetWebhook data: expected map, got %T", result.Data)
	}
	if got := item["authToken"]; got != "********1234" {
		t.Fatalf("expected masked authToken, got %#v", got)
	}
}

func TestCreateWebhookUsesSingularEventAndTriggerSource(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"id":                "wh1",
				"name":              gotBody["name"],
				"url":               gotBody["url"],
				"webhookEvent":      gotBody["webhookEvent"],
				"triggerSourceType": gotBody["triggerSourceType"],
				"triggerSource":     gotBody["triggerSource"],
				"authToken":         gotBody["authToken"],
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateWebhook(context.Background(), map[string]any{
		"name":                "live webhook",
		"url":                 "https://example.com/hook",
		"webhook_event":       "NEW_TIME_ENTRY",
		"trigger_source_type": "WORKSPACE_ID",
		"trigger_source":      []any{"ws1"},
		"auth_token":          "secret-token-1234",
	})
	if err != nil {
		t.Fatalf("CreateWebhook failed: %v", err)
	}
	if result.Action != "clockify_create_webhook" {
		t.Fatalf("expected create action, got %q", result.Action)
	}
	if gotBody["webhookEvent"] != "NEW_TIME_ENTRY" {
		t.Fatalf("expected singular webhookEvent, got body %#v", gotBody)
	}
	if _, ok := gotBody["events"]; ok {
		t.Fatalf("body must not use legacy events array: %#v", gotBody)
	}
	if gotBody["authToken"] != "secret-token-1234" {
		t.Fatalf("expected auth_token to map to authToken, got %#v", gotBody)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("CreateWebhook data: expected map, got %T", result.Data)
	}
	if got := data["authToken"]; got != "********1234" {
		t.Fatalf("expected returned authToken to be masked, got %#v", got)
	}
}

func TestCreateWebhookRejectsMissingName(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("CreateWebhook missing-name validation must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.CreateWebhook(context.Background(), map[string]any{
		"url":           "https://example.com/hook",
		"webhook_event": "NEW_TIME_ENTRY",
	})
	if err == nil {
		t.Fatal("expected missing name error")
	}
}

func TestCreateWebhookRejectsMissingWebhookEvent(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("CreateWebhook missing-event validation must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.CreateWebhook(context.Background(), map[string]any{
		"name": "live webhook",
		"url":  "https://example.com/hook",
	})
	if err == nil {
		t.Fatal("expected missing webhook_event error")
	}
}

func TestCreateWebhookDefaultsTriggerSourceWhenOmitted(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/webhooks" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "wh1"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateWebhook(context.Background(), map[string]any{
		"name":          "live webhook",
		"url":           "https://example.com/hook",
		"webhook_event": "NEW_TIME_ENTRY",
	})
	if err != nil {
		t.Fatalf("CreateWebhook failed: %v", err)
	}
	if result.Action != "clockify_create_webhook" {
		t.Fatalf("expected create action, got %q", result.Action)
	}
	if gotBody["triggerSourceType"] != "WORKSPACE_ID" {
		t.Fatalf("expected default triggerSourceType WORKSPACE_ID, got %#v", gotBody)
	}
	source, ok := gotBody["triggerSource"].([]any)
	if !ok || len(source) != 1 || source[0] != "ws1" {
		t.Fatalf("expected default triggerSource [ws1], got %#v", gotBody["triggerSource"])
	}
}

func TestCreateWebhookDryRunAvoidsPost(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateWebhook(context.Background(), map[string]any{
		"name":          "live webhook",
		"url":           "https://example.com/hook",
		"webhook_event": "NEW_TIME_ENTRY",
		"auth_token":    "dry-run-secret-9876",
		"dry_run":       true,
	})
	if err != nil {
		t.Fatalf("CreateWebhook dry run failed: %v", err)
	}
	if result.Action != "clockify_create_webhook" {
		t.Fatalf("expected create action, got %q", result.Action)
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok || dataMap["dry_run"] != true {
		t.Fatalf("expected dry-run data map, got %#v", result.Data)
	}
	payload, ok := dataMap["payload"].(map[string]any)
	if !ok || payload["webhookEvent"] != "NEW_TIME_ENTRY" || payload["triggerSourceType"] != "WORKSPACE_ID" {
		t.Fatalf("unexpected dry-run payload: %#v", dataMap["payload"])
	}
	if got := payload["authToken"]; got != "********9876" {
		t.Fatalf("expected dry-run authToken to be masked, got %#v", got)
	}
}

func TestCreateWebhookLegacyEventsArrayFallback(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/webhooks" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "wh1"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateWebhook(context.Background(), map[string]any{
		"name":   "legacy webhook",
		"url":    "https://example.com/hook",
		"events": []any{"TIMER_STOPPED"},
	})
	if err != nil {
		t.Fatalf("CreateWebhook failed: %v", err)
	}
	if result.Action != "clockify_create_webhook" {
		t.Fatalf("expected create action, got %q", result.Action)
	}
	if gotBody["webhookEvent"] != "TIMER_STOPPED" {
		t.Fatalf("expected legacy events[0] to map to webhookEvent, got %#v", gotBody)
	}
	if _, ok := gotBody["events"]; ok {
		t.Fatalf("body must not send legacy events array: %#v", gotBody)
	}
}

func TestUpdateWebhookCarriesRequiredLiveFields(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":                "wh1",
				"name":              "old",
				"url":               "https://example.com/old",
				"webhookEvent":      "NEW_TIME_ENTRY",
				"triggerSourceType": "WORKSPACE_ID",
				"triggerSource":     []string{"ws1"},
				"authToken":         "old-secret-1234",
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"id":                "wh1",
				"name":              gotBody["name"],
				"url":               gotBody["url"],
				"webhookEvent":      gotBody["webhookEvent"],
				"triggerSourceType": gotBody["triggerSourceType"],
				"triggerSource":     gotBody["triggerSource"],
				"authToken":         gotBody["authToken"],
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateWebhook(context.Background(), map[string]any{
		"webhook_id":    "wh1",
		"name":          "new",
		"webhook_event": "TIMER_STOPPED",
		"auth_token":    "new-secret-9876",
	})
	if err != nil {
		t.Fatalf("UpdateWebhook failed: %v", err)
	}
	if result.Action != "clockify_update_webhook" {
		t.Fatalf("expected update action, got %q", result.Action)
	}
	for _, key := range []string{"name", "url", "webhookEvent", "triggerSourceType", "triggerSource"} {
		if gotBody[key] == nil {
			t.Fatalf("update body missing required live field %q: %#v", key, gotBody)
		}
	}
	if gotBody["webhookEvent"] != "TIMER_STOPPED" {
		t.Fatalf("expected updated event, got body %#v", gotBody)
	}
	if gotBody["name"] != "new" {
		t.Fatalf("expected updated name, got body %#v", gotBody)
	}
	if gotBody["url"] != "https://example.com/old" {
		t.Fatalf("expected existing URL to be carried forward, got body %#v", gotBody)
	}
	if gotBody["triggerSourceType"] != "WORKSPACE_ID" {
		t.Fatalf("expected existing triggerSourceType to be carried forward, got body %#v", gotBody)
	}
	if gotBody["authToken"] != "new-secret-9876" {
		t.Fatalf("expected auth_token to map to authToken, got body %#v", gotBody)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("UpdateWebhook data: expected map, got %T", result.Data)
	}
	if got := data["authToken"]; got != "********9876" {
		t.Fatalf("expected returned authToken to be masked, got %#v", got)
	}
	source, ok := gotBody["triggerSource"].([]any)
	if !ok || len(source) != 1 || source[0] != "ws1" {
		t.Fatalf("expected existing triggerSource [ws1] to be carried forward, got %#v", gotBody["triggerSource"])
	}
}

func TestUpdateWebhookDryRunAvoidsPut(t *testing.T) {
	var putCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":                "wh1",
				"name":              "old",
				"url":               "https://example.com/old",
				"webhookEvent":      "NEW_TIME_ENTRY",
				"triggerSourceType": "WORKSPACE_ID",
				"triggerSource":     []string{"ws1"},
				"authToken":         "old-secret-1234",
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodPut:
			putCalled = true
			t.Fatal("dry-run must not PUT webhook updates")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateWebhook(context.Background(), map[string]any{
		"webhook_id":    "wh1",
		"name":          "new",
		"webhook_event": "TIMER_STOPPED",
		"auth_token":    "new-secret-9876",
		"dry_run":       true,
	})
	if err != nil {
		t.Fatalf("UpdateWebhook dry run failed: %v", err)
	}
	if putCalled {
		t.Fatal("PUT should not be called during dry run")
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok || dataMap["dry_run"] != true {
		t.Fatalf("expected dry-run data map, got %#v", result.Data)
	}
	payload, ok := dataMap["payload"].(map[string]any)
	if !ok || payload["name"] != "new" || payload["webhookEvent"] != "TIMER_STOPPED" {
		t.Fatalf("unexpected dry-run payload: %#v", dataMap["payload"])
	}
	if got := payload["authToken"]; got != "********9876" {
		t.Fatalf("expected dry-run payload authToken to be masked, got %#v", got)
	}
	current, ok := dataMap["current"].(map[string]any)
	if !ok {
		t.Fatalf("expected current webhook in dry-run data, got %#v", dataMap["current"])
	}
	if got := current["authToken"]; got != "********1234" {
		t.Fatalf("expected current authToken to be masked, got %#v", got)
	}
}

func TestUpdateWebhookRejectsNoFieldsProvided(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":                "wh1",
				"name":              "old",
				"url":               "https://example.com/old",
				"webhookEvent":      "NEW_TIME_ENTRY",
				"triggerSourceType": "WORKSPACE_ID",
				"triggerSource":     []string{"ws1"},
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodPut:
			t.Fatal("UpdateWebhook with no changed fields must not PUT")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.UpdateWebhook(context.Background(), map[string]any{"webhook_id": "wh1"})
	if err == nil || !strings.Contains(err.Error(), "at least one field") {
		t.Fatalf("expected no-fields error, got %v", err)
	}
}

func TestUpdateWebhookLegacyEventsArrayFallback(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":                "wh1",
				"name":              "old",
				"url":               "https://example.com/old",
				"webhookEvent":      "NEW_TIME_ENTRY",
				"triggerSourceType": "WORKSPACE_ID",
				"triggerSource":     []string{"ws1"},
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{"id": "wh1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateWebhook(context.Background(), map[string]any{
		"webhook_id": "wh1",
		"events":     []any{"TIMER_STOPPED"},
	})
	if err != nil {
		t.Fatalf("UpdateWebhook failed: %v", err)
	}
	if result.Action != "clockify_update_webhook" {
		t.Fatalf("expected update action, got %q", result.Action)
	}
	if gotBody["webhookEvent"] != "TIMER_STOPPED" {
		t.Fatalf("expected legacy events[0] to update webhookEvent, got %#v", gotBody)
	}
	if _, ok := gotBody["events"]; ok {
		t.Fatalf("body must not send legacy events array: %#v", gotBody)
	}
}
