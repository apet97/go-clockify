//go:build livee2e

package e2e_test

import (
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
		// list_expenses reads a paged {expenses:[...]} envelope.
		var env struct {
			Expenses []map[string]json.RawMessage `json:"expenses"`
		}
		get(mustWorkspacePath("expenses"), firstPageQuery(), &env)
		assertItemsHaveID(t, "expenses", env.Expenses)
	})

	t.Run("approvals", func(t *testing.T) {
		// approval-requests list accepts only PENDING/APPROVED/WITHDRAWN_APPROVAL
		// (AGENTS.md gotcha). The response is a bare array of request objects.
		var requests []map[string]json.RawMessage
		get(mustWorkspacePath("approval-requests"), map[string]string{"status": "PENDING"}, &requests)
		assertItemsHaveID(t, "approvals", requests)
	})

	t.Run("scheduling-assignments", func(t *testing.T) {
		// scheduling_assignments_list reads the /scheduling/assignments/all route
		// (bare /assignments returns 404 — AGENTS.md gotcha).
		var assignments []map[string]json.RawMessage
		get(mustWorkspacePath("scheduling", "assignments", "all"), firstPageQuery(), &assignments)
		assertItemsHaveID(t, "scheduling-assignments", assignments)
	})

	t.Run("time-off-policies", func(t *testing.T) {
		// time_off_policies_list reads a paged {policies:[...]} envelope.
		var env struct {
			Policies []map[string]json.RawMessage `json:"policies"`
		}
		get(mustWorkspacePath("time-off", "policies"), firstPageQuery(), &env)
		assertItemsHaveID(t, "time-off-policies", env.Policies)
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
		// webhooks_list reads the /webhooks route; response is a bare array.
		var hooks []map[string]json.RawMessage
		get(mustWorkspacePath("webhooks"), nil, &hooks)
		assertItemsHaveID(t, "webhooks", hooks)
	})
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(string(payload)))
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
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 10*1024*1024)).Decode(out)
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
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 10*1024*1024)).Decode(out)
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
