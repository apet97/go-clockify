package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
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
	if len(descs) != 16 {
		t.Fatalf("expected 16 invoice tools, got %d", len(descs))
	}

	names := map[string]bool{}
	for _, d := range descs {
		names[d.Tool.Name] = true
	}
	for _, want := range []string{
		"clockify_list_invoices",
		"clockify_get_invoice",
		"clockify_export_invoice",
		"clockify_create_invoice",
		"clockify_update_invoice",
		"clockify_delete_invoice",
		"clockify_send_invoice",
		"clockify_mark_invoice_paid",
		"clockify_get_invoice_settings",
		"clockify_list_invoice_payments",
		"clockify_list_invoice_items",
		"clockify_unbilled_for_client",
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
	if len(descs) != 11 {
		t.Fatalf("expected 11 expense tools, got %d", len(descs))
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
		"clockify_archive_expense_category",
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
			respondJSON(t, w, map[string]any{
				"total": 2,
				"invoices": []map[string]any{
					{"id": "inv1", "clientId": "c1", "status": "DRAFT", "amount": 150.0},
					{"id": "inv2", "clientId": "c2", "status": "SENT", "amount": 300.0},
				},
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
	items, ok := result.Data.([]InvoiceView)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 invoices, got %d", len(items))
	}
	if result.Meta["count"] != 2 {
		t.Fatalf("expected meta count=2, got %v", result.Meta["count"])
	}
	if result.Meta["total"] != 2 {
		t.Fatalf("expected meta total=2, got %v", result.Meta["total"])
	}
}

func TestCreateExpense(t *testing.T) {
	var gotForm map[string][]string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/expenses" && r.Method == http.MethodPost:
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				t.Fatalf("expected multipart/form-data, got %q", ct)
			}
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			gotForm = r.MultipartForm.Value
			respondJSON(t, w, map[string]any{
				"id":         "exp1",
				"amount":     gotForm["amount"][0],
				"date":       gotForm["date"][0],
				"categoryId": gotForm["categoryId"][0],
				"projectId":  gotForm["projectId"][0],
				"notes":      gotForm["notes"][0],
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	// user_id is supplied explicitly so the handler doesn't try to
	// resolve the calling user via GET /user.
	result, err := svc.createExpense(context.Background(), map[string]any{
		"user_id":     "u1",
		"amount":      42.50,
		"date":        "2026-04-01T00:00:00Z",
		"category_id": "cat1",
		"project_id":  "p1",
		"notes":       "Office supplies",
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

	// Verify the multipart form payload (each value lands as []string).
	if gotForm == nil {
		t.Fatal("expected multipart form values")
	}
	if v := gotForm["userId"]; len(v) == 0 || v[0] != "u1" {
		t.Fatalf("expected userId=u1, got %v", v)
	}
	if v := gotForm["amount"]; len(v) == 0 || v[0] != "42.5" {
		t.Fatalf("expected amount=42.5, got %v", v)
	}
	if v := gotForm["date"]; len(v) == 0 || v[0] != "2026-04-01T00:00:00Z" {
		t.Fatalf("expected date=2026-04-01T00:00:00Z, got %v", v)
	}
	if v := gotForm["categoryId"]; len(v) == 0 || v[0] != "cat1" {
		t.Fatalf("expected categoryId=cat1, got %v", v)
	}
	if v := gotForm["projectId"]; len(v) == 0 || v[0] != "p1" {
		t.Fatalf("expected projectId=p1, got %v", v)
	}
	if v := gotForm["notes"]; len(v) == 0 || v[0] != "Office supplies" {
		t.Fatalf("expected notes='Office supplies', got %v", v)
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
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/invoices/info" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"total": 3,
				"invoices": []map[string]any{
					{"id": "inv1", "status": "PAID", "amount": 200.0},
					{"id": "inv2", "status": "PAID", "amount": 350.0},
					{"id": "inv3", "status": "DRAFT", "amount": 100.0},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.invoiceReport(context.Background(), map[string]any{"status": "PAID"})
	if err != nil {
		t.Fatalf("invoice report failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	statuses, ok := gotBody["statuses"].([]any)
	if !ok || len(statuses) != 1 || statuses[0] != "PAID" {
		t.Fatalf("expected body statuses=[\"PAID\"], got %v", gotBody["statuses"])
	}
	data, ok := result.Data.(InvoiceReportView)
	if !ok {
		t.Fatalf("expected InvoiceReportView data, got %T", result.Data)
	}
	if data.TotalAmount == nil || data.TotalAmount.AmountCents != 650 {
		t.Fatalf("expected totalAmount=650 cents, got %#v", data.TotalAmount)
	}
	if data.PageTotalAmount == nil || data.PageTotalAmount.AmountCents != 650 {
		t.Fatalf("expected pageTotalAmount=650 cents, got %#v", data.PageTotalAmount)
	}
	if data.AggregationScope != "page" {
		t.Fatalf("expected aggregationScope=page, got %v", data.AggregationScope)
	}
	statusCounts := data.StatusCounts
	pageStatusCounts := data.PageStatusCounts
	if statusCounts["PAID"] != 2 {
		t.Fatalf("expected 2 PAID, got %d", statusCounts["PAID"])
	}
	if statusCounts["DRAFT"] != 1 {
		t.Fatalf("expected 1 DRAFT, got %d", statusCounts["DRAFT"])
	}
	if pageStatusCounts["PAID"] != 2 || pageStatusCounts["DRAFT"] != 1 {
		t.Fatalf("expected pageStatusCounts PAID=2 DRAFT=1, got %v", pageStatusCounts)
	}
	if result.Meta["total"] != 3 {
		t.Fatalf("expected meta total=3, got %v", result.Meta["total"])
	}
	if result.Meta["pageSize"] != 50 {
		t.Fatalf("expected meta pageSize=50, got %v", result.Meta["pageSize"])
	}
	if result.Meta["aggregationScope"] != "page" {
		t.Fatalf("expected meta aggregationScope=page, got %v", result.Meta["aggregationScope"])
	}
}

func TestExpenseReport(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/reports/expenses/detailed" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode expense report body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"expenses": []map[string]any{
					{"id": "e1", "amount": 50.0, "categoryId": "cat1"},
					{"id": "e2", "amount": 75.0, "categoryId": "cat1"},
					{"id": "e3", "amount": 120.0, "categoryId": "cat2"},
				},
				"totals": map[string]any{
					"expensesCount":       3,
					"totalAmount":         245.0,
					"totalAmountBillable": 125.0,
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.expenseReport(context.Background(), map[string]any{
		"start":       "2026-04-01T00:00:00.000",
		"end":         "2026-04-30T23:59:59.999",
		"page":        1,
		"page_size":   50,
		"sort_column": "AMOUNT",
		"sort_order":  "DESCENDING",
		"categories": map[string]any{
			"contains": "CONTAINS",
			"ids":      []any{"cat1"},
			"status":   "ACTIVE",
		},
	})
	if err != nil {
		t.Fatalf("expense report failed: %v", err)
	}
	if gotBody["dateRangeStart"] != "2026-04-01T00:00:00.000" || gotBody["dateRangeEnd"] != "2026-04-30T23:59:59.999" {
		t.Fatalf("date aliases not forwarded as dateRangeStart/dateRangeEnd: %#v", gotBody)
	}
	if gotBody["dateRangeType"] != "ABSOLUTE" || gotBody["exportType"] != "JSON" {
		t.Fatalf("unexpected report defaults: %#v", gotBody)
	}
	if gotBody["pageSize"] != float64(50) || gotBody["sortColumn"] != "AMOUNT" || gotBody["sortOrder"] != "DESCENDING" {
		t.Fatalf("expense report paging/sort not normalized: %#v", gotBody)
	}
	categories, _ := gotBody["categories"].(map[string]any)
	if categories["contains"] != "CONTAINS" || categories["status"] != "ACTIVE" {
		t.Fatalf("expense categories filter not normalized: %#v", categories)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	if _, ok := data["expenses"]; !ok {
		t.Fatalf("expected upstream expenses in data, got %#v", data)
	}
	if _, ok := data["totals"]; !ok {
		t.Fatalf("expected upstream totals in data, got %#v", data)
	}
	if result.Meta["source"] != "reports-api" {
		t.Fatalf("source meta = %v, want reports-api", result.Meta["source"])
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

func TestCreateExpenseMinorUnitAmountScalesToMajorUnits(t *testing.T) {
	var gotAmount string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/expenses" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		gotAmount = r.FormValue("amount")
		respondJSON(t, w, map[string]any{"id": "exp1", "amount": 12500})
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.createExpense(context.Background(), map[string]any{
		"user_id":     "u1",
		"amount":      12500.0,
		"amount_unit": "minor",
		"date":        "2026-04-01T00:00:00Z",
		"category_id": "cat1",
	})
	if err != nil {
		t.Fatalf("create expense minor amount failed: %v", err)
	}
	if gotAmount != "125" {
		t.Fatalf("expected minor-unit amount 12500 to send amount=125, got %q", gotAmount)
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
