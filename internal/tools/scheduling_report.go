package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *Service) AssignmentReport(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	args, err = s.assignmentReportArgsWithResolvedFilters(ctx, wsID, args)
	if err != nil {
		return ToolResult{}, err
	}
	start, end, err := assignmentRangeFromArgs(args, s.DefaultTimezone)
	if err != nil {
		return ToolResult{}, err
	}
	groups, err := assignmentGroupsFromArgs(args)
	if err != nil {
		return ToolResult{}, err
	}
	assignments, page, pageSize, err := s.listAssignmentsRaw(ctx, wsID, args)
	if err != nil {
		return ToolResult{}, err
	}
	userTotals, userTotalsErr := s.getWorkspaceScheduleUserTotalsRaw(ctx, wsID, args)
	projectTotals, _, projectTotalsErr := s.getProjectScheduleTotalsRaw(ctx, wsID, args)
	rowsByKey := scheduledRowsFromAssignments(assignments, groups, start, end, s.DefaultTimezone)
	applyAvailableFromUserTotals(rowsByKey, userTotals, groups)
	applyScheduledMoneyFromRows(rowsByKey, userTotals)
	applyScheduledMoneyFromRows(rowsByKey, projectTotals)
	tracked, reportsErr := s.reportFinancialsForAssignmentReport(ctx, wsID, groups, start, end, args)
	for key, trackedRow := range tracked {
		row := rowsByKey[key]
		if row == nil {
			copyRow := trackedRow
			row = &copyRow
			row.key = key
			row.sourceAppend("reports_api")
			rowsByKey[key] = row
			continue
		}
		row.trackedSeconds += trackedRow.trackedSeconds
		row.amountTracked = moneySum(row.amountTracked, trackedRow.amountTracked)
		row.costTracked = moneySum(row.costTracked, trackedRow.costTracked)
		row.realizedProfit, _ = profitMoney(row.amountTracked, row.costTracked, "")
	}
	if reportsErr != nil {
		for _, row := range rowsByKey {
			row.warnings = append(row.warnings, "reports API enrichment unavailable: "+reportsErr.Error())
		}
	}
	rows := materializeAssignmentReportRows(rowsByKey)
	data := AssignmentReportData{
		Range: FinancialRangeView{
			Start:         start.Format(time.RFC3339),
			End:           end.Format(time.RFC3339),
			DateRangeType: "ABSOLUTE",
			Timezone:      strings.TrimSpace(stringArg(args, "timezone")),
		},
		Groups: groups,
		Rows:   rows,
		Totals: assignmentReportTotals(rows),
		Raw: map[string]any{
			"assignments":     assignments,
			"user_totals":     userTotals,
			"project_totals":  projectTotals,
			"tracked_summary": trackedRawRows(tracked),
		},
	}
	meta := map[string]any{
		"workspaceId": wsID,
		"page":        page,
		"pageSize":    pageSize,
		"count":       len(rows),
	}
	if userTotalsErr != nil {
		meta["user_totals_error"] = userTotalsErr.Error()
	}
	if projectTotalsErr != nil {
		meta["project_totals_error"] = projectTotalsErr.Error()
	}
	if reportsErr != nil {
		meta["reports_api_error"] = reportsErr.Error()
	}
	return ok("clockify_assignment_report", data, meta), nil
}

func (s *Service) assignmentReportArgsWithResolvedFilters(ctx context.Context, wsID string, args map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(args)+2)
	for k, v := range args {
		out[k] = v
	}
	if userRef := stringArg(args, "user_id"); userRef != "" {
		if _, ok, err := strictStringSliceArg(args, "users"); err != nil {
			return nil, err
		} else if !ok {
			userID, err := s.resolveUserID(ctx, wsID, userRef)
			if err != nil {
				return nil, err
			}
			out["users"] = []string{userID}
		}
	}
	if projectRef := stringArg(args, "project_id"); projectRef != "" {
		if _, ok, err := strictStringSliceArg(args, "projects"); err != nil {
			return nil, err
		} else if !ok {
			projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
			if err != nil {
				return nil, err
			}
			out["projects"] = []string{projectID}
		}
	}
	return out, nil
}

func (a *assignmentReportAccumulator) sourceAppend(source string) {
	if a == nil || source == "" {
		return
	}
	if a.raw == nil {
		a.raw = map[string]any{}
	}
	existing, _ := a.raw["sources"].([]string)
	for _, item := range existing {
		if item == source {
			return
		}
	}
	a.raw["sources"] = append(existing, source)
}

func assignmentRangeFromArgs(args map[string]any, defaultTZ *time.Location) (time.Time, time.Time, error) {
	loc, err := loadLocation(stringArg(args, "timezone"), defaultTZ)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	startText, endText, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start, err := time.Parse(time.RFC3339, startText)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := time.Parse(time.RFC3339, endText)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}

func assignmentGroupsFromArgs(args map[string]any) ([]string, error) {
	groups, ok, err := strictStringSliceArg(args, "group_by")
	if err != nil {
		return nil, err
	}
	if !ok || len(groups) == 0 {
		return []string{"USER", "PROJECT", "TASK"}, nil
	}
	if len(groups) > len(assignmentReportGroups) {
		return nil, fmt.Errorf("group_by must contain 1 to %d values", len(assignmentReportGroups))
	}
	allowed := stringSet(assignmentReportGroups)
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		group = strings.ToUpper(strings.TrimSpace(group))
		if _, ok := allowed[group]; !ok {
			return nil, fmt.Errorf("group_by contains unsupported value %q", group)
		}
		out = append(out, group)
	}
	return out, nil
}

func scheduledRowsFromAssignments(assignments []map[string]any, groups []string, start, end time.Time, defaultTZ *time.Location) map[string]*assignmentReportAccumulator {
	rows := map[string]*assignmentReportAccumulator{}
	for _, assignment := range assignments {
		seconds := scheduledSecondsFromAssignment(assignment, start, end)
		key := assignmentReportKeyForRaw(assignment, groups, start, defaultTZ)
		if key == "" {
			key = "(ungrouped)"
		}
		row := rows[key]
		if row == nil {
			groupKey := assignmentGroupKeyForRaw(assignment, groups, start, defaultTZ)
			row = &assignmentReportAccumulator{
				key:      key,
				groupKey: groupKey,
				entities: assignmentEntities(assignment),
				raw:      map[string]any{"assignments": []map[string]any{}},
			}
			rows[key] = row
		}
		row.scheduledSeconds += seconds
		row.status = firstNonEmptyString(row.status, cleanReportID(firstPresent(assignment, "status", "assignmentStatus")))
		if list, ok := row.raw["assignments"].([]map[string]any); ok {
			row.raw["assignments"] = append(list, assignment)
		}
		row.amountScheduled = moneySum(row.amountScheduled, moneyFromAny(firstPresent(assignment, "amountScheduled", "scheduledAmount", "earned", "amount"), reportCurrency(assignment)))
		row.costScheduled = moneySum(row.costScheduled, moneyFromAny(firstPresent(assignment, "costScheduled", "scheduledCost", "cost"), reportCurrency(assignment)))
		row.expectedProfit, _ = profitMoney(row.amountScheduled, row.costScheduled, "")
	}
	return rows
}

func assignmentReportKeyForRaw(raw map[string]any, groups []string, start time.Time, defaultTZ *time.Location) string {
	groupKey := assignmentGroupKeyForRaw(raw, groups, start, defaultTZ)
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		parts = append(parts, group+"="+groupKey[group])
	}
	return strings.Join(parts, "|")
}

func assignmentGroupKeyForRaw(raw map[string]any, groups []string, fallback time.Time, defaultTZ *time.Location) map[string]string {
	out := map[string]string{}
	for _, group := range groups {
		out[group] = assignmentGroupValue(raw, group, fallback, defaultTZ)
	}
	return out
}

func assignmentGroupValue(raw map[string]any, group string, fallback time.Time, defaultTZ *time.Location) string {
	switch group {
	case "USER":
		return firstNonEmptyString(cleanReportID(firstPresent(raw, "userId", "user_id")), cleanReportID(firstPresent(raw, "userName", "user_name", "name")), "(without user)")
	case "PROJECT":
		return firstNonEmptyString(cleanReportID(firstPresent(raw, "projectId", "project_id")), cleanReportID(firstPresent(raw, "projectName", "project_name")), "(without project)")
	case "CLIENT":
		return firstNonEmptyString(cleanReportID(firstPresent(raw, "clientId", "client_id")), cleanReportID(firstPresent(raw, "clientName", "client_name")), "(without client)")
	case "TASK":
		return firstNonEmptyString(cleanReportID(firstPresent(raw, "taskId", "task_id")), cleanReportID(firstPresent(raw, "taskName", "task_name")), "(without task)")
	case "DATE":
		return assignmentBucketTime(raw, fallback, defaultTZ).Format("2006-01-02")
	case "WEEK":
		t := assignmentBucketTime(raw, fallback, defaultTZ)
		year, week := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	case "MONTH":
		return assignmentBucketTime(raw, fallback, defaultTZ).Format("2006-01")
	default:
		return ""
	}
}

func assignmentBucketTime(raw map[string]any, fallback time.Time, _ *time.Location) time.Time {
	for _, key := range []string{"date", "start", "time", "week", "month"} {
		if t, ok := parseAssignmentTime(firstPresent(raw, key)); ok {
			return t
		}
	}
	if interval, ok := raw["timeInterval"].(map[string]any); ok {
		if t, ok := parseAssignmentTime(firstPresent(interval, "start")); ok {
			return t
		}
	}
	if period, ok := raw["period"].(map[string]any); ok {
		if t, ok := parseAssignmentTime(firstPresent(period, "start")); ok {
			return t
		}
	}
	if fallback.IsZero() {
		return time.Now().UTC()
	}
	return fallback
}

func assignmentEntities(raw map[string]any) map[string]any {
	return map[string]any{
		"user": map[string]any{
			"id":   cleanReportID(firstPresent(raw, "userId", "user_id")),
			"name": cleanReportID(firstPresent(raw, "userName", "user_name")),
		},
		"project": map[string]any{
			"id":       cleanReportID(firstPresent(raw, "projectId", "project_id")),
			"name":     cleanReportID(firstPresent(raw, "projectName", "project_name")),
			"clientId": cleanReportID(firstPresent(raw, "clientId", "client_id")),
		},
		"client": map[string]any{
			"id":   cleanReportID(firstPresent(raw, "clientId", "client_id")),
			"name": cleanReportID(firstPresent(raw, "clientName", "client_name")),
		},
		"task": map[string]any{
			"id":   cleanReportID(firstPresent(raw, "taskId", "task_id")),
			"name": cleanReportID(firstPresent(raw, "taskName", "task_name")),
		},
	}
}

func scheduledSecondsFromAssignment(raw map[string]any, rangeStart, rangeEnd time.Time) int64 {
	for _, key := range []string{"totalSeconds", "scheduledSeconds", "durationSeconds"} {
		if n, ok := reportNumber(raw[key]); ok {
			return int64(math.Round(n))
		}
	}
	for _, key := range []string{"totalHours", "scheduledHours", "hours"} {
		if n, ok := reportNumber(raw[key]); ok {
			return int64(math.Round(n * 3600))
		}
	}
	hoursPerDay, ok := reportNumber(firstPresent(raw, "hoursPerDay", "hours_per_day"))
	if !ok || hoursPerDay <= 0 {
		return 0
	}
	start, end := assignmentPeriod(raw, rangeStart, rangeEnd)
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	if start.Before(rangeStart) {
		start = rangeStart
	}
	if end.After(rangeEnd) {
		end = rangeEnd
	}
	if !end.After(start) {
		return 0
	}
	days := inclusiveDays(start, end)
	return int64(math.Round(float64(days) * hoursPerDay * 3600))
}

func assignmentPeriod(raw map[string]any, fallbackStart, fallbackEnd time.Time) (time.Time, time.Time) {
	start, startOK := parseAssignmentTime(firstPresent(raw, "start"))
	end, endOK := parseAssignmentTime(firstPresent(raw, "end"))
	if period, ok := raw["period"].(map[string]any); ok {
		if !startOK {
			start, startOK = parseAssignmentTime(firstPresent(period, "start"))
		}
		if !endOK {
			end, endOK = parseAssignmentTime(firstPresent(period, "end"))
		}
	}
	if !startOK {
		start = fallbackStart
	}
	if !endOK {
		end = fallbackEnd
	}
	return start, end
}

func inclusiveDays(start, end time.Time) int {
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	if end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Hours()/24) + 1
}

func parseAssignmentTime(v any) (time.Time, bool) {
	text := cleanReportID(v)
	if text == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("2006-01-02", text); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

func applyAvailableFromUserTotals(rows map[string]*assignmentReportAccumulator, totals []map[string]any, groups []string) {
	if len(totals) == 0 {
		return
	}
	for _, total := range totals {
		userID := cleanReportID(firstPresent(total, "userId", "user_id"))
		available := capacitySecondsFromUserTotal(total)
		if available == 0 {
			continue
		}
		for _, row := range rows {
			if containsGroup(groups, "USER") && row.groupKey["USER"] != userID && userID != "" {
				continue
			}
			row.availableSeconds += available
		}
	}
}

func capacitySecondsFromUserTotal(total map[string]any) int64 {
	var out int64
	for _, day := range mapSlice(total["totalHoursPerDay"]) {
		if n, ok := reportNumber(firstPresent(day, "totalHours", "availableHours", "capacityHours")); ok {
			out += int64(math.Round(n * 3600))
		}
	}
	if out > 0 {
		return out
	}
	if n, ok := reportNumber(firstPresent(total, "capacity", "available", "availableSeconds")); ok {
		return int64(math.Round(n))
	}
	return 0
}

func applyScheduledMoneyFromRows(rows map[string]*assignmentReportAccumulator, rawRows []map[string]any) {
	for _, raw := range rawRows {
		keyCandidates := []string{
			cleanReportID(firstPresent(raw, "projectId", "project_id")),
			cleanReportID(firstPresent(raw, "userId", "user_id")),
		}
		for _, row := range rows {
			matches := false
			hasCandidate := false
			for _, key := range keyCandidates {
				if key == "" {
					continue
				}
				hasCandidate = true
				if row.groupKey["PROJECT"] == key || row.groupKey["USER"] == key {
					matches = true
				}
			}
			if !matches && hasCandidate {
				continue
			}
			row.amountScheduled = moneySum(row.amountScheduled, moneyFromAny(firstPresent(raw, "amountScheduled", "scheduledAmount", "amount", "earned"), reportCurrency(raw)))
			row.costScheduled = moneySum(row.costScheduled, moneyFromAny(firstPresent(raw, "costScheduled", "scheduledCost", "cost"), reportCurrency(raw)))
			row.expectedProfit, _ = profitMoney(row.amountScheduled, row.costScheduled, "")
		}
	}
}
