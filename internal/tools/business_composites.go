package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/paths"
)

type MoneyReportView struct {
	Range            FinancialRangeView       `json:"range"`
	GroupBy          []string                 `json:"group_by"`
	Rollups          []ReportRollupView       `json:"rollups"`
	GroupTotals      ReportGroupTotalsSummary `json:"group_totals_summary"`
	Totals           ReportTotalsSummary      `json:"totals_summary"`
	SuggestedActions []ToolSuggestion         `json:"suggestedActions,omitempty"`
	Warnings         []string                 `json:"warnings,omitempty"`
	Raw              map[string]any           `json:"raw,omitempty"`
}

type MonthlyBriefView struct {
	Month            string             `json:"month"`
	Range            FinancialRangeView `json:"range"`
	Money            MoneyReportView    `json:"money"`
	Audit            AuditEntriesView   `json:"audit"`
	SuggestedActions []ToolSuggestion   `json:"suggestedActions,omitempty"`
	Warnings         []string           `json:"warnings,omitempty"`
}

type AuditEntriesView struct {
	Range            FinancialRangeView    `json:"range"`
	Entries          []ReportEntryView     `json:"entries,omitempty"`
	EntrySummary     ReportEntrySummary    `json:"entry_summary"`
	ApprovalSummary  ReportApprovalSummary `json:"approval_summary"`
	EntitySummary    ReportEntitySummary   `json:"entity_summary"`
	Issues           []AuditEntryIssue     `json:"issues,omitempty"`
	SuggestedActions []ToolSuggestion      `json:"suggestedActions,omitempty"`
	Warnings         []string              `json:"warnings,omitempty"`
	Raw              map[string]any        `json:"raw,omitempty"`
}

type AuditEntryIssue struct {
	Type      string `json:"type"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	EntryID   string `json:"entry_id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

type UnbilledForClientView struct {
	ClientID         string              `json:"client_id,omitempty"`
	Client           string              `json:"client,omitempty"`
	Range            FinancialRangeView  `json:"range"`
	Entries          []ReportEntryView   `json:"entries,omitempty"`
	EntrySummary     ReportEntrySummary  `json:"entry_summary"`
	Totals           ReportTotalsSummary `json:"totals_summary"`
	Financials       EntryFinancials     `json:"financials"`
	SuggestedActions []ToolSuggestion    `json:"suggestedActions,omitempty"`
	Warnings         []string            `json:"warnings,omitempty"`
	Raw              map[string]any      `json:"raw,omitempty"`
}

func (s *Service) MoneyReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	finRange := compositeFinancialRange(args, time.Now())
	groups := compositeGroupBy(args, []string{"CLIENT", "PROJECT", "TASK"})
	payload, err := s.summaryPayloadForRange(ctx, wsID, finRange, groups, nil)
	if err != nil {
		return ResultEnvelope{}, err
	}
	raw := cloneMap(payload)
	rollups := summaryRollups(payload, groups)
	view := MoneyReportView{
		Range:            finRange,
		GroupBy:          groups,
		Rollups:          rollups,
		GroupTotals:      summarizeReportRollups(rollups),
		Totals:           summarizeReportTotals(payload, nil),
		SuggestedActions: reportDrilldownSuggestions(summaryReportBody(finRange, groups, nil)),
		Raw:              raw,
	}
	return ok("clockify_money_report", view, map[string]any{"workspaceId": wsID, "source": "reports-summary-api"}), nil
}

func (s *Service) MonthlyBrief(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	now := time.Now()
	finRange, monthLabel, err := monthlyBriefRange(args, now)
	if err != nil {
		return ResultEnvelope{}, err
	}
	moneyResult, err := s.MoneyReport(ctx, map[string]any{
		"financial_start": finRange.Start,
		"financial_end":   finRange.End,
		"group_by":        []any{"CLIENT", "PROJECT", "TASK"},
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	auditResult, err := s.AuditEntries(ctx, map[string]any{
		"financial_start": finRange.Start,
		"financial_end":   finRange.End,
		"page_size":       intArg(args, "page_size", 100),
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	money, typed := moneyResult.Data.(MoneyReportView)
	if !typed {
		return ResultEnvelope{}, fmt.Errorf("unexpected money report payload %T", moneyResult.Data)
	}
	audit, typed := auditResult.Data.(AuditEntriesView)
	if !typed {
		return ResultEnvelope{}, fmt.Errorf("unexpected audit entries payload %T", auditResult.Data)
	}
	view := MonthlyBriefView{
		Month: monthLabel,
		Range: finRange,
		Money: money,
		Audit: audit,
		SuggestedActions: []ToolSuggestion{
			{Tool: "clockify_money_report", Reason: "Drill into month-to-date money by client, project, and task.", Arguments: map[string]any{"financial_start": finRange.Start, "financial_end": finRange.End}},
			{Tool: "clockify_audit_entries", Reason: "Review entries with missing fields, locked state, approvals, or invoicing gaps.", Arguments: map[string]any{"financial_start": finRange.Start, "financial_end": finRange.End}},
		},
	}
	return ok("clockify_monthly_brief", view, moneyResult.Meta), nil
}

func (s *Service) AuditEntries(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	finRange := compositeFinancialRange(args, time.Now())
	payload, err := s.detailedPayloadForRange(ctx, wsID, finRange, args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	raw := cloneMap(payload)
	views := normalizeDetailedReportPayload(payload)
	issues := auditIssuesFromReportEntries(views.Entries)
	view := AuditEntriesView{
		Range:            finRange,
		Entries:          views.Entries,
		EntrySummary:     views.EntrySummary,
		ApprovalSummary:  views.ApprovalSummary,
		EntitySummary:    views.EntitySummary,
		Issues:           issues,
		SuggestedActions: detailedReportSuggestions(views.Entries),
		Raw:              raw,
	}
	return ok("clockify_audit_entries", view, map[string]any{"workspaceId": wsID, "source": "reports-detailed-api"}), nil
}

func (s *Service) UnbilledForClient(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	clientRef := strings.TrimSpace(stringArg(args, "client"))
	if clientRef == "" {
		return ResultEnvelope{}, fmt.Errorf("client is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	clientID, err := s.resolveClientID(ctx, wsID, clientRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	finRange := compositeFinancialRange(args, time.Now())
	payload, err := s.detailedPayloadForRange(ctx, wsID, finRange, mergeMap(args, map[string]any{
		"clients":         map[string]any{"contains": "CONTAINS", "ids": []string{clientID}, "status": "ALL"},
		"invoicing_state": "UNINVOICED",
	}))
	if err != nil {
		return ResultEnvelope{}, err
	}
	raw := cloneMap(payload)
	views := normalizeDetailedReportPayload(payload)
	view := UnbilledForClientView{
		ClientID:         clientID,
		Client:           clientRef,
		Range:            finRange,
		Entries:          views.Entries,
		EntrySummary:     views.EntrySummary,
		Totals:           views.TotalsSummary,
		Financials:       financialsFromReportEntries(views.Entries),
		SuggestedActions: []ToolSuggestion{{Tool: "clockify_client_report", Reason: "Review full business context for this client before creating an invoice.", Arguments: map[string]any{"client": clientID, "financial_start": finRange.Start, "financial_end": finRange.End}}},
		Raw:              raw,
	}
	return ok("clockify_unbilled_for_client", view, map[string]any{"workspaceId": wsID, "clientId": clientID, "source": "reports-detailed-api"}), nil
}

func (s *Service) summaryPayloadForRange(ctx context.Context, wsID string, finRange FinancialRangeView, groups []string, clients []string) (map[string]any, error) {
	path, err := paths.Workspace(wsID, "reports", "summary")
	if err != nil {
		return nil, err
	}
	body := summaryReportBody(finRange, groups, clients)
	var out map[string]any
	if err := s.Client.PostReports(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func summaryReportBody(finRange FinancialRangeView, groups []string, clients []string) map[string]any {
	body := map[string]any{
		"exportType":     "JSON",
		"dateRangeStart": finRange.Start,
		"dateRangeEnd":   finRange.End,
		"dateRangeType":  firstNonEmptyString(finRange.DateRangeType, "ABSOLUTE"),
		"amountShown":    "EARNED",
		"amounts":        []string{"EARNED", "COST", "PROFIT"},
		"summaryFilter":  map[string]any{"groups": groups},
	}
	if finRange.Timezone != "" {
		body["timeZone"] = finRange.Timezone
	}
	if len(clients) > 0 {
		body["clients"] = map[string]any{"contains": "CONTAINS", "ids": clients, "status": "ALL"}
	}
	return body
}

func (s *Service) detailedPayloadForRange(ctx context.Context, wsID string, finRange FinancialRangeView, args map[string]any) (map[string]any, error) {
	path, err := paths.Workspace(wsID, "reports", "detailed")
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"exportType":     "JSON",
		"dateRangeStart": finRange.Start,
		"dateRangeEnd":   finRange.End,
		"dateRangeType":  firstNonEmptyString(finRange.DateRangeType, "ABSOLUTE"),
		"amountShown":    "EARNED",
		"amounts":        []string{"EARNED", "COST", "PROFIT"},
		"detailedFilter": map[string]any{
			"page":     intArg(args, "page", 1),
			"pageSize": intArg(args, "page_size", 100),
			"options":  map[string]any{"totals": "CALCULATE"},
		},
	}
	if finRange.Timezone != "" {
		body["timeZone"] = finRange.Timezone
	}
	for _, key := range []string{"clients", "projects", "tasks", "users", "tags"} {
		if value, ok := args[key]; ok {
			body[key] = value
		}
	}
	if state := strings.TrimSpace(stringArg(args, "invoicing_state")); state != "" {
		body["invoicingState"] = strings.ToUpper(state)
	}
	var out map[string]any
	if err := s.Client.PostReports(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func compositeFinancialRange(args map[string]any, now time.Time) FinancialRangeView {
	start := strings.TrimSpace(firstNonEmptyString(stringArg(args, "financial_start"), stringArg(args, "start")))
	end := strings.TrimSpace(firstNonEmptyString(stringArg(args, "financial_end"), stringArg(args, "end")))
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if start == "" {
		start = now.AddDate(0, -1, 0).UTC().Format(time.RFC3339)
	}
	if end == "" {
		end = now.UTC().Format(time.RFC3339)
	}
	return FinancialRangeView{
		Start:         start,
		End:           end,
		DateRangeType: firstNonEmptyString(strings.ToUpper(strings.TrimSpace(stringArg(args, "financial_date_range_type"))), "ABSOLUTE"),
		Timezone:      strings.TrimSpace(firstNonEmptyString(stringArg(args, "financial_timezone"), stringArg(args, "timezone"))),
	}
}

func monthlyBriefRange(args map[string]any, now time.Time) (FinancialRangeView, string, error) {
	loc := time.UTC
	if tz := strings.TrimSpace(stringArg(args, "timezone")); tz != "" {
		loaded, err := time.LoadLocation(tz)
		if err != nil {
			return FinancialRangeView{}, "", err
		}
		loc = loaded
	}
	if now.IsZero() {
		now = time.Now()
	}
	rawMonth := strings.TrimSpace(stringArg(args, "month"))
	base := now.In(loc)
	if rawMonth != "" {
		parsed, err := time.ParseInLocation("2006-01", rawMonth, loc)
		if err != nil {
			return FinancialRangeView{}, "", fmt.Errorf("month must be YYYY-MM: %w", err)
		}
		base = parsed
	}
	start := time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0).Add(-time.Millisecond)
	return FinancialRangeView{
		Start:         start.UTC().Format(time.RFC3339),
		End:           end.UTC().Format(time.RFC3339),
		DateRangeType: "ABSOLUTE",
		Timezone:      loc.String(),
	}, start.Format("2006-01"), nil
}

func compositeGroupBy(args map[string]any, fallback []string) []string {
	values := anyStringSlice(firstPresent(args, "group_by", "groups"))
	if len(values) == 0 {
		return append([]string(nil), fallback...)
	}
	out := make([]string, 0, len(values))
	allowed := map[string]bool{"CLIENT": true, "PROJECT": true, "TASK": true, "DATE": true, "WEEK": true, "MONTH": true, "TIMEENTRY": true}
	for _, value := range values {
		group := strings.ToUpper(strings.TrimSpace(value))
		if group == "DAY" {
			group = "DATE"
		}
		if !allowed[group] {
			continue
		}
		out = append(out, group)
		if len(out) == 3 {
			break
		}
	}
	if len(out) == 0 {
		return append([]string(nil), fallback...)
	}
	return out
}

func auditIssuesFromReportEntries(entries []ReportEntryView) []AuditEntryIssue {
	var issues []AuditEntryIssue
	for _, entry := range entries {
		projectID, userID := "", ""
		if entry.Entities != nil {
			if entry.Entities.Project != nil {
				projectID = entry.Entities.Project.ID
			}
			if entry.Entities.User != nil {
				userID = entry.Entities.User.ID
			}
		}
		add := func(kind, message string) {
			issues = append(issues, AuditEntryIssue{Type: kind, Severity: "warning", Message: message, EntryID: entry.ID, ProjectID: projectID, UserID: userID})
		}
		if entry.Audit != nil {
			if boolPtrValue(entry.Audit.MissingDescription) {
				add("missing_description", "Entry is missing a description.")
			}
			if boolPtrValue(entry.Audit.MissingProject) {
				add("missing_project", "Entry is missing a project.")
			}
			if boolPtrValue(entry.Audit.MissingTask) {
				add("missing_task", "Entry is missing a task.")
			}
			if boolPtrValue(entry.Audit.Locked) {
				add("locked", "Entry is locked.")
			}
		}
		if entry.BillableState == billableStateUnset {
			add("billable_unset", "Entry did not expose a billable value in the report row.")
		}
		if entry.Invoicing != nil && strings.EqualFold(entry.Invoicing.State, "UNINVOICED") {
			add("uninvoiced", "Entry is not invoiced.")
		}
	}
	return issues
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeMap(base map[string]any, extra map[string]any) map[string]any {
	out := cloneMap(base)
	if out == nil {
		out = map[string]any{}
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func moneyReportInputSchema() map[string]any {
	props := map[string]any{
		"group_by": map[string]any{"type": "array", "description": "Up to 3 Summary Reports groups. Extra values are ignored.", "items": map[string]any{"type": "string", "enum": []string{"CLIENT", "PROJECT", "TASK", "DATE", "WEEK", "MONTH", "TIMEENTRY", "DAY"}}},
	}
	addFinancialRangeInputProperties(props)
	return map[string]any{"type": "object", "properties": props}
}

func monthlyBriefInputSchema() map[string]any {
	props := map[string]any{
		"month":     map[string]any{"type": "string", "description": "YYYY-MM month. Defaults to the current month in timezone."},
		"timezone":  timezoneInputProperty(),
		"page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
	}
	return map[string]any{"type": "object", "properties": props}
}

func auditEntriesInputSchema() map[string]any {
	props := map[string]any{
		"page":      map[string]any{"type": "integer", "minimum": 1},
		"page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
	}
	addFinancialRangeInputProperties(props)
	return map[string]any{"type": "object", "properties": props}
}

func unbilledForClientInputSchema() map[string]any {
	props := map[string]any{
		"client":    map[string]any{"type": "string", "description": "Client name or ID"},
		"page":      map[string]any{"type": "integer", "minimum": 1},
		"page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
	}
	addFinancialRangeInputProperties(props)
	return map[string]any{"type": "object", "required": []string{"client"}, "properties": props}
}
