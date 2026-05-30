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
	ResolveProjectNames   bool // hydrate missing project names for agent-facing aggregate summaries
}

// projectBucket accumulates seconds and entry count for a single project
// seen during aggregation. Project name is retained so we can build a
// ProjectSummary without keeping raw entries around.
type projectBucket struct {
	ID           string
	Name         string
	ClientID     string
	ClientName   string
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
	query["hydrated"] = "true"
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
			if !entryStartsWithinRange(entry, start, end) {
				continue
			}
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
			if opts.ResolveProjectNames {
				s.resolveAggregateProjectBuckets(ctx, wsID, result)
			}
			return result, wsID, user.ID, nil
		}
	}
	return nil, "", "", fmt.Errorf("report pagination safety stop reached at %d pages for range %s..%s; narrow the range", aggregatePageSafetyStop, start.Format(time.RFC3339), end.Format(time.RFC3339))
}

func entryStartsWithinRange(entry clockify.TimeEntry, start, end time.Time) bool {
	startTime, err := entry.StartTime()
	if err != nil {
		return true
	}
	return !startTime.Before(start) && startTime.Before(end)
}

func (s *Service) resolveAggregateProjectBuckets(ctx context.Context, wsID string, result *aggregateResult) {
	if s == nil || s.Client == nil || result == nil {
		return
	}
	toResolve := make(map[string]*projectBucket)
	for _, bucket := range result.ByProject {
		if bucket == nil || strings.TrimSpace(bucket.ID) == "" {
			continue
		}
		if strings.TrimSpace(bucket.Name) != "" && bucket.Name != "(no project)" {
			continue
		}
		toResolve[strings.TrimSpace(bucket.ID)] = bucket
	}
	if len(toResolve) == 0 {
		return
	}
	path, err := paths.Workspace(wsID, "projects")
	if err != nil {
		return
	}
	projects, err := clockify.ListAll[clockify.Project](ctx, s.Client, path, map[string]string{"hydrated": "true"})
	if err != nil {
		return
	}
	for _, project := range projects {
		bucket := toResolve[strings.TrimSpace(project.ID)]
		if bucket == nil {
			continue
		}
		bucket.Name = firstNonEmptyString(strings.TrimSpace(project.Name), bucket.Name)
		clientID, clientName := projectClientRef(project)
		bucket.ClientID = firstNonEmptyString(bucket.ClientID, clientID)
		bucket.ClientName = firstNonEmptyString(bucket.ClientName, clientName)
	}
}

func projectClientRef(project clockify.Project) (string, string) {
	clientID := strings.TrimSpace(project.ClientID)
	clientName := strings.TrimSpace(project.ClientName)
	if m, ok := project.Client.(map[string]any); ok {
		clientID = firstNonEmptyString(clientID, firstReportString(m, "id", "_id", "clientId"))
		clientName = firstNonEmptyString(clientName, firstReportString(m, "name", "clientName"))
	}
	return clientID, clientName
}

type reportLimitState struct {
	MaxEntries          int
	RequestedMaxEntries int
	MaxEntriesRequested bool
}

func (s *Service) reportLimitsForArgs(args map[string]any) reportLimitState {
	state := reportLimitState{}
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
		return state
	}
	state.MaxEntries = n
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
		"max_entries":         limits.MaxEntries,
		"applied_max_entries": limits.MaxEntries,
	}
	if limits.MaxEntriesRequested {
		limitMeta["requested_max_entries"] = limits.RequestedMaxEntries
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

// SummaryReport handles clockify_reports_summary: it runs Clockify's summary
// report for simple time totals over the requested range/grouping.
func (s *Service) SummaryReport(ctx context.Context, args map[string]any) (ToolResult, error) {
	return s.reportsAPIReport(ctx, args, reportEndpoint{
		toolName:  "clockify_reports_summary",
		pathName:  "summary",
		filterKey: "summary_filter",
	})
}

// WeeklySummary handles clockify_reports_weekly: it runs Clockify's weekly
// report over a 7-day range.
func (s *Service) WeeklySummary(ctx context.Context, args map[string]any) (ToolResult, error) {
	return s.weeklySummary(ctx, args, "")
}

func (s *Service) weeklySummary(ctx context.Context, args map[string]any, workspaceID string) (ToolResult, error) {
	if workspaceID != "" {
		withWorkspace := maps.Clone(args)
		if withWorkspace == nil {
			withWorkspace = map[string]any{}
		}
		withWorkspace["workspace_id"] = workspaceID
		args = withWorkspace
	}
	return s.reportsAPIReport(ctx, args, reportEndpoint{
		toolName:  "clockify_reports_weekly",
		pathName:  "weekly",
		filterKey: "weekly_filter",
	})
}

// QuickReport runs a local time-entries-wrapper rollup over the last N days
// (default 7), returning totals, top/per-project summaries, optional entry
// samples, and any running entries.
func (s *Service) QuickReport(ctx context.Context, args map[string]any) (ToolResult, error) {
	days := intArg(args, "days", 7)
	if days < 1 || days > quickReportMaxDays {
		return ToolResult{}, fmt.Errorf("days must be between 1 and %d", quickReportMaxDays)
	}
	loc, err := s.locationFromArgs(args)
	if err != nil {
		return ToolResult{}, err
	}
	end := time.Now().In(loc)
	start := end.AddDate(0, 0, -days)
	startUTC := start.UTC()
	endUTC := end.UTC()
	includeEntries := boolArg(args, "include_entries")
	limits := s.reportLimitsForArgs(args)
	wsID, projectID, err := s.reportProjectFilterID(ctx, args, "")
	if err != nil {
		return ToolResult{}, err
	}
	agg, wsID, userID, err := s.aggregateEntriesRangeForWorkspace(ctx, wsID, startUTC, endUTC, loc, aggregateOptions{
		PageSize:              reportPageSize,
		IncludeEntries:        includeEntries,
		MaxEntries:            limits.MaxEntries,
		SampleEntries:         5,
		CollectRunningEntries: true,
		ProjectID:             projectID,
		ResolveProjectNames:   true,
	})
	if err != nil {
		return ToolResult{}, err
	}
	projects := projectSummariesFromAgg(agg)
	totals := totalsFromAgg(agg)
	runningViews, runningFinancialMeta := s.enrichEntryViews(ctx, wsID, agg.RunningList)
	data := QuickReportData{
		Range:               DateRange{Start: startUTC.Format(time.RFC3339), End: endUTC.Format(time.RFC3339)},
		Totals:              totals,
		RunningEntries:      runningViews,
		ProjectsRepresented: len(projects),
		SuggestedActions:    reportSuggestedActions(projects, totals, startUTC, endUTC),
	}
	if len(projects) > 0 {
		data.TopProject = &projects[0]
	}
	var sampleFinancialMeta map[string]any
	if includeEntries {
		data.EntriesSample, sampleFinancialMeta = s.enrichEntryViews(ctx, wsID, agg.Entries)
	} else {
		data.EntriesSample, sampleFinancialMeta = s.enrichEntryViews(ctx, wsID, agg.EntriesSample)
	}
	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"days":        days,
		"timezone":    loc.String(),
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, limits))
	meta["financials"] = map[string]any{
		"running_entries": runningFinancialMeta,
		"entries_sample":  sampleFinancialMeta,
	}
	return ok("clockify_reports_summary", data, meta), nil
}

// MoneyReport handles clockify_reports_money: it runs the summary report tuned
// for billing-rate money breakdowns by user or project.
func (s *Service) MoneyReport(ctx context.Context, args map[string]any) (ToolResult, error) {
	return s.reportsAPIReport(ctx, args, reportEndpoint{
		toolName:  "clockify_reports_money",
		pathName:  "summary",
		filterKey: "summary_filter",
	})
}

// ExpenseReport handles clockify_reports_expense: it runs Clockify's detailed
// expense report over the requested range.
func (s *Service) ExpenseReport(ctx context.Context, args map[string]any) (ToolResult, error) {
	return s.reportsAPIReport(ctx, args, reportEndpoint{
		toolName:  "clockify_reports_expense",
		pathName:  "expenses/detailed",
		filterKey: "detailed_filter",
	})
}

// DetailedReport handles clockify_reports_detailed: it runs Clockify's detailed
// time report over the requested range.
func (s *Service) DetailedReport(ctx context.Context, args map[string]any) (ToolResult, error) {
	return s.reportsAPIReport(ctx, args, reportEndpoint{
		toolName:  "clockify_reports_detailed",
		pathName:  "detailed",
		filterKey: "detailed_filter",
	})
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
			Tool:      "clockify_entries_list",
			Reason:    fmt.Sprintf("Drill into %s entries for this report range.", reportProjectSuggestionLabel(top)),
			Arguments: args,
		})
	}

	logReason := "Log additional confirmed work in this report range after collecting the exact project, description, start, and end."
	if totals.Entries == 0 {
		logReason = "This report found no entries; log confirmed missing work after collecting the exact project, description, start, and end."
	}
	actions = append(actions, ToolSuggestion{
		Tool:        oneUserToolLogWork,
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
			ClientID:     b.ClientID,
			ClientName:   b.ClientName,
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
