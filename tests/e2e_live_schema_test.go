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

func TestLiveReadSideSchemaDiff(t *testing.T) {
	cfg := setupLiveSchemaConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
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

func setupLiveSchemaConfig(t *testing.T) config.Config {
	t.Helper()
	if os.Getenv("CLOCKIFY_API_KEY") == "" {
		t.Skip("Skipping live schema diff since CLOCKIFY_API_KEY is not set")
	}
	if os.Getenv("CLOCKIFY_RUN_LIVE_E2E") != "1" {
		t.Skip("Skipping live schema diff unless CLOCKIFY_RUN_LIVE_E2E=1")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func liveGetRaw(ctx context.Context, client *http.Client, cfg config.Config, path string, query map[string]string, out any) error {
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
