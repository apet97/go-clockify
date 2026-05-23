package safety

import (
	"strings"
	"testing"
	"time"
)

func TestTokenStoreAcceptsMatchingTokenOnceBeforeExpiry(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	store := NewTokenStore(TokenStoreOptions{TTL: 5 * time.Minute, Now: func() time.Time { return now }})
	payload := Payload{
		ToolName:    "clockify_projects_archive",
		WorkspaceID: "workspace_123",
		RiskClass:   "destructive",
		ArgsHash:    HashCanonical(map[string]any{"project_id": "p1"}),
	}

	token, previewHash, expiresAt, err := store.Issue(payload)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || previewHash == "" || !expiresAt.After(now) {
		t.Fatalf("bad token issue result")
	}
	if strings.Contains(token, payload.WorkspaceID) || strings.Contains(token, payload.ToolName) {
		t.Fatalf("token exposes payload: %q", token)
	}
	if err := store.Validate(token, payload); err != nil {
		t.Fatalf("Validate matching token failed: %v", err)
	}
	if err := store.Validate(token, payload); err == nil {
		t.Fatal("expected replayed token to fail")
	}
}

func TestTokenStoreRejectsMismatchedArgs(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	store := NewTokenStore(TokenStoreOptions{TTL: 5 * time.Minute, Now: func() time.Time { return now }})
	payload := Payload{
		ToolName:    "clockify_projects_archive",
		WorkspaceID: "workspace_123",
		RiskClass:   "destructive",
		ArgsHash:    HashCanonical(map[string]any{"project_id": "p1"}),
	}
	token, _, _, err := store.Issue(payload)
	if err != nil {
		t.Fatal(err)
	}
	other := payload
	other.ArgsHash = HashCanonical(map[string]any{"project_id": "p2"})
	if err := store.Validate(token, other); err == nil {
		t.Fatal("expected mismatched args to fail")
	}
}

func TestTokenStoreRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	store := NewTokenStore(TokenStoreOptions{TTL: time.Minute, Now: func() time.Time { return now }})
	payload := Payload{ToolName: "tool", WorkspaceID: "workspace", RiskClass: "admin", ArgsHash: "hash"}
	token, _, _, err := store.Issue(payload)
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return now.Add(2 * time.Minute) }
	if err := store.Validate(token, payload); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestHashCanonicalIgnoresControlFieldsAndIsStable(t *testing.T) {
	a := HashCanonical(map[string]any{
		"project_id":    "p1",
		"dry_run":       true,
		"confirm_token": "secret-token",
		"nested": map[string]any{
			"confirm_token": "nested-secret",
			"keep":          []any{"x", float64(2)},
		},
	})
	b := HashCanonical(map[string]any{
		"nested": map[string]any{
			"keep": []any{"x", float64(2)},
		},
		"project_id": "p1",
	})
	if a != b {
		t.Fatalf("canonical hashes differ: %s != %s", a, b)
	}
}
