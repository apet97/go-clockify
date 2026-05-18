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
// clockify_entries_create semantics.
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
			if body["role"] != "WORKSPACE_ADMIN" {
				t.Fatalf("role POST body role=%v, want WORKSPACE_ADMIN", body["role"])
			}
			respondJSON(t, w, map[string]any{"id": userID, "role": "WORKSPACE_ADMIN"})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/users/"+userID):
			respondJSON(t, w, map[string]any{"id": userID, "role": "WORKSPACE_ADMIN", "status": "ACTIVE"})
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
		"role":    "WORKSPACE_ADMIN",
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

func TestUpdateUserRoleAcceptsArrayResponse(t *testing.T) {
	const userID = "u1"
	const wsID = "w1"

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			respondJSON(t, w, map[string]any{"id": "self-admin", "name": "Admin"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/users/"+userID+"/roles"):
			respondJSON(t, w, []map[string]any{{"role": "WORKSPACE_ADMIN", "entityId": wsID}})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	svc := New(client, wsID)
	res, err := svc.UpdateUserRole(context.Background(), map[string]any{
		"user_id": userID,
		"role":    "WORKSPACE_ADMIN",
	})
	if err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}
	if _, ok := res.Data.([]any); ok {
		return
	}
	if _, ok := res.Data.([]map[string]any); !ok {
		t.Fatalf("expected array response to survive decoding, got %T %#v", res.Data, res.Data)
	}
}

// TestUpdateUserRoleREGULARDeletesExplicitElevatedGrant verifies that
// role=REGULAR strips elevated grants with Clockify's DELETE-with-body role
// endpoint instead of treating REGULAR as a direct assignable role.
func TestUpdateUserRoleREGULARDeletesExplicitElevatedGrant(t *testing.T) {
	const userID = "u1"
	const wsID = "w1"

	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			respondJSON(t, w, map[string]any{"id": "self-admin", "name": "Admin"})
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/users/"+userID+"/roles"):
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode role DELETE body: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, wsID)

	res, err := svc.UpdateUserRole(context.Background(), map[string]any{
		"user_id": userID,
		"role":    "REGULAR",
		"role_grants": []any{map[string]any{
			"role":      "WORKSPACE_ADMIN",
			"entity_id": wsID,
		}},
	})
	if err != nil {
		t.Fatalf("UpdateUserRole REGULAR: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok result: %+v", res)
	}
	if gotBody["role"] != "WORKSPACE_ADMIN" || gotBody["entityId"] != wsID {
		t.Fatalf("DELETE role body = %#v, want WORKSPACE_ADMIN/%s", gotBody, wsID)
	}
	data, ok := res.Data.(map[string]any)
	if !ok || data["role"] != "REGULAR" {
		t.Fatalf("unexpected REGULAR response data: %#v", res.Data)
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
