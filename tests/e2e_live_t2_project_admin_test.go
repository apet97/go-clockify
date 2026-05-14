//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"
)

// TestLiveOneUserProjectAdminCRUD covers current project-admin tools against
// a real Clockify backend. The test creates a project, creates a template,
// updates an estimate, replaces memberships on the owned test project, archives
// it, then deletes the archived project via the raw client.
//
// Gated by both CLOCKIFY_LIVE_ADMIN_ENABLED (memberships) and
// CLOCKIFY_LIVE_BILLING_ENABLED (estimates) per the campaign plan.
func TestLiveOneUserProjectAdminCRUD(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_ADMIN_ENABLED")
	requireCategory(t, "CLOCKIFY_LIVE_BILLING_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	// Seed project — the admin tools all act on a project we own.
	projectName := c.LivePrefix("padmin", 0)
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
			t.Fatalf("seed work package returned no projectId: %#v", ids)
		}
		projectID = id
		c.RegisterCleanup("project", id, func(ctx context.Context) error {
			return c.rawArchiveAndDeleteProject(ctx, id)
		})
	})

	// Template — distinct project entity flagged as template upstream.
	templateName := c.LivePrefix("padmin-tmpl", 0)
	var templateID string
	t.Run("create_project_template", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_projects_templates_create", map[string]any{
			"name":      templateName,
			"is_public": false,
		})
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("create_project_template returned no id: %#v", data)
		}
		templateID = id
		c.RegisterCleanup("project-template", id, func(ctx context.Context) error {
			return c.rawArchiveAndDeleteProject(ctx, id)
		})
	})

	t.Run("get_project_template_round_trips", func(t *testing.T) {
		if templateID == "" {
			t.Skip("template not created")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_projects_get", map[string]any{
			"project_id": templateID,
		})
		data := extractDataMap(t, result)
		if got, _ := data["id"].(string); got != templateID {
			t.Fatalf("get_project_template id round-trip: got %q want %q", got, templateID)
		}
	})

	t.Run("update_project_estimate_time", func(t *testing.T) {
		if projectID == "" {
			t.Skip("seed project missing")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// 8 hours = 28800 seconds.
		result := h.callOK(ctx, "clockify_projects_estimates_update", map[string]any{
			"project_id":     projectID,
			"estimate_type":  "TIME",
			"estimate_value": 28800,
		})
		sc := structuredContentMap(t, result)
		if okFlag, _ := sc["ok"].(bool); !okFlag {
			t.Fatalf("clockify_projects_estimates_update response carried ok=false")
		}
	})

	t.Run("set_project_memberships_replaces_member_list", func(t *testing.T) {
		if projectID == "" {
			t.Skip("seed project missing")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// SUMMARY rev 3 #6: PATCH /projects/{id}/memberships with the
		// full desired list. REPLACE semantics — sending only the
		// owner here leaves only the owner. Owner-only is safe on the
		// seeded project (no other members were added in this test).
		result := h.callOK(ctx, "clockify_projects_memberships_update", map[string]any{
			"project_id": projectID,
			"user_ids":   []any{c.OwnerUserID},
		})
		sc := structuredContentMap(t, result)
		if okFlag, _ := sc["ok"].(bool); !okFlag {
			t.Fatalf("clockify_projects_memberships_update carried ok=false: %#v", sc)
		}
		// The handler returns the memberships array as Data.
		members, ok := sc["data"].([]any)
		if !ok {
			t.Fatalf("expected memberships slice as data, got %T (%#v)", sc["data"], sc["data"])
		}
		if len(members) == 0 {
			t.Fatalf("memberships data unexpectedly empty: %#v", sc["data"])
		}
		first, ok := members[0].(map[string]any)
		if !ok {
			t.Fatalf("first membership is not an object: %T", members[0])
		}
		gotUser, _ := first["userId"].(string)
		if gotUser != c.OwnerUserID {
			t.Fatalf("expected first membership userId %q, got %q (REPLACE didn't apply?)",
				c.OwnerUserID, gotUser)
		}
	})

	t.Run("archive_projects_flips_archived_flag", func(t *testing.T) {
		if projectID == "" {
			t.Skip("seed project missing")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = h.callOK(ctx, "clockify_projects_archive", map[string]any{
			"project_id": projectID,
		})
		// Verify archived state via raw GET.
		var probe map[string]any
		if err := c.rawGetPath(ctx, "/projects/"+projectID, &probe); err != nil {
			t.Fatalf("get archived project: %v", err)
		}
		archived, _ := probe["archived"].(bool)
		if !archived {
			t.Fatalf("project %s not marked archived after archive_projects: %#v", projectID, probe)
		}
	})
}
