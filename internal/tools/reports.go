package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

// aggregateOptions controls the streaming aggregator used by report tools.
type aggregateOptions struct {
	PageSize       int  // clamped to [50, 200]; default 200
	IncludeEntries bool // retain raw entries on the result (memory grows with range)
	MaxEntries     int  // hard cap when IncludeEntries is true; 0 = unlimited
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
	Entries        []clockify.TimeEntry // populated only if IncludeEntries
	TotalSeconds   int64
	RunningEntries int
	ByProject      map[string]*projectBucket // project id (or "" for unassigned) -> bucket
	ByDay          map[string]*dayBucket     // YYYY-MM-DD in requested tz -> bucket
	EntriesCount   int                       // total entries walked across all pages
	PagesFetched   int
}

// aggregatePageSafetyStop mirrors clockify.ListAll's 1000-page safeguard.
const aggregatePageSafetyStop = 1000

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

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, "", "", err
	}
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return nil, "", "", err
	}

	baseQuery := entryRangeQuery(start, end)

	result := &aggregateResult{
		ByProject: make(map[string]*projectBucket),
		ByDay:     make(map[string]*dayBucket),
	}

	path, err := paths.Workspace(wsID, "user", user.ID, "time-entries")
	if err != nil {
		return nil, "", "", err
	}
	for page := 1; page <= aggregatePageSafetyStop; page++ {
		query := make(map[string]string, len(baseQuery)+2)
		for k, v := range baseQuery {
			query[k] = v
		}
		query["page"] = strconv.Itoa(page)
		query["page-size"] = strconv.Itoa(pageSize)

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

// effectiveReportCap returns the cap to apply for a report call, honoring an
// optional caller-supplied override bounded by the server-wide cap.
func (s *Service) effectiveReportCap(args map[string]any) int {
	serverCap := s.ReportMaxEntries
	override, ok := args["max_entries"]
	if !ok {
		return serverCap
	}
	n := intArg(args, "max_entries", -1)
	if n < 0 {
		return serverCap
	}
	if n == 0 {
		// Explicit 0 means "no extra cap" from the caller; still bounded by
		// the server-wide cap.
		return serverCap
	}
	if serverCap > 0 && n > serverCap {
		return serverCap
	}
	_ = override
	return n
}

// paginationMeta builds the structured pagination/limits meta block for a
// report response.
func paginationMeta(agg *aggregateResult, pageSize, effectiveMax int) map[string]any {
	return map[string]any{
		"pagination": map[string]any{
			"page_size":     pageSize,
			"pages_fetched": agg.PagesFetched,
			"entries_total": agg.EntriesCount,
		},
		"limits": map[string]any{
			"max_entries": effectiveMax,
		},
	}
}

// mergeMeta merges the structured pagination meta into the caller's base meta.
func mergeMeta(base, extra map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for k, v := range extra {
		base[k] = v
	}
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
	effectiveMax := s.effectiveReportCap(args)
	agg, wsID, userID, err := s.aggregateEntriesRange(ctx, start, end, time.UTC, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: include,
		MaxEntries:     effectiveMax,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	data := SummaryData{
		Range:     DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:    totalsFromAgg(agg),
		ByProject: projectSummariesFromAgg(agg),
	}
	if include {
		data.Entries = agg.Entries
	}
	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, effectiveMax))
	return ok("clockify_summary_report", data, meta), nil
}

func (s *Service) WeeklySummary(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	loc, err := loadLocation(stringArg(args, "timezone"), s.DefaultTimezone)
	if err != nil {
		return ResultEnvelope{}, err
	}
	start, end, err := weekBounds(stringArg(args, "week_start"), loc)
	if err != nil {
		return ResultEnvelope{}, err
	}
	include := boolArg(args, "include_entries")
	effectiveMax := s.effectiveReportCap(args)
	agg, wsID, userID, err := s.aggregateEntriesRange(ctx, start, end, loc, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: include,
		MaxEntries:     effectiveMax,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	data := WeeklySummaryData{
		Range:         DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:        totalsFromAgg(agg),
		ByDay:         daySummariesFromAgg(agg),
		ByProject:     projectSummariesFromAgg(agg),
		UnassignedKey: "(no project)",
	}
	if include {
		data.Entries = agg.Entries
	}
	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"timezone":    loc.String(),
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, effectiveMax))
	return ok("clockify_weekly_summary", data, meta), nil
}

func (s *Service) QuickReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	days := intArg(args, "days", 7)
	if days < 1 || days > 31 {
		return ResultEnvelope{}, fmt.Errorf("days must be between 1 and 31")
	}
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)
	// QuickReport always needs entries: it samples them and surfaces running
	// entries. The cap still applies when the caller asks for full results.
	effectiveMax := s.effectiveReportCap(args)
	agg, wsID, userID, err := s.aggregateEntriesRange(ctx, start, end, time.UTC, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: true,
		MaxEntries:     effectiveMax,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}
	projects := projectSummariesFromAgg(agg)
	data := QuickReportData{
		Range:               DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		Totals:              totalsFromAgg(agg),
		RunningEntries:      runningEntries(agg.Entries),
		ProjectsRepresented: len(projects),
	}
	if len(projects) > 0 {
		data.TopProject = &projects[0]
	}
	if boolArg(args, "include_entries") {
		data.EntriesSample = agg.Entries
	} else if len(agg.Entries) > 5 {
		data.EntriesSample = agg.Entries[:5]
	} else {
		data.EntriesSample = agg.Entries
	}
	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"days":        days,
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, effectiveMax))
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

	effectiveMax := s.effectiveReportCap(args)
	agg, wsID, userID, err := s.aggregateEntriesRange(ctx, start, end, time.UTC, aggregateOptions{
		PageSize:       reportPageSize,
		IncludeEntries: includeEntries,
		MaxEntries:     effectiveMax,
	})
	if err != nil {
		return ResultEnvelope{}, err
	}

	projectRef := stringArg(args, "project")
	var filterProjectID string
	if projectRef != "" {
		filterProjectID, err = resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
	}

	var totals SummaryTotals
	var filteredEntries []clockify.TimeEntry
	if filterProjectID != "" {
		bucket, hasBucket := agg.ByProject[filterProjectID]
		if hasBucket {
			totals = SummaryTotals{
				Entries:      bucket.Entries,
				TotalSeconds: bucket.TotalSeconds,
				TotalHours:   hours(bucket.TotalSeconds),
			}
		}
		if includeEntries {
			filteredEntries = make([]clockify.TimeEntry, 0, totals.Entries)
			for _, e := range agg.Entries {
				if e.ProjectID == filterProjectID {
					filteredEntries = append(filteredEntries, e)
				}
			}
		}
	} else {
		totals = totalsFromAgg(agg)
		if includeEntries {
			filteredEntries = agg.Entries
		}
	}

	data := map[string]any{
		"range":  DateRange{Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)},
		"totals": totals,
	}
	if includeEntries {
		data["entries"] = filteredEntries
	}

	meta := mergeMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"source":      "time-entries-wrapper",
	}, paginationMeta(agg, reportPageSize, effectiveMax))
	return ok("clockify_detailed_report", data, meta), nil
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalSeconds == out[j].TotalSeconds {
			return out[i].ProjectName < out[j].ProjectName
		}
		return out[i].TotalSeconds > out[j].TotalSeconds
	})
	return out
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
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
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

func runningEntries(entries []clockify.TimeEntry) []clockify.TimeEntry {
	out := make([]clockify.TimeEntry, 0)
	for _, entry := range entries {
		if entry.IsRunning() {
			out = append(out, entry)
		}
	}
	return out
}
