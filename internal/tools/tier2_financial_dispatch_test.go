package tools_test

// Dispatcher-level coverage for the Tier 2 financial groups (invoices +
// expenses). Each test drives the full MCP pipeline
// (initialize → tools/call → enforcement → handler → fake upstream) via
// dispatchTier2 so the registered handler closures, the schema gate,
// and the policy gate are all exercised end to end.
//
// The existing tier2_financial_test.go and tier2_expenses_test.go files
// use direct svc.listInvoices(ctx, args) calls for white-box coverage
// of response shape and error validation — that pattern is fine (see
// the testharness package docstring) but it skips the enforcement
// pipeline the safety layers live in. This file adds the missing
// dispatch-harness coverage alongside those happy-path tests so a
// regression in policy or schema enforcement for the financial tools
// surfaces in CI instead of in production.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/testharness"
)

// newFinancialUpstream stubs the endpoints touched by the invoices and
// expenses handlers. Each route returns the smallest response payload
// the handler needs to decode without erroring, so failures from the
// dispatch path surface cleanly instead of being lost in stub noise.
func newFinancialUpstream(t *testing.T) *testharness.FakeClockify {
	t.Helper()
	mux := http.NewServeMux()

	// Invoices collection: list (GET) + create (POST).
	mux.HandleFunc("/workspaces/test-workspace/invoices", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"total":1,"invoices":[{"id":"inv-1","status":"DRAFT","amount":100}]}`))
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"inv-new","status":"DRAFT","amount":250}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Single invoice: get (for delete's pre-fetch) + delete.
	mux.HandleFunc("/workspaces/test-workspace/invoices/inv-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"inv-1","status":"DRAFT","amount":100}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Expenses collection: list (GET) + create (POST).
	mux.HandleFunc("/workspaces/test-workspace/expenses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"expenses":{"expenses":[{"id":"exp-1","amount":50,"date":"2026-04-01"}],"count":1}}`))
		case http.MethodPost:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "exp-new"
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Single expense: get (for delete's pre-fetch) + delete.
	mux.HandleFunc("/workspaces/test-workspace/expenses/exp-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"exp-1","amount":50,"date":"2026-04-01"}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return testharness.NewFakeClockify(t, mux)
}

func TestTier2Dispatch_Invoices_List(t *testing.T) {
	upstream := newFinancialUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "invoices",
		Tool:     "clockify_list_invoices",
		Args:     map[string]any{"page": 1, "page_size": 25},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("list did not reach upstream")
	}
	if !strings.Contains(res.ResultText, "inv-1") {
		t.Fatalf("list result missing id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Invoices_Create(t *testing.T) {
	upstream := newFinancialUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "invoices",
		Tool:  "clockify_create_invoice",
		Args: map[string]any{
			"client_id": "client-1",
			"currency":  "USD",
			"due_date":  "2026-05-01",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("create outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("create did not reach upstream")
	}
	if !strings.Contains(res.ResultText, "inv-new") {
		t.Fatalf("create result missing new id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Invoices_DeleteDryRun(t *testing.T) {
	upstream := newFinancialUpstream(t)

	// Dry run hits GET (pre-fetch preview) but MUST NOT hit DELETE.
	// The request counter can't distinguish those, so we only assert
	// the successful dry-run envelope here; the tier2_financial_test.go
	// TestDeleteInvoiceDryRun covers the "DELETE was not called" shape.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "invoices",
		Tool:  "clockify_delete_invoice",
		Args: map[string]any{
			"invoice_id": "inv-1",
			"dry_run":    true,
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("delete(dry_run) outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("delete(dry_run) did not reach upstream for pre-fetch")
	}
	if !strings.Contains(res.ResultText, "dry_run") {
		t.Fatalf("delete(dry_run) missing dry_run marker: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Invoices_SchemaValidation(t *testing.T) {
	upstream := newFinancialUpstream(t)

	// Missing required invoice_id must be rejected by the schema gate
	// BEFORE the handler runs — UpstreamHit must be false, outcome
	// must be OutcomeInvalidParams (JSON-RPC -32602).
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "invoices",
		Tool:     "clockify_get_invoice",
		Args:     map[string]any{},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeInvalidParams {
		t.Fatalf("expected invalid_params, got %q (err=%q)", res.Outcome, res.ErrorMessage)
	}
	if res.UpstreamHit {
		t.Fatalf("schema-rejected call must not reach upstream")
	}
}

func TestTier2Dispatch_Expenses_List(t *testing.T) {
	upstream := newFinancialUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "expenses",
		Tool:     "clockify_list_expenses",
		Args:     map[string]any{"page": 1, "page_size": 25},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("list did not reach upstream")
	}
	if !strings.Contains(res.ResultText, "exp-1") {
		t.Fatalf("list result missing id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Expenses_Create(t *testing.T) {
	upstream := newFinancialUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "expenses",
		Tool:  "clockify_create_expense",
		Args: map[string]any{
			"amount":      42.5,
			"date":        "2026-04-01",
			"category_id": "cat-1",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("create outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("create did not reach upstream")
	}
	if !strings.Contains(res.ResultText, "exp-new") {
		t.Fatalf("create result missing new id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Expenses_PolicyReadOnlyBlocksCreate(t *testing.T) {
	upstream := newFinancialUpstream(t)

	// ReadOnly policy must reject the write tool at the policy gate,
	// BEFORE the handler runs — UpstreamHit false, outcome policy_denied.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:      "expenses",
		Tool:       "clockify_create_expense",
		Args:       map[string]any{"amount": 10.0, "date": "2026-04-01", "category_id": "cat-1"},
		PolicyMode: policy.ReadOnly,
		Upstream:   upstream,
	})
	if res.Outcome != testharness.OutcomePolicyDenied {
		t.Fatalf("expected policy_denied, got %q (err=%q)", res.Outcome, res.ErrorMessage)
	}
	if res.UpstreamHit {
		t.Fatalf("policy-denied call reached upstream")
	}
}
