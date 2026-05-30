package tools

import (
	"context"
	"fmt"
	"maps"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
)

const projectTaskEnrichmentPageSize = 200

// FinancialRangeView describes the date window a financial rollup was computed
// over: start/end, the date-range type, and timezone.
type FinancialRangeView struct {
	Start         string `json:"start,omitempty"`
	End           string `json:"end,omitempty"`
	DateRangeType string `json:"date_range_type,omitempty"`
	Timezone      string `json:"timezone,omitempty"`
}

// ProjectView is the fully enriched project output returned by project tools:
// the core project fields plus normalized custom fields, rates, financials,
// estimates, estimate progress, and embedded task views.
type ProjectView struct {
	ID                     string                       `json:"id"`
	Name                   string                       `json:"name"`
	ClientID               string                       `json:"clientId,omitempty"`
	ClientName             string                       `json:"clientName,omitempty"`
	Client                 any                          `json:"client,omitempty"`
	Color                  string                       `json:"color,omitempty"`
	Archived               bool                         `json:"archived"`
	Billable               bool                         `json:"billable,omitempty"`
	BudgetEstimate         any                          `json:"budgetEstimate,omitempty"`
	CostRate               *clockify.Rate               `json:"costRate,omitempty"`
	Currency               any                          `json:"currency,omitempty"`
	CustomFields           any                          `json:"customFields,omitempty"`
	CustomFieldsNormalized []CustomFieldValueView       `json:"custom_fields_normalized,omitempty"`
	Duration               string                       `json:"duration,omitempty"`
	Estimate               any                          `json:"estimate,omitempty"`
	EstimateReset          any                          `json:"estimateReset,omitempty"`
	Expenses               any                          `json:"expenses,omitempty"`
	Favorite               bool                         `json:"favorite,omitempty"`
	HourlyRate             *clockify.Rate               `json:"hourlyRate,omitempty"`
	Memberships            []clockify.ProjectMembership `json:"memberships,omitempty"`
	Note                   string                       `json:"note,omitempty"`
	Public                 bool                         `json:"public,omitempty"`
	Template               bool                         `json:"template,omitempty"`
	TimeEstimate           any                          `json:"timeEstimate,omitempty"`
	WorkspaceID            string                       `json:"workspaceId,omitempty"`

	Rates            ProjectRatesView     `json:"rates"`
	Financials       ProjectFinancials    `json:"financials"`
	Estimates        ProjectEstimateView  `json:"estimates"`
	EstimateProgress EstimateProgressView `json:"estimate_progress"`
	Tasks            []TaskView           `json:"tasks,omitempty"`
	ExpensesSummary  map[string]any       `json:"expenses_summary,omitempty"`
	RawHydrated      map[string]any       `json:"raw_hydrated,omitempty"`
}

// CompactProjectView is the trimmed project projection used in list responses:
// id, name, client, color, and the archived/billable/public/duration fields.
type CompactProjectView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ClientID   string `json:"clientId,omitempty"`
	ClientName string `json:"clientName,omitempty"`
	Color      string `json:"color,omitempty"`
	Archived   bool   `json:"archived"`
	Billable   bool   `json:"billable,omitempty"`
	Public     bool   `json:"public,omitempty"`
	Duration   string `json:"duration,omitempty"`
}

// TaskView is the enriched task output embedded in a ProjectView (or returned by
// task tools): the core task fields plus normalized custom fields, rates,
// financials, estimates, and estimate progress.
type TaskView struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name"`
	ProjectID              string                 `json:"projectId"`
	AssigneeID             string                 `json:"assigneeId,omitempty"`
	AssigneeIDs            []string               `json:"assigneeIds,omitempty"`
	Billable               bool                   `json:"billable"`
	BudgetEstimate         int64                  `json:"budgetEstimate,omitempty"`
	CostRate               *clockify.Rate         `json:"costRate,omitempty"`
	Duration               string                 `json:"duration,omitempty"`
	Estimate               string                 `json:"estimate,omitempty"`
	HourlyRate             *clockify.Rate         `json:"hourlyRate,omitempty"`
	Status                 string                 `json:"status,omitempty"`
	UserGroupIDs           []string               `json:"userGroupIds,omitempty"`
	CustomFieldsNormalized []CustomFieldValueView `json:"custom_fields_normalized,omitempty"`

	Rates            TaskRateView         `json:"rates"`
	Financials       ProjectFinancials    `json:"financials"`
	Estimates        ProjectEstimateView  `json:"estimates"`
	EstimateProgress EstimateProgressView `json:"estimate_progress"`
}

// ProjectRatesView aggregates a project's rates: project-level hourly/cost rates
// (raw and as MoneyView) plus per-member and per-task rate breakdowns.
type ProjectRatesView struct {
	ProjectHourly      *clockify.Rate          `json:"project_hourly,omitempty"`
	ProjectCost        *clockify.Rate          `json:"project_cost,omitempty"`
	ProjectHourlyMoney *MoneyView              `json:"project_hourly_money,omitempty"`
	ProjectCostMoney   *MoneyView              `json:"project_cost_money,omitempty"`
	Members            []ProjectMemberRateView `json:"members,omitempty"`
	Tasks              []TaskRateView          `json:"tasks,omitempty"`
}

// ProjectMemberRateView is the per-member rate projection within a
// ProjectRatesView: the membership identity plus hourly/cost rates (raw and as
// MoneyView).
type ProjectMemberRateView struct {
	UserID           string         `json:"user_id"`
	TargetID         string         `json:"target_id,omitempty"`
	MembershipType   string         `json:"membership_type,omitempty"`
	MembershipStatus string         `json:"membership_status,omitempty"`
	HourlyRate       *clockify.Rate `json:"hourly_rate,omitempty"`
	CostRate         *clockify.Rate `json:"cost_rate,omitempty"`
	HourlyMoney      *MoneyView     `json:"hourly_money,omitempty"`
	CostMoney        *MoneyView     `json:"cost_money,omitempty"`
}

// TaskRateView is the per-task rate projection within a ProjectRatesView: the
// task identity plus hourly/cost rates (raw and as MoneyView).
type TaskRateView struct {
	TaskID      string         `json:"task_id,omitempty"`
	TaskName    string         `json:"task_name,omitempty"`
	HourlyRate  *clockify.Rate `json:"hourly_rate,omitempty"`
	CostRate    *clockify.Rate `json:"cost_rate,omitempty"`
	HourlyMoney *MoneyView     `json:"hourly_money,omitempty"`
	CostMoney   *MoneyView     `json:"cost_money,omitempty"`
}

// ProjectFinancials holds a project's or task's earned/cost/profit money totals
// for a date range, along with the source/reason and the range they cover.
type ProjectFinancials struct {
	Earned *MoneyView          `json:"earned,omitempty"`
	Cost   *MoneyView          `json:"cost,omitempty"`
	Profit *MoneyView          `json:"profit,omitempty"`
	Source string              `json:"source"`
	Reason string              `json:"reason,omitempty"`
	Range  *FinancialRangeView `json:"range,omitempty"`
}

// ProjectEstimateView normalizes a project's or task's estimate fields: the raw
// upstream values plus their decoded duration/time-estimate seconds and budget.
type ProjectEstimateView struct {
	EstimateRaw         any        `json:"estimate_raw,omitempty"`
	BudgetEstimateRaw   any        `json:"budget_estimate_raw,omitempty"`
	TimeEstimateRaw     any        `json:"time_estimate_raw,omitempty"`
	EstimateResetRaw    any        `json:"estimate_reset_raw,omitempty"`
	DurationRaw         string     `json:"duration_raw,omitempty"`
	DurationSeconds     *int64     `json:"duration_seconds,omitempty"`
	EstimateSeconds     *int64     `json:"estimate_seconds,omitempty"`
	TimeEstimateSeconds *int64     `json:"time_estimate_seconds,omitempty"`
	BudgetEstimate      *MoneyView `json:"budget_estimate,omitempty"`
}

// EstimateProgressView compares tracked time against the estimate: tracked and
// remaining seconds, percent used, and an over-estimate flag, with source/reason.
type EstimateProgressView struct {
	TrackedDurationSeconds int64    `json:"tracked_duration_seconds"`
	EstimateSeconds        *int64   `json:"estimate_seconds,omitempty"`
	RemainingSeconds       *int64   `json:"remaining_seconds,omitempty"`
	PercentUsed            *float64 `json:"percent_used,omitempty"`
	OverEstimate           *bool    `json:"over_estimate,omitempty"`
	Source                 string   `json:"source"`
	Reason                 string   `json:"reason,omitempty"`
}

type reportGroupMoney struct {
	money           reportEntryMoney
	durationSeconds int64
	hasDuration     bool
}

func projectViewFromProject(project clockify.Project) ProjectView {
	view := ProjectView{
		ID:                     project.ID,
		Name:                   project.Name,
		ClientID:               project.ClientID,
		ClientName:             project.ClientName,
		Client:                 project.Client,
		Color:                  project.Color,
		Archived:               project.Archived,
		Billable:               project.Billable,
		BudgetEstimate:         project.BudgetEstimate,
		CostRate:               project.CostRate,
		Currency:               project.Currency,
		CustomFields:           project.CustomFields,
		CustomFieldsNormalized: customFieldValuesFromRaw(project.CustomFields),
		Duration:               project.Duration,
		Estimate:               project.Estimate,
		EstimateReset:          project.EstimateReset,
		Expenses:               project.Expenses,
		Favorite:               project.Favorite,
		HourlyRate:             project.HourlyRate,
		Memberships:            project.Memberships,
		Note:                   project.Note,
		Public:                 project.Public,
		Template:               project.Template,
		TimeEstimate:           project.TimeEstimate,
		WorkspaceID:            project.WorkspaceID,
		Rates:                  projectRatesFromProject(project),
		Financials: ProjectFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "financial enrichment was not available",
		},
		Estimates: estimatesFromProject(project),
	}
	view.ExpensesSummary = projectExpensesSummary(project.Expenses)
	view.RawHydrated = projectRawHydratedMap(project)
	view.EstimateProgress = progressFromEstimates(view.Estimates, 0, false)
	return view
}

func compactProjectViewFromProject(project clockify.Project) CompactProjectView {
	clientName := project.ClientName
	if clientName == "" {
		if client, ok := project.Client.(map[string]any); ok {
			clientName = firstReportString(client, "name", "clientName")
		}
	}
	return CompactProjectView{
		ID:         project.ID,
		Name:       project.Name,
		ClientID:   project.ClientID,
		ClientName: clientName,
		Color:      project.Color,
		Archived:   project.Archived,
		Billable:   project.Billable,
		Public:     project.Public,
		Duration:   project.Duration,
	}
}

func compactProjectViewsFromProjects(projects []clockify.Project) []CompactProjectView {
	out := make([]CompactProjectView, 0, len(projects))
	for _, project := range projects {
		out = append(out, compactProjectViewFromProject(project))
	}
	return out
}

func taskViewFromTask(task clockify.Task, project *clockify.Project) TaskView {
	view := TaskView{
		ID:             task.ID,
		Name:           task.Name,
		ProjectID:      task.ProjectID,
		AssigneeID:     task.AssigneeID,
		AssigneeIDs:    task.AssigneeIDs,
		Billable:       task.Billable,
		BudgetEstimate: task.BudgetEstimate,
		CostRate:       task.CostRate,
		Duration:       task.Duration,
		Estimate:       task.Estimate,
		HourlyRate:     task.HourlyRate,
		Status:         task.Status,
		UserGroupIDs:   task.UserGroupIDs,
		Rates:          taskRateFromTask(task),
		Financials: ProjectFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "financial enrichment was not available",
		},
		Estimates: estimatesFromTask(task, projectCurrency(project)),
	}
	view.EstimateProgress = progressFromEstimates(view.Estimates, 0, false)
	return view
}

func projectRatesFromProject(project clockify.Project) ProjectRatesView {
	rates := ProjectRatesView{
		ProjectHourly:      project.HourlyRate,
		ProjectCost:        project.CostRate,
		ProjectHourlyMoney: moneyPtr(MoneyFromRate(project.HourlyRate)),
		ProjectCostMoney:   moneyPtr(MoneyFromRate(project.CostRate)),
	}
	for _, membership := range project.Memberships {
		rates.Members = append(rates.Members, ProjectMemberRateView{
			UserID:           membership.UserID,
			TargetID:         membership.TargetID,
			MembershipType:   membership.MembershipType,
			MembershipStatus: membership.MembershipStatus,
			HourlyRate:       membership.HourlyRate,
			CostRate:         membership.CostRate,
			HourlyMoney:      moneyPtr(MoneyFromRate(membership.HourlyRate)),
			CostMoney:        moneyPtr(MoneyFromRate(membership.CostRate)),
		})
	}
	return rates
}

func taskRateFromTask(task clockify.Task) TaskRateView {
	return TaskRateView{
		TaskID:      task.ID,
		TaskName:    task.Name,
		HourlyRate:  task.HourlyRate,
		CostRate:    task.CostRate,
		HourlyMoney: moneyPtr(MoneyFromRate(task.HourlyRate)),
		CostMoney:   moneyPtr(MoneyFromRate(task.CostRate)),
	}
}

func estimatesFromProject(project clockify.Project) ProjectEstimateView {
	currency := projectCurrency(&project)
	out := ProjectEstimateView{
		EstimateRaw:       project.Estimate,
		BudgetEstimateRaw: project.BudgetEstimate,
		TimeEstimateRaw:   project.TimeEstimate,
		EstimateResetRaw:  project.EstimateReset,
		DurationRaw:       project.Duration,
	}
	if seconds, ok := clockifyDurationSeconds(project.Duration); ok {
		out.DurationSeconds = &seconds
	}
	if seconds, ok := estimateSecondsFromRaw(project.Estimate); ok {
		out.EstimateSeconds = &seconds
	}
	if seconds, ok := estimateSecondsFromRaw(project.TimeEstimate); ok {
		out.TimeEstimateSeconds = &seconds
		out.EstimateSeconds = &seconds
	}
	if budget, ok := budgetEstimateFromRaw(project.BudgetEstimate, currency); ok {
		out.BudgetEstimate = &budget
	}
	return out
}

func estimatesFromTask(task clockify.Task, currency string) ProjectEstimateView {
	out := ProjectEstimateView{
		EstimateRaw:       task.Estimate,
		BudgetEstimateRaw: task.BudgetEstimate,
		DurationRaw:       task.Duration,
	}
	if seconds, ok := clockifyDurationSeconds(task.Duration); ok {
		out.DurationSeconds = &seconds
	}
	if seconds, ok := clockifyDurationSeconds(task.Estimate); ok {
		out.EstimateSeconds = &seconds
	}
	if task.BudgetEstimate > 0 {
		budget := MoneyFromCents(task.BudgetEstimate, currency)
		out.BudgetEstimate = &budget
	}
	return out
}

func progressFromEstimates(est ProjectEstimateView, reportDuration int64, hasReportDuration bool) EstimateProgressView {
	tracked := int64(0)
	source := "unavailable"
	if est.DurationSeconds != nil {
		tracked = *est.DurationSeconds
		source = "project_duration"
	} else if hasReportDuration {
		tracked = reportDuration
		source = entryFinancialSourceReportsAPI
	}
	progress := EstimateProgressView{
		TrackedDurationSeconds: tracked,
		EstimateSeconds:        est.EstimateSeconds,
		Source:                 source,
	}
	if est.EstimateSeconds == nil {
		progress.Reason = "no time estimate was available"
		return progress
	}
	remaining := *est.EstimateSeconds - tracked
	progress.RemainingSeconds = &remaining
	percent := 0.0
	if *est.EstimateSeconds > 0 {
		percent = float64(tracked) / float64(*est.EstimateSeconds) * 100
	}
	progress.PercentUsed = &percent
	over := remaining < 0
	progress.OverEstimate = &over
	return progress
}

func (s *Service) enrichProjectView(ctx context.Context, wsID string, project clockify.Project, args map[string]any) (ProjectView, map[string]any) {
	views, meta := s.enrichProjectViews(ctx, wsID, []clockify.Project{project}, args)
	if len(views) == 0 {
		return projectViewFromProject(project), meta
	}
	return views[0], meta
}

func (s *Service) enrichProjectViews(ctx context.Context, wsID string, projects []clockify.Project, args map[string]any) ([]ProjectView, map[string]any) {
	views := make([]ProjectView, len(projects))
	projectIDs := make([]string, 0, len(projects))
	for i := range projects {
		views[i] = projectViewFromProject(projects[i])
		if projects[i].ID != "" {
			projectIDs = append(projectIDs, projects[i].ID)
		}
	}
	meta := map[string]any{
		"projects": len(projects),
		"source":   entryFinancialSourceUnavailable,
	}
	if len(projects) == 0 {
		return views, meta
	}
	finRange := financialRangeFromArgs(args)
	for i := range views {
		views[i].Financials.Range = &finRange
	}
	if !s.projectDeepEnrichmentEnabled() {
		meta["reason"] = "deep project enrichment disabled for non-canonical Clockify base URL"
		return views, meta
	}

	taskIDs := map[string]struct{}{}
	for i := range views {
		tasks := projects[i].Tasks
		if len(tasks) == 0 {
			var truncated bool
			var err error
			tasks, truncated, err = s.listProjectTasksForEnrichment(ctx, wsID, views[i].ID)
			if err != nil {
				meta["task_enrichment_error"] = err.Error()
				continue
			}
			if truncated {
				meta["tasks_truncated"] = true
			}
		} else {
			meta["tasks_source"] = "hydrated_project"
		}
		project := projects[i]
		taskViews := make([]TaskView, 0, len(tasks))
		for _, task := range tasks {
			tv := taskViewFromTask(task, &project)
			taskViews = append(taskViews, tv)
			views[i].Rates.Tasks = append(views[i].Rates.Tasks, tv.Rates)
			if task.ID != "" {
				taskIDs[task.ID] = struct{}{}
			}
		}
		views[i].Tasks = taskViews
	}

	projectTotals, taskTotals, reportErr := s.reportFinancialsForProjects(ctx, wsID, projectIDs, keysOfSet(taskIDs), finRange)
	reportMatched := 0
	for i := range views {
		if total, ok := projectTotals[views[i].ID]; ok && (total.money.hasMoney() || total.hasDuration) {
			if total.money.hasMoney() {
				views[i].Financials = projectFinancialsFromReport(total.money, finRange)
				reportMatched++
			}
			views[i].EstimateProgress = progressFromEstimates(views[i].Estimates, total.durationSeconds, total.hasDuration)
		} else {
			views[i].Financials = derivedProjectFinancials(projects[i], views[i].Estimates, finRange)
		}
		for j := range views[i].Tasks {
			task := views[i].Tasks[j]
			if total, ok := taskTotals[task.ID]; ok && (total.money.hasMoney() || total.hasDuration) {
				if total.money.hasMoney() {
					views[i].Tasks[j].Financials = projectFinancialsFromReport(total.money, finRange)
				}
				views[i].Tasks[j].EstimateProgress = progressFromEstimates(task.Estimates, total.durationSeconds, total.hasDuration)
			} else {
				views[i].Tasks[j].Financials = derivedTaskFinancials(views[i].Tasks[j], &projects[i], finRange)
			}
		}
	}
	meta["reports_api_matched"] = reportMatched
	if reportMatched > 0 {
		meta["source"] = entryFinancialSourceReportsAPI
	}
	if reportErr != nil {
		meta["reports_api_error"] = reportErr.Error()
	}
	return views, meta
}

func (s *Service) enrichTaskView(ctx context.Context, wsID string, projectID string, task clockify.Task, args map[string]any) (TaskView, map[string]any) {
	views, meta := s.enrichTaskViews(ctx, wsID, projectID, []clockify.Task{task}, args)
	if len(views) == 0 {
		return taskViewFromTask(task, nil), meta
	}
	return views[0], meta
}

func (s *Service) enrichTaskViews(ctx context.Context, wsID string, projectID string, tasks []clockify.Task, args map[string]any) ([]TaskView, map[string]any) {
	var project *clockify.Project
	if s.projectDeepEnrichmentEnabled() && wsID != "" && projectID != "" {
		if path, err := paths.Workspace(wsID, "projects", projectID); err == nil {
			query := map[string]string{"hydrated": "true"}
			var out clockify.Project
			if err := s.Client.Get(ctx, path, query, &out); err == nil {
				project = &out
			}
		}
	}
	views := make([]TaskView, len(tasks))
	taskIDs := make([]string, 0, len(tasks))
	for i := range tasks {
		views[i] = taskViewFromTask(tasks[i], project)
		if tasks[i].ID != "" {
			taskIDs = append(taskIDs, tasks[i].ID)
		}
	}
	meta := map[string]any{
		"tasks":  len(tasks),
		"source": entryFinancialSourceUnavailable,
	}
	if len(tasks) == 0 {
		return views, meta
	}
	finRange := financialRangeFromArgs(args)
	for i := range views {
		views[i].Financials.Range = &finRange
	}
	if !s.projectDeepEnrichmentEnabled() {
		meta["reason"] = "deep task enrichment disabled for non-canonical Clockify base URL"
		return views, meta
	}
	_, taskTotals, reportErr := s.reportFinancialsForProjects(ctx, wsID, []string{projectID}, taskIDs, finRange)
	reportMatched := 0
	for i := range views {
		if total, ok := taskTotals[views[i].ID]; ok && (total.money.hasMoney() || total.hasDuration) {
			if total.money.hasMoney() {
				views[i].Financials = projectFinancialsFromReport(total.money, finRange)
				reportMatched++
			}
			views[i].EstimateProgress = progressFromEstimates(views[i].Estimates, total.durationSeconds, total.hasDuration)
			continue
		}
		views[i].Financials = derivedTaskFinancials(views[i], project, finRange)
	}
	meta["reports_api_matched"] = reportMatched
	if reportMatched > 0 {
		meta["source"] = entryFinancialSourceReportsAPI
	}
	if reportErr != nil {
		meta["reports_api_error"] = reportErr.Error()
	}
	return views, meta
}

func (s *Service) projectDeepEnrichmentEnabled() bool {
	return s != nil && s.Client != nil && s.entryFinancialReportsEnabled()
}

func (s *Service) listProjectTasksForEnrichment(ctx context.Context, wsID, projectID string) ([]clockify.Task, bool, error) {
	if wsID == "" || projectID == "" {
		return nil, false, nil
	}
	path, err := paths.Workspace(wsID, "projects", projectID, "tasks")
	if err != nil {
		return nil, false, err
	}
	values := url.Values{}
	values.Set("page", "1")
	values.Set("page-size", strconv.Itoa(projectTaskEnrichmentPageSize))
	var out []clockify.Task
	if err := s.Client.GetValues(ctx, path, values, &out); err != nil {
		return nil, false, err
	}
	return out, len(out) >= projectTaskEnrichmentPageSize, nil
}

func (s *Service) reportFinancialsForProjects(ctx context.Context, wsID string, projectIDs, taskIDs []string, finRange FinancialRangeView) (map[string]reportGroupMoney, map[string]reportGroupMoney, error) {
	projectTotals := map[string]reportGroupMoney{}
	taskTotals := map[string]reportGroupMoney{}
	if s == nil || s.Client == nil || !s.projectDeepEnrichmentEnabled() {
		return projectTotals, taskTotals, fmt.Errorf("reports API enrichment disabled for non-canonical Clockify base URL")
	}
	if len(projectIDs) == 0 {
		return projectTotals, taskTotals, nil
	}
	path, err := paths.Workspace(wsID, "reports", "summary")
	if err != nil {
		return projectTotals, taskTotals, err
	}
	body := map[string]any{
		"amountShown":   "EARNED",
		"amounts":       []string{"EARNED", "COST", "PROFIT"},
		"dateRangeType": firstNonEmptyString(finRange.DateRangeType, "ABSOLUTE"),
		"projects": map[string]any{
			"contains": "CONTAINS",
			"ids":      projectIDs,
			"status":   "ALL",
		},
		"summaryFilter": map[string]any{
			"groups": []string{"CLIENT", "PROJECT", "TASK"},
		},
	}
	if finRange.Start != "" {
		body["dateRangeStart"] = finRange.Start
	}
	if finRange.End != "" {
		body["dateRangeEnd"] = finRange.End
	}
	if finRange.Timezone != "" {
		body["timeZone"] = finRange.Timezone
	}
	var payload map[string]any
	if err := s.Client.PostReports(ctx, path, body, &payload); err != nil {
		return projectTotals, taskTotals, err
	}
	projectSet := stringSet(projectIDs)
	taskSet := stringSet(taskIDs)
	collectReportGroupMoney(payload, projectSet, taskSet, projectTotals, taskTotals)
	return projectTotals, taskTotals, nil
}

func collectReportGroupMoney(v any, projectIDs, taskIDs map[string]struct{}, projectTotals, taskTotals map[string]reportGroupMoney) {
	switch item := v.(type) {
	case []any:
		for _, child := range item {
			collectReportGroupMoney(child, projectIDs, taskIDs, projectTotals, taskTotals)
		}
	case []map[string]any:
		for _, child := range item {
			collectReportGroupMoney(child, projectIDs, taskIDs, projectTotals, taskTotals)
		}
	case map[string]any:
		if id := reportGroupID(item, "project"); id != "" {
			if _, ok := projectIDs[id]; ok {
				projectTotals[id] = mergeReportGroupMoney(projectTotals[id], item)
			}
		}
		if id := reportGroupID(item, "task"); id != "" {
			if _, ok := taskIDs[id]; ok {
				taskTotals[id] = mergeReportGroupMoney(taskTotals[id], item)
			}
		}
		for _, child := range item {
			collectReportGroupMoney(child, projectIDs, taskIDs, projectTotals, taskTotals)
		}
	}
}

func reportGroupID(row map[string]any, kind string) string {
	keys := []string{"id", "_id", kind + "Id", kind + "_id"}
	for _, key := range keys {
		if id := cleanReportID(row[key]); id != "" {
			return id
		}
	}
	if nested, ok := row[kind].(map[string]any); ok {
		for _, key := range []string{"id", "_id"} {
			if id := cleanReportID(nested[key]); id != "" {
				return id
			}
		}
	}
	return ""
}

func cleanReportID(v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func mergeReportGroupMoney(existing reportGroupMoney, row map[string]any) reportGroupMoney {
	money := reportMoneyFromRow(row)
	if money.earned != nil {
		existing.money.earned = money.earned
	}
	if money.cost != nil {
		existing.money.cost = money.cost
	}
	if money.profit != nil {
		existing.money.profit = money.profit
	}
	if seconds, ok := reportDurationSeconds(row); ok {
		existing.durationSeconds = seconds
		existing.hasDuration = true
	}
	return existing
}

func reportDurationSeconds(row map[string]any) (int64, bool) {
	for _, key := range []string{"duration", "totalTime", "totalDuration", "total_time"} {
		value, ok := row[key]
		if !ok {
			continue
		}
		if s, ok := value.(string); ok {
			return clockifyDurationSeconds(s)
		}
		if n, ok := reportNumber(value); ok {
			return int64(math.Round(n)), true
		}
	}
	return 0, false
}

func projectFinancialsFromReport(money reportEntryMoney, finRange FinancialRangeView) ProjectFinancials {
	fin := ProjectFinancials{
		Earned: money.earned,
		Cost:   money.cost,
		Profit: money.profit,
		Source: entryFinancialSourceReportsAPI,
		Reason: money.reason,
		Range:  &finRange,
	}
	if fin.Profit == nil {
		fin.Profit, fin.Reason = profitMoney(fin.Earned, fin.Cost, fin.Reason)
	}
	return fin
}

func derivedProjectFinancials(project clockify.Project, estimates ProjectEstimateView, finRange FinancialRangeView) ProjectFinancials {
	duration := int64(0)
	if estimates.DurationSeconds != nil {
		duration = *estimates.DurationSeconds
	}
	return derivedFinancialsFromRates(project.HourlyRate, project.CostRate, project.Billable, duration, finRange)
}

func derivedTaskFinancials(task TaskView, project *clockify.Project, finRange FinancialRangeView) ProjectFinancials {
	hourly := task.HourlyRate
	cost := task.CostRate
	if hourly == nil && project != nil {
		hourly = project.HourlyRate
	}
	if cost == nil && project != nil {
		cost = project.CostRate
	}
	duration := int64(0)
	if task.Estimates.DurationSeconds != nil {
		duration = *task.Estimates.DurationSeconds
	}
	return derivedFinancialsFromRates(hourly, cost, task.Billable, duration, finRange)
}

func derivedFinancialsFromRates(hourly, costRate *clockify.Rate, billable bool, durationSeconds int64, finRange FinancialRangeView) ProjectFinancials {
	eff := EffectiveRate{Hourly: hourly, Cost: costRate}
	earned, cost := DerivedEarnings(eff, durationSeconds)
	var earnedPtr *MoneyView
	if billable {
		earnedPtr = moneyPtr(earned)
	} else if hourly != nil {
		zero := MoneyFromCents(0, hourly.Currency)
		earnedPtr = &zero
	}
	costPtr := moneyPtr(cost)
	if earnedPtr == nil && costPtr == nil {
		return ProjectFinancials{
			Source: entryFinancialSourceUnavailable,
			Reason: "no reports API amount row and no usable duration/rate fallback",
			Range:  &finRange,
		}
	}
	reason := ""
	profit, reason := profitMoney(earnedPtr, costPtr, reason)
	if reason == "" && !billable {
		reason = "earned amount is zero because the project or task is not marked billable"
	}
	return ProjectFinancials{
		Earned: earnedPtr,
		Cost:   costPtr,
		Profit: profit,
		Source: entryFinancialSourceDerivedRates,
		Reason: reason,
		Range:  &finRange,
	}
}

func financialRangeFromArgs(args map[string]any) FinancialRangeView {
	start := strings.TrimSpace(stringArg(args, "financial_start"))
	end := strings.TrimSpace(stringArg(args, "financial_end"))
	dateRangeType := strings.ToUpper(strings.TrimSpace(stringArg(args, "financial_date_range_type")))
	timezone := strings.TrimSpace(stringArg(args, "financial_timezone"))
	if start == "" && end == "" && dateRangeType == "" {
		now := time.Now().UTC()
		start = now.AddDate(-3, 0, 0).Format(time.RFC3339)
		end = now.Format(time.RFC3339)
		dateRangeType = "ABSOLUTE"
	}
	return FinancialRangeView{
		Start:         start,
		End:           end,
		DateRangeType: dateRangeType,
		Timezone:      timezone,
	}
}

func projectCurrency(project *clockify.Project) string {
	if project == nil {
		return ""
	}
	if nested, ok := project.Currency.(map[string]any); ok {
		if s := firstNonEmptyString(reportValueString(nested["code"]), reportValueString(nested["id"])); s != "" {
			return s
		}
	}
	if s := reportValueString(project.Currency); s != "" {
		return s
	}
	if project.HourlyRate != nil && project.HourlyRate.Currency != "" {
		return project.HourlyRate.Currency
	}
	if project.CostRate != nil && project.CostRate.Currency != "" {
		return project.CostRate.Currency
	}
	for _, membership := range project.Memberships {
		if membership.HourlyRate != nil && membership.HourlyRate.Currency != "" {
			return membership.HourlyRate.Currency
		}
		if membership.CostRate != nil && membership.CostRate.Currency != "" {
			return membership.CostRate.Currency
		}
	}
	return ""
}

func projectRawHydratedMap(project clockify.Project) map[string]any {
	raw := map[string]any{}
	if project.Currency != nil {
		raw["currency"] = project.Currency
	}
	if project.Client != nil {
		raw["client"] = project.Client
	}
	if project.CustomFields != nil {
		raw["customFields"] = project.CustomFields
	}
	if project.Expenses != nil {
		raw["expenses"] = project.Expenses
	}
	if project.Favorite {
		raw["favorite"] = project.Favorite
	}
	if len(project.Tasks) > 0 {
		raw["tasks"] = project.Tasks
	}
	if len(raw) == 0 {
		return nil
	}
	return maps.Clone(raw)
}

func projectExpensesSummary(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	out := map[string]any{"raw": raw}
	rows := mapSlice(raw)
	if len(rows) > 0 {
		out["count"] = len(rows)
		var total *MoneyView
		for _, row := range rows {
			total = moneySum(total, moneyFromAny(firstPresent(row, "amount", "total", "value"), reportCurrency(row)))
		}
		if total != nil {
			out["amount"] = total
		}
		return out
	}
	if m, ok := raw.(map[string]any); ok {
		if count, ok := reportInt(firstPresent(m, "count", "total")); ok {
			out["count"] = count
		}
		if money := moneyFromAny(firstPresent(m, "amount", "total", "value"), reportCurrency(m)); money != nil {
			out["amount"] = money
		}
	}
	return out
}

func estimateSecondsFromRaw(v any) (int64, bool) {
	switch raw := v.(type) {
	case string:
		return clockifyDurationSeconds(raw)
	case map[string]any:
		for _, key := range []string{"estimate", "duration", "value"} {
			if seconds, ok := estimateSecondsFromRaw(raw[key]); ok {
				return seconds, true
			}
		}
	}
	return 0, false
}

func budgetEstimateFromRaw(v any, currency string) (MoneyView, bool) {
	switch raw := v.(type) {
	case int64:
		return MoneyFromCents(raw, currency), raw > 0
	case int:
		return MoneyFromCents(int64(raw), currency), raw > 0
	case float64:
		if raw > 0 {
			return MoneyFromCents(int64(math.Round(raw)), currency), true
		}
	case map[string]any:
		for _, key := range []string{"estimate", "amount", "value"} {
			if money, ok := budgetEstimateFromRaw(raw[key], currency); ok {
				return money, true
			}
		}
	}
	return MoneyView{}, false
}

func clockifyDurationSeconds(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return int64(math.Round(float64(n) / 1000.0)), true
	}
	if strings.HasPrefix(raw, "PT") || strings.HasPrefix(raw, "P") {
		if seconds, ok := parseISODurationSeconds(raw); ok {
			return seconds, true
		}
	}
	if d, err := time.ParseDuration(strings.ToLower(raw)); err == nil {
		return int64(d.Seconds()), true
	}
	return 0, false
}

func parseISODurationSeconds(raw string) (int64, bool) {
	s := strings.ToUpper(strings.TrimPrefix(raw, "P"))
	inTime := false
	var num strings.Builder
	total := int64(0)
	flush := func(unit rune) bool {
		if num.Len() == 0 {
			return false
		}
		value, err := strconv.ParseFloat(num.String(), 64)
		if err != nil {
			return false
		}
		num.Reset()
		switch unit {
		case 'D':
			total += int64(math.Round(value * 86400))
		case 'H':
			total += int64(math.Round(value * 3600))
		case 'M':
			if inTime {
				total += int64(math.Round(value * 60))
			}
		case 'S':
			total += int64(math.Round(value))
		default:
			return false
		}
		return true
	}
	for _, r := range s {
		switch {
		case r == 'T':
			inTime = true
		case (r >= '0' && r <= '9') || r == '.':
			num.WriteRune(r)
		default:
			if !flush(r) {
				return 0, false
			}
		}
	}
	return total, true
}

func keysOfSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return out
}

func stringSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			out[item] = struct{}{}
		}
	}
	return out
}
