package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

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
		}), envelopeSchemaFor[[]InvoiceView]("clockify_list_invoices")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
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
		{Tool: toolRO("clockify_export_invoice", "Export an invoice and return the safe binary envelope: contentType, filename, bytes, bodyEncoding:\"base64\", base64Bytes, truncated:false, and body with the base64 payload. Defaults user_locale to en-US.", map[string]any{
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
		{Tool: toolRW("clockify_send_invoice", "Send the invoice email to the client. External side effect; dry_run previews without sending.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.sendInvoice(ctx, args)
		}},

		// 8. Mark invoice paid
		{Tool: toolRW("clockify_mark_invoice_paid", "Mark an invoice as paid using the live PATCH status route. Supports dry_run:true to preview the invoice that would be updated.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean", "description": "Preview only; returns the current invoice without updating status"},
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
		{Tool: withOutputSchema(toolRO("clockify_list_invoice_items", "List items for an invoice", map[string]any{
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

		// 13. Update invoice item
		{Tool: toolRW("clockify_update_invoice_item", "Update an invoice item by line index. unit_price uses the same unit convention as add_invoice_item: minor units by default, opt into major units via unit_price_unit:\"major\".", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id", "item_type"},
			"properties": map[string]any{
				"invoice_id":      map[string]any{"type": "string"},
				"item_index":      map[string]any{"type": "string", "description": "Invoice line order (live API path parameter), not a Mongo-style id from list_invoice_items"},
				"item_id":         map[string]any{"type": "string", "description": "Deprecated alias for item_index; accepts invoice line order, not a Mongo-style id"},
				"description":     map[string]any{"type": "string"},
				"quantity":        map[string]any{"type": "number"},
				"unit_price":      map[string]any{"type": "number", "description": "Unit price. Defaults to minor units/cents; pass unit_price_unit:\"major\" to send major currency units instead."},
				"unit_price_unit": invoiceMinorUnitSchema("unit_price"),
				"apply_taxes":     map[string]any{"type": "string", "enum": []string{"TAX1", "TAX2", "TAX1TAX2", "NONE"}, "description": "Tax application enum; defaults to NONE"},
				"item_type":       map[string]any{"type": "string", "description": "Workspace invoice item type name (required by live API)"},
				"dry_run":         map[string]any{"type": "boolean", "description": "Preview the invoice item update without making changes"},
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

func (s *Service) listInvoices(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	path, err := paths.Workspace(wsID, "invoices")
	if err != nil {
		return ResultEnvelope{}, err
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
			return ResultEnvelope{}, err
		}
		query["statuses"] = status
	}
	if err := s.Client.Get(ctx, path, query, &envelope); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_invoices", invoiceViewsFromRaw(envelope.Invoices), emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(envelope.Invoices),
		"total":       envelope.Total,
		"page":        page,
	}, "clockify_invoice_client_work")), nil
}

func (s *Service) getInvoice(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var invoice map[string]any
	if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_invoice", invoiceViewFromRaw(invoice), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) getInvoiceSettings(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", "settings")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var settings map[string]any
	if err := s.Client.Get(ctx, path, nil, &settings); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_invoice_settings", invoiceSettingsViewFromRaw(settings), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) listInvoicePayments(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	page, pageSize := paginationFromArgs(args)
	payments, err := s.invoicePaymentViews(ctx, wsID, invoiceID, page, pageSize)
	if err != nil {
		return ResultEnvelope{}, err
	}
	meta := addPaginationMeta(map[string]any{"workspaceId": wsID, "invoiceId": invoiceID, "count": len(payments)}, args, page, pageSize)
	return ok("clockify_list_invoice_payments", payments, meta), nil
}

func (s *Service) invoicePaymentViews(ctx context.Context, wsID, invoiceID string, page, pageSize int) ([]InvoicePaymentView, error) {
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "payments")
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Payments []map[string]any `json:"payments"`
		Items    []map[string]any `json:"items"`
		Data     []map[string]any `json:"data"`
	}
	query := map[string]string{"page": fmt.Sprintf("%d", page), "page-size": fmt.Sprintf("%d", pageSize)}
	if err := s.Client.Get(ctx, path, query, &envelope); err != nil {
		var rows []map[string]any
		if retryErr := s.Client.Get(ctx, path, query, &rows); retryErr != nil {
			return nil, fmt.Errorf("invoice payment fetch failed: %w (retry as a bare array also failed: %v)", err, retryErr)
		}
		return invoicePaymentViewsFromRaw(rows), nil
	}
	rows := envelope.Payments
	if len(rows) == 0 {
		rows = envelope.Items
	}
	if len(rows) == 0 {
		rows = envelope.Data
	}
	return invoicePaymentViewsFromRaw(rows), nil
}

// invoicesInfoSchema is the input schema for clockify_invoices_info.
func invoicesInfoSchema() map[string]any {
	return objectSchema(map[string]any{
		"properties": map[string]any{
			"page":      map[string]any{"type": "integer", "minimum": 1, "description": "Result page, 1-indexed. Default 1."},
			"page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "description": "Invoices per page. Default 50, maximum 200."},
			"statuses": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string", "enum": []string{"UNSENT", "SENT", "PARTIALLY_PAID", "PAID", "VOID", "OVERDUE"}},
				"description": "Invoice statuses to include. Omit to return invoices of every status.",
			},
		},
	})
}

// InvoicesInfo backs clockify_invoices_info: a bulk, paged invoice query via
// POST /invoices/info. Unlike clockify_invoices_list it returns the workspace
// total (filtered by the status filter) so a caller can compute has_more;
// unlike clockify_invoice_report it returns raw invoice rows, not money
// aggregates.
func (s *Service) InvoicesInfo(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	page, pageSize := paginationFromArgs(args)
	statuses, _, err := strictStringSliceArg(args, "statuses")
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", "info")
	if err != nil {
		return ResultEnvelope{}, err
	}
	body := map[string]any{"page": page, "pageSize": pageSize}
	if len(statuses) > 0 {
		body["statuses"] = statuses
	}
	var envelope struct {
		Total    int              `json:"total"`
		Invoices []map[string]any `json:"invoices"`
	}
	if err := s.Client.Post(ctx, path, body, &envelope); err != nil {
		return ResultEnvelope{}, err
	}
	meta := map[string]any{
		"workspaceId": wsID,
		"count":       len(envelope.Invoices),
		"total":       envelope.Total,
		"page":        page,
		"pageSize":    pageSize,
		"has_more":    page*pageSize < envelope.Total,
	}
	return ok("clockify_invoices_info", invoiceViewsFromRaw(envelope.Invoices), emptyListMeta(meta, "clockify_invoice_client_work")), nil
}

func (s *Service) exportInvoice(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "export")
	if err != nil {
		return ResultEnvelope{}, err
	}
	userLocale := strings.TrimSpace(stringArg(args, "user_locale"))
	if userLocale == "" {
		userLocale = "en-US"
	}
	query := url.Values{}
	query.Set("userLocale", userLocale)
	raw, err := s.Client.RequestRawValues(ctx, false, "GET", path, query, nil)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_export_invoice", documentedRawResponse(raw.Header, raw.Body), map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"userLocale":  userLocale,
		"binary":      true,
	}), nil
}

func (s *Service) exportInvoiceOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID, err := requiredIDArg(args, "invoice_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "export")
	if err != nil {
		return ResultEnvelope{}, err
	}
	query := url.Values{}
	if format := strings.TrimSpace(stringArg(args, "format")); format != "" {
		query.Set("format", format)
	}
	userLocale := firstNonEmpty([]string{stringArg(args, "user_locale"), "en-US"})
	query.Set("userLocale", userLocale)
	raw, err := s.Client.RequestRawValues(ctx, false, "GET", path, query, nil)
	if err != nil {
		return ResultEnvelope{}, err
	}
	data := documentedRawResponse(raw.Header, raw.Body)
	if body, ok := data["body"]; ok {
		data["content"] = body
	}
	return ok("clockify_invoices_export", data, map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"format":      strings.TrimSpace(stringArg(args, "format")),
		"userLocale":  userLocale,
		"binary":      true,
	}), nil
}

func (s *Service) importInvoiceTimeOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID, err := requiredIDArg(args, "invoice_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	body, err := buildInvoiceImportBody(s, args, false)
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items", "import")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var imported map[string]any
	if err := s.Client.Post(ctx, path, body, &imported); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_invoices_import_time", invoiceItemViewFromRaw(imported, ""), map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"from":        body["from"],
		"to":          body["to"],
	}), nil
}

// buildInvoiceImportBody assembles the request body for Clockify's invoice
// items-import endpoint. Clockify imports billable time (and optionally
// expenses) by date range scoped to a project filter — it does not accept a
// list of entry IDs. importExpenses selects whether expenses are imported too.
func buildInvoiceImportBody(s *Service, args map[string]any, importExpenses bool) (map[string]any, error) {
	from := strings.TrimSpace(stringArg(args, "from"))
	to := strings.TrimSpace(stringArg(args, "to"))
	if from == "" || to == "" {
		return nil, fmt.Errorf("from and to are required — they bracket the billable items to import (YYYY-MM-DD or RFC3339)")
	}
	loc := s.location()
	fromTime, err := parseFlexibleDateTime(from, loc)
	if err != nil {
		return nil, fmt.Errorf("could not parse date %q for from — use YYYY-MM-DD or RFC3339", from)
	}
	toTime, err := parseFlexibleDateTime(to, loc)
	if err != nil {
		return nil, fmt.Errorf("could not parse date %q for to — use YYYY-MM-DD or RFC3339", to)
	}
	// A date-only `to` (YYYY-MM-DD, no time) means the end of that day, so the
	// whole final day stays inside the window; then require a forward range —
	// a zero or negative window silently imports nothing.
	if !strings.Contains(to, "T") {
		toTime = toTime.Add(24*time.Hour - time.Second)
	}
	if !toTime.After(fromTime) {
		return nil, fmt.Errorf("to (%s) must be after from (%s) — widen the billing window", to, from)
	}
	groupType := strings.TrimSpace(stringArg(args, "time_entry_group_type"))
	if groupType == "" {
		groupType = "DETAILED"
	}
	projectFilter := map[string]any{"contains": "CONTAINS", "status": "ALL"}
	if projectIDs := stringSliceArg(args, "project_ids"); len(projectIDs) > 0 {
		projectFilter["ids"] = projectIDs
	}
	return map[string]any{
		"from":               fromTime.UTC().Format(time.RFC3339),
		"to":                 toTime.UTC().Format(time.RFC3339),
		"importExpenses":     importExpenses,
		"timeEntryGroupType": groupType,
		"projectFilter":      projectFilter,
	}, nil
}

func (s *Service) importInvoiceExpensesOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID, err := requiredIDArg(args, "invoice_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	body, err := buildInvoiceImportBody(s, args, true)
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items", "import")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var imported map[string]any
	if err := s.Client.Post(ctx, path, body, &imported); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_invoices_import_expenses", invoiceItemViewFromRaw(imported, ""), map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"from":        body["from"],
		"to":          body["to"],
	}), nil
}

func (s *Service) createInvoicePaymentOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID, err := requiredIDArg(args, "invoice_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := requirePresentArgs(args, "amount", "date", "note"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "payments")
	if err != nil {
		return ResultEnvelope{}, err
	}
	body := nativeBodyFromArgs(args, "amount", "note")
	if raw, ok := body["amount"]; ok {
		converted, err := convertToInvoiceMinorUnits(raw, stringArg(args, "amount_unit"))
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("amount: %w", err)
		}
		body["amount"] = converted
	}
	body["paymentDate"] = args["date"]
	var created map[string]any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}

	view := invoicePaymentViewFromRaw(created, "")
	if invoicePaymentViewID(view) == "" || invoicePaymentViewID(view) == invoiceID {
		if payments, err := s.invoicePaymentViews(ctx, wsID, invoiceID, 1, 50); err == nil && len(payments) > 0 {
			view = selectInvoicePaymentView(payments, args)
		}
	}
	meta := map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
	}
	if paymentID := invoicePaymentViewID(view); paymentID != "" {
		meta["paymentId"] = paymentID
	}
	return ok("clockify_invoices_payments_create", view, meta), nil
}

func selectInvoicePaymentView(payments []InvoicePaymentView, args map[string]any) InvoicePaymentView {
	note := stringArg(args, "note")
	date := stringArg(args, "date")
	for _, payment := range payments {
		if note != "" && invoicePaymentViewField(payment, "note") == note {
			return payment
		}
		if date != "" && invoicePaymentViewField(payment, "date") == date {
			return payment
		}
	}
	return payments[0]
}

func invoicePaymentViewID(view InvoicePaymentView) string {
	if id := firstReportString(map[string]any(view), "id", "_id", "paymentId", "payment_id"); id != "" {
		return id
	}
	if nested, ok := view["payment"].(map[string]any); ok {
		if id := firstReportString(nested, "id", "_id", "paymentId", "payment_id"); id != "" {
			return id
		}
	}
	if raw, ok := view["raw"].(map[string]any); ok {
		return firstReportString(raw, "id", "_id", "paymentId", "payment_id")
	}
	return ""
}

func invoicePaymentViewField(view InvoicePaymentView, field string) string {
	if nested, ok := view["payment"].(map[string]any); ok {
		if value := firstReportString(nested, field); value != "" {
			return value
		}
	}
	if value := firstReportString(map[string]any(view), field); value != "" {
		return value
	}
	if raw, ok := view["raw"].(map[string]any); ok {
		return firstReportString(raw, field)
	}
	return ""
}

func (s *Service) deleteInvoicePaymentOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID, err := requiredIDArg(args, "invoice_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	paymentID, err := requiredIDArg(args, "payment_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "payments", paymentID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_invoices_payments_delete", map[string]any{
		"deleted":   true,
		"invoiceId": invoiceID,
		"paymentId": paymentID,
	}, map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"paymentId":   paymentID,
	}), nil
}

func (s *Service) createInvoice(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	clientID := stringArg(args, "client_id")
	if err := resolve.ValidateID(clientID, "client_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
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
		return ResultEnvelope{}, err
	}
	var created map[string]any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_invoice", invoiceViewFromRaw(created), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) updateInvoice(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
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
		return ResultEnvelope{}, err
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
		return ResultEnvelope{}, fmt.Errorf("at least one field (client_id, number, issued_date, currency, due_date, note, subject, bill_from, client_address, tax_percent, tax2_percent, discount_percent, tax_type, status) must be provided for update")
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	updated := map[string]any{"id": invoiceID}
	if len(body) > 0 {
		var existing map[string]any
		if err := s.Client.Get(ctx, path, nil, &existing); err != nil {
			return ResultEnvelope{}, err
		}
		merged := invoiceUpdateBodyFromExisting(existing)
		for key, value := range body {
			merged[key] = value
		}
		if err := s.Client.Put(ctx, path, merged, &updated); err != nil {
			return ResultEnvelope{}, err
		}
	}
	if hasStatus {
		if err := s.patchInvoiceStatus(ctx, wsID, invoiceID, status); err != nil {
			return ResultEnvelope{}, err
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

func (s *Service) deleteInvoice(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		var invoice map[string]any
		if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_invoice",
			Data:   dryrun.WrapResult(invoice, "clockify_delete_invoice"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_invoice", map[string]any{
		"deleted":   true,
		"invoiceId": invoiceID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) sendInvoice(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		path, err := paths.Workspace(wsID, "invoices", invoiceID)
		if err != nil {
			return ResultEnvelope{}, err
		}
		var invoice map[string]any
		if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_send_invoice",
			Data:   dryrun.WrapResult(invoice, "clockify_send_invoice"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID, "send")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Post(ctx, path, nil, &result); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_send_invoice", result, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) markInvoicePaid(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		var invoice map[string]any
		if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_mark_invoice_paid",
			Data:   dryrun.WrapResult(invoice, "clockify_mark_invoice_paid"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	var invoice map[string]any
	if err := s.Client.Get(ctx, path, nil, &invoice); err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.patchInvoiceStatus(ctx, wsID, invoiceID, "PAID"); err != nil {
		return ResultEnvelope{}, err
	}
	invoice["status"] = "PAID"
	return ok("clockify_mark_invoice_paid", invoiceViewFromRaw(invoice), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) patchInvoiceStatus(ctx context.Context, wsID, invoiceID, status string) error {
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "status")
	if err != nil {
		return err
	}
	return s.Client.Patch(ctx, path, map[string]any{"invoiceStatus": status}, nil)
}

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
	var asFloat float64
	switch v := value.(type) {
	case float64:
		asFloat = v
	case float32:
		asFloat = float64(v)
	case int:
		asFloat = float64(v)
	case int32:
		asFloat = float64(v)
	case int64:
		asFloat = float64(v)
	default:
		return 0, fmt.Errorf("value must be a number, got %T", value)
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
	if v, ok := args["quantity"]; ok {
		body["quantity"] = v
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

func (s *Service) updateInvoiceItem(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
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

	body := map[string]any{}
	if v := stringArg(args, "description"); v != "" {
		body["description"] = v
	}
	if v, ok := args["quantity"]; ok {
		body["quantity"] = v
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

	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items", itemIndex)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if dryrun.Enabled(args) {
		return ok("clockify_update_invoice_item", dryrunPreviewPayload("clockify_update_invoice_item", body), map[string]any{
			"workspaceId": wsID,
			"invoiceId":   invoiceID,
			"itemIndex":   itemIndex,
			"itemId":      itemIndex,
		}), nil
	}
	var updated map[string]any
	if err := s.Client.Put(ctx, path, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_invoice_item", invoiceItemViewFromRaw(updated, ""), map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"itemIndex":   itemIndex,
		"itemId":      itemIndex,
	}), nil
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

func (s *Service) invoiceReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	// Reporting/filter endpoint is POST /workspaces/{ws}/invoices/info
	// (not GET /invoices, which is the paginated list). Body accepts
	// statuses (array), pagination, plus dateRangeType/dateRange/clientIds.
	// Verified live 2026-05-02 via clockify-api-probe-lab.
	body := map[string]any{
		"page":     page,
		"pageSize": pageSize,
	}
	if v := stringArg(args, "status"); v != "" {
		status, err := normalizeInvoiceStatus(v)
		if err != nil {
			return ResultEnvelope{}, err
		}
		body["statuses"] = []string{status}
	}

	path, err := paths.Workspace(wsID, "invoices", "info")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var envelope struct {
		Total    int              `json:"total"`
		Invoices []map[string]any `json:"invoices"`
	}
	if err := s.Client.Post(ctx, path, body, &envelope); err != nil {
		return ResultEnvelope{}, err
	}

	// Aggregates are page-scoped because the upstream endpoint returns a
	// paginated invoice slice plus the total matching count.
	views := invoiceViewsFromRaw(envelope.Invoices)
	summary := invoiceSummaryFromViews(views)

	return ok("clockify_invoice_report", InvoiceReportView{
		Invoices:         views,
		AggregationScope: "page",
		PageTotalAmount:  summary.Amount,
		PageStatusCounts: summary.ByStatus,
		TotalAmount:      summary.Amount,
		StatusCounts:     summary.ByStatus,
		Summary:          summary,
		Raw:              map[string]any{"invoices": envelope.Invoices, "total": envelope.Total},
	}, map[string]any{
		"workspaceId":      wsID,
		"count":            len(envelope.Invoices),
		"total":            envelope.Total,
		"page":             page,
		"pageSize":         pageSize,
		"aggregationScope": "page",
	}), nil
}
