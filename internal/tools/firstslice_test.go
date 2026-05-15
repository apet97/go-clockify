package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/testclockify"
)

var firstSliceTools = []string{
	"clockify_status",
	"clockify_clients_list",
	"clockify_clients_create",
	"clockify_projects_list",
	"clockify_projects_create",
	"clockify_tasks_list",
	"clockify_tasks_create",
	"clockify_tags_list",
	"clockify_tags_create",
	"clockify_entries_list",
	"clockify_entries_create",
	"clockify_demo_seed",
	"clockify_demo_cleanup",
}

func TestFirstSliceRegistryContainsOnlyPhase1Tools(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	reg := svc.FirstSliceRegistry()
	got := map[string]bool{}
	for _, d := range reg {
		got[d.Tool.Name] = true
	}
	for _, want := range firstSliceTools {
		if !got[want] {
			t.Fatalf("missing first-slice tool %q from registry", want)
		}
	}
	if len(got) != len(firstSliceTools) {
		t.Fatalf("registry count = %d, want %d: %#v", len(got), len(firstSliceTools), got)
	}
	for _, forbidden := range []string{
		blockedTerm("clockify_", "activate_group"),
		blockedTerm("clockify_", "activate_tool"),
		blockedTerm("clockify_", "deactivate_group"),
		blockedTerm("clockify_", "search_tools"),
		blockedTerm("clockify_", "list_tools"),
	} {
		if got[forbidden] {
			t.Fatalf("old product tool exposed: %s", forbidden)
		}
	}
}

func TestFirstSliceInitializeAndToolsList(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	server := mcp.NewServer("test", svc.FirstSliceRegistry(), nil, nil)

	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"single-user full-access", "All tools are loaded", "Use workflow tools first"} {
		if !strings.Contains(text, want) {
			t.Fatalf("initialize missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{"confirmation", "policy", blockedTerm("Tier ", "2"), blockedTerm("ten", "ant"), blockedTerm("hos", "ted"), "activate"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("initialize contains forbidden language %q: %s", forbidden, text)
		}
	}

	raw, err = server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range resp.Result.Tools {
		names[tool.Name] = true
	}
	for _, want := range firstSliceTools {
		if !names[want] {
			t.Fatalf("tools/list missing %s; got %#v", want, names)
		}
	}
	if resp.Result.Tools[0].Name != "clockify_status" {
		t.Fatalf("first listed tool = %q, want clockify_status", resp.Result.Tools[0].Name)
	}
}

func TestFirstSliceStatusDemoSeedAndCleanup(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC

	status, err := svc.ClockifyStatus(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	statusResult := status.(ToolResult)
	if !statusResult.OK || statusResult.IDs["workspaceId"] != fake.WorkspaceID || statusResult.IDs["userId"] == "" {
		t.Fatalf("bad status result: %+v", statusResult)
	}

	seed, err := svc.ClockifyDemoSeed(context.Background(), map[string]any{"run_id": "test"})
	if err != nil {
		t.Fatal(err)
	}
	seedResult := seed.(ToolResult)
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		if seedResult.IDs[key] == "" {
			t.Fatalf("seed missing id %s: %+v", key, seedResult)
		}
	}
	if len(seedResult.Changed.Created) != 5 {
		t.Fatalf("seed created = %d, want 5: %+v", len(seedResult.Changed.Created), seedResult.Changed)
	}

	seedAgain, err := svc.ClockifyDemoSeed(context.Background(), map[string]any{"run_id": "test"})
	if err != nil {
		t.Fatal(err)
	}
	reused := seedAgain.(ToolResult).Changed.Reused
	if len(reused) != 5 {
		t.Fatalf("seed idempotency reused = %d, want 5: %+v", len(reused), reused)
	}

	cleanup, err := svc.ClockifyDemoCleanup(context.Background(), map[string]any{"run_id": "test"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupResult := cleanup.(ToolResult)
	if len(cleanupResult.Changed.Deleted) != 5 {
		t.Fatalf("cleanup deleted = %d, want 5: %+v", len(cleanupResult.Changed.Deleted), cleanupResult)
	}
}

func TestEnsureDemoProjectRevivesArchivedLeftover(t *testing.T) {
	const (
		workspaceID = "ws1"
		projectID   = "project-archived"
		clientID    = "client-1"
		prefix      = "MCP-LIVE-SHARED"
	)
	archivedProject := clockify.Project{
		ID:          projectID,
		Name:        prefix + " Project",
		ClientID:    clientID,
		Archived:    true,
		Billable:    true,
		Public:      true,
		WorkspaceID: workspaceID,
	}
	var restored bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/"+workspaceID+"/projects":
			if r.URL.Query().Get("archived") == "true" {
				respondJSON(t, w, []clockify.Project{archivedProject})
				return
			}
			respondJSON(t, w, []clockify.Project{})
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/"+workspaceID+"/projects/"+projectID:
			respondJSON(t, w, archivedProject)
		case r.Method == http.MethodPut && r.URL.Path == "/workspaces/"+workspaceID+"/projects/"+projectID:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["archived"] != false {
				t.Fatalf("restore payload archived=%v, want false", body["archived"])
			}
			restored = true
			updated := archivedProject
			updated.Archived = false
			respondJSON(t, w, updated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	defer cleanup()

	svc := New(client, workspaceID)
	project, reused, err := svc.ensureDemoProject(context.Background(), prefix, clientID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reused || !restored || project.ID != projectID || project.Archived {
		t.Fatalf("project=%+v reused=%v restored=%v", project, reused, restored)
	}
}

func TestWorkflowUpsertScansAllPages(t *testing.T) {
	ctx := context.Background()
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)

	t.Run("client", func(t *testing.T) {
		for i := 0; i < 240; i++ {
			if _, err := svc.createClient(ctx, fmt.Sprintf("client-%03d", i)); err != nil {
				t.Fatal(err)
			}
		}
		page2, _, _, err := svc.listClients(ctx, map[string]any{"page": 2, "page_size": 200})
		if err != nil {
			t.Fatal(err)
		}
		target := page2[0]
		client, reused, err := svc.ensureClientNamed(ctx, target.Name, true)
		if err != nil {
			t.Fatal(err)
		}
		if !reused || client.ID != target.ID {
			t.Fatalf("ensureClientNamed reused=%v id=%q want reused id=%q", reused, client.ID, target.ID)
		}
	})

	t.Run("project", func(t *testing.T) {
		for i := 0; i < 240; i++ {
			if _, _, err := svc.createProject(ctx, map[string]any{}, fmt.Sprintf("project-%03d", i)); err != nil {
				t.Fatal(err)
			}
		}
		page2, _, _, err := svc.listProjects(ctx, map[string]any{"page": 2, "page_size": 200, "hydrated": false})
		if err != nil {
			t.Fatal(err)
		}
		target := page2[0]
		project, reused, err := svc.ensureProjectNamed(ctx, target.Name, "", true, map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if !reused || project.ID != target.ID {
			t.Fatalf("ensureProjectNamed reused=%v id=%q want reused id=%q", reused, project.ID, target.ID)
		}
	})

	t.Run("task", func(t *testing.T) {
		project, _, err := svc.createProject(ctx, map[string]any{}, "task-parent")
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 240; i++ {
			if _, err := svc.createTask(ctx, project.ID, fmt.Sprintf("task-%03d", i), nil); err != nil {
				t.Fatal(err)
			}
		}
		page2, _, _, err := svc.listTasks(ctx, project.ID, map[string]any{"page": 2, "page_size": 200})
		if err != nil {
			t.Fatal(err)
		}
		target := page2[0]
		task, reused, err := svc.ensureTaskNamed(ctx, project.ID, target.Name, true, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reused || task.ID != target.ID {
			t.Fatalf("ensureTaskNamed reused=%v id=%q want reused id=%q", reused, task.ID, target.ID)
		}
	})

	t.Run("tag", func(t *testing.T) {
		for i := 0; i < 240; i++ {
			if _, err := svc.createTag(ctx, fmt.Sprintf("tag-%03d", i)); err != nil {
				t.Fatal(err)
			}
		}
		page2, _, _, err := svc.listTags(ctx, map[string]any{"page": 2, "page_size": 200})
		if err != nil {
			t.Fatal(err)
		}
		target := page2[0]
		tag, reused, err := svc.ensureTagNamed(ctx, target.Name, true)
		if err != nil {
			t.Fatal(err)
		}
		if !reused || tag.ID != target.ID {
			t.Fatalf("ensureTagNamed reused=%v id=%q want reused id=%q", reused, tag.ID, target.ID)
		}
	})
}

func TestDemoCleanupScansAllProjectPages(t *testing.T) {
	ctx := context.Background()
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)

	for i := 0; i < 240; i++ {
		if _, _, err := svc.createProject(ctx, map[string]any{}, fmt.Sprintf("other-project-%03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	page2, _, _, err := svc.listProjects(ctx, map[string]any{"page": 2, "page_size": 200, "hydrated": false})
	if err != nil {
		t.Fatal(err)
	}
	target := page2[0]

	out, err := svc.ClockifyDemoCleanup(ctx, map[string]any{"prefix": target.Name})
	if err != nil {
		t.Fatal(err)
	}
	result := mustToolResult(t, out, nil)
	if len(result.Changed.Deleted) == 0 {
		t.Fatalf("cleanup did not delete prefixed project beyond first page: %+v", result)
	}
}

func TestFirstSliceWritesReturnIDsAndChanges(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC

	clientOut, err := svc.ClientsCreate(context.Background(), map[string]any{"name": "Phase 1 Client"})
	client := mustToolResult(t, clientOut, err)
	requireID(t, client, "clientId")
	requireCreated(t, client, "client")

	projectOut, err := svc.ProjectsCreate(context.Background(), map[string]any{
		"name":      "Phase 1 Project",
		"client_id": client.IDs["clientId"],
	})
	project := mustToolResult(t, projectOut, err)
	requireID(t, project, "projectId")
	requireCreated(t, project, "project")

	taskOut, err := svc.TasksCreate(context.Background(), map[string]any{
		"name":       "Phase 1 Task",
		"project_id": project.IDs["projectId"],
	})
	task := mustToolResult(t, taskOut, err)
	requireID(t, task, "taskId")
	requireCreated(t, task, "task")

	tagOut, err := svc.TagsCreate(context.Background(), map[string]any{"name": "Phase 1 Tag"})
	tag := mustToolResult(t, tagOut, err)
	requireID(t, tag, "tagId")
	requireCreated(t, tag, "tag")

	entryOut, err := svc.EntriesCreate(context.Background(), map[string]any{
		"start":       "2026-01-03 09:00",
		"end":         "2026-01-03 10:00",
		"description": "Phase 1 entry",
		"project_id":  project.IDs["projectId"],
		"task_id":     task.IDs["taskId"],
		"tag_ids":     []any{tag.IDs["tagId"]},
	})
	entry := mustToolResult(t, entryOut, err)
	requireID(t, entry, "entryId")
	requireCreated(t, entry, "entry")
}

func TestFirstSliceRecoverableErrorEnvelope(t *testing.T) {
	svc := New(clockify.NewClient("test-key", "http://127.0.0.1:1", time.Second, 0), "65b382b606de527a7ee2b60e")
	server := mcp.NewServer("test", svc.FirstSliceRegistry(), nil, nil)
	if _, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	raw, err := server.DispatchMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"clockify_clients_create","arguments":{"name":""}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Result struct {
			StructuredContent ToolError `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.StructuredContent.OK {
		t.Fatalf("expected ok=false envelope: %s", raw)
	}
	if resp.Result.StructuredContent.Recovery.Hint == "" {
		t.Fatalf("missing recovery hint: %+v", resp.Result.StructuredContent)
	}
}

func mustToolResult(t *testing.T, out any, err error) ToolResult {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	result, ok := out.(ToolResult)
	if !ok {
		t.Fatalf("result type = %T, want ToolResult", out)
	}
	if !result.OK {
		t.Fatalf("result not ok: %+v", result)
	}
	return result
}

func requireID(t *testing.T, result ToolResult, key string) {
	t.Helper()
	if result.IDs[key] == "" {
		t.Fatalf("%s missing from IDs: %+v", key, result.IDs)
	}
}

func requireCreated(t *testing.T, result ToolResult, entity string) {
	t.Helper()
	for _, ref := range result.Changed.Created {
		if ref.Type == entity && ref.ID != "" {
			return
		}
	}
	t.Fatalf("created %s missing from change set: %+v", entity, result.Changed)
}
