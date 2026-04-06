package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"goclmcp/internal/clockify"
	"goclmcp/internal/resolve"
)

func (s *Service) SummaryReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	start, end, err := parseRange(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, entryRangeQuery(start, end))
	if err != nil {
		return ResultEnvelope{}, err
	}
	data := SummaryData{
		Range:     DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:    buildTotals(entries),
		ByProject: summarizeByProject(entries),
	}
	if boolArg(args, "include_entries") {
		data.Entries = entries
	}
	return ok("clockify_summary_report", data, map[string]any{"workspaceId": wsID, "userId": userID, "source": "time-entries-wrapper"}), nil
}

func (s *Service) WeeklySummary(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	loc, err := loadLocation(stringArg(args, "timezone"))
	if err != nil {
		return ResultEnvelope{}, err
	}
	start, end, err := weekBounds(stringArg(args, "week_start"), loc)
	if err != nil {
		return ResultEnvelope{}, err
	}
	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, entryRangeQuery(start, end))
	if err != nil {
		return ResultEnvelope{}, err
	}
	data := WeeklySummaryData{
		Range:         DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:        buildTotals(entries),
		ByDay:         summarizeByDay(entries, loc),
		ByProject:     summarizeByProject(entries),
		UnassignedKey: "(no project)",
	}
	if boolArg(args, "include_entries") {
		data.Entries = entries
	}
	return ok("clockify_weekly_summary", data, map[string]any{"workspaceId": wsID, "userId": userID, "timezone": loc.String(), "source": "time-entries-wrapper"}), nil
}

func (s *Service) QuickReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	days := intArg(args, "days", 7)
	if days < 1 || days > 31 {
		return ResultEnvelope{}, fmt.Errorf("days must be between 1 and 31")
	}
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)
	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, entryRangeQuery(start, end))
	if err != nil {
		return ResultEnvelope{}, err
	}
	projects := summarizeByProject(entries)
	data := QuickReportData{
		Range:               DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:              buildTotals(entries),
		RunningEntries:      runningEntries(entries),
		ProjectsRepresented: len(projects),
	}
	if len(projects) > 0 {
		data.TopProject = &projects[0]
	}
	if boolArg(args, "include_entries") {
		data.EntriesSample = entries
	} else if len(entries) > 5 {
		data.EntriesSample = entries[:5]
	} else {
		data.EntriesSample = entries
	}
	return ok("clockify_quick_report", data, map[string]any{"workspaceId": wsID, "userId": userID, "days": days, "source": "time-entries-wrapper"}), nil
}

func (s *Service) DetailedReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	start, end, err := parseRange(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, entryRangeQuery(start, end))
	if err != nil {
		return ResultEnvelope{}, err
	}

	projectRef := stringArg(args, "project")
	filtered := entries
	if projectRef != "" {
		projectID, err := resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
		filtered = make([]clockify.TimeEntry, 0, len(entries))
		for _, e := range entries {
			if e.ProjectID == projectID {
				filtered = append(filtered, e)
			}
		}
	}

	data := map[string]any{
		"range":  DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		"totals": buildTotals(filtered),
	}

	// include_entries defaults to true
	includeEntries := true
	if v, exists := args["include_entries"]; exists {
		if b, isBool := v.(bool); isBool {
			includeEntries = b
		}
	}
	if includeEntries {
		data["entries"] = filtered
	}

	return ok("clockify_detailed_report", data, map[string]any{"workspaceId": wsID, "userId": userID, "source": "time-entries-wrapper"}), nil
}

func weekBounds(weekStart string, loc *time.Location) (time.Time, time.Time, error) {
	var base time.Time
	var err error
	if strings.TrimSpace(weekStart) == "" {
		base = time.Now().In(loc)
	} else {
		base, err = parseFlexibleDateTime(weekStart, loc)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	weekday := int(base.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startLocal := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(weekday - 1))
	endLocal := startLocal.AddDate(0, 0, 7)
	return startLocal.UTC(), endLocal.UTC(), nil
}

func summarizeByProject(entries []clockify.TimeEntry) []ProjectSummary {
	m := map[string]*ProjectSummary{}
	for _, entry := range entries {
		key := entry.ProjectID
		name := strings.TrimSpace(entry.ProjectName)
		if name == "" {
			name = "(no project)"
		}
		if key == "" {
			key = "(no project)"
		}
		item, ok := m[key]
		if !ok {
			item = &ProjectSummary{ProjectID: entry.ProjectID, ProjectName: name}
			m[key] = item
		}
		item.Entries++
		item.TotalSeconds += entry.DurationSeconds()
	}
	out := make([]ProjectSummary, 0, len(m))
	for _, item := range m {
		item.TotalHours = hours(item.TotalSeconds)
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalSeconds == out[j].TotalSeconds {
			return out[i].ProjectName < out[j].ProjectName
		}
		return out[i].TotalSeconds > out[j].TotalSeconds
	})
	return out
}

func summarizeByDay(entries []clockify.TimeEntry, loc *time.Location) []DaySummary {
	m := map[string]*DaySummary{}
	for _, entry := range entries {
		start, err := entry.StartTime()
		if err != nil {
			continue
		}
		key := start.In(loc).Format("2006-01-02")
		item, ok := m[key]
		if !ok {
			item = &DaySummary{Date: key}
			m[key] = item
		}
		item.Entries++
		item.TotalSeconds += entry.DurationSeconds()
	}
	out := make([]DaySummary, 0, len(m))
	for _, item := range m {
		item.TotalHours = hours(item.TotalSeconds)
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func buildTotals(entries []clockify.TimeEntry) SummaryTotals {
	var total int64
	var running int
	for _, entry := range entries {
		if entry.IsRunning() {
			running++
		}
		total += entry.DurationSeconds()
	}
	return SummaryTotals{Entries: len(entries), RunningEntries: running, TotalSeconds: total, TotalHours: hours(total)}
}

func runningEntries(entries []clockify.TimeEntry) []clockify.TimeEntry {
	out := make([]clockify.TimeEntry, 0)
	for _, entry := range entries {
		if entry.IsRunning() {
			out = append(out, entry)
		}
	}
	return out
}
