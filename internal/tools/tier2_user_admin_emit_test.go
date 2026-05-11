package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestUserResourceURI verifies the URI builder returns the canonical
// shape and safely degrades to the empty string when a required piece
// is missing. Callers rely on that behaviour to skip the emit step
// instead of pushing a malformed URI into the subscription set.
func TestUserResourceURI(t *testing.T) {
	cases := []struct {
		name, ws, user, want string
	}{
		{"happy_path", "w1", "u1", "clockify://workspace/w1/user/u1"},
		{"missing_workspace", "", "u1", ""},
		{"missing_user", "w1", "", ""},
		{"both_missing", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := userResourceURI(c.ws, c.user)
			if got != c.want {
				t.Fatalf("userResourceURI(%q, %q) = %q, want %q", c.ws, c.user, got, c.want)
			}
		})
	}
}

// TestUpdateUserRoleEmitsUserURI covers the W4-04a wiring on
// UpdateUserRole: after the PUT succeeds, the handler emits a
// notifications/resources/updated for the user URI. Cache is cold so
// the first notification carries format=none — matching the existing
// AddEntry semantics.
func TestUpdateUserRoleEmitsUserURI(t *testing.T) {
	const userID = "u1"
	const wsID = "w1"

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/users/"+userID+"/roles"):
			// Live API requires `entityId` (the workspace ID) alongside
			// `role`. Without it the upstream rejects with code 3000
			// even on POST. Pinned 2026-05-11 after QA agent 29 found
			// the missing field by probing the live endpoint.
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode role POST body: %v", err)
			}
			if body["entityId"] != wsID {
				t.Fatalf("role POST body missing entityId=%q, got %#v", wsID, body)
			}
			if body["role"] != "PROJECT_MANAGER" {
				t.Fatalf("role POST body role=%v, want PROJECT_MANAGER", body["role"])
			}
			respondJSON(t, w, map[string]any{"id": userID, "role": "PROJECT_MANAGER"})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/users/"+userID):
			respondJSON(t, w, map[string]any{"id": userID, "role": "PROJECT_MANAGER", "status": "ACTIVE"})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()

	_, err := svc.UpdateUserRole(context.Background(), map[string]any{
		"user_id": userID,
		"role":    "PROJECT_MANAGER",
	})
	if err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}

	calls := emit.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected 1 emit, got %d: %+v", len(calls), calls)
	}
	want := "clockify://workspace/" + wsID + "/user/" + userID
	if calls[0].URI != want {
		t.Fatalf("URI = %q, want %q", calls[0].URI, want)
	}
	if calls[0].Delta.Format != "none" {
		t.Fatalf("first emit should be format=none, got %q", calls[0].Delta.Format)
	}
}

// TestDeactivateUserEmitsUserURI mirrors the above for the
// DeactivateUser mutation path: after the INACTIVE PUT succeeds, the
// user URI is emitted with format=none on a cold cache.
func TestDeactivateUserEmitsUserURI(t *testing.T) {
	const userID = "u2"
	const wsID = "w1"

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/users/"+userID):
			respondJSON(t, w, map[string]any{"id": userID, "status": "INACTIVE"})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/users/"+userID):
			respondJSON(t, w, map[string]any{"id": userID, "status": "INACTIVE"})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	svc := New(client, wsID)
	emit := &recordingEmit{}
	svc.EmitResourceUpdate = emit.hook()

	_, err := svc.DeactivateUser(context.Background(), map[string]any{
		"user_id": userID,
		"dry_run": false,
	})
	if err != nil {
		t.Fatalf("DeactivateUser: %v", err)
	}

	calls := emit.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected 1 emit, got %d: %+v", len(calls), calls)
	}
	want := "clockify://workspace/" + wsID + "/user/" + userID
	if calls[0].URI != want {
		t.Fatalf("URI = %q, want %q", calls[0].URI, want)
	}
}
