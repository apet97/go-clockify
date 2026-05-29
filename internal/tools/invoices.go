package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func invoiceHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List invoices
		{Tool: withOutputSchema(toolRO("clockify_list_invoices", "List invoices in the workspace with pagination", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
				"status":    invoiceStatusSchema("Filter by live invoice status"),
			},
		}), envelopeSchemaFor[[]CompactInvoiceView]("clockify_list_invoices")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listInvoices(ctx, args)
		}},

		// 2. Get invoice
		{Tool: withOutputSchema(toolRO("clockify_get_invoice", "Get a single invoice by ID", map[string]any{
			"type":       "object",
			"required":   []string{"invoice_id"},
			"properties": map[string]any{"invoice_id": map[string]any{"type": "string"}},
		}), envelopeSchemaFor[InvoiceView]("clockify_get_invoice")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.getInvoice(ctx, args)
		}},

		// 3. Export invoice
		{Tool: toolRO("clockify_export_invoice", "Export an invoice. CSV keeps the safe base64 envelope; PDF/XLSX/ZIP return contentType, filename, bytes, bodyEncoding:\"file\", and a local path. Defaults user_locale to en-US.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id":  map[string]any{"type": "string"},
				"user_locale": map[string]any{"type": "string", "description": "Required by live Clockify export API; defaults to en-US"},
			},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.exportInvoice(ctx, args)
		}},

		// 4. Create invoice
		{Tool: toolRW("clockify_create_invoice", "Create a new invoice for a client. Supports dry_run:true.", map[string]any{
			"type":     "object",
			"required": []string{"client_id", "number", "issued_date", "currency", "due_date"},
			"properties": map[string]any{
				"client_id":   map[string]any{"type": "string", "description": "Client ID (required)"},
				"number":      map[string]any{"type": "string", "description": "Invoice number (required by live API)"},
				"issued_date": map[string]any{"type": "string", "description": "Issued date (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
				"currency":    map[string]any{"type": "string", "description": "Currency code (required; e.g. USD, EUR)"},
				"due_date":    map[string]any{"type": "string", "description": "Due date (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
				"note":        map[string]any{"type": "string", "description": "Invoice note"},
				"subject":     map[string]any{"type": "string", "description": "Invoice subject"},
				"bill_from":   map[string]any{"type": "string", "description": "Invoice bill-from label/address"},
				"client_address": map[string]any{
					"type":        "string",
					"description": "Client address text to print on the invoice",
				},
				"tax_percent":      map[string]any{"type": "number", "description": "Invoice tax percent"},
				"tax2_percent":     map[string]any{"type": "number", "description": "Second invoice tax percent"},
				"discount_percent": map[string]any{"type": "number", "description": "Invoice discount percent"},
				"tax_type":         map[string]any{"type": "string", "enum": []string{"COMPOUND", "SIMPLE", "NONE"}, "description": "Invoice tax calculation type"},
				"dry_run":          map[string]any{"type": "boolean", "description": "Preview the invoice payload without creating it"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.createInvoice(ctx, args)
		}},

		// 5. Update invoice
		{Tool: toolRW("clockify_update_invoice", "Update an existing invoice. Status changes use Clockify's live PATCH status route. Supports dry_run:true.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id":  map[string]any{"type": "string"},
				"client_id":   map[string]any{"type": "string"},
				"number":      map[string]any{"type": "string"},
				"issued_date": map[string]any{"type": "string", "description": "Issued date (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
				"currency":    map[string]any{"type": "string"},
				"due_date":    map[string]any{"type": "string", "description": "Due date (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
				"note":        map[string]any{"type": "string"},
				"subject":     map[string]any{"type": "string", "description": "Invoice subject"},
				"bill_from":   map[string]any{"type": "string", "description": "Invoice bill-from label/address"},
				"client_address": map[string]any{
					"type":        "string",
					"description": "Client address text to print on the invoice",
				},
				"tax_percent": map[string]any{"type": "number", "description": "Invoice tax percent"},
				"tax2_percent": map[string]any{
					"type":        "number",
					"description": "Second invoice tax percent",
				},
				"discount_percent": map[string]any{"type": "number", "description": "Invoice discount percent"},
				"tax_type":         map[string]any{"type": "string", "enum": []string{"COMPOUND", "SIMPLE", "NONE"}, "description": "Invoice tax calculation type"},
				"status":           invoiceStatusSchema("Invoice status. DRAFT is not a live value; use UNSENT for draft-like invoices."),
				"dry_run":          map[string]any{"type": "boolean", "description": "Preview the invoice update payload without applying it"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.updateInvoice(ctx, args)
		}},

		// 6. Delete invoice
		{Tool: toolDestructive("clockify_delete_invoice", "Delete an invoice permanently by ID. Supports dry_run preview.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.deleteInvoice(ctx, args)
		}},

		// 7. Send invoice
		{Tool: toolRW("clockify_send_invoice", "Explain that this Clockify API surface does not expose an invoice send-email endpoint; no destructive or external side effect occurs and the Clockify UI must send email delivery.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return nil, s.sendInvoice(ctx, args)
		}},

		// 8. Mark invoice paid
		{Tool: toolRW("clockify_mark_invoice_paid", "Check whether an invoice is already paid. If not, returns recovery guidance to create a payment with clockify_invoices_payments_create.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean", "description": "Preview only; returns the current invoice without creating a payment"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.markInvoicePaid(ctx, args)
		}},

		// 9. Get invoice settings
		{Tool: withOutputSchema(toolRO("clockify_get_invoice_settings", "Get workspace invoice settings, labels, defaults, and export fields", map[string]any{
			"type": "object",
		}), envelopeSchemaFor[InvoiceSettingsView]("clockify_get_invoice_settings")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.getInvoiceSettings(ctx, args)
		}},

		// 10. List invoice payments
		{Tool: withOutputSchema(toolRO("clockify_list_invoice_payments", "List payments recorded against an invoice", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"page":       map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size":  map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), envelopeSchemaFor[[]InvoicePaymentView]("clockify_list_invoice_payments")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listInvoicePayments(ctx, args)
		}},

		// 11. List invoice items
		{Tool: withOutputSchema(toolRO("clockify_list_invoice_items", "List the line items on an invoice, paginated via page and page_size.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
			},
		}), envelopeSchemaFor[[]InvoiceItemView]("clockify_list_invoice_items")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listInvoiceItems(ctx, args)
		}},

		// 11b. Unbilled for client
		{Tool: withOutputSchema(toolRO("clockify_unbilled_for_client", "List uninvoiced detailed-report entries and money totals for one client", unbilledForClientInputSchema()), envelopeSchemaFor[UnbilledForClientView]("clockify_unbilled_for_client")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.UnbilledForClient(ctx, args)
		}},

		// 12. Add invoice item
		{Tool: toolRW("clockify_add_invoice_item", "Add an item to an invoice. unit_price is sent to Clockify in minor units (cents) by default; pass unit_price_unit:\"major\" to enter the value in major currency units and let the MCP multiply by 100 before the POST.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id", "item_type"},
			"properties": map[string]any{
				"invoice_id":      map[string]any{"type": "string"},
				"description":     map[string]any{"type": "string"},
				"quantity":        map[string]any{"type": "number"},
				"unit_price":      map[string]any{"type": "number", "description": "Unit price. Defaults to minor units/cents (e.g. 12500 for $125.00); pass unit_price_unit:\"major\" to send major currency units instead."},
				"unit_price_unit": invoiceMinorUnitSchema("unit_price"),
				"apply_taxes":     map[string]any{"type": "string", "enum": []string{"TAX1", "TAX2", "TAX1TAX2", "NONE"}, "description": "Tax application enum; defaults to NONE"},
				"item_type":       map[string]any{"type": "string", "description": "Invoice item type name as configured in the workspace's invoice settings (required by the Clockify API)."},
				"dry_run":         map[string]any{"type": "boolean", "description": "Preview the invoice item request without making changes"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.addInvoiceItem(ctx, args)
		}},

		// 13. Update invoice item (unsupported by Clockify API)
		{Tool: toolRW("clockify_update_invoice_item", "Explain that Clockify does not expose an update endpoint for invoice line items; delete the line with clockify_invoices_items_delete and re-add it with clockify_invoices_items_add.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.updateInvoiceItem(ctx, args)
		}},

		// 14. Delete invoice item
		{Tool: toolDestructive("clockify_delete_invoice_item", "Permanently delete an invoice item by line index. Billing impact; destructive; supports dry_run preview.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"item_index": map[string]any{"type": "string", "description": "Invoice line order (live API path parameter), not a Mongo-style id from list_invoice_items"},
				"item_id":    map[string]any{"type": "string", "description": "Deprecated alias for item_index; accepts invoice line order, not a Mongo-style id"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.deleteInvoiceItem(ctx, args)
		}},

		// 15. Invoice report
		{Tool: withOutputSchema(toolRO("clockify_invoice_report", "Get invoices filtered by status with page-scoped totals", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":    invoiceStatusSchema("Filter by live invoice status"),
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), envelopeSchemaFor[InvoiceReportView]("clockify_invoice_report")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.invoiceReport(ctx, args)
		}},
	}
}

var liveInvoiceStatuses = map[string]bool{
	"UNSENT":         true,
	"SENT":           true,
	"PAID":           true,
	"PARTIALLY_PAID": true,
	"VOID":           true,
	"OVERDUE":        true,
}

func invoiceStatusSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"enum":        []string{"UNSENT", "SENT", "PAID", "PARTIALLY_PAID", "VOID", "OVERDUE"},
	}
}

func normalizeInvoiceStatus(raw string) (string, error) {
	status := strings.ToUpper(strings.TrimSpace(raw))
	if status == "" {
		return "", nil
	}
	if status == "DRAFT" {
		return "", fmt.Errorf("invoice status DRAFT is rejected by live Clockify; use UNSENT for draft-like invoices")
	}
	if !liveInvoiceStatuses[status] {
		return "", fmt.Errorf("invoice status must be one of UNSENT, SENT, PAID, PARTIALLY_PAID, VOID, OVERDUE; got %q", raw)
	}
	return status, nil
}

// ---------------------------------------------------------------------------
// Invoice handlers
// ---------------------------------------------------------------------------

func (s *Service) listInvoices(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	path, err := paths.Workspace(wsID, "invoices")
	if err != nil {
		return ToolResult{}, err
	}
	// Upstream returns {total: int, invoices: [...]} — discovered via
	// clockify-api-probe-lab against the live workspace 2026-05-02.
	var envelope struct {
		Total    int              `json:"total"`
		Invoices []map[string]any `json:"invoices"`
	}
	query := map[string]string{
		"page":      fmt.Sprintf("%d", page),
		"page-size": fmt.Sprintf("%d", pageSize),
	}
	// Wire param is `statuses` (plural) — verified by clockify-api-probe-lab.
	// See PR #53 SUMMARY #10. The arg name `status` matches invoice_report.
	if v := stringArg(args, "status"); v != "" {
		status, err := normalizeInvoiceStatus(v)
		if err != nil {
			return ToolResult{}, err
		}
		query["statuses"] = status
	}
	if err := s.Client.Get(ctx, path, query, &envelope); err != nil {
		return ToolResult{}, err
	}
	// has_more from the known total; the page-full heuristic is only a
	// fallback for when the API does not report a usable total, otherwise an
	// exact final page (total == page*pageSize) would falsely report more.
	hasMore := len(envelope.Invoices) == pageSize
	if envelope.Total > 0 {
		hasMore = page*pageSize < envelope.Total
	}
	return ok("clockify_list_invoices", compactInvoiceViewsFromRaw(envelope.Invoices), emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(envelope.Invoices),
		"total":       envelope.Total,
		"page":        page,
		"pageSize":    pageSize,
		"has_more":    hasMore,
	}, "clockify_invoice_client_work")), nil
}

func (s *Service) getInvoice(ctx context.Context, args map[string]any) (ToolResult, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ToolResult{}, err
	}
	var invoice map[string]any
	if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_get_invoice", invoiceViewFromRaw(invoice), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) getInvoiceSettings(ctx context.Context, _ map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", "settings")
	if err != nil {
		return ToolResult{}, err
	}
	var settings map[string]any
	if err := s.Client.Get(ctx, path, nil, &settings); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_get_invoice_settings", invoiceSettingsViewFromRaw(settings), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) createInvoice(ctx context.Context, args map[string]any) (ToolResult, error) {
	clientID := stringArg(args, "client_id")
	if err := resolve.ValidateID(clientID, "client_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	body := map[string]any{"clientId": clientID, "status": "UNSENT"}
	if v := stringArg(args, "number"); v != "" {
		body["number"] = v
	}
	if v := stringArg(args, "issued_date"); v != "" {
		body["issuedDate"] = v
	}
	if v := stringArg(args, "currency"); v != "" {
		body["currency"] = v
	}
	if v := stringArg(args, "due_date"); v != "" {
		body["dueDate"] = v
	}
	if v := stringArg(args, "note"); v != "" {
		body["note"] = v
	}
	if v := stringArg(args, "status"); v != "" {
		body["status"] = v
	}
	applyInvoiceOptionalFields(body, args)
	if dryrun.Enabled(args) {
		return ok("clockify_create_invoice", dryrunPreviewPayload("clockify_create_invoice", body), map[string]any{"workspaceId": wsID}), nil
	}

	path, err := paths.Workspace(wsID, "invoices")
	if err != nil {
		return ToolResult{}, err
	}
	var created map[string]any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_create_invoice", invoiceViewFromRaw(created), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) updateInvoice(ctx context.Context, args map[string]any) (ToolResult, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	body := map[string]any{}
	if v := stringArg(args, "client_id"); v != "" {
		body["clientId"] = v
	}
	if v := stringArg(args, "number"); v != "" {
		body["number"] = v
	}
	if v := stringArg(args, "issued_date"); v != "" {
		body["issuedDate"] = v
	}
	if v := stringArg(args, "currency"); v != "" {
		body["currency"] = v
	}
	if v := stringArg(args, "due_date"); v != "" {
		body["dueDate"] = v
	}
	if v := stringArg(args, "note"); v != "" {
		body["note"] = v
	}
	applyInvoiceOptionalFields(body, args)
	status, hasStatus, err := invoiceStatusFromArgs(args)
	if err != nil {
		return ToolResult{}, err
	}
	if dryrun.Enabled(args) {
		preview := map[string]any{}
		if len(body) > 0 {
			preview["invoice_update"] = body
		}
		if hasStatus {
			preview["status_update"] = map[string]any{"invoiceStatus": status}
		}
		return ok("clockify_update_invoice", dryrunPreviewPayload("clockify_update_invoice", preview), map[string]any{"workspaceId": wsID, "invoiceId": invoiceID}), nil
	}
	if len(body) == 0 && !hasStatus {
		return ToolResult{}, fmt.Errorf("at least one field (client_id, number, issued_date, currency, due_date, note, subject, bill_from, client_address, tax_percent, tax2_percent, discount_percent, tax_type, status) must be provided for update")
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ToolResult{}, err
	}
	updated := map[string]any{"id": invoiceID}
	if len(body) > 0 {
		var existing map[string]any
		if err := s.Client.Get(ctx, path, nil, &existing); err != nil {
			return ToolResult{}, err
		}
		merged := invoiceUpdateBodyFromExisting(existing)
		for key, value := range body {
			merged[key] = value
		}
		if err := s.Client.Put(ctx, path, merged, &updated); err != nil {
			return ToolResult{}, err
		}
	}
	if hasStatus {
		if err := s.patchInvoiceStatus(ctx, wsID, invoiceID, status); err != nil {
			return ToolResult{}, err
		}
		updated["status"] = status
	}
	return ok("clockify_update_invoice", invoiceViewFromRaw(updated), map[string]any{"workspaceId": wsID}), nil
}

func applyInvoiceOptionalFields(body map[string]any, args map[string]any) {
	if v := stringArg(args, "subject"); v != "" {
		body["subject"] = v
	}
	if v := stringArg(args, "bill_from"); v != "" {
		body["billFrom"] = v
	}
	if v := stringArg(args, "client_address"); v != "" {
		body["clientAddress"] = v
	}
	if v, ok := numberArg(args, "tax_percent"); ok {
		body["taxPercent"] = v
	}
	if v, ok := numberArg(args, "tax2_percent"); ok {
		body["tax2Percent"] = v
	}
	if v, ok := numberArg(args, "discount_percent"); ok {
		body["discountPercent"] = v
	}
	if v := stringArg(args, "tax_type"); v != "" {
		body["taxType"] = v
	}
}

func invoiceUpdateBodyFromExisting(existing map[string]any) map[string]any {
	body := map[string]any{}
	copyString := func(dst string, keys ...string) {
		for _, key := range keys {
			if value := firstReportString(existing, key); value != "" {
				body[dst] = value
				return
			}
		}
	}
	copyNumber := func(dst string, keys ...string) {
		for _, key := range keys {
			if value, ok := reportNumber(existing[key]); ok {
				body[dst] = value
				return
			}
		}
	}

	copyString("clientId", "clientId", "client_id")
	copyString("companyId", "companyId", "company_id")
	copyString("currency", "currency")
	copyString("dueDate", "dueDate", "due_date")
	copyString("issuedDate", "issuedDate", "issued_date")
	copyString("billFrom", "billFrom", "bill_from")
	copyString("clientAddress", "clientAddress", "client_address")
	copyString("note", "note")
	copyString("number", "number")
	copyString("subject", "subject")
	copyString("taxType", "taxType", "tax_type")
	copyNumber("discountPercent", "discountPercent", "discount_percent")
	copyNumber("tax2Percent", "tax2Percent", "tax2_percent")
	copyNumber("taxPercent", "taxPercent", "tax_percent")
	switch visible := firstPresent(existing, "visibleZeroFields", "visible_zero_fields").(type) {
	case string:
		body["visibleZeroFields"] = visible
	case []any:
		body["visibleZeroFields"] = visible
	case []string:
		body["visibleZeroFields"] = visible
	}
	return body
}

func invoiceStatusFromArgs(args map[string]any) (string, bool, error) {
	raw := strings.TrimSpace(stringArg(args, "status"))
	if raw == "" {
		return "", false, nil
	}
	status, err := normalizeInvoiceStatus(raw)
	return status, true, err
}

func (s *Service) deleteInvoice(ctx context.Context, args map[string]any) (ToolResult, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		var invoice map[string]any
		if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{
			OK:     true,
			Action: "clockify_delete_invoice",
			Data:   dryrun.WrapResult(invoice, "clockify_delete_invoice"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, path); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_delete_invoice", map[string]any{
		"deleted":   true,
		"invoiceId": invoiceID,
	}, map[string]any{"workspaceId": wsID}), nil
}

// sendInvoice is an error-only stub for clockify_send_invoice: the public
// Clockify API has no send-email endpoint, so every call returns the same
// "unsupported" error after validating invoice_id. Returning only error
// makes the dead success path explicit at the dispatcher.
func (s *Service) sendInvoice(_ context.Context, args map[string]any) error {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return err
	}
	return fmt.Errorf("unsupported: Clockify does not expose an invoice send-email endpoint in this API surface; send invoice email from the Clockify UI")
}

func (s *Service) markInvoicePaid(ctx context.Context, args map[string]any) (ToolResult, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		var invoice map[string]any
		if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{
			OK:     true,
			Action: "clockify_mark_invoice_paid",
			Data:   dryrun.WrapResult(invoice, "clockify_mark_invoice_paid"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	var invoice map[string]any
	if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
		return ToolResult{}, err
	}
	status := strings.ToUpper(firstReportString(invoice, "status", "invoiceStatus", "invoice_status"))
	if status == "PAID" {
		return ok("clockify_mark_invoice_paid", invoiceViewFromRaw(invoice), map[string]any{"workspaceId": wsID, "invoiceId": invoiceID}), nil
	}
	return ToolResult{}, fmt.Errorf("invoice %s is not paid yet; create a payment with clockify_invoices_payments_create using invoice_id, amount, and date, then reload the invoice", invoiceID)
}

func (s *Service) patchInvoiceStatus(ctx context.Context, wsID, invoiceID, status string) error {
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "status")
	if err != nil {
		return err
	}
	return s.Client.Patch(ctx, path, map[string]any{"invoiceStatus": status}, nil)
}
