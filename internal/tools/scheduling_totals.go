package tools

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/apet97/go-clockify/internal/paths"
)

func (s *Service) getWorkspaceScheduleUserTotals(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	totals, err := s.getWorkspaceScheduleUserTotalsRaw(ctx, wsID, args)
	if err != nil {
		return ToolResult{}, err
	}
	views := make([]map[string]any, 0, len(totals))
	for _, total := range totals {
		views = append(views, scheduleTotalView(total, "USER"))
	}
	return ok("clockify_get_workspace_schedule_user_totals", views, map[string]any{
		"workspaceId": wsID,
		"count":       len(totals),
	}), nil
}

func (s *Service) schedulingUserTotalsOneUser(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	userRef := stringArg(args, "user_id")
	if userRef == "" {
		return ToolResult{}, fmt.Errorf("user_id is required")
	}
	userID, err := s.resolveUserID(ctx, wsID, userRef)
	if err != nil {
		return ToolResult{}, err
	}
	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return ToolResult{}, err
	}
	query := map[string]string{"start": start, "end": end}
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "users", userID, "totals")
	if err != nil {
		return ToolResult{}, err
	}
	var raw any
	if err := s.Client.Get(ctx, path, query, &raw); err != nil {
		return ToolResult{}, err
	}
	var data any
	if capacity, ok := raw.(map[string]any); ok {
		data, _ = s.enrichScheduleCapacity(ctx, wsID, capacity, args)
	} else if rows := mapSlice(raw); len(rows) > 0 {
		data = scheduleTotalView(rows[0], "USER")
	} else {
		data = raw
	}
	return ok("clockify_scheduling_user_totals", data, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"start":       start,
		"end":         end,
	}), nil
}

func (s *Service) schedulingCapacityOneUser(ctx context.Context, args map[string]any) (ToolResult, error) {
	userIDs, _, err := strictStringSliceArg(args, "user_ids")
	if err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	rawArgs := copyArgs(args)
	if len(userIDs) > 0 {
		rawArgs["users"] = userIDs
	}
	totals, err := s.getWorkspaceScheduleUserTotalsRaw(ctx, wsID, rawArgs)
	if err != nil {
		return ToolResult{}, err
	}
	views := make([]map[string]any, 0, len(totals))
	for _, total := range totals {
		views = append(views, scheduleTotalView(total, "USER"))
	}
	return ok("clockify_scheduling_capacity", views, map[string]any{
		"note":        "total_hours_by_day shows scheduled hours per calendar day; capacity_by_day is populated only when the workspace has work-pattern capacity data configured.",
		"workspaceId": wsID,
		"userIds":     userIDs,
		"count":       len(totals),
	}), nil
}

func (s *Service) getWorkspaceScheduleUserTotalsRaw(ctx context.Context, wsID string, args map[string]any) ([]map[string]any, error) {
	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"start":    start,
		"end":      end,
		"page":     max(intArg(args, "page", 1), 1),
		"pageSize": min(max(intArg(args, "page_size", 50), 1), 200),
	}
	if search := strings.TrimSpace(stringArg(args, "search")); search != "" {
		body["search"] = search
	}
	if status := strings.ToUpper(strings.TrimSpace(stringArg(args, "status_filter"))); status != "" {
		body["statusFilter"] = status
	}
	if filter := scheduleUserFilter(args, "users"); filter != nil {
		body["userFilter"] = filter
	}
	if filter := scheduleUserFilter(args, "user_groups"); filter != nil {
		body["userGroupFilter"] = filter
	}
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "user-filter", "totals")
	if err != nil {
		return nil, err
	}
	var totals []map[string]any
	if err := s.Client.Post(ctx, path, body, &totals); err != nil {
		return nil, err
	}
	return totals, nil
}

func (s *Service) getSingleProjectScheduleTotals(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	projectID, err := s.resolveProjectID(ctx, wsID, stringArg(args, "project_id"))
	if err != nil {
		return ToolResult{}, err
	}
	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return ToolResult{}, err
	}
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "projects", "totals", projectID)
	if err != nil {
		return ToolResult{}, err
	}
	var total map[string]any
	if err := s.Client.Get(ctx, path, map[string]string{"start": start, "end": end}, &total); err != nil {
		return ToolResult{}, err
	}
	enriched, meta := s.enrichProjectScheduleTotals(ctx, wsID, []map[string]any{total}, args)
	if len(enriched) > 0 {
		total = enriched[0]
	}
	meta["workspaceId"] = wsID
	meta["projectId"] = projectID
	return ok("clockify_get_single_project_schedule_totals", total, meta), nil
}

func scheduleUserFilter(args map[string]any, key string) map[string]any {
	ids, ok, err := strictStringSliceArg(args, key)
	if err != nil || !ok || len(ids) == 0 {
		return nil
	}
	return map[string]any{"contains": "CONTAINS", "ids": ids, "status": "ALL"}
}

func (s *Service) enrichProjectScheduleTotals(ctx context.Context, wsID string, totals []map[string]any, args map[string]any) ([]map[string]any, map[string]any) {
	meta := map[string]any{"source": entryFinancialSourceUnavailable}
	out := make([]map[string]any, 0, len(totals))
	start, end, _ := assignmentRangeFromArgs(args, s.DefaultTimezone)
	tracked, err := s.reportFinancialsForAssignmentGroups(ctx, wsID, []string{"PROJECT"}, start, end, args)
	matched := 0
	for _, total := range totals {
		copyRow := map[string]any{}
		for k, v := range total {
			copyRow[k] = v
		}
		projectID := cleanReportID(firstPresent(copyRow, "projectId", "project_id"))
		key := "PROJECT=" + firstNonEmptyString(projectID, cleanReportID(firstPresent(copyRow, "projectName", "project_name")), "(without project)")
		if value, ok := tracked[key]; ok {
			copyRow["tracked"] = map[string]any{"duration": durationView(value.trackedSeconds), "amount": value.amountTracked, "cost": value.costTracked, "profit": value.realizedProfit, "source": entryFinancialSourceReportsAPI}
			scheduled := scheduleSecondsFromTotal(copyRow)
			copyRow["variance"] = map[string]any{"duration": durationView(scheduled - value.trackedSeconds), "source": "scheduled_minus_tracked"}
			matched++
		}
		out = append(out, scheduleTotalView(copyRow, "PROJECT"))
	}
	meta["reports_api_matched"] = matched
	if matched > 0 {
		meta["source"] = entryFinancialSourceReportsAPI
	}
	if err != nil {
		meta["reports_api_error"] = err.Error()
	}
	return out, meta
}

func (s *Service) enrichScheduleCapacity(ctx context.Context, wsID string, capacity map[string]any, args map[string]any) (map[string]any, map[string]any) {
	meta := map[string]any{"source": entryFinancialSourceUnavailable}
	out := map[string]any{}
	for k, v := range capacity {
		out[k] = v
	}
	start, end, _ := assignmentRangeFromArgs(args, s.DefaultTimezone)
	tracked, err := s.reportFinancialsForAssignmentGroups(ctx, wsID, []string{"USER"}, start, end, args)
	userID := cleanReportID(firstPresent(capacity, "userId", "user_id"))
	key := "USER=" + firstNonEmptyString(userID, cleanReportID(firstPresent(capacity, "userName", "user_name")), "(without user)")
	if value, ok := tracked[key]; ok {
		out["tracked"] = map[string]any{"duration": durationView(value.trackedSeconds), "amount": value.amountTracked, "cost": value.costTracked, "profit": value.realizedProfit, "source": entryFinancialSourceReportsAPI}
		meta["reports_api_matched"] = 1
		meta["source"] = entryFinancialSourceReportsAPI
	} else {
		meta["reports_api_matched"] = 0
	}
	if err != nil {
		meta["reports_api_error"] = err.Error()
	}
	return scheduleTotalView(out, "USER"), meta
}

func scheduleTotalView(raw map[string]any, kind string) map[string]any {
	view := map[string]any{}
	for k, v := range raw {
		view[k] = v
	}
	for _, key := range []string{
		"capacityPerDay", "capacity_per_day",
		"totalHoursPerDay", "total_hours_per_day",
		"workingDays", "working_days",
		"userStatus", "user_status",
		"userImage", "user_image",
		"assignments", "assignmentDays",
		"raw",
	} {
		delete(view, key)
	}
	scheduled := scheduleSecondsFromTotal(raw)
	view["scheduled"] = map[string]any{
		"duration": durationView(scheduled),
		"source":   "scheduling_api",
	}
	capacity := capacitySecondsFromUserTotal(raw)
	if capacity == 0 {
		capacity = int64(math.Round(numberOrZero(firstPresent(raw, "capacitySeconds", "availableSeconds"))))
	}
	if capacity != 0 {
		view["capacity"] = map[string]any{
			"duration": durationView(capacity),
			"source":   "scheduling_api",
		}
		view["availability"] = map[string]any{
			"duration": durationView(capacity - scheduled),
			"source":   "capacity_minus_scheduled",
		}
	}
	view["entities"] = assignmentEntities(raw)
	view["capacity_by_day"] = scheduleDayDurations(firstPresent(raw, "capacityPerDay", "capacity_per_day"), "capacity")
	view["total_hours_by_day"] = scheduleDayDurations(firstPresent(raw, "totalHoursPerDay", "total_hours_per_day"), "total")
	view["working_days"] = firstPresent(raw, "workingDays", "working_days")
	view["user_status"] = firstReportString(raw, "userStatus", "user_status")
	view["user_image"] = firstReportString(raw, "userImage", "user_image")
	if assignments := scheduleProjectHeatmap(raw); len(assignments) > 0 {
		view["project_heatmap"] = assignments
	}
	view["schedule_total_kind"] = kind
	return view
}

func scheduleDayDurations(raw any, label string) []map[string]any {
	rows := mapSlice(raw)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		seconds := int64(0)
		if n, ok := reportNumber(firstPresent(row, "seconds", label+"Seconds")); ok {
			seconds = int64(math.Round(n))
		} else if n, ok := reportNumber(firstPresent(row, "totalHours", "hours", "capacityHours")); ok {
			seconds = int64(math.Round(n * 3600))
		}
		out = append(out, map[string]any{
			"date":     firstReportString(row, "date", "day"),
			"duration": durationView(seconds),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func scheduleProjectHeatmap(raw map[string]any) []map[string]any {
	rows := mapSlice(firstPresent(raw, "assignments", "assignmentDays"))
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"date":           firstReportString(row, "date", "day"),
			"has_assignment": firstPresent(row, "hasAssignment", "has_assignment"),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func numberOrZero(v any) float64 {
	if n, ok := reportNumber(v); ok {
		return n
	}
	return 0
}

func scheduleSecondsFromTotal(row map[string]any) int64 {
	if n, ok := reportNumber(firstPresent(row, "totalSeconds", "scheduledSeconds")); ok {
		return int64(math.Round(n))
	}
	if n, ok := reportNumber(firstPresent(row, "totalHours", "scheduledHours")); ok {
		return int64(math.Round(n * 3600))
	}
	return 0
}

func durationView(seconds int64) AssignmentDurationView {
	return AssignmentDurationView{
		Seconds: seconds,
		Hours:   hours(seconds),
		Display: durationDisplay(seconds),
	}
}

func durationDisplay(seconds int64) string {
	sign := ""
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	return fmt.Sprintf("%s%d:%02d", sign, h, m)
}

func durationSecondsFromNested(v any) int64 {
	m, ok := v.(map[string]any)
	if !ok {
		return 0
	}
	if d, ok := m["duration"].(AssignmentDurationView); ok {
		return d.Seconds
	}
	if nested, ok := m["duration"].(map[string]any); ok {
		if n, ok := reportNumber(nested["seconds"]); ok {
			return int64(math.Round(n))
		}
	}
	return 0
}

func assignmentFinancialsMap(earned, cost, profit *MoneyView, source, reason string) map[string]any {
	out := map[string]any{"source": source}
	if earned != nil {
		out["earned"] = earned
	}
	if cost != nil {
		out["cost"] = cost
	}
	if profit != nil {
		out["profit"] = profit
	}
	if reason != "" {
		out["reason"] = reason
	}
	return out
}

func containsGroup(groups []string, want string) bool {
	for _, group := range groups {
		if group == want {
			return true
		}
	}
	return false
}
