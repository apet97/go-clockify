package resolve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestValidateID(t *testing.T) {
	if err := ValidateID("abc123", "project"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateID("", "project"); err == nil {
		t.Fatal("expected empty id error")
	}
	if err := ValidateID("bad/path", "project"); err == nil {
		t.Fatal("expected invalid char error")
	}
}

func TestLooksLikeClockifyID(t *testing.T) {
	if !looksLikeClockifyID("5e1b2c3d4e5f6a7b8c9d0e1f") {
		t.Fatal("expected valid clockify id")
	}
	if looksLikeClockifyID("not-an-id") {
		t.Fatal("did not expect non-id to pass")
	}
}

func TestLooksLikeEmail(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"user@example.com", true},
		{"just a name", false},
		{"@", false},
		{"a@b", false},
		{"a@b.c", true},
	}
	for _, tt := range tests {
		got := looksLikeEmail(tt.input)
		if got != tt.want {
			t.Errorf("looksLikeEmail(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*clockify.Client, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	client := clockify.NewClient("test-key", ts.URL, 5*time.Second, 0)
	return client, ts.Close
}

func TestResolveUserByEmailSuggestion(t *testing.T) {
	users := []map[string]any{
		{"id": "aaa111bbb222ccc333ddd444", "name": "Alice Smith", "email": "alice@example.com"},
		{"id": "bbb222ccc333ddd444eee555", "name": "Bob Jones", "email": "bob@example.com"},
	}

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})
	defer cleanup()

	ctx := context.Background()
	id, err := ResolveUserID(ctx, client, "ws123", "alice@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "aaa111bbb222ccc333ddd444" {
		t.Fatalf("expected Alice's ID, got %s", id)
	}
}

func TestResolveUserByNameMultiField(t *testing.T) {
	users := []map[string]any{
		{"id": "aaa111bbb222ccc333ddd444", "name": "Alice Smith", "email": "alice@example.com"},
		{"id": "bbb222ccc333ddd444eee555", "name": "Bob Jones", "email": "bob@example.com"},
	}

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})
	defer cleanup()

	ctx := context.Background()

	// Search by name
	id, err := ResolveUserID(ctx, client, "ws123", "Bob Jones")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "bbb222ccc333ddd444eee555" {
		t.Fatalf("expected Bob's ID, got %s", id)
	}

	// Name-based search also matches email field
	id, err = ResolveUserID(ctx, client, "ws123", "alice@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "aaa111bbb222ccc333ddd444" {
		t.Fatalf("expected Alice's ID from email match, got %s", id)
	}
}

func TestImprovedErrorNotFound(t *testing.T) {
	// Test user not found (uses ResolveUserID path)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{})
	})
	defer cleanup()

	ctx := context.Background()
	_, err := ResolveUserID(ctx, client, "ws123", "nobody")
	if err == nil {
		t.Fatal("expected error for not found user")
	}
	if !strings.Contains(err.Error(), "Use clockify_list_users") {
		t.Fatalf("error should suggest listing users, got: %v", err)
	}

	// Test project not found (uses resolveByNameOrID path)
	_, err = ResolveProjectID(ctx, client, "ws123", "nonexistent")
	if err == nil {
		t.Fatal("expected error for not found project")
	}
	if !strings.Contains(err.Error(), "Use clockify_list_projects") {
		t.Fatalf("error should suggest listing projects, got: %v", err)
	}
}

func TestImprovedErrorMultiple(t *testing.T) {
	// Test multiple users match
	users := []map[string]any{
		{"id": "aaa111bbb222ccc333ddd444", "name": "Alex", "email": "alex1@example.com"},
		{"id": "bbb222ccc333ddd444eee555", "name": "Alex", "email": "alex2@example.com"},
	}

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})
	defer cleanup()

	ctx := context.Background()
	_, err := ResolveUserID(ctx, client, "ws123", "Alex")
	if err == nil {
		t.Fatal("expected error for multiple matching users")
	}
	if !strings.Contains(err.Error(), "2 found") {
		t.Fatalf("error should contain count, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Use the full user ID") {
		t.Fatalf("error should suggest using full ID, got: %v", err)
	}

	// Test multiple projects match (uses resolveByNameOrID path)
	projects := []map[string]any{
		{"id": "aaa111bbb222ccc333ddd444", "name": "MyProject"},
		{"id": "bbb222ccc333ddd444eee555", "name": "MyProject"},
	}

	client2, cleanup2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects)
	})
	defer cleanup2()

	_, err = ResolveProjectID(ctx, client2, "ws123", "MyProject")
	if err == nil {
		t.Fatal("expected error for multiple matching projects")
	}
	if !strings.Contains(err.Error(), "2 found") {
		t.Fatalf("error should contain count, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Use the full project ID") {
		t.Fatalf("error should suggest using full ID, got: %v", err)
	}
}
