package tools

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"
)

// ReportRollupView is a node in a hierarchical report rollup: a group's
// identity, aggregated duration/entries/financials, optional per-currency and
// weekly-day breakdowns, and nested child rollups.
type ReportRollupView struct {
	Group              string                `json:"group,omitempty"`
	Level              int                   `json:"level,omitempty"`
	ID                 string                `json:"id,omitempty"`
	Name               string                `json:"name,omitempty"`
	Duration           EntryDurationView     `json:"duration"`
	Entries            int                   `json:"entries,omitempty"`
	Financials         EntryFinancials       `json:"financials"`
	MoneyByCurrency    []MoneyByCurrencyView `json:"money_by_currency,omitempty"`
	WeeklyDayBreakdown []WeeklyDayTotalView  `json:"weekly_day_breakdown,omitempty"`
	Children           []ReportRollupView    `json:"children,omitempty"`
}

// ReportGroupTotalsSummary aggregates a report's group totals: the group count,
// summed duration and financials, and a by-group-type breakdown.
type ReportGroupTotalsSummary struct {
	Count       int               `json:"count"`
	Duration    EntryDurationView `json:"duration"`
	Financials  EntryFinancials   `json:"financials"`
	ByGroupType map[string]int    `json:"by_group_type,omitempty"`
}

// WeeklyDayTotalView is the per-day total within a weekly report: the date (with
// a future-day flag), duration, financials, and per-currency money.
type WeeklyDayTotalView struct {
	Date            string                `json:"date,omitempty"`
	IsFuture        bool                  `json:"is_future,omitempty"`
	Duration        EntryDurationView     `json:"duration"`
	Financials      EntryFinancials       `json:"financials"`
	MoneyByCurrency []MoneyByCurrencyView `json:"money_by_currency,omitempty"`
}

func (s *Service) argsWithUserReportDefaults(ctx context.Context, args map[string]any) (map[string]any, map[string]any) {
	out := maps.Clone(args)
	if explicitReportTimezone(out) && explicitWeekStartDay(out) {
		return out, nil
	}
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return out, map[string]any{"default_source": "unavailable", "reason": err.Error()}
	}
	defaults := userViewFromUser(user).ReportDefaults
	meta := map[string]any{"default_source": "user_settings"}
	if !explicitReportTimezone(out) && defaults.Timezone != "" {
		out["timezone"] = defaults.Timezone
		meta["timezone"] = defaults.Timezone
	}
	if !explicitWeekStartDay(out) && defaults.WeekStart != "" {
		out["week_start_day"] = strings.ToUpper(defaults.WeekStart)
		meta["week_start"] = strings.ToUpper(defaults.WeekStart)
	}
	if len(meta) == 1 {
		return out, nil
	}
	return out, meta
}

func explicitReportTimezone(args map[string]any) bool {
	return strings.TrimSpace(stringArg(args, "timezone")) != "" || strings.TrimSpace(stringArg(args, "time_zone")) != ""
}

func explicitWeekStartDay(args map[string]any) bool {
	return strings.TrimSpace(stringArg(args, "week_start_day")) != ""
}

func appendSummaryReportViews(data map[string]any, body map[string]any) int {
	if data == nil {
		return 0
	}
	rollups := summaryRollups(data, summaryGroupsFromBody(body))
	data["summary_rollups"] = rollups
	groupTotals := summarizeReportRollups(rollups)
	totals := summarizeReportTotals(data, nil)
	if totals.Financials.Source == entryFinancialSourceReportsAPI {
		// The upstream `totals` row is the canonical figure (round-of-sum).
		// Re-derive the grouped summary's money from it so the two agree.
		groupTotals.Financials = totals.Financials
	}
	data["group_totals_summary"] = groupTotals
	data["totals_summary"] = totals
	data["suggestedActions"] = reportDrilldownSuggestions(body)
	// The normalized views above fully replace the raw upstream arrays; drop
	// them so the response stays within the MCP transport budget.
	delete(data, "groupOne")
	delete(data, "groupTotals")
	delete(data, "donutChart")
	delete(data, "totals")
	return len(rollups)
}

func appendWeeklyReportViews(data map[string]any, body map[string]any, includeFuture bool) int {
	if data == nil {
		return 0
	}
	now := reportNowForBody(body)
	rollups := summaryRollupsFiltered(data, nil, includeFuture, now)
	dayTotals := weeklyDayTotals(data, includeFuture, now)
	data["weekly_rollups"] = rollups
	data["weekly_day_totals"] = dayTotals
	data["totals_summary"] = summarizeReportTotals(data, nil)
	data["suggestedActions"] = reportDrilldownSuggestions(body)
	delete(data, "groupOne")
	delete(data, "totalsByDay")
	delete(data, "totals")
	return len(rollups)
}

func summaryGroupsFromBody(body map[string]any) []string {
	filter, _ := body["summaryFilter"].(map[string]any)
	raw, _ := filter["groups"].([]string)
	if len(raw) > 0 {
		return raw
	}
	if anyGroups, ok := filter["groups"].([]any); ok {
		out := make([]string, 0, len(anyGroups))
		for _, group := range anyGroups {
			if s := strings.ToUpper(reportValueString(group)); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func summaryRollups(data map[string]any, groups []string) []ReportRollupView {
	return summaryRollupsFiltered(data, groups, true, time.Time{})
}

func summaryRollupsFiltered(data map[string]any, groups []string, includeFuture bool, now time.Time) []ReportRollupView {
	rows := firstReportRows(data, "groupOne", "group_one", "groups", "rows", "items")
	out := make([]ReportRollupView, 0, len(rows))
	for _, row := range rows {
		out = append(out, reportRollupFromRowFiltered(row, groups, 0, includeFuture, now))
	}
	return out
}

func firstReportRows(data map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		if rows := mapSlice(data[key]); len(rows) > 0 {
			return rows
		}
	}
	return nil
}

func reportRollupFromRowFiltered(row map[string]any, groups []string, level int, includeFuture bool, now time.Time) ReportRollupView {
	group := ""
	if level < len(groups) {
		group = groups[level]
	}
	group = firstNonEmptyString(group, strings.ToUpper(firstReportString(row, "group", "type", "groupType")))
	money := reportMoneyFromRow(row)
	fin := EntryFinancials{Source: entryFinancialSourceUnavailable, Reason: "no report money fields were present for this rollup"}
	if money.hasMoney() {
		fin = money.financials()
	}
	seconds, _ := reportDurationSeconds(row)
	childrenRows := firstReportRows(row, "children", "items", "groupTwo", "group_two", "groupThree", "group_three")
	children := make([]ReportRollupView, 0, len(childrenRows))
	for _, child := range childrenRows {
		children = append(children, reportRollupFromRowFiltered(child, groups, level+1, includeFuture, now))
	}
	entries, _ := reportInt(firstPresent(row, "entriesCount", "entries_count", "count"))
	return ReportRollupView{
		Group:              group,
		Level:              level + 1,
		ID:                 rollupID(row, group),
		Name:               rollupName(row, group),
		Duration:           entryDurationView(seconds, "reports_api"),
		Entries:            entries,
		Financials:         fin,
		MoneyByCurrency:    reportMoneyByCurrency(row),
		WeeklyDayBreakdown: weeklyDayRows(row, includeFuture, now),
		Children:           children,
	}
}

func rollupID(row map[string]any, group string) string {
	for _, key := range []string{"id", "_id", "key", "groupId", "group_id", strings.ToLower(group) + "Id"} {
		if id := cleanReportID(row[key]); id != "" {
			return id
		}
	}
	if nested, ok := row[strings.ToLower(group)].(map[string]any); ok {
		for _, key := range []string{"id", "_id"} {
			if id := cleanReportID(nested[key]); id != "" {
				return id
			}
		}
	}
	return ""
}

func rollupName(row map[string]any, group string) string {
	for _, key := range []string{"name", "title", "label", "groupName", "group_name", strings.ToLower(group) + "Name"} {
		if name := reportValueString(row[key]); name != "" {
			return name
		}
	}
	if nested, ok := row[strings.ToLower(group)].(map[string]any); ok {
		for _, key := range []string{"name", "title"} {
			if name := reportValueString(nested[key]); name != "" {
				return name
			}
		}
	}
	if strings.EqualFold(group, "PROJECT") {
		return "(no project)"
	}
	return ""
}

func summarizeReportRollups(rollups []ReportRollupView) ReportGroupTotalsSummary {
	summary := ReportGroupTotalsSummary{Count: len(rollups), ByGroupType: map[string]int{}}
	var seconds int64
	var money reportEntryMoney
	var walkCounts func([]ReportRollupView)
	walkCounts = func(items []ReportRollupView) {
		for _, item := range items {
			if item.Group != "" {
				summary.ByGroupType[item.Group]++
			}
			walkCounts(item.Children)
		}
	}
	walkCounts(rollups)
	for _, item := range rollups {
		seconds += item.Duration.Seconds
		money = mergeEntryFinancials(money, item.Financials)
	}
	summary.Duration = entryDurationView(seconds, "reports_api")
	if money.hasMoney() {
		summary.Financials = money.financials()
	} else {
		summary.Financials = EntryFinancials{Source: entryFinancialSourceUnavailable, Reason: "no report money fields were present for grouped totals"}
	}
	if len(summary.ByGroupType) == 0 {
		summary.ByGroupType = nil
	}
	return summary
}

func mergeEntryFinancials(existing reportEntryMoney, fin EntryFinancials) reportEntryMoney {
	existing.earned = moneySum(existing.earned, fin.Earned)
	existing.cost = moneySum(existing.cost, fin.Cost)
	existing.profit = moneySum(existing.profit, fin.Profit)
	return existing
}

func weeklyDayTotals(data map[string]any, includeFuture bool, now time.Time) []WeeklyDayTotalView {
	return weeklyDayRows(data, includeFuture, now)
}

func weeklyDayRows(data map[string]any, includeFuture bool, now time.Time) []WeeklyDayTotalView {
	rows := firstReportRows(data, "totalsByDay", "totals_by_day", "dailyTotals", "daily_totals", "days")
	out := make([]WeeklyDayTotalView, 0, len(rows))
	for _, row := range rows {
		date := firstReportString(row, "date", "day", "start")
		isFuture := reportDayIsFuture(date, now)
		if isFuture && !includeFuture {
			continue
		}
		seconds, _ := reportDurationSeconds(row)
		money := reportMoneyFromRow(row)
		fin := EntryFinancials{Source: entryFinancialSourceUnavailable, Reason: "no report money fields were present for this day"}
		if money.hasMoney() {
			fin = money.financials()
		}
		out = append(out, WeeklyDayTotalView{
			Date:            date,
			IsFuture:        isFuture,
			Duration:        entryDurationView(seconds, "reports_api"),
			Financials:      fin,
			MoneyByCurrency: reportMoneyByCurrency(row),
		})
	}
	return out
}

func reportNowForBody(body map[string]any) time.Time {
	loc := time.UTC
	if tz := reportValueString(body["timeZone"]); tz != "" {
		if loaded, err := time.LoadLocation(tz); err == nil {
			loc = loaded
		}
	}
	return time.Now().In(loc)
}

func reportDayIsFuture(raw string, now time.Time) bool {
	if raw == "" {
		return false
	}
	if now.IsZero() {
		return false
	}
	date := raw
	if len(date) >= len("2006-01-02") {
		date = date[:len("2006-01-02")]
	}
	day, err := time.ParseInLocation("2006-01-02", date, now.Location())
	if err != nil {
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return day.After(today)
}

func reportDrilldownSuggestions(body map[string]any) []ToolSuggestion {
	suggestionArgs := map[string]any{}
	for _, key := range []string{"dateRangeStart", "dateRangeEnd", "dateRangeType", "timeZone", "approvalState", "invoicingState", "amountShown", "amounts", "clients", "projects", "tasks", "users", "userGroups", "currency"} {
		if value, ok := body[key]; ok {
			suggestionArgs[key] = value
		}
	}
	if _, ok := suggestionArgs["dateRangeStart"]; !ok {
		return nil
	}
	return []ToolSuggestion{{
		Tool:   "clockify_reports_detailed",
		Reason: "Open row-level entries behind these report totals, including approval, lock, invoicing, custom field, and money details.",
		Arguments: map[string]any{
			"date_range_start": suggestionArgs["dateRangeStart"],
			"date_range_end":   suggestionArgs["dateRangeEnd"],
			"detailed_filter": map[string]any{
				"page":      1,
				"page_size": 100,
				"options":   map[string]any{"totals": "CALCULATE"},
			},
		},
	}}
}

func validateWeeklyDateRange(body map[string]any, defaultTimezone *time.Location) error {
	startRaw := reportValueString(body["dateRangeStart"])
	endRaw := reportValueString(body["dateRangeEnd"])
	if startRaw == "" || endRaw == "" {
		return nil
	}
	loc, err := loadLocation(reportValueString(body["timeZone"]), defaultTimezone)
	if err != nil {
		return err
	}
	start, err := parseWeeklyRangeTime(startRaw, loc)
	if err != nil {
		return fmt.Errorf("weekly report dateRangeStart: %w", err)
	}
	end, err := parseWeeklyRangeTime(endRaw, loc)
	if err != nil {
		return fmt.Errorf("weekly report dateRangeEnd: %w", err)
	}
	d := end.Sub(start)
	if d < 7*24*time.Hour-time.Second || d > 7*24*time.Hour+time.Second {
		return fmt.Errorf("weekly report needs a range of exactly 7 days; got %s..%s (%.0f days) — omit `end` and pass only `start` (or `week_start`) so the server derives the week", startRaw, endRaw, d.Hours()/24)
	}
	return nil
}

func parseWeeklyRangeTime(raw string, loc *time.Location) (time.Time, error) {
	if t, err := parseFlexibleDateTime(raw, loc); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05.000", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD date, got %q", raw)
}

func weekStartOffset(day string) int {
	switch strings.ToUpper(strings.TrimSpace(day)) {
	case "TUESDAY":
		return 2
	case "WEDNESDAY":
		return 3
	case "THURSDAY":
		return 4
	case "FRIDAY":
		return 5
	case "SATURDAY":
		return 6
	case "SUNDAY":
		return 7
	default:
		return 1
	}
}

func weekdayOneBased(t time.Time) int {
	weekday := int(t.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}
