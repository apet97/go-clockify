package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/paths"
)

func (s *Service) reportFinancialsForAssignmentReport(ctx context.Context, wsID string, groups []string, start, end time.Time, args map[string]any) (map[string]assignmentReportAccumulator, error) {
	return s.reportFinancialsForAssignmentGroups(ctx, wsID, groups, start, end, args)
}

func (s *Service) reportFinancialsForAssignmentViews(ctx context.Context, wsID string, _ []AssignmentView, groups []string, start, end time.Time, args map[string]any) (map[string]assignmentReportAccumulator, error) {
	return s.reportFinancialsForAssignmentGroups(ctx, wsID, groups, start, end, args)
}

func (s *Service) reportFinancialsForAssignmentGroups(ctx context.Context, wsID string, groups []string, start, end time.Time, args map[string]any) (map[string]assignmentReportAccumulator, error) {
	out := map[string]assignmentReportAccumulator{}
	if s == nil || s.Client == nil || !s.entryFinancialReportsEnabled() {
		return out, fmt.Errorf("reports API enrichment disabled for non-canonical Clockify base URL")
	}
	if !assignmentGroupsUseSummary(groups) {
		return s.reportFinancialsForAssignmentGroupsDetailed(ctx, wsID, groups, start, end, args)
	}
	path, err := paths.Workspace(wsID, "reports", "summary")
	if err != nil {
		return out, err
	}
	body := map[string]any{
		"dateRangeStart": start.Format(time.RFC3339),
		"dateRangeEnd":   end.Format(time.RFC3339),
		"dateRangeType":  "ABSOLUTE",
		"amountShown":    "EARNED",
		"amounts":        []string{"EARNED", "COST", "PROFIT"},
		"summaryFilter":  map[string]any{"groups": groups},
	}
	if tz := strings.TrimSpace(stringArg(args, "timezone")); tz != "" {
		body["timeZone"] = tz
	}
	applyAssignmentReportFilters(body, args)
	var payload map[string]any
	if err := s.Client.PostReports(ctx, path, body, &payload); err != nil {
		return out, err
	}
	collectAssignmentReportRows(payload, groups, out)
	return out, nil
}

func assignmentGroupsUseSummary(groups []string) bool {
	if len(groups) == 0 || len(groups) > 3 {
		return false
	}
	for _, group := range groups {
		switch strings.ToUpper(strings.TrimSpace(group)) {
		case "CLIENT", "PROJECT", "TASK", "DATE", "WEEK", "MONTH", "TIMEENTRY":
		default:
			return false
		}
	}
	return true
}

func (s *Service) reportFinancialsForAssignmentGroupsDetailed(ctx context.Context, wsID string, groups []string, start, end time.Time, args map[string]any) (map[string]assignmentReportAccumulator, error) {
	out := map[string]assignmentReportAccumulator{}
	path, err := paths.Workspace(wsID, "reports", "detailed")
	if err != nil {
		return out, err
	}
	pageSize := min(max(intArg(args, "page_size", 100), 1), 1000)
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
		if tz := strings.TrimSpace(stringArg(args, "timezone")); tz != "" {
			body["timeZone"] = tz
		}
		applyAssignmentReportFilters(body, args)
		var payload map[string]any
		if err := s.Client.PostReports(ctx, path, body, &payload); err != nil {
			return out, err
		}
		rows := reportTimeEntryRows(payload)
		for _, item := range rows {
			keyMap := assignmentGroupKeyForRaw(item, groups, start, s.DefaultTimezone)
			key := reportGroupKeyString(keyMap, groups)
			row := out[key]
			row.key = key
			row.groupKey = keyMap
			if row.raw == nil {
				row.raw = map[string]any{"reports_detailed": []map[string]any{}}
			}
			if list, ok := row.raw["reports_detailed"].([]map[string]any); ok {
				row.raw["reports_detailed"] = append(list, item)
			}
			row.trackedSeconds += reportDurationSecondsOrZero(item)
			money := reportMoneyFromRow(item)
			row.amountTracked = moneySum(row.amountTracked, money.earned)
			row.costTracked = moneySum(row.costTracked, money.cost)
			row.realizedProfit, _ = profitMoney(row.amountTracked, row.costTracked, money.reason)
			row.entities = mergeAssignmentEntities(row.entities, reportEntities(item))
			row.sourceAppend("reports_detailed")
			out[key] = row
		}
		if len(rows) < pageSize {
			return out, nil
		}
	}
	return out, fmt.Errorf("reports API detailed pagination safety stop reached at %d pages", aggregatePageSafetyStop)
}

func applyAssignmentReportFilters(body map[string]any, args map[string]any) {
	for _, spec := range []struct {
		arg    string
		body   string
		status string
	}{
		{"users", "users", "ALL"},
		{"user_groups", "userGroups", "ALL"},
		{"projects", "projects", "ALL"},
		{"clients", "clients", "ALL"},
		{"tasks", "tasks", "ALL"},
	} {
		ids, ok, err := strictStringSliceArg(args, spec.arg)
		if err == nil && ok && len(ids) > 0 {
			body[spec.body] = map[string]any{"contains": "CONTAINS", "ids": ids, "status": spec.status}
		}
	}
}

func collectAssignmentReportRows(v any, groups []string, out map[string]assignmentReportAccumulator) {
	switch item := v.(type) {
	case []any:
		for _, child := range item {
			collectAssignmentReportRows(child, groups, out)
		}
	case map[string]any:
		if looksLikeSummaryGroup(item) {
			keyMap := reportGroupKeyForRow(item, groups)
			key := reportGroupKeyString(keyMap, groups)
			row := out[key]
			row.key = key
			row.groupKey = keyMap
			row.trackedSeconds += reportDurationSecondsOrZero(item)
			money := reportMoneyFromRow(item)
			row.amountTracked = moneySum(row.amountTracked, money.earned)
			row.costTracked = moneySum(row.costTracked, money.cost)
			row.realizedProfit, _ = profitMoney(row.amountTracked, row.costTracked, money.reason)
			row.entities = reportEntities(item)
			if row.raw == nil {
				row.raw = map[string]any{}
			}
			row.raw["report"] = item
			out[key] = row
		}
		for _, child := range item {
			collectAssignmentReportRows(child, groups, out)
		}
	}
}

func looksLikeSummaryGroup(row map[string]any) bool {
	if _, ok := reportDurationSeconds(row); ok {
		return true
	}
	return reportMoneyFromRow(row).hasMoney()
}

func reportDurationSecondsOrZero(row map[string]any) int64 {
	if seconds, ok := reportDurationSeconds(row); ok {
		return seconds
	}
	return 0
}

func reportGroupKeyForRow(row map[string]any, groups []string) map[string]string {
	out := map[string]string{}
	for _, group := range groups {
		out[group] = reportGroupValue(row, group)
	}
	return out
}

func reportGroupValue(row map[string]any, group string) string {
	kind := strings.ToLower(group)
	switch group {
	case "DATE", "WEEK", "MONTH":
		if value := reportTemporalGroupValue(row, group); value != "" {
			return value
		}
	}
	if id := reportGroupID(row, kind); id != "" {
		return id
	}
	for _, key := range []string{kind + "Name", kind + "_name", kind, "name", "title"} {
		if s := cleanReportID(row[key]); s != "" {
			return s
		}
	}
	return "(without " + strings.ToLower(group) + ")"
}

func reportTemporalGroupValue(row map[string]any, group string) string {
	for _, key := range []string{strings.ToLower(group), "date", "start", "time", "name", "title"} {
		if text := cleanReportID(row[key]); text != "" {
			if t, ok := parseAssignmentTime(text); ok {
				switch group {
				case "DATE":
					return t.Format("2006-01-02")
				case "WEEK":
					year, week := t.ISOWeek()
					return fmt.Sprintf("%04d-W%02d", year, week)
				case "MONTH":
					return t.Format("2006-01")
				}
			}
			return text
		}
	}
	if interval, ok := row["timeInterval"].(map[string]any); ok {
		if text := cleanReportID(firstPresent(interval, "start")); text != "" {
			if t, ok := parseAssignmentTime(text); ok {
				switch group {
				case "DATE":
					return t.Format("2006-01-02")
				case "WEEK":
					year, week := t.ISOWeek()
					return fmt.Sprintf("%04d-W%02d", year, week)
				case "MONTH":
					return t.Format("2006-01")
				}
			}
		}
	}
	return ""
}

func reportGroupKeyString(groupKey map[string]string, groups []string) string {
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		parts = append(parts, group+"="+groupKey[group])
	}
	return strings.Join(parts, "|")
}

func reportEntities(row map[string]any) map[string]any {
	return map[string]any{
		"user":    nestedReportEntity(row, "user"),
		"project": nestedReportEntity(row, "project"),
		"client":  nestedReportEntity(row, "client"),
		"task":    nestedReportEntity(row, "task"),
	}
}

func mergeAssignmentEntities(a, b map[string]any) map[string]any {
	if a == nil {
		a = map[string]any{}
	}
	for key, value := range b {
		if existing, ok := a[key].(map[string]any); ok {
			candidate, _ := value.(map[string]any)
			if cleanReportID(firstPresent(existing, "id", "name")) != "" {
				continue
			}
			if cleanReportID(firstPresent(candidate, "id", "name")) != "" {
				a[key] = candidate
			}
			continue
		}
		a[key] = value
	}
	return a
}

func nestedReportEntity(row map[string]any, kind string) map[string]any {
	id := reportEntityID(row, kind)
	name := cleanReportID(firstPresent(row, kind+"Name", kind+"_name"))
	if nested, ok := row[kind].(map[string]any); ok {
		id = firstNonEmptyString(id, cleanReportID(firstPresent(nested, "id", "_id")))
		name = firstNonEmptyString(name, cleanReportID(firstPresent(nested, "name", "title")))
	}
	return map[string]any{"id": id, "name": name}
}

func reportEntityID(row map[string]any, kind string) string {
	for _, key := range []string{kind + "Id", kind + "_id"} {
		if id := cleanReportID(row[key]); id != "" {
			return id
		}
	}
	if nested, ok := row[kind].(map[string]any); ok {
		return cleanReportID(firstPresent(nested, "id", "_id"))
	}
	return ""
}

func materializeAssignmentReportRows(rowsByKey map[string]*assignmentReportAccumulator) []AssignmentReportRow {
	keys := make([]string, 0, len(rowsByKey))
	for key := range rowsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]AssignmentReportRow, 0, len(keys))
	for _, key := range keys {
		acc := rowsByKey[key]
		amountDiff := moneyDiff(acc.amountScheduled, acc.amountTracked)
		costDiff := moneyDiff(acc.costScheduled, acc.costTracked)
		expectedProfit, expectedReason := profitMoney(acc.amountScheduled, acc.costScheduled, "")
		realizedProfit, realizedReason := profitMoney(acc.amountTracked, acc.costTracked, "")
		reason := firstNonEmptyString(expectedReason, realizedReason)
		row := AssignmentReportRow{
			Title:            assignmentRowTitle(acc.groupKey),
			GroupKey:         acc.groupKey,
			Entities:         acc.entities,
			Scheduled:        durationView(acc.scheduledSeconds),
			Available:        durationView(acc.availableSeconds),
			AmountScheduled:  acc.amountScheduled,
			CostScheduled:    acc.costScheduled,
			ExpectedProfit:   firstMoney(expectedProfit, acc.expectedProfit),
			Tracked:          durationView(acc.trackedSeconds),
			AmountTracked:    acc.amountTracked,
			CostTracked:      acc.costTracked,
			Difference:       durationView(acc.scheduledSeconds - acc.trackedSeconds),
			AmountDifference: amountDiff,
			CostDifference:   costDiff,
			RealizedProfit:   firstMoney(realizedProfit, acc.realizedProfit),
			Status:           acc.status,
			Financials:       AssignmentReportMoney{Earned: acc.amountTracked, Cost: acc.costTracked, Profit: firstMoney(realizedProfit, acc.realizedProfit), Source: assignmentFinancialSource(acc), Reason: reason},
			Source:           assignmentRowSource(acc),
			Warnings:         acc.warnings,
			Raw:              acc.raw,
		}
		rows = append(rows, row)
	}
	return rows
}

func assignmentRowTitle(groupKey map[string]string) string {
	parts := make([]string, 0, len(groupKey))
	for _, group := range assignmentReportGroups {
		if v := groupKey[group]; v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " / ")
}

func assignmentFinancialSource(acc *assignmentReportAccumulator) string {
	if acc == nil {
		return entryFinancialSourceUnavailable
	}
	if acc.amountTracked != nil || acc.costTracked != nil {
		return entryFinancialSourceReportsAPI
	}
	return entryFinancialSourceUnavailable
}

func assignmentRowSource(acc *assignmentReportAccumulator) string {
	if acc == nil {
		return entryFinancialSourceUnavailable
	}
	hasScheduled := acc.scheduledSeconds != 0 || acc.amountScheduled != nil || acc.costScheduled != nil
	hasTracked := acc.trackedSeconds != 0 || acc.amountTracked != nil || acc.costTracked != nil
	switch {
	case hasScheduled && hasTracked:
		return "scheduling_api+reports_api"
	case hasScheduled:
		return "scheduling_api"
	case hasTracked:
		return entryFinancialSourceReportsAPI
	default:
		return entryFinancialSourceUnavailable
	}
}

func assignmentReportTotals(rows []AssignmentReportRow) AssignmentReportTotals {
	var scheduled, available, tracked int64
	var amountScheduled, costScheduled, amountTracked, costTracked *MoneyView
	for _, row := range rows {
		scheduled += row.Scheduled.Seconds
		available += row.Available.Seconds
		tracked += row.Tracked.Seconds
		amountScheduled = moneySum(amountScheduled, row.AmountScheduled)
		costScheduled = moneySum(costScheduled, row.CostScheduled)
		amountTracked = moneySum(amountTracked, row.AmountTracked)
		costTracked = moneySum(costTracked, row.CostTracked)
	}
	expectedProfit, _ := profitMoney(amountScheduled, costScheduled, "")
	realizedProfit, _ := profitMoney(amountTracked, costTracked, "")
	return AssignmentReportTotals{
		Scheduled:        durationView(scheduled),
		Available:        durationView(available),
		Tracked:          durationView(tracked),
		Difference:       durationView(scheduled - tracked),
		AmountScheduled:  amountScheduled,
		CostScheduled:    costScheduled,
		ExpectedProfit:   expectedProfit,
		AmountTracked:    amountTracked,
		CostTracked:      costTracked,
		AmountDifference: moneyDiff(amountScheduled, amountTracked),
		CostDifference:   moneyDiff(costScheduled, costTracked),
		RealizedProfit:   realizedProfit,
	}
}

func trackedRawRows(rows map[string]assignmentReportAccumulator) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row.raw != nil {
			out = append(out, row.raw)
		}
	}
	return out
}
