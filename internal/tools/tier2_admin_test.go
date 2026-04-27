package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
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

// TestTestWebhookDryRun locks in audit finding 7: clockify_test_webhook
// is non-destructive but triggers an external delivery, so dry_run:true
// must short-circuit before the POST /test call. Pre-fix the schema
// did not even expose dry_run and the handler always sent the test.
func TestTestWebhookDryRun(t *testing.T) {
	var testPostCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":     "wh1",
				"url":    "https://example.com/hook",
				"events": []string{"NEW_TIME_ENTRY"},
			})
		case r.URL.Path == "/workspaces/ws1/webhooks/wh1/test" && r.Method == http.MethodPost:
			testPostCalled = true
			respondJSON(t, w, map[string]any{"status": "delivered"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")

	// 1. Dry-run: GETs the webhook record and returns wrapped envelope,
	//    must NOT POST /test.
	result, err := svc.TestWebhook(context.Background(), map[string]any{
		"webhook_id": "wh1",
		"dry_run":    true,
	})
	if err != nil {
		t.Fatalf("test webhook dry run failed: %v", err)
	}
	if testPostCalled {
		t.Fatal("dry-run must not POST /test")
	}
	if result.Action != "clockify_test_webhook" {
		t.Fatalf("unexpected action %q", result.Action)
	}

	// 2. Executed: POSTs /test as before.
	result, err = svc.TestWebhook(context.Background(), map[string]any{
		"webhook_id": "wh1",
	})
	if err != nil {
		t.Fatalf("test webhook execute failed: %v", err)
	}
	if !testPostCalled {
		t.Fatal("non-dry-run must POST /test")
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

// TestValidateWebhookURL_DNS_HostedProfile_RejectsPrivateA exercises
// audit finding 10: with WebhookValidateDNS=true (set automatically
// by hosted profiles), a hostname that resolves to a private or
// reserved IP must be rejected, not just literal IP addresses. The
// test injects a deterministic resolver so the test stays offline.
func TestValidateWebhookURL_DNS_HostedProfile_RejectsPrivateA(t *testing.T) {
	cases := []struct {
		name      string
		host      string
		ip        string
		wantBlock bool
	}{
		{"private_10", "internal.example.com", "10.0.0.1", true},
		{"private_172", "internal.example.com", "172.16.0.1", true},
		{"private_192", "internal.example.com", "192.168.1.1", true},
		{"loopback_dns", "internal.example.com", "127.0.0.1", true},
		{"link_local", "internal.example.com", "169.254.169.254", true}, // AWS metadata
		{"public", "public.example.com", "8.8.8.8", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &Service{
				WebhookValidateDNS: true,
				WebhookHostResolver: func(ctx context.Context, host string) ([]netip.Addr, error) {
					addr, err := netip.ParseAddr(c.ip)
					if err != nil {
						t.Fatalf("bad test ip %s: %v", c.ip, err)
					}
					return []netip.Addr{addr}, nil
				},
			}
			err := svc.validateWebhookURLForService(context.Background(), "https://"+c.host+"/hook")
			if c.wantBlock && err == nil {
				t.Fatalf("DNS-resolved %s → %s should be rejected", c.host, c.ip)
			}
			if !c.wantBlock && err != nil {
				t.Fatalf("DNS-resolved %s → %s should be allowed: %v", c.host, c.ip, err)
			}
		})
	}
}

// TestValidateWebhookURL_DNS_NoFlagSkipsResolution confirms that
// when WebhookValidateDNS is false (local/dev profile default), the
// resolver is never consulted — preserving the prior behaviour for
// operators who depend on internal Clockify webhooks pointing at
// hostnames that resolve to private IPs in their network.
func TestValidateWebhookURL_DNS_NoFlagSkipsResolution(t *testing.T) {
	resolverCalls := 0
	svc := &Service{
		WebhookValidateDNS: false,
		WebhookHostResolver: func(ctx context.Context, host string) ([]netip.Addr, error) {
			resolverCalls++
			return nil, nil
		},
	}
	if err := svc.validateWebhookURLForService(context.Background(), "https://internal.example.com/hook"); err != nil {
		t.Fatalf("unexpected error with WebhookValidateDNS=false: %v", err)
	}
	if resolverCalls != 0 {
		t.Fatalf("resolver should not be called when WebhookValidateDNS=false (got %d calls)", resolverCalls)
	}
}

// TestValidateWebhookURL_DNS_LookupErrorPropagates locks in fail-closed
// behaviour: a DNS error blocks webhook creation rather than silently
// allowing the URL through.
func TestValidateWebhookURL_DNS_LookupErrorPropagates(t *testing.T) {
	svc := &Service{
		WebhookValidateDNS: true,
		WebhookHostResolver: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return nil, fmt.Errorf("nxdomain")
		},
	}
	err := svc.validateWebhookURLForService(context.Background(), "https://does-not-exist.example.com/hook")
	if err == nil {
		t.Fatal("DNS lookup error must surface as a webhook validation failure")
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
