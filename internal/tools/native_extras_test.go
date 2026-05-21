package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// TestInvoicesInfoPagedQuery locks the clockify_invoices_info tool: it POSTs
// to /invoices/info, forwards the status filter, and reports total + has_more
// so a caller can page (audit P2-6b/P3-8).
func TestInvoicesInfoPagedQuery(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":149,"invoices":[{"id":"i1","number":"INV-1","status":"PAID"}]}`))
	}))
	defer ts.Close()

	svc := New(clockify.NewClient("k", ts.URL, 5*time.Second, 0), "000000000000000000000001")
	env, err := svc.InvoicesInfo(context.Background(), map[string]any{
		"page":      1,
		"page_size": 1,
		"statuses":  []any{"PAID"},
	})
	if err != nil {
		t.Fatalf("InvoicesInfo: %v", err)
	}
	if gotPath != "/workspaces/000000000000000000000001/invoices/info" {
		t.Fatalf("path = %s, want /workspaces/.../invoices/info", gotPath)
	}
	if gotBody["statuses"] == nil {
		t.Fatalf("statuses filter not forwarded: %v", gotBody)
	}
	if !env.OK {
		t.Fatalf("envelope not ok: %+v", env)
	}
	if got, _ := env.Meta["total"].(int); got != 149 {
		t.Fatalf("meta total = %v, want 149", env.Meta["total"])
	}
	if env.Meta["has_more"] != true {
		t.Fatalf("has_more = %v, want true (page 1 of 149 at pageSize 1)", env.Meta["has_more"])
	}
	invoices, ok := env.Data.([]CompactInvoiceView)
	if !ok {
		t.Fatalf("data type = %T, want []CompactInvoiceView", env.Data)
	}
	if len(invoices) != 1 || invoices[0].ID != "i1" || invoices[0].Number != "INV-1" || invoices[0].Status != "PAID" {
		t.Fatalf("compact invoices not preserved: %+v", invoices)
	}
}

// TestSchedulingPublishReceiptAndDryRun locks clockify_scheduling_publish: a
// dry_run previews without the PUT; a real call PUTs once and returns a
// published receipt synthesised from the request (audit P2-6a).
func TestSchedulingPublishReceiptAndDryRun(t *testing.T) {
	puts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts++
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc := New(clockify.NewClient("k", ts.URL, 5*time.Second, 0), "000000000000000000000001")
	svc.DefaultTimezone = time.UTC

	dry, err := svc.SchedulingPublish(context.Background(), map[string]any{
		"start":   "2026-05-01",
		"end":     "2026-05-31",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("dry_run publish: %v", err)
	}
	if data, _ := dry.Data.(map[string]any); data["dry_run"] != true {
		t.Fatalf("dry_run should return a preview, got %+v", dry.Data)
	}
	if puts != 0 {
		t.Fatalf("dry_run must not PUT; observed %d", puts)
	}

	env, err := svc.SchedulingPublish(context.Background(), map[string]any{
		"start": "2026-05-01",
		"end":   "2026-05-31",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	data, _ := env.Data.(map[string]any)
	if data["published"] != true {
		t.Fatalf("expected a published receipt, got %+v", env.Data)
	}
	if puts != 1 {
		t.Fatalf("expected exactly one PUT, observed %d", puts)
	}
}
