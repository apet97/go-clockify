package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/testclockify"
)

func TestResourcesListCurrentWorkspaceAndUser(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(t, w, map[string]any{"id": "ws1", "name": "Workspace One"})
	})
	defer cleanup()
	svc := &Service{Client: client, WorkspaceID: "ws1"}

	list, err := svc.ListResources(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{
		"clockify://status",
		"clockify://workspace",
		"clockify://user",
		"clockify://features",
		"clockify://tools",
		"clockify://mcp/tool-catalog",
		"clockify://mcp/api-parity",
		"clockify://mcp/live-evidence",
		"clockify://workflows",
		"clockify://demo/phase1",
		"clockify://recent/entries",
		"clockify://recent/projects",
		"clockify://workspace/ws1",
		"clockify://workspace/ws1/user/current",
	} {
		if !hasResourceURI(list, want) {
			t.Fatalf("missing resource %q in %+v", want, list)
		}
	}
}

func TestOneUserResourcesReadStatusWorkspaceToolsWorkflows(t *testing.T) {
	fake := testclockify.NewServer("ws1")
	defer fake.Close()
	client := clockify.NewClient("test-key", fake.URL, 5*time.Second, 0)
	svc := New(client, fake.WorkspaceID)

	for _, uri := range []string{
		"clockify://status",
		"clockify://workspace",
		"clockify://tools",
		"clockify://workflows",
	} {
		t.Run(uri, func(t *testing.T) {
			contents, err := svc.ReadResource(context.Background(), uri)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if len(contents) != 1 {
				t.Fatalf("contents: %+v", contents)
			}
			if contents[0].MimeType != "application/json" || contents[0].Text == "" {
				t.Fatalf("content: %+v", contents[0])
			}
		})
	}
}

func TestSelfInspectionResources(t *testing.T) {
	fake := testclockify.NewServer("ws1")
	defer fake.Close()
	client := clockify.NewClient("test-key", fake.URL, 5*time.Second, 0)
	svc := New(client, fake.WorkspaceID)

	catalog, err := svc.ReadResource(context.Background(), "clockify://mcp/tool-catalog")
	if err != nil {
		t.Fatalf("read tool catalog: %v", err)
	}
	if len(catalog) != 1 || catalog[0].MimeType != "application/json" || !strings.Contains(catalog[0].Text, `"count":156`) {
		t.Fatalf("tool catalog content: %+v", catalog)
	}
	for _, uri := range []string{"clockify://mcp/api-parity", "clockify://mcp/live-evidence"} {
		contents, err := svc.ReadResource(context.Background(), uri)
		if err != nil {
			t.Fatalf("read %s: %v", uri, err)
		}
		if len(contents) != 1 || contents[0].MimeType != "text/markdown" || strings.TrimSpace(contents[0].Text) == "" {
			t.Fatalf("%s content: %+v", uri, contents)
		}
	}
}

func TestDemoResourcesTrackSeedAndCleanup(t *testing.T) {
	fake := testclockify.NewServer("ws1")
	defer fake.Close()
	client := clockify.NewClient("test-key", fake.URL, 5*time.Second, 0)
	svc := New(client, fake.WorkspaceID)
	ctx := context.Background()
	uri := "clockify://demo/resource-test"

	before, err := svc.ReadResource(ctx, uri)
	if err != nil {
		t.Fatalf("read before seed: %v", err)
	}
	if !strings.Contains(before[0].Text, `"status":"not_seeded"`) {
		t.Fatalf("before seed: %q", before[0].Text)
	}

	if _, err := svc.ClockifyDemoSeed(ctx, map[string]any{"run_id": "resource-test"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	seeded, err := svc.ReadResource(ctx, uri)
	if err != nil {
		t.Fatalf("read seeded: %v", err)
	}
	if !strings.Contains(seeded[0].Text, `"status":"seeded"`) || !strings.Contains(seeded[0].Text, `"entryId"`) {
		t.Fatalf("seeded resource: %q", seeded[0].Text)
	}
	resources, err := svc.ListResources(ctx)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if !hasResourceURI(resources, uri) {
		t.Fatalf("resources/list did not expose demo run resource %q", uri)
	}

	if _, err := svc.ClockifyDemoCleanup(ctx, map[string]any{"run_id": "resource-test"}); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	cleaned, err := svc.ReadResource(ctx, uri)
	if err != nil {
		t.Fatalf("read cleaned: %v", err)
	}
	if !strings.Contains(cleaned[0].Text, `"status":"cleaned"`) || !strings.Contains(cleaned[0].Text, `"deletedCount"`) {
		t.Fatalf("cleaned resource: %q", cleaned[0].Text)
	}
}

func TestResourcesListTemplatesIncludesDemo(t *testing.T) {
	svc := &Service{}
	tmpls, err := svc.ListResourceTemplates(context.Background())
	if err != nil {
		t.Fatalf("templates: %v", err)
	}
	if len(tmpls) != 7 {
		t.Fatalf("expected 7 templates, got %d", len(tmpls))
	}
	foundDemo := false
	for _, tmpl := range tmpls {
		if tmpl.URITemplate == "clockify://demo/{run_id}" {
			foundDemo = true
		}
	}
	if !foundDemo {
		t.Fatalf("missing demo resource template in %+v", tmpls)
	}
}

func TestResourcesReadWorkspaceDispatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": "ws1", "name": "Workspace One"})
	})
	defer cleanup()
	svc := &Service{Client: client, WorkspaceID: "ws1"}

	contents, err := svc.ReadResource(context.Background(), "clockify://workspace/ws1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("contents: %+v", contents)
	}
	if contents[0].MimeType != "application/json" {
		t.Fatalf("mime: %q", contents[0].MimeType)
	}
	if !strings.Contains(contents[0].Text, `"id":"ws1"`) {
		t.Fatalf("body missing id: %q", contents[0].Text)
	}
}

func TestResourcesReadUserCurrentDispatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Fatalf("expected /user route, got %q", r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": "u1", "name": "Alice"})
	})
	defer cleanup()
	svc := &Service{Client: client, WorkspaceID: "ws1"}

	contents, err := svc.ReadResource(context.Background(), "clockify://workspace/ws1/user/current")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(contents[0].Text, `"id":"u1"`) {
		t.Fatalf("body: %q", contents[0].Text)
	}
}

func TestResourcesReadProjectDispatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/projects/p1" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": "p1", "name": "Alpha"})
	})
	defer cleanup()
	svc := &Service{Client: client, WorkspaceID: "ws1"}

	contents, err := svc.ReadResource(context.Background(), "clockify://workspace/ws1/project/p1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(contents[0].Text, `"id":"p1"`) {
		t.Fatalf("body: %q", contents[0].Text)
	}
}

func TestResourcesReadEntryDispatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/time-entries/e1" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": "e1", "description": "x"})
	})
	defer cleanup()
	svc := &Service{Client: client, WorkspaceID: "ws1"}

	contents, err := svc.ReadResource(context.Background(), "clockify://workspace/ws1/entry/e1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(contents[0].Text, `"id":"e1"`) {
		t.Fatalf("body: %q", contents[0].Text)
	}
}

func TestResourcesReadGroupDispatch(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/user-groups/g1" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"id": "g1", "name": "Engineering"})
	})
	defer cleanup()
	svc := &Service{Client: client, WorkspaceID: "ws1"}

	contents, err := svc.ReadResource(context.Background(), "clockify://workspace/ws1/group/g1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(contents[0].Text, `"id":"g1"`) {
		t.Fatalf("body: %q", contents[0].Text)
	}
}

func TestResourcesReadWeeklyReportDoesNotMutateServiceWorkspace(t *testing.T) {
	const configuredWorkspace = "configured-ws"
	const resourceWorkspace = "resource-ws"
	observedWorkspace := make(chan string, 1)

	var svc *Service
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Alice"})
		case "/workspaces/" + resourceWorkspace + "/reports/weekly":
			if r.Method != http.MethodPost {
				t.Fatalf("weekly report method = %s, want POST", r.Method)
			}
			observedWorkspace <- svc.WorkspaceID
			respondJSON(t, w, map[string]any{
				"totals":      []map[string]any{},
				"groupOne":    []map[string]any{},
				"totalsByDay": []map[string]any{},
			})
		default:
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
	})
	defer cleanup()

	svc = &Service{Client: client, WorkspaceID: configuredWorkspace}
	contents, err := svc.ReadResource(context.Background(), "clockify://workspace/"+resourceWorkspace+"/report/weekly/2026-04-06")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("contents: %+v", contents)
	}
	if got := <-observedWorkspace; got != configuredWorkspace {
		t.Fatalf("weekly resource read mutated service workspace during upstream call: got %q, want %q", got, configuredWorkspace)
	}
	if svc.WorkspaceID != configuredWorkspace {
		t.Fatalf("service workspace after read: got %q, want %q", svc.WorkspaceID, configuredWorkspace)
	}
}

func TestResourcesReadRejectsUnknownScheme(t *testing.T) {
	svc := &Service{}
	_, err := svc.ReadResource(context.Background(), "http://example.com")
	if err == nil {
		t.Fatal("expected error for non-clockify scheme")
	}
}

func TestResourcesReadRejectsMalformedURI(t *testing.T) {
	svc := &Service{}
	for _, uri := range []string{
		"clockify://",
		"clockify://workspace",
		"clockify://workspace/ws1/unknown/x",
		"clockify://workspace/ws1/report/weekly",
	} {
		if _, err := svc.ReadResource(context.Background(), uri); err == nil {
			t.Fatalf("expected error for uri %q", uri)
		}
	}
}

func hasResourceURI(resources []mcp.Resource, uri string) bool {
	for _, resource := range resources {
		if resource.URI == uri {
			return true
		}
	}
	return false
}
