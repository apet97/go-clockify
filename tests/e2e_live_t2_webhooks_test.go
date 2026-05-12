//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

func TestLiveT2WebhooksCRUD(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_FULL_SURFACE_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("webhooks")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	webhookName := c.RunID + "-wh"
	updatedWebhookName := c.RunID + "-w2"
	create := h.callOK(ctx, "clockify_create_webhook", map[string]any{
		"name":                webhookName,
		"url":                 "https://example.com/clockify-mcp-live",
		"webhook_event":       "NEW_TIME_ENTRY",
		"trigger_source_type": "WORKSPACE_ID",
		"trigger_source":      []any{c.WorkspaceID},
	})
	webhookID, _ := extractDataMap(t, create)["id"].(string)
	if webhookID == "" {
		t.Fatalf("clockify_create_webhook returned no id: %#v", create)
	}
	webhookDeleted := false
	c.RegisterCleanup("webhook", webhookID, func(ctx context.Context) error {
		if webhookDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/webhooks/"+webhookID)
	})

	got := h.callOK(ctx, "clockify_get_webhook", map[string]any{"webhook_id": webhookID})
	gotData := extractDataMap(t, got)
	if gotID, _ := gotData["id"].(string); gotID != webhookID {
		t.Fatalf("clockify_get_webhook id mismatch: got %q want %q", gotID, webhookID)
	}
	if event, _ := gotData["webhookEvent"].(string); event != "NEW_TIME_ENTRY" {
		t.Fatalf("clockify_get_webhook event mismatch: got %q data=%#v", event, gotData)
	}

	updated := h.callOK(ctx, "clockify_update_webhook", map[string]any{
		"webhook_id":    webhookID,
		"name":          updatedWebhookName,
		"webhook_event": "TIMER_STOPPED",
	})
	if event, _ := extractDataMap(t, updated)["webhookEvent"].(string); event != "TIMER_STOPPED" {
		t.Fatalf("clockify_update_webhook did not update event: %#v", updated)
	}

	events := extractList(t, h.callOK(ctx, "clockify_list_webhook_events", nil))
	if len(events) < 2 {
		t.Fatalf("clockify_list_webhook_events returned too few events: %#v", events)
	}

	_ = h.callOK(ctx, "clockify_test_webhook", map[string]any{
		"webhook_id": webhookID,
		"dry_run":    true,
	})

	_ = h.callOK(ctx, "clockify_delete_webhook", map[string]any{
		"webhook_id": webhookID,
		"dry_run":    true,
	})
	_ = h.callOK(ctx, "clockify_delete_webhook", map[string]any{"webhook_id": webhookID})
	webhookDeleted = true
}
