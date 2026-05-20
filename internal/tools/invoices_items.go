package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func (s *Service) listInvoiceItems(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Upstream rejects GET /invoices/{id}/items with 405. Items are
	// embedded in the single-invoice response; delegate to getInvoice
	// and extract the items array. Probe evidence: clockify-api-probe-
	// lab/findings/invoices.md (rev 2 2026-05-02).
	inv, err := s.getInvoice(ctx, args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	items := []InvoiceItemView{}
	if invoice, ok := inv.Data.(InvoiceView); ok {
		if typed, ok := invoice["items"].([]InvoiceItemView); ok {
			items = append(items, typed...)
		} else {
			items = append(items, invoiceItemViews(mapSlice(invoice["items"]), reportValueString(invoice["currency"]))...)
		}
	}
	return ok("clockify_list_invoice_items", items, emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"count":       len(items),
	}, "clockify_invoices_items_add")), nil
}

// invoiceMinorUnitSchema returns the shared schema for a *_unit input
// pinned to invoice surfaces where the wire format is Clockify's minor
// units (cents) per the canonical OpenAPI. The default is minor so a raw
// value passes through unchanged; major causes the handler to multiply
// by 100 before the upstream call.
func invoiceMinorUnitSchema(field string) map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"minor", "major"},
		"description": fmt.Sprintf("Unit for %s. minor (default) sends the value verbatim, which is the live API expectation; major multiplies by 100 before the request so callers can supply major currency units.", field),
	}
}

// convertToInvoiceMinorUnits resolves the *_unit toggle for an invoice
// amount field and returns the value Clockify expects on the wire (int64
// minor units). Returns the value unchanged when unit is minor, multiplies
// by 100 when unit is major, and validates that the resolved value fits
// in int64 with no fractional cents — the upstream AddInvoicePaymentRequest
// and InvoiceItemRequest both use integer minor units.
func convertToInvoiceMinorUnits(value any, unit string) (int64, error) {
	if value == nil {
		return 0, fmt.Errorf("value is required")
	}
	asFloat, ok := numberFromAny(value)
	if !ok {
		return 0, fmt.Errorf("value must be a number, got %s", jsonTypeName(value))
	}
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch unit {
	case "", "minor":
		if asFloat != float64(int64(asFloat)) {
			return 0, fmt.Errorf("minor-unit value %v must be an integer (cents); use _unit:\"major\" to convert from major currency units", value)
		}
		return int64(asFloat), nil
	case "major":
		// Round to two decimal places before promoting to cents to
		// dodge IEEE-754 drift like 1.23 * 100 == 122.99999...
		scaled := asFloat * 100
		rounded := int64(scaled + 0.5)
		if scaled < 0 {
			rounded = int64(scaled - 0.5)
		}
		return rounded, nil
	default:
		return 0, fmt.Errorf("unit must be minor or major, got %q", unit)
	}
}

func (s *Service) addInvoiceItem(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if v := stringArg(args, "description"); v != "" {
		body["description"] = v
	}
	if _, ok := args["quantity"]; ok {
		quantity, isNumber := numberArg(args, "quantity")
		if !isNumber || quantity <= 0 {
			return ResultEnvelope{}, fmt.Errorf("quantity must be a number greater than 0")
		}
		body["quantity"] = quantity
	}
	if v, ok := args["unit_price"]; ok {
		converted, err := convertToInvoiceMinorUnits(v, stringArg(args, "unit_price_unit"))
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("unit_price: %w", err)
		}
		body["unitPrice"] = converted
	}
	if v := stringArg(args, "apply_taxes"); v != "" {
		body["applyTaxes"] = v
	} else {
		body["applyTaxes"] = "NONE"
	}
	itemType := stringArg(args, "item_type")
	if itemType == "" {
		return ResultEnvelope{}, fmt.Errorf("item_type is required")
	}
	body["itemType"] = itemType

	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if dryrun.Enabled(args) {
		return ok("clockify_add_invoice_item", dryrunPreviewPayload("clockify_add_invoice_item", body), map[string]any{
			"workspaceId": wsID,
			"invoiceId":   invoiceID,
		}), nil
	}
	var created map[string]any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_add_invoice_item", invoiceItemViewFromRaw(created, ""), map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
	}), nil
}

func invoiceItemIndexArg(args map[string]any) (string, error) {
	if v := strings.TrimSpace(stringArg(args, "item_index")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(stringArg(args, "item_id")); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("item_index is required (legacy item_id is also accepted)")
}

// updateInvoiceItem reports the absence of a Clockify update endpoint for
// invoice line items. The live API rejects PUT on the items path with code
// 3000 ("Request method 'PUT' is not supported"); there is no PATCH route
// either. Callers replace a line by deleting and re-adding it.
func (s *Service) updateInvoiceItem(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	return ResultEnvelope{}, fmt.Errorf("unsupported: Clockify does not expose an update endpoint for invoice line items; delete the line with clockify_invoices_items_delete and re-add it with clockify_invoices_items_add")
}

func (s *Service) deleteInvoiceItem(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	itemIndex, err := invoiceItemIndexArg(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_invoice_item",
			Data: dryrun.MinimalResult("clockify_delete_invoice_item", map[string]any{
				"invoice_id": invoiceID,
				"item_index": itemIndex,
				"item_id":    itemIndex,
			}),
			Meta: map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items", itemIndex)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_invoice_item", map[string]any{
		"deleted":   true,
		"invoiceId": invoiceID,
		"itemIndex": itemIndex,
		"itemId":    itemIndex,
	}, map[string]any{"workspaceId": wsID}), nil
}
