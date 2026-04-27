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
	if err := ValidateID("5e1b2c3d4e5f6a7b8c9d0e1f", "project"); err != nil {
		t.Fatalf("valid clockify ID should pass: %v", err)
	}
	if err := ValidateID("", "project"); err == nil {
		t.Fatal("expected empty id error")
	}
	if err := ValidateID("bad/path", "project"); err == nil {
		t.Fatal("expected invalid char error for /")
	}
	if err := ValidateID("bad?query", "project"); err == nil {
		t.Fatal("expected invalid char error for ?")
	}
	if err := ValidateID("bad#frag", "project"); err == nil {
		t.Fatal("expected invalid char error for #")
	}
	if err := ValidateID("bad%2Fpath", "project"); err == nil {
		t.Fatal("expected invalid char error for %")
	}
	if err := ValidateID("../../../etc/passwd", "project"); err == nil {
		t.Fatal("expected invalid char error for ..")
	}
	if err := ValidateID("foo..bar", "project"); err == nil {
		t.Fatal("expected invalid char error for embedded ..")
	}
	if err := ValidateID("bad\x00null", "project"); err == nil {
		t.Fatal("expected invalid char error for null byte")
	}
	if err := ValidateID("bad\x1Fcontrol", "project"); err == nil {
		t.Fatal("expected invalid char error for control char")
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

// TestResolveProjectID_RejectsPrefixMatch verifies the strict-name-search
// contract: a user asking for "My" must not resolve to "MyProject" even
// if the server returns a permissive search result. The local
// exactMatches pass demands full equality; anything looser would be a
// security regression (prefix-based privilege escalation).
func TestResolveProjectID_RejectsPrefixMatch(t *testing.T) {
	// Simulate a server that ignored strict-name-search and returned
	// prefix matches anyway.
	projects := []map[string]any{
		{"id": "aaa111bbb222ccc333ddd444", "name": "MyProject"},
		{"id": "bbb222ccc333ddd444eee555", "name": "MyProjectTwo"},
	}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(projects)
	})
	defer cleanup()

	_, err := ResolveProjectID(context.Background(), client, "ws123", "My")
	if err == nil {
		t.Fatal("expected prefix 'My' not to resolve to MyProject* via loose match")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error (strict contract), got: %v", err)
	}
}

// TestResolveProjectID_CaseFoldingContract confirms the resolver's
// documented case-insensitive exact-match behaviour: an input that
// differs only in case must still resolve to the single matching
// project. This is intentionally lenient (operators rarely care about
// case) but is tied to the exactMatches contract via strings.EqualFold
// — if someone replaces EqualFold with strings.Compare a regression
// would silently break existing integrations.
func TestResolveProjectID_CaseFoldingContract(t *testing.T) {
	projects := []map[string]any{
		{"id": "aaa111bbb222ccc333ddd444", "name": "MyProject"},
	}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(projects)
	})
	defer cleanup()

	id, err := ResolveProjectID(context.Background(), client, "ws123", "myproject")
	if err != nil {
		t.Fatalf("expected case-folded match to resolve, got: %v", err)
	}
	if id != "aaa111bbb222ccc333ddd444" {
		t.Fatalf("expected MyProject ID, got %s", id)
	}
}

// TestValidateNameRef locks the contract for the new permissive
// validator: legitimate Clockify names containing /, %, &, .. or
// other punctuation must pass (they reach the API as
// query-parameter values, which url.Values safely encodes), while
// empty / oversized / control-byte input still fails.
func TestValidateNameRef(t *testing.T) {
	good := []string{
		"ACME / Support",
		"R&D 50%",
		"Q1..Q2",
		"Ümlaut Café",
		"foo?bar#baz",
		"plain",
	}
	for _, s := range good {
		if err := ValidateNameRef(s, "project"); err != nil {
			t.Errorf("legitimate name %q rejected: %v", s, err)
		}
	}
	bad := []struct {
		ref  string
		hint string
	}{
		{"", "empty"},
		{"   ", "whitespace-only"},
		{"foo\x00bar", "embedded NUL"},
		{"foo\x1fbar", "embedded control byte"},
		{strings.Repeat("a", maxIDLength+1), "oversized"},
	}
	for _, c := range bad {
		if err := ValidateNameRef(c.ref, "project"); err == nil {
			t.Errorf("%s should be rejected: %q", c.hint, c.ref)
		}
	}
}

// TestResolveProjectID_AcceptsNameWithPunctuation locks audit
// finding 2: a workspace name containing characters that ValidateID
// rejects (slash, ampersand, percent) must still resolve when the
// caller passes the name itself, because the lookup goes via a
// query-parameter and never touches a URL path. Pre-fix the strict
// validator rejected the input before the safe path could run.
func TestResolveProjectID_AcceptsNameWithPunctuation(t *testing.T) {
	projects := []map[string]any{
		{"id": "aaa111bbb222ccc333ddd444", "name": "ACME / Support"},
	}
	var seenName string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seenName = r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(projects)
	})
	defer cleanup()

	id, err := ResolveProjectID(context.Background(), client, "ws123", "ACME / Support")
	if err != nil {
		t.Fatalf("name with slash should resolve via query-param lookup: %v", err)
	}
	if id != "aaa111bbb222ccc333ddd444" {
		t.Fatalf("expected project id, got %q", id)
	}
	if seenName != "ACME / Support" {
		t.Fatalf("expected exact name forwarded as query param, got %q", seenName)
	}
}

// TestResolveProjectID_StillRejectsPathInjectionShapedID makes sure
// the relaxed name path doesn't open the door to a Clockify-ID-shaped
// input that nonetheless contains a "/". A 24-char hex-shaped value
// IS treated as an ID and runs through ValidateID.
func TestResolveProjectID_StillRejectsPathInjectionShapedID(t *testing.T) {
	// 24-char hex literal containing a slash is impossible (slash is
	// not in the hex class), so looksLikeClockifyID rejects it and the
	// input falls into the name path — where ValidateNameRef accepts
	// it as a name and the lookup safely encodes via url.Values. The
	// real defence is ValidateID running only on path-bound inputs;
	// we lock that by exercising a near-ID-shaped string.
	weird := "abcd1234abcd1234abcd123/" // 24 chars, ends in slash → not a valid ID
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	defer cleanup()

	_, err := ResolveProjectID(context.Background(), client, "ws123", weird)
	if err == nil {
		t.Fatal("expected not-found, since the test server returns no matches")
	}
	// The error must be a not-found, NOT a validation error referencing
	// path characters: this proves the input took the (safe) name path.
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found via name path; got: %v", err)
	}
}

// FuzzValidateID feeds random strings into ValidateID and requires that it
// never panics. Errors are expected for malicious input.
func FuzzValidateID(f *testing.F) {
	seeds := []string{
		"",
		"abc123",
		"5e1b2c3d4e5f6a7b8c9d0e1f",
		"../etc/passwd",
		"a/b/c",
		"a?b=c",
		"a#b",
		"\x00",
		"very-long-" + strings.Repeat("x", 300),
		"\n\t  ",
		"with spaces",
		"uni\u202ecode",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_ = ValidateID(input, "fuzz")
	})
}
