package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

func TestOneUserToolDescriptionsCallOutOperationalSideEffects(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	tools := map[string]mcp.Tool{}
	for _, descriptor := range svc.FullAccessRegistry() {
		tools[descriptor.Tool.Name] = descriptor.Tool
	}

	cases := map[string][]string{
		"clockify_invoices_send":            {"send", "email", "external side effect", "dry_run"},
		"clockify_webhooks_test":            {"test delivery", "external side effect", "dry_run"},
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
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	tools := map[string]mcp.Tool{}
	for _, descriptor := range svc.FullAccessRegistry() {
		tools[descriptor.Tool.Name] = descriptor.Tool
	}

	for _, name := range []string{"clockify_reports_export", "clockify_invoices_export"} {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("missing tool %s", name)
		}
		desc := strings.ToLower(tool.Description)
		for _, term := range []string{"contenttype", "filename", "bytes", "bodyencoding", "base64bytes", "truncated:false", "body", "base64 payload"} {
			if !strings.Contains(desc, term) {
				t.Fatalf("%s description %q missing safe envelope term %q", name, tool.Description, term)
			}
		}
	}
}

func TestOneUserWorkflowAnnotationsGiveActionableRoutingHints(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	for _, descriptor := range svc.FullAccessRegistry() {
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
