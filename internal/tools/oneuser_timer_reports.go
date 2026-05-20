package tools

import (
	"context"
	"strings"

	"github.com/apet97/go-clockify/internal/mcp"
)

func (s *Service) timerAndReportDescriptors() []mcp.ToolDescriptor {
	out := make([]mcp.ToolDescriptor, 0, 8)
	out = append(out,
		firstSliceDescriptor(66, toolRO("clockify_entries_running", "Return the current running timer, if any.", objectSchema(nil)), s.EntriesRunning),
		firstSliceDescriptor(67, toolRW("clockify_entries_timer_start", "Start a running time-entry timer for the current user.", objectSchema(map[string]any{"properties": map[string]any{
			"project_id":  map[string]any{"type": "string"},
			"project":     map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"billable":    map[string]any{"type": "boolean", "description": "Override the project's billable default. Omit to inherit the project setting."},
		}})), s.EntriesTimerStart),
		firstSliceDescriptor(68, toolRWIdem("clockify_entries_timer_stop", "Stop the current user's running time-entry timer.", objectSchema(map[string]any{"properties": map[string]any{
			"end": map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		}})), s.EntriesTimerStop),
		firstSliceDescriptor(69, toolRO("clockify_entries_timer_status", "Show whether the current user has a running timer.", objectSchema(nil)), s.EntriesTimerStatus),
		firstSliceDescriptor(70, toolRW("clockify_entries_timer_switch", "Switch the running timer to another project.", objectSchema(map[string]any{"required": []string{"project"}, "properties": map[string]any{
			"project":     map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"task_id":     map[string]any{"type": "string"},
			"billable":    map[string]any{"type": "boolean"},
		}})), s.EntriesTimerSwitch),
		firstSliceDescriptor(104, toolRO("clockify_reports_detailed", "Run the local detailed time report helper. Large results truncate to the size cap.", reportHelperSchema()), aliasHandler("clockify_reports_detailed", "report", "", s.DetailedReport)),
		firstSliceDescriptor(105, toolRO("clockify_reports_summary", "Run the local summary report helper. Large results truncate to the size cap.", reportHelperSchema()), aliasHandler("clockify_reports_summary", "report", "", s.SummaryReport)),
		firstSliceDescriptor(106, toolRO("clockify_reports_weekly", "Run the local weekly report helper. Range must be exactly 7 days; pass week_start (YYYY-MM-DD) alone to auto-derive the week end. Large results truncate to the size cap.", reportHelperSchema()), aliasHandler("clockify_reports_weekly", "report", "", s.WeeklySummary)),
	)
	return out
}

func reportHelperSchema() map[string]any {
	return objectSchema(map[string]any{"properties": map[string]any{
		"start":           map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"end":             map[string]any{"type": "string", "description": flexibleDatetimeDescription},
		"project":         map[string]any{"type": "string"},
		"include_entries": map[string]any{"type": "boolean"},
		"max_entries":     map[string]any{"type": "integer", "minimum": 0, "description": "Advisory only. The Clockify reports API returns a fixed page size; narrow start/end to get fewer rows. meta.truncated reports when the row cap was hit."},
		"max_rollups":     map[string]any{"type": "integer", "minimum": 0, "description": "Summary/weekly reports: keep only the N largest project rollups by tracked time (default 15). Aggregates still cover every rollup. Pass 0 for the full, unbounded list."},
		"week_start":      map[string]any{"type": "string", "description": "YYYY-MM-DD or RFC3339 week start."},
		"timezone":        map[string]any{"type": "string"},
	}})
}

func (s *Service) EntriesRunning(ctx context.Context, _ map[string]any) (any, error) {
	timer, err := s.currentTimer(ctx)
	if err != nil {
		return nil, err
	}
	return result("clockify_entries_running", "entry", map[string]string{"workspaceId": s.WorkspaceID}, timer, ChangeSet{}, nil, nil), nil
}

func (s *Service) EntriesTimerStart(ctx context.Context, args map[string]any) (any, error) {
	out, err := s.StartTimerArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return standardizeDomainResult("clockify_entries_timer_start", "entry", "created", out, args), nil
}

func (s *Service) EntriesTimerStop(ctx context.Context, args map[string]any) (any, error) {
	out, err := s.StopTimer(ctx, args)
	if err != nil {
		return nil, err
	}
	if env, ok := out.(ResultEnvelope); ok {
		if data, ok := env.Data.(map[string]any); ok && !boolFromAny(data["stopped"]) && strings.TrimSpace(reportValueString(data["reason"])) == "no timer running" {
			ids := map[string]string{}
			for key, value := range env.Meta {
				if str, ok := value.(string); ok && str != "" {
					ids[key] = str
				}
			}
			return result("clockify_entries_timer_stop", "entry", ids, data, ChangeSet{}, nil, nil, env.Meta), nil
		}
	}
	return standardizeDomainResult("clockify_entries_timer_stop", "entry", "updated", out, args), nil
}

func (s *Service) EntriesTimerStatus(ctx context.Context, _ map[string]any) (any, error) {
	out, err := s.TimerStatus(ctx)
	if err != nil {
		return nil, err
	}
	return standardizeDomainResult("clockify_entries_timer_status", "entry", "", out, nil), nil
}

func (s *Service) EntriesTimerSwitch(ctx context.Context, args map[string]any) (any, error) {
	out, err := s.SwitchProject(ctx, args)
	if err != nil {
		return nil, err
	}
	return standardizeDomainResult("clockify_entries_timer_switch", "entry", "updated", out, args), nil
}
