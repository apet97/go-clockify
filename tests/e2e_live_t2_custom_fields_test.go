//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestLiveOneUserCustomFieldsCRUD covers the current custom-field tools end
// to end through the MCP path: a test project is created, a TEXT custom field
// is created on it, the field is renamed, fetched, value-set, deleted, and
// finally raw-verified to be gone.
//
// Gated by CLOCKIFY_LIVE_SETTINGS_ENABLED=true because creating /
// deleting custom fields shifts schema-level metadata visible to
// every workspace member; the gate keeps the test off when the
// maintainer doesn't want to touch settings.
func TestLiveOneUserCustomFieldsCRUD(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_SETTINGS_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	// A dedicated test project keeps set_custom_field_value scoped
	// to entities this run owns; the workspace already contains 100+
	// pre-existing projects we must not touch.
	projectName := c.LivePrefix("cf-host", 0)
	var projectID string
	t.Run("seed_project", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_create_work_package", map[string]any{
			"project": projectName,
		})
		ids := extractIDs(t, result)
		id := ids["projectId"]
		if id == "" {
			t.Fatalf("clockify_create_work_package returned no projectId: %#v", ids)
		}
		projectID = id
		c.RegisterCleanup("project", id, func(ctx context.Context) error {
			return c.rawArchiveAndDeleteProject(ctx, id)
		})
	})

	fieldName := c.LivePrefix("cf-text", 0)
	var fieldID string
	fieldDeleted := false
	t.Run("create_custom_field", func(t *testing.T) {
		if projectID == "" {
			t.Skip("seed project unavailable")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// The descriptor's docstring advertises "TEXT, NUMBER,
		// DROPDOWN, CHECKBOX, LINK" but Clockify's actual enum is
		// {TXT, NUMBER, DROPDOWN_SINGLE, DROPDOWN_MULTIPLE, CHECKBOX,
		// LINK}. The handler upper-cases the input but does not
		// translate, so callers must use the upstream-accurate value.
		// This is a documented descriptor-vs-upstream drift; the
		// handler's docstring should be updated to reflect the
		// upstream enum.
		resp, err := h.rawCall(ctx, "clockify_custom_fields_create", map[string]any{
			"name":       fieldName,
			"field_type": "TXT",
		})
		if err != nil {
			t.Fatalf("rawCall: %v", err)
		}
		// Workspace-state precondition: Clockify caps custom fields
		// at 50 per workspace. When the cap is hit the test cannot
		// run until the maintainer prunes existing fields. We detect
		// the cap via the upstream's specific error and skip cleanly,
		// rather than failing — the test logic is correct, the
		// workspace is full.
		if rmap, ok := resp.Result.(map[string]any); ok {
			if isErr, _ := rmap["isError"].(bool); isErr {
				content, _ := rmap["content"].([]any)
				var msg string
				for _, c := range content {
					if e, ok := c.(map[string]any); ok {
						if txt, ok := e["text"].(string); ok {
							msg += txt
						}
					}
				}
				if strings.Contains(msg, "limit of 50 custom fields") {
					t.Skipf("workspace at custom-field limit (50/50) — prune existing fields and re-run: %s", msg)
				}
				t.Fatalf("create_custom_field returned isError: %s", msg)
			}
		}
		rmap, ok := resp.Result.(map[string]any)
		if !ok {
			t.Fatalf("result was not a map: %T", resp.Result)
		}
		data := extractDataMap(t, rmap)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("clockify_custom_fields_create returned no id: %#v", data)
		}
		ftype, _ := data["type"].(string)
		if ftype != "TXT" {
			t.Fatalf("custom field type expected TXT, got %q", ftype)
		}
		fieldID = id
		c.RegisterCleanup("custom-field", id, func(ctx context.Context) error {
			if fieldDeleted {
				return nil
			}
			return c.rawDeletePath(ctx, "/custom-fields/"+id)
		})
	})

	t.Run("get_custom_field_round_trips_id", func(t *testing.T) {
		if fieldID == "" {
			t.Skip("create_custom_field did not produce an id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_custom_fields_get", map[string]any{
			"field_id": fieldID,
		})
		data := extractDataMap(t, result)
		got, _ := data["id"].(string)
		if got != fieldID {
			t.Fatalf("get_custom_field id round-trip: got %q want %q", got, fieldID)
		}
	})

	t.Run("update_custom_field_renames", func(t *testing.T) {
		if fieldID == "" {
			t.Skip("create_custom_field did not produce an id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		newName := fieldName + "-renamed"
		result := h.callOK(ctx, "clockify_custom_fields_update", map[string]any{
			"field_id": fieldID,
			"name":     newName,
		})
		data := extractDataMap(t, result)
		gotName, _ := data["name"].(string)
		if gotName != newName {
			t.Fatalf("update_custom_field rename: got %q want %q", gotName, newName)
		}
	})

	t.Run("set_custom_field_value_on_project", func(t *testing.T) {
		if fieldID == "" || projectID == "" {
			t.Skip("missing field or project id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// On a TEXT field, the upstream accepts a string value.
		result := h.callOK(ctx, "clockify_custom_fields_set_value", map[string]any{
			"field_id":   fieldID,
			"project_id": projectID,
			"value":      c.LivePrefix("cf-value", 0),
		})
		// Just confirm the response carries ok=true; per-shape
		// inspection of the upstream's set-value response varies
		// across Clockify versions and isn't worth pinning here.
		sc := structuredContentMap(t, result)
		if okFlag, _ := sc["ok"].(bool); !okFlag {
			t.Fatalf("set_custom_field_value response carried ok=false: %#v", sc)
		}
	})

	t.Run("real_delete_removes_field", func(t *testing.T) {
		if fieldID == "" {
			t.Skip("create_custom_field did not produce an id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = h.callOK(ctx, "clockify_custom_fields_delete", map[string]any{
			"field_id": fieldID,
		})
		fieldDeleted = true
		// Independent verification: the upstream no longer returns
		// the field in the supported list route.
		exists, err := liveCustomFieldExists(ctx, c, fieldID)
		if err != nil {
			t.Fatalf("list custom fields after delete: %v", err)
		}
		if exists {
			t.Fatalf("field %s still readable after delete", fieldID)
		}
	})
}

func liveCustomFieldExists(ctx context.Context, c *liveCampaignContext, fieldID string) (bool, error) {
	var fields []map[string]any
	if err := c.rawGetPath(ctx, "/custom-fields", &fields); err != nil {
		return false, err
	}
	for _, field := range fields {
		if id, _ := field["id"].(string); id == fieldID {
			return true, nil
		}
	}
	return false, nil
}
