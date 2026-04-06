package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goclmcp/internal/clockify"
	"goclmcp/internal/dryrun"
	"goclmcp/internal/resolve"
)

func (s *Service) LogTime(ctx context.Context, args map[string]any) (any, error) {
	if dryrun.Enabled(args) {
		return dryrun.Preview("clockify_log_time", args), nil
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	start, end, err := parseStartEnd(args)
	if err != nil {
		return nil, err
	}
	projectID := stringArg(args, "project_id")
	projectRef := stringArg(args, "project")
	if projectID == "" && projectRef != "" {
		projectID, err = resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
		if err != nil {
			return nil, err
		}
	}
	payload := map[string]any{
		"start":       start.Format(time.RFC3339),
		"end":         end.Format(time.RFC3339),
		"description": stringArg(args, "description"),
	}
	if projectID != "" {
		payload["projectId"] = projectID
	}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	var out clockify.TimeEntry
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/time-entries", payload, &out); err != nil {
		return nil, err
	}
	return ok("clockify_log_time", LogTimeData{Entry: out, ResolvedProject: projectID}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) FindAndUpdateEntry(ctx context.Context, args map[string]any) (any, error) {
	parsed, err := parseFindAndUpdateArgs(args)
	if err != nil {
		return nil, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	entry, matchedBy, err := s.findSingleEntry(ctx, parsed)
	if err != nil {
		return nil, err
	}
	updatedFields := make([]string, 0, 4)
	if parsed.NewDescription != "" && parsed.NewDescription != entry.Description {
		entry.Description = parsed.NewDescription
		updatedFields = append(updatedFields, "description")
	}
	if parsed.ProjectID != "" || parsed.Project != "" {
		projectID := parsed.ProjectID
		if projectID == "" {
			projectID, err = resolve.ResolveProjectID(ctx, s.Client, wsID, parsed.Project)
			if err != nil {
				return nil, err
			}
		}
		if projectID != entry.ProjectID {
			entry.ProjectID = projectID
			updatedFields = append(updatedFields, "projectId")
		}
	}
	if parsed.Start != "" {
		if _, err := time.Parse(time.RFC3339, parsed.Start); err != nil {
			return nil, fmt.Errorf("start must be RFC3339: %w", err)
		}
		if entry.TimeInterval.Start != parsed.Start {
			entry.TimeInterval.Start = parsed.Start
			updatedFields = append(updatedFields, "start")
		}
	}
	if parsed.End != "" {
		if _, err := time.Parse(time.RFC3339, parsed.End); err != nil {
			return nil, fmt.Errorf("end must be RFC3339: %w", err)
		}
		if entry.TimeInterval.End != parsed.End {
			entry.TimeInterval.End = parsed.End
			updatedFields = append(updatedFields, "end")
		}
	}
	if parsed.Billable != nil && *parsed.Billable != entry.Billable {
		entry.Billable = *parsed.Billable
		updatedFields = append(updatedFields, "billable")
	}
	if len(updatedFields) == 0 {
		return ok("clockify_find_and_update_entry", FindAndUpdateEntryData{Entry: entry, MatchedBy: matchedBy, UpdatedFields: updatedFields}, map[string]any{"workspaceId": wsID, "noop": true}), nil
	}
	if parsed.DryRun {
		return dryrun.Preview("clockify_find_and_update_entry", args), nil
	}
	payload := map[string]any{
		"start":       entry.TimeInterval.Start,
		"description": entry.Description,
		"projectId":   entry.ProjectID,
		"billable":    entry.Billable,
	}
	if entry.TimeInterval.End != "" {
		payload["end"] = entry.TimeInterval.End
	}
	var out clockify.TimeEntry
	if err := s.Client.Put(ctx, "/workspaces/"+wsID+"/time-entries/"+entry.ID, payload, &out); err != nil {
		return nil, err
	}
	return ok("clockify_find_and_update_entry", FindAndUpdateEntryData{Entry: out, MatchedBy: matchedBy, UpdatedFields: updatedFields}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) SwitchProject(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectRef := stringArg(args, "project")
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	description := stringArg(args, "description")

	// Stop the current timer; ignore "no running timer" errors.
	var stoppedEntry any
	stopResult, stopErr := s.StopTimer(ctx, map[string]any{})
	if stopErr != nil {
		if !strings.Contains(stopErr.Error(), "404") && !strings.Contains(stopErr.Error(), "400") {
			return ResultEnvelope{}, fmt.Errorf("stop timer: %w", stopErr)
		}
		// No timer was running — proceed with start.
	} else {
		stoppedEntry = stopResult
	}

	// Start a new timer with the given project.
	startResult, startErr := s.StartTimer(ctx, "", projectRef, description)
	if startErr != nil {
		return ResultEnvelope{}, fmt.Errorf("start timer: %w", startErr)
	}

	return ok("clockify_switch_project", map[string]any{
		"stopped": stoppedEntry,
		"started": startResult.Data,
	}, startResult.Meta), nil
}

func parseFindAndUpdateArgs(args map[string]any) (findAndUpdateArgs, error) {
	out := findAndUpdateArgs{
		DescriptionContains: stringArg(args, "description_contains"),
		ExactDescription:    stringArg(args, "exact_description"),
		EntryID:             stringArg(args, "entry_id"),
		StartAfter:          stringArg(args, "start_after"),
		StartBefore:         stringArg(args, "start_before"),
		NewDescription:      stringArg(args, "new_description"),
		ProjectID:           stringArg(args, "project_id"),
		Project:             stringArg(args, "project"),
		Start:               stringArg(args, "start"),
		End:                 stringArg(args, "end"),
		DryRun:              boolArg(args, "dry_run"),
	}
	if v, ok := args["billable"].(bool); ok {
		out.Billable = &v
	}
	if out.EntryID == "" && out.DescriptionContains == "" && out.ExactDescription == "" {
		return out, fmt.Errorf("provide entry_id, exact_description, or description_contains to find an entry")
	}
	if out.NewDescription == "" && out.ProjectID == "" && out.Project == "" && out.Start == "" && out.End == "" && out.Billable == nil {
		return out, fmt.Errorf("provide at least one update field")
	}
	for _, pair := range []struct{ label, value string }{{"start_after", out.StartAfter}, {"start_before", out.StartBefore}} {
		if pair.value != "" {
			if _, err := time.Parse(time.RFC3339, pair.value); err != nil {
				return out, fmt.Errorf("%s must be RFC3339: %w", pair.label, err)
			}
		}
	}
	return out, nil
}

func (s *Service) findSingleEntry(ctx context.Context, args findAndUpdateArgs) (clockify.TimeEntry, map[string]any, error) {
	query := map[string]string{"page-size": "100"}
	if args.StartAfter != "" {
		query["start"] = args.StartAfter
	}
	if args.StartBefore != "" {
		query["end"] = args.StartBefore
	}
	entries, _, _, err := s.listEntriesWithQuery(ctx, query)
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	matches := make([]clockify.TimeEntry, 0)
	for _, entry := range entries {
		if args.EntryID != "" && entry.ID != args.EntryID {
			continue
		}
		if args.ExactDescription != "" && !strings.EqualFold(strings.TrimSpace(entry.Description), strings.TrimSpace(args.ExactDescription)) {
			continue
		}
		if args.DescriptionContains != "" && !strings.Contains(strings.ToLower(entry.Description), strings.ToLower(args.DescriptionContains)) {
			continue
		}
		matches = append(matches, entry)
	}
	if len(matches) == 0 {
		return clockify.TimeEntry{}, nil, fmt.Errorf("no matching entry found")
	}
	if len(matches) > 1 {
		return clockify.TimeEntry{}, nil, fmt.Errorf("multiple entries matched; narrow the filters")
	}
	matchedBy := map[string]any{}
	if args.EntryID != "" {
		matchedBy["entryId"] = args.EntryID
	}
	if args.ExactDescription != "" {
		matchedBy["exactDescription"] = args.ExactDescription
	}
	if args.DescriptionContains != "" {
		matchedBy["descriptionContains"] = args.DescriptionContains
	}
	if args.StartAfter != "" {
		matchedBy["startAfter"] = args.StartAfter
	}
	if args.StartBefore != "" {
		matchedBy["startBefore"] = args.StartBefore
	}
	return matches[0], matchedBy, nil
}
