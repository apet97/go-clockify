package tools_test

// Per-tool micro-benchmarks for a representative slice of the Tier-2
// destructive write surface. The Tier-1 benches in writes_bench_test.go
// cover the six always-on write tools; this file extends that coverage to
// five lazy-loaded tools across four Tier-2 groups so a regression in any
// of them is visible to the weekly bench-regression gate.
//
// The groups exercised here were chosen to cover distinct code paths:
//
//   expenses        — pure POST with optional-field sub-struct assembly
//   invoices        — POST gated on resolve.ValidateID(client_id)
//   custom_fields   — POST with enum validation + array-of-strings body
//   approvals       — both POST (submit_for_approval) and PUT
//                     (approve_timesheet) so the enforcement pipeline's
//                     mutation-path handling shows up in two flavours
//
// Tier-2 tools are not in svc.Registry() by design — they're activated at
// runtime via Server.ActivateGroup. BenchHarness handles that via
// InvokeOpts.ActivateTier2Groups so the measured dispatch path here is
// identical to production: parse tools/call → enforcement pipeline →
// tool handler → clockify client → upstream.
//
// Run locally:
//
//	go test -bench=BenchmarkClockify -benchmem -benchtime=100x -count=1 \
//	  -run='^$' ./internal/tools/...

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/testharness"
)

// stubClockifyForTier2Writes returns a fake upstream covering the five
// Tier-2 write paths benched in this file. Each branch returns the
// smallest possible JSON body the handler can parse without erroring —
// enough to keep the benchmark's upstream cost sub-millisecond and the
// measured handler/dispatch cost dominant.
func stubClockifyForTier2Writes(b *testing.B) *testharness.FakeClockify {
	b.Helper()
	const expenseJSON = `{"id":"exp-bench","amount":42,"date":"2026-01-01"}`
	const invoiceJSON = `{"id":"inv-bench","clientId":"c-bench","status":"DRAFT"}`
	const customFieldJSON = `{"id":"cf-bench","name":"bench","type":"TEXT"}`
	const approvalJSON = `{"id":"ap-bench","status":"PENDING","start":"2026-01-01","end":"2026-01-07"}`
	const approvedJSON = `{"id":"ap-bench","status":"APPROVED"}`

	return testharness.NewFakeClockify(b, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		// expenses — clockify_create_expense
		case strings.HasSuffix(path, "/expenses") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(expenseJSON))
		// invoices — clockify_create_invoice
		case strings.HasSuffix(path, "/invoices") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(invoiceJSON))
		// custom-fields — clockify_create_custom_field
		case strings.HasSuffix(path, "/custom-fields") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(customFieldJSON))
		// approval-requests POST — clockify_submit_for_approval
		case strings.HasSuffix(path, "/approval-requests") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(approvalJSON))
		// approval-requests PUT — clockify_approve_timesheet
		case strings.Contains(path, "/approval-requests/") && r.Method == http.MethodPut:
			_, _ = w.Write([]byte(approvedJSON))
		default:
			http.NotFound(w, r)
		}
	}))
}

// invokeTier2WriteBench runs one Tier-2 tools/call per iteration through
// the amortised BenchHarness. Mirrors invokeWriteBench in writes_bench_test.go
// but seeds the harness with the Tier-2 groups the benchmarks dispatch
// against so Server.ActivateGroup runs once at harness construction
// rather than per-iteration.
func invokeTier2WriteBench(b *testing.B, tool string, groups []string, argsFn func() map[string]any) {
	b.Helper()
	upstream := stubClockifyForTier2Writes(b)
	client := clockify.NewClient("test-api-key", upstream.URL(), 5*time.Second, 0)
	b.Cleanup(client.Close)

	harness := testharness.NewBenchHarness(b, testharness.InvokeOpts{
		PolicyMode:          policy.Standard,
		Upstream:            upstream,
		Client:              client,
		ActivateTier2Groups: groups,
	})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result := harness.Call(ctx, tool, argsFn())
		if result.Outcome != testharness.OutcomeSuccess {
			b.Fatalf("%s: outcome=%q err=%q raw=%s",
				tool, result.Outcome, result.ErrorMessage, string(result.Raw))
		}
	}
}

// BenchmarkClockifyCreateExpense measures the expenses.createExpense
// path: validate required args → assemble body → POST
// /workspaces/{ws}/expenses. One upstream HTTP request per iteration.
func BenchmarkClockifyCreateExpense(b *testing.B) {
	invokeTier2WriteBench(b, "clockify_create_expense", []string{"expenses"}, func() map[string]any {
		return map[string]any{
			"amount":      42.5,
			"date":        "2026-01-01",
			"category_id": "cat-bench",
			"description": "bench",
		}
	})
}

// BenchmarkClockifyCreateInvoice measures the invoices.createInvoice
// path: ValidateID(client_id) → assemble body → POST
// /workspaces/{ws}/invoices. One upstream HTTP request per iteration.
func BenchmarkClockifyCreateInvoice(b *testing.B) {
	invokeTier2WriteBench(b, "clockify_create_invoice", []string{"invoices"}, func() map[string]any {
		return map[string]any{
			"client_id": "c-bench",
			"currency":  "USD",
			"due_date":  "2026-02-01",
			"note":      "bench",
		}
	})
}

// BenchmarkClockifyCreateCustomField measures the custom_fields.CreateCustomField
// path: strings.ToUpper(field_type) enum check → body assembly → POST
// /workspaces/{ws}/custom-fields. One upstream HTTP request per iteration.
func BenchmarkClockifyCreateCustomField(b *testing.B) {
	invokeTier2WriteBench(b, "clockify_create_custom_field", []string{"custom_fields"}, func() map[string]any {
		return map[string]any{
			"name":       "bench",
			"field_type": "TEXT",
			"required":   false,
		}
	})
}

// BenchmarkClockifySubmitForApproval measures the approvals.submitForApproval
// path: required-field check → body assembly → POST
// /workspaces/{ws}/approval-requests. One upstream HTTP request per iteration.
func BenchmarkClockifySubmitForApproval(b *testing.B) {
	invokeTier2WriteBench(b, "clockify_submit_for_approval", []string{"approvals"}, func() map[string]any {
		return map[string]any{
			"start": "2026-01-01T00:00:00Z",
			"end":   "2026-01-07T23:59:59Z",
		}
	})
}

// BenchmarkClockifyApproveTimesheet measures the approvals.approveTimesheet
// mutation path (no dry_run): ValidateID(approval_id) → PUT
// /workspaces/{ws}/approval-requests/{id}. One upstream HTTP request per
// iteration, distinct from submitForApproval because the PUT response
// envelope and status-field merge logic are different code paths.
func BenchmarkClockifyApproveTimesheet(b *testing.B) {
	invokeTier2WriteBench(b, "clockify_approve_timesheet", []string{"approvals"}, func() map[string]any {
		return map[string]any{
			"approval_id": "ap-bench",
		}
	})
}
