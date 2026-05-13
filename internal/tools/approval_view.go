package tools

import (
	"maps"
	"strconv"
	"strings"
)

type ApprovalView struct {
	ID               string                          `json:"id,omitempty"`
	Request          map[string]any                  `json:"request,omitempty"`
	Owner            map[string]any                  `json:"owner,omitempty"`
	Creator          map[string]any                  `json:"creator,omitempty"`
	Status           map[string]any                  `json:"status,omitempty"`
	AuditTrail       *ApprovalAuditTrail             `json:"audit_trail,omitempty"`
	DateRange        any                             `json:"date_range,omitempty"`
	Totals           map[string]any                  `json:"totals,omitempty"`
	DurationTotals   ApprovalDurationTotals          `json:"duration_totals"`
	MoneyTotals      ApprovalMoneyTotals             `json:"money_totals"`
	Financials       EntryFinancials                 `json:"financials"`
	EntrySummary     ReportEntrySummary              `json:"entry_summary"`
	ExpenseSummary   ApprovalExpenseSummary          `json:"expense_summary"`
	ClientSummary    []ReportClientSummary           `json:"client_summary,omitempty"`
	Rollups          map[string][]ReportEntityRollup `json:"rollups,omitempty"`
	Entries          []ReportEntryView               `json:"entries,omitempty"`
	Expenses         []map[string]any                `json:"expenses,omitempty"`
	Actions          []string                        `json:"actions,omitempty"`
	SuggestedActions []ToolSuggestion                `json:"suggestedActions,omitempty"`
	Raw              map[string]any                  `json:"raw,omitempty"`
}

type ApprovalAuditTrail struct {
	State         string         `json:"state,omitempty"`
	UpdatedBy     string         `json:"updated_by,omitempty"`
	UpdatedByName string         `json:"updated_by_name,omitempty"`
	UpdatedAt     string         `json:"updated_at,omitempty"`
	Note          string         `json:"note,omitempty"`
	Creator       map[string]any `json:"creator,omitempty"`
	Source        string         `json:"source,omitempty"`
	Raw           map[string]any `json:"raw,omitempty"`
}

type ApprovalDurationTotals struct {
	Approved *EntryDurationView `json:"approved,omitempty"`
	Pending  *EntryDurationView `json:"pending,omitempty"`
	Tracked  *EntryDurationView `json:"tracked,omitempty"`
	Billable *EntryDurationView `json:"billable,omitempty"`
	Break    *EntryDurationView `json:"break,omitempty"`
}

type ApprovalMoneyTotals struct {
	Earned   *MoneyView `json:"earned,omitempty"`
	Cost     *MoneyView `json:"cost,omitempty"`
	Expenses *MoneyView `json:"expenses,omitempty"`
	Profit   *MoneyView `json:"profit,omitempty"`
	Source   string     `json:"source"`
	Reason   string     `json:"reason,omitempty"`
}

type ApprovalExpenseSummary struct {
	Count         int        `json:"count"`
	BillableCount int        `json:"billable_count,omitempty"`
	Amount        *MoneyView `json:"amount,omitempty"`
	Source        string     `json:"source"`
	Reason        string     `json:"reason,omitempty"`
}

func approvalViewFromRaw(raw map[string]any) ApprovalView {
	request := approvalRequestMap(raw)
	view := ApprovalView{
		ID:        firstNonEmptyString(firstReportString(request, "id", "_id", "get_id"), firstReportString(raw, "id", "_id", "get_id")),
		Request:   request,
		Owner:     cloneOptionalMap(firstPresent(request, "owner")),
		Creator:   cloneOptionalMap(firstPresent(request, "creator")),
		Status:    approvalStatusMap(request),
		DateRange: firstPresent(request, "dateRange", "date_range"),
		Totals:    approvalTotalsMap(raw),
		Financials: EntryFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "approval response did not include money fields",
		},
		Raw: maps.Clone(raw),
	}
	money := reportMoneyFromRow(raw)
	if money.hasMoney() {
		view.Financials = money.financials()
	}
	for _, row := range mapSlice(raw["entries"]) {
		view.Entries = append(view.Entries, reportEntryViewFromRow(row))
	}
	for _, row := range mapSlice(raw["expenses"]) {
		view.Expenses = append(view.Expenses, maps.Clone(row))
	}
	view.DurationTotals = approvalDurationTotals(raw, request)
	view.MoneyTotals = approvalMoneyTotals(raw, request, view.Entries, view.Expenses)
	if view.MoneyTotals.Source != entryFinancialSourceUnavailable {
		view.Financials = EntryFinancials{
			Earned: view.MoneyTotals.Earned,
			Cost:   view.MoneyTotals.Cost,
			Profit: view.MoneyTotals.Profit,
			Source: view.MoneyTotals.Source,
			Reason: view.MoneyTotals.Reason,
		}
	}
	view.EntrySummary = summarizeReportEntries(view.Entries)
	view.ExpenseSummary = approvalExpenseSummary(raw, view.Expenses)
	view.ClientSummary = summarizeReportClients(view.Entries)
	view.Rollups = approvalEntryRollups(view.Entries)
	view.AuditTrail = approvalAuditTrail(raw, request)
	view.Actions = approvalActions(view.Status)
	view.SuggestedActions = approvalSuggestedActions(view)
	return view
}

func approvalViewsFromRaw(items []map[string]any) []ApprovalView {
	out := make([]ApprovalView, 0, len(items))
	for _, item := range items {
		out = append(out, approvalViewFromRaw(item))
	}
	return out
}

func approvalDataView(raw any) any {
	switch value := raw.(type) {
	case map[string]any:
		return approvalViewFromRaw(value)
	case []map[string]any:
		return approvalViewsFromRaw(value)
	case []any:
		views := make([]ApprovalView, 0, len(value))
		for _, item := range value {
			if m, ok := item.(map[string]any); ok {
				views = append(views, approvalViewFromRaw(m))
			}
		}
		if len(views) == len(value) {
			return views
		}
	}
	return raw
}

func approvalRequestMap(raw map[string]any) map[string]any {
	for _, key := range []string{"approvalRequest", "request", "approval_request"} {
		if nested, ok := raw[key].(map[string]any); ok {
			return maps.Clone(nested)
		}
	}
	return maps.Clone(raw)
}

func approvalStatusMap(request map[string]any) map[string]any {
	state := strings.ToUpper(firstReportString(request, "status", "state"))
	out := map[string]any{}
	if state != "" {
		out["state"] = state
	}
	if nested, ok := request["status"].(map[string]any); ok {
		for k, v := range nested {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func approvalAuditTrail(raw, request map[string]any) *ApprovalAuditTrail {
	status := approvalStatusMap(request)
	if status == nil {
		status = approvalStatusMap(raw)
	}
	creator := cloneOptionalMap(firstPresent(request, "creator"))
	if len(creator) == 0 {
		creator = cloneOptionalMap(firstPresent(raw, "creator"))
	}
	if len(creator) > 0 {
		if _, ok := creator["email"]; !ok {
			if email := firstReportString(creator, "userEmail", "user_email"); email != "" {
				creator["email"] = email
			}
		}
	}
	trail := &ApprovalAuditTrail{
		State:         strings.ToUpper(reportValueString(status["state"])),
		UpdatedBy:     firstNonEmptyString(reportValueString(status["updatedBy"]), firstReportString(request, "updatedBy", "updated_by")),
		UpdatedByName: firstNonEmptyString(reportValueString(status["updatedByUserName"]), firstReportString(request, "updatedByUserName", "updated_by_user_name")),
		UpdatedAt:     firstNonEmptyString(reportValueString(status["updatedAt"]), firstReportString(request, "updatedAt", "updated_at")),
		Note:          firstNonEmptyString(reportValueString(status["note"]), firstReportString(request, "note")),
		Creator:       creator,
		Source:        "approval_api",
	}
	if len(status) > 0 {
		trail.Raw = maps.Clone(status)
	}
	if trail.State == "" && trail.UpdatedBy == "" && trail.UpdatedByName == "" && trail.UpdatedAt == "" && trail.Note == "" && len(trail.Creator) == 0 {
		return nil
	}
	return trail
}

func approvalTotalsMap(raw map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"approvedTime",
		"billableTime",
		"breakTime",
		"pendingTime",
		"trackedTime",
		"expenseTotal",
	} {
		if value, ok := raw[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func approvalEntryRollups(entries []ReportEntryView) map[string][]ReportEntityRollup {
	if len(entries) == 0 {
		return nil
	}
	summary := summarizeReportEntities(entries)
	out := map[string][]ReportEntityRollup{}
	if len(summary.Users) > 0 {
		out["users"] = summary.Users
	}
	if len(summary.Projects) > 0 {
		out["projects"] = summary.Projects
	}
	if len(summary.Tasks) > 0 {
		out["tasks"] = summary.Tasks
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func approvalDurationTotals(raw, request map[string]any) ApprovalDurationTotals {
	return ApprovalDurationTotals{
		Approved: approvalDurationValue(raw, request, "approvedTime", "approved_time"),
		Pending:  approvalDurationValue(raw, request, "pendingTime", "pending_time"),
		Tracked:  approvalDurationValue(raw, request, "trackedTime", "tracked_time"),
		Billable: approvalDurationValue(raw, request, "billableTime", "billable_time"),
		Break:    approvalDurationValue(raw, request, "breakTime", "break_time"),
	}
}

func approvalDurationValue(raw, request map[string]any, keys ...string) *EntryDurationView {
	if seconds, ok := approvalDurationSeconds(firstPresent(raw, keys...)); ok {
		view := entryDurationView(seconds, "approval_api")
		return &view
	}
	if seconds, ok := approvalDurationSeconds(firstPresent(request, keys...)); ok {
		view := entryDurationView(seconds, "approval_api")
		return &view
	}
	return nil
}

func approvalDurationSeconds(value any) (int64, bool) {
	switch raw := value.(type) {
	case nil:
		return 0, false
	case string:
		if seconds, ok := clockifyDurationSeconds(raw); ok {
			return seconds, true
		}
		return clockDurationSeconds(raw)
	case map[string]any:
		for _, key := range []string{"duration", "seconds", "value", "time"} {
			if seconds, ok := approvalDurationSeconds(raw[key]); ok {
				return seconds, true
			}
		}
	case EntryDurationView:
		return raw.Seconds, true
	default:
		if number, ok := reportNumber(raw); ok {
			return int64(number), true
		}
	}
	return 0, false
}

func clockDurationSeconds(raw string) (int64, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, false
	}
	sign := int64(1)
	if strings.HasPrefix(parts[0], "-") {
		sign = -1
		parts[0] = strings.TrimPrefix(parts[0], "-")
	}
	var values []int64
	for _, part := range parts {
		number, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return 0, false
		}
		values = append(values, number)
	}
	if len(values) == 2 {
		return sign * ((values[0] * 3600) + (values[1] * 60)), true
	}
	return sign * ((values[0] * 3600) + (values[1] * 60) + values[2]), true
}

func approvalMoneyTotals(raw, request map[string]any, entries []ReportEntryView, expenses []map[string]any) ApprovalMoneyTotals {
	currency := firstNonEmptyString(reportCurrency(raw), reportCurrency(request))
	money := reportMoneyFromRow(raw)
	if approvalHasNestedRequest(raw) {
		money = mergeReportEntryMoney(money, reportMoneyFromRow(request))
	}
	if !money.hasMoney() && len(entries) > 0 {
		money = entryFinancialsMoney(financialsFromReportEntries(entries))
	}
	expensesTotal := moneyFromAny(firstPresent(raw, "expenseTotal", "expensesTotal", "expense_total"), currency)
	if expensesTotal == nil {
		expensesTotal = moneyFromAny(firstPresent(request, "expenseTotal", "expensesTotal", "expense_total"), currency)
	}
	if expensesTotal == nil {
		for _, expense := range expenses {
			expensesTotal = moneySum(expensesTotal, moneyFromAny(firstPresent(expense, "amount", "total", "billableAmount"), firstNonEmptyString(reportCurrency(expense), currency)))
		}
	}
	if !money.hasMoney() && expensesTotal == nil {
		return ApprovalMoneyTotals{
			Source: entryFinancialSourceUnavailable,
			Reason: "approval response did not include money fields",
		}
	}
	profit := money.profit
	reason := money.reason
	if profit == nil {
		profit, reason = profitMoney(money.earned, money.cost, reason)
	}
	return ApprovalMoneyTotals{
		Earned:   money.earned,
		Cost:     money.cost,
		Expenses: expensesTotal,
		Profit:   profit,
		Source:   "approval_api",
		Reason:   reason,
	}
}

func approvalHasNestedRequest(raw map[string]any) bool {
	for _, key := range []string{"approvalRequest", "request", "approval_request"} {
		if _, ok := raw[key].(map[string]any); ok {
			return true
		}
	}
	return false
}

func approvalExpenseSummary(raw map[string]any, expenses []map[string]any) ApprovalExpenseSummary {
	summary := ApprovalExpenseSummary{Count: len(expenses), Source: "approval_api"}
	currency := reportCurrency(raw)
	for _, expense := range expenses {
		summary.Amount = moneySum(summary.Amount, moneyFromAny(firstPresent(expense, "amount", "total", "billableAmount"), firstNonEmptyString(reportCurrency(expense), currency)))
		if billable, ok := firstReportBool(expense, "billable", "isBillable"); ok && billable {
			summary.BillableCount++
		}
	}
	if summary.Amount == nil {
		summary.Amount = moneyFromAny(firstPresent(raw, "expenseTotal", "expensesTotal", "expense_total"), currency)
	}
	if summary.Count == 0 && summary.Amount == nil {
		summary.Source = entryFinancialSourceUnavailable
		summary.Reason = "approval response did not include expense details"
	}
	return summary
}

func approvalActions(status map[string]any) []string {
	state := ""
	if status != nil {
		state = strings.ToUpper(reportValueString(status["state"]))
	}
	switch state {
	case "PENDING":
		return []string{"approve", "reject", "withdraw"}
	case "REJECTED", "WITHDRAWN_SUBMISSION", "WITHDRAWN_APPROVAL", "WITHDRAWN":
		return []string{"resubmit"}
	default:
		return nil
	}
}

func approvalSuggestedActions(view ApprovalView) []ToolSuggestion {
	if view.ID == "" {
		return nil
	}
	suggestions := make([]ToolSuggestion, 0, len(view.Actions))
	for _, action := range view.Actions {
		switch action {
		case "approve":
			suggestions = append(suggestions, ToolSuggestion{
				Tool:      "clockify_approve_timesheet",
				Reason:    "Approve this pending approval request.",
				Arguments: map[string]any{"approval_id": view.ID},
			})
		case "reject":
			suggestions = append(suggestions, ToolSuggestion{
				Tool:      "clockify_reject_timesheet",
				Reason:    "Reject this pending approval request with a note.",
				Arguments: map[string]any{"approval_id": view.ID},
			})
		case "withdraw":
			suggestions = append(suggestions, ToolSuggestion{
				Tool:      "clockify_withdraw_approval",
				Reason:    "Withdraw this approval request if it should not remain submitted.",
				Arguments: map[string]any{"approval_id": view.ID},
			})
		case "resubmit":
			suggestions = append(suggestions, ToolSuggestion{
				Tool:        "clockify_resubmit_for_approval",
				Reason:      "Resubmit the relevant period after fixing rejected or withdrawn entries.",
				MissingArgs: []string{"period", "period_start"},
			})
		}
	}
	return suggestions
}

func cloneOptionalMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return maps.Clone(m)
	}
	return nil
}

func approvalViewEnvelope(action string, slice bool) map[string]any {
	if slice {
		return envelopeSchemaFor[[]ApprovalView](action)
	}
	return envelopeSchemaFor[ApprovalView](action)
}
