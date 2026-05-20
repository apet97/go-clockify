package tools

import (
	"context"
	"time"
)

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
