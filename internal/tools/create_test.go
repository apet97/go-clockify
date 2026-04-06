package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"goclmcp/internal/clockify"
)

func TestCreateProject(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "My Project" {
				t.Fatalf("expected name 'My Project', got %v", body["name"])
			}
			respondJSON(t, w, clockify.Project{ID: "p1", Name: "My Project", Color: "#FF0000"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateProject(context.Background(), map[string]any{
		"name":  "My Project",
		"color": "#FF0000",
	})
	if err != nil {
		t.Fatalf("create project failed: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected OK=true")
	}
	project, ok := result.Data.(clockify.Project)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if project.ID != "p1" || project.Name != "My Project" {
		t.Fatalf("unexpected project: %+v", project)
	}
}

func TestCreateProjectWithClient(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/clients" && r.Method == http.MethodGet:
			// ListAll for client resolution
			respondJSON(t, w, []map[string]any{
				{"id": "c1", "name": "Acme Corp"},
			})
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "Client Project" {
				t.Fatalf("expected name 'Client Project', got %v", body["name"])
			}
			if body["clientId"] != "c1" {
				t.Fatalf("expected clientId 'c1', got %v", body["clientId"])
			}
			respondJSON(t, w, clockify.Project{ID: "p2", Name: "Client Project", ClientID: "c1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateProject(context.Background(), map[string]any{
		"name":   "Client Project",
		"client": "Acme Corp",
	})
	if err != nil {
		t.Fatalf("create project with client failed: %v", err)
	}
	project, ok := result.Data.(clockify.Project)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if project.ClientID != "c1" {
		t.Fatalf("expected clientID c1, got %s", project.ClientID)
	}
}

func TestCreateClient(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/clients" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "New Client" {
				t.Fatalf("expected name 'New Client', got %v", body["name"])
			}
			respondJSON(t, w, clockify.ClientEntity{ID: "c1", Name: "New Client"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateClient(context.Background(), map[string]any{
		"name": "New Client",
	})
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	ce, ok := result.Data.(clockify.ClientEntity)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if ce.ID != "c1" || ce.Name != "New Client" {
		t.Fatalf("unexpected client: %+v", ce)
	}
}

func TestCreateTag(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/tags" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "urgent" {
				t.Fatalf("expected name 'urgent', got %v", body["name"])
			}
			respondJSON(t, w, clockify.Tag{ID: "t1", Name: "urgent"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateTag(context.Background(), map[string]any{
		"name": "urgent",
	})
	if err != nil {
		t.Fatalf("create tag failed: %v", err)
	}
	tag, ok := result.Data.(clockify.Tag)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if tag.ID != "t1" || tag.Name != "urgent" {
		t.Fatalf("unexpected tag: %+v", tag)
	}
}

func TestCreateTask(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			// ListAll for project resolution
			respondJSON(t, w, []map[string]any{
				{"id": "p1", "name": "My Project"},
			})
		case r.URL.Path == "/workspaces/ws1/projects/p1/tasks" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "Build feature" {
				t.Fatalf("expected name 'Build feature', got %v", body["name"])
			}
			respondJSON(t, w, clockify.Task{ID: "tk1", Name: "Build feature", ProjectID: "p1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CreateTask(context.Background(), map[string]any{
		"project": "My Project",
		"name":    "Build feature",
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	task, ok := result.Data.(clockify.Task)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if task.ID != "tk1" || task.ProjectID != "p1" {
		t.Fatalf("unexpected task: %+v", task)
	}
}

func TestCreateProjectMissingName(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected")
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.CreateProject(context.Background(), map[string]any{
		"name": "",
	})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected 'name is required' error, got %v", err)
	}
}

func TestCreateTaskMissingProject(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected")
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.CreateTask(context.Background(), map[string]any{
		"project": "",
		"name":    "Some task",
	})
	if err == nil || !strings.Contains(err.Error(), "project is required") {
		t.Fatalf("expected 'project is required' error, got %v", err)
	}
}
