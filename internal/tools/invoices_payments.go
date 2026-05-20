package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

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

func normalizeInvoicePaymentDate(raw string) string {
	if raw == "" || strings.Contains(raw, "T") {
		return raw
	}
	return raw + "T00:00:00Z"
}

func (s *Service) createInvoicePaymentOneUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	invoiceID, err := requiredIDArg(args, "invoice_id")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := requirePresentArgs(args, "amount", "date"); err != nil {
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
	body["paymentDate"] = normalizeInvoicePaymentDate(strings.TrimSpace(stringArg(args, "date")))
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
	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_invoices_payments_delete",
			Data: dryrun.MinimalResult("clockify_invoices_payments_delete", map[string]any{
				"invoice_id": invoiceID,
				"payment_id": paymentID,
			}),
			Meta: map[string]any{"workspaceId": wsID, "invoiceId": invoiceID, "paymentId": paymentID},
		}, nil
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
