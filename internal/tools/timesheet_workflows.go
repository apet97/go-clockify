package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
)

type TimesheetReviewData struct {
	Range            DateRange            `json:"range"`
	Totals           SummaryTotals        `json:"totals"`
	ByDay            []DaySummary         `json:"byDay"`
	ByProject        []ProjectSummary     `json:"byProject"`
	Issues           []TimesheetIssue     `json:"issues"`
	SuggestedActions []ToolSuggestion     `json:"suggestedActions"`
	Entries          []clockify.TimeEntry `json:"entries,omitempty"`
}

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

type TimeEntryDraft struct {
	Start       string `json:"start"`
	End         string `json:"end"`
	ProjectID   string `json:"projectId,omitempty"`
	Project     string `json:"project,omitempty"`
	Description string `json:"description"`
	Billable    *bool  `json:"billable,omitempty"`
}

type TimeEntryRef struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	ProjectName string `json:"projectName,omitempty"`
	Start       string `json:"start,omitempty"`
	End         string `json:"end,omitempty"`
}

type TimesheetFillGapData struct {
	DryRun    bool               `json:"dryRun"`
	Proposed  TimeEntryDraft     `json:"proposed"`
	Entry     clockify.TimeEntry `json:"entry,omitempty"`
	Overlaps  []TimeEntryRef     `json:"overlaps,omitempty"`
	Validated bool               `json:"validated"`
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

	limits := s.reportLimitsForArgs(args)
	effectiveMax := limits.AppliedMaxEntries
	agg, wsID, userID, err := s.aggregateEntriesRange(ctx, start, end, loc, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: true,
		MaxEntries:     effectiveMax,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	entries := sortedEntriesByStart(agg.Entries)
	issues, suggestions := buildTimesheetReview(entries, start, end, loc, workdayStart, workdayEnd, minGapMinutes, maxSuggestions)
	data := TimesheetReviewData{
		Range:            DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:           totalsFromAgg(agg),
		ByDay:            daySummariesFromAgg(agg),
		ByProject:        projectSummariesFromAgg(agg),
		Issues:           issues,
		SuggestedActions: suggestions,
	}
	if boolArg(args, "include_entries") {
		data.Entries = entries
	}
	meta := mergeMeta(map[string]any{
		"workspaceId":  wsID,
		"userId":       userID,
		"timezone":     loc.String(),
		"mode":         mode,
		"source":       "time-entries-workflow-review",
		"workdayStart": workdayStart,
		"workdayEnd":   workdayEnd,
	}, paginationMeta(agg, reportPageSize, limits))
	return ok("clockify_timesheet_review", data, meta), nil
}

func (s *Service) TimesheetFillGap(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	start, end, err := parseRange(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	description := strings.TrimSpace(stringArg(args, "description"))
	if description == "" {
		return ResultEnvelope{}, fmt.Errorf("description is required")
	}
	projectID := stringArg(args, "project_id")
	projectRef := stringArg(args, "project")
	if projectID == "" && strings.TrimSpace(projectRef) == "" {
		return ResultEnvelope{}, fmt.Errorf("project or project_id is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if projectID == "" {
		projectID, err = s.resolveProjectID(ctx, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
	}

	overlaps, userID, err := s.findEntryOverlaps(ctx, start, end)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if len(overlaps) > 0 && !boolArg(args, "allow_overlap") {
		return ResultEnvelope{}, fmt.Errorf("requested gap overlaps %d existing entr%s; pass allow_overlap=true only after manual review", len(overlaps), pluralY(len(overlaps)))
	}

	var billablePtr *bool
	if billable, ok := args["billable"].(bool); ok {
		billablePtr = &billable
	}
	draft := TimeEntryDraft{
		Start:       start.Format(time.RFC3339),
		End:         end.Format(time.RFC3339),
		ProjectID:   projectID,
		Project:     projectRef,
		Description: description,
		Billable:    billablePtr,
	}
	if dryrun.Enabled(args) {
		return ok("clockify_timesheet_fill_gap", TimesheetFillGapData{
			DryRun:    true,
			Proposed:  draft,
			Overlaps:  overlaps,
			Validated: true,
		}, map[string]any{"workspaceId": wsID, "userId": userID}), nil
	}

	payload := map[string]any{
		"start":       draft.Start,
		"end":         draft.End,
		"projectId":   projectID,
		"description": description,
	}
	if billablePtr != nil {
		payload["billable"] = *billablePtr
	}
	path, err := paths.Workspace(wsID, "time-entries")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var entry clockify.TimeEntry
	if err := s.Client.Post(ctx, path, payload, &entry); err != nil {
		return ResultEnvelope{}, err
	}
	s.emitEntryAndWeeklyWithState(ctx, wsID, entry)
	return ok("clockify_timesheet_fill_gap", TimesheetFillGapData{
		Proposed:  draft,
		Entry:     entry,
		Overlaps:  overlaps,
		Validated: true,
	}, map[string]any{"workspaceId": wsID, "userId": userID}), nil
}

func (s *Service) rejectEntryOverlap(ctx context.Context, start, end time.Time, args map[string]any) error {
	if boolArg(args, "allow_overlap") {
		return nil
	}
	overlaps, _, err := s.findEntryOverlaps(ctx, start, end)
	if err != nil {
		return err
	}
	if len(overlaps) == 0 {
		return nil
	}
	return fmt.Errorf("requested entry overlaps %d existing entr%s; pass allow_overlap=true only after manual review", len(overlaps), pluralY(len(overlaps)))
}

func timesheetReviewRange(args map[string]any, loc *time.Location) (time.Time, time.Time, string, error) {
	startRaw := strings.TrimSpace(stringArg(args, "start"))
	endRaw := strings.TrimSpace(stringArg(args, "end"))
	if startRaw != "" || endRaw != "" {
		if startRaw == "" || endRaw == "" {
			return time.Time{}, time.Time{}, "", fmt.Errorf("start and end must be provided together")
		}
		start, err := parseFlexibleDateTime(startRaw, loc)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("start: %w", err)
		}
		end, err := parseFlexibleDateTime(endRaw, loc)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("end: %w", err)
		}
		if !end.After(start) {
			return time.Time{}, time.Time{}, "", fmt.Errorf("end must be after start")
		}
		return start.UTC(), end.UTC(), "range", nil
	}
	if weekStart := strings.TrimSpace(stringArg(args, "week_start")); weekStart != "" {
		start, end, err := weekBounds(weekStart, loc)
		return start, end, "week", err
	}
	base := time.Now().In(loc)
	if dateRaw := strings.TrimSpace(stringArg(args, "date")); dateRaw != "" {
		parsed, err := parseFlexibleDateTime(dateRaw, loc)
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
				Tool:   "clockify_stop_timer",
				Reason: "Stop the running timer if the work session is finished.",
			})
		}
		if strings.TrimSpace(entry.ProjectID) == "" {
			issues = append(issues, TimesheetIssue{
				Type:     "missing_project",
				Severity: "warning",
				Message:  "Entry has no project assigned.",
				EntryID:  entry.ID,
				Start:    startTime.Format(time.RFC3339),
				End:      endTime.Format(time.RFC3339),
			})
			addSuggestion(ToolSuggestion{
				Tool:        "clockify_find_and_update_entry",
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
				Tool:        "clockify_find_and_update_entry",
				Reason:      "Add a human-readable description to this entry.",
				Arguments:   map[string]any{"entry_id": entry.ID},
				MissingArgs: []string{"new_description"},
			})
		}
	}
	for _, issue := range overlapIssues(entries) {
		issues = append(issues, issue)
		addSuggestion(ToolSuggestion{
			Tool:        "clockify_update_entry",
			Reason:      "Adjust one overlapping entry after confirming the intended boundary.",
			Arguments:   map[string]any{"entry_id": firstNonEmpty(issue.RelatedEntryIDs)},
			MissingArgs: []string{"start", "end"},
		})
	}
	if minGapMinutes > 0 {
		for _, issue := range gapIssues(entries, start, end, loc, workdayStart, workdayEnd, time.Duration(minGapMinutes)*time.Minute) {
			issues = append(issues, issue)
			addSuggestion(ToolSuggestion{
				Tool:   "clockify_timesheet_fill_gap",
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
	agg, _, userID, err := s.aggregateEntriesRange(ctx, start, end, time.UTC, aggregateOptions{
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
