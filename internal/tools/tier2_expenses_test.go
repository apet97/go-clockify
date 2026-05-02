package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestTier2_Expenses_FullSweep covers the complete expenses Tier 2
// surface — expense CRUD plus expense-category CRUD — through a mocked
// Clockify HTTP server. Mirrors the invoices sweep so coverage stays
// consistent across the two domain modules.
func TestTier2_Expenses_FullSweep(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		// /user is hit by the createExpense handler when user_id is
		// not supplied: it resolves the calling user via getCurrentUser.
		case r.Method == "GET" && r.URL.Path == "/user":
			respondJSON(t, w, map[string]any{"id": "u1", "name": "Tester", "email": "t@example.com"})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/expenses":
			respondJSON(t, w, map[string]any{
				"expenses": map[string]any{
					"expenses": []map[string]any{{"id": "exp1", "amount": 100}},
					"count":    1,
				},
			})
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/expenses/exp1":
			respondJSON(t, w, map[string]any{"id": "exp1", "amount": 100})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/expenses":
			// The handler now POSTs multipart/form-data. Pin the
			// content-type and the required form fields here so a
			// regression to JSON surfaces locally.
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				t.Fatalf("create_expense expected multipart/form-data, got %q", ct)
			}
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("create_expense parse multipart: %v", err)
			}
			for _, field := range []string{"userId", "amount", "date", "categoryId"} {
				if r.FormValue(field) == "" {
					t.Fatalf("create_expense missing required field %q (form=%v)", field, r.Form)
				}
			}
			respondJSON(t, w, map[string]any{"id": "exp-new", "amount": 200})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/expenses/exp1":
			respondJSON(t, w, map[string]any{"id": "exp1", "amount": 250})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/expenses/exp1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/expenses/categories":
			respondJSON(t, w, map[string]any{
				"count":      1,
				"categories": []map[string]any{{"id": "cat1", "name": "Travel"}},
			})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/expenses/categories":
			respondJSON(t, w, map[string]any{"id": "cat-new", "name": "Software"})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/expenses/categories/cat1":
			respondJSON(t, w, map[string]any{"id": "cat1", "name": "Updated"})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/expenses/categories/cat1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	client, cleanup := newTestClient(t, mux.ServeHTTP)
	defer cleanup()
	svc := New(client, "ws1")
	ctx := context.Background()

	// listExpenses
	res, err := svc.listExpenses(ctx, map[string]any{"page": 1, "page_size": 50})
	mustOK(t, res, err, "clockify_list_expenses")

	// getExpense + validation
	res, err = svc.getExpense(ctx, map[string]any{"expense_id": "exp1"})
	mustOK(t, res, err, "clockify_get_expense")
	if _, err := svc.getExpense(ctx, map[string]any{"expense_id": ""}); err == nil {
		t.Fatal("expected validation error for empty expense_id")
	}

	// createExpense — happy + validation (missing amount, then missing date)
	res, err = svc.createExpense(ctx, map[string]any{
		"amount":      150.0,
		"date":        "2026-04-11",
		"category_id": "cat1",
		"project_id":  "p1",
		"description": "Lunch",
	})
	mustOK(t, res, err, "clockify_create_expense")
	if _, err := svc.createExpense(ctx, map[string]any{"date": "2026-04-11"}); err == nil {
		t.Fatal("expected error for missing amount")
	}
	if _, err := svc.createExpense(ctx, map[string]any{"amount": 1.0}); err == nil {
		t.Fatal("expected error for missing date")
	}

	// updateExpense — every optional field set
	res, err = svc.updateExpense(ctx, map[string]any{
		"expense_id":  "exp1",
		"amount":      250.0,
		"date":        "2026-04-12",
		"category_id": "cat1",
		"project_id":  "p2",
		"description": "Dinner",
	})
	mustOK(t, res, err, "clockify_update_expense")
	if _, err := svc.updateExpense(ctx, map[string]any{"expense_id": ""}); err == nil {
		t.Fatal("expected validation error for empty expense_id")
	}

	// deleteExpense — dry-run, executed, validation
	res, err = svc.deleteExpense(ctx, map[string]any{"expense_id": "exp1", "dry_run": true})
	mustOK(t, res, err, "clockify_delete_expense")
	res, err = svc.deleteExpense(ctx, map[string]any{"expense_id": "exp1"})
	mustOK(t, res, err, "clockify_delete_expense")
	if _, err := svc.deleteExpense(ctx, map[string]any{"expense_id": ""}); err == nil {
		t.Fatal("expected validation error for empty expense_id")
	}

	// listExpenseCategories
	res, err = svc.listExpenseCategories(ctx, nil)
	mustOK(t, res, err, "clockify_list_expense_categories")

	// createExpenseCategory + missing-name validation
	res, err = svc.createExpenseCategory(ctx, map[string]any{"name": "Software"})
	mustOK(t, res, err, "clockify_create_expense_category")
	if _, err := svc.createExpenseCategory(ctx, map[string]any{"name": ""}); err == nil {
		t.Fatal("expected error for missing category name")
	}

	// updateExpenseCategory + validation
	res, err = svc.updateExpenseCategory(ctx, map[string]any{"category_id": "cat1", "name": "Updated"})
	mustOK(t, res, err, "clockify_update_expense_category")
	if _, err := svc.updateExpenseCategory(ctx, map[string]any{"category_id": ""}); err == nil {
		t.Fatal("expected validation error for empty category_id")
	}

	// deleteExpenseCategory — dry-run, executed, validation
	res, err = svc.deleteExpenseCategory(ctx, map[string]any{"category_id": "cat1", "dry_run": true})
	mustOK(t, res, err, "clockify_delete_expense_category")
	res, err = svc.deleteExpenseCategory(ctx, map[string]any{"category_id": "cat1"})
	mustOK(t, res, err, "clockify_delete_expense_category")
	if _, err := svc.deleteExpenseCategory(ctx, map[string]any{"category_id": ""}); err == nil {
		t.Fatal("expected validation error for empty category_id")
	}
}

// TestTier2_Expenses_GroupRegistration sanity-checks the catalog entry.
func TestTier2_Expenses_GroupRegistration(t *testing.T) {
	g, ok := Tier2Groups["expenses"]
	if !ok {
		t.Fatal("expenses group not registered")
	}
	if g.Builder == nil {
		t.Fatal("expenses Builder is nil")
	}
	descs := g.Builder(New(nil, "ws1"))
	if len(descs) < 9 {
		t.Fatalf("expected at least 9 expense tools, got %d", len(descs))
	}
}
