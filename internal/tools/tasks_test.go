package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

// testTaskID and testTaskProjectID are 24-char hex values that the
// resolver treats as Clockify ObjectIDs — resolution short-circuits to
// the GET-by-ID path without a list lookup.
const testTaskID = "6a00f6bc2568d3d293061e2a"
const testTaskProjectID = "6a00f2e32568d3d29305ecc2"

func TestGetTaskByID(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects/"+testTaskProjectID+"/tasks/"+testTaskID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Build API", ProjectID: testTaskProjectID, Status: "ACTIVE"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.GetTask(context.Background(), map[string]any{
		"project": testTaskProjectID,
		"task":    testTaskID,
	})
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	task, ok := result.Data.(TaskView)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if task.ID != testTaskID || task.Name != "Build API" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if result.Meta["workspaceId"] != "ws1" || result.Meta["projectId"] != testTaskProjectID || result.Meta["taskId"] != testTaskID {
		t.Fatalf("unexpected meta: %+v", result.Meta)
	}
}

func TestGetTaskRequiresProject(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.GetTask(context.Background(), map[string]any{"task": testTaskID}); err == nil {
		t.Fatal("expected error when project is missing")
	} else if !strings.Contains(err.Error(), "project") {
		t.Fatalf("expected error to mention project, got %v", err)
	}
}

func TestGetTaskRequiresTask(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// list projects for name resolution
		if r.URL.Path == "/workspaces/ws1/projects" {
			respondJSON(t, w, []map[string]any{{"id": testTaskProjectID, "name": "My Project"}})
			return
		}
		// get individual project
		if r.URL.Path == "/workspaces/ws1/projects/"+testTaskProjectID {
			respondJSON(t, w, clockify.Project{ID: testTaskProjectID, Name: "My Project"})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.GetTask(context.Background(), map[string]any{"project": testTaskProjectID}); err == nil {
		t.Fatal("expected error when task is missing")
	} else if !strings.Contains(err.Error(), "task") {
		t.Fatalf("expected error to mention task, got %v", err)
	}
}

func TestUpdateTaskFetchThenMerge(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		taskPath := "/workspaces/ws1/projects/" + testTaskProjectID + "/tasks/" + testTaskID
		switch {
		case r.URL.Path == taskPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Old Name", ProjectID: testTaskProjectID, Status: "ACTIVE", Billable: false})
		case r.URL.Path == taskPath && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "New Name", ProjectID: testTaskProjectID, Status: "DONE", Billable: true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateTask(context.Background(), map[string]any{
		"project":  testTaskProjectID,
		"task":     testTaskID,
		"name":     "New Name",
		"status":   "DONE",
		"billable": true,
	})
	if err != nil {
		t.Fatalf("update task failed: %v", err)
	}
	if putBody["name"] != "New Name" {
		t.Fatalf("PUT body must carry new name, got %v", putBody["name"])
	}
	if putBody["status"] != "DONE" {
		t.Fatalf("PUT body must carry new status, got %v", putBody["status"])
	}
	if putBody["billable"] != true {
		t.Fatalf("PUT body must carry billable:true, got %v", putBody["billable"])
	}
	changed, _ := result.Meta["changedFields"].([]string)
	if len(changed) != 3 {
		t.Fatalf("expected changedFields=[name status billable], got %v", changed)
	}
}

func TestUpdateTaskPreservesUnchangedFields(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		taskPath := "/workspaces/ws1/projects/" + testTaskProjectID + "/tasks/" + testTaskID
		switch {
		case r.URL.Path == taskPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Existing", ProjectID: testTaskProjectID, Status: "ACTIVE", Estimate: "PT2H"})
		case r.URL.Path == taskPath && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Existing", ProjectID: testTaskProjectID, Status: "ACTIVE", Estimate: "PT2H"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	// caller supplies no changes — PUT should still carry existing name
	if _, err := svc.UpdateTask(context.Background(), map[string]any{
		"project": testTaskProjectID,
		"task":    testTaskID,
	}); err != nil {
		t.Fatalf("update task failed: %v", err)
	}
	if putBody["name"] != "Existing" {
		t.Fatalf("PUT body must preserve existing name, got %v", putBody["name"])
	}
	if putBody["estimate"] != "PT2H" {
		t.Fatalf("PUT body must preserve existing estimate, got %v", putBody["estimate"])
	}
}

func TestUpdateTaskDryRunDoesNotMutate(t *testing.T) {
	var mutated bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		taskPath := "/workspaces/ws1/projects/" + testTaskProjectID + "/tasks/" + testTaskID
		switch {
		case r.URL.Path == taskPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Existing", ProjectID: testTaskProjectID})
		case r.URL.Path == taskPath && r.Method == http.MethodPut:
			mutated = true
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateTask(context.Background(), map[string]any{
		"project": testTaskProjectID,
		"task":    testTaskID,
		"name":    "New Name",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("update task dry-run failed: %v", err)
	}
	if mutated {
		t.Fatal("dry-run must not issue PUT")
	}
	if result.Action != "clockify_tasks_update" {
		t.Fatalf("unexpected action: %q", result.Action)
	}
}

func TestUpdateTaskRequiresProject(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.UpdateTask(context.Background(), map[string]any{"task": testTaskID}); err == nil {
		t.Fatal("expected error when project is missing")
	}
}

func TestUpdateTaskRequiresTask(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/workspaces/ws1/projects" {
			respondJSON(t, w, []map[string]any{{"id": testTaskProjectID, "name": "My Project"}})
			return
		}
		if r.URL.Path == "/workspaces/ws1/projects/"+testTaskProjectID {
			respondJSON(t, w, clockify.Project{ID: testTaskProjectID, Name: "My Project"})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.UpdateTask(context.Background(), map[string]any{"project": testTaskProjectID}); err == nil {
		t.Fatal("expected error when task is missing")
	}
}

func TestDeleteTaskMarksActiveTaskDoneThenDeletes(t *testing.T) {
	var deleted bool
	var putBody map[string]any
	var calls []string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		taskPath := "/workspaces/ws1/projects/" + testTaskProjectID + "/tasks/" + testTaskID
		switch {
		case r.URL.Path == taskPath && r.Method == http.MethodGet:
			calls = append(calls, "GET")
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Old Task", ProjectID: testTaskProjectID, Status: "ACTIVE"})
		case r.URL.Path == taskPath && r.Method == http.MethodPut:
			calls = append(calls, "PUT")
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Old Task", ProjectID: testTaskProjectID, Status: "DONE"})
		case r.URL.Path == taskPath && r.Method == http.MethodDelete:
			calls = append(calls, "DELETE")
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteTask(context.Background(), map[string]any{
		"project": testTaskProjectID,
		"task":    testTaskID,
	})
	if err != nil {
		t.Fatalf("delete task failed: %v", err)
	}
	if !deleted {
		t.Fatal("expected DELETE call")
	}
	if strings.Join(calls, ",") != "GET,PUT,DELETE" {
		t.Fatalf("expected GET,PUT,DELETE call order, got %v", calls)
	}
	if putBody["name"] != "Old Task" || putBody["status"] != "DONE" {
		t.Fatalf("PUT body must preserve task and mark DONE before delete, got %#v", putBody)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["deleted"] != true || data["taskId"] != testTaskID {
		t.Fatalf("unexpected response data: %+v", data)
	}
}

func TestDeleteTaskAlreadyDoneDeletesWithoutPut(t *testing.T) {
	var putCalled bool
	var deleted bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		taskPath := "/workspaces/ws1/projects/" + testTaskProjectID + "/tasks/" + testTaskID
		switch {
		case r.URL.Path == taskPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Done Task", ProjectID: testTaskProjectID, Status: "DONE"})
		case r.URL.Path == taskPath && r.Method == http.MethodPut:
			putCalled = true
			respondJSON(t, w, map[string]any{})
		case r.URL.Path == taskPath && r.Method == http.MethodDelete:
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.DeleteTask(context.Background(), map[string]any{
		"project": testTaskProjectID,
		"task":    testTaskID,
	}); err != nil {
		t.Fatalf("delete done task failed: %v", err)
	}
	if putCalled {
		t.Fatal("DONE task must not be PUT before DELETE")
	}
	if !deleted {
		t.Fatal("expected DELETE call")
	}
}

func TestDeleteTaskDryRunDoesNotMutate(t *testing.T) {
	var mutated bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		taskPath := "/workspaces/ws1/projects/" + testTaskProjectID + "/tasks/" + testTaskID
		switch {
		case r.URL.Path == taskPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Task{ID: testTaskID, Name: "Old Task", ProjectID: testTaskProjectID, Status: "ACTIVE"})
		case r.URL.Path == taskPath && r.Method == http.MethodPut:
			mutated = true
			respondJSON(t, w, map[string]any{})
		case r.URL.Path == taskPath && r.Method == http.MethodDelete:
			mutated = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteTask(context.Background(), map[string]any{
		"project": testTaskProjectID,
		"task":    testTaskID,
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("delete task dry-run failed: %v", err)
	}
	if mutated {
		t.Fatal("dry-run must not issue DELETE")
	}
	if result.Action != "clockify_tasks_delete" {
		t.Fatalf("unexpected action: %q", result.Action)
	}
	if result.Meta["wouldMarkDoneBeforeDelete"] != true {
		t.Fatalf("dry-run should preview mark-done step, meta=%+v", result.Meta)
	}
}

func TestDeleteTaskRequiresProject(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.DeleteTask(context.Background(), map[string]any{"task": testTaskID}); err == nil {
		t.Fatal("expected error when project is missing")
	}
}

func TestDeleteTaskRequiresTask(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/workspaces/ws1/projects" {
			respondJSON(t, w, []map[string]any{{"id": testTaskProjectID, "name": "My Project"}})
			return
		}
		if r.URL.Path == "/workspaces/ws1/projects/"+testTaskProjectID {
			respondJSON(t, w, clockify.Project{ID: testTaskProjectID, Name: "My Project"})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.DeleteTask(context.Background(), map[string]any{"project": testTaskProjectID}); err == nil {
		t.Fatal("expected error when task is missing")
	}
}
