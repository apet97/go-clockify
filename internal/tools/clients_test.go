package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

const testClientID = "6a00f2542568d3d29305e74d"

func TestDeleteClientArchivesActiveClient(t *testing.T) {
	var (
		archiveBody    map[string]any
		archived       bool
		deleted        bool
		archiveBefore  bool
		deletedAfterFn bool
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: false})
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.Project{})
		case r.URL.Path == path && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&archiveBody); err != nil {
				t.Fatalf("decode archive body: %v", err)
			}
			archived = true
			archiveBefore = !deleted
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: true})
		case r.URL.Path == path && r.Method == http.MethodDelete:
			deleted = true
			deletedAfterFn = archived
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteClient(context.Background(), map[string]any{"client": testClientID})
	if err != nil {
		t.Fatalf("delete client failed: %v", err)
	}

	if !archived {
		t.Fatal("expected PUT archive call on active client")
	}
	if !archiveBefore {
		t.Fatal("archive must happen before delete")
	}
	if !deleted {
		t.Fatal("expected DELETE call after archive")
	}
	if !deletedAfterFn {
		t.Fatal("DELETE must occur after PUT archived=true")
	}
	if archiveBody["name"] != "Acme" {
		t.Fatalf("archive PUT must include existing name, got %v", archiveBody["name"])
	}
	if archiveBody["archived"] != true {
		t.Fatalf("archive PUT must set archived=true, got %v", archiveBody["archived"])
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["deleted"] != true || data["clientId"] != testClientID {
		t.Fatalf("unexpected response data: %+v", data)
	}
}

func TestDeleteClientSkipsArchiveWhenAlreadyArchived(t *testing.T) {
	var putCalls int
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: true})
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.Project{})
		case r.URL.Path == path && r.Method == http.MethodPut:
			putCalls++
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: true})
		case r.URL.Path == path && r.Method == http.MethodDelete:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.DeleteClient(context.Background(), map[string]any{"client": testClientID}); err != nil {
		t.Fatalf("delete client failed: %v", err)
	}
	if putCalls != 0 {
		t.Fatalf("expected no PUT calls on already-archived client, got %d", putCalls)
	}
}

func TestDeleteClientRefusesActiveProjects(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: false})
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.Project{{ID: testProjectID, Name: "Active", ClientID: testClientID, Archived: false}})
		case r.URL.Path == path && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
			t.Fatalf("client with active projects must not be mutated")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.DeleteClient(context.Background(), map[string]any{"client": testClientID})
	if err == nil || !strings.Contains(err.Error(), "active projects") {
		t.Fatalf("expected active-project guard, got %v", err)
	}
}

func TestDeleteClientDryRunDoesNotMutate(t *testing.T) {
	var mutated bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: false})
		case r.URL.Path == path && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
			mutated = true
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteClient(context.Background(), map[string]any{"client": testClientID, "dry_run": true})
	if err != nil {
		t.Fatalf("delete client dry-run failed: %v", err)
	}
	if mutated {
		t.Fatal("dry-run must not issue PUT or DELETE")
	}
	if result.Action != "clockify_clients_delete" {
		t.Fatalf("unexpected action: %q", result.Action)
	}
}

func TestDeleteClientRequiresClientArg(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.DeleteClient(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error when client arg is missing")
	}
}

func TestUpdateClientPreservesExistingFields(t *testing.T) {
	// Existing client has name + address + email; caller only updates
	// the note. PUT body must include name, address, and email
	// verbatim so Clockify's full-replacement semantics do not null
	// them out.
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{
				ID:      testClientID,
				Name:    "Acme",
				Address: "123 Foo St",
				Email:   "ops@acme.example",
			})
		case r.URL.Path == path && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.ClientEntity{
				ID:      testClientID,
				Name:    "Acme",
				Address: "123 Foo St",
				Email:   "ops@acme.example",
				Note:    "new note",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateClient(context.Background(), map[string]any{
		"client": testClientID,
		"note":   "new note",
	})
	if err != nil {
		t.Fatalf("update client failed: %v", err)
	}
	if putBody["name"] != "Acme" {
		t.Fatalf("PUT body must preserve name, got %v", putBody["name"])
	}
	if putBody["address"] != "123 Foo St" {
		t.Fatalf("PUT body must preserve existing address, got %v", putBody["address"])
	}
	if putBody["email"] != "ops@acme.example" {
		t.Fatalf("PUT body must preserve existing email, got %v", putBody["email"])
	}
	if putBody["note"] != "new note" {
		t.Fatalf("PUT body must carry new note, got %v", putBody["note"])
	}

	changed, _ := result.Meta["changedFields"].([]string)
	if len(changed) != 1 || changed[0] != "note" {
		t.Fatalf("expected changedFields=[note], got %v", changed)
	}
}

func TestUpdateClientChangesNameAndAddress(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Old", Address: "Old St"})
		case r.URL.Path == path && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "New", Address: "New St"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.UpdateClient(context.Background(), map[string]any{
		"client":  testClientID,
		"name":    "New",
		"address": "New St",
	}); err != nil {
		t.Fatalf("update client failed: %v", err)
	}
	if putBody["name"] != "New" || putBody["address"] != "New St" {
		t.Fatalf("PUT body must carry new name+address, got %v", putBody)
	}
}

func TestUpdateClientArchivedFlagToggle(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: false})
		case r.URL.Path == path && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Archived: true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.UpdateClient(context.Background(), map[string]any{
		"client":   testClientID,
		"archived": true,
	}); err != nil {
		t.Fatalf("update client failed: %v", err)
	}
	if putBody["archived"] != true {
		t.Fatalf("PUT body must set archived=true, got %v", putBody["archived"])
	}
	if putBody["name"] != "Acme" {
		t.Fatalf("PUT body must preserve name when toggling archived, got %v", putBody["name"])
	}
}

func TestUpdateClientDryRunDoesNotMutate(t *testing.T) {
	var mutated bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := "/workspaces/ws1/clients/" + testClientID
		switch {
		case r.URL.Path == path && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme"})
		case r.URL.Path == path && r.Method == http.MethodPut:
			mutated = true
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateClient(context.Background(), map[string]any{
		"client":  testClientID,
		"name":    "Acme New",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("update client dry-run failed: %v", err)
	}
	if mutated {
		t.Fatal("dry-run must not issue PUT")
	}
	if result.Action != "clockify_clients_update" {
		t.Fatalf("unexpected action: %q", result.Action)
	}
}

func TestUpdateClientRequiresClientArg(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.UpdateClient(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error when client arg is missing")
	}
}

func TestClientViewPreservesClientFields(t *testing.T) {
	view := clientViewFromClient(clockify.ClientEntity{
		ID:           testClientID,
		Name:         "Acme",
		Address:      "123 Foo St",
		Email:        "ops@acme.example",
		Note:         "preferred net30",
		CurrencyCode: "USD",
		CurrencyID:   "cur1",
		WorkspaceID:  "ws1",
	})
	if view.ID != testClientID || view.Name != "Acme" || view.Email != "ops@acme.example" {
		t.Fatalf("top-level client fields not preserved: %#v", view)
	}
	if view.Currency.Code != "USD" || view.Currency.ID != "cur1" {
		t.Fatalf("currency block not mapped: %#v", view.Currency)
	}
	if view.Contact.Address != "123 Foo St" || view.Contact.Note != "preferred net30" {
		t.Fatalf("contact block not mapped: %#v", view.Contact)
	}
	if view.WorkspaceID != "ws1" || view.CurrencyCode != "USD" || view.CurrencyID != "cur1" {
		t.Fatalf("canonical client fields not preserved: %#v", view)
	}
}

// TestClientReportAddsInvoiceDetailEntriesAndHealth was removed with the
// unwired ClientReport composite (see clockify_reports_detailed for the
// shipped client-detail surface).
