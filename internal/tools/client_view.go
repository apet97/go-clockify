package tools

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
)

type ClientCurrencyView struct {
	Code string `json:"code,omitempty"`
	ID   string `json:"id,omitempty"`
}

type ClientContactView struct {
	Email    string `json:"email,omitempty"`
	Address  string `json:"address,omitempty"`
	Note     string `json:"note,omitempty"`
	CCEmails any    `json:"ccEmails,omitempty"`
}

type ClientProjectSummary struct {
	Count         int  `json:"count"`
	ActiveCount   int  `json:"active_count,omitempty"`
	ArchivedCount int  `json:"archived_count,omitempty"`
	BillableCount int  `json:"billable_count,omitempty"`
	TasksCount    int  `json:"tasks_count,omitempty"`
	Truncated     bool `json:"truncated,omitempty"`
}

type ClientTimeSummary struct {
	TrackedDurationSeconds int64   `json:"tracked_duration_seconds,omitempty"`
	TrackedHours           float64 `json:"tracked_hours,omitempty"`
	TrackedDisplay         string  `json:"tracked_display,omitempty"`
	EntriesCount           int     `json:"entries_count,omitempty"`
	Source                 string  `json:"source"`
	Reason                 string  `json:"reason,omitempty"`
}

type ClientInvoiceSummary struct {
	TotalCount    int            `json:"total_count,omitempty"`
	ReturnedCount int            `json:"returned_count,omitempty"`
	Amount        *MoneyView     `json:"amount,omitempty"`
	Balance       *MoneyView     `json:"balance,omitempty"`
	Paid          *MoneyView     `json:"paid,omitempty"`
	OverdueCount  int            `json:"overdue_count,omitempty"`
	ByStatus      map[string]int `json:"by_status,omitempty"`
	Source        string         `json:"source"`
	Reason        string         `json:"reason,omitempty"`
}

type ClientApprovalSummary struct {
	Source                 string                       `json:"source"`
	Reason                 string                       `json:"reason,omitempty"`
	WithApprovalRequest    int                          `json:"with_approval_request,omitempty"`
	WithoutApprovalRequest int                          `json:"without_approval_request,omitempty"`
	RequestIDs             []string                     `json:"request_ids,omitempty"`
	States                 map[string]int               `json:"states,omitempty"`
	DurationsByState       map[string]EntryDurationView `json:"durations_by_state,omitempty"`
}

type ClientView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address,omitempty"`
	Archived     bool   `json:"archived,omitempty"`
	CCEmails     any    `json:"ccEmails,omitempty"`
	CurrencyCode string `json:"currencyCode,omitempty"`
	CurrencyID   string `json:"currencyId,omitempty"`
	Email        string `json:"email,omitempty"`
	Note         string `json:"note,omitempty"`
	WorkspaceID  string `json:"workspaceId,omitempty"`

	Currency        ClientCurrencyView    `json:"currency"`
	Contact         ClientContactView     `json:"contact"`
	ProjectSummary  ClientProjectSummary  `json:"project_summary"`
	Financials      ProjectFinancials     `json:"financials"`
	TimeSummary     ClientTimeSummary     `json:"time_summary"`
	InvoiceSummary  ClientInvoiceSummary  `json:"invoice_summary"`
	ApprovalSummary ClientApprovalSummary `json:"approval_summary"`
	Warnings        []string              `json:"warnings,omitempty"`
	Raw             map[string]any        `json:"raw,omitempty"`
}

type clientReportTotals struct {
	money        reportEntryMoney
	duration     int64
	hasDuration  bool
	entriesCount int
}

func clientViewFromClient(client clockify.ClientEntity) ClientView {
	view := ClientView{
		ID:           client.ID,
		Name:         client.Name,
		Address:      client.Address,
		Archived:     client.Archived,
		CCEmails:     client.CCEmails,
		CurrencyCode: client.CurrencyCode,
		CurrencyID:   client.CurrencyID,
		Email:        client.Email,
		Note:         client.Note,
		WorkspaceID:  client.WorkspaceID,
		Currency: ClientCurrencyView{
			Code: client.CurrencyCode,
			ID:   client.CurrencyID,
		},
		Contact: ClientContactView{
			Email:    client.Email,
			Address:  client.Address,
			Note:     client.Note,
			CCEmails: client.CCEmails,
		},
		Financials: ProjectFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "financial enrichment was not available",
		},
		TimeSummary: ClientTimeSummary{
			Source: entryFinancialSourceUnavailable,
			Reason: "tracked-time enrichment was not available",
		},
		InvoiceSummary: ClientInvoiceSummary{
			Source: entryFinancialSourceUnavailable,
			Reason: "invoice enrichment is only loaded by clockify_reports_detailed",
		},
		ApprovalSummary: ClientApprovalSummary{
			Source: entryFinancialSourceUnavailable,
			Reason: "approval enrichment requires detailed report rows",
		},
		Raw: clientRawMap(client),
	}
	return view
}

func clientRawMap(client clockify.ClientEntity) map[string]any {
	raw := map[string]any{
		"id":       client.ID,
		"name":     client.Name,
		"archived": client.Archived,
	}
	if client.Address != "" {
		raw["address"] = client.Address
	}
	if client.CCEmails != nil {
		raw["ccEmails"] = client.CCEmails
	}
	if client.CurrencyCode != "" {
		raw["currencyCode"] = client.CurrencyCode
	}
	if client.CurrencyID != "" {
		raw["currencyId"] = client.CurrencyID
	}
	if client.Email != "" {
		raw["email"] = client.Email
	}
	if client.Note != "" {
		raw["note"] = client.Note
	}
	if client.WorkspaceID != "" {
		raw["workspaceId"] = client.WorkspaceID
	}
	return raw
}

func (s *Service) enrichClientView(ctx context.Context, wsID string, client clockify.ClientEntity, args map[string]any, deep bool) (ClientView, map[string]any) {
	views, meta := s.enrichClientViews(ctx, wsID, []clockify.ClientEntity{client}, args, deep)
	if len(views) == 0 {
		return clientViewFromClient(client), meta
	}
	return views[0], meta
}

func (s *Service) enrichClientViews(ctx context.Context, wsID string, clients []clockify.ClientEntity, args map[string]any, deep bool) ([]ClientView, map[string]any) {
	views := make([]ClientView, len(clients))
	clientIDs := make([]string, 0, len(clients))
	for i := range clients {
		views[i] = clientViewFromClient(clients[i])
		if clients[i].ID != "" {
			clientIDs = append(clientIDs, clients[i].ID)
		}
	}
	meta := map[string]any{
		"clients": len(clients),
		"source":  entryFinancialSourceUnavailable,
	}
	if len(clients) == 0 {
		return views, meta
	}
	finRange := financialRangeFromArgs(args)
	for i := range views {
		views[i].Financials.Range = &finRange
	}
	if !s.projectDeepEnrichmentEnabled() {
		addClientWarningAll(views, "deep client enrichment disabled for non-canonical Clockify base URL")
		meta["reason"] = "deep client enrichment disabled for non-canonical Clockify base URL"
		return views, meta
	}

	projects, truncated, projectErr := s.listProjectsForClientEnrichment(ctx, wsID, clientIDs)
	if projectErr != nil {
		addClientWarningAll(views, "project enrichment failed: "+projectErr.Error())
		meta["project_enrichment_error"] = projectErr.Error()
	} else {
		if truncated {
			meta["projects_truncated"] = true
		}
		applyClientProjectSummaries(views, projects, truncated)
	}

	totals, reportErr := s.reportFinancialsForClients(ctx, wsID, clientIDs, finRange)
	reportMatched := 0
	for i := range views {
		total, ok := totals[views[i].ID]
		if !ok {
			continue
		}
		if total.money.hasMoney() {
			views[i].Financials = projectFinancialsFromReport(total.money, finRange)
			reportMatched++
		}
		if total.hasDuration || total.entriesCount > 0 {
			views[i].TimeSummary = ClientTimeSummary{
				TrackedDurationSeconds: total.duration,
				TrackedHours:           hours(total.duration),
				TrackedDisplay:         durationDisplay(total.duration),
				EntriesCount:           total.entriesCount,
				Source:                 entryFinancialSourceReportsAPI,
			}
		}
	}
	meta["reports_api_matched"] = reportMatched
	if reportMatched > 0 {
		meta["source"] = entryFinancialSourceReportsAPI
	}
	if reportErr != nil {
		addClientWarningAll(views, "reports API enrichment failed: "+reportErr.Error())
		meta["reports_api_error"] = reportErr.Error()
	}
	if deep {
		for i := range views {
			invoices, err := s.invoiceSummaryForClient(ctx, wsID, views[i].ID, args)
			if err != nil {
				views[i].Warnings = append(views[i].Warnings, "invoice enrichment failed: "+err.Error())
			} else {
				views[i].InvoiceSummary = invoices
			}
			detailed, err := s.detailedReportViewsForClient(ctx, wsID, views[i].ID, finRange)
			if err != nil {
				views[i].Warnings = append(views[i].Warnings, "approval/detail enrichment failed: "+err.Error())
			} else {
				views[i].ApprovalSummary = clientApprovalSummaryFromReport(detailed.ApprovalSummary)
			}
		}
	}
	return views, meta
}

func addClientWarningAll(views []ClientView, warning string) {
	for i := range views {
		views[i].Warnings = append(views[i].Warnings, warning)
	}
}

func (s *Service) listProjectsForClientEnrichment(ctx context.Context, wsID string, clientIDs []string) ([]clockify.Project, bool, error) {
	if len(clientIDs) == 0 {
		return nil, false, nil
	}
	path, err := paths.Workspace(wsID, "projects")
	if err != nil {
		return nil, false, err
	}
	values := url.Values{}
	values.Set("page", "1")
	values.Set("page-size", strconv.Itoa(projectTaskEnrichmentPageSize))
	values.Set("contains-client", "true")
	values.Set("client-status", "ALL")
	values.Set("hydrated", "true")
	for _, id := range clientIDs {
		values.Add("clients", id)
	}
	var projects []clockify.Project
	if err := s.Client.GetValues(ctx, path, values, &projects); err != nil {
		return nil, false, err
	}
	return projects, len(projects) >= projectTaskEnrichmentPageSize, nil
}

func applyClientProjectSummaries(views []ClientView, projects []clockify.Project, truncated bool) {
	index := map[string]int{}
	for i := range views {
		index[views[i].ID] = i
	}
	for _, project := range projects {
		i, ok := index[project.ClientID]
		if !ok {
			continue
		}
		summary := &views[i].ProjectSummary
		summary.Count++
		if project.Archived {
			summary.ArchivedCount++
		} else {
			summary.ActiveCount++
		}
		if project.Billable {
			summary.BillableCount++
		}
		if truncated {
			summary.Truncated = true
		}
	}
}

func (s *Service) reportFinancialsForClients(ctx context.Context, wsID string, clientIDs []string, finRange FinancialRangeView) (map[string]clientReportTotals, error) {
	out := map[string]clientReportTotals{}
	if len(clientIDs) == 0 {
		return out, nil
	}
	path, err := paths.Workspace(wsID, "reports", "summary")
	if err != nil {
		return out, err
	}
	body := map[string]any{
		"amountShown":   "EARNED",
		"amounts":       []string{"EARNED", "COST", "PROFIT"},
		"dateRangeType": firstNonEmptyString(finRange.DateRangeType, "ABSOLUTE"),
		"clients": map[string]any{
			"contains": "CONTAINS",
			"ids":      clientIDs,
			"status":   "ALL",
		},
		"summaryFilter": map[string]any{
			"groups": []string{"CLIENT", "PROJECT", "TASK"},
		},
	}
	if finRange.Start != "" {
		body["dateRangeStart"] = finRange.Start
	}
	if finRange.End != "" {
		body["dateRangeEnd"] = finRange.End
	}
	if finRange.Timezone != "" {
		body["timeZone"] = finRange.Timezone
	}
	var payload map[string]any
	if err := s.Client.PostReports(ctx, path, body, &payload); err != nil {
		return out, err
	}
	collectClientReportTotals(payload, stringSet(clientIDs), out)
	return out, nil
}

func collectClientReportTotals(v any, clientIDs map[string]struct{}, out map[string]clientReportTotals) {
	switch item := v.(type) {
	case []any:
		for _, child := range item {
			collectClientReportTotals(child, clientIDs, out)
		}
	case []map[string]any:
		for _, child := range item {
			collectClientReportTotals(child, clientIDs, out)
		}
	case map[string]any:
		if id := reportGroupID(item, "client"); id != "" {
			if _, ok := clientIDs[id]; ok {
				out[id] = mergeClientReportTotals(out[id], item)
			}
		}
		for _, child := range item {
			collectClientReportTotals(child, clientIDs, out)
		}
	}
}

func mergeClientReportTotals(existing clientReportTotals, row map[string]any) clientReportTotals {
	existing.money = mergeReportEntryMoney(existing.money, reportMoneyFromRow(row))
	if seconds, ok := reportDurationSeconds(row); ok {
		existing.duration += seconds
		existing.hasDuration = true
	}
	if count, ok := reportInt(firstPresent(row, "entriesCount", "entries_count", "count")); ok {
		existing.entriesCount += count
	}
	return existing
}

func (s *Service) invoiceSummaryForClient(ctx context.Context, wsID, clientID string, args map[string]any) (ClientInvoiceSummary, error) {
	path, err := paths.Workspace(wsID, "invoices", "info")
	if err != nil {
		return ClientInvoiceSummary{}, err
	}
	page := max(intArg(args, "invoice_page", 1), 1)
	pageSize := intArg(args, "invoice_page_size", 50)
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	body := map[string]any{
		"page":     page,
		"pageSize": pageSize,
		"clients": map[string]any{
			"contains": "CONTAINS",
			"ids":      []string{clientID},
			"status":   "ALL",
		},
	}
	var payload map[string]any
	if err := s.Client.Post(ctx, path, body, &payload); err != nil {
		return ClientInvoiceSummary{}, err
	}
	return invoiceSummaryFromPayload(payload), nil
}

func invoiceSummaryFromPayload(payload map[string]any) ClientInvoiceSummary {
	summary := ClientInvoiceSummary{
		Source:   "invoice_api",
		ByStatus: map[string]int{},
	}
	if total, ok := reportInt(firstPresent(payload, "total", "count")); ok {
		summary.TotalCount = total
	}
	for _, invoice := range mapSlice(payload["invoices"]) {
		summary.ReturnedCount++
		currency := reportCurrency(invoice)
		summary.Amount = moneySum(summary.Amount, moneyFromAny(firstPresent(invoice, "amount"), currency))
		summary.Balance = moneySum(summary.Balance, moneyFromAny(firstPresent(invoice, "balance"), currency))
		summary.Paid = moneySum(summary.Paid, moneyFromAny(firstPresent(invoice, "paid"), currency))
		status := strings.ToUpper(firstReportString(invoice, "status"))
		if status != "" {
			summary.ByStatus[status]++
		}
		if days, ok := reportInt(firstPresent(invoice, "daysOverdue", "days_overdue")); ok && days > 0 {
			summary.OverdueCount++
		}
	}
	if len(summary.ByStatus) == 0 {
		summary.ByStatus = nil
	}
	if summary.TotalCount == 0 {
		summary.TotalCount = summary.ReturnedCount
	}
	return summary
}

func (s *Service) detailedReportViewsForClient(ctx context.Context, wsID, clientID string, finRange FinancialRangeView) (detailedReportViews, error) {
	path, err := paths.Workspace(wsID, "reports", "detailed")
	if err != nil {
		return detailedReportViews{}, err
	}
	body := map[string]any{
		"amountShown":   "EARNED",
		"amounts":       []string{"EARNED", "COST", "PROFIT"},
		"dateRangeType": firstNonEmptyString(finRange.DateRangeType, "ABSOLUTE"),
		"clients": map[string]any{
			"contains": "CONTAINS",
			"ids":      []string{clientID},
			"status":   "ALL",
		},
		"detailedFilter": map[string]any{
			"page":     1,
			"pageSize": projectTaskEnrichmentPageSize,
			"options":  map[string]any{"totals": "CALCULATE"},
		},
	}
	if finRange.Start != "" {
		body["dateRangeStart"] = finRange.Start
	}
	if finRange.End != "" {
		body["dateRangeEnd"] = finRange.End
	}
	if finRange.Timezone != "" {
		body["timeZone"] = finRange.Timezone
	}
	var payload map[string]any
	if err := s.Client.PostReports(ctx, path, body, &payload); err != nil {
		return detailedReportViews{}, err
	}
	return normalizeDetailedReportPayload(payload), nil
}

func clientApprovalSummaryFromReport(summary ReportApprovalSummary) ClientApprovalSummary {
	return ClientApprovalSummary{
		Source:                 entryFinancialSourceReportsAPI,
		WithApprovalRequest:    summary.WithApprovalRequest,
		WithoutApprovalRequest: summary.WithoutApprovalRequest,
		RequestIDs:             summary.RequestIDs,
		States:                 summary.States,
		DurationsByState:       summary.DurationsByState,
	}
}
