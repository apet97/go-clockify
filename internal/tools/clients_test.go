package tools

import (
	"context"
	"encoding/json"
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
	ce, ok := result.Data.(ClientView)
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
	ce, ok := result.Data.(ClientView)
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
	if result.Action != "clockify_delete_client" {
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
	if result.Action != "clockify_update_client" {
		t.Fatalf("unexpected action: %q", result.Action)
	}
}

func TestUpdateClientRequiresClientArg(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.UpdateClient(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error when client arg is missing")
	}
}

func TestCreateClientForwardsAddressEmailNote(t *testing.T) {
	var body map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/clients" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, clockify.ClientEntity{
				ID:      testClientID,
				Name:    "Acme",
				Address: "123 Foo St",
				Email:   "ops@acme.example",
				Note:    "preferred net30",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.CreateClient(context.Background(), map[string]any{
		"name":    "Acme",
		"address": "123 Foo St",
		"email":   "ops@acme.example",
		"note":    "preferred net30",
	}); err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	if body["name"] != "Acme" || body["address"] != "123 Foo St" || body["email"] != "ops@acme.example" || body["note"] != "preferred net30" {
		t.Fatalf("POST body missing fields: %+v", body)
	}
}

func TestCreateClientOmitsBlankOptionalFields(t *testing.T) {
	var body map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/clients" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.CreateClient(context.Background(), map[string]any{
		"name":    "Acme",
		"address": "   ",
		"email":   "",
	}); err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	if _, hasAddr := body["address"]; hasAddr {
		t.Fatalf("whitespace-only address must not be forwarded, got %+v", body)
	}
	if _, hasEmail := body["email"]; hasEmail {
		t.Fatalf("empty email must not be forwarded, got %+v", body)
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
	if view.Raw["id"] != testClientID || view.Raw["currencyCode"] != "USD" {
		t.Fatalf("raw client payload not preserved: %#v", view.Raw)
	}
}

func TestListClientsEnrichesProjectsAndReports(t *testing.T) {
	var projectQuery string
	var reportCalls int
	var reportBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/clients" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.ClientEntity{{ID: "c1", Name: "Client A", CurrencyCode: "USD"}, {ID: "c2", Name: "Client B"}})
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			projectQuery = r.URL.RawQuery
			respondJSON(t, w, []clockify.Project{{
				ID:       "p1",
				Name:     "Project A",
				ClientID: "c1",
				Billable: true,
			}})
		case r.URL.Path == "/workspaces/ws1/reports/summary" && r.Method == http.MethodPost:
			reportCalls++
			if err := json.NewDecoder(r.Body).Decode(&reportBody); err != nil {
				t.Fatalf("decode report body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"groupOne": []map[string]any{{
					"clientId":     "c1",
					"duration":     7200,
					"entriesCount": 2,
					"amounts": []map[string]any{
						{"type": "EARNED", "value": 50000, "currency": "USD"},
						{"type": "COST", "value": 12000, "currency": "USD"},
						{"type": "PROFIT", "value": 38000, "currency": "USD"},
					},
				}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	result, err := svc.ListClients(context.Background(), map[string]any{
		"financial_start": "2023-05-13T00:00:00Z",
		"financial_end":   "2026-05-13T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("ListClients: %v", err)
	}
	clients := result.Data.([]ClientView)
	if len(clients) != 2 {
		t.Fatalf("clients length = %d, want 2", len(clients))
	}
	if !strings.Contains(projectQuery, "clients=c1") || !strings.Contains(projectQuery, "clients=c2") ||
		!strings.Contains(projectQuery, "contains-client=true") || !strings.Contains(projectQuery, "client-status=ALL") ||
		!strings.Contains(projectQuery, "hydrated=true") {
		t.Fatalf("project enrichment query missing filters: %s", projectQuery)
	}
	if reportCalls != 1 {
		t.Fatalf("reports calls = %d, want 1", reportCalls)
	}
	filter := reportBody["summaryFilter"].(map[string]any)
	groups := filter["groups"].([]any)
	if len(groups) != 3 || groups[0] != "CLIENT" || groups[1] != "PROJECT" || groups[2] != "TASK" {
		t.Fatalf("summary groups = %#v", groups)
	}
	if clients[0].ProjectSummary.Count != 1 || clients[0].ProjectSummary.BillableCount != 1 {
		t.Fatalf("project summary not mapped: %#v", clients[0].ProjectSummary)
	}
	if clients[0].Financials.Source != entryFinancialSourceReportsAPI || clients[0].Financials.Earned.AmountCents != 50000 {
		t.Fatalf("financials not mapped: %#v", clients[0].Financials)
	}
	if clients[0].TimeSummary.TrackedDurationSeconds != 7200 || clients[0].TimeSummary.EntriesCount != 2 {
		t.Fatalf("time summary not mapped: %#v", clients[0].TimeSummary)
	}
}

func TestClientReportAddsInvoiceDetailEntriesAndHealth(t *testing.T) {
	var invoiceCalled bool
	var detailedCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/clients/"+testClientID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.ClientEntity{ID: testClientID, Name: "Acme", CurrencyCode: "USD"})
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.Project{{ID: "p1", Name: "Project A", ClientID: testClientID, Billable: true}})
		case r.URL.Path == "/workspaces/ws1/projects/p1/tasks" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.Task{})
		case r.URL.Path == "/workspaces/ws1/reports/summary" && r.Method == http.MethodPost:
			respondJSON(t, w, map[string]any{
				"groupOne": []map[string]any{{
					"clientId": testClientID,
					"duration": 3600,
					"amounts":  []map[string]any{{"type": "EARNED", "value": 25000, "currency": "USD"}},
				}},
			})
		case r.URL.Path == "/workspaces/ws1/invoices/info" && r.Method == http.MethodPost:
			invoiceCalled = true
			respondJSON(t, w, map[string]any{
				"total": 1,
				"invoices": []map[string]any{{
					"id":           "inv1",
					"clientId":     testClientID,
					"amount":       25000,
					"balance":      5000,
					"paid":         20000,
					"currencyCode": "USD",
					"status":       "PARTIALLY_PAID",
					"daysOverdue":  3,
				}},
			})
		case r.URL.Path == "/workspaces/ws1/reports/detailed" && r.Method == http.MethodPost:
			detailedCalled = true
			respondJSON(t, w, map[string]any{
				"timeentries": []map[string]any{{
					"_id":               "e1",
					"description":       "",
					"clientId":          testClientID,
					"clientName":        "Acme",
					"projectId":         "p1",
					"projectName":       "Project A",
					"locked":            true,
					"duration":          3600,
					"approvalRequestId": "apr1",
					"approvalState":     "PENDING",
					"amounts":           []map[string]any{{"type": "EARNED", "value": 25000, "currency": "USD"}},
				}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	result, err := svc.ClientReport(context.Background(), map[string]any{"client": testClientID})
	if err != nil {
		t.Fatalf("ClientReport: %v", err)
	}
	report := result.Data.(ClientReportView)
	if !invoiceCalled || !detailedCalled {
		t.Fatalf("expected invoice and detailed report enrichment calls, invoice=%v detailed=%v", invoiceCalled, detailedCalled)
	}
	if report.Client.InvoiceSummary.TotalCount != 1 || report.Client.InvoiceSummary.OverdueCount != 1 {
		t.Fatalf("invoice summary not mapped: %#v", report.Client.InvoiceSummary)
	}
	if report.EntryHealth.LockedCount != 1 || report.EntryHealth.MissingDescriptionCount != 1 {
		t.Fatalf("entry health not mapped: %#v", report.EntryHealth)
	}
	if report.Client.ApprovalSummary.States["PENDING"] != 1 || len(report.Entries) != 1 {
		t.Fatalf("approval/detail summary not mapped: approvals=%#v entries=%#v", report.Client.ApprovalSummary, report.Entries)
	}
}
