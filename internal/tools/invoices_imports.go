package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/paths"
)

func (s *Service) importInvoiceTimeOneUser(ctx context.Context, args map[string]any) (ToolResult, error) {
	invoiceID, err := requiredIDArg(args, "invoice_id")
	if err != nil {
		return ToolResult{}, err
	}
	body, err := buildInvoiceImportBody(s, args, false)
	if err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items", "import")
	if err != nil {
		return ToolResult{}, err
	}
	var imported map[string]any
	if err := s.Client.Post(ctx, path, body, &imported); err != nil {
		return ToolResult{}, err
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
	primaryGroupBy := strings.TrimSpace(stringArg(args, "time_entry_primary_group_by"))
	if groupType == "GROUPED" && primaryGroupBy == "" {
		return nil, fmt.Errorf("time_entry_group_type \"GROUPED\" requires time_entry_primary_group_by (one of USER, PROJECT, DATE)")
	}
	projectFilter := map[string]any{"contains": "CONTAINS", "status": "ALL"}
	projectIDs, _, err := strictStringSliceArg(args, "project_ids")
	if err != nil {
		return nil, err
	}
	if len(projectIDs) > 0 {
		projectFilter["ids"] = projectIDs
	}
	body := map[string]any{
		"from":               fromTime.UTC().Format(time.RFC3339),
		"to":                 toTime.UTC().Format(time.RFC3339),
		"importExpenses":     importExpenses,
		"timeEntryGroupType": groupType,
		"projectFilter":      projectFilter,
	}
	if primaryGroupBy != "" {
		body["timeEntryPrimaryGroupBy"] = primaryGroupBy
	}
	return body, nil
}

func (s *Service) importInvoiceExpensesOneUser(ctx context.Context, args map[string]any) (ToolResult, error) {
	invoiceID, err := requiredIDArg(args, "invoice_id")
	if err != nil {
		return ToolResult{}, err
	}
	body, err := buildInvoiceImportBody(s, args, true)
	if err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	path, err := paths.Workspace(wsID, "invoices", invoiceID, "items", "import")
	if err != nil {
		return ToolResult{}, err
	}
	var imported map[string]any
	if err := s.Client.Post(ctx, path, body, &imported); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_invoices_import_expenses", invoiceItemViewFromRaw(imported, ""), map[string]any{
		"workspaceId": wsID,
		"invoiceId":   invoiceID,
		"from":        body["from"],
		"to":          body["to"],
	}), nil
}
