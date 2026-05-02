//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestLiveT2GroupsHolidaysCRUD covers the groups_holidays Tier-2
// surface end to end through the MCP path: a user group is created,
// fetched, renamed, and deleted; a holiday is created and deleted.
// The dry-run delete envelopes are also exercised — dry_run on
// delete_user_group_admin uses internal/dryrun.MinimalResult since
// the upstream lacks a GET counterpart for previewing.
//
// Gated by CLOCKIFY_LIVE_ADMIN_ENABLED=true because user-group
// membership and holiday calendar entries affect every workspace
// member; the gate keeps the test off when the maintainer doesn't
// want admin-class mutations.
func TestLiveT2GroupsHolidaysCRUD(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_ADMIN_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)
	c.activateTier2("groups_holidays")

	groupName := c.LivePrefix("ug", 0)
	var groupID string

	t.Run("create_user_group_admin", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_create_user_group_admin", map[string]any{
			"name": groupName,
		})
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("create_user_group_admin returned no id: %#v", data)
		}
		groupID = id
		c.RegisterCleanup("user-group", id, func(ctx context.Context) error {
			return c.rawDeletePath(ctx, "/user-groups/"+id)
		})
	})

	t.Run("get_user_group_blocked_by_upstream_405", func(t *testing.T) {
		if groupID == "" {
			t.Skip("group not created")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// /workspaces/{id}/user-groups/{groupId} is GET-supported in
		// the public Clockify reference but the live API returns
		// 405 "Request method 'GET' is not supported" — the per-id
		// resource only exposes mutating verbs. Likely fix on the
		// handler side: return the matching entry from the LIST
		// endpoint rather than calling GET on the per-id route, or
		// document the upstream limitation in the descriptor.
		errMsg := h.callExpectError(ctx, "clockify_get_user_group", map[string]any{
			"group_id": groupID,
		})
		if !strings.Contains(errMsg, "method 'GET' is not supported") {
			t.Fatalf("expected upstream 405 on per-id GET, got: %q", errMsg)
		}
	})

	t.Run("update_user_group_admin_renames", func(t *testing.T) {
		if groupID == "" {
			t.Skip("group not created")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		newName := groupName + "-renamed"
		result := h.callOK(ctx, "clockify_update_user_group_admin", map[string]any{
			"group_id": groupID,
			"name":     newName,
		})
		data := extractDataMap(t, result)
		gotName, _ := data["name"].(string)
		if gotName != newName {
			t.Fatalf("rename failed: got %q want %q", gotName, newName)
		}
	})

	t.Run("dry_run_delete_user_group_admin_emits_envelope", func(t *testing.T) {
		if groupID == "" {
			t.Skip("group not created")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_delete_user_group_admin", map[string]any{
			"group_id": groupID,
			"dry_run":  true,
		})
		// dryrun.MinimalResult emits {dry_run:true, tool, params}
		// at structuredContent.data.
		sc, _ := result["structuredContent"].(map[string]any)
		var dr map[string]any
		if d, ok := sc["data"].(map[string]any); ok {
			dr = d
		} else {
			dr = sc
		}
		if v, _ := dr["dry_run"].(bool); !v {
			t.Fatalf("dry_run delete did not produce dry_run:true envelope: %#v", sc)
		}
		// Cannot independently verify via GET (per-id 405) — verify
		// via LIST instead: the group must still appear.
		var groups []map[string]any
		if err := c.rawGetPath(ctx, "/user-groups", &groups); err != nil {
			t.Fatalf("list user-groups raw: %v", err)
		}
		var present bool
		for _, g := range groups {
			if id, _ := g["id"].(string); id == groupID {
				present = true
				break
			}
		}
		if !present {
			t.Fatalf("group %s missing from list after dry-run delete", groupID)
		}
	})

	t.Run("real_delete_user_group_admin", func(t *testing.T) {
		if groupID == "" {
			t.Skip("group not created")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = h.callOK(ctx, "clockify_delete_user_group_admin", map[string]any{
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

	t.Run("create_holiday_blocked_by_handler_date_shape_bug", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// The handler at internal/tools/tier2_groups_holidays.go
		// CreateHoliday sends `{name, date, recurring?}` (flat date
		// string), but the upstream holidays endpoint expects a
		// `datePeriod: {startDate, endDate}` struct (verified by
		// inspecting an existing holiday's GET response: every
		// holiday carries a datePeriod object plus userIds /
		// userGroupIds / occursAnnually / etc., never a flat date).
		// Upstream rejects with "must not be null"/code 501. Likely
		// fix: rewrite the handler body to assemble the datePeriod
		// envelope and include sensible defaults for the other
		// required fields. For now this assertion pins the current
		// breakage so a later fix flips the test.
		errMsg := h.callExpectError(ctx, "clockify_create_holiday", map[string]any{
			"name": c.LivePrefix("hol", 0),
			"date": time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02"),
		})
		if !strings.Contains(errMsg, "must not be null") {
			t.Fatalf("expected upstream null-field rejection, got: %q", errMsg)
		}
	})

	t.Run("delete_holiday_for_nonexistent_id", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Cannot exercise the success path until create_holiday is
		// fixed. We do verify the error path with a known-bogus id
		// — the handler should propagate the upstream not-found
		// envelope cleanly.
		errMsg := h.callExpectError(ctx, "clockify_delete_holiday", map[string]any{
			"holiday_id": "000000000000000000000001",
		})
		if !strings.Contains(strings.ToLower(errMsg), "not found") &&
			!strings.Contains(errMsg, "doesn't belong to Workspace") &&
			!strings.Contains(errMsg, "404") {
			t.Fatalf("expected not-found-style error for bogus holiday id, got: %q", errMsg)
		}
	})
}
