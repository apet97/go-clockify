package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
)

const (
	entryFinancialSourceReportsAPI   = "reports_api"
	entryFinancialSourceDerivedRates = "derived_rates"
	entryFinancialSourceUnavailable  = "unavailable"
)

// EntryView is the model-facing time-entry envelope used by every tool that
// returns entries. It intentionally keeps the original clockify.TimeEntry JSON
// fields at the top level and appends a financials block so old clients keep
// working while agents can answer money questions without a second tool call.
type EntryView struct {
	ID                string                 `json:"id"`
	Description       string                 `json:"description"`
	ProjectID         string                 `json:"projectId"`
	ProjectName       string                 `json:"projectName,omitempty"`
	TaskID            string                 `json:"taskId,omitempty"`
	TagIDs            []string               `json:"tagIds,omitempty"`
	Billable          bool                   `json:"billable"`
	BillableState     string                 `json:"billable_state"`
	BillablePresent   bool                   `json:"billable_present"`
	CostRate          *clockify.Rate         `json:"costRate,omitempty"`
	CustomFieldValues any                    `json:"customFieldValues,omitempty"`
	CustomFields      []CustomFieldValueView `json:"custom_fields_normalized,omitempty"`
	HourlyRate        *clockify.Rate         `json:"hourlyRate,omitempty"`
	IsLocked          bool                   `json:"isLocked,omitempty"`
	KioskID           string                 `json:"kioskId,omitempty"`
	Type              string                 `json:"type,omitempty"`
	UserID            string                 `json:"userId,omitempty"`
	WorkspaceID       string                 `json:"workspaceId,omitempty"`
	TimeInterval      clockify.TimeInterval  `json:"timeInterval"`
	Financials        EntryFinancials        `json:"financials"`
	Approval          *EntryApprovalView     `json:"approval,omitempty"`
	Invoicing         *EntryInvoicingView    `json:"invoicing,omitempty"`
	Entities          *EntryEntitiesView     `json:"entities,omitempty"`
	Audit             *EntryAuditView        `json:"audit,omitempty"`
}

type EntryFinancials struct {
	Earned *MoneyView     `json:"earned,omitempty"`
	Cost   *MoneyView     `json:"cost,omitempty"`
	Profit *MoneyView     `json:"profit,omitempty"`
	Source string         `json:"source"`
	Reason string         `json:"reason,omitempty"`
	Rate   *EntryRateView `json:"rate,omitempty"`
}

func entryViewFromEntry(entry clockify.TimeEntry) EntryView {
	billablePresent := entry.BillablePresent || entry.Billable
	return EntryView{
		ID:                entry.ID,
		Description:       entry.Description,
		ProjectID:         entry.ProjectID,
		ProjectName:       entry.ProjectName,
		TaskID:            entry.TaskID,
		TagIDs:            entry.TagIDs,
		Billable:          entry.Billable,
		BillableState:     billableStateFromPresence(entry.Billable, billablePresent),
		BillablePresent:   billablePresent,
		CostRate:          entry.CostRate,
		CustomFieldValues: entry.CustomFieldValues,
		CustomFields:      customFieldValuesFromRaw(entry.CustomFieldValues),
		HourlyRate:        entry.HourlyRate,
		IsLocked:          entry.IsLocked,
		KioskID:           entry.KioskID,
		Type:              entry.Type,
		UserID:            entry.UserID,
		WorkspaceID:       entry.WorkspaceID,
		TimeInterval:      entry.TimeInterval,
		Financials: EntryFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "financial enrichment was not available",
		},
	}
}

func (s *Service) enrichEntryView(ctx context.Context, wsID string, entry clockify.TimeEntry) (EntryView, map[string]any) {
	views, meta := s.enrichEntryViews(ctx, wsID, []clockify.TimeEntry{entry})
	if len(views) == 0 {
		return entryViewFromEntry(entry), meta
	}
	return views[0], meta
}

func (s *Service) enrichEntryViews(ctx context.Context, wsID string, entries []clockify.TimeEntry) ([]EntryView, map[string]any) {
	views := make([]EntryView, len(entries))
	for i := range entries {
		views[i] = entryViewFromEntry(entries[i])
	}
	meta := map[string]any{
		"entries": len(entries),
		"source":  entryFinancialSourceUnavailable,
	}
	if len(entries) == 0 {
		meta["unavailable"] = 0
		return views, meta
	}

	reportValues, reportErr := s.reportEnrichmentsForEntries(ctx, wsID, entries)
	reportMatched := 0
	for i := range views {
		if value, ok := reportValues[entries[i].ID]; ok {
			applyReportEntryEnrichment(&views[i], value)
			if value.hasMoney() {
				views[i].Financials = value.financials()
				reportMatched++
			}
		}
	}

	derived := 0
	unavailable := 0
	allowRemoteRateLookup := reportErr == nil
	for i := range views {
		if views[i].Financials.Source == entryFinancialSourceReportsAPI {
			continue
		}
		fin := s.derivedFinancialsForEntry(ctx, wsID, entries[i], allowRemoteRateLookup)
		if fin.Source == entryFinancialSourceDerivedRates {
			views[i].Financials = fin
			derived++
			continue
		}
		if reportErr != nil {
			views[i].Financials.Reason = fmt.Sprintf("reports API unavailable and no usable rate fallback: %v", reportErr)
		}
		unavailable++
	}

	meta["reports_api_matched"] = reportMatched
	meta["derived_rates"] = derived
	meta["unavailable"] = unavailable
	switch {
	case reportMatched == len(entries):
		meta["source"] = entryFinancialSourceReportsAPI
	case reportMatched > 0 || derived > 0:
		meta["source"] = "mixed"
	}
	if reportErr != nil {
		meta["reports_api_error"] = reportErr.Error()
	}
	return views, meta
}

func applyReportEntryEnrichment(view *EntryView, enrichment reportEntryEnrichment) {
	if view == nil {
		return
	}
	reportView := enrichment.view
	if reportView.Description != "" && view.Description == "" {
		view.Description = reportView.Description
	}
	if reportView.Type != "" && view.Type == "" {
		view.Type = reportView.Type
	}
	if reportView.Billable != nil {
		view.Billable = *reportView.Billable
		view.BillablePresent = true
		view.BillableState = billableStateFromPresence(*reportView.Billable, true)
	} else if reportView.BillablePresent {
		view.BillablePresent = true
		view.BillableState = reportView.BillableState
	}
	if reportView.Locked != nil {
		view.IsLocked = *reportView.Locked
	}
	if reportView.CustomFieldValues != nil && view.CustomFieldValues == nil {
		view.CustomFieldValues = reportView.CustomFieldValues
	}
	if len(reportView.CustomFields) > 0 && len(view.CustomFields) == 0 {
		view.CustomFields = reportView.CustomFields
	} else if len(view.CustomFields) == 0 {
		view.CustomFields = customFieldValuesFromRaw(view.CustomFieldValues)
	}
	if reportView.Entities != nil {
		view.Entities = reportView.Entities
		if reportView.Entities.Project != nil {
			view.ProjectID = firstNonEmptyString(view.ProjectID, reportView.Entities.Project.ID)
			view.ProjectName = firstNonEmptyString(view.ProjectName, reportView.Entities.Project.Name)
		}
		if reportView.Entities.Task != nil {
			view.TaskID = firstNonEmptyString(view.TaskID, reportView.Entities.Task.ID)
		}
		if reportView.Entities.User != nil {
			view.UserID = firstNonEmptyString(view.UserID, reportView.Entities.User.ID)
		}
		if len(view.TagIDs) == 0 && len(reportView.Entities.Tags) > 0 {
			for _, tag := range reportView.Entities.Tags {
				if tag.ID != "" {
					view.TagIDs = append(view.TagIDs, tag.ID)
				}
			}
		}
	}
	if reportView.Approval != nil {
		view.Approval = reportView.Approval
	}
	if reportView.Invoicing != nil {
		view.Invoicing = reportView.Invoicing
	}
	if reportView.Audit != nil {
		view.Audit = reportView.Audit
	}
}

func withFinancialMeta(meta map[string]any, financial map[string]any) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	if len(financial) > 0 {
		meta["financials"] = financial
	}
	return meta
}

func (s *Service) reportEnrichmentsForEntries(ctx context.Context, wsID string, entries []clockify.TimeEntry) (map[string]reportEntryEnrichment, error) {
	if s == nil || s.Client == nil || !s.entryFinancialReportsEnabled() {
		return nil, fmt.Errorf("reports API enrichment disabled for non-canonical Clockify base URL")
	}
	start, end, ok := entryRangeBounds(entries)
	if !ok {
		return nil, fmt.Errorf("entries do not have a usable time range")
	}
	if wsID == "" {
		var err error
		wsID, err = s.ResolveWorkspaceID(ctx)
		if err != nil {
			return nil, err
		}
	}
	path, err := paths.Workspace(wsID, "reports", "detailed")
	if err != nil {
		return nil, err
	}
	want := map[string]struct{}{}
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) != "" {
			want[entry.ID] = struct{}{}
		}
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("entries do not have IDs for reports API matching")
	}

	out := make(map[string]reportEntryEnrichment, len(entries))
	const pageSize = 200
	for page := 1; page <= aggregatePageSafetyStop; page++ {
		body := map[string]any{
			"dateRangeStart": start.Format(time.RFC3339),
			"dateRangeEnd":   end.Format(time.RFC3339),
			"dateRangeType":  "ABSOLUTE",
			"amountShown":    "EARNED",
			"amounts":        []string{"EARNED", "COST", "PROFIT"},
			"detailedFilter": map[string]any{
				"page":     page,
				"pageSize": pageSize,
				"options":  map[string]any{"totals": "CALCULATE"},
			},
		}
		var payload map[string]any
		if err := s.Client.PostReports(ctx, path, body, &payload); err != nil {
			return out, err
		}
		rows := reportTimeEntryRows(payload)
		for _, row := range rows {
			id := reportRowEntryID(row)
			if id == "" {
				continue
			}
			if _, ok := want[id]; !ok {
				continue
			}
			view := reportEntryViewFromRow(row)
			out[id] = reportEntryEnrichment{
				money: reportMoneyFromRow(row),
				view:  view,
			}
		}
		if len(out) == len(want) || len(rows) < pageSize {
			return out, nil
		}
	}
	return out, fmt.Errorf("reports API pagination safety stop reached at %d pages", aggregatePageSafetyStop)
}

func (s *Service) entryFinancialReportsEnabled() bool {
	if s == nil || s.Client == nil {
		return false
	}
	return s.EntryFinancialReports || s.Client.ReportsBaseURL() == "https://reports.api.clockify.me/v1"
}

func entryRangeBounds(entries []clockify.TimeEntry) (time.Time, time.Time, bool) {
	var start, end time.Time
	for _, entry := range entries {
		entryStart, err := entry.StartTime()
		if err != nil {
			continue
		}
		entryEnd, err := entry.EndTime()
		if err != nil {
			continue
		}
		if entryEnd.IsZero() {
			entryEnd = time.Now().UTC()
		}
		if entryEnd.Before(entryStart) {
			entryEnd = entryStart
		}
		if start.IsZero() || entryStart.Before(start) {
			start = entryStart
		}
		if end.IsZero() || entryEnd.After(end) {
			end = entryEnd
		}
	}
	if start.IsZero() || end.IsZero() {
		return time.Time{}, time.Time{}, false
	}
	if !end.After(start) {
		end = start.Add(time.Second)
	}
	return start.UTC(), end.UTC(), true
}

type reportEntryMoney struct {
	earned *MoneyView
	cost   *MoneyView
	profit *MoneyView
	reason string
}

type reportEntryEnrichment struct {
	money reportEntryMoney
	view  ReportEntryView
}

func (e reportEntryEnrichment) hasMoney() bool {
	return e.money.hasMoney()
}

func (e reportEntryEnrichment) financials() EntryFinancials {
	return e.money.financials()
}

func (m reportEntryMoney) hasMoney() bool {
	return m.earned != nil || m.cost != nil || m.profit != nil
}

func (m reportEntryMoney) financials() EntryFinancials {
	fin := EntryFinancials{
		Earned: m.earned,
		Cost:   m.cost,
		Profit: m.profit,
		Source: entryFinancialSourceReportsAPI,
		Reason: m.reason,
	}
	if fin.Profit == nil {
		fin.Profit, fin.Reason = profitMoney(fin.Earned, fin.Cost, fin.Reason)
	}
	return fin
}

func reportTimeEntryRows(payload map[string]any) []map[string]any {
	for _, key := range []string{"timeEntries", "timeentries", "entries", "rows"} {
		if rows := mapSlice(payload[key]); len(rows) > 0 {
			return rows
		}
	}
	if data, ok := payload["data"].(map[string]any); ok {
		return reportTimeEntryRows(data)
	}
	return nil
}

func mapSlice(v any) []map[string]any {
	switch items := v.(type) {
	case []map[string]any:
		return items
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func reportRowEntryID(row map[string]any) string {
	for _, key := range []string{"id", "entryId", "timeEntryId", "time_entry_id", "_id", "get_id", "getId"} {
		if id := strings.TrimSpace(fmt.Sprint(row[key])); id != "" && id != "<nil>" {
			return id
		}
	}
	for _, key := range []string{"timeEntry", "entry"} {
		nested, ok := row[key].(map[string]any)
		if !ok {
			continue
		}
		if id := reportRowEntryID(nested); id != "" {
			return id
		}
	}
	return ""
}

func reportMoneyFromRow(row map[string]any) reportEntryMoney {
	money := reportEntryMoney{}
	rowCurrency := reportCurrency(row)
	for _, item := range mapSlice(row["amounts"]) {
		kind := strings.ToUpper(strings.TrimSpace(fmt.Sprint(firstPresent(item, "type", "amountType", "name"))))
		value, ok := reportNumber(firstPresent(item, "value", "amount", "total"))
		if !ok {
			continue
		}
		mv := MoneyFromReportNumber(value, firstNonEmptyString(reportCurrency(item), rowCurrency))
		assignReportMoney(&money, kind, mv)
	}
	assignDirectReportMoney(&money, row, rowCurrency, "EARNED", "earned", "earnedAmount", "earned_amount", "billableAmount", "billable_amount", "billable", "amount", "totalAmount", "total_amount")
	assignDirectReportMoney(&money, row, rowCurrency, "COST", "cost", "costAmount", "cost_amount")
	assignDirectReportMoney(&money, row, rowCurrency, "PROFIT", "profit", "profitAmount", "profit_amount")
	return money
}

func assignDirectReportMoney(money *reportEntryMoney, row map[string]any, currency, kind string, keys ...string) {
	for _, key := range keys {
		value, ok := row[key]
		if !ok {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			currency = firstNonEmptyString(reportCurrency(nested), currency)
			value = firstPresent(nested, "value", "amount", "total")
		}
		n, ok := reportNumber(value)
		if !ok {
			continue
		}
		assignReportMoney(money, kind, MoneyFromReportNumber(n, currency))
		return
	}
}

func assignReportMoney(money *reportEntryMoney, kind string, value MoneyView) {
	switch kind {
	case "EARNED", "BILLABLE", "BILLABLE_AMOUNT", "AMOUNT":
		money.earned = moneyPtr(value)
	case "COST", "COST_AMOUNT":
		money.cost = moneyPtr(value)
	case "PROFIT", "PROFIT_AMOUNT":
		money.profit = moneyPtr(value)
	}
}

func firstPresent(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return nil
}

func reportNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case jsonNumber:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

type jsonNumber interface {
	Float64() (float64, error)
}

func reportCurrency(m map[string]any) string {
	if m == nil {
		return ""
	}
	if nested, ok := m["currency"].(map[string]any); ok {
		for _, key := range []string{"code", "id"} {
			if s := strings.TrimSpace(fmt.Sprint(nested[key])); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	for _, key := range []string{"currency", "currencyCode", "currency_code"} {
		if s := strings.TrimSpace(fmt.Sprint(m[key])); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Service) derivedFinancialsForEntry(ctx context.Context, wsID string, entry clockify.TimeEntry, allowRemoteLookup bool) EntryFinancials {
	rate := BuildEntryRateView(nil, nil, nil, &entry)
	if moneyPtr(rate.BillableEarnings) == nil && moneyPtr(rate.Cost) == nil && allowRemoteLookup && s.entryFinancialReportsEnabled() {
		ws, project, task := s.rateContextForEntry(ctx, wsID, entry)
		rate = BuildEntryRateView(ws, project, task, &entry)
	}
	var earned *MoneyView
	if entry.Billable {
		earned = moneyPtr(rate.BillableEarnings)
	} else if rate.EffectiveHourly != nil {
		zero := MoneyFromCents(0, rate.EffectiveHourly.Currency)
		earned = &zero
	}
	cost := moneyPtr(rate.Cost)
	if earned == nil && cost == nil {
		return EntryFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "no reports API amount row and no usable hourly or cost rate",
		}
	}
	reason := ""
	profit, reason := profitMoney(earned, cost, reason)
	if reason == "" && !entry.Billable {
		reason = "earned amount is zero because the entry is not marked billable"
	}
	return EntryFinancials{
		Earned: earned,
		Cost:   cost,
		Profit: profit,
		Source: entryFinancialSourceDerivedRates,
		Reason: reason,
		Rate:   &rate,
	}
}

func (s *Service) rateContextForEntry(ctx context.Context, wsID string, entry clockify.TimeEntry) (*clockify.Workspace, *clockify.Project, *clockify.Task) {
	if s == nil || s.Client == nil || !s.entryFinancialReportsEnabled() {
		return nil, nil, nil
	}
	var ws *clockify.Workspace
	if wsID != "" {
		if path, err := paths.Workspace(wsID); err == nil {
			var out clockify.Workspace
			if err := s.Client.Get(ctx, path, nil, &out); err == nil {
				ws = &out
			}
		}
	}
	var project *clockify.Project
	if wsID != "" && strings.TrimSpace(entry.ProjectID) != "" {
		if path, err := paths.Workspace(wsID, "projects", entry.ProjectID); err == nil {
			var out clockify.Project
			if err := s.Client.Get(ctx, path, nil, &out); err == nil {
				project = &out
			}
		}
	}
	var task *clockify.Task
	if wsID != "" && strings.TrimSpace(entry.ProjectID) != "" && strings.TrimSpace(entry.TaskID) != "" {
		if path, err := paths.Workspace(wsID, "projects", entry.ProjectID, "tasks", entry.TaskID); err == nil {
			var out clockify.Task
			if err := s.Client.Get(ctx, path, nil, &out); err == nil {
				task = &out
			}
		}
	}
	return ws, project, task
}

func moneyPtr(m MoneyView) *MoneyView {
	if m.IsZero() {
		return nil
	}
	return &m
}

func profitMoney(earned, cost *MoneyView, existingReason string) (*MoneyView, string) {
	if earned == nil || cost == nil {
		return nil, existingReason
	}
	if earned.Currency != "" && cost.Currency != "" && earned.Currency != cost.Currency {
		reason := "profit omitted because earned and cost currencies differ"
		if existingReason != "" {
			reason = existingReason + "; " + reason
		}
		return nil, reason
	}
	currency := firstNonEmptyString(earned.Currency, cost.Currency)
	profit := MoneyFromCents(earned.AmountCents-cost.AmountCents, currency)
	if profit.AmountCents == 0 && currency == "" && math.Abs(float64(earned.AmountCents-cost.AmountCents)) == 0 {
		return nil, existingReason
	}
	return &profit, existingReason
}
