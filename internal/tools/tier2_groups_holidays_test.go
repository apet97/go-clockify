package tools

import (
	"context"
	"net/http"
	"testing"
)

// TestTier2_GroupsHolidays_FullSweep covers user-group + holiday admin
// handlers via a mocked Clockify API.
func TestTier2_GroupsHolidays_FullSweep(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/user-groups":
			respondJSON(t, w, []map[string]any{{"id": "g1", "name": "Engineering"}})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/user-groups/g1":
			respondJSON(t, w, map[string]any{"id": "g1", "name": "Engineering"})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/user-groups":
			respondJSON(t, w, map[string]any{"id": "g-new", "name": "Design"})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/user-groups/g1":
			respondJSON(t, w, map[string]any{"id": "g1", "name": "Renamed"})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/user-groups/g1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/holidays":
			respondJSON(t, w, []map[string]any{{"id": "h1", "name": "New Year"}})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/holidays":
			respondJSON(t, w, map[string]any{"id": "h-new", "name": "Memorial Day"})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/holidays/h1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	client, cleanup := newTestClient(t, mux.ServeHTTP)
	defer cleanup()
	svc := New(client, "ws1")
	ctx := context.Background()

	// User groups
	res, err := svc.ListUserGroupsAdmin(ctx)
	mustOK(t, res, err, "clockify_list_user_groups_admin")

	res, err = svc.GetUserGroup(ctx, map[string]any{"group_id": "g1"})
	mustOK(t, res, err, "clockify_get_user_group")
	if _, err := svc.GetUserGroup(ctx, map[string]any{"group_id": ""}); err == nil {
		t.Fatal("expected validation error for empty group_id")
	}

	res, err = svc.CreateUserGroupAdmin(ctx, map[string]any{
		"name":     "Design",
		"user_ids": []any{"u1", "u2"},
	})
	mustOK(t, res, err, "clockify_create_user_group_admin")
	if _, err := svc.CreateUserGroupAdmin(ctx, map[string]any{"name": ""}); err == nil {
		t.Fatal("expected validation error for missing name")
	}

	res, err = svc.UpdateUserGroupAdmin(ctx, map[string]any{
		"group_id": "g1",
		"name":     "Renamed",
		"user_ids": []any{"u3"},
	})
	mustOK(t, res, err, "clockify_update_user_group_admin")
	if _, err := svc.UpdateUserGroupAdmin(ctx, map[string]any{"group_id": ""}); err == nil {
		t.Fatal("expected validation error for empty group_id")
	}

	// Delete user group: dry-run + executed + validation
	res, err = svc.DeleteUserGroupAdmin(ctx, map[string]any{"group_id": "g1", "dry_run": true})
	mustOK(t, res, err, "clockify_delete_user_group_admin")
	res, err = svc.DeleteUserGroupAdmin(ctx, map[string]any{"group_id": "g1"})
	mustOK(t, res, err, "clockify_delete_user_group_admin")
	if _, err := svc.DeleteUserGroupAdmin(ctx, map[string]any{"group_id": ""}); err == nil {
		t.Fatal("expected validation error for empty group_id")
	}

	// Holidays
	res, err = svc.ListHolidays(ctx)
	mustOK(t, res, err, "clockify_list_holidays")

	res, err = svc.CreateHoliday(ctx, map[string]any{
		"name":      "Memorial Day",
		"date":      "2026-05-25",
		"recurring": true,
	})
	mustOK(t, res, err, "clockify_create_holiday")
	if _, err := svc.CreateHoliday(ctx, map[string]any{"name": "", "date": "2026-05-25"}); err == nil {
		t.Fatal("expected validation error for missing name")
	}
	if _, err := svc.CreateHoliday(ctx, map[string]any{"name": "x", "date": ""}); err == nil {
		t.Fatal("expected validation error for missing date")
	}

	// Delete holiday: dry-run + executed + validation
	res, err = svc.DeleteHoliday(ctx, map[string]any{"holiday_id": "h1", "dry_run": true})
	mustOK(t, res, err, "clockify_delete_holiday")
	res, err = svc.DeleteHoliday(ctx, map[string]any{"holiday_id": "h1"})
	mustOK(t, res, err, "clockify_delete_holiday")
	if _, err := svc.DeleteHoliday(ctx, map[string]any{"holiday_id": ""}); err == nil {
		t.Fatal("expected validation error for empty holiday_id")
	}
}
