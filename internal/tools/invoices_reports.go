package tools

import (
	"context"

	"github.com/apet97/go-clockify/internal/paths"
)

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
	return ok("clockify_invoices_info", compactInvoiceViewsFromRaw(envelope.Invoices), emptyListMeta(meta, "clockify_invoice_client_work")), nil
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
