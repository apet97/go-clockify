package tools

import (
	"context"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// TestTier2_Expenses_FullSweep covers the complete expenses domain
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
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				t.Fatalf("update_expense expected multipart/form-data, got %q", ct)
			}
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("update_expense parse multipart: %v", err)
			}
			cf := r.MultipartForm.Value["changeFields"]
			if len(cf) == 0 {
				t.Fatalf("update_expense missing changeFields")
			}
			respondJSON(t, w, map[string]any{"id": "exp1", "amount": 250})
		case r.Method == "DELETE" && r.URL.Path == "/workspaces/ws1/expenses/exp1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "GET" && r.URL.Path == "/workspaces/ws1/expenses/categories":
			respondJSON(t, w, map[string]any{
				"count":      1,
				"categories": []map[string]any{{"id": "cat1", "name": "Travel"}},
			})
		case r.Method == "POST" && r.URL.Path == "/workspaces/ws1/expenses/categories":
			if !strings.Contains(readBody(t, r), `"priceInCents":1250`) {
				t.Fatalf("create expense category body missing priceInCents")
			}
			respondJSON(t, w, map[string]any{"id": "cat-new", "name": "Software", "priceInCents": 1250})
		case r.Method == "PUT" && r.URL.Path == "/workspaces/ws1/expenses/categories/cat1":
			body := readBody(t, r)
			if !strings.Contains(body, `"hasUnitPrice":true`) || !strings.Contains(body, `"unit":"mile"`) {
				t.Fatalf("update expense category body missing unit-price fields: %s", body)
			}
			respondJSON(t, w, map[string]any{"id": "cat1", "name": "Updated", "hasUnitPrice": true, "unit": "mile"})
		case r.Method == "PATCH" && r.URL.Path == "/workspaces/ws1/expenses/categories/cat1/status":
			body := readBody(t, r)
			if !strings.Contains(body, `"archived":true`) {
				t.Fatalf("archive expense category body missing archived=true: %s", body)
			}
			respondJSON(t, w, map[string]any{"id": "cat1", "name": "Updated", "archived": true})
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

	// updateExpense — every optional field set + change_fields
	res, err = svc.updateExpense(ctx, map[string]any{
		"expense_id":    "exp1",
		"change_fields": []any{"AMOUNT", "DATE", "CATEGORY", "PROJECT", "NOTES", "BILLABLE"},
		"amount":        250.0,
		"date":          "2026-04-12T00:00:00Z",
		"category_id":   "cat1",
		"project_id":    "p2",
		"notes":         "Dinner",
		"billable":      false,
	})
	mustOK(t, res, err, "clockify_update_expense")
	if _, err := svc.updateExpense(ctx, map[string]any{"expense_id": ""}); err == nil {
		t.Fatal("expected validation error for empty expense_id")
	}
	if _, err := svc.updateExpense(ctx, map[string]any{"expense_id": "exp1"}); err == nil {
		t.Fatal("expected validation error for missing change_fields")
	}
	if _, err := svc.updateExpense(ctx, map[string]any{"expense_id": "exp1", "change_fields": []any{"BOGUS"}}); err == nil {
		t.Fatal("expected validation error for unsupported change_fields token")
	}
	// Drift sentinel: regression to PUT JSON would fail the
	// content-type assertion in the mock above before this line; this
	// extra branch ensures the change_fields enum gate also stays
	// hot — flipping "USER" to "" disables the validator and the
	// upstream silently no-ops the update, which the next assertion
	// would not catch on its own.
	if _, err := svc.updateExpense(ctx, map[string]any{"expense_id": "exp1", "change_fields": []any{"USER"}, "user_id": "u-7"}); err != nil {
		t.Fatalf("change_fields=[USER] with user_id should succeed; got %v", err)
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
	res, err = svc.createExpenseCategory(ctx, map[string]any{"name": "Software", "has_unit_price": true, "price_in_cents": 1250, "unit": "seat"})
	mustOK(t, res, err, "clockify_create_expense_category")
	res, err = svc.createExpenseCategory(ctx, map[string]any{"name": "Software", "dry_run": true})
	mustOK(t, res, err, "clockify_create_expense_category")
	if _, err := svc.createExpenseCategory(ctx, map[string]any{"name": ""}); err == nil {
		t.Fatal("expected error for missing category name")
	}

	// updateExpenseCategory + validation
	res, err = svc.updateExpenseCategory(ctx, map[string]any{"category_id": "cat1", "name": "Updated", "has_unit_price": true, "unit": "mile"})
	mustOK(t, res, err, "clockify_update_expense_category")
	res, err = svc.updateExpenseCategory(ctx, map[string]any{"category_id": "cat1", "name": "Preview", "dry_run": true})
	mustOK(t, res, err, "clockify_update_expense_category")
	if _, err := svc.updateExpenseCategory(ctx, map[string]any{"category_id": ""}); err == nil {
		t.Fatal("expected validation error for empty category_id")
	}

	// archiveExpenseCategory — dry-run, executed, validation
	res, err = svc.archiveExpenseCategory(ctx, map[string]any{"category_id": "cat1", "dry_run": true})
	mustOK(t, res, err, "clockify_archive_expense_category")
	res, err = svc.archiveExpenseCategory(ctx, map[string]any{"category_id": "cat1", "archived": true})
	mustOK(t, res, err, "clockify_archive_expense_category")
	if _, err := svc.archiveExpenseCategory(ctx, map[string]any{"category_id": ""}); err == nil {
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

func TestCreateExpenseContractDefaultsUserAndAllowsNoFileNoProject(t *testing.T) {
	var userHit bool
	var upstreamHit bool
	var gotForm map[string][]string
	var gotFiles map[string][]*multipart.FileHeader

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			userHit = true
			respondJSON(t, w, map[string]any{"id": "u-current", "name": "Tester", "email": "t@example.com"})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/expenses":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("create_expense expected multipart/form-data, got %q", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			gotForm = r.MultipartForm.Value
			gotFiles = r.MultipartForm.File
			respondJSON(t, w, map[string]any{"id": "exp-new", "amount": 12.5})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	for _, tt := range []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "amount", args: map[string]any{"date": "2026-04-11T00:00:00Z", "category_id": "cat1"}, want: "amount is required"},
		{name: "date", args: map[string]any{"amount": 12.5, "category_id": "cat1"}, want: "date is required"},
		{name: "category", args: map[string]any{"amount": 12.5, "date": "2026-04-11T00:00:00Z"}, want: "category_id is required"},
	} {
		upstreamHit = false
		_, err := svc.createExpense(context.Background(), tt.args)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("%s: expected error containing %q, got %v", tt.name, tt.want, err)
		}
		if upstreamHit {
			t.Fatalf("%s: validation hit upstream", tt.name)
		}
	}

	upstreamHit = false
	res, err := svc.createExpense(context.Background(), map[string]any{
		"amount":      12.5,
		"date":        "2026-04-11T00:00:00Z",
		"category_id": "cat1",
		"notes":       "no-file live narrowing",
		"billable":    true,
	})
	mustOK(t, res, err, "clockify_create_expense")
	if !userHit {
		t.Fatal("expected createExpense to resolve omitted user_id via /user")
	}
	for field, want := range map[string]string{
		"userId":     "u-current",
		"amount":     "12.5",
		"date":       "2026-04-11T00:00:00Z",
		"categoryId": "cat1",
		"notes":      "no-file live narrowing",
		"billable":   "true",
	} {
		if got := gotForm[field]; len(got) != 1 || got[0] != want {
			t.Fatalf("expected multipart %s=%q, got form=%v", field, want, gotForm)
		}
	}
	if _, ok := gotForm["projectId"]; ok {
		t.Fatalf("project_id is live-optional and should be omitted when not supplied: form=%v", gotForm)
	}
	if len(gotFiles) != 0 {
		t.Fatalf("file is intentionally not part of the no-file create contract, got files=%v", gotFiles)
	}
}

func TestUpdateExpenseFallsBackToExistingFields(t *testing.T) {
	var gotForm map[string][]string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/expenses/exp1":
			respondJSON(t, w, map[string]any{
				"id":         "exp1",
				"amount":     100.5,
				"date":       "2026-04-11T00:00:00Z",
				"categoryId": "cat-old",
				"billable":   true,
			})
		case r.Method == http.MethodPut && r.URL.Path == "/workspaces/ws1/expenses/exp1":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("update_expense expected multipart/form-data, got %q", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			gotForm = r.MultipartForm.Value
			respondJSON(t, w, map[string]any{"id": "exp1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.updateExpense(context.Background(), map[string]any{
		"expense_id":    "exp1",
		"change_fields": []any{"CATEGORY"},
		"category_id":   "cat-new",
		"user_id":       "u-7",
	})
	mustOK(t, res, err, "clockify_update_expense")
	if gotForm["amount"][0] != "100.5" {
		t.Fatalf("expected fallback amount=100.5, got form=%v", gotForm)
	}
	if gotForm["date"][0] != "2026-04-11T00:00:00Z" {
		t.Fatalf("expected fallback date, got form=%v", gotForm)
	}
	if gotForm["categoryId"][0] != "cat-new" {
		t.Fatalf("expected updated categoryId=cat-new, got form=%v", gotForm)
	}
	if gotForm["billable"][0] != "true" {
		t.Fatalf("expected fallback billable=true, got form=%v", gotForm)
	}
	if gotForm["userId"][0] != "u-7" {
		t.Fatalf("expected explicit userId=u-7, got form=%v", gotForm)
	}
}

func TestUpdateExpenseFallsBackToCurrentUserWhenUserIDOmitted(t *testing.T) {
	userHit := false
	var gotForm map[string][]string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/expenses/exp1":
			respondJSON(t, w, map[string]any{
				"id":         "exp1",
				"amount":     100,
				"date":       "2026-04-11T00:00:00Z",
				"categoryId": "cat1",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			userHit = true
			respondJSON(t, w, map[string]any{"id": "u-current", "name": "Tester", "email": "t@example.com"})
		case r.Method == http.MethodPut && r.URL.Path == "/workspaces/ws1/expenses/exp1":
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			gotForm = r.MultipartForm.Value
			respondJSON(t, w, map[string]any{"id": "exp1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.updateExpense(context.Background(), map[string]any{
		"expense_id":    "exp1",
		"change_fields": []any{"USER"},
	})
	if err == nil || !strings.Contains(err.Error(), "user_id is required") {
		t.Fatalf("expected missing user_id error, got %v", err)
	}
	if userHit {
		t.Fatal("updateExpense must reject change_fields=[USER] without resolving /user")
	}

	res, err := svc.updateExpense(context.Background(), map[string]any{
		"expense_id":    "exp1",
		"change_fields": []any{"CATEGORY"},
		"category_id":   "cat2",
	})
	mustOK(t, res, err, "clockify_update_expense")
	if !userHit {
		t.Fatal("expected updateExpense to resolve omitted user_id via /user")
	}
	if gotForm["userId"][0] != "u-current" {
		t.Fatalf("expected current user fallback userId=u-current, got form=%v", gotForm)
	}
}

func TestUpdateExpenseRequiresValuesForChangeFieldsBeforeUpstream(t *testing.T) {
	upstreamHit := false
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		t.Fatalf("updateExpense validation must not hit upstream; got %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "amount",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"AMOUNT"}},
			want: "amount is required",
		},
		{
			name: "date",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"DATE"}},
			want: "date is required",
		},
		{
			name: "project",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"PROJECT"}},
			want: "project_id is required",
		},
		{
			name: "task",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"TASK"}},
			want: "task_id is required",
		},
		{
			name: "category",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"CATEGORY"}},
			want: "category_id is required",
		},
		{
			name: "notes",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"NOTES"}},
			want: "notes is required",
		},
		{
			name: "billable false is present",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"BILLABLE"}},
			want: "billable is required",
		},
		{
			name: "user",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"USER"}},
			want: "user_id is required",
		},
		{
			name: "file token rejected at validation",
			args: map[string]any{"expense_id": "exp1", "change_fields": []any{"FILE"}},
			want: `change_fields contains unsupported token "FILE"`,
		},
	}

	for _, tt := range tests {
		upstreamHit = false
		_, err := svc.updateExpense(context.Background(), tt.args)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("%s: expected error containing %q, got %v", tt.name, tt.want, err)
		}
		if upstreamHit {
			t.Fatalf("%s: validation hit upstream", tt.name)
		}
	}
}

func TestListExpensesDateRangeFilters(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/workspaces/ws1/expenses":
			q := r.URL.Query()
			if got := q.Get("page"); got != "2" {
				t.Fatalf("expected page=2, got %q", got)
			}
			if got := q.Get("page-size"); got != "75" {
				t.Fatalf("expected page-size=75, got %q", got)
			}
			if got := q.Get("start"); got != "2026-04-01" {
				t.Fatalf("expected start filter, got %q", got)
			}
			if got := q.Get("end"); got != "2026-04-30" {
				t.Fatalf("expected end filter, got %q", got)
			}
			respondJSON(t, w, map[string]any{
				"expenses": map[string]any{
					"expenses": []map[string]any{{"id": "exp1", "amount": 100}},
					"count":    1,
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.listExpenses(context.Background(), map[string]any{
		"page":      2,
		"page_size": 75,
		"start":     "2026-04-01",
		"end":       "2026-04-30",
	})
	mustOK(t, res, err, "clockify_list_expenses")
	if res.Meta["pageSize"] != 75 {
		t.Fatalf("expected meta pageSize=75, got %v", res.Meta["pageSize"])
	}
}

// TestTier2_Expenses_BuilderShape sanity-checks expenseHandlers' size.
func TestTier2_Expenses_BuilderShape(t *testing.T) {
	descs := expenseHandlers(New(nil, "ws1"))
	if len(descs) < 9 {
		t.Fatalf("expected at least 9 expense tools, got %d", len(descs))
	}
}

func TestCreateExpenseSchemaKeepsLiveProbeNarrowing(t *testing.T) {
	// Uploaded EXPENSESOPEAPI marks file/projectId/userId required, but
	// clockify-api-probe-lab/findings/expenses.md records live 201 responses
	// without file/projectId and a live 400 only when userId is omitted. The
	// handler resolves user_id via /user, so the public schema intentionally
	// requires only the locally non-resolvable fields.
	svc := New(nil, "ws1")
	var schema map[string]any
	for _, desc := range expenseHandlers(svc) {
		if desc.Tool.Name == "clockify_create_expense" {
			schema = desc.Tool.InputSchema
			break
		}
	}
	if schema == nil {
		t.Fatal("missing create expense schema")
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("create expense required schema = %T", schema["required"])
	}
	for _, want := range []string{"amount", "date", "category_id"} {
		if !containsString(required, want) {
			t.Fatalf("create_expense required missing %s: %#v", want, required)
		}
	}
	for _, narrowed := range []string{"file", "project_id", "user_id"} {
		if containsString(required, narrowed) {
			t.Fatalf("create_expense should not require live-optional/resolved field %s: %#v", narrowed, required)
		}
	}
}

// TestCreateExpenseUploadsReceiptWhenFileFieldsSupplied pins the
// with-file multipart shape: when all three file_* fields are present
// the handler must send a real file part named "file" alongside the
// form fields, and the bytes on the wire must round-trip the decoded
// base64.
func TestCreateExpenseUploadsReceiptWhenFileFieldsSupplied(t *testing.T) {
	const (
		filename    = "receipt.png"
		contentType = "image/png"
	)
	receiptBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

	var (
		userHit  bool
		gotForm  map[string][]string
		gotFiles map[string][]*multipart.FileHeader
	)
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			userHit = true
			respondJSON(t, w, map[string]any{"id": "u-current", "name": "Tester"})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/expenses":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("expected multipart upload, got %q", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			gotForm = r.MultipartForm.Value
			gotFiles = r.MultipartForm.File
			respondJSON(t, w, map[string]any{"id": "exp-uploaded", "amount": 9.99, "fileId": "file-1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.createExpense(context.Background(), map[string]any{
		"amount":              9.99,
		"date":                "2026-04-11T00:00:00Z",
		"category_id":         "cat1",
		"file_name":           filename,
		"file_content_base64": base64.StdEncoding.EncodeToString(receiptBytes),
		"file_content_type":   contentType,
	})
	mustOK(t, res, err, "clockify_create_expense")
	if !userHit {
		t.Fatalf("expected /user lookup before upload")
	}
	if got := gotForm["categoryId"]; len(got) != 1 || got[0] != "cat1" {
		t.Fatalf("form categoryId = %v", gotForm)
	}
	files, ok := gotFiles["file"]
	if !ok || len(files) != 1 {
		t.Fatalf("expected one file part under field 'file', got %v", gotFiles)
	}
	header := files[0]
	if header.Filename != filename {
		t.Fatalf("filename = %q, want %q", header.Filename, filename)
	}
	if got := header.Header.Get("Content-Type"); got != contentType {
		t.Fatalf("file part Content-Type = %q, want %q", got, contentType)
	}
	f, err := header.Open()
	if err != nil {
		t.Fatalf("open file part: %v", err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read file part: %v", err)
	}
	if string(got) != string(receiptBytes) {
		t.Fatalf("uploaded bytes = %x, want %x", got, receiptBytes)
	}
}

// TestCreateExpenseNoFileSendsPlainMultipart confirms the no-file path
// is preserved: a create without file_* fields must still POST plain
// multipart/form-data with zero file parts.
func TestCreateExpenseNoFileSendsPlainMultipart(t *testing.T) {
	var gotFiles map[string][]*multipart.FileHeader
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			respondJSON(t, w, map[string]any{"id": "u-current"})
		case r.Method == http.MethodPost && r.URL.Path == "/workspaces/ws1/expenses":
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			gotFiles = r.MultipartForm.File
			respondJSON(t, w, map[string]any{"id": "exp-nofile", "amount": 5})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.createExpense(context.Background(), map[string]any{
		"amount":      5.0,
		"date":        "2026-04-11T00:00:00Z",
		"category_id": "cat1",
	})
	mustOK(t, res, err, "clockify_create_expense")
	if len(gotFiles) != 0 {
		t.Fatalf("no-file create must not send file parts, got %v", gotFiles)
	}
}

// TestCreateExpenseRejectsPartialFileTrio fails closed: a caller that
// supplies only some of the file_* fields must get a clear error
// before any upstream call.
func TestCreateExpenseRejectsPartialFileTrio(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			respondJSON(t, w, map[string]any{"id": "u-current"})
			return
		}
		t.Fatalf("partial file trio must fail before the expenses POST: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	partials := []map[string]any{
		{"file_name": "r.pdf"},
		{"file_content_base64": base64.StdEncoding.EncodeToString([]byte("hi"))},
		{"file_content_type": "application/pdf"},
		{"file_name": "r.pdf", "file_content_type": "application/pdf"},
	}
	for i, partial := range partials {
		args := map[string]any{"amount": 1.0, "date": "2026-04-11T00:00:00Z", "category_id": "cat1"}
		for k, v := range partial {
			args[k] = v
		}
		_, err := svc.createExpense(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "file_name, file_content_base64, and file_content_type") {
			t.Fatalf("case %d: expected partial-trio error, got %v", i, err)
		}
	}
}

// TestCreateExpenseRejectsInvalidBase64 keeps the decode failure local
// with an actionable message instead of shipping garbage upstream.
func TestCreateExpenseRejectsInvalidBase64(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			respondJSON(t, w, map[string]any{"id": "u-current"})
			return
		}
		t.Fatalf("invalid base64 must fail before the expenses POST: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.createExpense(context.Background(), map[string]any{
		"amount":              1.0,
		"date":                "2026-04-11T00:00:00Z",
		"category_id":         "cat1",
		"file_name":           "r.pdf",
		"file_content_base64": "not valid base64!!!",
		"file_content_type":   "application/pdf",
	})
	if err == nil || !strings.Contains(err.Error(), "not valid base64") {
		t.Fatalf("expected base64 decode error, got %v", err)
	}
}

// TestCreateExpenseRejectsEmptyReceiptBody pins that an empty
// file_content_base64 — a zero-byte receipt — never reaches HTTP. An
// empty body counts as a missing field, so name+type with an empty body
// fails the trio guard locally rather than POSTing a 0-byte file part.
func TestCreateExpenseRejectsEmptyReceiptBody(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			respondJSON(t, w, map[string]any{"id": "u-current"})
			return
		}
		t.Fatalf("empty receipt body must fail before the expenses POST: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.createExpense(context.Background(), map[string]any{
		"amount":              1.0,
		"date":                "2026-04-11T00:00:00Z",
		"category_id":         "cat1",
		"file_name":           "empty.pdf",
		"file_content_base64": "",
		"file_content_type":   "application/pdf",
	})
	if err == nil || !strings.Contains(err.Error(), "must be provided together") {
		t.Fatalf("empty receipt body should fail the trio guard before HTTP, got %v", err)
	}
}

// TestUpdateExpenseSchemaOmitsFileToken pins that the change_fields
// enum no longer advertises FILE — receipt replacement on update is
// deliberately unsupported.
func TestUpdateExpenseSchemaOmitsFileToken(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("schema-only test must not make HTTP calls: %s %s", r.Method, r.URL.Path)
	})
	defer cleanup()
	svc := New(client, "ws1")
	for _, d := range expenseHandlers(svc) {
		if d.Tool.Name != "clockify_update_expense" {
			continue
		}
		props, _ := d.Tool.InputSchema["properties"].(map[string]any)
		cf, _ := props["change_fields"].(map[string]any)
		items, _ := cf["items"].(map[string]any)
		enum, ok := items["enum"].([]string)
		if !ok {
			t.Fatalf("change_fields enum = %T", items["enum"])
		}
		if containsString(enum, "FILE") {
			t.Fatalf("change_fields enum must not advertise FILE: %v", enum)
		}
		return
	}
	t.Fatal("clockify_update_expense descriptor not found")
}

func TestListExpenseCategories_PaginationMeta(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/expenses/categories" && r.Method == http.MethodGet:
			respondJSON(t, w, map[string]any{
				"count": 12,
				"categories": []map[string]any{
					{"id": "cat1", "name": "Category 1"},
					{"id": "cat2", "name": "Category 2"},
					{"id": "cat3", "name": "Category 3"},
					{"id": "cat4", "name": "Category 4"},
					{"id": "cat5", "name": "Category 5"},
					{"id": "cat6", "name": "Category 6"},
					{"id": "cat7", "name": "Category 7"},
				},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	first, err := svc.listExpenseCategories(context.Background(), map[string]any{"page_size": 5})
	if err != nil {
		t.Fatalf("listExpenseCategories page 1: %v", err)
	}
	items, ok := first.Data.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any, got %T", first.Data)
	}
	if len(items) != 5 {
		t.Fatalf("page 1 len=%d, want 5", len(items))
	}
	if first.Meta["page"] != 1 || first.Meta["pageSize"] != 5 || first.Meta["has_more"] != true {
		t.Fatalf("unexpected page 1 meta: %#v", first.Meta)
	}

	second, err := svc.listExpenseCategories(context.Background(), map[string]any{"page": 2, "page_size": 5})
	if err != nil {
		t.Fatalf("listExpenseCategories page 2: %v", err)
	}
	secondItems := second.Data.([]map[string]any)
	if len(secondItems) != 2 {
		t.Fatalf("page 2 len=%d, want 2", len(secondItems))
	}
	if items[0]["id"] == secondItems[0]["id"] {
		t.Fatalf("page 2 did not advance: page1=%#v page2=%#v", items, secondItems)
	}
}
