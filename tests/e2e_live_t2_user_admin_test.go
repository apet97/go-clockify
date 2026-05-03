//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

func TestLiveT2UserAdminCRUDAndOwnerSafety(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_ADMIN_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("user_admin")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	created := h.callOK(ctx, "clockify_create_user_group", map[string]any{
		"name": c.LivePrefix("ua", 0),
	})
	groupID, _ := extractDataMap(t, created)["id"].(string)
	if groupID == "" {
		t.Fatalf("clockify_create_user_group returned no id: %#v", created)
	}
	groupDeleted := false
	c.RegisterCleanup("user-admin-group", groupID, func(ctx context.Context) error {
		if groupDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/user-groups/"+groupID)
	})

	updated := h.callOK(ctx, "clockify_update_user_group", map[string]any{
		"group_id": groupID,
		"name":     c.LivePrefix("ua-renamed", 0),
	})
	if gotID, _ := extractDataMap(t, updated)["id"].(string); gotID != groupID {
		t.Fatalf("clockify_update_user_group id mismatch: got %q want %q", gotID, groupID)
	}

	_ = h.callOK(ctx, "clockify_add_user_to_group", map[string]any{
		"group_id": groupID,
		"user_id":  c.OwnerUserID,
	})

	_ = h.callOK(ctx, "clockify_remove_user_from_group", map[string]any{
		"group_id": groupID,
		"user_id":  c.OwnerUserID,
		"dry_run":  true,
	})
	_ = h.callOK(ctx, "clockify_remove_user_from_group", map[string]any{
		"group_id": groupID,
		"user_id":  c.OwnerUserID,
	})

	roleErr := h.callExpectError(ctx, "clockify_update_user_role", map[string]any{
		"user_id": "000000000000000000000001",
		"role":    "REGULAR",
	})
	if !containsErrorText(roleErr, "not found", "doesn't belong", "404", "400", "405", "method not allowed", "permission") {
		t.Fatalf("expected not-found/permission/method-style update_user_role error, got %q", roleErr)
	}

	dryDeactivate := h.callOK(ctx, "clockify_deactivate_user", map[string]any{
		"user_id": c.OwnerUserID,
		"dry_run": true,
	})
	if dryRun, _ := extractDataMap(t, dryDeactivate)["dry_run"].(bool); !dryRun {
		t.Fatalf("clockify_deactivate_user dry-run did not return dry_run=true: %#v", dryDeactivate)
	}

	_ = h.callOK(ctx, "clockify_delete_user_group", map[string]any{
		"group_id": groupID,
		"dry_run":  true,
	})
	_ = h.callOK(ctx, "clockify_delete_user_group", map[string]any{"group_id": groupID})
	groupDeleted = true
}

func containsErrorText(s string, needles ...string) bool {
	lower := strings.ToLower(s)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
