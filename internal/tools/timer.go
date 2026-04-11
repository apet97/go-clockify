package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/resolve"
)

func (s *Service) StartTimer(ctx context.Context, projectID, projectRef, description string) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if projectID == "" && projectRef != "" {
		projectID, err = resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
	}
	payload := map[string]any{"start": time.Now().UTC().Format(time.RFC3339), "description": description}
	if projectID != "" {
		payload["projectId"] = projectID
	}
	var out clockify.TimeEntry
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/time-entries", payload, &out); err != nil {
		return ResultEnvelope{}, err
	}
	meta := map[string]any{"workspaceId": wsID}
	if projectID != "" {
		meta["projectId"] = projectID
	}
	s.emitResourceUpdate(ctx, entryResourceURI(wsID, out.ID))
	return ok("clockify_start_timer", out, meta), nil
}

func (s *Service) StopTimer(ctx context.Context, args map[string]any) (any, error) {
	if dryrun.Enabled(args) {
		return dryrun.Preview("clockify_stop_timer", args), nil
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
	var out clockify.TimeEntry
	if err := s.Client.Patch(ctx, "/workspaces/"+wsID+"/user/"+user.ID+"/time-entries", payload, &out); err != nil {
		return nil, err
	}
	s.emitResourceUpdate(ctx, entryResourceURI(wsID, out.ID))
	return ok("clockify_stop_timer", out, map[string]any{"workspaceId": wsID, "userId": user.ID}), nil
}

func (s *Service) TimerStatus(ctx context.Context) (ResultEnvelope, error) {
	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, map[string]string{"page-size": "1"})
	if err != nil {
		return ResultEnvelope{}, err
	}
	meta := map[string]any{"workspaceId": wsID, "userId": userID}

	if len(entries) == 0 || !entries[0].IsRunning() {
		return ok("clockify_timer_status", map[string]any{
			"running": false,
			"entry":   nil,
			"elapsed": "",
		}, meta), nil
	}

	entry := entries[0]
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

	return ok("clockify_timer_status", map[string]any{
		"running": true,
		"entry":   entry,
		"elapsed": elapsedStr,
	}, meta), nil
}
