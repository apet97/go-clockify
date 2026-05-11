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
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			// Self lookup used by the self-modification guard in
			// UpdateUserRole. Returning a different ID than the
			// modification target ensures the guard passes and we
			// reach the role-change POST below.
			respondJSON(t, w, map[string]any{"id": "self-admin", "name": "Admin"})
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

// TestUpdateUserRoleREGULARReturnsAccurateError verifies that calling
// UpdateUserRole with role=REGULAR returns a clear error explaining that
// REGULAR strips elevated grants (not removes the user), why the MCP cannot
// do it yet, and that clockify_deactivate_user is NOT a substitute.
// No HTTP call should be made.
func TestUpdateUserRoleREGULARReturnsAccurateError(t *testing.T) {
	const userID = "u1"
	const wsID = "w1"

	called := false
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})
	defer cleanup()

	svc := New(client, wsID)

	_, err := svc.UpdateUserRole(context.Background(), map[string]any{
		"user_id": userID,
		"role":    "REGULAR",
	})
	if err == nil {
		t.Fatal("expected an error for role=REGULAR, got nil")
	}
	msg := err.Error()

	// Must describe what REGULAR actually does (strips elevated grants).
	if !strings.Contains(msg, "strips") && !strings.Contains(msg, "elevated") {
		t.Errorf("error message must mention 'strips' or 'elevated'; got: %q", msg)
	}

	// Must explicitly state clockify_deactivate_user is NOT a substitute.
	if !strings.Contains(msg, "NOT a substitute") {
		t.Errorf("error message must contain 'NOT a substitute'; got: %q", msg)
	}

	// Must NOT recommend clockify_deactivate_user as the right tool.
	if strings.Contains(msg, "use clockify_deactivate_user") {
		t.Errorf("error message must not recommend 'use clockify_deactivate_user'; got: %q", msg)
	}

	// No HTTP call should have been made.
	if called {
		t.Error("no upstream HTTP call should be made for role=REGULAR")
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
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			// Self lookup used by the self-deactivation guard in
			// DeactivateUser. ID differs from the target so the
			// guard passes and the test exercises the real path.
			respondJSON(t, w, map[string]any{"id": "self-admin", "name": "Admin"})
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

// TestUpdateUserRoleRefusesSelfModification pins the new self-guard.
// When the API key owner targets their own user ID, the call must
// short-circuit before issuing the POST so the operator cannot
// accidentally strip their own access. The stub intentionally
// returns the same self.ID as the modification target.
func TestUpdateUserRoleRefusesSelfModification(t *testing.T) {
	const selfID = "u-self"
	const wsID = "w1"

	postCalls := 0
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			respondJSON(t, w, map[string]any{"id": selfID, "name": "Owner"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/users/"+selfID+"/roles"):
			postCalls++
			respondJSON(t, w, map[string]any{"id": selfID})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	svc := New(client, wsID)
	_, err := svc.UpdateUserRole(context.Background(), map[string]any{
		"user_id": selfID,
		"role":    "TEAM_MANAGER",
	})
	if err == nil {
		t.Fatal("expected error from self-modification guard, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to change the API key owner") {
		t.Fatalf("error message does not mention self-modification guard: %v", err)
	}
	if postCalls != 0 {
		t.Fatalf("self-modification guard fired too late: %d role POSTs reached upstream", postCalls)
	}
}

// TestDeactivateUserRefusesSelf pins the equivalent guard on the
// deactivate path. A successful upstream PUT here would lock the
// operator out of the workspace; the guard must short-circuit
// before the PUT issues.
func TestDeactivateUserRefusesSelf(t *testing.T) {
	const selfID = "u-self"
	const wsID = "w1"

	putCalls := 0
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			respondJSON(t, w, map[string]any{"id": selfID, "name": "Owner"})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/users/"+selfID):
			putCalls++
			respondJSON(t, w, map[string]any{"id": selfID, "status": "INACTIVE"})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	svc := New(client, wsID)
	_, err := svc.DeactivateUser(context.Background(), map[string]any{
		"user_id": selfID,
		"dry_run": false,
	})
	if err == nil {
		t.Fatal("expected error from self-deactivation guard, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to deactivate the API key owner") {
		t.Fatalf("error message does not mention self-deactivation guard: %v", err)
	}
	if putCalls != 0 {
		t.Fatalf("self-deactivation guard fired too late: %d deactivate PUTs reached upstream", putCalls)
	}
}
