package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestInvoiceHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs, ok := svc.Tier2Handlers("invoices")
	if !ok {
		t.Fatal("invoices group not registered")
	}
	if len(descs) != 12 {
		t.Fatalf("expected 12 invoice tools, got %d", len(descs))
	}

	names := map[string]bool{}
	for _, d := range descs {
		names[d.Tool.Name] = true
	}
	for _, want := range []string{
		"clockify_list_invoices",
		"clockify_get_invoice",
		"clockify_create_invoice",
		"clockify_update_invoice",
		"clockify_delete_invoice",
		"clockify_send_invoice",
		"clockify_mark_invoice_paid",
		"clockify_list_invoice_items",
		"clockify_add_invoice_item",
		"clockify_update_invoice_item",
		"clockify_delete_invoice_item",
		"clockify_invoice_report",
	} {
		if !names[want] {
			t.Fatalf("missing invoice tool: %s", want)
		}
	}
}

func TestExpenseHandlersCount(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	descs, ok := svc.Tier2Handlers("expenses")
	if !ok {
		t.Fatal("expenses group not registered")
	}
	if len(descs) != 10 {
		t.Fatalf("expected 10 expense tools, got %d", len(descs))
	}

	names := map[string]bool{}
	for _, d := range descs {
		names[d.Tool.Name] = true
	}
	for _, want := range []string{
		"clockify_list_expenses",
		"clockify_get_expense",
		"clockify_create_expense",
		"clockify_update_expense",
		"clockify_delete_expense",
		"clockify_list_expense_categories",
		"clockify_create_expense_category",
		"clockify_update_expense_category",
		"clockify_delete_expense_category",
		"clockify_expense_report",
	} {
		if !names[want] {
			t.Fatalf("missing expense tool: %s", want)
		}
	}
}

func TestListInvoices(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/invoices" && r.Method == http.MethodGet:
			if got := r.URL.Query().Get("page"); got != "1" {
				t.Fatalf("expected page=1, got %s", got)
			}
			if got := r.URL.Query().Get("page-size"); got != "50" {
				t.Fatalf("expected page-size=50, got %s", got)
			}
			respondJSON(t, w, []map[string]any{
				{"id": "inv1", "clientId": "c1", "status": "DRAFT", "amount": 150.0},
				{"id": "inv2", "clientId": "c2", "status": "SENT", "amount": 300.0},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.listInvoices(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("list invoices failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	if result.Action != "clockify_list_invoices" {
		t.Fatalf("expected action clockify_list_invoices, got %s", result.Action)
	}
	items, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 invoices, got %d", len(items))
	}
	if result.Meta["count"] != 2 {
		t.Fatalf("expected meta count=2, got %v", result.Meta["count"])
	}
}

func TestCreateExpense(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/expenses" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"id":          "exp1",
				"amount":      gotBody["amount"],
				"date":        gotBody["date"],
				"categoryId":  gotBody["categoryId"],
				"projectId":   gotBody["projectId"],
				"description": gotBody["description"],
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.createExpense(context.Background(), map[string]any{
		"amount":      42.50,
		"date":        "2026-04-01",
		"category_id": "cat1",
		"project_id":  "p1",
		"description": "Office supplies",
	})
	if err != nil {
		t.Fatalf("create expense failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	if result.Action != "clockify_create_expense" {
		t.Fatalf("expected action clockify_create_expense, got %s", result.Action)
	}

	// Verify the POST payload
	if gotBody == nil {
		t.Fatal("expected POST body")
	}
	if gotBody["amount"] != 42.50 {
		t.Fatalf("expected amount 42.50, got %v", gotBody["amount"])
	}
	if gotBody["date"] != "2026-04-01" {
		t.Fatalf("expected date 2026-04-01, got %v", gotBody["date"])
	}
	if gotBody["categoryId"] != "cat1" {
		t.Fatalf("expected categoryId cat1, got %v", gotBody["categoryId"])
	}
	if gotBody["projectId"] != "p1" {
		t.Fatalf("expected projectId p1, got %v", gotBody["projectId"])
	}
	if gotBody["description"] != "Office supplies" {
		t.Fatalf("expected description 'Office supplies', got %v", gotBody["description"])
	}
}

func TestDeleteInvoiceDryRun(t *testing.T) {
	var deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/invoices/inv123" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":       "inv123",
				"clientId": "c1",
				"status":   "DRAFT",
				"amount":   500.0,
			})
		case r.URL.Path == "/workspaces/ws1/invoices/inv123" && r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.deleteInvoice(context.Background(), map[string]any{
		"invoice_id": "inv123",
		"dry_run":    true,
	})
	if err != nil {
		t.Fatalf("delete invoice dry run failed: %v", err)
	}
	if result.Action != "clockify_delete_invoice" {
		t.Fatalf("expected action clockify_delete_invoice, got %s", result.Action)
	}
	if deleteCalled {
		t.Fatal("DELETE should NOT be called during dry run")
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data for dry run, got %T", result.Data)
	}
	if dataMap["dry_run"] != true {
		t.Fatal("expected dry_run=true in result data")
	}
	if dataMap["note"] == nil {
		t.Fatal("expected note in dry run result")
	}
}

func TestDeleteExpenseDryRun(t *testing.T) {
	var deleteCalled bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/expenses/exp456" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"id":     "exp456",
				"amount": 100.0,
				"date":   "2026-03-15",
			})
		case r.URL.Path == "/workspaces/ws1/expenses/exp456" && r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.deleteExpense(context.Background(), map[string]any{
		"expense_id": "exp456",
		"dry_run":    true,
	})
	if err != nil {
		t.Fatalf("delete expense dry run failed: %v", err)
	}
	if deleteCalled {
		t.Fatal("DELETE should NOT be called during dry run")
	}
	dataMap, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data for dry run, got %T", result.Data)
	}
	if dataMap["dry_run"] != true {
		t.Fatal("expected dry_run=true in result data")
	}
}

func TestInvoiceReport(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/invoices" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{
				{"id": "inv1", "status": "PAID", "amount": 200.0},
				{"id": "inv2", "status": "PAID", "amount": 350.0},
				{"id": "inv3", "status": "DRAFT", "amount": 100.0},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.invoiceReport(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("invoice report failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if data["totalAmount"] != 650.0 {
		t.Fatalf("expected totalAmount=650, got %v", data["totalAmount"])
	}
	statusCounts, ok := data["statusCounts"].(map[string]int)
	if !ok {
		t.Fatalf("expected statusCounts map, got %T", data["statusCounts"])
	}
	if statusCounts["PAID"] != 2 {
		t.Fatalf("expected 2 PAID, got %d", statusCounts["PAID"])
	}
	if statusCounts["DRAFT"] != 1 {
		t.Fatalf("expected 1 DRAFT, got %d", statusCounts["DRAFT"])
	}
}

func TestExpenseReport(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/expenses" && r.Method == http.MethodGet:
			respondJSON(t, w, []map[string]any{
				{"id": "e1", "amount": 50.0, "categoryId": "cat1"},
				{"id": "e2", "amount": 75.0, "categoryId": "cat1"},
				{"id": "e3", "amount": 120.0, "categoryId": "cat2"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.expenseReport(context.Background(), map[string]any{
		"start": "2026-04-01",
		"end":   "2026-04-30",
	})
	if err != nil {
		t.Fatalf("expense report failed: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if data["totalAmount"] != 245.0 {
		t.Fatalf("expected totalAmount=245, got %v", data["totalAmount"])
	}
	byCategory, ok := data["byCategory"].(map[string]float64)
	if !ok {
		t.Fatalf("expected byCategory map, got %T", data["byCategory"])
	}
	if byCategory["cat1"] != 125.0 {
		t.Fatalf("expected cat1=125, got %v", byCategory["cat1"])
	}
}

func TestTier2CatalogRegistration(t *testing.T) {
	if _, ok := Tier2Groups["invoices"]; !ok {
		t.Fatal("invoices group not registered in Tier2Groups")
	}
	if _, ok := Tier2Groups["expenses"]; !ok {
		t.Fatal("expenses group not registered in Tier2Groups")
	}
	if g := Tier2Groups["invoices"]; len(g.Keywords) == 0 {
		t.Fatal("invoices group should have keywords")
	}
	if g := Tier2Groups["expenses"]; len(g.Keywords) == 0 {
		t.Fatal("expenses group should have keywords")
	}
}

func TestCreateExpenseMissingAmount(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected")
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.createExpense(context.Background(), map[string]any{
		"date": "2026-04-01",
	})
	if err == nil {
		t.Fatal("expected error for missing amount")
	}
}

func TestCreateExpenseMissingDate(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected")
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.createExpense(context.Background(), map[string]any{
		"amount": 42.50,
	})
	if err == nil {
		t.Fatal("expected error for missing date")
	}
}
