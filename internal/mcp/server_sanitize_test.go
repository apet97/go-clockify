package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeUpstreamError mimics clockify.APIError shape via duck-typing —
// keeps the mcp package free of an internal/clockify import while
// proving the sanitizable interface in server.go works end-to-end.
type fakeUpstreamError struct {
	verbose   string
	sanitized string
}

func (e *fakeUpstreamError) Error() string     { return e.verbose }
func (e *fakeUpstreamError) Sanitized() string { return e.sanitized }

// TestSanitizeUpstreamErrors_DefaultExposesBody locks in the
// developer-friendly default: tool errors flow through to the MCP
// client unchanged. Local-stdio operators rely on this for fast
// diagnostics; the body is not a per-tenant leak in a single-user
// install.
func TestSanitizeUpstreamErrors_DefaultExposesBody(t *testing.T) {
	wantBody := "insufficient permissions tenant=acme-internal"
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "leaky"},
		Handler: func(context.Context, map[string]any) (any, error) {
			return nil, &fakeUpstreamError{
				verbose:   "clockify PUT /workspaces/ws1/invoices/inv1 failed: 403 Forbidden: " + wantBody,
				sanitized: "clockify PUT /workspaces/ws1/invoices/inv1 failed: 403 Forbidden",
			}
		},
	}}, nil, nil)
	server.SanitizeUpstreamErrors = false

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"leaky","arguments":{}}}`,
	}, "\n")
	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, wantBody) {
		t.Fatalf("local-mode response should include verbose body, got: %s", got)
	}
}

// TestSanitizeUpstreamErrors_HostedDropsBody locks in audit finding 9
// fix: hosted profile flips SanitizeUpstreamErrors=true and the MCP
// client now sees the verb/path/status only — never the body. Mirrors
// the wire that shared-service / prod-postgres deployments use.
func TestSanitizeUpstreamErrors_HostedDropsBody(t *testing.T) {
	wantBody := "insufficient permissions tenant=acme-internal"
	wantStatus := "403 Forbidden"
	server := NewServer("test", []ToolDescriptor{{
		Tool: Tool{Name: "leaky"},
		Handler: func(context.Context, map[string]any) (any, error) {
			return nil, &fakeUpstreamError{
				verbose:   "clockify PUT /workspaces/ws1/invoices/inv1 failed: 403 Forbidden: " + wantBody,
				sanitized: "clockify PUT /workspaces/ws1/invoices/inv1 failed: " + wantStatus,
			}
		},
	}}, nil, nil)
	server.SanitizeUpstreamErrors = true

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"leaky","arguments":{}}}`,
	}, "\n")
	var out strings.Builder
	if err := server.Run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	got := out.String()
	if strings.Contains(got, wantBody) {
		t.Errorf("hosted-mode response leaked body: %s", got)
	}
	if !strings.Contains(got, wantStatus) {
		t.Errorf("hosted-mode response should keep status: %s", got)
	}
}

// TestSanitizeUpstreamErrors_NonSanitizableUnaffected — errors that
// don't implement Sanitized() (validation, transport, generic) keep
// their full message. We don't want the hosted flag to also redact
// programmer-facing schema validation messages.
func TestSanitizeUpstreamErrors_NonSanitizableUnaffected(t *testing.T) {
	got := sanitizeClientError(errors.New("boom: not sanitizable"))
	if got != "boom: not sanitizable" {
		t.Fatalf("non-sanitizable error mangled: %q", got)
	}
}
