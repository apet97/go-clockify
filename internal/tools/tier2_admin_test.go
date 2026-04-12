package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestUserAdminHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descriptors, ok := svc.Tier2Handlers("user_admin")
	if !ok {
		t.Fatal("user_admin group not found")
	}
	if len(descriptors) != 8 {
		t.Fatalf("expected 8 user_admin tools, got %d", len(descriptors))
	}
}

func TestWebhookHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descriptors, ok := svc.Tier2Handlers("webhooks")
	if !ok {
		t.Fatal("webhooks group not found")
	}
	if len(descriptors) != 7 {
		t.Fatalf("expected 7 webhook tools, got %d", len(descriptors))
	}
}

func TestListUserGroups(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/user-groups" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{
				{"id": "g1", "name": "Engineering"},
				{"id": "g2", "name": "Design"},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.ListUserGroups(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("list user groups failed: %v", err)
	}
	if result.Action != "clockify_list_user_groups" {
		t.Fatalf("expected action clockify_list_user_groups, got %s", result.Action)
	}
	groups, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestCreateWebhookValidatesHTTPS(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected for invalid URL")
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.CreateWebhook(context.Background(), map[string]any{
		"url":    "http://example.com/hook",
		"events": []any{"NEW_TIME_ENTRY"},
	})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS validation error, got %v", err)
	}
}

func TestCreateWebhookBlocksPrivateIP(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected for private IP URL")
	})
	defer cleanup()

	svc := New(client, "ws1")

	cases := []struct {
		url string
	}{
		{"https://localhost/hook"},
		{"https://api.localhost/hook"},
		{"https://127.0.0.1/hook"},
		{"https://10.0.0.1/hook"},
		{"https://192.168.1.1/hook"},
		{"https://172.16.0.1/hook"},
		{"https://172.31.255.1/hook"},
		{"https://0.0.0.0/hook"},
		{"https://169.254.169.254/hook"},
		{"https://100.64.0.1/hook"},
		{"https://[::1]/hook"},
		{"https://[fe80::1]/hook"},
		{"https://[fd00::1]/hook"},
	}

	for _, tc := range cases {
		_, err := svc.CreateWebhook(context.Background(), map[string]any{
			"url":    tc.url,
			"events": []any{"NEW_TIME_ENTRY"},
		})
		if err == nil || !strings.Contains(err.Error(), "cannot target") {
			t.Fatalf("URL %s: expected reserved-target error, got %v", tc.url, err)
		}
	}
}

func TestDeleteWebhookDryRun(t *testing.T) {
	var deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":     "wh1",
				"url":    "https://example.com/hook",
				"events": []string{"NEW_TIME_ENTRY"},
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteWebhook(context.Background(), map[string]any{
		"webhook_id": "wh1",
		"dry_run":    true,
	})
	if err != nil {
		t.Fatalf("delete webhook dry run failed: %v", err)
	}
	if result.Action != "clockify_delete_webhook" {
		t.Fatalf("expected action clockify_delete_webhook, got %s", result.Action)
	}
	if deleteCalled {
		t.Fatal("DELETE should NOT be called during dry run")
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data for dry run, got %T", result.Data)
	}
	if dataMap["dry_run"] != true {
		t.Fatal("expected dry_run=true in result data")
	}
	if dataMap["note"] == nil {
		t.Fatal("expected note in dry run result")
	}
}

func TestCreateUserGroup(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/user-groups" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "Backend Team" {
				t.Fatalf("expected name 'Backend Team', got %v", body["name"])
			}
			respondJSON(t, w, map[string]any{"id": "g1", "name": "Backend Team"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateUserGroup(context.Background(), map[string]any{"name": "Backend Team"})
	if err != nil {
		t.Fatalf("create user group failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["name"] != "Backend Team" {
		t.Fatalf("unexpected group name: %v", data["name"])
	}
}

func TestValidateWebhookURL(t *testing.T) {
	// Valid URLs should pass
	if err := validateWebhookURL("https://example.com/hook"); err != nil {
		t.Fatalf("valid URL rejected: %v", err)
	}
	if err := validateWebhookURL("https://hooks.example.com:8443/callback"); err != nil {
		t.Fatalf("valid URL with port rejected: %v", err)
	}
	if err := validateWebhookURL("https://8.8.8.8/hook"); err != nil {
		t.Fatalf("public IPv4 should be allowed: %v", err)
	}
	if err := validateWebhookURL("https://[2001:4860:4860::8888]/hook"); err != nil {
		t.Fatalf("public IPv6 should be allowed: %v", err)
	}

	// 172.15.x should be allowed (not in 16-31 range)
	if err := validateWebhookURL("https://172.15.0.1/hook"); err != nil {
		t.Fatalf("172.15.x should be allowed: %v", err)
	}
	// 172.32.x should be allowed (not in 16-31 range)
	if err := validateWebhookURL("https://172.32.0.1/hook"); err != nil {
		t.Fatalf("172.32.x should be allowed: %v", err)
	}
}

func TestValidateWebhookURLRejectsCredentials(t *testing.T) {
	if err := validateWebhookURL("https://user:pass@example.com/hook"); err == nil {
		t.Fatal("expected embedded credentials to be rejected")
	}
}

func TestDeactivateUserDryRun(t *testing.T) {
	var putCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCalled = true
		}
		t.Fatalf("no API call expected during dry run")
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeactivateUser(context.Background(), map[string]any{
		"user_id": "u1",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("deactivate user dry run failed: %v", err)
	}
	if result.Action != "clockify_deactivate_user" {
		t.Fatalf("expected action clockify_deactivate_user, got %s", result.Action)
	}
	if putCalled {
		t.Fatal("PUT should NOT be called during dry run")
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data for dry run, got %T", result.Data)
	}
	if dataMap["dry_run"] != true {
		t.Fatal("expected dry_run=true in result data")
	}
}
