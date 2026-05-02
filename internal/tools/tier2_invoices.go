package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "invoices",
		Description: "Invoice management — create, send, track payments",
		Keywords:    []string{"invoice", "billing", "payment", "send"},
		ToolNames: []string{
			"clockify_list_invoices",
			"clockify_get_invoice",
			"clockify_create_invoice",
			"clockify_update_invoice",
			"clockify_delete_invoice",
			"clockify_send_invoice",
			"clockify_mark_invoice_paid",
			"clockify_list_invoice_items",
			"clockify_add_invoice_item",
			"clockify_update_invoice_item",
			"clockify_delete_invoice_item",
			"clockify_invoice_report",
		},
		Builder: invoiceHandlers,
	})
}

func invoiceHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List invoices
		{Tool: toolRO("clockify_list_invoices", "List invoices in the workspace with pagination", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listInvoices(ctx, args)
		}},

		// 2. Get invoice
		{Tool: toolRO("clockify_get_invoice", "Get a single invoice by ID", map[string]any{
			"type":       "object",
			"required":   []string{"invoice_id"},
			"properties": map[string]any{"invoice_id": map[string]any{"type": "string"}},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.getInvoice(ctx, args)
		}},

		// 3. Create invoice
		{Tool: toolRW("clockify_create_invoice", "Create a new invoice for a client", map[string]any{
			"type":     "object",
			"required": []string{"client_id"},
			"properties": map[string]any{
				"client_id": map[string]any{"type": "string", "description": "Client ID (required)"},
				"currency":  map[string]any{"type": "string", "description": "Currency code (e.g. USD, EUR)"},
				"due_date":  map[string]any{"type": "string", "description": "Due date (YYYY-MM-DD)"},
				"note":      map[string]any{"type": "string", "description": "Invoice note"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.createInvoice(ctx, args)
		}},

		// 4. Update invoice
		{Tool: toolRW("clockify_update_invoice", "Update an existing invoice", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"client_id":  map[string]any{"type": "string"},
				"currency":   map[string]any{"type": "string"},
				"due_date":   map[string]any{"type": "string"},
				"note":       map[string]any{"type": "string"},
				"status":     map[string]any{"type": "string", "description": "Invoice status"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.updateInvoice(ctx, args)
		}},

		// 5. Delete invoice
		{Tool: toolDestructive("clockify_delete_invoice", "Delete an invoice by ID", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.deleteInvoice(ctx, args)
		}},

		// 6. Send invoice
		{Tool: toolRW("clockify_send_invoice", "Send an invoice to the client", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.sendInvoice(ctx, args)
		}},

		// 7. Mark invoice paid
		{Tool: toolRW("clockify_mark_invoice_paid", "Mark an invoice as paid (sets status=PAID). Supports dry_run:true to preview the invoice that would be updated.", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean", "description": "Preview only; returns the current invoice without updating status"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.markInvoicePaid(ctx, args)
		}},

		// 8. List invoice items
		{Tool: toolRO("clockify_list_invoice_items", "List items for an invoice", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
			},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listInvoiceItems(ctx, args)
		}},

		// 9. Add invoice item
		{Tool: toolRW("clockify_add_invoice_item", "Add an item to an invoice", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id"},
			"properties": map[string]any{
				"invoice_id":  map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				"quantity":    map[string]any{"type": "number"},
				"unit_price":  map[string]any{"type": "number"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.addInvoiceItem(ctx, args)
		}},

		// 10. Update invoice item
		{Tool: toolRW("clockify_update_invoice_item", "Update an invoice item", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id", "item_id"},
			"properties": map[string]any{
				"invoice_id":  map[string]any{"type": "string"},
				"item_id":     map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				"quantity":    map[string]any{"type": "number"},
				"unit_price":  map[string]any{"type": "number"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.updateInvoiceItem(ctx, args)
		}},

		// 11. Delete invoice item
		{Tool: toolDestructive("clockify_delete_invoice_item", "Delete an item from an invoice", map[string]any{
			"type":     "object",
			"required": []string{"invoice_id", "item_id"},
			"properties": map[string]any{
				"invoice_id": map[string]any{"type": "string"},
				"item_id":    map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.deleteInvoiceItem(ctx, args)
		}},

		// 12. Invoice report
		{Tool: toolRO("clockify_invoice_report", "Get invoices filtered by status with aggregated totals", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":    map[string]any{"type": "string", "description": "Filter by status (e.g. PAID, SENT, DRAFT)"},
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.invoiceReport(ctx, args)
		}},
	}
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
	if err := s.Client.Get(ctx, path, map[string]string{
		"page":      fmt.Sprintf("%d", page),
		"page-size": fmt.Sprintf("%d", pageSize),
	}, &envelope); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_invoices", envelope.Invoices, map[string]any{
		"workspaceId": wsID,
		"count":       len(envelope.Invoices),
		"total":       envelope.Total,
		"page":        page,
	}), nil
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
	return ok("clockify_get_invoice", invoice, map[string]any{"workspaceId": wsID}), nil
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

	body := map[string]any{"clientId": clientID}
	if v := stringArg(args, "currency"); v != "" {
		body["currency"] = v
	}
	if v := stringArg(args, "due_date"); v != "" {
		body["dueDate"] = v
	}
	if v := stringArg(args, "note"); v != "" {
		body["note"] = v
	}

	path, err := paths.Workspace(wsID, "invoices")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var created map[string]any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_invoice", created, map[string]any{"workspaceId": wsID}), nil
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

	path, err := paths.Workspace(wsID, "invoices", invoiceID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var updated map[string]any
	if err := s.Client.Put(ctx, path, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_invoice", updated, map[string]any{"workspaceId": wsID}), nil
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

	body := map[string]any{"status": "PAID"}
	var updated map[string]any
	if err := s.Client.Put(ctx, path, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_mark_invoice_paid", updated, map[string]any{"workspaceId": wsID}), nil
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
	invoice, _ := inv.Data.(map[string]any)
	items := []map[string]any{}
	if raw, ok := invoice["items"].([]any); ok {
		for _, r := range raw {
			if item, ok := r.(map[string]any); ok {
				items = append(items, item)
			}
		}
	}
	return ok("clockify_list_invoice_items", items, map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"count":       len(items),
	}), nil
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
		body["unitPrice"] = v
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var created map[string]any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_add_invoice_item", created, map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
	}), nil
}

func (s *Service) updateInvoiceItem(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	itemID := stringArg(args, "item_id")
	if err := resolve.ValidateID(itemID, "item_id"); err != nil {
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
		body["unitPrice"] = v
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items", itemID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var updated map[string]any
	if err := s.Client.Put(ctx, path, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_invoice_item", updated, map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"itemId":      itemID,
	}), nil
}

func (s *Service) deleteInvoiceItem(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return ResultEnvelope{}, err
	}
	itemID := stringArg(args, "item_id")
	if err := resolve.ValidateID(itemID, "item_id"); err != nil {
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
				"item_id":    itemID,
			}),
			Meta: map[string]any{"workspaceId": wsID},
		}, nil
	}

	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items", itemID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_invoice_item", map[string]any{
		"deleted":   true,
		"invoiceId": invoiceID,
		"itemId":    itemID,
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
		body["statuses"] = []string{v}
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

	// Aggregate totals from returned invoices.
	var totalAmount float64
	statusCounts := map[string]int{}
	for _, inv := range envelope.Invoices {
		if amt, ok := inv["amount"].(float64); ok {
			totalAmount += amt
		}
		if st, ok := inv["status"].(string); ok {
			statusCounts[st]++
		}
	}

	return ok("clockify_invoice_report", map[string]any{
		"invoices":     envelope.Invoices,
		"totalAmount":  totalAmount,
		"statusCounts": statusCounts,
	}, map[string]any{
		"workspaceId": wsID,
		"count":       len(envelope.Invoices),
		"total":       envelope.Total,
		"page":        page,
	}), nil
}
