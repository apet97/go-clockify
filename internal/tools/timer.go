package tools

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
)

func (s *Service) StartTimer(ctx context.Context, projectID, projectRef, description string) (ResultEnvelope, error) {
	return s.startTimer(ctx, map[string]any{
		"project_id":  projectID,
		"project":     projectRef,
		"description": description,
	})
}

func (s *Service) StartTimerArgs(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.startTimer(ctx, args)
}

func (s *Service) startTimer(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID := stringArg(args, "project_id")
	projectRef := stringArg(args, "project")
	if projectID == "" && projectRef != "" {
		projectID, err = s.resolveProjectID(ctx, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
	}
	payload := map[string]any{"start": time.Now().UTC().Format(time.RFC3339), "description": stringArg(args, "description")}
	if projectID != "" {
		payload["projectId"] = projectID
	}
	if entryType := stringArg(args, "type"); entryType != "" {
		payload["type"] = entryType
	}
	meta := map[string]any{"workspaceId": wsID}
	if projectID != "" {
		meta["projectId"] = projectID
	}
	if dryrun.Enabled(args) {
		running, _, userID, err := s.listEntriesWithQuery(ctx, map[string]string{"in-progress": "true", "page-size": "1"})
		if err != nil {
			return ResultEnvelope{}, err
		}
		if userID != "" {
			meta["userId"] = userID
		}
		validation := validationOK("timer_state_check")
		preview := dryrunPreviewPayloadValidated(oneUserToolEntriesTimerStart, payload, validation)
		if len(running) > 0 && running[0].IsRunning() {
			validation = validationFailed("timer_state_check", ValidationProblem{
				Code:        "running_timer_exists",
				Message:     "a timer is already running",
				Remediation: "Stop the current timer before starting another timer.",
			})
			preview = dryrunPreviewPayloadValidated(oneUserToolEntriesTimerStart, payload, validation)
			view, financialMeta := s.enrichEntryView(ctx, wsID, running[0])
			preview["running_entry"] = view
			meta = withFinancialMeta(meta, financialMeta)
		}
		preview["args"] = maps.Clone(args)
		return ok(oneUserToolEntriesTimerStart, preview, meta), nil
	}
	// C1: never create a timer that cannot later be stopped.
	if running, _, _, runErr := s.listEntriesWithQuery(ctx, map[string]string{"in-progress": "true", "page-size": "1"}); runErr == nil && len(running) > 0 && running[0].IsRunning() {
		return ResultEnvelope{}, fmt.Errorf("a timer is already running (entry %s) — stop it first with clockify_entries_timer_stop", running[0].ID)
	}
	if projectID == "" && s.workspaceForcesProjects(ctx, wsID) {
		return ResultEnvelope{}, fmt.Errorf("this workspace requires a project on every time entry — pass project_id or project")
	}
	path, err := paths.Workspace(wsID, "time-entries")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out clockify.TimeEntry
	if err := s.Client.Post(ctx, path, payload, &out); err != nil {
		return ResultEnvelope{}, err
	}
	s.emitEntryAndWeeklyWithState(ctx, wsID, out)
	view, financialMeta := s.enrichEntryView(ctx, wsID, out)
	return ok(oneUserToolEntriesTimerStart, view, withFinancialMeta(meta, financialMeta)), nil
}

func (s *Service) StopTimer(ctx context.Context, args map[string]any) (any, error) {
	if dryrun.Enabled(args) {
		entries, wsID, userID, err := s.listEntriesWithQuery(ctx, map[string]string{"in-progress": "true", "page-size": "1"})
		if err != nil {
			return nil, err
		}
		meta := map[string]any{"workspaceId": wsID, "userId": userID}
		payload := map[string]any{"end": time.Now().UTC().Format(time.RFC3339)}
		if len(entries) == 0 || !entries[0].IsRunning() {
			preview := dryrunPreviewPayloadValidated("clockify_entries_timer_stop", payload, validationFailed("timer_state_check", ValidationProblem{
				Code:        "no_running_timer",
				Message:     "there is no running timer to stop",
				Remediation: "Use clockify_entries_timer_status to confirm timer state before retrying.",
			}))
			preview["args"] = maps.Clone(args)
			return ok("clockify_entries_timer_stop", preview, meta), nil
		}
		view, financialMeta := s.enrichEntryView(ctx, wsID, entries[0])
		meta = withFinancialMeta(meta, financialMeta)
		preview := dryrunPreviewPayloadValidated("clockify_entries_timer_stop", payload, validationOK("timer_state_check"))
		preview["args"] = maps.Clone(args)
		preview["running_entry"] = view
		return ok("clockify_entries_timer_stop", preview, meta), nil
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"end": time.Now().UTC().Format(time.RFC3339)}
	path, err := paths.Workspace(wsID, "user", user.ID, "time-entries")
	if err != nil {
		return nil, err
	}
	var out clockify.TimeEntry
	if err := s.Client.Patch(ctx, path, payload, &out); err != nil {
		if strings.Contains(err.Error(), "Project is either required") {
			return nil, fmt.Errorf("the running timer has no project and this workspace requires one — assign a project with clockify_entries_update, then retry clockify_entries_timer_stop (upstream: %w)", err)
		}
		return nil, err
	}
	if out.ID == "" {
		return nil, fmt.Errorf("no running timer: no stopped entry returned by Clockify")
	}
	s.emitEntryAndWeeklyWithState(ctx, wsID, out)
	view, financialMeta := s.enrichEntryView(ctx, wsID, out)
	return ok("clockify_entries_timer_stop", view, withFinancialMeta(map[string]any{"workspaceId": wsID, "userId": user.ID}, financialMeta)), nil
}

func (s *Service) TimerStatus(ctx context.Context) (ResultEnvelope, error) {
	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, map[string]string{"in-progress": "true"})
	if err != nil {
		return ResultEnvelope{}, err
	}
	meta := map[string]any{"workspaceId": wsID, "userId": userID}

	if len(entries) == 0 || !entries[0].IsRunning() {
		return ok(oneUserToolEntriesTimerStatus, map[string]any{
			"running": false,
			"entry":   nil,
			"elapsed": "",
		}, meta), nil
	}

	entry := entries[0]
	view, financialMeta := s.enrichEntryView(ctx, wsID, entry)
	meta = withFinancialMeta(meta, financialMeta)
	startTime, err := entry.StartTime()
	if err != nil {
		return ResultEnvelope{}, fmt.Errorf("parse start time: %w", err)
	}
	elapsed := time.Since(startTime)
	var elapsedStr string
	if elapsed >= time.Hour {
		elapsedStr = fmt.Sprintf("%dh %dm", int(elapsed.Hours()), int(elapsed.Minutes())%60)
	} else {
		elapsedStr = fmt.Sprintf("%dm %ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
	}

	return ok(oneUserToolEntriesTimerStatus, map[string]any{
		"running": true,
		"entry":   view,
		"elapsed": elapsedStr,
	}, meta), nil
}

// workspaceForcesProjects reports whether the workspace requires a
// project on every time entry (workspaceSettings.forceProjects). On any
// fetch error it returns false — never block a write on a settings read.
func (s *Service) workspaceForcesProjects(ctx context.Context, wsID string) bool {
	ws, err := s.getWorkspace(ctx, wsID)
	if err != nil {
		return false
	}
	return boolFromAny(workspaceSettingAny(ws.WorkspaceSettings, "forceProjects", "force_projects"))
}
