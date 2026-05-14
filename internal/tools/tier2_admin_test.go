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
	descriptors, ok := tier2Handlers(svc, "user_admin")
	if !ok {
		t.Fatal("user_admin group not found")
	}
	if len(descriptors) != 12 {
		t.Fatalf("expected 12 user_admin tools, got %d", len(descriptors))
	}
}

func TestWebhookHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descriptors, ok := tier2Handlers(svc, "webhooks")
	if !ok {
		t.Fatal("webhooks group not found")
	}
	if len(descriptors) != 8 {
		t.Fatalf("expected 8 webhook tools, got %d", len(descriptors))
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

func TestMemberProfileTools(t *testing.T) {
	var patchBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/member-profile/user1" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"name":        "Pat",
				"workingDays": []string{"MONDAY", "TUESDAY"},
			})
		case r.URL.Path == "/workspaces/ws1/member-profile/user1" && r.Method == http.MethodPatch:
			if err := json.NewDecoder(r.Body).Decode(&patchBody); err != nil {
				t.Fatalf("decode profile patch: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"name":        patchBody["name"],
				"weekStart":   patchBody["weekStart"],
				"workingDays": patchBody["workingDays"],
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.GetMemberProfile(context.Background(), map[string]any{"user_id": "user1"})
	mustOK(t, result, err, "clockify_get_member_profile")

	result, err = svc.UpdateMemberProfile(context.Background(), map[string]any{
		"user_id":              "user1",
		"name":                 "Pat Updated",
		"week_start":           "monday",
		"work_capacity":        "PT7H",
		"working_days":         []any{"monday", "tuesday", "wednesday"},
		"remove_profile_image": true,
		"user_custom_fields":   []any{map[string]any{"id": "cf1", "value": "blue"}},
	})
	mustOK(t, result, err, "clockify_update_member_profile")
	if patchBody["weekStart"] != "MONDAY" {
		t.Fatalf("expected weekStart=MONDAY, got %#v", patchBody)
	}
	days, ok := patchBody["workingDays"].([]any)
	if !ok || len(days) != 3 || days[0] != "MONDAY" || days[2] != "WEDNESDAY" {
		t.Fatalf("expected workingDays array of enum strings, got %#v", patchBody["workingDays"])
	}
	if _, ok := patchBody["working_days"]; ok {
		t.Fatalf("payload must not contain snake_case working_days: %#v", patchBody)
	}
	if patchBody["removeProfileImage"] != true {
		t.Fatalf("expected removeProfileImage=true, got %#v", patchBody)
	}
}

func TestUpdateMemberProfileDryRunSerializesWorkingDays(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("member profile dry-run must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateMemberProfile(context.Background(), map[string]any{
		"user_id":       "user1",
		"working_days":  []string{"monday", "friday"},
		"work_capacity": "PT6H",
		"dry_run":       true,
	})
	mustOK(t, result, err, "clockify_update_member_profile")
	data, ok := result.Data.(map[string]any)
	if !ok || data["dry_run"] != true {
		t.Fatalf("expected dry-run data, got %#v", result.Data)
	}
	payload, ok := data["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected dry-run payload map, got %#v", data["payload"])
	}
	days, ok := payload["workingDays"].([]string)
	if !ok || len(days) != 2 || days[0] != "MONDAY" || days[1] != "FRIDAY" {
		t.Fatalf("expected dry-run workingDays []string, got %#v", payload["workingDays"])
	}
}

func TestCreateUserGroupDryRun(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateUserGroup(context.Background(), map[string]any{
		"name":    "Backend Team",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("create user group dry run failed: %v", err)
	}
	if result.Action != "clockify_create_user_group" {
		t.Fatalf("expected action clockify_create_user_group, got %s", result.Action)
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok || dataMap["dry_run"] != true {
		t.Fatalf("expected dry-run data map, got %#v", result.Data)
	}
	payload, ok := dataMap["payload"].(map[string]any)
	if !ok || payload["name"] != "Backend Team" {
		t.Fatalf("unexpected dry-run payload: %#v", dataMap["payload"])
	}
}

func TestAddUserToGroupDryRun(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("add user to group dry-run must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.AddUserToGroup(context.Background(), map[string]any{
		"group_id": "abc123def456789012345678",
		"user_id":  "abc123def456789012345679",
		"dry_run":  true,
	})
	if err != nil {
		t.Fatalf("add user to group dry-run failed: %v", err)
	}
	if result.Action != "clockify_add_user_to_group" {
		t.Fatalf("expected action clockify_add_user_to_group, got %s", result.Action)
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok || dataMap["dry_run"] != true {
		t.Fatalf("expected dry-run data map, got %#v", result.Data)
	}
	payload, ok := dataMap["payload"].(map[string]any)
	if !ok || payload["userId"] != "abc123def456789012345679" {
		t.Fatalf("unexpected dry-run payload: %#v", dataMap["payload"])
	}
}

func TestInviteUserDryRunAndExecute(t *testing.T) {
	var gotBody map[string]any
	var gotSendEmail string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/users" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotSendEmail = r.URL.Query().Get("send-email")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "ws1", "users": []any{map[string]any{"email": gotBody["email"]}}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	dry, err := svc.InviteUser(context.Background(), map[string]any{
		"email":   "invitee@example.com",
		"dry_run": true,
	})
	mustOK(t, dry, err, "clockify_invite_user")
	if data, ok := dry.Data.(map[string]any); !ok || data["dry_run"] != true || data["send_email"] != true {
		t.Fatalf("expected dry-run invite preview, got %#v", dry.Data)
	}

	res, err := svc.InviteUser(context.Background(), map[string]any{
		"email":      "invitee@example.com",
		"send_email": false,
	})
	mustOK(t, res, err, "clockify_invite_user")
	if gotSendEmail != "false" {
		t.Fatalf("expected send-email=false query, got %q", gotSendEmail)
	}
	if gotBody["email"] != "invitee@example.com" {
		t.Fatalf("expected email body, got %#v", gotBody)
	}
}

// TestValidateWebhookURL_DNS_StrictProfile_RejectsPrivateA exercises
// the strict-DNS path: with WebhookValidateDNS=true, a hostname that
// resolves to a private or reserved IP must be rejected, not just
// literal IP addresses. The test injects a deterministic resolver so
// the test stays offline.
func TestValidateWebhookURL_DNS_StrictProfile_RejectsPrivateA(t *testing.T) {
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

func TestValidateWebhookURLRejectsPrivateIPv6Literals(t *testing.T) {
	cases := []string{
		"https://[::1]/hook",
		"https://[fc00::1]/hook",
		"https://[fd12:3456:789a::1]/hook",
		"https://[fe80::1]/hook",
		"https://[::ffff:10.0.0.1]/hook",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if err := validateWebhookURL(raw); err == nil {
				t.Fatalf("expected private/reserved IPv6 literal to be rejected: %s", raw)
			}
		})
	}
}

func TestValidateWebhookURLReportsCIDRClass(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://127.0.0.1/hook", "rejected_by=loopback"},
		{"https://10.0.0.1/hook", "rejected_by=private"},
		{"https://169.254.169.254/hook", "rejected_by=link-local"},
		{"https://localhost/hook", "rejected_by=localhost"},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			err := validateWebhookURL(c.raw)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %s", err, c.want)
			}
		})
	}
}

func TestValidateWebhookURL_DNS_PunycodeAndCNAMEToPrivate(t *testing.T) {
	cases := []struct {
		name string
		host string
		ip   string
	}{
		{"punycode_private", "xn--bcher-kva.example", "10.0.0.8"},
		{"cname_final_private", "cname-to-private.example.com", "192.168.10.25"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var resolvedHost string
			svc := &Service{
				WebhookValidateDNS: true,
				WebhookHostResolver: func(ctx context.Context, host string) ([]netip.Addr, error) {
					resolvedHost = host
					addr, err := netip.ParseAddr(c.ip)
					if err != nil {
						t.Fatalf("bad test ip %s: %v", c.ip, err)
					}
					return []netip.Addr{addr}, nil
				},
			}
			err := svc.validateWebhookURLForService(context.Background(), "https://"+c.host+"/hook")
			if err == nil {
				t.Fatalf("%s resolving to %s should be rejected", c.host, c.ip)
			}
			if resolvedHost != c.host {
				t.Fatalf("resolver host = %q, want %q", resolvedHost, c.host)
			}
		})
	}
}

func TestValidateWebhookURL_DNS_ReResolvesEveryValidation(t *testing.T) {
	answers := []string{"8.8.8.8", "10.0.0.9"}
	calls := 0
	svc := &Service{
		WebhookValidateDNS: true,
		WebhookHostResolver: func(ctx context.Context, host string) ([]netip.Addr, error) {
			if calls >= len(answers) {
				t.Fatalf("unexpected resolver call %d", calls+1)
			}
			addr, err := netip.ParseAddr(answers[calls])
			if err != nil {
				t.Fatalf("bad test ip %s: %v", answers[calls], err)
			}
			calls++
			return []netip.Addr{addr}, nil
		},
	}
	if err := svc.validateWebhookURLForService(context.Background(), "https://rebinding.example.com/hook"); err != nil {
		t.Fatalf("first public answer should be allowed: %v", err)
	}
	if err := svc.validateWebhookURLForService(context.Background(), "https://rebinding.example.com/hook"); err == nil {
		t.Fatal("second private answer should be rejected after re-resolution")
	}
	if calls != 2 {
		t.Fatalf("resolver calls = %d, want 2", calls)
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

// TestValidateWebhookURL_DNS_AllowedDomains exercises the operator
// escape hatch: when the host matches WebhookAllowedDomains, the
// private-IP check is bypassed entirely. Use case is split-horizon
// DNS where a known-trusted hostname legitimately resolves to a
// private IP only on the control-plane network. See
// docs/runbooks/webhook-dns-validation.md §4b.
func TestValidateWebhookURL_DNS_AllowedDomains(t *testing.T) {
	cases := []struct {
		name        string
		allow       []string
		host        string
		ip          string
		wantAllowed bool
	}{
		// Exact-match entries.
		{"exact_match_bypasses", []string{"webhook.example.com"}, "webhook.example.com", "10.0.0.1", true},
		{"exact_no_match_still_rejects", []string{"webhook.example.com"}, "other.example.com", "10.0.0.1", false},
		// Suffix-match entries (leading dot).
		{"suffix_match_bypasses", []string{".example.com"}, "webhook.example.com", "10.0.0.1", true},
		{"suffix_match_subsubdomain", []string{".example.com"}, "api.eu.example.com", "10.0.0.1", true},
		// Critical: leading-dot suffix must NOT match a domain that
		// merely *contains* the suffix in the middle. Without the
		// leading-dot anchor an attacker could register
		// `attacker.example.com.evil.com` and bypass the gate.
		{"suffix_no_match_on_substring", []string{".example.com"}, "attacker.example.com.evil.com", "10.0.0.1", false},
		// Empty / whitespace entries are skipped so a typo in the
		// CSV form doesn't accidentally match every host.
		{"empty_entry_skipped", []string{"", " ", "webhook.example.com"}, "webhook.example.com", "10.0.0.1", true},
		{"all_empty_no_bypass", []string{"", " ", "\t"}, "webhook.example.com", "10.0.0.1", false},
		// Case-insensitive matching on the entry side; host is
		// already lowercased by the caller.
		{"case_insensitive_entry", []string{"WEBHOOK.EXAMPLE.COM"}, "webhook.example.com", "10.0.0.1", true},
		// Public IP path is unaffected by the allowlist.
		{"public_ip_unaffected", []string{".example.com"}, "public.example.com", "8.8.8.8", true},
		// Empty allowlist preserves the historical reject-on-private
		// behaviour exactly.
		{"empty_allowlist_rejects_private", nil, "internal.example.com", "10.0.0.1", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &Service{
				WebhookValidateDNS:    true,
				WebhookAllowedDomains: c.allow,
				WebhookHostResolver: func(ctx context.Context, host string) ([]netip.Addr, error) {
					addr, err := netip.ParseAddr(c.ip)
					if err != nil {
						t.Fatalf("bad test ip %s: %v", c.ip, err)
					}
					return []netip.Addr{addr}, nil
				},
			}
			err := svc.validateWebhookURLForService(context.Background(), "https://"+c.host+"/hook")
			if c.wantAllowed && err != nil {
				t.Fatalf("host %s (allow=%v) should be allowed: %v", c.host, c.allow, err)
			}
			if !c.wantAllowed && err == nil {
				t.Fatalf("host %s (allow=%v) should be rejected", c.host, c.allow)
			}
		})
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

func TestUpdateUserRoleDryRun(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateUserRole(context.Background(), map[string]any{
		"user_id": "u1",
		"role":    "WORKSPACE_ADMIN",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("update user role dry run failed: %v", err)
	}
	if result.Action != "clockify_update_user_role" {
		t.Fatalf("expected action clockify_update_user_role, got %s", result.Action)
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok || dataMap["dry_run"] != true {
		t.Fatalf("expected dry-run data map, got %#v", result.Data)
	}
	payload, ok := dataMap["payload"].(map[string]any)
	if !ok || payload["role"] != "WORKSPACE_ADMIN" {
		t.Fatalf("unexpected dry-run payload: %#v", dataMap["payload"])
	}
}
