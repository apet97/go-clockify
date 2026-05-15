package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/paths"
)

type reportEndpoint struct {
	toolName  string
	pathName  string
	filterKey string
}

func (s *Service) AttendanceReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.reportsAPIReport(ctx, args, reportEndpoint{
		toolName:  "clockify_reports_attendance",
		pathName:  "attendance",
		filterKey: "attendance_filter",
	})
}

func (s *Service) reportsAPIReport(ctx context.Context, args map[string]any, endpoint reportEndpoint) (ResultEnvelope, error) {
	wsID := strings.TrimSpace(stringArg(args, "workspace_id"))
	if wsID == "" {
		var err error
		wsID, err = s.ResolveWorkspaceID(ctx)
		if err != nil {
			return ResultEnvelope{}, err
		}
	}
	args, defaultsMeta := s.argsWithUserReportDefaults(ctx, args)
	body, err := buildReportsAPIBody(args, endpoint.filterKey, endpoint.pathName == "weekly", s.DefaultTimezone)
	if err != nil {
		return ResultEnvelope{}, err
	}
	applyReportsAPIMoneyDefaults(body, endpoint)
	path, err := paths.Workspace(wsID, "reports", endpoint.pathName)
	if err != nil {
		return ResultEnvelope{}, err
	}
	data, binary, err := s.postReportsAPI(ctx, path, body)
	if err != nil {
		return ResultEnvelope{}, err
	}
	meta := map[string]any{
		"workspaceId": wsID,
		"source":      "reports-api",
		"exportType":  body["exportType"],
	}
	if defaultsMeta != nil {
		meta["defaults"] = defaultsMeta
	}
	if binary {
		meta["binary"] = true
	} else if endpoint.pathName == "detailed" {
		meta["normalizedEntries"] = appendDetailedReportViews(data)
	} else if endpoint.pathName == "summary" {
		meta["normalizedRollups"] = appendSummaryReportViews(data, body)
	} else if endpoint.pathName == "weekly" {
		meta["normalizedRollups"] = appendWeeklyReportViews(data, body, boolArg(args, "include_future"))
	}
	return ok(endpoint.toolName, data, meta), nil
}

func applyReportsAPIMoneyDefaults(body map[string]any, endpoint reportEndpoint) {
	if endpoint.pathName != "detailed" && endpoint.pathName != "summary" && endpoint.pathName != "weekly" {
		return
	}
	if _, hasShown := body["amountShown"]; hasShown {
		return
	}
	if _, hasAmounts := body["amounts"]; hasAmounts {
		return
	}
	body["amountShown"] = "EARNED"
	body["amounts"] = []string{"EARNED", "COST", "PROFIT"}
}

func (s *Service) postReportsAPI(ctx context.Context, path string, body map[string]any) (map[string]any, bool, error) {
	exportType, _ := body["exportType"].(string)
	switch strings.ToUpper(strings.TrimSpace(exportType)) {
	case "", "JSON", "JSON_V1":
		var out map[string]any
		if err := s.Client.PostReports(ctx, path, body, &out); err != nil {
			return nil, false, err
		}
		return out, false, nil
	default:
		raw, err := s.Client.PostReportsRaw(ctx, path, body)
		if err != nil {
			return nil, false, err
		}
		return documentedRawResponse(raw.Header, raw.Body), true, nil
	}
}

func buildReportsAPIBody(args map[string]any, filterKey string, deriveWeeklyRange bool, defaultTimezone *time.Location) (map[string]any, error) {
	body := map[string]any{"exportType": "JSON"}
	setReportCommonFields(body, args)
	if deriveWeeklyRange {
		if err := setWeeklyDateRange(body, args, defaultTimezone); err != nil {
			return nil, err
		}
	}
	if _, ok := body["dateRangeType"]; !ok {
		if _, hasStart := body["dateRangeStart"]; hasStart {
			if _, hasEnd := body["dateRangeEnd"]; hasEnd {
				body["dateRangeType"] = "ABSOLUTE"
			}
		}
	}
	if filterKey != "" {
		filter, ok, err := mapArg(args, filterKey)
		if err != nil {
			return nil, err
		}
		if !ok {
			filter = defaultReportFilter(filterKey)
		}
		normalized, err := normalizeReportSpecificFilter(filterKey, filter)
		if err != nil {
			return nil, err
		}
		body[snakeReportKeyToCamel(filterKey)] = normalized
	}
	if deriveWeeklyRange {
		if err := validateWeeklyDateRange(body, defaultTimezone); err != nil {
			return nil, err
		}
	}
	return body, nil
}

func defaultReportFilter(filterKey string) map[string]any {
	switch filterKey {
	case "summary_filter":
		return map[string]any{"groups": []any{"PROJECT"}}
	case "weekly_filter":
		return map[string]any{"group": "PROJECT", "subgroup": "TIME"}
	default:
		return map[string]any{}
	}
}

func setReportCommonFields(body map[string]any, args map[string]any) {
	setIfString(body, args, "amount_shown", "amountShown")
	if amounts, ok, err := strictStringSliceArg(args, "amounts"); err == nil && ok {
		body["amounts"] = amounts
	}
	setIfString(body, args, "approval_state", "approvalState")
	setIfBool(body, args, "archived", "archived")
	setIfBool(body, args, "billable", "billable")
	setIfString(body, args, "date_format", "dateFormat")
	setIfString(body, args, "date_range_start", "dateRangeStart")
	setIfString(body, args, "date_range_end", "dateRangeEnd")
	if _, ok := body["dateRangeStart"]; !ok {
		setIfString(body, args, "start", "dateRangeStart")
	}
	if _, ok := body["dateRangeEnd"]; !ok {
		setIfString(body, args, "end", "dateRangeEnd")
	}
	setIfString(body, args, "date_range_type", "dateRangeType")
	setIfString(body, args, "description", "description")
	setIfString(body, args, "export_type", "exportType")
	setIfString(body, args, "invoicing_state", "invoicingState")
	setIfBool(body, args, "rounding", "rounding")
	setIfString(body, args, "sort_order", "sortOrder")
	setIfString(body, args, "time_format", "timeFormat")
	if timezone := strings.TrimSpace(stringArg(args, "time_zone")); timezone != "" {
		body["timeZone"] = timezone
	} else if timezone := strings.TrimSpace(stringArg(args, "timezone")); timezone != "" {
		body["timeZone"] = timezone
	}
	setIfString(body, args, "user_locale", "userLocale")
	setIfString(body, args, "week_start_day", "weekStart")
	setIfBool(body, args, "without_description", "withoutDescription")
	setIfString(body, args, "zoom_level", "zoomLevel")
	for _, spec := range []struct {
		argKey      string
		bodyKey     string
		userStatus  bool
		tagContains bool
	}{
		{"clients", "clients", false, false},
		{"currency", "currency", false, false},
		{"projects", "projects", false, false},
		{"tasks", "tasks", false, false},
		{"user_groups", "userGroups", true, false},
		{"users", "users", true, false},
		{"tags", "tags", false, true},
	} {
		if filter, ok, err := mapArg(args, spec.argKey); err == nil && ok {
			body[spec.bodyKey] = normalizeContainsFilter(filter, spec.userStatus, spec.tagContains)
		}
	}
	if customFields, ok, err := mapSliceArg(args, "custom_fields"); err == nil && ok {
		body["customFields"] = normalizeCustomFieldFilters(customFields)
	}
	if userCustomFields, ok, err := mapSliceArg(args, "user_custom_fields"); err == nil && ok {
		body["userCustomFields"] = normalizeCustomFieldFilters(userCustomFields)
	}
}

func setWeeklyDateRange(body map[string]any, args map[string]any, defaultTimezone *time.Location) error {
	if _, hasStart := body["dateRangeStart"]; hasStart {
		return nil
	}
	loc, err := loadLocation(stringArg(args, "timezone"), defaultTimezone)
	if err != nil {
		return err
	}
	if tz := strings.TrimSpace(stringArg(args, "time_zone")); tz != "" {
		loc, err = loadLocation(tz, defaultTimezone)
		if err != nil {
			return err
		}
	}
	baseText := strings.TrimSpace(stringArg(args, "week_start"))
	var base time.Time
	if baseText == "" {
		base = time.Now().In(loc)
	} else {
		base, err = parseFlexibleDateTime(baseText, loc)
		if err != nil {
			return err
		}
	}
	weekStart := weekStartOffset(stringArg(args, "week_start_day"))
	offset := (weekdayOneBased(base) - weekStart + 7) % 7
	start := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -offset)
	end := start.AddDate(0, 0, 7).Add(-time.Millisecond)
	body["dateRangeStart"] = start.Format("2006-01-02T15:04:05.000")
	body["dateRangeEnd"] = end.Format("2006-01-02T15:04:05.000")
	return nil
}

func normalizeReportSpecificFilter(filterKey string, raw map[string]any) (map[string]any, error) {
	switch filterKey {
	case "attendance_filter":
		return normalizeAttendanceFilter(raw), nil
	case "detailed_filter":
		return normalizeDetailedFilter(raw), nil
	case "summary_filter":
		return normalizeSummaryFilter(raw)
	case "weekly_filter":
		return normalizeWeeklyFilter(raw)
	default:
		return nil, fmt.Errorf("unsupported report filter %s", filterKey)
	}
}

func normalizeAttendanceFilter(raw map[string]any) map[string]any {
	out := map[string]any{}
	for _, spec := range []struct{ in, out string }{
		{"break_filters", "breakFilters"},
		{"capacity_filters", "capacityFilters"},
		{"end_filters", "endFilters"},
		{"overtime_filters", "overtimeFilters"},
		{"start_filters", "startFilters"},
		{"work_filters", "workFilters"},
	} {
		if filters, ok, err := compareFiltersFromMap(raw, spec.in); err == nil && ok {
			out[spec.out] = filters
		}
	}
	if v, ok := boolFromMap(raw, "has_time_off"); ok {
		out["hasTimeOff"] = v
	}
	if v, ok := intFromMap(raw, "page"); ok {
		out["page"] = v
	}
	if v, ok := intFromMap(raw, "page_size"); ok {
		out["pageSize"] = v
	}
	if v := stringFromMap(raw, "sort_column"); v != "" {
		out["sortColumn"] = v
	}
	return out
}

func normalizeDetailedFilter(raw map[string]any) map[string]any {
	out := map[string]any{}
	if audit, ok := nestedMap(raw, "audit_filter", "auditFilter"); ok {
		out["auditFilter"] = normalizeAuditFilter(audit)
	}
	if options, ok := nestedMap(raw, "options", "options"); ok {
		opts := map[string]any{}
		if totals := stringFromMap(options, "totals"); totals != "" {
			opts["totals"] = totals
		}
		out["options"] = opts
	}
	if v, ok := intFromMap(raw, "page"); ok {
		out["page"] = v
	}
	if v, ok := intFromMap(raw, "page_size"); ok {
		out["pageSize"] = v
	}
	if v := stringFromMap(raw, "sort_column"); v != "" {
		out["sortColumn"] = v
	}
	return out
}

func normalizeSummaryFilter(raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	groups, ok, err := stringSliceFromMap(raw, "groups")
	if err != nil {
		return nil, err
	}
	if !ok || len(groups) == 0 {
		groups = []string{"PROJECT"}
	}
	if len(groups) < 1 || len(groups) > 3 {
		return nil, fmt.Errorf("summary_filter.groups must contain 1 to 3 values")
	}
	normalized := make([]string, 0, len(groups))
	for _, group := range groups {
		switch strings.ToUpper(strings.TrimSpace(group)) {
		case "DAY":
			normalized = append(normalized, "DATE")
		case "CLIENT", "PROJECT", "TASK", "DATE", "WEEK", "MONTH", "TIMEENTRY":
			normalized = append(normalized, strings.ToUpper(strings.TrimSpace(group)))
		case "USER":
			return nil, fmt.Errorf("summary_filter.groups does not support USER; use clockify_assignment_report for user-grouped scheduled-vs-tracked joins or clockify_reports_detailed for user rows")
		default:
			return nil, fmt.Errorf("summary_filter.groups contains unsupported value %q", group)
		}
	}
	out["groups"] = normalized
	if v := stringFromMap(raw, "sort_column"); v != "" {
		out["sortColumn"] = v
	}
	if v := stringFromMap(raw, "summary_chart_type"); v != "" {
		out["summaryChartType"] = v
	}
	return out, nil
}

func normalizeWeeklyFilter(raw map[string]any) (map[string]any, error) {
	group := strings.ToUpper(strings.TrimSpace(stringFromMap(raw, "group")))
	subgroup := strings.ToUpper(strings.TrimSpace(stringFromMap(raw, "subgroup")))
	if group == "" {
		group = "PROJECT"
	}
	if subgroup == "" {
		subgroup = "TIME"
	}
	if group != "PROJECT" && group != "USER" {
		return nil, fmt.Errorf("weekly_filter.group must be PROJECT or USER")
	}
	if subgroup != "TIME" {
		return nil, fmt.Errorf("weekly_filter.subgroup must be TIME")
	}
	return map[string]any{"group": group, "subgroup": subgroup}, nil
}

func normalizeAuditFilter(raw map[string]any) map[string]any {
	out := map[string]any{}
	if v, ok := intFromMap(raw, "duration"); ok {
		out["duration"] = v
	}
	if v, ok := boolFromMap(raw, "duration_shorter"); ok {
		out["durationShorter"] = v
	}
	if v, ok := boolFromMap(raw, "without_project"); ok {
		out["withoutProject"] = v
	}
	if v, ok := boolFromMap(raw, "without_task"); ok {
		out["withoutTask"] = v
	}
	return out
}

func normalizeContainsFilter(raw map[string]any, _ bool, tagContains bool) map[string]any {
	out := map[string]any{}
	if v := stringFromMap(raw, "contains"); v != "" {
		out["contains"] = v
	}
	if ids, ok, err := stringSliceFromMap(raw, "ids"); err == nil && ok {
		out["ids"] = ids
	}
	if v := stringFromMap(raw, "status"); v != "" {
		out["status"] = v
	}
	if tagContains {
		if v := stringFromMap(raw, "contained_in_timeentry"); v != "" {
			out["containedInTimeentry"] = v
		}
	}
	return out
}

func normalizeCustomFieldFilters(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		item := map[string]any{}
		if v := stringFromMap(raw, "id"); v != "" {
			item["id"] = v
		}
		if v, ok := boolFromMap(raw, "is_empty"); ok {
			item["isEmpty"] = v
		}
		if v := stringFromMap(raw, "number_condition"); v != "" {
			item["numberCondition"] = v
		}
		if v := stringFromMap(raw, "type"); v != "" {
			item["type"] = v
		}
		if v, ok := raw["value"]; ok {
			item["value"] = v
		}
		out = append(out, item)
	}
	return out
}

func compareFiltersFromMap(raw map[string]any, key string) ([]map[string]any, bool, error) {
	items, ok, err := mapSliceArg(raw, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		normalized := map[string]any{}
		if v := stringFromMap(item, "filtration_type"); v != "" {
			normalized["filtrationType"] = v
		}
		if v := stringFromMap(item, "value"); v != "" {
			normalized["value"] = v
		}
		out = append(out, normalized)
	}
	return out, true, nil
}

func snakeReportKeyToCamel(key string) string {
	switch key {
	case "attendance_filter":
		return "attendanceFilter"
	case "detailed_filter":
		return "detailedFilter"
	case "summary_filter":
		return "summaryFilter"
	case "weekly_filter":
		return "weeklyFilter"
	default:
		return key
	}
}
