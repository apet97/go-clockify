package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestTier2_Invoices_FullSweep exercises every handler in the invoices
// Tier 2 group via mocked Clockify API responses. The goal is broad
// coverage of the listInvoices→...→deleteInvoiceItem chain — each handler
// is otherwise unreachable from the existing test surface and contributes
// to the internal/tools coverage gap.
func TestTier2_Invoices_FullSweep(t *testing.T) {
	requests := []struct {
		method string
		path   string
	}{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, struct {
			method string
			path   string
		}{r.Method, r.URL.Path})
		switch {
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/invoices":
			respondJSON(t, w, []map[string]any{{"id": "inv1", "status": "DRAFT"}, {"id": "inv2", "status": "PAID"}})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/invoices/inv1":
			respondJSON(t, w, map[string]any{"id": "inv1", "status": "DRAFT", "currency": "USD"})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/invoices":
			respondJSON(t, w, map[string]any{"id": "inv-new", "clientId": "c1", "status": "DRAFT"})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/invoices/inv1":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			respondJSON(t, w, map[string]any{"id": "inv1", "status": body["status"], "currency": body["currency"]})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/invoices/inv1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/invoices/inv1/send":
			respondJSON(t, w, map[string]any{"status": "SENT"})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/invoices/inv1/items":
			respondJSON(t, w, []map[string]any{{"id": "item1", "description": "Hour"}})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/invoices/inv1/items":
			respondJSON(t, w, map[string]any{"id": "item-new", "description": "New item"})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/invoices/inv1/items/item1":
			respondJSON(t, w, map[string]any{"id": "item1", "description": "Updated"})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/invoices/inv1/items/item1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	client, cleanup := newTestClient(t, mux.ServeHTTP)
	defer cleanup()
	svc := New(client, "ws1")
	ctx := context.Background()

	// 1. listInvoices
	res, err := svc.listInvoices(ctx, map[string]any{"page": 2, "page_size": 25})
	mustOK(t, res, err, "clockify_list_invoices")

	// 2. getInvoice — happy
	res, err = svc.getInvoice(ctx, map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_get_invoice")

	// 2b. getInvoice — validation error (empty id)
	if _, err := svc.getInvoice(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 3. createInvoice — happy
	res, err = svc.createInvoice(ctx, map[string]any{
		"client_id": "c1",
		"currency":  "USD",
		"due_date":  "2026-05-01",
		"note":      "Q2 invoice",
	})
	mustOK(t, res, err, "clockify_create_invoice")

	// 3b. createInvoice — validation
	if _, err := svc.createInvoice(ctx, map[string]any{"client_id": ""}); err == nil {
		t.Fatal("expected validation error for empty client_id")
	}

	// 4. updateInvoice — happy
	res, err = svc.updateInvoice(ctx, map[string]any{
		"invoice_id": "inv1",
		"client_id":  "c1",
		"currency":   "EUR",
		"due_date":   "2026-06-01",
		"note":       "updated",
		"status":     "SENT",
	})
	mustOK(t, res, err, "clockify_update_invoice")

	// 4b. updateInvoice — validation
	if _, err := svc.updateInvoice(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 5a. deleteInvoice dry-run — fetches the invoice for preview
	res, err = svc.deleteInvoice(ctx, map[string]any{"invoice_id": "inv1", "dry_run": true})
	mustOK(t, res, err, "clockify_delete_invoice")

	// 5b. deleteInvoice executed
	res, err = svc.deleteInvoice(ctx, map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_delete_invoice")

	// 5c. deleteInvoice validation
	if _, err := svc.deleteInvoice(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 6a. sendInvoice dry-run
	res, err = svc.sendInvoice(ctx, map[string]any{"invoice_id": "inv1", "dry_run": true})
	mustOK(t, res, err, "clockify_send_invoice")

	// 6b. sendInvoice executed
	res, err = svc.sendInvoice(ctx, map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_send_invoice")

	// 6c. sendInvoice validation
	if _, err := svc.sendInvoice(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 7. markInvoicePaid
	res, err = svc.markInvoicePaid(ctx, map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_mark_invoice_paid")

	// 7b. markInvoicePaid validation
	if _, err := svc.markInvoicePaid(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 8. listInvoiceItems
	res, err = svc.listInvoiceItems(ctx, map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_list_invoice_items")

	// 8b. listInvoiceItems validation
	if _, err := svc.listInvoiceItems(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 9. addInvoiceItem
	res, err = svc.addInvoiceItem(ctx, map[string]any{
		"invoice_id":  "inv1",
		"description": "Consulting",
		"quantity":    8,
		"unit_price":  150,
	})
	mustOK(t, res, err, "clockify_add_invoice_item")

	// 9b. addInvoiceItem validation
	if _, err := svc.addInvoiceItem(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 10. updateInvoiceItem
	res, err = svc.updateInvoiceItem(ctx, map[string]any{
		"invoice_id":  "inv1",
		"item_id":     "item1",
		"description": "Updated description",
		"quantity":    10,
		"unit_price":  175,
	})
	mustOK(t, res, err, "clockify_update_invoice_item")

	// 10b. updateInvoiceItem validation — missing item_id
	if _, err := svc.updateInvoiceItem(ctx, map[string]any{"invoice_id": "inv1", "item_id": ""}); err == nil {
		t.Fatal("expected validation error for empty item_id")
	}
	// 10c. validation — missing invoice_id
	if _, err := svc.updateInvoiceItem(ctx, map[string]any{"invoice_id": "", "item_id": "item1"}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 11a. deleteInvoiceItem dry-run
	res, err = svc.deleteInvoiceItem(ctx, map[string]any{
		"invoice_id": "inv1", "item_id": "item1", "dry_run": true,
	})
	mustOK(t, res, err, "clockify_delete_invoice_item")

	// 11b. deleteInvoiceItem executed
	res, err = svc.deleteInvoiceItem(ctx, map[string]any{"invoice_id": "inv1", "item_id": "item1"})
	mustOK(t, res, err, "clockify_delete_invoice_item")

	// 11c. deleteInvoiceItem validation — missing item_id
	if _, err := svc.deleteInvoiceItem(ctx, map[string]any{"invoice_id": "inv1", "item_id": ""}); err == nil {
		t.Fatal("expected validation error for empty item_id")
	}
	// 11d. validation — missing invoice_id
	if _, err := svc.deleteInvoiceItem(ctx, map[string]any{"invoice_id": "", "item_id": "item1"}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// Sanity: at least 13 upstream requests (deleteInvoiceItem dry-run uses
	// MinimalResult and does not hit the network).
	if len(requests) < 13 {
		t.Fatalf("expected at least 13 upstream requests, got %d: %+v", len(requests), requests)
	}
}

// TestTier2_Invoices_GroupRegistration verifies the group is registered
// in the Tier2Groups catalog and the Builder produces all 12 descriptors.
func TestTier2_Invoices_GroupRegistration(t *testing.T) {
	g, ok := Tier2Groups["invoices"]
	if !ok {
		t.Fatal("invoices group not registered")
	}
	if g.Name != "invoices" || g.Builder == nil {
		t.Fatalf("group missing name or builder: %+v", g)
	}
	svc := New(nil, "ws1")
	descs := g.Builder(svc)
	if len(descs) != 12 {
		t.Fatalf("expected 12 invoice tools, got %d", len(descs))
	}
	wantPrefix := "clockify_"
	for _, d := range descs {
		if !strings.HasPrefix(d.Tool.Name, wantPrefix) {
			t.Fatalf("unexpected tool name: %s", d.Tool.Name)
		}
		if d.Handler == nil {
			t.Fatalf("missing handler: %s", d.Tool.Name)
		}
	}
}

// mustOK is a small assertion helper for ResultEnvelope happy-paths.
func mustOK(t *testing.T, res ResultEnvelope, err error, wantAction string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s failed: %v", wantAction, err)
	}
	if !res.OK {
		t.Fatalf("%s ok=false: %+v", wantAction, res)
	}
	if res.Action != wantAction {
		t.Fatalf("%s wrong action: got %s", wantAction, res.Action)
	}
}
