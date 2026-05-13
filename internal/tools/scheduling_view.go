package tools

import (
	"context"
	"fmt"
	"maps"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/paths"
)

var assignmentReportGroups = []string{"USER", "PROJECT", "CLIENT", "TASK", "DATE", "WEEK", "MONTH"}

type AssignmentView map[string]any

func assignmentReportCSVColumns() []string {
	return []string{
		"User",
		"Project",
		"Client",
		"Task",
		"Scheduled",
		"Available",
		"Amount Scheduled (USD)",
		"Cost Scheduled (USD)",
		"Expected Profit (USD)",
		"Tracked",
		"Amount Tracked (USD)",
		"Cost Tracked (USD)",
		"Difference",
		"Amount Difference (USD)",
		"Cost Difference (USD)",
		"Realized Profit (USD)",
		"Status",
	}
}

type AssignmentDurationView struct {
	Seconds int64   `json:"seconds"`
	Hours   float64 `json:"hours"`
	Display string  `json:"display,omitempty"`
}

type AssignmentReportMoney struct {
	Earned *MoneyView `json:"earned,omitempty"`
	Cost   *MoneyView `json:"cost,omitempty"`
	Profit *MoneyView `json:"profit,omitempty"`
	Source string     `json:"source"`
	Reason string     `json:"reason,omitempty"`
}

type AssignmentReportRow struct {
	Title            string                 `json:"title"`
	GroupKey         map[string]string      `json:"group_key"`
	Entities         map[string]any         `json:"entities,omitempty"`
	Scheduled        AssignmentDurationView `json:"scheduled"`
	Available        AssignmentDurationView `json:"available"`
	AmountScheduled  *MoneyView             `json:"amount_scheduled,omitempty"`
	CostScheduled    *MoneyView             `json:"cost_scheduled,omitempty"`
	ExpectedProfit   *MoneyView             `json:"expected_profit,omitempty"`
	Tracked          AssignmentDurationView `json:"tracked"`
	AmountTracked    *MoneyView             `json:"amount_tracked,omitempty"`
	CostTracked      *MoneyView             `json:"cost_tracked,omitempty"`
	Difference       AssignmentDurationView `json:"difference"`
	AmountDifference *MoneyView             `json:"amount_difference,omitempty"`
	CostDifference   *MoneyView             `json:"cost_difference,omitempty"`
	RealizedProfit   *MoneyView             `json:"realized_profit,omitempty"`
	Status           string                 `json:"status,omitempty"`
	Financials       AssignmentReportMoney  `json:"financials"`
	Source           string                 `json:"source"`
	Warnings         []string               `json:"warnings,omitempty"`
	Raw              map[string]any         `json:"raw,omitempty"`
}

type AssignmentReportTotals struct {
	Scheduled        AssignmentDurationView `json:"scheduled"`
	Available        AssignmentDurationView `json:"available"`
	Tracked          AssignmentDurationView `json:"tracked"`
	Difference       AssignmentDurationView `json:"difference"`
	AmountScheduled  *MoneyView             `json:"amount_scheduled,omitempty"`
	CostScheduled    *MoneyView             `json:"cost_scheduled,omitempty"`
	ExpectedProfit   *MoneyView             `json:"expected_profit,omitempty"`
	AmountTracked    *MoneyView             `json:"amount_tracked,omitempty"`
	CostTracked      *MoneyView             `json:"cost_tracked,omitempty"`
	AmountDifference *MoneyView             `json:"amount_difference,omitempty"`
	CostDifference   *MoneyView             `json:"cost_difference,omitempty"`
	RealizedProfit   *MoneyView             `json:"realized_profit,omitempty"`
}

type AssignmentReportData struct {
	Range  FinancialRangeView     `json:"range"`
	Groups []string               `json:"groups"`
	Rows   []AssignmentReportRow  `json:"rows"`
	Totals AssignmentReportTotals `json:"totals"`
	Raw    map[string]any         `json:"raw,omitempty"`
}

type assignmentReportAccumulator struct {
	key              string
	groupKey         map[string]string
	entities         map[string]any
	scheduledSeconds int64
	availableSeconds int64
	trackedSeconds   int64
	amountScheduled  *MoneyView
	costScheduled    *MoneyView
	amountTracked    *MoneyView
	costTracked      *MoneyView
	expectedProfit   *MoneyView
	realizedProfit   *MoneyView
	status           string
	warnings         []string
	raw              map[string]any
}

func assignmentViewEnvelope(action string, slice bool) map[string]any {
	view := assignmentViewSchema()
	data := view
	if slice {
		data = map[string]any{"type": "array", "items": view}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"ok", "action", "data"},
		"properties": map[string]any{
			"ok":     map[string]any{"type": "boolean"},
			"action": map[string]any{"type": "string", "const": action},
			"data":   data,
			"meta": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}
}

func assignmentViewSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			"scheduled": map[string]any{"type": "object", "additionalProperties": true},
			"tracked":   map[string]any{"type": "object", "additionalProperties": true},
			"financials": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"properties": map[string]any{
					"source": map[string]any{"type": "string", "enum": []string{entryFinancialSourceReportsAPI, entryFinancialSourceDerivedRates, entryFinancialSourceUnavailable, "mixed"}},
				},
			},
			"variance": map[string]any{"type": "object", "additionalProperties": true},
			"entities": map[string]any{"type": "object", "additionalProperties": true},
			"warnings": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
}

func assignmentReportInputSchema() map[string]any {
	filter := func(desc string) map[string]any {
		return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}}
	}
	return map[string]any{
		"type":     "object",
		"required": []string{"start", "end"},
		"properties": map[string]any{
			"start":         map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
			"end":           map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
			"timezone":      timezoneInputProperty(),
			"page":          map[string]any{"type": "integer", "description": "Page number (default 1)"},
			"page_size":     map[string]any{"type": "integer", "description": "Items per page (default 50, max 200 upstream for user totals)"},
			"status_filter": map[string]any{"type": "string", "enum": []string{"PUBLISHED", "UNPUBLISHED", "ALL"}},
			"search":        map[string]any{"type": "string", "description": "Search users/projects where supported by scheduling endpoints"},
			"group_by":      map[string]any{"type": "array", "minItems": 1, "description": "1 to 3 grouping levels", "items": map[string]any{"type": "string", "enum": assignmentReportGroups}},
			"users":         filter("User IDs to include"),
			"user_groups":   filter("User group IDs to include"),
			"projects":      filter("Project IDs to include"),
			"clients":       filter("Client IDs to include"),
			"tasks":         filter("Task IDs to include"),
			"user_id":       map[string]any{"type": "string", "description": "Convenience single-user filter; ID, name, or email"},
			"project_id":    map[string]any{"type": "string", "description": "Convenience single-project filter; ID or name"},
		},
	}
}

func userScheduleTotalsInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"start", "end"},
		"properties": map[string]any{
			"start":         map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
			"end":           map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
			"timezone":      timezoneInputProperty(),
			"page":          map[string]any{"type": "integer", "description": "Page number (default 1)"},
			"page_size":     map[string]any{"type": "integer", "description": "Items per page (default 50, max 200 upstream)"},
			"search":        map[string]any{"type": "string"},
			"status_filter": map[string]any{"type": "string", "enum": []string{"PUBLISHED", "UNPUBLISHED", "ALL"}},
			"users":         stringArraySchema("User IDs to include"),
			"user_groups":   stringArraySchema("User group IDs to include"),
		},
	}
}

func singleProjectScheduleTotalsInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"project_id", "start", "end"},
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or name"},
			"start":      map[string]any{"type": "string", "description": "Range start. " + flexibleDatetimeDescription},
			"end":        map[string]any{"type": "string", "description": "Range end. " + flexibleDatetimeDescription},
			"timezone":   timezoneInputProperty(),
		},
	}
}

func (s *Service) enrichAssignmentViews(ctx context.Context, wsID string, assignments []map[string]any, args map[string]any) ([]AssignmentView, map[string]any) {
	views := make([]AssignmentView, 0, len(assignments))
	meta := map[string]any{"assignments": len(assignments), "source": entryFinancialSourceUnavailable}
	start, end, _ := assignmentRangeFromArgs(args, s.DefaultTimezone)
	for _, assignment := range assignments {
		view := assignmentViewFromRaw(assignment, start, end)
		views = append(views, view)
	}
	if len(views) == 0 {
		return views, meta
	}
	tracked, err := s.reportFinancialsForAssignmentViews(ctx, wsID, views, []string{"USER", "PROJECT", "TASK"}, start, end, args)
	matched := 0
	for i := range views {
		key := assignmentReportKeyForRaw(views[i], []string{"USER", "PROJECT", "TASK"}, start, s.DefaultTimezone)
		if value, ok := tracked[key]; ok {
			applyTrackedToAssignmentView(views[i], value)
			matched++
		}
	}
	meta["reports_api_matched"] = matched
	if matched > 0 {
		meta["source"] = entryFinancialSourceReportsAPI
	}
	if err != nil {
		meta["reports_api_error"] = err.Error()
		for i := range views {
			appendAssignmentViewWarning(views[i], "reports API enrichment unavailable: "+err.Error())
		}
	}
	return views, meta
}

func appendAssignmentViewWarning(view AssignmentView, warning string) {
	if warning == "" {
		return
	}
	existing, _ := view["warnings"].([]string)
	view["warnings"] = append(existing, warning)
}

func assignmentViewFromRaw(raw map[string]any, start, end time.Time) AssignmentView {
	view := AssignmentView{}
	for k, v := range raw {
		view[k] = v
	}
	scheduledSeconds := scheduledSecondsFromAssignment(raw, start, end)
	view["scheduled"] = map[string]any{
		"duration": durationView(scheduledSeconds),
		"source":   "assignment",
	}
	view["tracked"] = map[string]any{
		"duration": durationView(0),
		"source":   entryFinancialSourceUnavailable,
	}
	view["financials"] = map[string]any{
		"source": entryFinancialSourceUnavailable,
		"reason": "tracked financial enrichment was not available",
	}
	view["variance"] = map[string]any{
		"duration": durationView(scheduledSeconds),
		"source":   "scheduled_minus_tracked",
	}
	view["entities"] = assignmentEntities(raw)
	view["assignment"] = assignmentDetails(raw)
	return view
}

func applyTrackedToAssignmentView(view AssignmentView, tracked assignmentReportAccumulator) {
	view["tracked"] = map[string]any{
		"duration": durationView(tracked.trackedSeconds),
		"source":   entryFinancialSourceReportsAPI,
	}
	scheduledSeconds := durationSecondsFromNested(view["scheduled"])
	view["variance"] = map[string]any{
		"duration": durationView(scheduledSeconds - tracked.trackedSeconds),
		"source":   "scheduled_minus_tracked",
	}
	financials := assignmentFinancialsMap(tracked.amountTracked, tracked.costTracked, tracked.realizedProfit, entryFinancialSourceReportsAPI, "")
	view["financials"] = financials
}

func (s *Service) AssignmentReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	args, err = s.assignmentReportArgsWithResolvedFilters(ctx, wsID, args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	start, end, err := assignmentRangeFromArgs(args, s.DefaultTimezone)
	if err != nil {
		return ResultEnvelope{}, err
	}
	groups, err := assignmentGroupsFromArgs(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	assignments, page, pageSize, err := s.listAssignmentsRaw(ctx, wsID, args)
	if err != nil {
		return ResultEnvelope{}, err
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

func (s *Service) getWorkspaceScheduleUserTotals(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	totals, err := s.getWorkspaceScheduleUserTotalsRaw(ctx, wsID, args)
	if err != nil {
		return ResultEnvelope{}, err
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

func (s *Service) getSingleProjectScheduleTotals(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := s.resolveProjectID(ctx, wsID, stringArg(args, "project_id"))
	if err != nil {
		return ResultEnvelope{}, err
	}
	loc, _ := s.locationFromArgs(args)
	start, end, err := schedulingRangeArgs(args, loc)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "scheduling", "assignments", "projects", "totals", projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var total map[string]any
	if err := s.Client.Get(ctx, path, map[string]string{"start": start, "end": end}, &total); err != nil {
		return ResultEnvelope{}, err
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

func assignmentBucketTime(raw map[string]any, fallback time.Time, defaultTZ *time.Location) time.Time {
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

func (s *Service) reportFinancialsForAssignmentReport(ctx context.Context, wsID string, groups []string, start, end time.Time, args map[string]any) (map[string]assignmentReportAccumulator, error) {
	return s.reportFinancialsForAssignmentGroups(ctx, wsID, groups, start, end, args)
}

func (s *Service) reportFinancialsForAssignmentViews(ctx context.Context, wsID string, views []AssignmentView, groups []string, start, end time.Time, args map[string]any) (map[string]assignmentReportAccumulator, error) {
	return s.reportFinancialsForAssignmentGroups(ctx, wsID, groups, start, end, args)
}

func (s *Service) reportFinancialsForAssignmentGroups(ctx context.Context, wsID string, groups []string, start, end time.Time, args map[string]any) (map[string]assignmentReportAccumulator, error) {
	out := map[string]assignmentReportAccumulator{}
	if s == nil || s.Client == nil || !s.entryFinancialReportsEnabled() {
		return out, fmt.Errorf("reports API enrichment disabled for non-canonical Clockify base URL")
	}
	if !assignmentGroupsUseSummary(groups) {
		return s.reportFinancialsForAssignmentGroupsDetailed(ctx, wsID, groups, start, end, args)
	}
	path, err := paths.Workspace(wsID, "reports", "summary")
	if err != nil {
		return out, err
	}
	body := map[string]any{
		"dateRangeStart": start.Format(time.RFC3339),
		"dateRangeEnd":   end.Format(time.RFC3339),
		"dateRangeType":  "ABSOLUTE",
		"amountShown":    "EARNED",
		"amounts":        []string{"EARNED", "COST", "PROFIT"},
		"summaryFilter":  map[string]any{"groups": groups},
	}
	if tz := strings.TrimSpace(stringArg(args, "timezone")); tz != "" {
		body["timeZone"] = tz
	}
	applyAssignmentReportFilters(body, args)
	var payload map[string]any
	if err := s.Client.PostReports(ctx, path, body, &payload); err != nil {
		return out, err
	}
	collectAssignmentReportRows(payload, groups, out)
	return out, nil
}

func assignmentGroupsUseSummary(groups []string) bool {
	if len(groups) == 0 || len(groups) > 3 {
		return false
	}
	for _, group := range groups {
		switch strings.ToUpper(strings.TrimSpace(group)) {
		case "CLIENT", "PROJECT", "TASK", "DATE", "WEEK", "MONTH", "TIMEENTRY":
		default:
			return false
		}
	}
	return true
}

func (s *Service) reportFinancialsForAssignmentGroupsDetailed(ctx context.Context, wsID string, groups []string, start, end time.Time, args map[string]any) (map[string]assignmentReportAccumulator, error) {
	out := map[string]assignmentReportAccumulator{}
	path, err := paths.Workspace(wsID, "reports", "detailed")
	if err != nil {
		return out, err
	}
	pageSize := min(max(intArg(args, "page_size", 100), 1), 1000)
	for page := 1; page <= aggregatePageSafetyStop; page++ {
		body := map[string]any{
			"dateRangeStart": start.Format(time.RFC3339),
			"dateRangeEnd":   end.Format(time.RFC3339),
			"dateRangeType":  "ABSOLUTE",
			"amountShown":    "EARNED",
			"amounts":        []string{"EARNED", "COST", "PROFIT"},
			"detailedFilter": map[string]any{
				"page":     page,
				"pageSize": pageSize,
				"options":  map[string]any{"totals": "CALCULATE"},
			},
		}
		if tz := strings.TrimSpace(stringArg(args, "timezone")); tz != "" {
			body["timeZone"] = tz
		}
		applyAssignmentReportFilters(body, args)
		var payload map[string]any
		if err := s.Client.PostReports(ctx, path, body, &payload); err != nil {
			return out, err
		}
		rows := reportTimeEntryRows(payload)
		for _, item := range rows {
			keyMap := assignmentGroupKeyForRaw(item, groups, start, s.DefaultTimezone)
			key := reportGroupKeyString(keyMap, groups)
			row := out[key]
			row.key = key
			row.groupKey = keyMap
			if row.raw == nil {
				row.raw = map[string]any{"reports_detailed": []map[string]any{}}
			}
			if list, ok := row.raw["reports_detailed"].([]map[string]any); ok {
				row.raw["reports_detailed"] = append(list, item)
			}
			row.trackedSeconds += reportDurationSecondsOrZero(item)
			money := reportMoneyFromRow(item)
			row.amountTracked = moneySum(row.amountTracked, money.earned)
			row.costTracked = moneySum(row.costTracked, money.cost)
			row.realizedProfit, _ = profitMoney(row.amountTracked, row.costTracked, money.reason)
			row.entities = mergeAssignmentEntities(row.entities, reportEntities(item))
			row.sourceAppend("reports_detailed")
			out[key] = row
		}
		if len(rows) < pageSize {
			return out, nil
		}
	}
	return out, fmt.Errorf("reports API detailed pagination safety stop reached at %d pages", aggregatePageSafetyStop)
}

func applyAssignmentReportFilters(body map[string]any, args map[string]any) {
	for _, spec := range []struct {
		arg    string
		body   string
		status string
	}{
		{"users", "users", "ALL"},
		{"user_groups", "userGroups", "ALL"},
		{"projects", "projects", "ALL"},
		{"clients", "clients", "ALL"},
		{"tasks", "tasks", "ALL"},
	} {
		ids, ok, err := strictStringSliceArg(args, spec.arg)
		if err == nil && ok && len(ids) > 0 {
			body[spec.body] = map[string]any{"contains": "CONTAINS", "ids": ids, "status": spec.status}
		}
	}
}

func collectAssignmentReportRows(v any, groups []string, out map[string]assignmentReportAccumulator) {
	switch item := v.(type) {
	case []any:
		for _, child := range item {
			collectAssignmentReportRows(child, groups, out)
		}
	case map[string]any:
		if looksLikeSummaryGroup(item) {
			keyMap := reportGroupKeyForRow(item, groups)
			key := reportGroupKeyString(keyMap, groups)
			row := out[key]
			row.key = key
			row.groupKey = keyMap
			row.trackedSeconds += reportDurationSecondsOrZero(item)
			money := reportMoneyFromRow(item)
			row.amountTracked = moneySum(row.amountTracked, money.earned)
			row.costTracked = moneySum(row.costTracked, money.cost)
			row.realizedProfit, _ = profitMoney(row.amountTracked, row.costTracked, money.reason)
			row.entities = reportEntities(item)
			if row.raw == nil {
				row.raw = map[string]any{}
			}
			row.raw["report"] = item
			out[key] = row
		}
		for _, child := range item {
			collectAssignmentReportRows(child, groups, out)
		}
	}
}

func looksLikeSummaryGroup(row map[string]any) bool {
	if _, ok := reportDurationSeconds(row); ok {
		return true
	}
	return reportMoneyFromRow(row).hasMoney()
}

func reportDurationSecondsOrZero(row map[string]any) int64 {
	if seconds, ok := reportDurationSeconds(row); ok {
		return seconds
	}
	return 0
}

func reportGroupKeyForRow(row map[string]any, groups []string) map[string]string {
	out := map[string]string{}
	for _, group := range groups {
		out[group] = reportGroupValue(row, group)
	}
	return out
}

func reportGroupValue(row map[string]any, group string) string {
	kind := strings.ToLower(group)
	switch group {
	case "DATE", "WEEK", "MONTH":
		if value := reportTemporalGroupValue(row, group); value != "" {
			return value
		}
	}
	if id := reportGroupID(row, kind); id != "" {
		return id
	}
	for _, key := range []string{kind + "Name", kind + "_name", kind, "name", "title"} {
		if s := cleanReportID(row[key]); s != "" {
			return s
		}
	}
	return "(without " + strings.ToLower(group) + ")"
}

func reportTemporalGroupValue(row map[string]any, group string) string {
	for _, key := range []string{strings.ToLower(group), "date", "start", "time", "name", "title"} {
		if text := cleanReportID(row[key]); text != "" {
			if t, ok := parseAssignmentTime(text); ok {
				switch group {
				case "DATE":
					return t.Format("2006-01-02")
				case "WEEK":
					year, week := t.ISOWeek()
					return fmt.Sprintf("%04d-W%02d", year, week)
				case "MONTH":
					return t.Format("2006-01")
				}
			}
			return text
		}
	}
	if interval, ok := row["timeInterval"].(map[string]any); ok {
		if text := cleanReportID(firstPresent(interval, "start")); text != "" {
			if t, ok := parseAssignmentTime(text); ok {
				switch group {
				case "DATE":
					return t.Format("2006-01-02")
				case "WEEK":
					year, week := t.ISOWeek()
					return fmt.Sprintf("%04d-W%02d", year, week)
				case "MONTH":
					return t.Format("2006-01")
				}
			}
		}
	}
	return ""
}

func reportGroupKeyString(groupKey map[string]string, groups []string) string {
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		parts = append(parts, group+"="+groupKey[group])
	}
	return strings.Join(parts, "|")
}

func reportEntities(row map[string]any) map[string]any {
	return map[string]any{
		"user":    nestedReportEntity(row, "user"),
		"project": nestedReportEntity(row, "project"),
		"client":  nestedReportEntity(row, "client"),
		"task":    nestedReportEntity(row, "task"),
	}
}

func mergeAssignmentEntities(a, b map[string]any) map[string]any {
	if a == nil {
		a = map[string]any{}
	}
	for key, value := range b {
		if existing, ok := a[key].(map[string]any); ok {
			candidate, _ := value.(map[string]any)
			if cleanReportID(firstPresent(existing, "id", "name")) != "" {
				continue
			}
			if cleanReportID(firstPresent(candidate, "id", "name")) != "" {
				a[key] = candidate
			}
			continue
		}
		a[key] = value
	}
	return a
}

func nestedReportEntity(row map[string]any, kind string) map[string]any {
	id := reportEntityID(row, kind)
	name := cleanReportID(firstPresent(row, kind+"Name", kind+"_name"))
	if nested, ok := row[kind].(map[string]any); ok {
		id = firstNonEmptyString(id, cleanReportID(firstPresent(nested, "id", "_id")))
		name = firstNonEmptyString(name, cleanReportID(firstPresent(nested, "name", "title")))
	}
	return map[string]any{"id": id, "name": name}
}

func reportEntityID(row map[string]any, kind string) string {
	for _, key := range []string{kind + "Id", kind + "_id"} {
		if id := cleanReportID(row[key]); id != "" {
			return id
		}
	}
	if nested, ok := row[kind].(map[string]any); ok {
		return cleanReportID(firstPresent(nested, "id", "_id"))
	}
	return ""
}

func materializeAssignmentReportRows(rowsByKey map[string]*assignmentReportAccumulator) []AssignmentReportRow {
	keys := make([]string, 0, len(rowsByKey))
	for key := range rowsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]AssignmentReportRow, 0, len(keys))
	for _, key := range keys {
		acc := rowsByKey[key]
		amountDiff := moneyDiff(acc.amountScheduled, acc.amountTracked)
		costDiff := moneyDiff(acc.costScheduled, acc.costTracked)
		expectedProfit, expectedReason := profitMoney(acc.amountScheduled, acc.costScheduled, "")
		realizedProfit, realizedReason := profitMoney(acc.amountTracked, acc.costTracked, "")
		reason := firstNonEmptyString(expectedReason, realizedReason)
		row := AssignmentReportRow{
			Title:            assignmentRowTitle(acc.groupKey),
			GroupKey:         acc.groupKey,
			Entities:         acc.entities,
			Scheduled:        durationView(acc.scheduledSeconds),
			Available:        durationView(acc.availableSeconds),
			AmountScheduled:  acc.amountScheduled,
			CostScheduled:    acc.costScheduled,
			ExpectedProfit:   firstMoney(expectedProfit, acc.expectedProfit),
			Tracked:          durationView(acc.trackedSeconds),
			AmountTracked:    acc.amountTracked,
			CostTracked:      acc.costTracked,
			Difference:       durationView(acc.scheduledSeconds - acc.trackedSeconds),
			AmountDifference: amountDiff,
			CostDifference:   costDiff,
			RealizedProfit:   firstMoney(realizedProfit, acc.realizedProfit),
			Status:           acc.status,
			Financials:       AssignmentReportMoney{Earned: acc.amountTracked, Cost: acc.costTracked, Profit: firstMoney(realizedProfit, acc.realizedProfit), Source: assignmentFinancialSource(acc), Reason: reason},
			Source:           assignmentRowSource(acc),
			Warnings:         acc.warnings,
			Raw:              acc.raw,
		}
		rows = append(rows, row)
	}
	return rows
}

func assignmentRowTitle(groupKey map[string]string) string {
	parts := make([]string, 0, len(groupKey))
	for _, group := range assignmentReportGroups {
		if v := groupKey[group]; v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " / ")
}

func assignmentFinancialSource(acc *assignmentReportAccumulator) string {
	if acc == nil {
		return entryFinancialSourceUnavailable
	}
	if acc.amountTracked != nil || acc.costTracked != nil {
		return entryFinancialSourceReportsAPI
	}
	return entryFinancialSourceUnavailable
}

func assignmentRowSource(acc *assignmentReportAccumulator) string {
	if acc == nil {
		return entryFinancialSourceUnavailable
	}
	hasScheduled := acc.scheduledSeconds != 0 || acc.amountScheduled != nil || acc.costScheduled != nil
	hasTracked := acc.trackedSeconds != 0 || acc.amountTracked != nil || acc.costTracked != nil
	switch {
	case hasScheduled && hasTracked:
		return "scheduling_api+reports_api"
	case hasScheduled:
		return "scheduling_api"
	case hasTracked:
		return entryFinancialSourceReportsAPI
	default:
		return entryFinancialSourceUnavailable
	}
}

func assignmentReportTotals(rows []AssignmentReportRow) AssignmentReportTotals {
	var scheduled, available, tracked int64
	var amountScheduled, costScheduled, amountTracked, costTracked *MoneyView
	for _, row := range rows {
		scheduled += row.Scheduled.Seconds
		available += row.Available.Seconds
		tracked += row.Tracked.Seconds
		amountScheduled = moneySum(amountScheduled, row.AmountScheduled)
		costScheduled = moneySum(costScheduled, row.CostScheduled)
		amountTracked = moneySum(amountTracked, row.AmountTracked)
		costTracked = moneySum(costTracked, row.CostTracked)
	}
	expectedProfit, _ := profitMoney(amountScheduled, costScheduled, "")
	realizedProfit, _ := profitMoney(amountTracked, costTracked, "")
	return AssignmentReportTotals{
		Scheduled:        durationView(scheduled),
		Available:        durationView(available),
		Tracked:          durationView(tracked),
		Difference:       durationView(scheduled - tracked),
		AmountScheduled:  amountScheduled,
		CostScheduled:    costScheduled,
		ExpectedProfit:   expectedProfit,
		AmountTracked:    amountTracked,
		CostTracked:      costTracked,
		AmountDifference: moneyDiff(amountScheduled, amountTracked),
		CostDifference:   moneyDiff(costScheduled, costTracked),
		RealizedProfit:   realizedProfit,
	}
}

func trackedRawRows(rows map[string]assignmentReportAccumulator) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row.raw != nil {
			out = append(out, row.raw)
		}
	}
	return out
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
	view["raw"] = raw
	return view
}

func assignmentDetails(raw map[string]any) map[string]any {
	details := map[string]any{
		"id":                       firstReportString(raw, "id", "_id", "assignmentId", "assignment_id"),
		"published":                firstPresent(raw, "published", "isPublished"),
		"start_time":               firstReportString(raw, "startTime", "start_time"),
		"hours_per_day":            firstPresent(raw, "hoursPerDay", "hours_per_day"),
		"include_non_working_days": firstPresent(raw, "includeNonWorkingDays", "include_non_working_days"),
		"exclude_days":             firstPresent(raw, "excludeDays", "exclude_days"),
		"recurring":                firstPresent(raw, "recurring", "recurringAssignment", "recurring_assignment"),
		"period":                   firstPresent(raw, "period"),
		"source":                   "scheduling_api",
	}
	for key, value := range maps.Clone(details) {
		if value == nil || value == "" {
			delete(details, key)
		}
	}
	return details
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
			"raw":      maps.Clone(row),
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
			"raw":            maps.Clone(row),
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

func moneyFromAny(v any, currency string) *MoneyView {
	if v == nil {
		return nil
	}
	switch typed := v.(type) {
	case *MoneyView:
		return typed
	case MoneyView:
		return &typed
	}
	if nested, ok := v.(map[string]any); ok {
		currency = firstNonEmptyString(reportCurrency(nested), currency)
		v = firstPresent(nested, "amount_cents", "amountCents", "value", "amount", "total")
	}
	if n, ok := reportNumber(v); ok {
		return moneyPtr(MoneyFromReportNumber(n, currency))
	}
	return nil
}

func moneySum(a, b *MoneyView) *MoneyView {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Currency != "" && b.Currency != "" && a.Currency != b.Currency {
		return a
	}
	return moneyPtr(MoneyFromCents(a.AmountCents+b.AmountCents, firstNonEmptyString(a.Currency, b.Currency)))
}

func moneyDiff(a, b *MoneyView) *MoneyView {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		zero := MoneyFromCents(0, b.Currency)
		a = &zero
	}
	if b == nil {
		zero := MoneyFromCents(0, a.Currency)
		b = &zero
	}
	if a.Currency != "" && b.Currency != "" && a.Currency != b.Currency {
		return nil
	}
	return moneyPtr(MoneyFromCents(a.AmountCents-b.AmountCents, firstNonEmptyString(a.Currency, b.Currency)))
}

func firstMoney(values ...*MoneyView) *MoneyView {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
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
