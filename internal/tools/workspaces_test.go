package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestResolveWorkspaceID_RejectsMalformedExplicitID pins the
// defence-in-depth check added to ResolveWorkspaceID: a programmatically
// wired workspace ID (Service.WorkspaceID set via New) must still go
// through resolve.ValidateID even though config.LoadOneUser already validates
// an env-supplied CLOCKIFY_WORKSPACE_ID. Without this guard, a future
// handler that consumes the workspace ID outside paths.Workspace would
// bypass validation, and the path-safety static test deliberately
// exempts workspace IDs (it trusts config-load to have validated them).
//
// The fake upstream fails the test if hit — a malformed workspace ID
// must short-circuit before any HTTP call.
func TestResolveWorkspaceID_RejectsMalformedExplicitID(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("malformed workspace ID must not reach upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	cases := []struct {
		name    string
		wsID    string
		wantSub string
	}{
		{"slash", "abc/def", "invalid characters"},
		{"percent", "abc%2fdef", "invalid characters"},
		{"hash", "abc#def", "invalid characters"},
		{"question", "abc?def", "invalid characters"},
		{"only-whitespace", "   ", "cannot be empty"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := New(client, tc.wsID)
			_, err := svc.ResolveWorkspaceID(context.Background())
			if err == nil {
				t.Fatalf("expected error for malformed workspace ID %q, got nil", tc.wsID)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q must mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestResolveWorkspaceID_AcceptsValidExplicitID confirms the
// validation does not regress the common case: a well-formed
// workspace ID is returned unchanged with no upstream call.
func TestResolveWorkspaceID_AcceptsValidExplicitID(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("valid workspace ID must not need an upstream call; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws-abc-123")
	got, err := svc.ResolveWorkspaceID(context.Background())
	if err != nil {
		t.Fatalf("unexpected error for valid workspace ID: %v", err)
	}
	if got != "ws-abc-123" {
		t.Fatalf("expected ws-abc-123, got %q", got)
	}
}
