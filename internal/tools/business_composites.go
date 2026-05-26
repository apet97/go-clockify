package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/paths"
)

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

func (s *Service) UnbilledForClient(ctx context.Context, args map[string]any) (ToolResult, error) {
	clientRef := strings.TrimSpace(stringArg(args, "client"))
	if clientRef == "" {
		return ToolResult{}, fmt.Errorf("client is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	clientID, err := s.resolveClientID(ctx, wsID, clientRef)
	if err != nil {
		return ToolResult{}, err
	}
	finRange := compositeFinancialRange(args, time.Now())
	payload, err := s.detailedPayloadForRange(ctx, wsID, finRange, mergeMap(args, map[string]any{
		"clients":         map[string]any{"contains": "CONTAINS", "ids": []string{clientID}, "status": "ALL"},
		"invoicing_state": "UNINVOICED",
	}))
	if err != nil {
		return ToolResult{}, err
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
		SuggestedActions: []ToolSuggestion{{Tool: "clockify_reports_detailed", Reason: "Review full business context for this client before creating an invoice.", Arguments: map[string]any{"client": clientID, "financial_start": finRange.Start, "financial_end": finRange.End}}},
		Raw:              raw,
	}
	return ok("clockify_unbilled_for_client", view, map[string]any{"workspaceId": wsID, "clientId": clientID, "source": "reports-detailed-api"}), nil
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
	setContainsFilterFromID(body, args, "client_id", "clients", false)
	setContainsFilterFromID(body, args, "project_id", "projects", false)
	setContainsFilterFromID(body, args, "task_id", "tasks", false)
	setContainsFilterFromID(body, args, "user_id", "users", true)
	if tagIDs := anyStringSlice(firstPresent(args, "tag_ids")); len(tagIDs) > 0 {
		body["tags"] = map[string]any{"contains": "CONTAINS", "ids": tagIDs, "containedInTimeentry": "CONTAINS"}
	}
	if v, ok := args["billable"].(bool); ok {
		body["billable"] = v
	}
	if description := strings.TrimSpace(stringArg(args, "description")); description != "" {
		body["description"] = description
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

func setContainsFilterFromID(body map[string]any, args map[string]any, argKey, bodyKey string, userStatus bool) {
	id := strings.TrimSpace(stringArg(args, argKey))
	if id == "" {
		return
	}
	filter := map[string]any{"contains": "CONTAINS", "ids": []string{id}}
	if userStatus {
		filter["status"] = "ALL"
	}
	body[bodyKey] = filter
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

func unbilledForClientInputSchema() map[string]any {
	props := map[string]any{
		"client":    map[string]any{"type": "string", "description": "Client name or ID"},
		"page":      map[string]any{"type": "integer", "minimum": 1},
		"page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
	}
	addFinancialRangeInputProperties(props)
	return map[string]any{"type": "object", "required": []string{"client"}, "properties": props}
}
