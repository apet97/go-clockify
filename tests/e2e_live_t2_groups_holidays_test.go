//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestLiveOneUserGroupsHolidaysCRUD covers the current groups and holidays
// tools end to end through the MCP path: a user group is created, fetched,
// renamed, and deleted; a holiday is created and deleted.
//
// Gated by CLOCKIFY_LIVE_ADMIN_ENABLED=true because user-group
// membership and holiday calendar entries affect every workspace
// member; the gate keeps the test off when the maintainer doesn't
// want admin-class mutations.
func TestLiveOneUserGroupsHolidaysCRUD(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_ADMIN_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	groupName := c.LivePrefix("ug", 0)
	var groupID string

	t.Run("create_user_group_admin", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_groups_create", map[string]any{
			"name": groupName,
		})
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("clockify_groups_create returned no id: %#v", data)
		}
		groupID = id
		c.RegisterCleanup("user-group", id, func(ctx context.Context) error {
			return c.rawDeletePath(ctx, "/user-groups/"+id)
		})
	})

	t.Run("get_user_group_scans_list_endpoint", func(t *testing.T) {
		if groupID == "" {
			t.Skip("group not created")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// The live per-id GET route returns 405, so the handler uses
		// the supported LIST endpoint and returns the matching group.
		result := h.callOK(ctx, "clockify_groups_get", map[string]any{
			"group_id": groupID,
		})
		data := extractDataMap(t, result)
		if got, _ := data["id"].(string); got != groupID {
			t.Fatalf("get_user_group id mismatch: got %q want %q; data=%#v", got, groupID, data)
		}
	})

	t.Run("update_user_group_admin_renames", func(t *testing.T) {
		if groupID == "" {
			t.Skip("group not created")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		newName := groupName + "-renamed"
		result := h.callOK(ctx, "clockify_groups_update", map[string]any{
			"group_id": groupID,
			"name":     newName,
		})
		data := extractDataMap(t, result)
		gotName, _ := data["name"].(string)
		if gotName != newName {
			t.Fatalf("rename failed: got %q want %q", gotName, newName)
		}
	})

	t.Run("real_delete_user_group_admin", func(t *testing.T) {
		if groupID == "" {
			t.Skip("group not created")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = h.callOK(ctx, "clockify_groups_delete", map[string]any{
			"group_id": groupID,
		})
		// Independent verification via LIST (per-id GET is 405 on
		// this Clockify version).
		var groups []map[string]any
		if err := c.rawGetPath(ctx, "/user-groups", &groups); err != nil {
			t.Fatalf("list user-groups raw: %v", err)
		}
		for _, g := range groups {
			if id, _ := g["id"].(string); id == groupID {
				t.Fatalf("group %s still present after delete", groupID)
			}
		}
	})

	t.Run("create_holiday_with_datePeriod_envelope", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// SUMMARY rev 3 #8: body must carry datePeriod.{startDate,
		// endDate} plus at least one of users.ids / userGroups.ids,
		// and "recurring" is renamed to "occursAnnually". Single-day
		// holiday +1 year out keeps the calendar clean — clean up
		// via the raw client below regardless of asserts.
		startDate := time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02")
		result := h.callOK(ctx, "clockify_holidays_create", map[string]any{
			"name":       c.LivePrefix("hol", 0),
			"start_date": startDate,
			"user_ids":   []any{c.OwnerUserID},
		})
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("clockify_holidays_create returned no id: %#v", data)
		}
		dp, _ := data["datePeriod"].(map[string]any)
		if dp == nil || dp["startDate"] == nil {
			t.Fatalf("create_holiday missing datePeriod in response: %#v", data)
		}
		c.RegisterCleanup("holiday", id, func(ctx context.Context) error {
			return c.rawDeletePath(ctx, "/holidays/"+id)
		})
	})

	t.Run("delete_holiday_for_nonexistent_id", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Cannot exercise the success path until create_holiday is
		// fixed. We do verify the error path with a known-bogus id
		// — the handler should propagate the upstream not-found
		// envelope cleanly.
		errMsg := h.callExpectError(ctx, "clockify_holidays_delete", map[string]any{
			"holiday_id": "000000000000000000000001",
		})
		if !strings.Contains(strings.ToLower(errMsg), "not found") &&
			!strings.Contains(errMsg, "doesn't belong to Workspace") &&
			!strings.Contains(errMsg, "404") {
			t.Fatalf("expected not-found-style error for bogus holiday id, got: %q", errMsg)
		}
	})
}
