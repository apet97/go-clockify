package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/apet97/go-clockify/internal/jsonschema"
)

func assertQueryValue(t *testing.T, query url.Values, key, want string) {
	t.Helper()
	if got := query.Get(key); got != want {
		t.Fatalf("query %s = %q, want %q (full query: %s)", key, got, want, query.Encode())
	}
}

// TestTier2_GroupsHolidays_FullSweep covers user-group + holiday admin
// handlers via a mocked Clockify API.
func TestTier2_GroupsHolidays_FullSweep(t *testing.T) {
	mux := http.NewServeMux()
	addedGroupUsers := map[string]bool{}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/user-groups":
			respondJSON(t, w, []map[string]any{{"id": "g1", "name": "Engineering"}})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/user-groups/g1":
			respondJSON(t, w, map[string]any{"id": "g1", "name": "Engineering"})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/user-groups":
			respondJSON(t, w, map[string]any{"id": "g-new", "name": "Design"})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/user-groups/g-new/users":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("add group user parse body: %v", err)
			}
			uid, _ := body["userId"].(string)
			if uid == "" {
				t.Fatalf("add group user missing userId: %v", body)
			}
			addedGroupUsers[uid] = true
			respondJSON(t, w, map[string]any{"id": "g-new", "userId": uid})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/user-groups/g1":
			respondJSON(t, w, map[string]any{"id": "g1", "name": "Renamed"})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/user-groups/g1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/holidays":
			respondJSON(t, w, []map[string]any{{"id": "h1", "name": "New Year"}})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/holidays":
			// Pin SUMMARY rev 3 #8: body must carry a nested
			// datePeriod and at least one of users/userGroups.
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("create_holiday parse body: %v", err)
			}
			dp, ok := body["datePeriod"].(map[string]any)
			if !ok || dp["startDate"] == nil || dp["endDate"] == nil {
				t.Fatalf("create_holiday missing datePeriod.start/end: %#v", body)
			}
			if body["users"] == nil && body["userGroups"] == nil {
				t.Fatalf("create_holiday must include users or userGroups: %#v", body)
			}
			respondJSON(t, w, map[string]any{
				"id":         "h-new",
				"name":       body["name"],
				"datePeriod": dp,
			})
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
	res, err := svc.ListUserGroupsAdmin(ctx, map[string]any{})
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
	for _, uid := range []string{"u1", "u2"} {
		if !addedGroupUsers[uid] {
			t.Fatalf("CreateUserGroupAdmin did not add requested user %s; added=%v", uid, addedGroupUsers)
		}
	}
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
	if res.Meta["count"] != 1 || res.Meta["total"] != 1 || res.Meta["page"] != 1 || res.Meta["has_more"] != false {
		t.Fatalf("holidays list meta should include count/total/page/pageSize/has_more, got %#v", res.Meta)
	}

	res, err = svc.CreateHoliday(ctx, map[string]any{
		"name":            "Memorial Day",
		"start_date":      "2026-05-25",
		"end_date":        "2026-05-25",
		"occurs_annually": true,
		"user_ids":        []any{"u1"},
	})
	mustOK(t, res, err, "clockify_create_holiday")
	holiday := res.Data.(map[string]any)
	datePeriod := holiday["date_period"].(map[string]any)
	if datePeriod["start_date"] != "2026-05-25" || datePeriod["end_date"] != "2026-05-25" {
		t.Fatalf("create holiday missing snake_case date_period: %#v", holiday)
	}
	if _, err := svc.CreateHoliday(ctx, map[string]any{"name": "", "start_date": "2026-05-25", "user_ids": []any{"u1"}}); err == nil {
		t.Fatal("expected validation error for missing name")
	}
	if _, err := svc.CreateHoliday(ctx, map[string]any{"name": "x", "start_date": "", "user_ids": []any{"u1"}}); err == nil {
		t.Fatal("expected validation error for missing start_date")
	}
	if _, err := svc.CreateHoliday(ctx, map[string]any{"name": "x", "start_date": "2026-05-25"}); err == nil {
		t.Fatal("expected validation error when both user_ids and user_group_ids are missing")
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

func TestListUserGroupsAdminForwardsDocumentedFilters(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/user-groups" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		query := r.URL.Query()
		assertQueryValue(t, query, "page", "2")
		assertQueryValue(t, query, "page-size", "25")
		assertQueryValue(t, query, "project-id", "p1")
		assertQueryValue(t, query, "name", "Engineering")
		assertQueryValue(t, query, "sort-column", "NAME")
		assertQueryValue(t, query, "sort-order", "DESCENDING")
		assertQueryValue(t, query, "includeTeamManagers", "true")
		respondJSON(t, w, []map[string]any{{"id": "g1", "name": "Engineering"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.ListUserGroupsAdmin(context.Background(), map[string]any{
		"page":                  2,
		"page_size":             25,
		"project_id":            "p1",
		"name":                  "Engineering",
		"sort_column":           "name",
		"sort_order":            "descending",
		"include_team_managers": true,
	})
	mustOK(t, res, err, "clockify_list_user_groups_admin")
	if res.Meta["count"] != 1 {
		t.Fatalf("expected count=1 metadata, got %#v", res.Meta)
	}
}

func TestCreateHolidayInputSchemaRequiresAssignment(t *testing.T) {
	svc := New(nil, "ws1")
	descriptors, ok := tier2Handlers(svc, "groups_holidays")
	if !ok {
		t.Fatal("missing groups_holidays handlers")
	}
	var schema map[string]any
	for _, d := range descriptors {
		if d.Tool.Name == "clockify_create_holiday" {
			schema = d.Tool.InputSchema
			break
		}
	}
	if schema == nil {
		t.Fatal("clockify_create_holiday descriptor not found")
	}

	// The input schema must be a plain object: Anthropic tool input schemas
	// reject a top-level anyOf/oneOf/allOf, which breaks subagent launches.
	for _, k := range []string{"anyOf", "oneOf", "allOf"} {
		if _, present := schema[k]; present {
			t.Fatalf("schema must not carry a top-level %q", k)
		}
	}

	// The "at least one of user_ids/user_group_ids" rule is enforced by the
	// handler instead. Missing assignment is rejected before any API call.
	if _, err := svc.CreateHoliday(context.Background(), map[string]any{
		"name":       "Memorial Day",
		"start_date": "2026-05-25",
	}); err == nil {
		t.Fatal("expected CreateHoliday to reject holidays without user_ids or user_group_ids")
	}
	if _, err := svc.CreateHoliday(context.Background(), map[string]any{
		"name":       "Memorial Day",
		"start_date": "2026-05-25",
		"user_ids":   []any{},
	}); err == nil {
		t.Fatal("expected CreateHoliday to reject empty user_ids")
	}
}

func TestListHolidaysInPeriodSchemaAllowsUserIDAlias(t *testing.T) {
	svc := New(nil, "ws1")
	descriptors, ok := tier2Handlers(svc, "groups_holidays")
	if !ok {
		t.Fatal("missing groups_holidays handlers")
	}
	var schema map[string]any
	for _, d := range descriptors {
		if d.Tool.Name == "clockify_list_holidays_in_period" {
			schema = d.Tool.InputSchema
			break
		}
	}
	if schema == nil {
		t.Fatal("clockify_list_holidays_in_period descriptor not found")
	}

	// user_id is a documented alias for assigned_to that the handler honors.
	// The schema must accept a user_id-only call; listing assigned_to in
	// required made schema validation reject the alias before the handler ran.
	if err := jsonschema.Validate(schema, map[string]any{
		"user_id": "u1",
		"start":   "2026-05-01",
		"end":     "2026-05-31",
	}); err != nil {
		t.Fatalf("user_id alias should satisfy the schema: %v", err)
	}
	if err := jsonschema.Validate(schema, map[string]any{
		"assigned_to": "u1",
		"start":       "2026-05-01",
		"end":         "2026-05-31",
	}); err != nil {
		t.Fatalf("assigned_to should satisfy the schema: %v", err)
	}

	// The "assigned_to or user_id" rule is enforced by the handler instead.
	if _, err := svc.ListHolidaysInPeriod(context.Background(), map[string]any{
		"start": "2026-05-01",
		"end":   "2026-05-31",
	}); err == nil {
		t.Fatal("expected ListHolidaysInPeriod to reject a missing assignee")
	}
}
