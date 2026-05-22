package tools

import (
	"context"

	"github.com/apet97/go-clockify/internal/resolve"
)

func unsupportedGuidance(action string, ids map[string]string, hint, recoveryTool string, recoveryArgs map[string]any) ToolError {
	supported := false
	performed := false
	return ToolError{
		OK:        false,
		Supported: &supported,
		Performed: &performed,
		Action:    action,
		IDs:       cleanIDs(ids),
		Error: ErrorInfo{
			Code:    "unsupported",
			Message: hint,
		},
		Recovery: RecoveryHint{
			Hint: hint,
			Tool: recoveryTool,
			Args: recoveryArgs,
		},
	}
}

func (s *Service) sendInvoiceGuidance(_ context.Context, args map[string]any) (any, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return nil, err
	}
	hint := "Clockify does not expose invoice email sending through the public API. Use the Clockify UI for email delivery."
	return unsupportedGuidance(
		"clockify_invoices_send_guidance",
		map[string]string{"invoiceId": invoiceID},
		hint,
		"clockify_invoices_get",
		map[string]any{"invoice_id": invoiceID},
	), nil
}

func (s *Service) updateInvoiceItemGuidance(_ context.Context, args map[string]any) (any, error) {
	invoiceID := stringArg(args, "invoice_id")
	if err := resolve.ValidateID(invoiceID, "invoice_id"); err != nil {
		return nil, err
	}
	hint := "Clockify does not expose an update endpoint for invoice line items. Delete the line with clockify_invoices_items_delete, then re-add it with clockify_invoices_items_add."
	return unsupportedGuidance(
		"clockify_invoices_items_update_guidance",
		map[string]string{"invoiceId": invoiceID},
		hint,
		"clockify_invoices_items_delete",
		map[string]any{"invoice_id": invoiceID},
	), nil
}

func (s *Service) testWebhookGuidance(_ context.Context, args map[string]any) (any, error) {
	webhookID := stringArg(args, "webhook_id")
	if err := resolve.ValidateID(webhookID, "webhook_id"); err != nil {
		return nil, err
	}
	hint := "Clockify does not expose a webhook test-send endpoint through the public API. Trigger a real event or inspect delivery logs in the Clockify UI."
	return unsupportedGuidance(
		"clockify_webhooks_test_guidance",
		map[string]string{"webhookId": webhookID},
		hint,
		"clockify_webhooks_get",
		map[string]any{"webhook_id": webhookID},
	), nil
}
