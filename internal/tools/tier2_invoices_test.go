package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
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
			respondJSON(t, w, map[string]any{
				"total": 2,
				"invoices": []map[string]any{
					{"id": "inv1", "status": "UNSENT"},
					{"id": "inv2", "status": "PAID"},
				},
			})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/invoices/inv1":
			respondJSON(t, w, map[string]any{
				"id":         "inv1",
				"clientId":   "c1",
				"number":     "INV-1",
				"issuedDate": "2026-04-01T00:00:00Z",
				"dueDate":    "2026-05-01T00:00:00Z",
				"status":     "UNSENT",
				"currency":   "USD",
				"items":      []map[string]any{{"id": "item1", "description": "Hour"}},
			})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/invoices/inv1/export":
			if got := r.URL.Query().Get("userLocale"); got != "en-US" {
				t.Fatalf("expected export userLocale=en-US, got %q", got)
			}
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF invoice"))
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/invoices":
			respondJSON(t, w, map[string]any{"id": "inv-new", "clientId": "c1", "status": "UNSENT"})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/invoices/inv1":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, ok := body["status"]; ok {
				t.Fatalf("invoice status must not be sent in PUT body: %#v", body)
			}
			respondJSON(t, w, map[string]any{"id": "inv1", "status": "UNSENT", "currency": body["currency"]})
		case r.Method == "PATCH" && r.URL.Path == "/workspaces/ws1/invoices/inv1/status":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["invoiceStatus"] == "" {
				t.Fatalf("expected invoiceStatus patch body, got %#v", body)
			}
			respondJSON(t, w, map[string]any{"ok": true})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/invoices/inv1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/invoices/inv1/send":
			respondJSON(t, w, map[string]any{"status": "SENT"})
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

	// 2b. exportInvoice — defaults user_locale to en-US
	res, err = svc.exportInvoice(ctx, map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_export_invoice")
	raw, ok := res.Data.(map[string]any)
	if !ok || raw["body"] != base64.StdEncoding.EncodeToString([]byte("%PDF invoice")) {
		t.Fatalf("exportInvoice data = %#v", res.Data)
	}

	// 2c. getInvoice — validation error (empty id)
	if _, err := svc.getInvoice(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 3. createInvoice — happy
	res, err = svc.createInvoice(ctx, map[string]any{
		"client_id":   "c1",
		"number":      "INV-NEW",
		"issued_date": "2026-04-01T00:00:00Z",
		"currency":    "USD",
		"due_date":    "2026-05-01T00:00:00Z",
		"note":        "Q2 invoice",
	})
	mustOK(t, res, err, "clockify_create_invoice")

	// 3b. createInvoice — validation
	if _, err := svc.createInvoice(ctx, map[string]any{"client_id": ""}); err == nil {
		t.Fatal("expected validation error for empty client_id")
	}

	// 4. updateInvoice — happy
	res, err = svc.updateInvoice(ctx, map[string]any{
		"invoice_id":  "inv1",
		"client_id":   "c1",
		"number":      "INV-1A",
		"issued_date": "2026-04-02T00:00:00Z",
		"currency":    "EUR",
		"due_date":    "2026-06-01T00:00:00Z",
		"note":        "updated",
		"status":      "SENT",
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

	// 7a. markInvoicePaid dry-run — must GET (preview), not PUT
	beforePaidDryRun := len(requests)
	res, err = svc.markInvoicePaid(ctx, map[string]any{"invoice_id": "inv1", "dry_run": true})
	mustOK(t, res, err, "clockify_mark_invoice_paid")
	for _, r := range requests[beforePaidDryRun:] {
		if r.method == "PUT" {
			t.Fatalf("markInvoicePaid dry-run must not PUT, got %s %s", r.method, r.path)
		}
	}

	// 7b. markInvoicePaid executed
	res, err = svc.markInvoicePaid(ctx, map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_mark_invoice_paid")

	// 7c. markInvoicePaid validation
	if _, err := svc.markInvoicePaid(ctx, map[string]any{"invoice_id": ""}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// 8. listInvoiceItems — reads embedded items from getInvoice
	res, err = svc.listInvoiceItems(ctx, map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_list_invoice_items")
	items, ok := res.Data.([]InvoiceItemView)
	if !ok {
		t.Fatalf("listInvoiceItems data: expected []InvoiceItemView, got %T", res.Data)
	}
	if len(items) != 1 || items[0]["id"] != "item1" {
		t.Fatalf("listInvoiceItems items: expected [{id:item1}], got %#v", items)
	}

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
		"item_type":   "NEW DEFAULT",
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
		"item_type":   "NEW DEFAULT",
	})
	mustOK(t, res, err, "clockify_update_invoice_item")

	// 10b. updateInvoiceItem validation — missing item_index/item_id
	if _, err := svc.updateInvoiceItem(ctx, map[string]any{"invoice_id": "inv1", "item_id": ""}); err == nil {
		t.Fatal("expected validation error for empty item_index/item_id")
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

	// 11c. deleteInvoiceItem validation — missing item_index/item_id
	if _, err := svc.deleteInvoiceItem(ctx, map[string]any{"invoice_id": "inv1", "item_id": ""}); err == nil {
		t.Fatal("expected validation error for empty item_index/item_id")
	}
	// 11d. validation — missing invoice_id
	if _, err := svc.deleteInvoiceItem(ctx, map[string]any{"invoice_id": "", "item_id": "item1"}); err == nil {
		t.Fatal("expected validation error for empty invoice_id")
	}

	// Sanity: at least 15 upstream requests (deleteInvoiceItem dry-run uses
	// MinimalResult and does not hit the network).
	if len(requests) < 15 {
		t.Fatalf("expected at least 15 upstream requests, got %d: %+v", len(requests), requests)
	}
}

// TestTier2_Invoices_GroupRegistration verifies the group is registered
// in the Tier2Groups catalog and the Builder produces all descriptors.
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
	if len(descs) != 16 {
		t.Fatalf("expected 16 invoice tools, got %d", len(descs))
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

// TestTier2_Invoices_ListSendsStatusesNotStatus pins SUMMARY #10:
// when the caller passes `status`, the handler must emit ?statuses=
// (plural) upstream and must NOT emit ?status=. Upstream wire name
// verified by clockify-api-probe-lab 2026-05-02.
func TestTier2_Invoices_ListSendsStatusesNotStatus(t *testing.T) {
	var capturedQuery url.Values
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/invoices" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		capturedQuery = r.URL.Query()
		respondJSON(t, w, map[string]any{
			"total":    1,
			"invoices": []map[string]any{{"id": "inv1", "status": "PAID"}},
		})
	})
	defer cleanup()
	svc := New(client, "ws1")

	res, err := svc.listInvoices(context.Background(), map[string]any{
		"status":    "PAID",
		"page":      1,
		"page_size": 50,
	})
	mustOK(t, res, err, "clockify_list_invoices")

	if got := capturedQuery.Get("statuses"); got != "PAID" {
		t.Fatalf("expected ?statuses=PAID, got %q (full query=%q)", got, capturedQuery.Encode())
	}
	if got := capturedQuery.Get("status"); got != "" {
		t.Fatalf("must not send legacy ?status, got %q (full query=%q)", got, capturedQuery.Encode())
	}
}

func TestMarkInvoicePaidUsesStatusPatchBody(t *testing.T) {
	var gotPatchBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/invoices/inv1":
			respondJSON(t, w, map[string]any{
				"id":         "inv1",
				"clientId":   "client1",
				"number":     "INV-2026-001",
				"issuedDate": "2026-05-01T00:00:00Z",
				"dueDate":    "2026-05-15T00:00:00Z",
				"currency":   "USD",
				"note":       "existing note",
				"status":     "SENT",
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/workspaces/ws1/invoices/inv1/status":
			if err := json.NewDecoder(r.Body).Decode(&gotPatchBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{"ok": true})
		case r.Method == http.MethodPut:
			t.Fatalf("markInvoicePaid must use PATCH status route, not PUT")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.markInvoicePaid(context.Background(), map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_mark_invoice_paid")

	if gotPatchBody["invoiceStatus"] != "PAID" {
		t.Fatalf("expected invoiceStatus=PAID patch body, got %#v", gotPatchBody)
	}
	if _, ok := gotPatchBody["status"]; ok {
		t.Fatalf("status patch body must not use legacy status key: %#v", gotPatchBody)
	}
}

func TestCreateInvoiceUsesCamelCaseBodyKeys(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/invoices" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"id": "inv-new"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.createInvoice(context.Background(), map[string]any{
		"client_id":   "client1",
		"number":      "INV-NEW",
		"issued_date": "2026-04-01T00:00:00Z",
		"currency":    "USD",
		"due_date":    "2026-05-01T00:00:00Z",
		"note":        "Q2 invoice",
	})
	mustOK(t, res, err, "clockify_create_invoice")

	want := map[string]any{
		"clientId":   "client1",
		"number":     "INV-NEW",
		"issuedDate": "2026-04-01T00:00:00Z",
		"currency":   "USD",
		"dueDate":    "2026-05-01T00:00:00Z",
		"note":       "Q2 invoice",
	}
	for key, wantValue := range want {
		if gotBody[key] != wantValue {
			t.Fatalf("expected %s=%v in create invoice body, got %#v", key, wantValue, gotBody)
		}
	}
	for _, legacy := range []string{"client_id", "issued_date", "due_date"} {
		if _, ok := gotBody[legacy]; ok {
			t.Fatalf("body must not include legacy key %q: %#v", legacy, gotBody)
		}
	}
}

func TestUpdateInvoiceUsesCamelCaseBodyKeys(t *testing.T) {
	var gotBody, gotPatchBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/workspaces/ws1/invoices/inv1":
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{"id": "inv1"})
		case r.Method == http.MethodPatch && r.URL.Path == "/workspaces/ws1/invoices/inv1/status":
			if err := json.NewDecoder(r.Body).Decode(&gotPatchBody); err != nil {
				t.Fatalf("decode patch body: %v", err)
			}
			respondJSON(t, w, map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.updateInvoice(context.Background(), map[string]any{
		"invoice_id":  "inv1",
		"client_id":   "client1",
		"number":      "INV-1A",
		"issued_date": "2026-04-02T00:00:00Z",
		"currency":    "EUR",
		"due_date":    "2026-06-01T00:00:00Z",
		"note":        "updated",
		"status":      "SENT",
	})
	mustOK(t, res, err, "clockify_update_invoice")

	want := map[string]any{
		"clientId":   "client1",
		"number":     "INV-1A",
		"issuedDate": "2026-04-02T00:00:00Z",
		"currency":   "EUR",
		"dueDate":    "2026-06-01T00:00:00Z",
		"note":       "updated",
	}
	for key, wantValue := range want {
		if gotBody[key] != wantValue {
			t.Fatalf("expected %s=%v in update invoice body, got %#v", key, wantValue, gotBody)
		}
	}
	for _, legacy := range []string{"client_id", "issued_date", "due_date"} {
		if _, ok := gotBody[legacy]; ok {
			t.Fatalf("body must not include legacy key %q: %#v", legacy, gotBody)
		}
	}
	if _, ok := gotBody["status"]; ok {
		t.Fatalf("status must not be sent in invoice-field PUT body: %#v", gotBody)
	}
	if gotPatchBody["invoiceStatus"] != "SENT" {
		t.Fatalf("expected split status patch invoiceStatus=SENT, got %#v", gotPatchBody)
	}
}

func TestInvoiceCreateAndUpdateDryRunAvoidsMutation(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	create, err := svc.createInvoice(context.Background(), map[string]any{
		"client_id":   "client1",
		"number":      "INV-DRY",
		"issued_date": "2026-04-01T00:00:00Z",
		"due_date":    "2026-05-01T00:00:00Z",
		"dry_run":     true,
	})
	mustOK(t, create, err, "clockify_create_invoice")
	createData, ok := create.Data.(map[string]any)
	if !ok || createData["dry_run"] != true {
		t.Fatalf("create dry-run data = %#v", create.Data)
	}
	createPayload, ok := createData["payload"].(map[string]any)
	if !ok || createPayload["clientId"] != "client1" || createPayload["number"] != "INV-DRY" {
		t.Fatalf("create dry-run payload = %#v", createData["payload"])
	}

	update, err := svc.updateInvoice(context.Background(), map[string]any{
		"invoice_id": "inv1",
		"status":     "SENT",
		"dry_run":    true,
	})
	mustOK(t, update, err, "clockify_update_invoice")
	updateData, ok := update.Data.(map[string]any)
	if !ok || updateData["dry_run"] != true {
		t.Fatalf("update dry-run data = %#v", update.Data)
	}
	updatePayload, ok := updateData["payload"].(map[string]any)
	statusPayload, _ := updatePayload["status_update"].(map[string]any)
	if !ok || statusPayload["invoiceStatus"] != "SENT" {
		t.Fatalf("update dry-run payload = %#v", updateData["payload"])
	}
}

func TestInvoiceStatusValidationRejectsDraft(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("DRAFT validation must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateInvoice(context.Background(), map[string]any{
		"invoice_id": "inv1",
		"status":     "DRAFT",
	})
	if err == nil || !strings.Contains(err.Error(), "use UNSENT") {
		t.Fatalf("expected actionable DRAFT rejection, got %v", err)
	}
}

func TestExportInvoiceDefaultsUserLocale(t *testing.T) {
	var gotQuery url.Values
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/invoices/inv1/export" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF default locale"))
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.exportInvoice(context.Background(), map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_export_invoice")
	if got := gotQuery.Get("userLocale"); got != "en-US" {
		t.Fatalf("expected default userLocale=en-US, got %q", got)
	}
	data, ok := res.Data.(map[string]any)
	if !ok || data["body"] != base64.StdEncoding.EncodeToString([]byte("%PDF default locale")) {
		t.Fatalf("expected base64 raw export envelope, got %#v", res.Data)
	}
}

func TestInvoiceItemBodiesUseCamelCaseAndApplyTaxesDefault(t *testing.T) {
	var addBody, updateBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/invoices/inv1/items":
			if err := json.NewDecoder(r.Body).Decode(&addBody); err != nil {
				t.Fatalf("decode add body: %v", err)
			}
			respondJSON(t, w, map[string]any{"id": "item-new"})
		case r.Method == http.MethodPut && r.URL.Path == "/workspaces/ws1/invoices/inv1/items/item1":
			if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			respondJSON(t, w, map[string]any{"id": "item1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.addInvoiceItem(context.Background(), map[string]any{
		"invoice_id":  "inv1",
		"description": "Consulting",
		"quantity":    8,
		"unit_price":  150,
		"item_type":   "NEW DEFAULT",
	})
	mustOK(t, res, err, "clockify_add_invoice_item")
	if addBody["itemType"] != "NEW DEFAULT" {
		t.Fatalf("expected add body itemType, got %#v", addBody)
	}
	if addBody["applyTaxes"] != "NONE" {
		t.Fatalf("expected add body default applyTaxes=NONE, got %#v", addBody)
	}
	if addBody["unitPrice"] != float64(150) {
		t.Fatalf("expected add body unitPrice=150, got %#v", addBody)
	}

	res, err = svc.updateInvoiceItem(context.Background(), map[string]any{
		"invoice_id":  "inv1",
		"item_id":     "item1",
		"description": "Updated description",
		"quantity":    10,
		"unit_price":  175,
		"item_type":   "NEW DEFAULT",
		"apply_taxes": "TAX1",
	})
	mustOK(t, res, err, "clockify_update_invoice_item")
	if updateBody["itemType"] != "NEW DEFAULT" {
		t.Fatalf("expected update body itemType, got %#v", updateBody)
	}
	if updateBody["applyTaxes"] != "TAX1" {
		t.Fatalf("expected update body explicit applyTaxes=TAX1, got %#v", updateBody)
	}
	if updateBody["unitPrice"] != float64(175) {
		t.Fatalf("expected update body unitPrice=175, got %#v", updateBody)
	}

	for _, body := range []map[string]any{addBody, updateBody} {
		for _, legacy := range []string{"item_type", "apply_taxes", "unit_price"} {
			if _, ok := body[legacy]; ok {
				t.Fatalf("invoice item body must not include legacy key %q: %#v", legacy, body)
			}
		}
	}
}

func TestAddInvoiceItemRejectsMissingItemType(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("add invoice item missing item_type must not reach upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.addInvoiceItem(context.Background(), map[string]any{
		"invoice_id":  "inv1",
		"description": "Consulting",
		"quantity":    8,
		"unit_price":  150,
	})
	if err == nil {
		t.Fatal("expected missing item_type error")
	}
}

func TestUpdateInvoiceItemRejectsMissingItemType(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("update invoice item missing item_type must not reach upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateInvoiceItem(context.Background(), map[string]any{
		"invoice_id":  "inv1",
		"item_id":     "item1",
		"description": "Updated description",
		"quantity":    10,
		"unit_price":  175,
	})
	if err == nil {
		t.Fatal("expected missing item_type error")
	}
}

func TestInvoiceItemIndexPrimaryFieldAndLegacyItemIDAlias(t *testing.T) {
	seen := map[string]bool{}
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.Path] = true
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/workspaces/ws1/invoices/inv1/items/2":
			respondJSON(t, w, map[string]any{"id": "line-2"})
		case r.Method == http.MethodDelete && r.URL.Path == "/workspaces/ws1/invoices/inv1/items/3":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/workspaces/ws1/invoices/inv1/items/legacy-4":
			respondJSON(t, w, map[string]any{"id": "legacy-4"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.updateInvoiceItem(context.Background(), map[string]any{
		"invoice_id":  "inv1",
		"item_index":  "2",
		"description": "Updated description",
		"item_type":   "NEW DEFAULT",
	})
	mustOK(t, res, err, "clockify_update_invoice_item")
	if res.Meta["itemIndex"] != "2" || res.Meta["itemId"] != "2" {
		t.Fatalf("expected itemIndex/itemId meta aliases for line 2, got %#v", res.Meta)
	}

	res, err = svc.deleteInvoiceItem(context.Background(), map[string]any{
		"invoice_id": "inv1",
		"item_index": "3",
	})
	mustOK(t, res, err, "clockify_delete_invoice_item")
	data, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", res.Data)
	}
	if data["itemIndex"] != "3" || data["itemId"] != "3" {
		t.Fatalf("expected itemIndex/itemId data aliases for line 3, got %#v", data)
	}

	res, err = svc.updateInvoiceItem(context.Background(), map[string]any{
		"invoice_id":  "inv1",
		"item_id":     "legacy-4",
		"description": "Legacy alias",
		"item_type":   "NEW DEFAULT",
	})
	mustOK(t, res, err, "clockify_update_invoice_item")

	for _, want := range []string{
		"PUT /workspaces/ws1/invoices/inv1/items/2",
		"DELETE /workspaces/ws1/invoices/inv1/items/3",
		"PUT /workspaces/ws1/invoices/inv1/items/legacy-4",
	} {
		if !seen[want] {
			t.Fatalf("expected request %s, saw %#v", want, seen)
		}
	}
}

func TestInvoiceItemIndexRequiredBeforeUpstream(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("missing invoice item index must not reach upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.updateInvoiceItem(context.Background(), map[string]any{
		"invoice_id": "inv1",
		"item_type":  "NEW DEFAULT",
	}); err == nil {
		t.Fatal("expected missing item_index error for update")
	}
	if _, err := svc.deleteInvoiceItem(context.Background(), map[string]any{"invoice_id": "inv1"}); err == nil {
		t.Fatal("expected missing item_index error for delete")
	}
}

func TestListInvoiceItemsReadsEmbeddedInvoiceItems(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/invoices/inv1" {
			t.Fatalf("list invoice items must read invoice, not /items route; got %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{
			"id":    "inv1",
			"items": []map[string]any{{"id": "item1", "description": "Hour"}},
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.listInvoiceItems(context.Background(), map[string]any{"invoice_id": "inv1"})
	mustOK(t, res, err, "clockify_list_invoice_items")
	items, ok := res.Data.([]InvoiceItemView)
	if !ok {
		t.Fatalf("expected []InvoiceItemView, got %T", res.Data)
	}
	if len(items) != 1 || items[0]["id"] != "item1" {
		t.Fatalf("expected embedded invoice item item1, got %#v", items)
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
