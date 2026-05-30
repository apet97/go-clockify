package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

func TestOneUserToolDescriptionsCallOutOperationalSideEffects(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	tools := map[string]mcp.Tool{}
	for _, descriptor := range mustRegistry(t, svc) {
		tools[descriptor.Tool.Name] = descriptor.Tool
	}

	cases := map[string][]string{
		"clockify_invoices_send_guidance":   {"does not expose", "email", "clockify ui"},
		"clockify_webhooks_test_guidance":   {"test delivery", "does not expose", "real event"},
		"clockify_users_invite":             {"invite", "email", "external side effect", "dry_run"},
		"clockify_invoices_delete":          {"delete", "dry_run"},
		"clockify_webhooks_delete":          {"delete", "destructive", "dry_run"},
		"clockify_users_deactivate":         {"deactivate", "remove access", "dry_run"},
		"clockify_time_off_requests_create": {"create", "time off", "approval"},
		"clockify_time_off_requests_delete": {"delete", "time off", "dry_run"},
		"clockify_time_off_approve":         {"approve", "approval state"},
		"clockify_time_off_deny":            {"deny", "approval state"},
		"clockify_approvals_submit":         {"submit", "approval request"},
		"clockify_approvals_approve":        {"approve", "update", "state"},
		"clockify_approvals_reject":         {"reject", "update", "state"},
		"clockify_approvals_withdraw":       {"withdraw", "update", "state"},
		"clockify_approvals_resubmit":       {"resubmit", "approval state"},
	}
	for name, wantTerms := range cases {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("missing tool %s", name)
		}
		desc := strings.ToLower(tool.Description)
		if strings.Contains(desc, blockedTerm("hos", "ted")) || strings.Contains(desc, blockedTerm("policy ", "mode")) || strings.Contains(desc, "activation") {
			t.Fatalf("%s description contains forbidden product language: %q", name, tool.Description)
		}
		for _, term := range wantTerms {
			if !strings.Contains(desc, strings.ToLower(term)) {
				t.Fatalf("%s description %q missing %q", name, tool.Description, term)
			}
		}
	}
}

func TestOneUserBinaryExportToolDescriptionsNameSafeEnvelope(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	tools := map[string]mcp.Tool{}
	for _, descriptor := range mustRegistry(t, svc) {
		tools[descriptor.Tool.Name] = descriptor.Tool
	}

	for _, name := range []string{"clockify_reports_export", "clockify_invoices_export"} {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("missing tool %s", name)
		}
		desc := strings.ToLower(tool.Description)
		for _, term := range []string{"contenttype", "filename", "bytes", "bodyencoding", "base64bytes", "truncated:false", "body", "base64 payload", "path"} {
			if !strings.Contains(desc, term) {
				t.Fatalf("%s description %q missing safe envelope term %q", name, tool.Description, term)
			}
		}
	}
}

func TestOneUserWorkflowAnnotationsGiveActionableRoutingHints(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	for _, descriptor := range mustRegistry(t, svc) {
		if descriptor.Tool.Annotations["category"] != "workflow" {
			continue
		}
		bestFor, ok := descriptor.Tool.Annotations["bestFor"].([]string)
		if !ok || len(bestFor) == 0 {
			t.Fatalf("%s missing non-empty bestFor annotation: %+v", descriptor.Tool.Name, descriptor.Tool.Annotations)
		}
		if _, ok := descriptor.Tool.Annotations["preferOver"].([]string); !ok {
			t.Fatalf("%s missing preferOver annotation: %+v", descriptor.Tool.Name, descriptor.Tool.Annotations)
		}
	}
}

func TestOneUserInputParameterDescriptionsAreUseful(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	for _, descriptor := range mustRegistry(t, svc) {
		props, _ := descriptor.Tool.InputSchema["properties"].(map[string]any)
		for name, raw := range props {
			prop, _ := raw.(map[string]any)
			desc, _ := prop["description"].(string)
			if len(strings.TrimSpace(desc)) < 12 {
				t.Fatalf("%s.%s description is not useful enough for MCP contract audit: %q", descriptor.Tool.Name, name, desc)
			}
		}
	}
}

func TestOneUserDestructiveLookingToolsAdvertiseDestructiveHint(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	for _, descriptor := range mustRegistry(t, svc) {
		if !contractAuditDestructiveName(descriptor.Tool.Name) {
			continue
		}
		if descriptor.Tool.Annotations["destructiveHint"] != true {
			t.Fatalf("%s looks destructive to MCP contract audit but destructiveHint=%v", descriptor.Tool.Name, descriptor.Tool.Annotations["destructiveHint"])
		}
	}
}

func contractAuditDestructiveName(name string) bool {
	if strings.HasSuffix(name, "_guidance") {
		return false
	}
	destructiveTerms := map[string]bool{
		"delete":  true,
		"remove":  true,
		"destroy": true,
		"archive": true,
		"purge":   true,
		"send":    true,
		"publish": true,
		"invite":  true,
		"revoke":  true,
		"charge":  true,
		"bill":    true,
		"grant":   true,
		"disable": true,
		"cancel":  true,
		"close":   true,
	}
	for part := range strings.FieldsSeq(strings.ReplaceAll(name, "_", " ")) {
		if destructiveTerms[part] {
			return true
		}
	}
	return false
}

func TestOneUserSuggestedActionsReferenceRegisteredTools(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "000000000000000000000001")
	registered := map[string]bool{}
	for _, descriptor := range mustRegistry(t, svc) {
		registered[descriptor.Tool.Name] = true
	}

	suggestions := approvalSuggestedActions(ApprovalView{
		ID:      "65b382b606de527a7ee2b610",
		Actions: []string{"approve", "reject", "withdraw", "resubmit"},
	})
	if len(suggestions) != 4 {
		t.Fatalf("suggestions = %d, want 4: %+v", len(suggestions), suggestions)
	}
	for _, suggestion := range suggestions {
		if !registered[suggestion.Tool] {
			t.Fatalf("suggested action references unregistered tool %q: %+v", suggestion.Tool, suggestion)
		}
	}
}
