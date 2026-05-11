package tools

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

// TestFindAndUpdateEntryByIDRejectsOtherUserEntry pins the ownership
// contract on `clockify_find_and_update_entry`'s entry_id branch. The
// entry_id branch calls `findEntryByID` (workflows.go:391) which fetches
// the entry via the admin path /workspaces/{ws}/time-entries/{id};
// without an ownership guard, an elevated API key could rename, retime,
// or re-bill another user's entry through this tool. The other branch
// (search by description/range) already routes through the user-scoped
// /workspaces/{ws}/user/{userID}/time-entries path in `findSingleEntry`
// — this test pins the gap, not the safe path.
//
// Fails RED on this commit; goes GREEN when findEntryByID compares
// the fetched UserID to the current user.
func TestFindAndUpdateEntryByIDRejectsOtherUserEntry(t *testing.T) {
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
	_, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"entry_id":        otherUserEntryID,
		"new_description": "hostile rename",
	})
	if err == nil {
		t.Fatal("expected ownership error; FindAndUpdateEntry permitted entry_id-branch mutation across users")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not owned") &&
		!strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		t.Fatalf("expected ownership-flavored error, got %q", err.Error())
	}
	if got := putCalls.Load(); got != 0 {
		t.Fatalf("ownership guard must short-circuit before PUT; saw %d PUT call(s)", got)
	}
}

// TestFindAndUpdateEntryByIDRejectsEntryWithoutUserID pins the same
// fail-closed posture as the entries.go siblings: an entry without
// a userId cannot have its ownership proven, so the entry_id branch
// of clockify_find_and_update_entry must refuse to mutate.
func TestFindAndUpdateEntryByIDRejectsEntryWithoutUserID(t *testing.T) {
	var putCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				Description:  "no userId",
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
	_, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"entry_id":        otherUserEntryID,
		"new_description": "should not apply",
	})
	if err == nil {
		t.Fatal("expected ownership error; FindAndUpdateEntry permitted entry_id-branch mutation on entry with empty userId")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no userid") &&
		!strings.Contains(strings.ToLower(err.Error()), "ambiguous ownership") {
		t.Fatalf("expected ambiguous-ownership error, got %q", err.Error())
	}
	if got := putCalls.Load(); got != 0 {
		t.Fatalf("guard must short-circuit before PUT on missing userId; saw %d PUT call(s)", got)
	}
}

// TestFindAndUpdateEntryByIDPermitsOwnEntryAndIssuesPUT is the
// positive path for the entry_id branch: matching userId must allow
// the PUT to reach upstream.
func TestFindAndUpdateEntryByIDPermitsOwnEntryAndIssuesPUT(t *testing.T) {
	var putCalls atomic.Int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "user-SELF", Name: "Self"})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-SELF",
				Description:  "mine",
				ProjectID:    "p1",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z", End: "2026-05-01T10:00:00Z"},
			})
		case r.URL.Path == "/workspaces/ws1/time-entries/"+otherUserEntryID && r.Method == http.MethodPut:
			putCalls.Add(1)
			respondJSON(t, w, clockify.TimeEntry{
				ID:           otherUserEntryID,
				UserID:       "user-SELF",
				Description:  "renamed",
				ProjectID:    "p1",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-01T09:00:00Z", End: "2026-05-01T10:00:00Z"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.FindAndUpdateEntry(context.Background(), map[string]any{
		"entry_id":        otherUserEntryID,
		"new_description": "renamed",
	})
	if err != nil {
		t.Fatalf("FindAndUpdateEntry on own entry must succeed, got %v", err)
	}
	if got := putCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 PUT on own entry, got %d", got)
	}
}
