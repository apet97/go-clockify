//go:build legacy_platform && livee2e

package e2e_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/policy"
)

func TestLiveT2UserAdminCRUDAndOwnerSafety(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_ADMIN_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("user_admin")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	_ = h.callOK(ctx, "clockify_get_member_profile", map[string]any{
		"user_id": c.OwnerUserID,
	})
	dryProfile := h.callOK(ctx, "clockify_update_member_profile", map[string]any{
		"user_id":      c.OwnerUserID,
		"working_days": []any{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"},
		"dry_run":      true,
	})
	if dryRun, _ := extractDataMap(t, dryProfile)["dry_run"].(bool); !dryRun {
		t.Fatalf("clockify_update_member_profile dry-run did not return dry_run=true: %#v", dryProfile)
	}

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
	if !containsErrorText(roleErr, "not found", "doesn't belong", "404", "400", "405", "method not allowed", "permission", "role-strip helper", "requires delete") {
		t.Fatalf("expected not-found/permission/method/local-safety update_user_role error, got %q", roleErr)
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

func TestLiveT2UserInviteValidationProbe(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_ADMIN_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var out map[string]any
	err := h.Service.Client.Post(
		ctx,
		"/workspaces/"+c.WorkspaceID+"/users?send-email=false",
		map[string]any{"email": ""},
		&out,
	)
	if err == nil {
		t.Fatalf("invite validation probe unexpectedly succeeded; refusing to risk creating an invited user: %#v", out)
	}

	var apiErr *clockify.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("invite validation probe returned non-API error: %T %v", err, err)
	}
	switch apiErr.StatusCode {
	case 400, 402, 403, 405:
		// Expected safe outcomes: request validation, plan/permission
		// gating, or an unsupported method. The probe uses
		// send-email=false and an empty email so none of these send
		// mail or create a pending member.
	default:
		t.Fatalf("invite validation probe returned unexpected status %d: %s", apiErr.StatusCode, apiErr.Body)
	}
	if !containsErrorText(apiErr.Body, "email", "user", "subscription", "paid", "permission", "not supported", "method", "invalid", "required", "blank", "400", "403", "405") {
		t.Fatalf("invite validation probe body did not identify validation/permission/plan/method refusal: %q", apiErr.Body)
	}
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
