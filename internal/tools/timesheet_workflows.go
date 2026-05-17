package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

type TimesheetReviewData struct {
	Range            DateRange        `json:"range"`
	Totals           SummaryTotals    `json:"totals"`
	ByDay            []DaySummary     `json:"byDay"`
	ByProject        []ProjectSummary `json:"byProject"`
	Issues           []TimesheetIssue `json:"issues"`
	SuggestedActions []ToolSuggestion `json:"suggestedActions"`
	Entries          []EntryView      `json:"entries,omitempty"`
}

const defaultTimesheetReviewMaxRows = 15

type TimesheetIssue struct {
	Type            string   `json:"type"`
	Severity        string   `json:"severity"`
	Message         string   `json:"message"`
	EntryID         string   `json:"entryId,omitempty"`
	RelatedEntryIDs []string `json:"relatedEntryIds,omitempty"`
	Start           string   `json:"start,omitempty"`
	End             string   `json:"end,omitempty"`
}

type ToolSuggestion struct {
	Tool        string         `json:"tool"`
	Reason      string         `json:"reason"`
	Arguments   map[string]any `json:"arguments,omitempty"`
	MissingArgs []string       `json:"missingArgs,omitempty"`
}

type TimeEntryRef struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	ProjectName string `json:"projectName,omitempty"`
	Start       string `json:"start,omitempty"`
	End         string `json:"end,omitempty"`
}

func (s *Service) TimesheetReview(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	loc, err := loadLocation(stringArg(args, "timezone"), s.DefaultTimezone)
	if err != nil {
		return ResultEnvelope{}, err
	}
	start, end, mode, err := timesheetReviewRange(args, loc)
	if err != nil {
		return ResultEnvelope{}, err
	}
	workdayStart := stringArg(args, "workday_start")
	if strings.TrimSpace(workdayStart) == "" {
		workdayStart = "09:00"
	}
	workdayEnd := stringArg(args, "workday_end")
	if strings.TrimSpace(workdayEnd) == "" {
		workdayEnd = "17:00"
	}
	if _, _, err := parseHHMM(workdayStart, "workday_start"); err != nil {
		return ResultEnvelope{}, err
	}
	if _, _, err := parseHHMM(workdayEnd, "workday_end"); err != nil {
		return ResultEnvelope{}, err
	}
	minGapMinutes := intArg(args, "min_gap_minutes", 30)
	if minGapMinutes < 0 {
		return ResultEnvelope{}, fmt.Errorf("min_gap_minutes must be >= 0")
	}
	maxSuggestions := intArg(args, "max_suggestions", 10)
	if maxSuggestions < 0 {
		return ResultEnvelope{}, fmt.Errorf("max_suggestions must be >= 0")
	}
	if maxSuggestions > 50 {
		maxSuggestions = 50
	}
	maxRows := intArg(args, "max_rows", defaultTimesheetReviewMaxRows)
	if maxRows < 0 {
		return ResultEnvelope{}, fmt.Errorf("max_rows must be >= 0")
	}

	limits := s.reportLimitsForArgs(args)
	effectiveMax := limits.AppliedMaxEntries
	agg, wsID, userID, err := s.aggregateEntriesRange(ctx, start, end, loc, aggregateOptions{
		PageSize:            reportPageSize,
		IncludeEntries:      true,
		MaxEntries:          effectiveMax,
		ResolveProjectNames: true,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	entries := sortedEntriesByStart(agg.Entries)
	issues, suggestions := buildTimesheetReview(entries, start, end, loc, workdayStart, workdayEnd, minGapMinutes, maxSuggestions)
	data := TimesheetReviewData{
		Range:            DateRange{Start: start.In(loc).Format(time.RFC3339), End: end.In(loc).Format(time.RFC3339)},
		Totals:           totalsFromAgg(agg),
		ByDay:            daySummariesFromAgg(agg),
		ByProject:        projectSummariesFromAgg(agg),
		Issues:           issues,
		SuggestedActions: suggestions,
	}
	truncationMeta := capTimesheetReviewDetails(&data, maxRows)
	baseMeta := mergeMeta(map[string]any{
		"workspaceId":  wsID,
		"userId":       userID,
		"timezone":     loc.String(),
		"mode":         mode,
		"source":       "time-entries-workflow-review",
		"workdayStart": workdayStart,
		"workdayEnd":   workdayEnd,
		"maxRows":      maxRows,
	}, truncationMeta)
	if boolArg(args, "include_entries") {
		views, financialMeta := s.enrichEntryViews(ctx, wsID, entries)
		data.Entries = views
		truncationMeta = capTimesheetReviewDetails(&data, maxRows)
		baseMeta = mergeMeta(baseMeta, truncationMeta)
		meta := paginationMeta(agg, reportPageSize, limits)
		meta["financials"] = financialMeta
		return ok(oneUserToolReviewDay, data, mergeMeta(baseMeta, meta)), nil
	}
	meta := mergeMeta(baseMeta, paginationMeta(agg, reportPageSize, limits))
	return ok(oneUserToolReviewDay, data, meta), nil
}

func capTimesheetReviewDetails(data *TimesheetReviewData, maxRows int) map[string]any {
	meta := map[string]any{}
	if data == nil || maxRows == 0 {
		return meta
	}
	if total := len(data.ByProject); total > maxRows {
		data.ByProject = data.ByProject[:maxRows]
		addReviewCapMeta(meta, "byProject", total, maxRows)
	}
	if total := len(data.Issues); total > maxRows {
		data.Issues = data.Issues[:maxRows]
		addReviewCapMeta(meta, "issues", total, maxRows)
	}
	if total := len(data.Entries); total > maxRows {
		data.Entries = data.Entries[:maxRows]
		addReviewCapMeta(meta, "entries", total, maxRows)
	}
	if len(meta) > 0 {
		meta["truncated"] = true
		meta["next_hint"] = "Review details were capped by max_rows; lower the date range or raise max_rows to inspect more rows."
	}
	return meta
}

func addReviewCapMeta(meta map[string]any, prefix string, total, returned int) {
	meta[prefix+"Total"] = total
	meta[prefix+"Returned"] = returned
}

func timesheetReviewRange(args map[string]any, loc *time.Location) (time.Time, time.Time, string, error) {
	startRaw := strings.TrimSpace(stringArg(args, "start"))
	endRaw := strings.TrimSpace(stringArg(args, "end"))
	if startRaw != "" || endRaw != "" {
		if startRaw == "" || endRaw == "" {
			return time.Time{}, time.Time{}, "", fmt.Errorf("start and end must be provided together")
		}
		start, end, err := parseRangeInLocation(map[string]any{"start": startRaw, "end": endRaw}, loc)
		if err != nil {
			return time.Time{}, time.Time{}, "", err
		}
		return start, end, "range", nil
	}
	if weekStart := strings.TrimSpace(stringArg(args, "week_start")); weekStart != "" {
		start, end, err := weekBounds(weekStart, loc)
		return start, end, "week", err
	}
	base := time.Now().In(loc)
	if dateRaw := strings.TrimSpace(stringArg(args, "date")); dateRaw != "" {
		var parsed time.Time
		var err error
		if isBareDateString(dateRaw) {
			parsed, err = time.ParseInLocation("2006-01-02", dateRaw, loc)
		} else {
			parsed, err = parseFlexibleDateTime(dateRaw, loc)
		}
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("date: %w", err)
		}
		base = parsed.In(loc)
	}
	startLocal := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, loc)
	return startLocal.UTC(), startLocal.AddDate(0, 0, 1).UTC(), "day", nil
}

func buildTimesheetReview(entries []clockify.TimeEntry, start, end time.Time, loc *time.Location, workdayStart, workdayEnd string, minGapMinutes, maxSuggestions int) ([]TimesheetIssue, []ToolSuggestion) {
	issues := make([]TimesheetIssue, 0)
	suggestions := make([]ToolSuggestion, 0)
	addSuggestion := func(s ToolSuggestion) {
		if maxSuggestions == 0 || len(suggestions) >= maxSuggestions {
			return
		}
		suggestions = append(suggestions, s)
	}
	for _, entry := range entries {
		startTime, endTime, ok := timeEntryBounds(entry)
		if !ok {
			issues = append(issues, TimesheetIssue{
				Type:     "invalid_time_interval",
				Severity: "warning",
				Message:  "Entry has a time interval this server could not parse.",
				EntryID:  entry.ID,
			})
			continue
		}
		if entry.IsRunning() {
			issues = append(issues, TimesheetIssue{
				Type:     "running_timer",
				Severity: "warning",
				Message:  "A timer is still running in the reviewed range.",
				EntryID:  entry.ID,
				Start:    startTime.Format(time.RFC3339),
			})
			addSuggestion(ToolSuggestion{
				Tool:   "clockify_entries_timer_stop",
				Reason: "Stop the running timer if the work session is finished.",
			})
		}
		if !entry.IsRunning() && !endTime.After(startTime) {
			issues = append(issues, TimesheetIssue{
				Type:     "zero_duration",
				Severity: "warning",
				Message:  "Entry has zero tracked duration.",
				EntryID:  entry.ID,
				Start:    startTime.Format(time.RFC3339),
				End:      endTime.Format(time.RFC3339),
			})
			addSuggestion(ToolSuggestion{
				Tool:        "clockify_entries_update",
				Reason:      "Adjust the start or end time after confirming the intended duration.",
				Arguments:   map[string]any{"entry_id": entry.ID},
				MissingArgs: []string{"start", "end"},
			})
		}
		if strings.TrimSpace(entry.ProjectID) == "" && strings.ToUpper(strings.TrimSpace(entry.Type)) != "BREAK" {
			issues = append(issues, TimesheetIssue{
				Type:     "missing_project",
				Severity: "warning",
				Message:  "Entry has no project assigned.",
				EntryID:  entry.ID,
				Start:    startTime.Format(time.RFC3339),
				End:      endTime.Format(time.RFC3339),
			})
			addSuggestion(ToolSuggestion{
				Tool:        oneUserToolFixEntry,
				Reason:      "Assign a project to this entry once the correct project is known.",
				Arguments:   map[string]any{"entry_id": entry.ID},
				MissingArgs: []string{"project"},
			})
		}
		if strings.TrimSpace(entry.Description) == "" {
			issues = append(issues, TimesheetIssue{
				Type:     "missing_description",
				Severity: "info",
				Message:  "Entry has no description.",
				EntryID:  entry.ID,
				Start:    startTime.Format(time.RFC3339),
				End:      endTime.Format(time.RFC3339),
			})
			addSuggestion(ToolSuggestion{
				Tool:        oneUserToolFixEntry,
				Reason:      "Add a human-readable description to this entry.",
				Arguments:   map[string]any{"entry_id": entry.ID},
				MissingArgs: []string{"new_description"},
			})
		}
	}
	for _, issue := range overlapIssues(entries) {
		issues = append(issues, issue)
		addSuggestion(ToolSuggestion{
			Tool:        "clockify_entries_update",
			Reason:      "Adjust one overlapping entry after confirming the intended boundary.",
			Arguments:   map[string]any{"entry_id": firstNonEmpty(issue.RelatedEntryIDs)},
			MissingArgs: []string{"start", "end"},
		})
	}
	if minGapMinutes > 0 {
		for _, issue := range gapIssues(entries, start, end, loc, workdayStart, workdayEnd, time.Duration(minGapMinutes)*time.Minute) {
			issues = append(issues, issue)
			addSuggestion(ToolSuggestion{
				Tool:   oneUserToolEntriesCreateFromGap,
				Reason: "Fill this open workday gap after confirming what work was performed.",
				Arguments: map[string]any{
					"start":   issue.Start,
					"end":     issue.End,
					"dry_run": true,
				},
				MissingArgs: []string{"project", "description"},
			})
		}
	}
	return issues, suggestions
}

func sortedEntriesByStart(in []clockify.TimeEntry) []clockify.TimeEntry {
	out := append([]clockify.TimeEntry(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, _, aOK := timeEntryBounds(out[i])
		b, _, bOK := timeEntryBounds(out[j])
		if !aOK {
			return false
		}
		if !bOK {
			return true
		}
		return a.Before(b)
	})
	return out
}

func overlapIssues(entries []clockify.TimeEntry) []TimesheetIssue {
	sorted := sortedEntriesByStart(entries)
	out := make([]TimesheetIssue, 0)
	for i := 1; i < len(sorted); i++ {
		prevStart, prevEnd, prevOK := timeEntryBounds(sorted[i-1])
		curStart, curEnd, curOK := timeEntryBounds(sorted[i])
		if !prevOK || !curOK {
			continue
		}
		if curStart.Before(prevEnd) {
			out = append(out, TimesheetIssue{
				Type:            "overlap",
				Severity:        "warning",
				Message:         "Two entries overlap in time.",
				RelatedEntryIDs: []string{sorted[i-1].ID, sorted[i].ID},
				Start:           maxTime(prevStart, curStart).Format(time.RFC3339),
				End:             minTime(prevEnd, curEnd).Format(time.RFC3339),
			})
		}
	}
	return out
}

func gapIssues(entries []clockify.TimeEntry, rangeStart, rangeEnd time.Time, loc *time.Location, workdayStart, workdayEnd string, minGap time.Duration) []TimesheetIssue {
	startHour, startMinute, _ := parseHHMM(workdayStart, "workday_start")
	endHour, endMinute, _ := parseHHMM(workdayEnd, "workday_end")
	out := make([]TimesheetIssue, 0)
	rangeStartLocal := rangeStart.In(loc)
	rangeEndLocal := rangeEnd.In(loc)
	for day := time.Date(rangeStartLocal.Year(), rangeStartLocal.Month(), rangeStartLocal.Day(), 0, 0, 0, 0, loc); day.Before(rangeEndLocal); day = day.AddDate(0, 0, 1) {
		windowStart := time.Date(day.Year(), day.Month(), day.Day(), startHour, startMinute, 0, 0, loc).UTC()
		windowEnd := time.Date(day.Year(), day.Month(), day.Day(), endHour, endMinute, 0, 0, loc).UTC()
		if !windowEnd.After(windowStart) {
			continue
		}
		windowStart = maxTime(windowStart, rangeStart)
		windowEnd = minTime(windowEnd, rangeEnd)
		if !windowEnd.After(windowStart) {
			continue
		}
		cursor := windowStart
		for _, entry := range entries {
			entryStart, entryEnd, ok := timeEntryBounds(entry)
			if !ok || !entryEnd.After(windowStart) || !entryStart.Before(windowEnd) {
				continue
			}
			entryStart = maxTime(entryStart, windowStart)
			entryEnd = minTime(entryEnd, windowEnd)
			if entryStart.After(cursor) && entryStart.Sub(cursor) >= minGap {
				out = append(out, gapIssue(cursor, entryStart))
			}
			if entryEnd.After(cursor) {
				cursor = entryEnd
			}
		}
		if windowEnd.After(cursor) && windowEnd.Sub(cursor) >= minGap {
			out = append(out, gapIssue(cursor, windowEnd))
		}
	}
	return out
}

func gapIssue(start, end time.Time) TimesheetIssue {
	return TimesheetIssue{
		Type:     "gap",
		Severity: "info",
		Message:  "Reviewed workday has an untracked gap at least as large as min_gap_minutes.",
		Start:    start.Format(time.RFC3339),
		End:      end.Format(time.RFC3339),
	}
}

func (s *Service) findEntryOverlaps(ctx context.Context, start, end time.Time) ([]TimeEntryRef, string, error) {
	lookbackStart := start.Add(-24 * time.Hour)
	agg, _, userID, err := s.aggregateEntriesRange(ctx, lookbackStart, end, time.UTC, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: true,
	})
	if err != nil {
		return nil, "", err
	}
	out := make([]TimeEntryRef, 0)
	for _, entry := range agg.Entries {
		entryStart, entryEnd, ok := timeEntryBounds(entry)
		if !ok {
			continue
		}
		if start.Before(entryEnd) && end.After(entryStart) {
			out = append(out, timeEntryRef(entry, entryStart, entryEnd))
		}
	}
	return out, userID, nil
}

func timeEntryBounds(entry clockify.TimeEntry) (time.Time, time.Time, bool) {
	start, err := entry.StartTime()
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	end, err := entry.EndTime()
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if end.Before(start) {
		return start, start, true
	}
	return start.UTC(), end.UTC(), true
}

func timeEntryRef(entry clockify.TimeEntry, start, end time.Time) TimeEntryRef {
	return TimeEntryRef{
		ID:          entry.ID,
		Description: entry.Description,
		ProjectID:   entry.ProjectID,
		ProjectName: entry.ProjectName,
		Start:       start.Format(time.RFC3339),
		End:         end.Format(time.RFC3339),
	}
}

func parseHHMM(raw, label string) (int, int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(raw))
	if err != nil {
		return 0, 0, fmt.Errorf("%s must be HH:MM in 24-hour time", label)
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
