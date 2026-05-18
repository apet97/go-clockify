package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestClientProjectTaskDocSchemaProperties(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descriptors := map[string]map[string]any{}
	for _, d := range svc.FullAccessRegistry() {
		descriptors[d.Tool.Name] = d.Tool.InputSchema
	}
	requireProps := func(tool string, props ...string) {
		t.Helper()
		schema, ok := descriptors[tool]
		if !ok {
			t.Fatalf("missing descriptor for %s", tool)
		}
		got, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s properties = %T", tool, schema["properties"])
		}
		for _, prop := range props {
			if _, ok := got[prop]; !ok {
				t.Fatalf("%s missing property %s", tool, prop)
			}
		}
	}

	requireProps("clockify_clients_list", "page", "page_size", "name", "archived", "address", "note", "sort_column", "sort_order")
	requireProps("clockify_clients_get", "client", "client_id")
	requireProps("clockify_clients_create", "name")
	requireProps("clockify_clients_update", "client", "client_id", "name", "archive_projects", "mark_tasks_as_done")
	requireProps("clockify_projects_list", "page", "page_size", "name", "strict_name_search", "archived", "billable", "clients", "contains_client", "client_status", "users", "contains_user", "user_status", "is_template", "sort_column", "sort_order", "hydrated", "access", "expense_limit", "expense_date", "user_groups", "contains_group")
	requireProps("clockify_projects_get", "project", "project_id", "hydrated", "custom_field_entity_type", "expense_limit", "expense_date")
	requireProps("clockify_projects_create", "name", "client", "client_id", "color", "billable", "is_public")
	requireProps("clockify_projects_update", "project", "project_id", "name", "client", "client_id", "color", "billable", "is_public", "archived")
	requireProps("clockify_projects_rates_update", "project_id", "user_id", "rate_kind", "amount")
	requireProps("clockify_tasks_list", "project", "project_id", "page", "page_size", "name", "strict_name_search", "is_active", "sort_column", "sort_order")
	requireProps("clockify_tasks_get", "project", "project_id", "task", "task_id")
	requireProps("clockify_tasks_create", "project", "project_id", "name", "billable", "contains_assignee")
	requireProps("clockify_tasks_update", "project", "project_id", "task", "task_id", "name", "billable", "assignee_ids", "status", "contains_assignee", "membership_status")
	requireProps("clockify_tasks_rates_update", "project_id", "task_id", "rate_kind", "amount")
}

func TestClientsListWrapperFiltersForwarded(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/clients" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		want := map[string]string{
			"name":        "Client X",
			"address":     "123 Main St",
			"note":        "priority",
			"sort-column": "NAME",
			"sort-order":  "DESCENDING",
			"archived":    "false",
			"page":        "3",
			"page-size":   "25",
		}
		for key, value := range want {
			if got := q.Get(key); got != value {
				t.Fatalf("query %s = %q, want %q (raw=%s)", key, got, value, r.URL.RawQuery)
			}
		}
		respondJSON(t, w, []clockify.ClientEntity{{ID: testClientID, Name: "Client X"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.ClientsList(context.Background(), map[string]any{
		"name":        "Client X",
		"address":     "123 Main St",
		"note":        "priority",
		"sort_column": "NAME",
		"sort_order":  "DESCENDING",
		"archived":    false,
		"page":        3,
		"page_size":   25,
	}); err != nil {
		t.Fatalf("ClientsList: %v", err)
	}
}

func TestUpdateClientDocFieldsAndArchiveQueryForwarded(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == path:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Client X", Email: "clientx@example.com"})
		case r.Method == http.MethodPut && r.URL.Path == path:
			q := r.URL.Query()
			if q.Get("archive-projects") != "true" || q.Get("mark-tasks-as-done") != "false" {
				t.Fatalf("unexpected client update query: %s", r.URL.RawQuery)
			}
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode PUT body: %v", err)
			}
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Client X", Archived: true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.UpdateClient(context.Background(), map[string]any{
		"client":             testClientID,
		"archived":           true,
		"cc_emails":          []any{"billing@example.com"},
		"currency_id":        "53a687e29ae1f428e7ebe888",
		"archive_projects":   true,
		"mark_tasks_as_done": false,
	}); err != nil {
		t.Fatalf("UpdateClient: %v", err)
	}
	if got := putBody["currencyId"]; got != "53a687e29ae1f428e7ebe888" {
		t.Fatalf("currencyId = %v", got)
	}
	if got := putBody["ccEmails"]; !reflect.DeepEqual(got, []any{"billing@example.com"}) {
		t.Fatalf("ccEmails = %#v", got)
	}
	if got := putBody["archived"]; got != true {
		t.Fatalf("archived = %v", got)
	}
}

func TestProjectsListWrapperFiltersForwarded(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/projects" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		want := map[string]string{
			"name":               "Project X",
			"strict-name-search": "true",
			"archived":           "false",
			"billable":           "true",
			"contains-client":    "true",
			"client-status":      "ACTIVE",
			"contains-user":      "false",
			"user-status":        "ALL",
			"is-template":        "false",
			"sort-column":        "NAME",
			"sort-order":         "ASCENDING",
			"hydrated":           "true",
			"access":             "PUBLIC",
			"expense-limit":      "5000",
			"expense-date":       "2026-05-12",
			"contains-group":     "true",
			"page":               "2",
			"page-size":          "10",
		}
		for key, value := range want {
			if got := q.Get(key); got != value {
				t.Fatalf("query %s = %q, want %q (raw=%s)", key, got, value, r.URL.RawQuery)
			}
		}
		if got := q["clients"]; !reflect.DeepEqual(got, []string{"c-1", "c-2"}) {
			t.Fatalf("clients query = %#v", got)
		}
		if got := q["users"]; !reflect.DeepEqual(got, []string{"u-1", "u-2"}) {
			t.Fatalf("users query = %#v", got)
		}
		if got := q["userGroups"]; !reflect.DeepEqual(got, []string{"g-1"}) {
			t.Fatalf("userGroups query = %#v", got)
		}
		respondJSON(t, w, []clockify.Project{{ID: testProjectID, Name: "Project X"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.ProjectsList(context.Background(), map[string]any{
		"name":               "Project X",
		"strict_name_search": true,
		"archived":           false,
		"billable":           true,
		"clients":            []any{"c-1", "c-2"},
		"contains_client":    true,
		"client_status":      "ACTIVE",
		"users":              []any{"u-1", "u-2"},
		"contains_user":      false,
		"user_status":        "ALL",
		"is_template":        false,
		"sort_column":        "NAME",
		"sort_order":         "ASCENDING",
		"hydrated":           true,
		"access":             "PUBLIC",
		"expense_limit":      5000,
		"expense_date":       "2026-05-12",
		"user_groups":        []any{"g-1"},
		"contains_group":     true,
		"page":               2,
		"page_size":          10,
	}); err != nil {
		t.Fatalf("ProjectsList: %v", err)
	}
}

func TestProjectDocUpdateRichBodyForwarded(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/projects/" + testProjectID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == path:
			respondJSON(t, w, clockify.Project{ID: testProjectID, Name: "Existing", ClientID: "c-old", Archived: false})
		case r.Method == http.MethodPut && r.URL.Path == path:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode PUT body: %v", err)
			}
			respondJSON(t, w, clockify.Project{ID: testProjectID, Name: "Existing", ClientID: "c-new"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.UpdateProject(context.Background(), map[string]any{
		"project":   testProjectID,
		"client_id": "c-new",
		"estimate_reset": map[string]any{
			"active":       true,
			"day_of_month": 1.0,
			"day_of_week":  "MONDAY",
			"hour":         8.0,
			"interval":     "MONTHLY",
			"is_active":    true,
			"month":        "JANUARY",
		},
	}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if putBody["clientId"] != "c-new" {
		t.Fatalf("clientId = %v", putBody["clientId"])
	}
	assertNestedValue(t, putBody, []string{"estimateReset", "dayOfWeek"}, "MONDAY")
	assertNestedValue(t, putBody, []string{"estimateReset", "isActive"}, true)
}

func TestTasksListWrapperFiltersForwarded(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/projects/"+testProjectID+"/tasks" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		want := map[string]string{
			"name":               "Task X",
			"strict-name-search": "true",
			"is-active":          "false",
			"sort-column":        "ID",
			"sort-order":         "ASCENDING",
			"page":               "4",
			"page-size":          "20",
		}
		for key, value := range want {
			if got := q.Get(key); got != value {
				t.Fatalf("query %s = %q, want %q (raw=%s)", key, got, value, r.URL.RawQuery)
			}
		}
		respondJSON(t, w, []clockify.Task{{ID: testTaskID, Name: "Task X", ProjectID: testProjectID}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.TasksList(context.Background(), map[string]any{
		"project":            testProjectID,
		"name":               "Task X",
		"strict_name_search": true,
		"is_active":          false,
		"sort_column":        "ID",
		"sort_order":         "ASCENDING",
		"page":               4,
		"page_size":          20,
	}); err != nil {
		t.Fatalf("TasksList: %v", err)
	}
}

func assertNestedValue(t *testing.T, body map[string]any, path []string, want any) {
	t.Helper()
	cur := any(body)
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%v: parent is %T, want object", path, cur)
		}
		cur = m[key]
	}
	if !reflect.DeepEqual(cur, want) {
		t.Fatalf("%v = %#v, want %#v", path, cur, want)
	}
}
