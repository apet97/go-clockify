package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
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
	if _, ok := items[0]["authToken"]; ok {
		t.Fatalf("authToken must not be returned: %#v", items[0])
	}
	if items[0]["secret_token_present"] != true || items[0]["auth_token_suffix"] != "wxyz" {
		t.Fatalf("expected token presence + suffix, got %#v", items[0])
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
	if _, ok := item["authToken"]; ok {
		t.Fatalf("authToken must not be returned: %#v", item)
	}
	if item["secret_token_present"] != true || item["auth_token_suffix"] != "1234" {
		t.Fatalf("expected token presence + suffix, got %#v", item)
	}
}

func TestWebhookMaskingRedactsSnakeCaseAuthToken(t *testing.T) {
	const webhookAuthCanary = "canary-webhook-auth-token-1234567890"
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/abc123def456789012345678" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":           "abc123def456789012345678",
				"auth_token":   webhookAuthCanary,
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
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), webhookAuthCanary) {
		t.Fatalf("tool result leaked webhook auth canary: %s", raw)
	}
	if strings.Contains(string(raw), `"authToken"`) || strings.Contains(string(raw), `"auth_token"`) {
		t.Fatalf("tool result exposed auth token field: %s", raw)
	}
	if !strings.Contains(string(raw), "7890") {
		t.Fatalf("expected masked suffix in result: %s", raw)
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
	if _, ok := data["authToken"]; ok {
		t.Fatalf("authToken must not be returned: %#v", data)
	}
	if data["secret_token_present"] != true || data["auth_token_suffix"] != "1234" {
		t.Fatalf("expected token presence + suffix, got %#v", data)
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

func TestCreateWebhookUserEventsRequireUserSource(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("invalid user-event webhook must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.CreateWebhook(context.Background(), map[string]any{
		"name":          "user webhook",
		"url":           "https://example.com/hook",
		"webhook_event": "USER_UPDATED",
	})
	if err == nil || !strings.Contains(err.Error(), "trigger_source_type=USER_ID") {
		t.Fatalf("expected user-source validation error, got %v", err)
	}
}

func TestCreateWebhookUserEventsAcceptUserSource(t *testing.T) {
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
		"name":                "user webhook",
		"url":                 "https://example.com/hook",
		"webhook_event":       "USER_EMAIL_CHANGED",
		"trigger_source_type": "USER_ID",
		"trigger_source":      []any{"user1"},
	})
	if err != nil {
		t.Fatalf("CreateWebhook user-event failed: %v", err)
	}
	if result.Action != "clockify_create_webhook" {
		t.Fatalf("expected create action, got %q", result.Action)
	}
	if gotBody["triggerSourceType"] != "USER_ID" {
		t.Fatalf("expected USER_ID trigger source type, got %#v", gotBody)
	}
	source, ok := gotBody["triggerSource"].([]any)
	if !ok || len(source) != 1 || source[0] != "user1" {
		t.Fatalf("expected user trigger source, got %#v", gotBody["triggerSource"])
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
	if _, ok := payload["authToken"]; ok {
		t.Fatalf("authToken must not be returned in dry-run payload: %#v", payload)
	}
	if payload["secret_token_present"] != true || payload["auth_token_suffix"] != "9876" {
		t.Fatalf("expected dry-run token presence + suffix, got %#v", payload)
	}
}

func TestCreateWebhookRejectsUnsafeDNSAnswersByDefault(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unsafe DNS webhook validation must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	if !svc.WebhookValidateDNS {
		t.Fatal("New must enable webhook DNS validation by default")
	}
	svc.WebhookHostResolver = func(ctx context.Context, host string) ([]netip.Addr, error) {
		if host != "webhook.example.com" {
			t.Fatalf("expected resolver host webhook.example.com, got %q", host)
		}
		return []netip.Addr{
			netip.MustParseAddr("10.0.0.5"),
			netip.MustParseAddr("127.0.0.1"),
			netip.MustParseAddr("169.254.169.254"),
			netip.MustParseAddr("192.0.2.10"),
		}, nil
	}

	_, err := svc.CreateWebhook(context.Background(), map[string]any{
		"name":          "safe name",
		"url":           "https://webhook.example.com/hook",
		"webhook_event": "NEW_TIME_ENTRY",
		"dry_run":       true,
	})
	if err == nil || !strings.Contains(err.Error(), "rejected_by=private") {
		t.Fatalf("expected private DNS rejection, got %v", err)
	}
}

func TestCreateWebhookRejectsLocalhostCredentialsAndNonHTTPS(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("invalid webhook URL validation must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{name: "localhost", url: "https://localhost/hook", wantErr: "localhost"},
		{name: "dot localhost", url: "https://api.localhost/hook", wantErr: "localhost"},
		{name: "embedded credentials", url: "https://user:pass@example.com/hook", wantErr: "embedded credentials"},
		{name: "http", url: "http://example.com/hook", wantErr: "HTTPS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(client, "ws1")
			svc.WebhookHostResolver = func(ctx context.Context, host string) ([]netip.Addr, error) {
				t.Fatalf("resolver must not run after static URL rejection; got host %q", host)
				return nil, nil
			}
			_, err := svc.CreateWebhook(context.Background(), map[string]any{
				"name":          "safe name",
				"url":           tt.url,
				"webhook_event": "NEW_TIME_ENTRY",
				"dry_run":       true,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestWebhookAllowedDomainsBypassOnlyConfiguredHosts(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("allowlist dry-run validation must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	resolverCalls := 0
	svc.WebhookHostResolver = func(ctx context.Context, host string) ([]netip.Addr, error) {
		resolverCalls++
		return []netip.Addr{netip.MustParseAddr("10.0.0.5")}, nil
	}
	svc.WebhookAllowedDomains = []string{"webhook.example.com", ".trusted.example"}

	allowed := []string{
		"https://webhook.example.com/hook",
		"https://api.trusted.example/hook",
	}
	for _, url := range allowed {
		if _, err := svc.CreateWebhook(context.Background(), map[string]any{
			"name":          "safe name",
			"url":           url,
			"webhook_event": "NEW_TIME_ENTRY",
			"dry_run":       true,
		}); err != nil {
			t.Fatalf("expected allowlisted URL %s to bypass private DNS check, got %v", url, err)
		}
	}
	if resolverCalls != 0 {
		t.Fatalf("allowlisted hosts should not call resolver, got %d calls", resolverCalls)
	}

	_, err := svc.CreateWebhook(context.Background(), map[string]any{
		"name":          "safe name",
		"url":           "https://webhook.example.com.evil/hook",
		"webhook_event": "NEW_TIME_ENTRY",
		"dry_run":       true,
	})
	if err == nil || !strings.Contains(err.Error(), "rejected_by=private") {
		t.Fatalf("expected non-allowlisted host to retain DNS safety, got %v", err)
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
	if _, ok := data["authToken"]; ok {
		t.Fatalf("authToken must not be returned: %#v", data)
	}
	if data["secret_token_present"] != true || data["auth_token_suffix"] != "9876" {
		t.Fatalf("expected returned token presence + suffix, got %#v", data)
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
	if _, ok := payload["authToken"]; ok {
		t.Fatalf("authToken must not be returned in dry-run payload: %#v", payload)
	}
	if payload["secret_token_present"] != true || payload["auth_token_suffix"] != "9876" {
		t.Fatalf("expected dry-run payload token presence + suffix, got %#v", payload)
	}
	current, ok := dataMap["current"].(map[string]any)
	if !ok {
		t.Fatalf("expected current webhook in dry-run data, got %#v", dataMap["current"])
	}
	if _, ok := current["authToken"]; ok {
		t.Fatalf("authToken must not be returned in current webhook: %#v", current)
	}
	if current["secret_token_present"] != true || current["auth_token_suffix"] != "1234" {
		t.Fatalf("expected current token presence + suffix, got %#v", current)
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

func TestUpdateWebhookUserEventsRequireUserSource(t *testing.T) {
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
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodPut:
			putCalled = true
			t.Fatal("invalid user-event webhook update must not PUT")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.UpdateWebhook(context.Background(), map[string]any{
		"webhook_id":    "wh1",
		"webhook_event": "USER_UPDATED",
	})
	if err == nil || !strings.Contains(err.Error(), "trigger_source_type=USER_ID") {
		t.Fatalf("expected user-source validation error, got %v", err)
	}
	if putCalled {
		t.Fatal("PUT should not be called after user-source validation failure")
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

func TestDeleteWebhookDryRunAvoidsDeleteAndMasksAuthToken(t *testing.T) {
	var deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":           "wh1",
				"authToken":    "delete-secret-1234",
				"webhookEvent": "NEW_TIME_ENTRY",
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodDelete:
			deleteCalled = true
			t.Fatal("dry-run must not DELETE webhook")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteWebhook(context.Background(), map[string]any{
		"webhook_id": "wh1",
		"dry_run":    true,
	})
	if err != nil {
		t.Fatalf("DeleteWebhook dry run failed: %v", err)
	}
	if deleteCalled {
		t.Fatal("DELETE should not be called during dry run")
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), "delete-secret-1234") || strings.Contains(string(raw), `"authToken"`) {
		t.Fatalf("dry-run delete leaked webhook auth token: %s", raw)
	}
	if !strings.Contains(string(raw), "1234") {
		t.Fatalf("expected masked suffix in dry-run delete result: %s", raw)
	}
}

func TestTestWebhookDryRunAvoidsPostAndMasksAuthToken(t *testing.T) {
	var postCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":           "wh1",
				"authToken":    "test-dry-run-secret-1234",
				"webhookEvent": "NEW_TIME_ENTRY",
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1/test" && r.Method == http.MethodPost:
			postCalled = true
			t.Fatal("dry-run must not POST webhook test delivery")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TestWebhook(context.Background(), map[string]any{
		"webhook_id": "wh1",
		"dry_run":    true,
	})
	if err != nil {
		t.Fatalf("TestWebhook dry run failed: %v", err)
	}
	if postCalled {
		t.Fatal("POST should not be called during dry run")
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), "test-dry-run-secret-1234") || strings.Contains(string(raw), `"authToken"`) {
		t.Fatalf("dry-run test leaked webhook auth token: %s", raw)
	}
	if !strings.Contains(string(raw), "1234") {
		t.Fatalf("expected masked suffix in dry-run test result: %s", raw)
	}
}

func TestTestWebhookMasksAuthToken(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1/test" && r.Method == http.MethodPost:
			respondJSON(t, w, map[string]any{
				"id":           "wh1",
				"authToken":    "test-secret-9876",
				"webhookEvent": "NEW_TIME_ENTRY",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.TestWebhook(context.Background(), map[string]any{"webhook_id": "wh1"})
	if err != nil {
		t.Fatalf("TestWebhook failed: %v", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), "test-secret-9876") || strings.Contains(string(raw), `"authToken"`) {
		t.Fatalf("test webhook leaked webhook auth token: %s", raw)
	}
	if !strings.Contains(string(raw), "9876") {
		t.Fatalf("expected masked suffix in test result: %s", raw)
	}
}

// TestWebhookNameSchemaBoundsMatchClockifySpec pins the create/update
// webhook `name` property to the live API contract: length 2..30
// (Clockify WEBHOOKDOC.md). Without the schema bounds an out-of-range
// name would round-trip to the upstream and surface as a cryptic
// upstream-side error; with the bounds the dispatcher's schema
// validator rejects it before the handler runs.
func TestWebhookNameSchemaBoundsMatchClockifySpec(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("schema-bounds test must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	descs := webhookHandlers(svc)

	want := map[string]bool{
		"clockify_create_webhook": false,
		"clockify_update_webhook": false,
	}
	for _, d := range descs {
		if _, wanted := want[d.Tool.Name]; !wanted {
			continue
		}
		want[d.Tool.Name] = true
		schema := d.Tool.InputSchema
		props, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s: schema.properties not a map", d.Tool.Name)
		}
		name, ok := props["name"].(map[string]any)
		if !ok {
			t.Fatalf("%s: schema.properties.name not a map", d.Tool.Name)
		}
		if name["minLength"] != 2 {
			t.Fatalf("%s: schema.properties.name.minLength must be 2, got %#v", d.Tool.Name, name["minLength"])
		}
		if name["maxLength"] != 30 {
			t.Fatalf("%s: schema.properties.name.maxLength must be 30, got %#v", d.Tool.Name, name["maxLength"])
		}
	}
	for tool, found := range want {
		if !found {
			t.Fatalf("descriptor %q not registered by webhookHandlers", tool)
		}
	}
}
