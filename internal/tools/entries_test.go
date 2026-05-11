package tools

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

// otherUserEntryID is a 24-char hex value the resolver treats as a
// valid Clockify ObjectID so the handler reaches the ownership guard
// rather than short-circuiting on resolve.ValidateID.
const otherUserEntryID = "6a00f6bc2568d3d293061e2a"

// TestUpdateEntryRejectsOtherUserEntry pins the documented contract
// for personal time-entry mutations: docs/policy/production-tool-scope.md
// states "Mutations are constrained to the API key owner's own
// entries". `internal/tools/entries.go` UpdateEntry currently fetches
// the entry via the admin path /workspaces/{ws}/time-entries/{id}
// and never compares the returned UserID to the current user; with an
// elevated API key it would happily PUT another user's entry. This
// test seeds the fake so the fetched entry has UserID="user-OTHER"
// while /user reports "user-SELF", then asserts (1) the handler
// returns a permission-denied error and (2) no PUT is issued.
//
// Fails RED on this commit; goes GREEN when the ownership guard
// lands in UpdateEntry.
func TestUpdateEntryRejectsOtherUserEntry(t *testing.T) {
	var putCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-OTHER",
				Description:  "not mine",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodPut:
			putCalls.Add(1)
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.UpdateEntry(context.Background(), map[string]any{
		"entry_id":    otherUserEntryID,
		"description": "rename hostile",
	})
	if err == nil {
		t.Fatal("expected ownership error; UpdateEntry permitted mutation across users")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not owned") &&
		!strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		t.Fatalf("expected ownership-flavored error, got %q", err.Error())
	}
	if got := putCalls.Load(); got != 0 {
		t.Fatalf("ownership guard must short-circuit before PUT; saw %d PUT call(s)", got)
	}
}

// TestDeleteEntryRejectsOtherUserEntry mirrors UpdateEntry's pin for
// the destructive sibling. The DELETE must not be issued when the
// fetched entry belongs to a different user.
func TestDeleteEntryRejectsOtherUserEntry(t *testing.T) {
	var deleteCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-OTHER",
				Description:  "not mine",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodDelete:
			deleteCalls.Add(1)
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.DeleteEntry(context.Background(), map[string]any{
		"entry_id": otherUserEntryID,
	})
	if err == nil {
		t.Fatal("expected ownership error; DeleteEntry permitted mutation across users")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not owned") &&
		!strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		t.Fatalf("expected ownership-flavored error, got %q", err.Error())
	}
	if got := deleteCalls.Load(); got != 0 {
		t.Fatalf("ownership guard must short-circuit before DELETE; saw %d DELETE call(s)", got)
	}
}
