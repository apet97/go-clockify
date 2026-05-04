package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
	"github.com/apet97/go-clockify/internal/timeparse"
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
		projectID, err = s.resolveProjectID(ctx, wsID, projectRef)
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
	if err := s.rejectEntryOverlap(ctx, start, end, args); err != nil {
		return nil, err
	}
	path, err := paths.Workspace(wsID, "time-entries")
	if err != nil {
		return nil, err
	}
	var out clockify.TimeEntry
	if err := s.Client.Post(ctx, path, payload, &out); err != nil {
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
			projectID, err = s.resolveProjectID(ctx, wsID, parsed.Project)
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
		if entry.TimeInterval.Start != parsed.Start {
			entry.TimeInterval.Start = parsed.Start
			updatedFields = append(updatedFields, "start")
		}
	}
	if parsed.End != "" {
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
	path, err := paths.Workspace(wsID, "time-entries", entry.ID)
	if err != nil {
		return nil, err
	}
	var out clockify.TimeEntry
	if err := s.Client.Put(ctx, path, payload, &out); err != nil {
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
	stopOutcome := "stopped"
	stopResult, stopErr := s.StopTimer(ctx, map[string]any{})
	if stopErr != nil {
		var apiErr *clockify.APIError
		if errors.As(stopErr, &apiErr) && (apiErr.StatusCode == 404 || apiErr.StatusCode == 400) {
			// No timer was running — proceed with start.
			stopOutcome = "no_running_timer"
		} else {
			return ResultEnvelope{}, fmt.Errorf("stop timer: %w", stopErr)
		}
	} else {
		stoppedEntry = stopResult
	}

	// Start a new timer with the given project.
	startResult, startErr := s.StartTimer(ctx, "", projectRef, description)
	if startErr != nil {
		return ResultEnvelope{}, fmt.Errorf("start timer: %w", startErr)
	}

	return ok("clockify_switch_project", map[string]any{
		"stop_outcome": stopOutcome,
		"stopped":      stoppedEntry,
		"started":      startResult.Data,
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
	for _, pair := range []struct {
		label string
		dst   *string
	}{
		{"start_after", &out.StartAfter},
		{"start_before", &out.StartBefore},
		{"start", &out.Start},
		{"end", &out.End},
	} {
		normalized, err := normalizeFindAndUpdateDatetime(pair.label, *pair.dst)
		if err != nil {
			return out, err
		}
		*pair.dst = normalized
	}
	return out, nil
}

func normalizeFindAndUpdateDatetime(label, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := timeparse.ParseDatetime(value, time.UTC)
	if err != nil {
		return "", fmt.Errorf("invalid %s: %w", label, err)
	}
	return timeparse.FormatISO(parsed), nil
}

func (s *Service) findSingleEntry(ctx context.Context, args findAndUpdateArgs) (clockify.TimeEntry, map[string]any, error) {
	if args.EntryID != "" {
		return s.findEntryByID(ctx, args)
	}

	const pageSize = 200
	baseQuery := map[string]string{"page-size": strconv.Itoa(pageSize)}
	if args.StartAfter != "" {
		baseQuery["start"] = args.StartAfter
	}
	if args.StartBefore != "" {
		baseQuery["end"] = args.StartBefore
	}
	if desc := strings.TrimSpace(args.ExactDescription); desc != "" {
		baseQuery["description"] = desc
	} else if desc := strings.TrimSpace(args.DescriptionContains); desc != "" {
		baseQuery["description"] = desc
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	path, err := paths.Workspace(wsID, "user", user.ID, "time-entries")
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}

	matches := make([]clockify.TimeEntry, 0)
	entriesScanned := 0
	pagesFetched := 0
	query := make(map[string]string, len(baseQuery)+1)
	for k, v := range baseQuery {
		query[k] = v
	}
	for page := 1; page <= aggregatePageSafetyStop; page++ {
		query["page"] = strconv.Itoa(page)

		var entries []clockify.TimeEntry
		if err := s.Client.Get(ctx, path, query, &entries); err != nil {
			return clockify.TimeEntry{}, nil, err
		}
		pagesFetched++
		entriesScanned += len(entries)
		for _, entry := range entries {
			if entryMatchesFindArgs(entry, args) {
				matches = append(matches, entry)
				if len(matches) > 1 {
					return clockify.TimeEntry{}, nil, fmt.Errorf("multiple entries matched; narrow the filters, or use clockify_resolve_name before retrying if a project/user/client name is ambiguous")
				}
			}
		}
		if len(entries) < pageSize {
			if len(matches) == 0 {
				return clockify.TimeEntry{}, nil, fmt.Errorf("no matching entry found; re-check exact IDs or use clockify_resolve_name before retrying if a project/user/client name is ambiguous")
			}
			matchedBy := matchedByForFindArgs(args)
			matchedBy["pagesFetched"] = pagesFetched
			matchedBy["entriesScanned"] = entriesScanned
			return matches[0], matchedBy, nil
		}
	}
	return clockify.TimeEntry{}, nil, fmt.Errorf("entry search pagination safety stop reached at %d pages; narrow the filters", aggregatePageSafetyStop)
}

func (s *Service) findEntryByID(ctx context.Context, args findAndUpdateArgs) (clockify.TimeEntry, map[string]any, error) {
	if err := resolve.ValidateID(args.EntryID, "entry_id"); err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	path, err := paths.Workspace(wsID, "time-entries", args.EntryID)
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	var entry clockify.TimeEntry
	if err := s.Client.Get(ctx, path, nil, &entry); err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	if !entryMatchesFindArgs(entry, args) {
		return clockify.TimeEntry{}, nil, fmt.Errorf("no matching entry found")
	}
	return entry, matchedByForFindArgs(args), nil
}

func matchedByForFindArgs(args findAndUpdateArgs) map[string]any {
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
	return matchedBy
}

func entryMatchesFindArgs(entry clockify.TimeEntry, args findAndUpdateArgs) bool {
	if args.EntryID != "" && entry.ID != args.EntryID {
		return false
	}
	if args.ExactDescription != "" && !strings.EqualFold(strings.TrimSpace(entry.Description), strings.TrimSpace(args.ExactDescription)) {
		return false
	}
	if args.DescriptionContains != "" && !strings.Contains(strings.ToLower(entry.Description), strings.ToLower(args.DescriptionContains)) {
		return false
	}
	if args.StartAfter != "" || args.StartBefore != "" {
		start, err := entry.StartTime()
		if err != nil {
			return false
		}
		if args.StartAfter != "" {
			after, err := time.Parse(time.RFC3339, args.StartAfter)
			if err != nil || start.Before(after) {
				return false
			}
		}
		if args.StartBefore != "" {
			before, err := time.Parse(time.RFC3339, args.StartBefore)
			if err != nil || !start.Before(before) {
				return false
			}
		}
	}
	return true
}
