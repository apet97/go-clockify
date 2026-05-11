package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

// testProjectID is a 24-char hex value the resolver treats as an
// ObjectID, so resolution short-circuits to a direct GET-by-ID and
// the test does not need to stub the list endpoint.
const testProjectID = "6b00f2542568d3d29305e74e"

// TestUpdateProjectFetchThenMerge pins the fetch-then-merge contract:
// the handler must GET the project first, layer caller-provided
// non-empty fields on top, and PUT the full merged shape back.
// Empty/unspecified fields must NOT clear server-side data.
func TestUpdateProjectFetchThenMerge(t *testing.T) {
	var putBody map[string]any
	var getCalls, putCalls int
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/projects/" + testProjectID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			getCalls++
			respondJSON(t, w, clockify.Project{
				ID:       testProjectID,
				Name:     "Alpha",
				ClientID: "c-existing",
				Color:    "#11aa22",
				Note:     "original note",
				Billable: true,
				Public:   false,
				Archived: false,
			})
		case r.URL.Path == path && r.Method == http.MethodPut:
			putCalls++
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode PUT body: %v", err)
			}
			respondJSON(t, w, clockify.Project{
				ID:       testProjectID,
				Name:     "Alpha Renamed",
				ClientID: "c-existing",
				Color:    "#11aa22",
				Note:     "original note",
				Billable: false,
				Public:   false,
				Archived: false,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateProject(context.Background(), map[string]any{
		"project":  testProjectID,
		"name":     "Alpha Renamed",
		"billable": false,
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	if getCalls != 1 || putCalls != 1 {
		t.Fatalf("expected 1 GET + 1 PUT, got %d + %d", getCalls, putCalls)
	}
	if putBody["name"] != "Alpha Renamed" {
		t.Fatalf("merged name not pushed; got %#v", putBody["name"])
	}
	if putBody["billable"] != false {
		t.Fatalf("merged billable flag not pushed; got %#v", putBody["billable"])
	}
	// Caller did NOT supply note/color/client — they must persist.
	if putBody["note"] != "original note" {
		t.Fatalf("note must be preserved on merge; got %#v", putBody["note"])
	}
	if putBody["color"] != "#11aa22" {
		t.Fatalf("color must be preserved on merge; got %#v", putBody["color"])
	}
	if putBody["clientId"] != "c-existing" {
		t.Fatalf("clientId must be preserved on merge; got %#v", putBody["clientId"])
	}

	updated, ok := result.Data.(clockify.Project)
	if !ok {
		t.Fatalf("expected Project data, got %T", result.Data)
	}
	if updated.Name != "Alpha Renamed" {
		t.Fatalf("returned project name = %q", updated.Name)
	}
	changed, ok := result.Meta["changedFields"].([]string)
	if !ok {
		t.Fatalf("expected []string changedFields meta, got %T", result.Meta["changedFields"])
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields (name, billable), got %v", changed)
	}
}

// TestUpdateProjectRequiresProject covers the early-return guard so a
// caller that omits the project argument cannot trip a nil-pointer
// later in the merge path.
func TestUpdateProjectRequiresProject(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.UpdateProject(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error when project ref is empty")
	} else if !strings.Contains(err.Error(), "project") {
		t.Fatalf("expected error to mention project, got %v", err)
	}
}

// TestDeleteProjectArchivesActiveProject pins the archive-first
// contract that mirrors DeleteClient: a still-active project must
// be archived (PUT with name+archived=true) before the DELETE issues.
// The order is asserted; if delete fires before archive completes,
// archiveBefore stays false and the test reds.
func TestDeleteProjectArchivesActiveProject(t *testing.T) {
	var (
		archiveBody   map[string]any
		archived      bool
		deleted       bool
		archiveBefore bool
		deletedAfter  bool
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/projects/" + testProjectID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Project{
				ID:       testProjectID,
				Name:     "Alpha",
				Archived: false,
			})
		case r.URL.Path == path && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&archiveBody); err != nil {
				t.Fatalf("decode archive body: %v", err)
			}
			archived = true
			archiveBefore = !deleted
			respondJSON(t, w, clockify.Project{ID: testProjectID, Name: "Alpha", Archived: true})
		case r.URL.Path == path && r.Method == http.MethodDelete:
			deleted = true
			deletedAfter = archived
			respondJSON(t, w, clockify.Project{ID: testProjectID, Name: "Alpha", Archived: true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.DeleteProject(context.Background(), map[string]any{
		"project": testProjectID,
	}); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if !archiveBefore {
		t.Fatal("archive PUT must run before the DELETE — DeleteProject is archive-first by contract")
	}
	if !deletedAfter {
		t.Fatal("delete must run after the archive PUT")
	}
	if archiveBody["archived"] != true {
		t.Fatalf("archive PUT must set archived=true, got %#v", archiveBody)
	}
	if archiveBody["name"] != "Alpha" {
		t.Fatalf("archive PUT must echo name=Alpha (Clockify validator); got %#v", archiveBody["name"])
	}
}

// TestDeleteProjectSkipsArchiveWhenAlreadyArchived ensures the
// archive-first guard does not fire when the project is already
// archived (no point in re-archiving).
func TestDeleteProjectSkipsArchiveWhenAlreadyArchived(t *testing.T) {
	var putCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/projects/" + testProjectID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Project{
				ID:       testProjectID,
				Name:     "Alpha",
				Archived: true,
			})
		case r.URL.Path == path && r.Method == http.MethodPut:
			putCalled = true
		case r.URL.Path == path && r.Method == http.MethodDelete:
			respondJSON(t, w, clockify.Project{ID: testProjectID, Name: "Alpha", Archived: true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.DeleteProject(context.Background(), map[string]any{
		"project": testProjectID,
	}); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if putCalled {
		t.Fatal("PUT must not be called when the project is already archived")
	}
}

// TestDeleteProjectDryRunSkipsMutations verifies the dry-run preview
// stops after the read and never issues either the archive PUT or
// the DELETE.
func TestDeleteProjectDryRunSkipsMutations(t *testing.T) {
	var putCalled, deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/projects/" + testProjectID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Project{ID: testProjectID, Name: "Alpha", Archived: false})
		case r.URL.Path == path && r.Method == http.MethodPut:
			putCalled = true
		case r.URL.Path == path && r.Method == http.MethodDelete:
			deleteCalled = true
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.DeleteProject(context.Background(), map[string]any{
		"project": testProjectID,
		"dry_run": true,
	}); err != nil {
		t.Fatalf("DeleteProject dry-run: %v", err)
	}
	if putCalled || deleteCalled {
		t.Fatalf("dry-run must not mutate; archive=%v delete=%v", putCalled, deleteCalled)
	}
}
