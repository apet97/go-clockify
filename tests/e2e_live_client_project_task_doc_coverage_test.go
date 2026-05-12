//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

// TestLiveClientProjectTaskDocCoverage exercises the client, project, and task
// endpoint family documented in clockify-api-probe-lab through the MCP path.
// It uses only entities created with this run's prefix and cleans them through
// the same archive-before-delete helpers used by the broader live campaign.
func TestLiveClientProjectTaskDocCoverage(t *testing.T) {
	requireWriteEnabled(t)

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	clientName := c.LivePrefix("doc-client", 0)
	client := h.callOK(ctx, "clockify_create_client", map[string]any{
		"name":    clientName,
		"address": "Ground Floor, ABC Bldg., Palo Alto, California, USA 94020",
		"email":   strings.ReplaceAll(clientName, "-", ".") + "@example.com",
		"note":    "client/project/task doc coverage",
	})
	clientID, _ := extractDataMap(t, client)["id"].(string)
	if clientID == "" {
		t.Fatalf("clockify_create_client returned no id: %#v", client)
	}
	c.RegisterCleanup("client", clientID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteClient(ctx, clientID)
	})

	_ = h.callOK(ctx, "clockify_list_clients", map[string]any{
		"name":        clientName,
		"sort_column": "NAME",
		"sort_order":  "ASCENDING",
		"archived":    "false",
		"page":        1,
		"page_size":   25,
	})
	updateClientArgs := map[string]any{
		"client":             clientID,
		"cc_emails":          []any{"invoice-recipient@example.com"},
		"archive_projects":   false,
		"mark_tasks_as_done": false,
	}
	if currencyID := firstWorkspaceCurrencyID(t, h, ctx); currencyID != "" {
		updateClientArgs["currency_id"] = currencyID
	}
	_ = h.callOK(ctx, "clockify_update_client", updateClientArgs)
	_ = h.callOK(ctx, "clockify_get_client", map[string]any{"client": clientID})

	projectName := c.LivePrefix("doc-project", 0)
	project := h.callOK(ctx, "clockify_create_project", map[string]any{
		"name":      projectName,
		"client_id": clientID,
		"color":     "#03a9f4",
		"billable":  true,
		"is_public": false,
		"note":      "project doc coverage",
		"estimate":  map[string]any{"estimate": "PT1H30M", "type": "MANUAL"},
	})
	projectID, _ := extractDataMap(t, project)["id"].(string)
	if projectID == "" {
		t.Fatalf("clockify_create_project returned no id: %#v", project)
	}
	c.RegisterCleanup("project", projectID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteProject(ctx, projectID)
	})

	_ = h.callOK(ctx, "clockify_list_projects", map[string]any{
		"name":               projectName,
		"strict_name_search": true,
		"archived":           false,
		"billable":           true,
		"clients":            []any{clientID},
		"contains_client":    true,
		"client_status":      "ACTIVE",
		"users":              []any{c.OwnerUserID},
		"contains_user":      true,
		"user_status":        "ACTIVE",
		"is_template":        false,
		"sort_column":        "NAME",
		"sort_order":         "ASCENDING",
		"hydrated":           true,
		"access":             "PRIVATE",
		"page":               1,
		"page_size":          25,
	})
	_ = h.callOK(ctx, "clockify_update_project", map[string]any{
		"project":   projectID,
		"client_id": clientID,
		"note":      "project doc coverage updated",
		"archived":  false,
	})
	_ = h.callOK(ctx, "clockify_get_project", map[string]any{"project": projectID})

	taskName := c.LivePrefix("doc-task", 0)
	task := h.callOK(ctx, "clockify_create_task", map[string]any{
		"project_id":        projectID,
		"name":              taskName,
		"assignee_ids":      []any{c.OwnerUserID},
		"budget_estimate":   3600,
		"contains_assignee": true,
		"billable":          true,
		"estimate":          "PT1H",
		"status":            "ACTIVE",
	})
	taskID, _ := extractDataMap(t, task)["id"].(string)
	if taskID == "" {
		t.Fatalf("clockify_create_task returned no id: %#v", task)
	}

	_ = h.callOK(ctx, "clockify_list_tasks", map[string]any{
		"project":            projectID,
		"name":               taskName,
		"strict_name_search": true,
		"is_active":          true,
		"sort_column":        "ID",
		"sort_order":         "ASCENDING",
		"page":               1,
		"page_size":          25,
	})
	_ = h.callOK(ctx, "clockify_update_task", map[string]any{
		"project":           projectID,
		"task":              taskID,
		"name":              taskName + "-updated",
		"assignee_ids":      []any{c.OwnerUserID},
		"budget_estimate":   5400,
		"contains_assignee": true,
		"membership_status": "ACTIVE",
		"billable":          true,
		"estimate":          "PT1H30M",
		"status":            "DONE",
	})
	_ = h.callOK(ctx, "clockify_get_task", map[string]any{"project": projectID, "task": taskID})
	_ = h.callOK(ctx, "clockify_delete_task", map[string]any{"project": projectID, "task": taskID})
	_ = h.callOK(ctx, "clockify_delete_project", map[string]any{"project": projectID})
	_ = h.callOK(ctx, "clockify_delete_client", map[string]any{"client": clientID})
}

// TestLiveProjectAdminDocCoverage covers the PROJECTSDOC/TASKDOC admin-rate
// endpoints that live in the lazily activated project_admin group. Billing and
// admin gates are explicit because these calls mutate project membership/rate
// metadata and may be plan-gated in ordinary workspaces.
func TestLiveProjectAdminDocCoverage(t *testing.T) {
	requireWriteEnabled(t)
	requireCategory(t, "CLOCKIFY_LIVE_ADMIN_ENABLED")
	requireCategory(t, "CLOCKIFY_LIVE_BILLING_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("project_admin")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	project := h.callOK(ctx, "clockify_create_project", map[string]any{
		"name": c.LivePrefix("doc-admin-project", 0),
	})
	projectID, _ := extractDataMap(t, project)["id"].(string)
	if projectID == "" {
		t.Fatalf("seed project returned no id: %#v", project)
	}
	c.RegisterCleanup("project", projectID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteProject(ctx, projectID)
	})

	template := h.callOK(ctx, "clockify_create_project_template", map[string]any{
		"name":      c.LivePrefix("doc-admin-template", 0),
		"is_public": false,
	})
	templateID, _ := extractDataMap(t, template)["id"].(string)
	if templateID == "" {
		t.Fatalf("seed template returned no id: %#v", template)
	}
	c.RegisterCleanup("project-template", templateID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteProject(ctx, templateID)
	})

	fromTemplate, fromTemplateErr := liveCallMaybe(t, h, ctx, "clockify_create_project_from_template", map[string]any{
		"name":                c.LivePrefix("doc-from-template", 0),
		"template_project_id": templateID,
		"is_public":           false,
	})
	fromTemplateID := ""
	if fromTemplateErr != "" {
		requireExpectedDocLiveRefusal(t, "clockify_create_project_from_template", fromTemplateErr)
	} else {
		fromTemplateID, _ = extractDataMap(t, fromTemplate)["id"].(string)
		if fromTemplateID == "" {
			t.Fatalf("create_project_from_template returned no id: %#v", fromTemplate)
		}
		c.RegisterCleanup("project-from-template", fromTemplateID, func(ctx context.Context) error {
			return c.rawArchiveAndDeleteProject(ctx, fromTemplateID)
		})
	}

	task := h.callOK(ctx, "clockify_create_task", map[string]any{
		"project_id": projectID,
		"name":       c.LivePrefix("doc-admin-task", 0),
		"status":     "ACTIVE",
	})
	taskID, _ := extractDataMap(t, task)["id"].(string)
	if taskID == "" {
		t.Fatalf("seed task returned no id: %#v", task)
	}

	templateTargetID := templateID
	if fromTemplateID != "" {
		templateTargetID = fromTemplateID
	}
	callDocToolMaybe(t, h, ctx, "clockify_update_project_template", map[string]any{
		"project_id":  templateTargetID,
		"is_template": true,
	})
	callDocToolMaybe(t, h, ctx, "clockify_update_project_estimate", map[string]any{
		"project_id": projectID,
		"time_estimate": map[string]any{
			"active":               true,
			"estimate":             "PT1H30M",
			"include_non_billable": false,
			"type":                 "MANUAL",
		},
	})
	callDocToolMaybe(t, h, ctx, "clockify_update_project_memberships", map[string]any{
		"project_id": projectID,
		"memberships": []any{map[string]any{
			"user_id": c.OwnerUserID,
		}},
	})
	callDocToolMaybe(t, h, ctx, "clockify_assign_project_memberships", map[string]any{
		"project_id": projectID,
		"user_ids":   []any{c.OwnerUserID},
		"remove":     false,
	})
	callDocToolMaybe(t, h, ctx, "clockify_update_project_user_cost_rate", map[string]any{
		"project_id": projectID,
		"user_id":    c.OwnerUserID,
		"amount":     1000,
		"since":      "2026-05-12T00:00:00Z",
	})
	callDocToolMaybe(t, h, ctx, "clockify_update_project_user_hourly_rate", map[string]any{
		"project_id": projectID,
		"user_id":    c.OwnerUserID,
		"amount":     2500,
		"since":      "2026-05-12T00:00:00Z",
	})
	callDocToolMaybe(t, h, ctx, "clockify_update_task_cost_rate", map[string]any{
		"project_id": projectID,
		"task_id":    taskID,
		"amount":     1000,
		"since":      "2026-05-12T00:00:00Z",
	})
	callDocToolMaybe(t, h, ctx, "clockify_update_task_hourly_rate", map[string]any{
		"project_id": projectID,
		"task_id":    taskID,
		"amount":     2500,
		"since":      "2026-05-12T00:00:00Z",
	})
	if fromTemplateID != "" {
		_ = h.callOK(ctx, "clockify_archive_projects", map[string]any{"project_ids": []any{fromTemplateID}})
	}
	_ = h.callOK(ctx, "clockify_archive_projects", map[string]any{"project_ids": []any{projectID}})
}

func callDocToolMaybe(t *testing.T, h *liveMCPHarness, ctx context.Context, tool string, args map[string]any) {
	t.Helper()
	if _, errText := liveCallMaybe(t, h, ctx, tool, args); errText != "" {
		requireExpectedDocLiveRefusal(t, tool, errText)
	}
}

func requireExpectedDocLiveRefusal(t *testing.T, tool, errText string) {
	t.Helper()
	lower := strings.ToLower(errText)
	accepted := []string{
		"403",
		"400",
		"permission",
		"forbidden",
		"not allowed",
		"not permitted",
		"subscription",
		"paid",
		"plan",
		"already",
		"membership",
	}
	for _, marker := range accepted {
		if strings.Contains(lower, marker) {
			t.Logf("%s live-probed documented route and received upstream refusal: %s", tool, errText)
			return
		}
	}
	t.Fatalf("%s unexpected live error: %s", tool, errText)
}

func firstWorkspaceCurrencyID(t *testing.T, h *liveMCPHarness, ctx context.Context) string {
	t.Helper()
	result := h.callOK(ctx, "clockify_get_workspace", nil)
	data := extractDataMap(t, result)
	currencies, ok := data["currencies"].([]any)
	if !ok {
		return ""
	}
	for _, raw := range currencies {
		currency, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := currency["id"].(string); id != "" {
			return id
		}
	}
	return ""
}
