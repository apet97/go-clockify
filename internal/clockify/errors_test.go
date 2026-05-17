package clockify

import (
	"strings"
	"testing"
)

// TestAPIError_ErrorIncludesBody locks in the verbose form used in
// server-side logs and local-development MCP responses: method, path,
// status, and the upstream body all appear.
func TestAPIError_ErrorIncludesBody(t *testing.T) {
	e := &APIError{
		Method:     "PUT",
		Path:       "/workspaces/ws1/invoices/inv1",
		StatusCode: 403,
		Status:     "403 Forbidden",
		Body:       `{"error":"insufficient permissions","tenant":"acme-internal"}`,
	}
	got := e.Error()
	if !strings.Contains(got, "PUT") || !strings.Contains(got, "/invoices/inv1") {
		t.Errorf("verbose error missing method/path: %q", got)
	}
	if !strings.Contains(got, "insufficient permissions") || !strings.Contains(got, "acme-internal") {
		t.Errorf("verbose error must include body for ops debugging: %q", got)
	}
}

func TestAPIError_ErrorCompactsClockifyMessageCodeBody(t *testing.T) {
	e := &APIError{
		Method:     "POST",
		Path:       "/workspaces/ws1/projects",
		StatusCode: 400,
		Status:     "400 Bad Request",
		Body:       `{"message":"Project with this name already exists","code":501}`,
	}
	got := e.Error()
	if !strings.Contains(got, "Project with this name already exists") || !strings.Contains(got, "clockify_error_code=501") {
		t.Fatalf("compact error missing upstream message/code: %q", got)
	}
	if strings.Contains(got, "/workspaces/ws1/projects") {
		t.Fatalf("compact upstream error should omit workspace path: %q", got)
	}
}

// TestAPIError_SanitizedDropsBody locks in audit finding 9 fix: the
// sanitized form keeps the verb/path/status (those are always safe to
// surface) but omits Body, which can carry per-tenant identifiers.
func TestAPIError_SanitizedDropsBody(t *testing.T) {
	e := &APIError{
		Method:     "PUT",
		Path:       "/workspaces/ws1/invoices/inv1",
		StatusCode: 403,
		Status:     "403 Forbidden",
		Body:       `{"error":"insufficient permissions","tenant":"acme-internal"}`,
	}
	got := e.Sanitized()
	if !strings.Contains(got, "403 Forbidden") {
		t.Errorf("sanitized must keep status: %q", got)
	}
	if strings.Contains(got, "insufficient permissions") {
		t.Errorf("sanitized leaked body content: %q", got)
	}
	if strings.Contains(got, "acme-internal") {
		t.Errorf("sanitized leaked tenant identifier: %q", got)
	}
}

// TestAPIError_SanitizedNilSafe — the sanitizable interface is hit
// from a deferred error path, so a nil receiver should not panic.
func TestAPIError_SanitizedNilSafe(t *testing.T) {
	var e *APIError
	if got := e.Sanitized(); got != "" {
		t.Errorf("nil sanitized should be empty, got %q", got)
	}
}

func TestClientFacingMessageStripsPrefixAndCode(t *testing.T) {
	e := &APIError{
		Method:     "GET",
		Path:       "/workspaces/x/clients/y",
		StatusCode: 400,
		Status:     "400 Bad Request",
		Body:       `{"message":"Client doesn't belong to Workspace","code":501}`,
	}
	msg := e.ClientFacingMessage()
	if msg == "" {
		t.Fatalf("ClientFacingMessage returned empty")
	}
	for _, leak := range []string{"clockify_error_code", "/workspaces/", "failed:"} {
		if strings.Contains(msg, leak) {
			t.Fatalf("ClientFacingMessage leaked %q: %s", leak, msg)
		}
	}
}
