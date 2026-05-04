package tools

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
)

// aggregateOptions controls the streaming aggregator used by report tools.
type aggregateOptions struct {
	PageSize              int  // clamped to [50, 200]; default 200
	IncludeEntries        bool // retain raw entries on the result (memory grows with range)
	MaxEntries            int  // hard cap when IncludeEntries is true; 0 = unlimited
	SampleEntries         int  // retain at most this many sample entries independent of IncludeEntries
	CollectRunningEntries bool // retain running entries independent of IncludeEntries
	ProjectID             string
}

// projectBucket accumulates seconds and entry count for a single project
// seen during aggregation. Project name is retained so we can build a
// ProjectSummary without keeping raw entries around.
type projectBucket struct {
	ID           string
	Name         string
	Entries      int
	TotalSeconds int64
}

// dayBucket accumulates seconds and entry count for a single day bucket.
type dayBucket struct {
	Entries      int
	TotalSeconds int64
}

// aggregateResult carries the streaming totals produced by aggregateEntriesRange.
type aggregateResult struct {
	Entries           []clockify.TimeEntry // populated only if IncludeEntries
	EntriesSample     []clockify.TimeEntry // bounded by SampleEntries
	RunningList       []clockify.TimeEntry // populated only if CollectRunningEntries
	TotalSeconds      int64
	RunningEntries    int
	ByProject         map[string]*projectBucket // project id (or "" for unassigned) -> bucket
	ByDay             map[string]*dayBucket     // YYYY-MM-DD in requested tz -> bucket
	EntriesCount      int                       // total entries walked across all pages
	PagesFetched      int
	ProjectFilter     string
	PageSize          int
	RequestedPageSize int
	PageSizeClamped   bool
}

// aggregatePageSafetyStop mirrors clockify.ListAll's 1000-page safeguard.
const aggregatePageSafetyStop = 1000

const quickReportMaxDays = 365

// aggregateEntriesRange streams time entries for the current user across the
// given date range, paginating until the API runs out of data. Totals and
// bucketed maps are updated incrementally. Raw entries are retained only when
// IncludeEntries is true, which keeps memory bounded for large ranges.
//
// When IncludeEntries is true AND MaxEntries > 0 AND the number of walked
// entries crosses the cap, the function fails closed with actionable guidance
// so callers cannot silently miss data while also expecting a full entry list.
// When IncludeEntries is false the cap is not enforced — memory is bounded by
// design.
func (s *Service) aggregateEntriesRange(ctx context.Context, start, end time.Time, loc *time.Location, opts aggregateOptions) (*aggregateResult, string, string, error) {
	return s.aggregateEntriesRangeForWorkspace(ctx, "", start, end, loc, opts)
}

func (s *Service) aggregateEntriesRangeForWorkspace(ctx context.Context, workspaceID string, start, end time.Time, loc *time.Location, opts aggregateOptions) (*aggregateResult, string, string, error) {
	requestedPageSize := opts.PageSize
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 200
	}
	if pageSize < 50 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if loc == nil {
		loc = time.UTC
	}

	wsID := workspaceID
	if wsID == "" {
		var err error
		wsID, err = s.ResolveWorkspaceID(ctx)
		if err != nil {
			return nil, "", "", err
		}
	}
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return nil, "", "", err
	}

	baseQuery := entryRangeQuery(start, end)

	result := &aggregateResult{
		ByProject:         make(map[string]*projectBucket),
		ByDay:             make(map[string]*dayBucket),
		ProjectFilter:     opts.ProjectID,
		PageSize:          pageSize,
		RequestedPageSize: requestedPageSize,
		PageSizeClamped:   requestedPageSize > 0 && requestedPageSize != pageSize,
	}

	path, err := paths.Workspace(wsID, "user", user.ID, "time-entries")
	if err != nil {
		return nil, "", "", err
	}
	query := make(map[string]string, len(baseQuery)+3)
	maps.Copy(query, baseQuery)
	query["page-size"] = strconv.Itoa(pageSize)
	if opts.ProjectID != "" {
		query["project"] = opts.ProjectID
	}
	for page := 1; page <= aggregatePageSafetyStop; page++ {
		query["page"] = strconv.Itoa(page)

		var batch []clockify.TimeEntry
		if err := s.Client.Get(ctx, path, query, &batch); err != nil {
			return nil, "", "", err
		}
		result.PagesFetched++
		// Progress notification — total is unknown until the final short page
		// is seen, so report it as -1 mid-walk and let the caller finalise.
		s.EmitProgress(ctx, float64(result.PagesFetched), -1, fmt.Sprintf("fetched %d entries", result.EntriesCount))

		for _, entry := range batch {
			result.EntriesCount++
			secs := entry.DurationSeconds()
			result.TotalSeconds += secs
			if entry.IsRunning() {
				result.RunningEntries++
				if opts.CollectRunningEntries {
					result.RunningList = append(result.RunningList, entry)
				}
			}

			projectKey := entry.ProjectID
			bucket, ok := result.ByProject[projectKey]
			if !ok {
				name := strings.TrimSpace(entry.ProjectName)
				if name == "" {
					name = "(no project)"
				}
				bucket = &projectBucket{ID: projectKey, Name: name}
				result.ByProject[projectKey] = bucket
			} else if bucket.Name == "(no project)" && strings.TrimSpace(entry.ProjectName) != "" {
				bucket.Name = strings.TrimSpace(entry.ProjectName)
			}
			bucket.Entries++
			bucket.TotalSeconds += secs

			if startTime, err := entry.StartTime(); err == nil {
				dayKey := startTime.In(loc).Format("2006-01-02")
				day, ok := result.ByDay[dayKey]
				if !ok {
					day = &dayBucket{}
					result.ByDay[dayKey] = day
				}
				day.Entries++
				day.TotalSeconds += secs
			}

			if opts.IncludeEntries {
				result.Entries = append(result.Entries, entry)
			}
			if opts.SampleEntries > 0 && len(result.EntriesSample) < opts.SampleEntries {
				result.EntriesSample = append(result.EntriesSample, entry)
			}
		}

		if opts.IncludeEntries && opts.MaxEntries > 0 && result.EntriesCount > opts.MaxEntries {
			return nil, "", "", fmt.Errorf("entry cap of %d exceeded for range %s..%s; re-run with include_entries=false or narrow the date range", opts.MaxEntries, start.Format(time.RFC3339), end.Format(time.RFC3339))
		}

		if len(batch) < pageSize {
			return result, wsID, user.ID, nil
		}
	}
	return nil, "", "", fmt.Errorf("report pagination safety stop reached at %d pages for range %s..%s; narrow the range", aggregatePageSafetyStop, start.Format(time.RFC3339), end.Format(time.RFC3339))
}

type reportLimitState struct {
	AppliedMaxEntries   int
	ServerMaxEntries    int
	RequestedMaxEntries int
	MaxEntriesRequested bool
	MaxEntriesClamped   bool
}

func (s *Service) reportLimitsForArgs(args map[string]any) reportLimitState {
	serverCap := s.ReportMaxEntries
	state := reportLimitState{
		AppliedMaxEntries: serverCap,
		ServerMaxEntries:  serverCap,
	}
	_, ok := args["max_entries"]
	if !ok {
		return state
	}
	state.MaxEntriesRequested = true
	n := intArg(args, "max_entries", -1)
	state.RequestedMaxEntries = n
	if n < 0 {
		return state
	}
	if n == 0 {
		// Explicit 0 means "no extra cap" from the caller; still bounded by
		// the server-wide cap.
		return state
	}
	if serverCap > 0 && n > serverCap {
		state.AppliedMaxEntries = serverCap
		state.MaxEntriesClamped = true
		return state
	}
	state.AppliedMaxEntries = n
	return state
}

// paginationMeta builds the structured pagination/limits meta block for a
// report response.
func paginationMeta(agg *aggregateResult, pageSize int, limits reportLimitState) map[string]any {
	appliedPageSize := pageSize
	if agg.PageSize > 0 {
		appliedPageSize = agg.PageSize
	}
	pagination := map[string]any{
		"page_size":         appliedPageSize,
		"applied_page_size": appliedPageSize,
		"pages_fetched":     agg.PagesFetched,
		"entries_total":     agg.EntriesCount,
	}
	if agg.PageSizeClamped {
		pagination["requested_page_size"] = agg.RequestedPageSize
		pagination["clamped"] = true
	}
	limitMeta := map[string]any{
		"max_entries":         limits.AppliedMaxEntries,
		"applied_max_entries": limits.AppliedMaxEntries,
		"server_max_entries":  limits.ServerMaxEntries,
	}
	if limits.MaxEntriesRequested {
		limitMeta["requested_max_entries"] = limits.RequestedMaxEntries
		if limits.MaxEntriesClamped {
			limitMeta["clamped"] = true
		}
	}
	meta := map[string]any{
		"pagination": pagination,
		"limits":     limitMeta,
	}
	if agg.ProjectFilter != "" {
		pagination["project_filter_resolved_id"] = agg.ProjectFilter
	}
	return meta
}

// mergeMeta merges the structured pagination meta into the caller's base meta.
func mergeMeta(base, extra map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	maps.Copy(base, extra)
	return base
}

// reportPageSize is the effective page size used when paginating report
// queries. Chosen as the upper bound of the Clockify UI pagination limit so
// we minimise HTTP round-trips for wide ranges while still keeping response
// sizes bounded.
const reportPageSize = 200

func (s *Service) SummaryReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	start, end, err := parseRange(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	include := boolArg(args, "include_entries")
	limits := s.reportLimitsForArgs(args)
	effectiveMax := limits.AppliedMaxEntries
	wsID, projectID, err := s.reportProjectFilterID(ctx, args, "")
	if err != nil {
		return ResultEnvelope{}, err
	}
	agg, wsID, userID, err := s.aggregateEntriesRangeForWorkspace(ctx, wsID, start, end, time.UTC, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: include,
		MaxEntries:     effectiveMax,
		ProjectID:      projectID,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	totals := totalsFromAgg(agg)
	projects := projectSummariesFromAgg(agg)
	data := SummaryData{
		Range:            DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:           totals,
		ByProject:        projects,
		SuggestedActions: reportSuggestedActions(projects, totals, start, end),
	}
	if include {
		data.Entries = agg.Entries
	}
	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, limits))
	return ok("clockify_summary_report", data, meta), nil
}

func (s *Service) WeeklySummary(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.weeklySummary(ctx, args, "")
}

func (s *Service) weeklySummary(ctx context.Context, args map[string]any, workspaceID string) (ResultEnvelope, error) {
	loc, err := loadLocation(stringArg(args, "timezone"), s.DefaultTimezone)
	if err != nil {
		return ResultEnvelope{}, err
	}
	start, end, err := weekBounds(stringArg(args, "week_start"), loc)
	if err != nil {
		return ResultEnvelope{}, err
	}
	include := boolArg(args, "include_entries")
	limits := s.reportLimitsForArgs(args)
	effectiveMax := limits.AppliedMaxEntries
	wsID, projectID, err := s.reportProjectFilterID(ctx, args, workspaceID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	agg, wsID, userID, err := s.aggregateEntriesRangeForWorkspace(ctx, wsID, start, end, loc, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: include,
		MaxEntries:     effectiveMax,
		ProjectID:      projectID,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	totals := totalsFromAgg(agg)
	projects := projectSummariesFromAgg(agg)
	data := WeeklySummaryData{
		Range:            DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:           totals,
		ByDay:            daySummariesFromAgg(agg),
		ByProject:        projects,
		SuggestedActions: reportSuggestedActions(projects, totals, start, end),
		UnassignedKey:    "(no project)",
	}
	if include {
		data.Entries = agg.Entries
	}
	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"timezone":    loc.String(),
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, limits))
	return ok("clockify_weekly_summary", data, meta), nil
}

func (s *Service) QuickReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	days := intArg(args, "days", 7)
	if days < 1 || days > quickReportMaxDays {
		return ResultEnvelope{}, fmt.Errorf("days must be between 1 and %d", quickReportMaxDays)
	}
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)
	includeEntries := boolArg(args, "include_entries")
	limits := s.reportLimitsForArgs(args)
	effectiveMax := limits.AppliedMaxEntries
	wsID, projectID, err := s.reportProjectFilterID(ctx, args, "")
	if err != nil {
		return ResultEnvelope{}, err
	}
	agg, wsID, userID, err := s.aggregateEntriesRangeForWorkspace(ctx, wsID, start, end, time.UTC, aggregateOptions{
		PageSize:              reportPageSize,
		IncludeEntries:        includeEntries,
		MaxEntries:            effectiveMax,
		SampleEntries:         5,
		CollectRunningEntries: true,
		ProjectID:             projectID,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	projects := projectSummariesFromAgg(agg)
	totals := totalsFromAgg(agg)
	data := QuickReportData{
		Range:               DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:              totals,
		RunningEntries:      agg.RunningList,
		ProjectsRepresented: len(projects),
		SuggestedActions:    reportSuggestedActions(projects, totals, start, end),
	}
	if len(projects) > 0 {
		data.TopProject = &projects[0]
	}
	if includeEntries {
		data.EntriesSample = agg.Entries
	} else {
		data.EntriesSample = agg.EntriesSample
	}
	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"days":        days,
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, limits))
	return ok("clockify_quick_report", data, meta), nil
}

func (s *Service) DetailedReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	start, end, err := parseRange(args)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// include_entries defaults to true for DetailedReport.
	includeEntries := true
	if v, exists := args["include_entries"]; exists {
		if b, isBool := v.(bool); isBool {
			includeEntries = b
		}
	}

	limits := s.reportLimitsForArgs(args)
	effectiveMax := limits.AppliedMaxEntries
	wsID, filterProjectID, err := s.reportProjectFilterID(ctx, args, "")
	if err != nil {
		return ResultEnvelope{}, err
	}
	agg, wsID, userID, err := s.aggregateEntriesRangeForWorkspace(ctx, wsID, start, end, time.UTC, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: includeEntries,
		MaxEntries:     effectiveMax,
		ProjectID:      filterProjectID,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}

	totals := totalsFromAgg(agg)
	projects := projectSummariesFromAgg(agg)
	var filteredEntries []clockify.TimeEntry
	if includeEntries {
		filteredEntries = agg.Entries
	}

	data := map[string]any{
		"range":            DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		"totals":           totals,
		"byProject":        projects,
		"suggestedActions": reportSuggestedActions(projects, totals, start, end),
	}
	if includeEntries {
		data["entries"] = filteredEntries
	}

	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, limits))
	return ok("clockify_detailed_report", data, meta), nil
}

func reportSuggestedActions(projects []ProjectSummary, totals SummaryTotals, start, end time.Time) []ToolSuggestion {
	startText := start.Format(time.RFC3339)
	endText := end.Format(time.RFC3339)
	actions := make([]ToolSuggestion, 0, 2)

	if len(projects) > 0 {
		top := projects[0]
		args := map[string]any{
			"start":     startText,
			"end":       endText,
			"page_size": 50,
		}
		if projectRef := reportProjectSuggestionRef(top); projectRef != "" {
			args["project"] = projectRef
		}
		actions = append(actions, ToolSuggestion{
			Tool:      "clockify_list_entries",
			Reason:    fmt.Sprintf("Drill into %s entries for this report range.", reportProjectSuggestionLabel(top)),
			Arguments: args,
		})
	}

	logReason := "Log additional confirmed work in this report range after collecting the exact project, description, start, and end."
	if totals.Entries == 0 {
		logReason = "This report found no entries; log confirmed missing work after collecting the exact project, description, start, and end."
	}
	actions = append(actions, ToolSuggestion{
		Tool:        "clockify_log_time",
		Reason:      logReason,
		Arguments:   map[string]any{"dry_run": true},
		MissingArgs: []string{"project", "description", "start", "end"},
	})

	return actions
}

func reportProjectSuggestionRef(project ProjectSummary) string {
	if strings.TrimSpace(project.ProjectID) != "" {
		return project.ProjectID
	}
	name := strings.TrimSpace(project.ProjectName)
	if name == "" || name == "(no project)" {
		return ""
	}
	return name
}

func reportProjectSuggestionLabel(project ProjectSummary) string {
	name := strings.TrimSpace(project.ProjectName)
	if name == "" {
		return "the top project"
	}
	return name
}

func (s *Service) reportProjectFilterID(ctx context.Context, args map[string]any, workspaceID string) (string, string, error) {
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectRef == "" {
		return workspaceID, "", nil
	}
	wsID := workspaceID
	if wsID == "" {
		var err error
		wsID, err = s.ResolveWorkspaceID(ctx)
		if err != nil {
			return "", "", err
		}
	}
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
	if err != nil {
		return "", "", err
	}
	return wsID, projectID, nil
}

// totalsFromAgg builds a SummaryTotals from an aggregateResult without
// touching raw entries.
func totalsFromAgg(agg *aggregateResult) SummaryTotals {
	return SummaryTotals{
		Entries:        agg.EntriesCount,
		RunningEntries: agg.RunningEntries,
		TotalSeconds:   agg.TotalSeconds,
		TotalHours:     hours(agg.TotalSeconds),
	}
}

// projectSummariesFromAgg materializes and sorts the per-project rollups
// from the streaming aggregator.
func projectSummariesFromAgg(agg *aggregateResult) []ProjectSummary {
	out := make([]ProjectSummary, 0, len(agg.ByProject))
	for _, b := range agg.ByProject {
		out = append(out, ProjectSummary{
			ProjectID:    b.ID,
			ProjectName:  b.Name,
			Entries:      b.Entries,
			TotalSeconds: b.TotalSeconds,
			TotalHours:   hours(b.TotalSeconds),
		})
	}
	sortProjectSummaries(out)
	return out
}

func sortProjectSummaries(out []ProjectSummary) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalSeconds == out[j].TotalSeconds {
			return out[i].ProjectName < out[j].ProjectName
		}
		return out[i].TotalSeconds > out[j].TotalSeconds
	})
}

// daySummariesFromAgg materializes and sorts the per-day rollups.
func daySummariesFromAgg(agg *aggregateResult) []DaySummary {
	out := make([]DaySummary, 0, len(agg.ByDay))
	for date, b := range agg.ByDay {
		out = append(out, DaySummary{
			Date:         date,
			Entries:      b.Entries,
			TotalSeconds: b.TotalSeconds,
			TotalHours:   hours(b.TotalSeconds),
		})
	}
	sortDaySummaries(out)
	return out
}

func sortDaySummaries(out []DaySummary) {
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
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
