package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

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
		switch strings.ToUpper(format) {
		case "PDF":
			query.Set("format", strings.ToUpper(format))
		default:
			return ResultEnvelope{}, fmt.Errorf("format must be PDF; Clockify's invoice export endpoint does not produce CSV/XLSX (got %q)", format)
		}
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
