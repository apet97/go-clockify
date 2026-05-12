//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

// TestLiveTagCRUDDocCoverage exercises the tag CRUD tools through the MCP
// path against a prefixed sacrificial tag. Tags support direct DELETE, so the
// cleanup path only needs the tag ID when the test fails before delete_tag.
func TestLiveTagCRUDDocCoverage(t *testing.T) {
	requireWriteEnabled(t)

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	created := h.callOK(ctx, "clockify_create_tag", map[string]any{
		"name": c.LivePrefix("doc-tag", 0),
	})
	tagID, _ := extractDataMap(t, created)["id"].(string)
	if tagID == "" {
		t.Fatalf("clockify_create_tag returned no id: %#v", created)
	}

	deleted := false
	c.RegisterCleanup("tag", tagID, func(ctx context.Context) error {
		if deleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/tags/"+tagID)
	})

	_ = h.callOK(ctx, "clockify_get_tag", map[string]any{"tag": tagID})

	updatedName := c.LivePrefix("doc-tag-updated", 0)
	updated := h.callOK(ctx, "clockify_update_tag", map[string]any{
		"tag":      tagID,
		"name":     updatedName,
		"archived": false,
	})
	updatedData := extractDataMap(t, updated)
	if got, _ := updatedData["name"].(string); got != updatedName {
		t.Fatalf("clockify_update_tag name=%q, want %q (data=%#v)", got, updatedName, updatedData)
	}

	byName := h.callOK(ctx, "clockify_get_tag", map[string]any{"tag": updatedName})
	if got, _ := extractDataMap(t, byName)["id"].(string); got != tagID {
		t.Fatalf("clockify_get_tag by updated name returned id=%q, want %q", got, tagID)
	}

	deletedResult := h.callOK(ctx, "clockify_delete_tag", map[string]any{"tag": tagID})
	deletedData := extractDataMap(t, deletedResult)
	if ok, _ := deletedData["deleted"].(bool); !ok {
		t.Fatalf("clockify_delete_tag response missing deleted=true: %#v", deletedData)
	}
	deleted = true
}
