package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/resolve"
	"github.com/apet97/go-clockify/internal/timeparse"
)

func (s *Service) ClockifyLogWork(ctx context.Context, args map[string]any) (any, error) {
	if strings.TrimSpace(stringArg(args, "end")) == "" {
		return nil, fmt.Errorf("end is required for clockify_log_work; use clockify_start_work for a running timer")
	}
	if err := s.rejectLogWorkOverlap(ctx, args); err != nil {
		return nil, err
	}
	warnings := s.futureEntryWarningsFromArgs(args, time.Now().UTC())
	entry, ids, err := s.createEntry(ctx, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_log_work", "entry", ids, entry, ChangeSet{Created: []EntityRef{entryRef(entry)}}, warnings, []NextAction{
		{Tool: "clockify_review_day", Args: reviewDayArgsFromEntry(entry), Reason: "Review the day after logging work."},
		{Tool: "clockify_fix_entry", Args: map[string]any{"entry_id": entry.ID}, Reason: "Adjust this entry if any details are wrong."},
	}), nil
}

// rejectLogWorkOverlap fails closed when a clockify_log_work entry overlaps an
// existing entry, unless allow_overlap is set. logWorkSchema advertises
// allow_overlap; this is where the handler honors it.
func (s *Service) rejectLogWorkOverlap(ctx context.Context, args map[string]any) error {
	if boolArg(args, "allow_overlap") {
		return nil
	}
	loc := s.location()
	startRaw := strings.TrimSpace(stringArg(args, "start"))
	start, err := timeparse.ParseDatetime(startRaw, loc)
	if err != nil {
		return fmt.Errorf("could not parse date %q for start — use YYYY-MM-DD or RFC3339", startRaw)
	}
	endRaw := strings.TrimSpace(stringArg(args, "end"))
	end, err := timeparse.ParseDatetime(endRaw, loc)
	if err != nil {
		return fmt.Errorf("could not parse date %q for end — use YYYY-MM-DD or RFC3339", endRaw)
	}
	overlaps, _, err := s.findEntryOverlaps(ctx, start, end)
	if err != nil {
		return err
	}
	if len(overlaps) > 0 {
		details := make([]string, 0, len(overlaps))
		for _, o := range overlaps {
			proj := o.ProjectName
			if proj == "" {
				proj = o.ProjectID
			}
			if proj == "" {
				proj = "no project"
			}
			details = append(details, fmt.Sprintf("entry %s [%s..%s, %s]", o.ID, o.Start, o.End, proj))
		}
		return fmt.Errorf("requested entry overlaps %d existing entr%s (%s); pass allow_overlap=true only after manual review", len(overlaps), pluralY(len(overlaps)), strings.Join(details, "; "))
	}
	return nil
}

func (s *Service) ClockifyStartWork(ctx context.Context, args map[string]any) (any, error) {
	startArgs, startDefaulted, err := s.prepareStartWorkArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	entry, ids, err := s.createEntry(ctx, startArgs)
	if err != nil {
		return nil, err
	}
	meta := map[string]any{}
	if startDefaulted {
		meta["startWasDefaulted"] = true
		meta["resolvedStart"] = startArgs["start"]
	}
	return result("clockify_start_work", "entry", ids, entry, ChangeSet{Created: []EntityRef{entryRef(entry)}}, nil, []NextAction{
		{Tool: "clockify_stop_work", Reason: "Stop this timer when the work session is finished."},
		{Tool: "clockify_switch_work", Reason: "Switch to another project/task without manually stopping first."},
	}, meta), nil
}

func (s *Service) ClockifyStopWork(ctx context.Context, args map[string]any) (any, error) {
	out, err := s.StopTimer(ctx, args)
	if err != nil {
		return nil, err
	}
	resultOut := standardizeDomainResult("clockify_stop_work", "entry", "updated", out, args)
	if env, ok := out.(ResultEnvelope); ok {
		if data, ok := env.Data.(map[string]any); ok && !boolFromAny(data["stopped"]) && strings.TrimSpace(reportValueString(data["reason"])) == "no timer running" {
			resultOut.Changed = ChangeSet{}
			resultOut.IDs = cleanIDs(map[string]string{
				"workspaceId": stringFromAny(env.Meta["workspaceId"]),
				"userId":      stringFromAny(env.Meta["userId"]),
			})
		}
		if entry, ok := env.Data.(clockify.TimeEntry); ok {
			resultOut.IDs = cleanIDs(map[string]string{
				"workspaceId": stringFromAny(env.Meta["workspaceId"]),
				"userId":      stringFromAny(env.Meta["userId"]),
				"entryId":     entry.ID,
				"projectId":   entry.ProjectID,
				"taskId":      entry.TaskID,
			})
			resultOut.Changed = ChangeSet{Updated: []EntityRef{entryRef(entry)}}
		}
		if entry, ok := env.Data.(EntryView); ok {
			resultOut.IDs = cleanIDs(map[string]string{
				"workspaceId": firstNonEmptyString(entry.WorkspaceID, stringFromAny(env.Meta["workspaceId"])),
				"userId":      firstNonEmptyString(entry.UserID, stringFromAny(env.Meta["userId"])),
				"entryId":     entry.ID,
				"projectId":   entry.ProjectID,
				"taskId":      entry.TaskID,
			})
			resultOut.Changed = ChangeSet{Updated: []EntityRef{{Type: "entry", ID: entry.ID, Name: entry.Description}}}
		}
	}
	resultOut.Next = []NextAction{{Tool: "clockify_review_day", Reason: "Review the day after stopping work."}}
	return resultOut, nil
}

func summarizeToolResult(r any) map[string]any {
	if r == nil {
		return nil
	}
	tr, ok := r.(ToolResult)
	if !ok {
		return map[string]any{"raw": r}
	}
	return map[string]any{
		"ok":     tr.OK,
		"action": tr.Action,
		"ids":    tr.IDs,
	}
}

func (s *Service) ClockifySwitchWork(ctx context.Context, args map[string]any) (any, error) {
	warnings := []Warning{}
	startArgs, startDefaulted, err := s.prepareStartWorkArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	// meta echoes a defaulted start so the model never has to guess whether
	// the resolved start time came from the caller or from time.Now().
	meta := map[string]any{}
	if startDefaulted {
		meta["startWasDefaulted"] = true
		meta["resolvedStart"] = startArgs["start"]
	}
	stopped, stopErr := s.ClockifyStopWork(ctx, map[string]any{})
	if stopErr != nil {
		if !isNoRunningTimer(stopErr) {
			return nil, fmt.Errorf("stop current work: %w", stopErr)
		}
		warnings = append(warnings, Warning{Code: "no_running_timer", Message: "No running timer was found; started the new work item."})
	}
	started, err := s.ClockifyStartWork(ctx, startArgs)
	if err != nil {
		if stopped != nil {
			warnings = append(warnings, Warning{Code: "partial_failure", Message: "Stopped the previous timer but could not start the new one."})
		}
		return result("clockify_switch_work", "entry", map[string]string{"workspaceId": s.WorkspaceID}, map[string]any{
			"status":  "partial_failure",
			"stopped": stopped,
			"error":   err.Error(),
		}, ChangeSet{Updated: refsFromToolResult(stopped)}, warnings, []NextAction{
			{Tool: "clockify_start_work", Args: startArgs, Reason: "Retry starting the target timer after fixing the error."},
		}, meta), nil
	}
	startResult, _ := started.(ToolResult)
	ids := map[string]string{"workspaceId": s.WorkspaceID}
	for k, v := range startResult.IDs {
		ids[k] = v
	}
	return result("clockify_switch_work", "entry", ids, map[string]any{
		"status":  "ok",
		"stopped": summarizeToolResult(stopped),
		"started": summarizeToolResult(started),
	}, ChangeSet{Created: startResult.Changed.Created, Updated: refsFromToolResult(stopped)}, warnings, []NextAction{
		{Tool: "clockify_stop_work", Reason: "Stop the newly started timer when finished."},
	}, meta), nil
}

// prepareStartWorkArgs resolves caller args into the shape createEntry
// expects. The returned bool reports whether `start` was absent and
// defaulted to now, so the caller can echo the resolved value (Axiom 22).
func (s *Service) prepareStartWorkArgs(ctx context.Context, args map[string]any) (map[string]any, bool, error) {
	startArgs := copyArgs(args)
	startDefaulted := false
	if strings.TrimSpace(stringArg(startArgs, "start")) == "" {
		startArgs["start"] = time.Now().UTC().Format(time.RFC3339)
		startDefaulted = true
	} else if raw := stringArg(startArgs, "start"); raw != "" {
		if _, err := timeparse.ParseDatetime(raw, s.location()); err != nil {
			return nil, false, fmt.Errorf("could not parse date %q for start — use YYYY-MM-DD or RFC3339", raw)
		}
	}
	delete(startArgs, "end")

	projectID := strings.TrimSpace(stringArg(startArgs, "project_id"))
	if projectID == "" {
		if project := strings.TrimSpace(stringArg(startArgs, "project")); project != "" {
			resolved, err := s.resolveProjectID(ctx, s.WorkspaceID, project)
			if err != nil {
				return nil, false, err
			}
			projectID = resolved
			startArgs["project_id"] = resolved
			delete(startArgs, "project")
		}
	} else if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return nil, false, err
	}

	if taskID := strings.TrimSpace(stringArg(startArgs, "task_id")); taskID != "" {
		if err := resolve.ValidateID(taskID, "task_id"); err != nil {
			return nil, false, err
		}
	} else if task := strings.TrimSpace(stringArg(startArgs, "task")); task != "" {
		if projectID == "" {
			return nil, false, fmt.Errorf("project_id or project is required when resolving task by name")
		}
		resolved, err := s.resolveTaskID(ctx, s.WorkspaceID, projectID, task)
		if err != nil {
			return nil, false, err
		}
		startArgs["task_id"] = resolved
		delete(startArgs, "task")
	}

	tagIDs, err := s.tagIDsFromArgs(ctx, startArgs)
	if err != nil {
		return nil, false, err
	}
	if len(tagIDs) > 0 {
		startArgs["tag_ids"] = tagIDs
		delete(startArgs, "tag")
	}

	return startArgs, startDefaulted, nil
}

func (s *Service) ClockifyFixEntry(ctx context.Context, args map[string]any) (any, error) {
	fixArgs := copyArgs(args)
	if v := strings.TrimSpace(stringArg(fixArgs, "description")); v != "" && strings.TrimSpace(stringArg(fixArgs, "new_description")) == "" {
		fixArgs["new_description"] = v
	}
	out, err := s.FindAndUpdateEntry(ctx, fixArgs)
	if err != nil {
		return nil, err
	}
	standard := standardizeDomainResult("clockify_fix_entry", "entry", "updated", out, fixArgs)
	if env, ok := out.(ResultEnvelope); ok {
		if data, ok := env.Data.(FindAndUpdateEntryData); ok {
			standard.IDs = cleanIDs(map[string]string{"workspaceId": stringFromAny(env.Meta["workspaceId"]), "entryId": data.Entry.ID, "projectId": data.Entry.ProjectID})
			ref := EntityRef{Type: "entry", ID: data.Entry.ID, Name: data.Entry.Description}
			if len(data.UpdatedFields) == 0 {
				standard.Changed = ChangeSet{Reused: []EntityRef{ref}}
			} else {
				standard.Changed = ChangeSet{Updated: []EntityRef{ref}}
			}
		}
	}
	standard.Next = []NextAction{{Tool: "clockify_review_day", Reason: "Review the affected day after fixing the entry."}}
	return standard, nil
}

func reviewDayArgsFromEntry(entry clockify.TimeEntry) map[string]any {
	start, err := entry.StartTime()
	if err != nil {
		return nil
	}
	return map[string]any{"date": start.Format("2006-01-02")}
}

func refsFromToolResult(value any) []EntityRef {
	if result, ok := value.(ToolResult); ok {
		return append([]EntityRef(nil), result.Changed.Updated...)
	}
	return nil
}

func isNoRunningTimer(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *clockify.APIError
	if strings.Contains(strings.ToLower(err.Error()), "no running") {
		return true
	}
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 400 || apiErr.StatusCode == 404
	}
	return false
}
