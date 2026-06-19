//go:build livee2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/paths"
)

const liveRawSchemaTimeout = 30 * time.Second

func TestLiveRawClockifyReadSideSchemaDiff(t *testing.T) {
	cfg := setupLiveSchemaConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), liveRawSchemaTimeout)
	defer cancel()

	httpClient := &http.Client{Timeout: liveRawSchemaTimeout}
	get := func(path string, query map[string]string, out any) {
		t.Helper()
		if err := liveGetRaw(ctx, httpClient, cfg, path, query, out); err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
	}

	var user map[string]json.RawMessage
	get("/user", nil, &user)
	assertNoUnknownFields[clockify.User](t, "/user", user)

	var workspaces []map[string]json.RawMessage
	get("/workspaces", nil, &workspaces)
	assertNonEmpty(t, "/workspaces", len(workspaces))
	for i, ws := range workspaces {
		assertNoUnknownFields[clockify.Workspace](t, fmt.Sprintf("/workspaces[%d]", i), ws)
	}

	wsID := strings.TrimSpace(cfg.WorkspaceID)
	if wsID == "" {
		wsID = firstStringField(workspaces, "id")
	}
	if wsID == "" {
		t.Fatal("no workspace id available for read-side schema diff")
	}

	workspacePath, err := paths.Workspace(wsID)
	if err != nil {
		t.Fatalf("workspace path: %v", err)
	}
	var workspace map[string]json.RawMessage
	get(workspacePath, nil, &workspace)
	assertNoUnknownFields[clockify.Workspace](t, workspacePath, workspace)

	projectsPath, err := paths.Workspace(wsID, "projects")
	if err != nil {
		t.Fatalf("projects path: %v", err)
	}
	var projects []map[string]json.RawMessage
	get(projectsPath, firstPageQuery(), &projects)
	for i, project := range projects {
		assertNoUnknownFields[clockify.Project](t, fmt.Sprintf("%s[%d]", projectsPath, i), project)
	}

	clientsPath, err := paths.Workspace(wsID, "clients")
	if err != nil {
		t.Fatalf("clients path: %v", err)
	}
	var clients []map[string]json.RawMessage
	get(clientsPath, firstPageQuery(), &clients)
	for i, client := range clients {
		assertNoUnknownFields[clockify.ClientEntity](t, fmt.Sprintf("%s[%d]", clientsPath, i), client)
	}

	tagsPath, err := paths.Workspace(wsID, "tags")
	if err != nil {
		t.Fatalf("tags path: %v", err)
	}
	var tags []map[string]json.RawMessage
	get(tagsPath, firstPageQuery(), &tags)
	for i, tag := range tags {
		assertNoUnknownFields[clockify.Tag](t, fmt.Sprintf("%s[%d]", tagsPath, i), tag)
	}

	projectID := firstStringField(projects, "id")
	if projectID == "" {
		t.Log("no projects returned; task schema diff has no live sample")
	} else {
		tasksPath, err := paths.Workspace(wsID, "projects", projectID, "tasks")
		if err != nil {
			t.Fatalf("tasks path: %v", err)
		}
		var tasks []map[string]json.RawMessage
		get(tasksPath, firstPageQuery(), &tasks)
		for i, task := range tasks {
			assertNoUnknownFields[clockify.Task](t, fmt.Sprintf("%s[%d]", tasksPath, i), task)
		}
	}

	userID := stringField(user, "id")
	if userID == "" {
		t.Fatal("/user response did not include id")
	}
	entriesPath, err := paths.Workspace(wsID, "user", userID, "time-entries")
	if err != nil {
		t.Fatalf("time entries path: %v", err)
	}
	var entries []map[string]json.RawMessage
	get(entriesPath, map[string]string{"page": "1", "page-size": "10"}, &entries)
	for i, entry := range entries {
		assertNoUnknownFields[clockify.TimeEntry](t, fmt.Sprintf("%s[%d]", entriesPath, i), entry)
		if raw := entry["timeInterval"]; len(raw) > 0 && string(raw) != "null" {
			var interval map[string]json.RawMessage
			if err := json.Unmarshal(raw, &interval); err != nil {
				t.Fatalf("%s[%d].timeInterval: %v", entriesPath, i, err)
			}
			assertNoUnknownFields[clockify.TimeInterval](t, fmt.Sprintf("%s[%d].timeInterval", entriesPath, i), interval)
		}
	}
	if len(entries) == 0 {
		t.Log("no time entries returned; TimeEntry schema diff has no live sample")
	}
}

// TestLiveRawClockifyWriteSideSchemaDiff extends the read-side schema canary to
// the domains the original test did not cover: invoices, expenses, approvals,
// scheduling, time off, groups, custom fields, and webhooks. It stays read-only
// — every probe is a GET (or, for time-off request listing, the POST that
// Clockify requires; a GET there returns 405). None of these domains has a
// dedicated typed response model in internal/clockify, so each subtest decodes
// the documented container shape the matching handler relies on and asserts the
// key id field is populated when the collection is non-empty. Empty collections
// in the sacrificial workspace are skipped rather than failed, mirroring the
// read-side test's "no live sample" logging.
func TestLiveRawClockifyWriteSideSchemaDiff(t *testing.T) {
	cfg := setupLiveSchemaConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), liveRawSchemaTimeout)
	defer cancel()

	httpClient := &http.Client{Timeout: liveRawSchemaTimeout}
	get := func(path string, query map[string]string, out any) {
		t.Helper()
		if err := liveGetRaw(ctx, httpClient, cfg, path, query, out); err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
	}

	wsID := strings.TrimSpace(cfg.WorkspaceID)
	if wsID == "" {
		t.Fatal("no workspace id available for write-side schema diff")
	}
	mustWorkspacePath := func(segments ...string) string {
		t.Helper()
		p, err := paths.Workspace(wsID, segments...)
		if err != nil {
			t.Fatalf("workspace path %v: %v", segments, err)
		}
		return p
	}

	t.Run("invoice-settings", func(t *testing.T) {
		// Invoice settings is a single object (the /invoices/settings route the
		// get_invoice_settings handler reads); confirm it decodes to an object.
		var settings map[string]json.RawMessage
		get(mustWorkspacePath("invoices", "settings"), nil, &settings)
		if len(settings) == 0 {
			t.Skip("invoice settings returned an empty object")
		}
	})

	t.Run("invoices", func(t *testing.T) {
		// list_invoices reads {total:int, invoices:[...]} (see invoices.go).
		var env struct {
			Total    int                          `json:"total"`
			Invoices []map[string]json.RawMessage `json:"invoices"`
		}
		get(mustWorkspacePath("invoices"), firstPageQuery(), &env)
		assertItemsHaveID(t, "invoices", env.Invoices)
	})

	t.Run("expense-categories", func(t *testing.T) {
		// expenses_categories_* reads a paged {categories:[...]} envelope.
		var env struct {
			Categories []map[string]json.RawMessage `json:"categories"`
		}
		get(mustWorkspacePath("expenses", "categories"), firstPageQuery(), &env)
		assertItemsHaveID(t, "expense-categories", env.Categories)
	})

	t.Run("expenses", func(t *testing.T) {
		// GET /workspaces/{ws}/expenses returns the WorkspaceExpensesDtoV1 ->
		// ExpensesWithCountDtoV1 shape: the top-level "expenses" key is an
		// OBJECT {count:int, expenses:[...]}, sitting alongside dailyTotals[]
		// and weeklyTotals[] — not a bare array. The list_expenses handler
		// decodes this same doubly-nested envelope (see expenses.go), so the
		// oracle must mirror it. Verified live 2026-05-02.
		var env struct {
			Expenses struct {
				Count    int                          `json:"count"`
				Expenses []map[string]json.RawMessage `json:"expenses"`
			} `json:"expenses"`
			DailyTotals  []json.RawMessage `json:"dailyTotals"`
			WeeklyTotals []json.RawMessage `json:"weeklyTotals"`
		}
		get(mustWorkspacePath("expenses"), firstPageQuery(), &env)
		assertItemsHaveID(t, "expenses", env.Expenses.Expenses)
	})

	t.Run("approvals", func(t *testing.T) {
		// approval-requests list accepts only PENDING/APPROVED/WITHDRAWN_APPROVAL
		// (AGENTS.md gotcha). The response is a bare array, but each element is a
		// wrapper: the request object (and its id) lives under the nested
		// "approvalRequest" key, alongside sibling rollups (approvedTime,
		// billableAmount, trackedTime, ...). The list_approvals handler reads the
		// id via approvalRequestMap -> approvalRequest, so the oracle must dive
		// into that nested object rather than expecting a top-level id.
		var requests []map[string]json.RawMessage
		get(mustWorkspacePath("approval-requests"), map[string]string{"status": "PENDING"}, &requests)
		assertApprovalRequestsHaveID(t, "approvals", requests)
	})

	t.Run("scheduling-assignments", func(t *testing.T) {
		// scheduling_assignments_list reads the /scheduling/assignments/all route
		// (bare /assignments returns 404 — AGENTS.md gotcha). That route requires
		// "start" and "end" query parameters; omitting them is a 400
		// ("Required request parameter 'start' ... is not present"). The
		// listAssignmentsRaw handler always supplies a start/end range, so the
		// oracle must too.
		query := firstPageQuery()
		query["start"] = "2020-01-01T00:00:00Z"
		query["end"] = "2020-12-31T23:59:59Z"
		var assignments []map[string]json.RawMessage
		get(mustWorkspacePath("scheduling", "assignments", "all"), query, &assignments)
		assertItemsHaveID(t, "scheduling-assignments", assignments)
	})

	t.Run("time-off-policies", func(t *testing.T) {
		// GET /workspaces/{ws}/time-off/policies returns a BARE ARRAY of policy
		// objects (the fetchTimeOffPolicies handler decodes []map[string]any
		// directly), not a {policies:[...]} envelope. The oracle must mirror
		// that bare-array shape.
		var policies []map[string]json.RawMessage
		get(mustWorkspacePath("time-off", "policies"), firstPageQuery(), &policies)
		assertItemsHaveID(t, "time-off-policies", policies)
	})

	t.Run("time-off-requests", func(t *testing.T) {
		// Time-off request listing is POST, not GET (a GET returns 405 — see the
		// AGENTS.md gotcha and clockify_request_time_off). The response is a
		// {requests:[...], count:int} envelope.
		body := map[string]any{
			"start":      "2020-01-01T00:00:00Z",
			"end":        "2020-01-02T00:00:00Z",
			"statuses":   []string{"PENDING"},
			"page":       1,
			"pageSize":   1,
			"users":      []string{},
			"userGroups": []string{},
		}
		var env struct {
			Requests []map[string]json.RawMessage `json:"requests"`
		}
		if err := livePostRaw(ctx, httpClient, cfg, mustWorkspacePath("time-off", "requests"), body, &env); err != nil {
			t.Fatalf("POST time-off/requests: %v", err)
		}
		assertItemsHaveID(t, "time-off-requests", env.Requests)
	})

	t.Run("groups", func(t *testing.T) {
		// groups_list reads the /user-groups route; response is a bare array.
		var groups []map[string]json.RawMessage
		get(mustWorkspacePath("user-groups"), firstPageQuery(), &groups)
		assertItemsHaveID(t, "groups", groups)
	})

	t.Run("custom-fields", func(t *testing.T) {
		// custom_fields_list reads the /custom-fields route; response is a bare
		// array of field definitions.
		var fields []map[string]json.RawMessage
		get(mustWorkspacePath("custom-fields"), firstPageQuery(), &fields)
		assertItemsHaveID(t, "custom-fields", fields)
	})

	t.Run("webhooks", func(t *testing.T) {
		// GET /workspaces/{ws}/webhooks returns an OBJECT envelope
		// {workspaceWebhookCount:int, webhooks:[...]}, not a bare array. The
		// ListWebhooks handler decodes exactly this shape (see webhooks.go),
		// so the oracle must pull the items out of the webhooks field.
		var env struct {
			WorkspaceWebhookCount int                          `json:"workspaceWebhookCount"`
			Webhooks              []map[string]json.RawMessage `json:"webhooks"`
		}
		get(mustWorkspacePath("webhooks"), nil, &env)
		assertItemsHaveID(t, "webhooks", env.Webhooks)
	})
}

func TestLiveRawClockifyWriteCRUDShapeOracle(t *testing.T) {
	cfg := setupLiveSchemaConfig(t)
	requireLiveWriteShapeGate(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	httpClient := &http.Client{Timeout: liveRawSchemaTimeout}
	wsID := strings.TrimSpace(cfg.WorkspaceID)
	prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX"))
	if wsID == "" || prefix == "" {
		t.Fatal("CLOCKIFY_WORKSPACE_ID and CLOCKIFY_LIVE_PREFIX are required")
	}
	pathLabel := func(path string) string { return livePathLabel(cfg, path) }
	mustWorkspacePath := func(segments ...string) string {
		t.Helper()
		p, err := paths.Workspace(wsID, segments...)
		if err != nil {
			t.Fatalf("workspace path %v: %v", segments, err)
		}
		return p
	}

	var user map[string]json.RawMessage
	if _, err := liveJSONRaw(ctx, httpClient, cfg, http.MethodGet, "/user", nil, &user); err != nil {
		t.Fatalf("GET /user: %v", err)
	}
	userID := stringField(user, "id")
	if userID == "" {
		t.Fatal("/user response did not include id")
	}

	clientPath := mustWorkspacePath("clients")
	clientName := prefix + " shape client"
	var client map[string]json.RawMessage
	status, err := liveJSONRaw(ctx, httpClient, cfg, http.MethodPost, clientPath, map[string]any{"name": clientName}, &client)
	if err != nil {
		t.Fatalf("POST %s: %v", pathLabel(clientPath), err)
	}
	assertNoUnknownFields[clockify.ClientEntity](t, "clients.create", client)
	clientID := requireStringField(t, "clients.create", client, "id")
	t.Cleanup(func() { cleanupClientRaw(context.Background(), t, httpClient, cfg, wsID, clientID, clientName) })
	logFindingsRow(t, http.MethodPost, "api.clockify.me", "/workspaces/{workspaceId}/clients", status, "fixtures/live-shape/clients-create.json")

	clientItemPath := mustWorkspacePath("clients", clientID)
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodPut, clientItemPath, map[string]any{"name": clientName + " updated", "archived": false}, &client)
	if err != nil {
		t.Fatalf("PUT %s: %v", pathLabel(clientItemPath), err)
	}
	assertNoUnknownFields[clockify.ClientEntity](t, "clients.update", client)
	logFindingsRow(t, http.MethodPut, "api.clockify.me", "/workspaces/{workspaceId}/clients/{clientId}", status, "fixtures/live-shape/clients-update.json")
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodGet, clientItemPath, nil, &client)
	if err != nil {
		t.Fatalf("GET %s: %v", pathLabel(clientItemPath), err)
	}
	assertNoUnknownFields[clockify.ClientEntity](t, "clients.get", client)
	logFindingsRow(t, http.MethodGet, "api.clockify.me", "/workspaces/{workspaceId}/clients/{clientId}", status, "fixtures/live-shape/clients-get.json")

	projectPath := mustWorkspacePath("projects")
	projectName := prefix + " shape project"
	projectBody := map[string]any{
		"name":     projectName,
		"clientId": clientID,
		"color":    "#03A9F4",
		"billable": true,
		"isPublic": false,
	}
	var project map[string]json.RawMessage
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodPost, projectPath, projectBody, &project)
	if err != nil {
		t.Fatalf("POST %s: %v", pathLabel(projectPath), err)
	}
	assertNoUnknownFields[clockify.Project](t, "projects.create", project)
	projectID := requireStringField(t, "projects.create", project, "id")
	t.Cleanup(func() { cleanupProjectRaw(context.Background(), t, httpClient, cfg, wsID, projectID, projectName) })
	logFindingsRow(t, http.MethodPost, "api.clockify.me", "/workspaces/{workspaceId}/projects", status, "fixtures/live-shape/projects-create.json")

	projectItemPath := mustWorkspacePath("projects", projectID)
	projectBody["name"] = projectName + " updated"
	projectBody["archived"] = false
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodPut, projectItemPath, projectBody, &project)
	if err != nil {
		t.Fatalf("PUT %s: %v", pathLabel(projectItemPath), err)
	}
	assertNoUnknownFields[clockify.Project](t, "projects.update", project)
	logFindingsRow(t, http.MethodPut, "api.clockify.me", "/workspaces/{workspaceId}/projects/{projectId}", status, "fixtures/live-shape/projects-update.json")
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodGet, projectItemPath, nil, &project)
	if err != nil {
		t.Fatalf("GET %s: %v", pathLabel(projectItemPath), err)
	}
	assertNoUnknownFields[clockify.Project](t, "projects.get", project)
	logFindingsRow(t, http.MethodGet, "api.clockify.me", "/workspaces/{workspaceId}/projects/{projectId}", status, "fixtures/live-shape/projects-get.json")

	tagPath := mustWorkspacePath("tags")
	tagName := prefix + " shape tag"
	var tag map[string]json.RawMessage
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodPost, tagPath, map[string]any{"name": tagName}, &tag)
	if err != nil {
		t.Fatalf("POST %s: %v", pathLabel(tagPath), err)
	}
	assertNoUnknownFields[clockify.Tag](t, "tags.create", tag)
	tagID := requireStringField(t, "tags.create", tag, "id")
	t.Cleanup(func() {
		cleanupDeleteRaw(context.Background(), t, httpClient, cfg, mustWorkspacePath("tags", tagID),
			findingsDelete{templatePath: "/workspaces/{workspaceId}/tags/{tagId}", fixture: "fixtures/live-shape/tags-delete.txt"})
	})
	logFindingsRow(t, http.MethodPost, "api.clockify.me", "/workspaces/{workspaceId}/tags", status, "fixtures/live-shape/tags-create.json")

	tagItemPath := mustWorkspacePath("tags", tagID)
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodPut, tagItemPath, map[string]any{"name": tagName + " updated", "archived": false}, &tag)
	if err != nil {
		t.Fatalf("PUT %s: %v", pathLabel(tagItemPath), err)
	}
	assertNoUnknownFields[clockify.Tag](t, "tags.update", tag)
	logFindingsRow(t, http.MethodPut, "api.clockify.me", "/workspaces/{workspaceId}/tags/{tagId}", status, "fixtures/live-shape/tags-update.json")
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodGet, tagItemPath, nil, &tag)
	if err != nil {
		t.Fatalf("GET %s: %v", pathLabel(tagItemPath), err)
	}
	assertNoUnknownFields[clockify.Tag](t, "tags.get", tag)
	logFindingsRow(t, http.MethodGet, "api.clockify.me", "/workspaces/{workspaceId}/tags/{tagId}", status, "fixtures/live-shape/tags-get.json")

	taskPath := mustWorkspacePath("projects", projectID, "tasks")
	taskName := prefix + " shape task"
	var task map[string]json.RawMessage
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodPost, taskPath, map[string]any{"name": taskName, "billable": true}, &task)
	if err != nil {
		t.Fatalf("POST %s: %v", pathLabel(taskPath), err)
	}
	assertNoUnknownFields[clockify.Task](t, "tasks.create", task)
	taskID := requireStringField(t, "tasks.create", task, "id")
	t.Cleanup(func() { cleanupTaskRaw(context.Background(), t, httpClient, cfg, wsID, projectID, taskID, taskName) })
	logFindingsRow(t, http.MethodPost, "api.clockify.me", "/workspaces/{workspaceId}/projects/{projectId}/tasks", status, "fixtures/live-shape/tasks-create.json")

	taskItemPath := mustWorkspacePath("projects", projectID, "tasks", taskID)
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodPut, taskItemPath, map[string]any{"name": taskName + " updated", "billable": true, "status": "ACTIVE"}, &task)
	if err != nil {
		t.Fatalf("PUT %s: %v", pathLabel(taskItemPath), err)
	}
	assertNoUnknownFields[clockify.Task](t, "tasks.update", task)
	logFindingsRow(t, http.MethodPut, "api.clockify.me", "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}", status, "fixtures/live-shape/tasks-update.json")
	status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodGet, taskItemPath, nil, &task)
	if err != nil {
		t.Fatalf("GET %s: %v", pathLabel(taskItemPath), err)
	}
	assertNoUnknownFields[clockify.Task](t, "tasks.get", task)
	logFindingsRow(t, http.MethodGet, "api.clockify.me", "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}", status, "fixtures/live-shape/tasks-get.json")

	t.Run("time-entry-crud", func(t *testing.T) {
		if hasRequiredTimeEntryCustomFields(ctx, t, httpClient, cfg, wsID) {
			t.Skip("workspace has required time-entry custom fields; raw shape oracle needs captured customFields payload")
		}
		entryStart := time.Now().UTC().Add(-90 * time.Minute).Truncate(time.Second)
		entryEnd := entryStart.Add(30 * time.Minute)
		entryBody := map[string]any{
			"start":       entryStart.Format(time.RFC3339),
			"end":         entryEnd.Format(time.RFC3339),
			"description": prefix + " shape entry",
			"projectId":   projectID,
			"taskId":      taskID,
			"tagIds":      []string{tagID},
			"billable":    true,
		}
		var entry map[string]json.RawMessage
		entryPath := mustWorkspacePath("time-entries")
		status, err := liveJSONRaw(ctx, httpClient, cfg, http.MethodPost, entryPath, entryBody, &entry)
		if err != nil {
			t.Fatalf("POST %s: %v", pathLabel(entryPath), err)
		}
		assertNoUnknownFields[clockify.TimeEntry](t, "timeEntries.create", entry)
		entryID := requireStringField(t, "timeEntries.create", entry, "id")
		t.Cleanup(func() {
			cleanupDeleteRaw(context.Background(), t, httpClient, cfg, mustWorkspacePath("time-entries", entryID),
				findingsDelete{templatePath: "/workspaces/{workspaceId}/time-entries/{timeEntryId}", fixture: "fixtures/live-shape/time-entries-delete.txt"})
		})
		logFindingsRow(t, http.MethodPost, "api.clockify.me", "/workspaces/{workspaceId}/time-entries", status, "fixtures/live-shape/time-entries-create.json")

		entryItemPath := mustWorkspacePath("time-entries", entryID)
		entryBody["description"] = prefix + " shape entry updated"
		status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodPut, entryItemPath, entryBody, &entry)
		if err != nil {
			t.Fatalf("PUT %s: %v", pathLabel(entryItemPath), err)
		}
		assertNoUnknownFields[clockify.TimeEntry](t, "timeEntries.update", entry)
		logFindingsRow(t, http.MethodPut, "api.clockify.me", "/workspaces/{workspaceId}/time-entries/{timeEntryId}", status, "fixtures/live-shape/time-entries-update.json")
		status, err = liveJSONRaw(ctx, httpClient, cfg, http.MethodGet, entryItemPath, nil, &entry)
		if err != nil {
			t.Fatalf("GET %s: %v", pathLabel(entryItemPath), err)
		}
		assertNoUnknownFields[clockify.TimeEntry](t, "timeEntries.get", entry)
		logFindingsRow(t, http.MethodGet, "api.clockify.me", "/workspaces/{workspaceId}/time-entries/{timeEntryId}", status, "fixtures/live-shape/time-entries-get.json")
	})

	t.Run("reports", func(t *testing.T) {
		start, end := previousFullWeekRange(time.Now().UTC())
		for _, report := range []struct {
			name      string
			path      string
			body      map[string]any
			wantAnyOf []string
		}{
			{
				name: "summary",
				path: "/workspaces/" + wsID + "/reports/summary",
				body: map[string]any{
					"dateRangeStart": start,
					"dateRangeEnd":   end,
					"exportType":     "JSON",
					"summaryFilter":  map[string]any{"groups": []string{"PROJECT"}},
				},
				wantAnyOf: []string{"totals", "groupOne"},
			},
			{
				name: "detailed",
				path: "/workspaces/" + wsID + "/reports/detailed",
				body: map[string]any{
					"dateRangeStart": start,
					"dateRangeEnd":   end,
					"exportType":     "JSON",
					"detailedFilter": map[string]any{},
				},
				wantAnyOf: []string{"timeentries", "totals"},
			},
			{
				name: "weekly",
				path: "/workspaces/" + wsID + "/reports/weekly",
				body: map[string]any{
					"dateRangeStart": start,
					"dateRangeEnd":   end,
					"exportType":     "JSON",
					"weeklyFilter":   map[string]any{"group": "PROJECT", "subgroup": "TIME"},
				},
				wantAnyOf: []string{"totals", "groupOne", "weekTotals"},
			},
			{
				name: "attendance",
				path: "/workspaces/" + wsID + "/reports/attendance",
				body: map[string]any{
					"dateRangeStart":    start,
					"dateRangeEnd":      end,
					"exportType":        "JSON",
					"attendanceFilter":  map[string]any{},
					"sortOrder":         "ASCENDING",
					"includeNonWorking": false,
				},
				wantAnyOf: []string{"entities"},
			},
		} {
			t.Run(report.name, func(t *testing.T) {
				var body map[string]json.RawMessage
				status, err := liveReportsJSONRaw(ctx, httpClient, cfg, http.MethodPost, report.path, report.body, &body)
				if err != nil {
					t.Fatalf("POST reports %s: %v", report.name, err)
				}
				assertHasAnyKey(t, "reports."+report.name, body, report.wantAnyOf...)
				logFindingsRow(t, http.MethodPost, "reports.api.clockify.me", strings.Replace(report.path, wsID, "{workspaceId}", 1), status, "fixtures/live-shape/reports-"+report.name+".json")
			})
		}
	})
}

func requireLiveWriteShapeGate(t *testing.T, cfg config.OneUserConfig) {
	t.Helper()
	if os.Getenv("CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS") != "1" {
		t.Skip("set CLOCKIFY_LIVE_HAPPY_PATH_CAMPAIGNS=1 to run mutating write-shape oracle")
	}
	if got := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_WORKSPACE_CONFIRM")); got != strings.TrimSpace(cfg.WorkspaceID) {
		t.Fatalf("CLOCKIFY_LIVE_WORKSPACE_CONFIRM must match CLOCKIFY_WORKSPACE_ID for mutating write-shape oracle")
	}
	if strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX")) == "" {
		t.Fatal("CLOCKIFY_LIVE_PREFIX is required for mutating write-shape oracle")
	}
}

func liveReportsJSONRaw(ctx context.Context, client *http.Client, cfg config.OneUserConfig, method, path string, body, out any) (int, error) {
	return liveJSONAtBase(ctx, client, cfg, "https://reports.api.clockify.me/v1", method, path, body, out)
}

func liveJSONRaw(ctx context.Context, client *http.Client, cfg config.OneUserConfig, method, path string, body, out any) (int, error) {
	return liveJSONAtBase(ctx, client, cfg, strings.TrimRight(cfg.BaseURL, "/"), method, path, body, out)
}

func liveJSONAtBase(ctx context.Context, client *http.Client, cfg config.OneUserConfig, baseURL, method, path string, body, out any) (int, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Api-Key", cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "clockify-mcp-live-schema-diff")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return resp.StatusCode, fmt.Errorf("%s: %s", resp.Status, redactLiveText(cfg, strings.TrimSpace(string(respBody))))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		return resp.StatusCode, nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 10*1024*1024)).Decode(out); err != nil && err != io.EOF {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

func cleanupClientRaw(ctx context.Context, t *testing.T, client *http.Client, cfg config.OneUserConfig, wsID, clientID, fallbackName string) {
	t.Helper()
	path, err := paths.Workspace(wsID, "clients", clientID)
	if err != nil {
		t.Errorf("cleanup client path: %v", err)
		return
	}
	name := currentNameRaw(ctx, client, cfg, path, fallbackName)
	var archived map[string]json.RawMessage
	if _, err := liveJSONRaw(ctx, client, cfg, http.MethodPut, path, map[string]any{"name": name, "archived": true}, &archived); err != nil {
		t.Errorf("archive client cleanup %s: %v", clientID, err)
	}
	cleanupDeleteRaw(ctx, t, client, cfg, path,
		findingsDelete{templatePath: "/workspaces/{workspaceId}/clients/{clientId}", fixture: "fixtures/live-shape/clients-delete.txt"})
}

func cleanupProjectRaw(ctx context.Context, t *testing.T, client *http.Client, cfg config.OneUserConfig, wsID, projectID, fallbackName string) {
	t.Helper()
	path, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		t.Errorf("cleanup project path: %v", err)
		return
	}
	name := currentNameRaw(ctx, client, cfg, path, fallbackName)
	var archived map[string]json.RawMessage
	if _, err := liveJSONRaw(ctx, client, cfg, http.MethodPut, path, map[string]any{"name": name, "archived": true}, &archived); err != nil {
		t.Errorf("archive project cleanup %s: %v", projectID, err)
	}
	cleanupDeleteRaw(ctx, t, client, cfg, path,
		findingsDelete{templatePath: "/workspaces/{workspaceId}/projects/{projectId}", fixture: "fixtures/live-shape/projects-delete.txt"})
}

func cleanupTaskRaw(ctx context.Context, t *testing.T, client *http.Client, cfg config.OneUserConfig, wsID, projectID, taskID, fallbackName string) {
	t.Helper()
	path, err := paths.Workspace(wsID, "projects", projectID, "tasks", taskID)
	if err != nil {
		t.Errorf("cleanup task path: %v", err)
		return
	}
	name := currentNameRaw(ctx, client, cfg, path, fallbackName)
	var done map[string]json.RawMessage
	if _, err := liveJSONRaw(ctx, client, cfg, http.MethodPut, path, map[string]any{"name": name, "billable": true, "status": "DONE"}, &done); err != nil {
		t.Errorf("mark task done cleanup %s: %v", taskID, err)
	}
	cleanupDeleteRaw(ctx, t, client, cfg, path,
		findingsDelete{templatePath: "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}", fixture: "fixtures/live-shape/tasks-delete.txt"})
}

// findingsDelete carries the template-path + fixture metadata needed to emit a
// promotable findings row for a cleanup DELETE. It is optional: a zero value
// (no templatePath) keeps the cleanup status-silent, preserving the prior
// behaviour for deletes whose op should not be promoted.
type findingsDelete struct {
	templatePath string
	fixture      string
}

// cleanupDeleteRaw issues the teardown DELETE. Historically it discarded the
// HTTP status entirely, which is exactly why the ~5 DELETE ops could never be
// promoted to live-success — the harness exercised them but never captured the
// real 2xx. It now CAPTURES the DELETE status and, when findings metadata is
// supplied, emits a copy-pasteable findings row for the real captured status
// (the row only promotes a stamp if the status is a 2xx, per the generator's
// status_bucket). It still treats a delete error as a test error so a failed
// teardown never silently leaks objects or fabricates a row.
func cleanupDeleteRaw(ctx context.Context, t *testing.T, client *http.Client, cfg config.OneUserConfig, path string, finding ...findingsDelete) {
	t.Helper()
	status, err := liveJSONRaw(ctx, client, cfg, http.MethodDelete, path, nil, nil)
	if err != nil {
		t.Errorf("delete cleanup %s: %v", livePathLabel(cfg, path), err)
		return
	}
	for _, f := range finding {
		if f.templatePath == "" {
			continue
		}
		logFindingsRow(t, http.MethodDelete, "api.clockify.me", f.templatePath, status, f.fixture)
	}
}

func currentNameRaw(ctx context.Context, client *http.Client, cfg config.OneUserConfig, path, fallback string) string {
	var current map[string]json.RawMessage
	if _, err := liveJSONRaw(ctx, client, cfg, http.MethodGet, path, nil, &current); err == nil {
		if name := stringField(current, "name"); name != "" {
			return name
		}
	}
	return fallback
}

func hasRequiredTimeEntryCustomFields(ctx context.Context, t *testing.T, client *http.Client, cfg config.OneUserConfig, wsID string) bool {
	t.Helper()
	path, err := paths.Workspace(wsID, "custom-fields")
	if err != nil {
		t.Fatalf("custom fields path: %v", err)
	}
	var fields []map[string]any
	if _, err := liveJSONRaw(ctx, client, cfg, http.MethodGet, path, nil, &fields); err != nil {
		t.Fatalf("GET %s: %v", livePathLabel(cfg, path), err)
	}
	for _, field := range fields {
		required, _ := field["required"].(bool)
		if !required {
			continue
		}
		for _, key := range []string{"entityType", "entity_type", "targetEntityType", "target_entity_type", "resourceType", "resource_type", "documentCode", "document_code"} {
			if liveScopeIncludesTimeEntry(field[key]) {
				return true
			}
		}
		for _, key := range []string{"entityTypes", "entity_types", "targetEntityTypes", "target_entity_types", "appliesTo", "applies_to"} {
			if liveScopeIncludesTimeEntry(field[key]) {
				return true
			}
		}
	}
	return false
}

func logFindingsRow(t *testing.T, method, host, path string, status int, fixture string) {
	t.Helper()
	t.Logf("| %s | %s | %s | %d | %s |", method, host, path, status, fixture)
}

func requireStringField(t *testing.T, label string, item map[string]json.RawMessage, field string) string {
	t.Helper()
	value := stringField(item, field)
	if value == "" {
		t.Fatalf("%s missing non-empty %s; fields=%v", label, field, sortedKeys(item))
	}
	return value
}

func assertHasAnyKey(t *testing.T, label string, item map[string]json.RawMessage, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := item[key]; ok {
			return
		}
	}
	t.Fatalf("%s missing expected keys %v; got %v", label, keys, sortedKeys(item))
}

// assertItemsHaveID asserts that every object in a decoded collection carries a
// non-empty id field, at the same strictness as the read-side schema checks. An
// empty collection is skipped (the sacrificial workspace may legitimately hold
// none of a given entity), mirroring the read-side "no live sample" handling.
func assertItemsHaveID(t *testing.T, label string, items []map[string]json.RawMessage) {
	t.Helper()
	if len(items) == 0 {
		t.Skipf("%s: no items returned; schema diff has no live sample", label)
	}
	for i, item := range items {
		if id := stringField(item, "id"); id == "" {
			t.Fatalf("%s[%d]: expected non-empty id, got fields %v", label, i, sortedKeys(item))
		}
	}
}

// assertApprovalRequestsHaveID mirrors assertItemsHaveID for the approval-requests
// list, whose elements wrap the request (and its id) under a nested
// "approvalRequest" object instead of carrying a top-level id.
func assertApprovalRequestsHaveID(t *testing.T, label string, items []map[string]json.RawMessage) {
	t.Helper()
	if len(items) == 0 {
		t.Skipf("%s: no items returned; schema diff has no live sample", label)
	}
	for i, item := range items {
		// Tolerate a top-level id if the API ever flattens, but the live
		// shape nests it under approvalRequest.
		if id := stringField(item, "id"); id != "" {
			continue
		}
		nestedRaw, ok := item["approvalRequest"]
		if !ok {
			t.Fatalf("%s[%d]: expected an approvalRequest wrapper, got fields %v", label, i, sortedKeys(item))
		}
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(nestedRaw, &nested); err != nil {
			t.Fatalf("%s[%d]: approvalRequest is not an object: %v", label, i, err)
		}
		if id := stringField(nested, "id"); id == "" {
			t.Fatalf("%s[%d]: approvalRequest has no id, fields %v", label, i, sortedKeys(nested))
		}
	}
}

func sortedKeys(item map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// livePostRaw issues a JSON POST against the live API, mirroring liveGetRaw. It
// exists for time-off request listing, which Clockify exposes only via POST.
func livePostRaw(ctx context.Context, client *http.Client, cfg config.OneUserConfig, path string, body, out any) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	u := strings.TrimRight(cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "clockify-mcp-live-schema-diff")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return fmt.Errorf("%s: %s", resp.Status, redactLiveText(cfg, strings.TrimSpace(string(respBody))))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 10*1024*1024)).Decode(out)
}

func previousFullWeekRange(now time.Time) (string, string) {
	dayOffset := (int(now.Weekday()) + 6) % 7
	thisMonday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -dayOffset)
	start := thisMonday.AddDate(0, 0, -7)
	end := thisMonday.Add(-time.Millisecond)
	return start.Format("2006-01-02T15:04:05.000"), end.Format("2006-01-02T15:04:05.000")
}

func setupLiveSchemaConfig(t *testing.T) config.OneUserConfig {
	t.Helper()
	if os.Getenv("CLOCKIFY_API_KEY") == "" {
		t.Skip("Skipping live schema diff since CLOCKIFY_API_KEY is not set")
	}
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" {
		t.Skip("Skipping live schema diff unless CLOCKIFY_RUN_LIVE_E2E=1")
	}
	cfg, err := config.LoadOneUser()
	if err != nil {
		t.Fatalf("load one-user config: %v", err)
	}
	MarkLiveTestRan()
	return cfg
}

func liveGetRaw(ctx context.Context, client *http.Client, cfg config.OneUserConfig, path string, query map[string]string, out any) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/") + path)
	if err != nil {
		return err
	}
	q := u.Query()
	for k, v := range query {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "clockify-mcp-live-schema-diff")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return fmt.Errorf("%s: %s", resp.Status, redactLiveText(cfg, strings.TrimSpace(string(body))))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 10*1024*1024)).Decode(out)
}

func livePathLabel(cfg config.OneUserConfig, path string) string {
	return redactLiveText(cfg, path)
}

func redactLiveText(cfg config.OneUserConfig, text string) string {
	pairs := make([]string, 0, 4)
	if cfg.APIKey != "" {
		pairs = append(pairs, cfg.APIKey, "<api-key>")
	}
	if cfg.WorkspaceID != "" {
		pairs = append(pairs, cfg.WorkspaceID, "{workspaceId}")
	}
	if len(pairs) == 0 {
		return text
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

func firstPageQuery() map[string]string {
	return map[string]string{"page": "1", "page-size": "50"}
}

func assertNonEmpty(t *testing.T, label string, n int) {
	t.Helper()
	if n == 0 {
		t.Fatalf("%s returned no objects; cannot prove schema shape", label)
	}
}

func assertNoUnknownFields[T any](t *testing.T, label string, obj map[string]json.RawMessage) {
	t.Helper()
	allowed := jsonFieldSet[T]()
	unknown := make([]string, 0)
	for name := range obj {
		if !allowed[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return
	}
	sort.Strings(unknown)
	t.Fatalf("%s returned fields not represented in %T: %s", label, *new(T), strings.Join(unknown, ", "))
}

func jsonFieldSet[T any]() map[string]bool {
	var zero T
	typ := reflect.TypeOf(zero)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	fields := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = field.Name
		}
		fields[name] = true
	}
	return fields
}

func firstStringField(items []map[string]json.RawMessage, field string) string {
	for _, item := range items {
		if v := stringField(item, field); v != "" {
			return v
		}
	}
	return ""
}

func stringField(item map[string]json.RawMessage, field string) string {
	var value string
	if raw := item[field]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}
