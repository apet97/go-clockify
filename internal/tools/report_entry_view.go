package tools

import (
	"fmt"
	"maps"
	"sort"
	"strings"
)

// EntryDurationView is the normalized duration block used by report-derived
// entry views and summaries.
type EntryDurationView struct {
	Seconds int64   `json:"seconds"`
	Hours   float64 `json:"hours"`
	Display string  `json:"display"`
	Source  string  `json:"source,omitempty"`
}

// EntryEntityRef keeps entity IDs and names close to the entry that produced
// them without forcing callers to understand Clockify's varying row shapes.
type EntryEntityRef struct {
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Email string         `json:"email,omitempty"`
	Color string         `json:"color,omitempty"`
	Raw   map[string]any `json:"raw,omitempty"`
}

type EntryEntitiesView struct {
	User    *EntryEntityRef  `json:"user,omitempty"`
	Client  *EntryEntityRef  `json:"client,omitempty"`
	Project *EntryEntityRef  `json:"project,omitempty"`
	Task    *EntryEntityRef  `json:"task,omitempty"`
	Tags    []EntryEntityRef `json:"tags,omitempty"`
}

type EntryApprovalView struct {
	RequestID string         `json:"request_id,omitempty"`
	State     string         `json:"state,omitempty"`
	Source    string         `json:"source,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
}

type EntryInvoicingView struct {
	Invoiced      *bool          `json:"invoiced,omitempty"`
	State         string         `json:"state,omitempty"`
	InvoiceID     string         `json:"invoice_id,omitempty"`
	InvoiceNumber string         `json:"invoice_number,omitempty"`
	Source        string         `json:"source,omitempty"`
	Raw           map[string]any `json:"raw,omitempty"`
}

type EntryAuditView struct {
	Locked             *bool          `json:"locked,omitempty"`
	MissingDescription *bool          `json:"missing_description,omitempty"`
	MissingProject     *bool          `json:"missing_project,omitempty"`
	MissingTask        *bool          `json:"missing_task,omitempty"`
	Source             string         `json:"source,omitempty"`
	Raw                map[string]any `json:"raw,omitempty"`
}

type MoneyByCurrencyView struct {
	Type     string         `json:"type,omitempty"`
	Currency string         `json:"currency,omitempty"`
	Money    MoneyView      `json:"money"`
	Raw      map[string]any `json:"raw,omitempty"`
}

type ReportRateBreakdownView struct {
	Rate         *MoneyView     `json:"rate,omitempty"`
	Amount       *MoneyView     `json:"amount,omitempty"`
	EarnedAmount *MoneyView     `json:"earnedAmount,omitempty"`
	EarnedRate   *MoneyView     `json:"earnedRate,omitempty"`
	CostAmount   *MoneyView     `json:"costAmount,omitempty"`
	CostRate     *MoneyView     `json:"costRate,omitempty"`
	Currency     string         `json:"currency,omitempty"`
	Raw          map[string]any `json:"raw,omitempty"`
}

type ReportEntryView struct {
	ID                string                   `json:"id,omitempty"`
	Description       string                   `json:"description,omitempty"`
	Type              string                   `json:"type,omitempty"`
	Billable          *bool                    `json:"billable,omitempty"`
	Locked            *bool                    `json:"locked,omitempty"`
	Duration          *EntryDurationView       `json:"duration,omitempty"`
	TimeInterval      map[string]any           `json:"time_interval,omitempty"`
	Entities          *EntryEntitiesView       `json:"entities,omitempty"`
	CustomFieldValues any                      `json:"customFieldValues,omitempty"`
	CustomFields      []CustomFieldValueView   `json:"custom_fields_normalized,omitempty"`
	Approval          *EntryApprovalView       `json:"approval,omitempty"`
	Invoicing         *EntryInvoicingView      `json:"invoicing,omitempty"`
	Audit             *EntryAuditView          `json:"audit,omitempty"`
	Financials        EntryFinancials          `json:"financials"`
	RateBreakdown     *ReportRateBreakdownView `json:"rate_breakdown,omitempty"`
	MoneyByCurrency   []MoneyByCurrencyView    `json:"money_by_currency,omitempty"`
	Raw               map[string]any           `json:"raw,omitempty"`
}

type ReportEntrySummary struct {
	Count                   int               `json:"count"`
	Duration                EntryDurationView `json:"duration"`
	BillableCount           int               `json:"billable_count,omitempty"`
	UnbillableCount         int               `json:"unbillable_count,omitempty"`
	LockedCount             int               `json:"locked_count,omitempty"`
	MissingDescriptionCount int               `json:"missing_description_count,omitempty"`
	MissingProjectCount     int               `json:"missing_project_count,omitempty"`
	MissingTaskCount        int               `json:"missing_task_count,omitempty"`
	ByType                  map[string]int    `json:"by_type,omitempty"`
}

type ReportApprovalSummary struct {
	WithApprovalRequest    int                          `json:"with_approval_request,omitempty"`
	WithoutApprovalRequest int                          `json:"without_approval_request,omitempty"`
	RequestIDs             []string                     `json:"request_ids,omitempty"`
	States                 map[string]int               `json:"states,omitempty"`
	DurationsByState       map[string]EntryDurationView `json:"durations_by_state,omitempty"`
}

type ReportTotalsSummary struct {
	Rows              int                   `json:"rows"`
	EntriesCount      int                   `json:"entries_count,omitempty"`
	TotalTime         *EntryDurationView    `json:"total_time,omitempty"`
	TotalBillableTime *EntryDurationView    `json:"total_billable_time,omitempty"`
	Financials        EntryFinancials       `json:"financials"`
	MoneyByCurrency   []MoneyByCurrencyView `json:"money_by_currency,omitempty"`
	Raw               any                   `json:"raw,omitempty"`
}

type ReportEntityRollup struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name,omitempty"`
	Email    string            `json:"email,omitempty"`
	Color    string            `json:"color,omitempty"`
	Count    int               `json:"count"`
	Duration EntryDurationView `json:"duration"`
}

type ReportEntitySummary struct {
	Users    []ReportEntityRollup `json:"users,omitempty"`
	Clients  []ReportEntityRollup `json:"clients,omitempty"`
	Projects []ReportEntityRollup `json:"projects,omitempty"`
	Tasks    []ReportEntityRollup `json:"tasks,omitempty"`
	Tags     []ReportEntityRollup `json:"tags,omitempty"`
}

type ReportClientSummary struct {
	Client          EntryEntityRef        `json:"client"`
	EntrySummary    ReportEntrySummary    `json:"entry_summary"`
	Financials      EntryFinancials       `json:"financials"`
	ApprovalSummary ReportApprovalSummary `json:"approval_summary"`
}

type detailedReportViews struct {
	Entries          []ReportEntryView     `json:"entries"`
	EntrySummary     ReportEntrySummary    `json:"entry_summary"`
	ApprovalSummary  ReportApprovalSummary `json:"approval_summary"`
	TotalsSummary    ReportTotalsSummary   `json:"totals_summary"`
	EntitySummary    ReportEntitySummary   `json:"entity_summary"`
	ClientSummary    []ReportClientSummary `json:"client_summary,omitempty"`
	SuggestedActions []ToolSuggestion      `json:"suggestedActions,omitempty"`
}

func normalizeDetailedReportPayload(data map[string]any) detailedReportViews {
	rows := reportTimeEntryRows(data)
	entries := make([]ReportEntryView, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, reportEntryViewFromRow(row))
	}
	return detailedReportViews{
		Entries:          entries,
		EntrySummary:     summarizeReportEntries(entries),
		ApprovalSummary:  summarizeReportApprovals(entries),
		TotalsSummary:    summarizeReportTotals(data, entries),
		EntitySummary:    summarizeReportEntities(entries),
		ClientSummary:    summarizeReportClients(entries),
		SuggestedActions: detailedReportSuggestions(entries),
	}
}

func appendDetailedReportViews(data map[string]any) int {
	if data == nil {
		return 0
	}
	raw := maps.Clone(data)
	views := normalizeDetailedReportPayload(data)
	data["raw"] = raw
	data["entries"] = views.Entries
	data["entry_summary"] = views.EntrySummary
	data["approval_summary"] = views.ApprovalSummary
	data["totals_summary"] = views.TotalsSummary
	data["entity_summary"] = views.EntitySummary
	data["client_summary"] = views.ClientSummary
	data["suggestedActions"] = views.SuggestedActions
	return len(views.Entries)
}

func reportEntryViewFromRow(row map[string]any) ReportEntryView {
	view := ReportEntryView{
		ID:          reportRowEntryID(row),
		Description: firstReportString(row, "description", "name"),
		Type:        strings.ToUpper(firstReportString(row, "type", "entryType", "timeEntryType")),
		Financials: EntryFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "no report money fields were present for this entry row",
		},
		Raw: maps.Clone(row),
	}
	if billable, ok := firstReportBool(row, "billable", "isBillable"); ok {
		view.Billable = &billable
	}
	if locked, ok := firstReportBool(row, "locked", "isLocked"); ok {
		view.Locked = &locked
	}
	if duration, ok := reportDurationSeconds(row); ok {
		d := entryDurationView(duration, "reports_api")
		view.Duration = &d
	}
	if interval := reportEntryTimeInterval(row); len(interval) > 0 {
		view.TimeInterval = interval
		if view.Duration == nil {
			if duration, ok := reportDurationSeconds(interval); ok {
				d := entryDurationView(duration, "time_interval")
				view.Duration = &d
			}
		}
	}
	if custom := firstPresent(row, "customFieldValues", "customFields", "custom_field_values"); custom != nil {
		view.CustomFieldValues = custom
		view.CustomFields = customFieldValuesFromRaw(custom)
	}
	entities := reportEntryEntities(row)
	if !entryEntitiesEmpty(entities) {
		view.Entities = &entities
	}
	approval := reportEntryApproval(row)
	if !entryApprovalEmpty(approval) {
		view.Approval = &approval
	}
	invoicing := reportEntryInvoicing(row)
	if !entryInvoicingEmpty(invoicing) {
		view.Invoicing = &invoicing
	}
	audit := reportEntryAudit(row, view)
	if !entryAuditEmpty(audit) {
		view.Audit = &audit
	}
	money := reportMoneyFromRow(row)
	if money.hasMoney() {
		view.Financials = money.financials()
	}
	view.RateBreakdown = reportRateBreakdownFromRow(row)
	view.MoneyByCurrency = reportMoneyByCurrency(row)
	return view
}

func summarizeReportEntries(entries []ReportEntryView) ReportEntrySummary {
	summary := ReportEntrySummary{Count: len(entries), ByType: map[string]int{}}
	var seconds int64
	for _, entry := range entries {
		if entry.Duration != nil {
			seconds += entry.Duration.Seconds
		}
		if entry.Billable != nil {
			if *entry.Billable {
				summary.BillableCount++
			} else {
				summary.UnbillableCount++
			}
		}
		if entry.Locked != nil && *entry.Locked {
			summary.LockedCount++
		}
		if entry.Audit != nil {
			if boolPtrValue(entry.Audit.MissingDescription) {
				summary.MissingDescriptionCount++
			}
			if boolPtrValue(entry.Audit.MissingProject) {
				summary.MissingProjectCount++
			}
			if boolPtrValue(entry.Audit.MissingTask) {
				summary.MissingTaskCount++
			}
		}
		if entry.Type != "" {
			summary.ByType[entry.Type]++
		}
	}
	if len(summary.ByType) == 0 {
		summary.ByType = nil
	}
	summary.Duration = entryDurationView(seconds, "entries")
	return summary
}

func summarizeReportApprovals(entries []ReportEntryView) ReportApprovalSummary {
	summary := ReportApprovalSummary{States: map[string]int{}, DurationsByState: map[string]EntryDurationView{}}
	seen := map[string]struct{}{}
	durationByState := map[string]int64{}
	for _, entry := range entries {
		if entry.Approval == nil || entry.Approval.RequestID == "" {
			summary.WithoutApprovalRequest++
		} else {
			summary.WithApprovalRequest++
			if _, ok := seen[entry.Approval.RequestID]; !ok {
				seen[entry.Approval.RequestID] = struct{}{}
				summary.RequestIDs = append(summary.RequestIDs, entry.Approval.RequestID)
			}
		}
		state := "UNKNOWN"
		if entry.Approval != nil && entry.Approval.State != "" {
			state = strings.ToUpper(entry.Approval.State)
		}
		summary.States[state]++
		if entry.Duration != nil {
			durationByState[state] += entry.Duration.Seconds
		}
	}
	if len(summary.States) == 0 {
		summary.States = nil
	}
	if len(durationByState) == 0 {
		summary.DurationsByState = nil
	} else {
		for state, seconds := range durationByState {
			summary.DurationsByState[state] = entryDurationView(seconds, "entries")
		}
	}
	return summary
}

func summarizeReportTotals(data map[string]any, entries []ReportEntryView) ReportTotalsSummary {
	totals := firstPresent(data, "totals", "total")
	rows := mapSlice(totals)
	summary := ReportTotalsSummary{
		Rows: len(rows),
		Financials: EntryFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "report totals did not include money fields",
		},
		Raw: totals,
	}
	var totalTime int64
	var totalBillable int64
	var hasTotalTime bool
	var hasBillable bool
	money := reportEntryMoney{}
	for _, row := range rows {
		if count, ok := reportInt(firstPresent(row, "entriesCount", "entries_count", "count")); ok {
			summary.EntriesCount += count
		}
		if seconds, ok := reportDurationSeconds(map[string]any{"duration": firstPresent(row, "totalTime", "total_time", "totalDuration", "duration")}); ok {
			totalTime += seconds
			hasTotalTime = true
		}
		if seconds, ok := reportDurationSeconds(map[string]any{"duration": firstPresent(row, "totalBillableTime", "total_billable_time")}); ok {
			totalBillable += seconds
			hasBillable = true
		}
		money = mergeReportEntryMoney(money, reportMoneyFromRow(row))
	}
	if len(rows) == 0 {
		summary.EntriesCount = len(entries)
		for _, entry := range entries {
			if entry.Duration != nil {
				totalTime += entry.Duration.Seconds
				hasTotalTime = true
				if entry.Billable != nil && *entry.Billable {
					totalBillable += entry.Duration.Seconds
					hasBillable = true
				}
			}
			money = mergeReportEntryMoney(money, entryFinancialsMoney(entry.Financials))
		}
	}
	if hasTotalTime {
		view := entryDurationView(totalTime, "totals")
		summary.TotalTime = &view
	}
	if hasBillable {
		view := entryDurationView(totalBillable, "totals")
		summary.TotalBillableTime = &view
	}
	if money.hasMoney() {
		summary.Financials = money.financials()
	}
	if byCurrency := reportMoneyByCurrency(data); len(byCurrency) > 0 {
		summary.MoneyByCurrency = byCurrency
	} else {
		for _, row := range rows {
			summary.MoneyByCurrency = append(summary.MoneyByCurrency, reportMoneyByCurrency(row)...)
		}
	}
	return summary
}

func summarizeReportEntities(entries []ReportEntryView) ReportEntitySummary {
	users := map[string]*entityRollupAccumulator{}
	clients := map[string]*entityRollupAccumulator{}
	projects := map[string]*entityRollupAccumulator{}
	tasks := map[string]*entityRollupAccumulator{}
	tags := map[string]*entityRollupAccumulator{}
	for _, entry := range entries {
		if entry.Entities == nil {
			continue
		}
		seconds := int64(0)
		if entry.Duration != nil {
			seconds = entry.Duration.Seconds
		}
		addEntityRollup(users, entry.Entities.User, seconds)
		addEntityRollup(clients, entry.Entities.Client, seconds)
		addEntityRollup(projects, entry.Entities.Project, seconds)
		addEntityRollup(tasks, entry.Entities.Task, seconds)
		for i := range entry.Entities.Tags {
			ref := entry.Entities.Tags[i]
			addEntityRollup(tags, &ref, seconds)
		}
	}
	return ReportEntitySummary{
		Users:    entityRollups(users),
		Clients:  entityRollups(clients),
		Projects: entityRollups(projects),
		Tasks:    entityRollups(tasks),
		Tags:     entityRollups(tags),
	}
}

func summarizeReportClients(entries []ReportEntryView) []ReportClientSummary {
	grouped := map[string][]ReportEntryView{}
	refs := map[string]EntryEntityRef{}
	for _, entry := range entries {
		if entry.Entities == nil || entry.Entities.Client == nil {
			continue
		}
		ref := *entry.Entities.Client
		key := firstNonEmptyString(ref.ID, ref.Name)
		if key == "" {
			continue
		}
		grouped[key] = append(grouped[key], entry)
		refs[key] = ref
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ReportClientSummary, 0, len(keys))
	for _, key := range keys {
		entries := grouped[key]
		out = append(out, ReportClientSummary{
			Client:          refs[key],
			EntrySummary:    summarizeReportEntries(entries),
			Financials:      financialsFromReportEntries(entries),
			ApprovalSummary: summarizeReportApprovals(entries),
		})
	}
	return out
}

func detailedReportSuggestions(entries []ReportEntryView) []ToolSuggestion {
	suggestions := make([]ToolSuggestion, 0, 2)
	clients := summarizeReportClients(entries)
	if len(clients) > 0 {
		client := clients[0].Client
		ref := firstNonEmptyString(client.ID, client.Name)
		if ref != "" {
			suggestions = append(suggestions, ToolSuggestion{
				Tool:      "clockify_client_report",
				Reason:    "Drill into the top client from this detailed report.",
				Arguments: map[string]any{"client": ref},
			})
		}
	}
	entrySummary := summarizeReportEntries(entries)
	if entrySummary.MissingDescriptionCount > 0 || entrySummary.MissingProjectCount > 0 || entrySummary.MissingTaskCount > 0 || entrySummary.LockedCount > 0 {
		suggestions = append(suggestions, ToolSuggestion{
			Tool:   "clockify_timesheet_review",
			Reason: "Review locked or incomplete entries found in this detailed report.",
		})
	}
	return suggestions
}

type entityRollupAccumulator struct {
	ref     EntryEntityRef
	count   int
	seconds int64
}

func addEntityRollup(acc map[string]*entityRollupAccumulator, ref *EntryEntityRef, seconds int64) {
	if ref == nil {
		return
	}
	key := firstNonEmptyString(ref.ID, ref.Name, ref.Email)
	if key == "" {
		return
	}
	item, ok := acc[key]
	if !ok {
		item = &entityRollupAccumulator{ref: *ref}
		acc[key] = item
	}
	item.count++
	item.seconds += seconds
}

func entityRollups(acc map[string]*entityRollupAccumulator) []ReportEntityRollup {
	if len(acc) == 0 {
		return nil
	}
	out := make([]ReportEntityRollup, 0, len(acc))
	for _, item := range acc {
		out = append(out, ReportEntityRollup{
			ID:       item.ref.ID,
			Name:     item.ref.Name,
			Email:    item.ref.Email,
			Color:    item.ref.Color,
			Count:    item.count,
			Duration: entryDurationView(item.seconds, "entries"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return firstNonEmptyString(out[i].Name, out[i].ID) < firstNonEmptyString(out[j].Name, out[j].ID)
	})
	return out
}

func financialsFromReportEntries(entries []ReportEntryView) EntryFinancials {
	money := reportEntryMoney{}
	for _, entry := range entries {
		money = mergeReportEntryMoney(money, entryFinancialsMoney(entry.Financials))
	}
	if money.hasMoney() {
		return money.financials()
	}
	return EntryFinancials{
		Source: entryFinancialSourceUnavailable,
		Reason: "no report money fields were present for these entries",
	}
}

func mergeReportEntryMoney(existing, next reportEntryMoney) reportEntryMoney {
	existing.earned = moneySum(existing.earned, next.earned)
	existing.cost = moneySum(existing.cost, next.cost)
	existing.profit = moneySum(existing.profit, next.profit)
	if existing.reason == "" {
		existing.reason = next.reason
	}
	return existing
}

func entryFinancialsMoney(fin EntryFinancials) reportEntryMoney {
	return reportEntryMoney{
		earned: fin.Earned,
		cost:   fin.Cost,
		profit: fin.Profit,
		reason: fin.Reason,
	}
}

func reportInt(v any) (int, bool) {
	if n, ok := reportNumber(v); ok {
		return int(n), true
	}
	return 0, false
}

func reportEntryTimeInterval(row map[string]any) map[string]any {
	for _, key := range []string{"timeInterval", "time_interval"} {
		if interval, ok := row[key].(map[string]any); ok {
			return maps.Clone(interval)
		}
	}
	out := map[string]any{}
	for _, spec := range []struct {
		in  string
		out string
	}{
		{"start", "start"},
		{"timeStart", "start"},
		{"dateStart", "start"},
		{"end", "end"},
		{"timeEnd", "end"},
		{"dateEnd", "end"},
		{"duration", "duration"},
	} {
		if v, ok := row[spec.in]; ok {
			out[spec.out] = v
		}
	}
	return out
}

func reportEntryEntities(row map[string]any) EntryEntitiesView {
	return EntryEntitiesView{
		User:    entityRefFromReport(row, "user", "userId", "userName", "userEmail", ""),
		Client:  entityRefFromReport(row, "client", "clientId", "clientName", "", ""),
		Project: entityRefFromReport(row, "project", "projectId", "projectName", "", "projectColor"),
		Task:    entityRefFromReport(row, "task", "taskId", "taskName", "", ""),
		Tags:    tagRefsFromReport(row),
	}
}

func entityRefFromReport(row map[string]any, nestedKey, idKey, nameKey, emailKey, colorKey string) *EntryEntityRef {
	ref := EntryEntityRef{}
	if nested, ok := row[nestedKey].(map[string]any); ok {
		ref.Raw = maps.Clone(nested)
		ref.ID = firstReportString(nested, "id", "_id", "get_id")
		ref.Name = firstReportString(nested, "name", "title")
		ref.Email = firstReportString(nested, "email")
		ref.Color = firstReportString(nested, "color")
	}
	ref.ID = firstNonEmptyString(ref.ID, firstReportString(row, idKey))
	ref.Name = firstNonEmptyString(ref.Name, firstReportString(row, nameKey))
	if emailKey != "" {
		ref.Email = firstNonEmptyString(ref.Email, firstReportString(row, emailKey))
	}
	if colorKey != "" {
		ref.Color = firstNonEmptyString(ref.Color, firstReportString(row, colorKey))
	}
	if ref.ID == "" && ref.Name == "" && ref.Email == "" && ref.Color == "" && len(ref.Raw) == 0 {
		return nil
	}
	return &ref
}

func tagRefsFromReport(row map[string]any) []EntryEntityRef {
	var tags []EntryEntityRef
	for _, tag := range mapSlice(row["tags"]) {
		ref := EntryEntityRef{
			ID:   firstReportString(tag, "id", "_id", "get_id"),
			Name: firstReportString(tag, "name"),
			Raw:  maps.Clone(tag),
		}
		if ref.ID != "" || ref.Name != "" {
			tags = append(tags, ref)
		}
	}
	return tags
}

func reportEntryApproval(row map[string]any) EntryApprovalView {
	approval := EntryApprovalView{
		RequestID: firstReportString(row, "approvalRequestId", "approval_request_id", "approvalId"),
		State:     strings.ToUpper(firstReportString(row, "approvalState", "approvalStatus", "approval_state", "state", "status")),
		Source:    "reports_api",
	}
	for _, key := range []string{"approvalRequest", "approval", "approval_request"} {
		if nested, ok := row[key].(map[string]any); ok {
			approval.Raw = maps.Clone(nested)
			approval.RequestID = firstNonEmptyString(approval.RequestID, firstReportString(nested, "id", "_id", "get_id", "approvalRequestId"))
			approval.State = firstNonEmptyString(approval.State, strings.ToUpper(firstReportString(nested, "status", "state", "approvalState")))
		}
	}
	return approval
}

func reportEntryInvoicing(row map[string]any) EntryInvoicingView {
	view := EntryInvoicingView{
		State:         strings.ToUpper(firstReportString(row, "invoicingState", "invoicing_state")),
		InvoiceID:     firstReportString(row, "invoiceId", "invoiceID", "invoice_id"),
		InvoiceNumber: firstReportString(row, "invoiceNumber", "invoice_number"),
		Source:        "reports_api",
	}
	if invoiced, ok := firstReportBool(row, "invoiced", "isInvoiced"); ok {
		view.Invoiced = &invoiced
	}
	for _, key := range []string{"invoicingInfo", "invoicing", "invoice"} {
		if nested, ok := row[key].(map[string]any); ok {
			view.Raw = maps.Clone(nested)
			view.State = firstNonEmptyString(view.State, strings.ToUpper(firstReportString(nested, "state", "status", "invoicingState")))
			view.InvoiceID = firstNonEmptyString(view.InvoiceID, firstReportString(nested, "invoiceId", "invoiceID", "id", "_id"))
			view.InvoiceNumber = firstNonEmptyString(view.InvoiceNumber, firstReportString(nested, "invoiceNumber", "number"))
			if view.Invoiced == nil {
				if invoiced, ok := firstReportBool(nested, "invoiced", "isInvoiced"); ok {
					view.Invoiced = &invoiced
				}
			}
		}
	}
	return view
}

func reportEntryAudit(row map[string]any, view ReportEntryView) EntryAuditView {
	audit := EntryAuditView{Source: "reports_api"}
	if view.Locked != nil {
		audit.Locked = view.Locked
	}
	if missing, ok := firstReportBool(row, "missingDescription", "withoutDescription", "without_description"); ok {
		audit.MissingDescription = &missing
	} else if _, ok := row["description"]; ok {
		missing = strings.TrimSpace(view.Description) == ""
		audit.MissingDescription = &missing
	}
	if missing, ok := firstReportBool(row, "missingProject", "withoutProject", "without_project"); ok {
		audit.MissingProject = &missing
	} else if _, ok := row["projectId"]; ok {
		missing = view.Entities == nil || view.Entities.Project == nil || view.Entities.Project.ID == ""
		audit.MissingProject = &missing
	}
	if missing, ok := firstReportBool(row, "missingTask", "withoutTask", "without_task"); ok {
		audit.MissingTask = &missing
	} else if _, ok := row["taskId"]; ok {
		missing = view.Entities == nil || view.Entities.Task == nil || view.Entities.Task.ID == ""
		audit.MissingTask = &missing
	}
	if nested, ok := row["audit"].(map[string]any); ok {
		audit.Raw = maps.Clone(nested)
	}
	return audit
}

func entryDurationView(seconds int64, source string) EntryDurationView {
	return EntryDurationView{
		Seconds: seconds,
		Hours:   hours(seconds),
		Display: durationDisplay(seconds),
		Source:  source,
	}
}

func firstReportString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if s := reportValueString(value); s != "" {
				return s
			}
		}
	}
	return ""
}

func reportValueString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

func firstReportBool(row map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := row[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case bool:
			return v, true
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "true":
				return true, true
			case "false":
				return false, true
			}
		}
	}
	return false, false
}

func entryEntitiesEmpty(entities EntryEntitiesView) bool {
	return entities.User == nil && entities.Client == nil && entities.Project == nil && entities.Task == nil && len(entities.Tags) == 0
}

func entryApprovalEmpty(approval EntryApprovalView) bool {
	return approval.RequestID == "" && approval.State == "" && len(approval.Raw) == 0
}

func entryInvoicingEmpty(invoicing EntryInvoicingView) bool {
	return invoicing.Invoiced == nil && invoicing.State == "" && invoicing.InvoiceID == "" && invoicing.InvoiceNumber == "" && len(invoicing.Raw) == 0
}

func entryAuditEmpty(audit EntryAuditView) bool {
	return audit.Locked == nil && audit.MissingDescription == nil && audit.MissingProject == nil && audit.MissingTask == nil && len(audit.Raw) == 0
}

func boolPtrValue(value *bool) bool {
	return value != nil && *value
}

func reportRateBreakdownFromRow(row map[string]any) *ReportRateBreakdownView {
	currency := reportCurrency(row)
	view := ReportRateBreakdownView{
		Rate:         reportMoneyField(row, currency, "rate", "hourlyRate"),
		Amount:       reportMoneyField(row, currency, "amount", "billableAmount"),
		EarnedAmount: reportMoneyField(row, currency, "earnedAmount", "earned_amount"),
		EarnedRate:   reportMoneyField(row, currency, "earnedRate", "earned_rate"),
		CostAmount:   reportMoneyField(row, currency, "costAmount", "cost_amount"),
		CostRate:     reportMoneyField(row, currency, "costRate", "cost_rate"),
		Currency:     currency,
	}
	if view.Rate == nil && view.Amount == nil && view.EarnedAmount == nil && view.EarnedRate == nil && view.CostAmount == nil && view.CostRate == nil {
		return nil
	}
	view.Raw = map[string]any{}
	for _, key := range []string{"rate", "amount", "earnedAmount", "earnedRate", "costAmount", "costRate", "currency"} {
		if value, ok := row[key]; ok {
			view.Raw[key] = value
		}
	}
	return &view
}

func reportMoneyField(row map[string]any, currency string, keys ...string) *MoneyView {
	for _, key := range keys {
		value, ok := row[key]
		if !ok {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			currency = firstNonEmptyString(reportCurrency(nested), currency)
			value = firstPresent(nested, "value", "amount", "total")
		}
		if n, ok := reportNumber(value); ok {
			return moneyPtr(MoneyFromReportNumber(n, currency))
		}
	}
	return nil
}

func reportMoneyByCurrency(row map[string]any) []MoneyByCurrencyView {
	if row == nil {
		return nil
	}
	var out []MoneyByCurrencyView
	addRows := func(kind string, rows []map[string]any) {
		for _, item := range rows {
			value, ok := reportNumber(firstPresent(item, "value", "amount", "total"))
			if !ok {
				continue
			}
			currency := firstNonEmptyString(reportCurrency(item), firstReportString(item, "currencyCode", "currency_code"))
			out = append(out, MoneyByCurrencyView{
				Type:     strings.ToUpper(firstNonEmptyString(kind, firstReportString(item, "type", "amountType", "name"))),
				Currency: currency,
				Money:    MoneyFromReportNumber(value, currency),
				Raw:      maps.Clone(item),
			})
		}
	}
	addRows("", mapSlice(row["amountByCurrency"]))
	addRows("", mapSlice(row["amount_by_currency"]))
	addRows("TOTAL", mapSlice(row["totalAmountByCurrency"]))
	addRows("TOTAL", mapSlice(row["total_amount_by_currency"]))
	for _, amount := range mapSlice(row["amounts"]) {
		kind := strings.ToUpper(firstReportString(amount, "type", "amountType", "name"))
		addRows(kind, mapSlice(amount["amountByCurrency"]))
		addRows(kind, mapSlice(amount["amount_by_currency"]))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
