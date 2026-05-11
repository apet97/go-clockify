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
