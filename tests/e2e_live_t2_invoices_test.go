//go:build legacy_platform && livee2e

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

func TestLiveT2InvoicesCRUD(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_BILLING_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("invoices")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := h.callOK(ctx, "clockify_create_client", map[string]any{
		"name": c.LivePrefix("invoice-client", 0),
	})
	clientID, _ := extractDataMap(t, client)["id"].(string)
	if clientID == "" {
		t.Fatalf("clockify_create_client returned no id: %#v", client)
	}
	c.RegisterCleanup("client", clientID, func(ctx context.Context) error {
		return c.rawArchiveAndDeleteClient(ctx, clientID)
	})

	issuedDate := time.Now().UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z")
	dueDate := time.Now().UTC().AddDate(0, 0, 14).Truncate(time.Second).Format("2006-01-02T15:04:05Z")
	invoice := h.callOK(ctx, "clockify_create_invoice", map[string]any{
		"client_id":   clientID,
		"number":      c.LivePrefix("inv", 0),
		"issued_date": issuedDate,
		"currency":    "USD",
		"due_date":    dueDate,
		"note":        c.LivePrefix("invoice", 0),
	})
	invoiceID, _ := extractDataMap(t, invoice)["id"].(string)
	if invoiceID == "" {
		t.Fatalf("clockify_create_invoice returned no id: %#v", invoice)
	}
	invoiceDeleted := false
	c.RegisterCleanup("invoice", invoiceID, func(ctx context.Context) error {
		if invoiceDeleted {
			return nil
		}
		return c.rawDeletePath(ctx, "/invoices/"+invoiceID)
	})

	_ = h.callOK(ctx, "clockify_list_invoices", map[string]any{"page": 1, "page_size": 5})
	got := h.callOK(ctx, "clockify_get_invoice", map[string]any{"invoice_id": invoiceID})
	if gotID, _ := extractDataMap(t, got)["id"].(string); gotID != invoiceID {
		t.Fatalf("clockify_get_invoice id mismatch: got %q want %q", gotID, invoiceID)
	}
	_ = h.callOK(ctx, "clockify_export_invoice", map[string]any{
		"invoice_id": invoiceID,
	})

	updated := h.callOK(ctx, "clockify_update_invoice", map[string]any{
		"invoice_id":  invoiceID,
		"client_id":   clientID,
		"number":      c.LivePrefix("inv", 1),
		"issued_date": issuedDate,
		"currency":    "USD",
		"due_date":    dueDate,
		"note":        c.LivePrefix("invoice-updated", 0),
	})
	if updatedID, _ := extractDataMap(t, updated)["id"].(string); updatedID != invoiceID {
		t.Fatalf("clockify_update_invoice id mismatch: got %q want %q", updatedID, invoiceID)
	}
	_ = h.callOK(ctx, "clockify_update_invoice", map[string]any{
		"invoice_id": invoiceID,
		"status":     "SENT",
	})

	item := h.callOK(ctx, "clockify_add_invoice_item", map[string]any{
		"invoice_id":  invoiceID,
		"description": c.LivePrefix("invoice-item", 0),
		"quantity":    100.0,
		// Raw minor-unit value, passed through without conversion.
		"unit_price":  1000.0,
		"apply_taxes": "NONE",
		"item_type":   "NEW DEFAULT",
	})
	itemID, _ := extractDataMap(t, item)["id"].(string)
	if itemID == "" {
		t.Logf("clockify_add_invoice_item returned no line-item id; per-item mutation routes are probed as unsupported: %#v", item)
		itemID = "000000000000000000000001"
	}

	items := h.callOK(ctx, "clockify_list_invoice_items", map[string]any{"invoice_id": invoiceID})
	itemRows := extractList(t, items)
	if len(itemRows) == 0 {
		t.Fatalf("clockify_list_invoice_items returned no items after add: %#v", items)
	}
	if first, ok := itemRows[0].(map[string]any); ok {
		if order, ok := first["order"].(float64); ok {
			itemID = fmt.Sprintf("%.0f", order)
		}
	}
	if itemID == "" {
		t.Fatalf("clockify_list_invoice_items returned no line order: %#v", itemRows[0])
	}

	updateItemErr := h.callExpectError(ctx, "clockify_update_invoice_item", map[string]any{
		"invoice_id":  invoiceID,
		"item_id":     itemID,
		"description": c.LivePrefix("invoice-item-updated", 0),
		"quantity":    100.0,
		"unit_price":  2000.0,
		"apply_taxes": "NONE",
		"item_type":   "NEW DEFAULT",
	})
	if !containsAny(updateItemErr, "not supported", "not found", "doesn't belong") {
		t.Fatalf("expected live unsupported/not-found response for update_invoice_item, got: %q", updateItemErr)
	}

	_ = h.callOK(ctx, "clockify_delete_invoice_item", map[string]any{
		"invoice_id": invoiceID,
		"item_id":    itemID,
	})

	_ = h.callOK(ctx, "clockify_send_invoice", map[string]any{
		"invoice_id": invoiceID,
		"dry_run":    true,
	})
	_ = h.callOK(ctx, "clockify_mark_invoice_paid", map[string]any{
		"invoice_id": invoiceID,
		"dry_run":    true,
	})
	_ = h.callOK(ctx, "clockify_mark_invoice_paid", map[string]any{
		"invoice_id": invoiceID,
	})

	_ = h.callOK(ctx, "clockify_invoice_report", map[string]any{"page": 1, "page_size": 5})
	_ = h.callOK(ctx, "clockify_delete_invoice", map[string]any{
		"invoice_id": invoiceID,
		"dry_run":    true,
	})
	_ = h.callOK(ctx, "clockify_delete_invoice", map[string]any{
		"invoice_id": invoiceID,
	})
	invoiceDeleted = true
}

func containsAny(s string, needles ...string) bool {
	lower := strings.ToLower(s)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
