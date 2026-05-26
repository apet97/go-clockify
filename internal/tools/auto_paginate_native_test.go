package tools

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// TestInvoicesListAutoPaginateWalksEveryPage exercises the wrapped
// native invoices_list handler. invoices is one of the three bespoke
// upstream envelopes covered by phase 2: GET /workspaces/ws1/invoices
// returns {total, invoices: [...]}. The wrapper walks pages by
// resubmitting `page=N` until a short batch comes back, then merges
// the per-page Data slices.
func TestInvoicesListAutoPaginateWalksEveryPage(t *testing.T) {
	var requests int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if !strings.HasSuffix(r.URL.Path, "/workspaces/ws1/invoices") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		switch r.URL.Query().Get("page") {
		case "1":
			full := make([]map[string]any, autoPaginatePageSize)
			for i := range full {
				full[i] = map[string]any{"id": "inv-p1", "currency": "USD"}
			}
			respondJSON(t, w, map[string]any{"total": autoPaginatePageSize + 3, "invoices": full})
		case "2":
			respondJSON(t, w, map[string]any{"total": autoPaginatePageSize + 3, "invoices": []map[string]any{
				{"id": "inv-p2-a", "currency": "USD"},
				{"id": "inv-p2-b", "currency": "USD"},
				{"id": "inv-p2-c", "currency": "USD"},
			}})
		default:
			t.Fatalf("unexpected page %q", r.URL.Query().Get("page"))
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	wrapped := autoPaginated(svc.listInvoices)
	result, err := wrapped(context.Background(), map[string]any{"auto_paginate": true})
	if err != nil {
		t.Fatalf("listInvoices auto: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("expected 2 HTTP requests, got %d", got)
	}
	if v, _ := result.Meta["auto_paginate"].(bool); !v {
		t.Fatalf("meta.auto_paginate = %v, want true", result.Meta["auto_paginate"])
	}
	if v, _ := result.Meta["has_more"].(bool); v {
		t.Fatalf("meta.has_more = true, want false (auto consumed every page)")
	}
	if got, _ := result.Meta["count"].(int); got != autoPaginatePageSize+3 {
		t.Fatalf("meta.count = %d, want %d", got, autoPaginatePageSize+3)
	}
	data := reflect.ValueOf(result.Data)
	if data.Kind() != reflect.Slice || data.Len() != autoPaginatePageSize+3 {
		t.Fatalf("merged data slice len = %d, want %d", data.Len(), autoPaginatePageSize+3)
	}
}

// TestExpensesListAutoPaginateRespectsMaxRows covers expenses
// (doubly-nested upstream envelope: {expenses: {expenses: [...], count}})
// and the max_rows truncation path. Returning a steady stream of full
// pages forces the loop to halt on the cap rather than on a short page,
// proving the truncation branch fires for this handler shape too.
func TestExpensesListAutoPaginateRespectsMaxRows(t *testing.T) {
	var requests int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// listExpenses also queries the workspace endpoint to look up the
		// default currency; only count the expenses-list calls.
		if strings.HasSuffix(r.URL.Path, "/workspaces/ws1") {
			respondJSON(t, w, map[string]any{"id": "ws1", "currencies": []map[string]any{{"code": "USD"}}})
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/workspaces/ws1/expenses") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		atomic.AddInt32(&requests, 1)
		page := make([]map[string]any, autoPaginatePageSize)
		for i := range page {
			page[i] = map[string]any{
				"id":         "exp",
				"category":   map[string]any{"name": "Travel"},
				"projectId":  "p",
				"amount":     1000,
				"unitOfTime": "USD",
				"billable":   true,
			}
		}
		respondJSON(t, w, map[string]any{
			"expenses": map[string]any{
				"expenses": page,
				"count":    10_000,
			},
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	wrapped := autoPaginated(svc.listExpenses)
	result, err := wrapped(context.Background(), map[string]any{
		"auto_paginate": true,
		"max_rows":      float64(250),
	})
	if err != nil {
		t.Fatalf("listExpenses auto: %v", err)
	}
	// max_rows=250, page_size=200 → 2 fetches, second trimmed to 50.
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("expected 2 HTTP requests under max_rows=250, got %d", got)
	}
	if v, _ := result.Meta["truncated"].(bool); !v {
		t.Fatalf("meta.truncated = %v, want true", result.Meta["truncated"])
	}
	if got, _ := result.Meta["count"].(int); got != 250 {
		t.Fatalf("meta.count = %d, want 250 (max_rows cap)", got)
	}
	data := reflect.ValueOf(result.Data)
	if data.Kind() != reflect.Slice || data.Len() != 250 {
		t.Fatalf("merged data slice len = %d, want 250", data.Len())
	}
}

// TestCustomFieldsListWithoutAutoPaginateUnchanged guards the
// pass-through path on a handler whose upstream returns a flat JSON
// array (no envelope). Without the flag, the wrapper must invoke the
// handler exactly once and not stamp the auto/truncated meta keys.
func TestCustomFieldsListWithoutAutoPaginateUnchanged(t *testing.T) {
	var requests int32
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		respondJSON(t, w, []map[string]any{
			{"id": "cf1", "name": "Cost Center"},
			{"id": "cf2", "name": "Region"},
		})
	})
	defer cleanup()

	svc := New(client, "ws1")
	wrapped := autoPaginated(svc.ListCustomFields)
	result, err := wrapped(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("ListCustomFields: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected 1 HTTP request, got %d", got)
	}
	if _, present := result.Meta["auto_paginate"]; present {
		t.Fatalf("meta.auto_paginate leaked onto single-page path: %+v", result.Meta)
	}
	if _, present := result.Meta["truncated"]; present {
		t.Fatalf("meta.truncated leaked onto single-page path: %+v", result.Meta)
	}
}

// TestInjectAutoPaginateSchemaPropsAddsKnobsWhenPagePresent proves the
// schema mutator only fires when the schema already declares `page` —
// non-paginated tools (holidays, webhook events) must not start
// advertising auto_paginate they cannot honour.
func TestInjectAutoPaginateSchemaPropsAddsKnobsWhenPagePresent(t *testing.T) {
	paged := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"page":      map[string]any{"type": "integer"},
			"page_size": map[string]any{"type": "integer"},
		},
	}
	injectAutoPaginateSchemaProps(paged)
	props := paged["properties"].(map[string]any)
	if _, ok := props["auto_paginate"]; !ok {
		t.Fatalf("auto_paginate not injected on paged schema: %+v", props)
	}
	if _, ok := props["max_rows"]; !ok {
		t.Fatalf("max_rows not injected on paged schema: %+v", props)
	}

	unpaged := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	injectAutoPaginateSchemaProps(unpaged)
	props = unpaged["properties"].(map[string]any)
	if _, ok := props["auto_paginate"]; ok {
		t.Fatalf("auto_paginate leaked onto non-paged schema: %+v", props)
	}
}
