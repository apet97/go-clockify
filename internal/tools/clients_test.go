package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

// testClientID is a 24-char hex value that the resolver treats as a
// Clockify ObjectID — i.e. resolution short-circuits straight to the
// GET-by-ID path without a list lookup.
const testClientID = "6a00f2542568d3d29305e74d"

func TestGetClientByID(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/clients/"+testClientID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", Email: "ops@acme.example"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.GetClient(context.Background(), testClientID)
	if err != nil {
		t.Fatalf("get client failed: %v", err)
	}
	ce, ok := result.Data.(clockify.ClientEntity)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if ce.ID != testClientID || ce.Name != "Acme" {
		t.Fatalf("unexpected client: %+v", ce)
	}
	if ce.Email != "ops@acme.example" {
		t.Fatalf("expected email preserved, got %q", ce.Email)
	}
	if result.Meta["workspaceId"] != "ws1" || result.Meta["clientId"] != testClientID {
		t.Fatalf("unexpected meta: %+v", result.Meta)
	}
}

func TestGetClientByExactName(t *testing.T) {
	var listCalls int
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/clients" && r.Method == http.MethodGet:
			listCalls++
			respondJSON(t, w, []map[string]any{
				{"id": "c-alpha", "name": "Alpha"},
				{"id": "c-beta", "name": "Beta"},
			})
		case r.URL.Path == "/workspaces/ws1/clients/c-beta" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: "c-beta", Name: "Beta"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.GetClient(context.Background(), "Beta")
	if err != nil {
		t.Fatalf("get client by name failed: %v", err)
	}
	if listCalls == 0 {
		t.Fatal("expected name resolution to query the list endpoint")
	}
	ce, ok := result.Data.(clockify.ClientEntity)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if ce.ID != "c-beta" {
		t.Fatalf("expected resolved id c-beta, got %q", ce.ID)
	}
}

func TestGetClientRequiresClientRef(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.GetClient(context.Background(), ""); err == nil {
		t.Fatal("expected error when client ref is empty")
	} else if !strings.Contains(err.Error(), "client") {
		t.Fatalf("expected error to mention client, got %v", err)
	}
}
