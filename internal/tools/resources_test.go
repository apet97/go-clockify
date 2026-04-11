package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
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
	if len(list) != 2 {
		t.Fatalf("expected 2 concrete resources, got %d", len(list))
	}
	if !strings.HasPrefix(list[0].URI, "clockify://workspace/ws1") {
		t.Fatalf("first uri: %q", list[0].URI)
	}
	if !strings.HasSuffix(list[1].URI, "/user/current") {
		t.Fatalf("second uri: %q", list[1].URI)
	}
}

func TestResourcesListTemplatesFive(t *testing.T) {
	svc := &Service{}
	tmpls, err := svc.ListResourceTemplates(context.Background())
	if err != nil {
		t.Fatalf("templates: %v", err)
	}
	if len(tmpls) != 5 {
		t.Fatalf("expected 5 templates, got %d", len(tmpls))
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
