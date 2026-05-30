package tools

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
	"github.com/apet97/go-clockify/internal/timeparse"
)

// FindAndUpdateEntry backs clockify_fix_entry: it locates one time entry by ID
// or strict filters, then applies the selected field updates.
func (s *Service) FindAndUpdateEntry(ctx context.Context, args map[string]any) (any, error) {
	loc, err := s.locationFromArgs(args)
	if err != nil {
		return nil, err
	}
	parsed, err := parseFindAndUpdateArgs(args, loc)
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
	current := entry
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
	if parsed.TaskID != "" && parsed.TaskID != entry.TaskID {
		entry.TaskID = parsed.TaskID
		updatedFields = append(updatedFields, "taskId")
	}
	if parsed.TagIDs != nil && !stringSlicesEqual(parsed.TagIDs, entry.TagIDs) {
		entry.TagIDs = parsed.TagIDs
		updatedFields = append(updatedFields, "tagIds")
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
		entry.BillablePresent = true
		updatedFields = append(updatedFields, "billable")
	}
	if parsed.DryRun {
		view, financialMeta := s.enrichEntryView(ctx, wsID, entry)
		validation := validationOK("lookup_and_payload_check")
		return ok(oneUserToolFixEntry, FindAndUpdateEntryData{
			Entry:          view,
			MatchedBy:      matchedBy,
			UpdatedFields:  updatedFields,
			MatchedEntryID: current.ID,
			Current:        timeEntryUpdatePreview(current),
			Proposed:       proposedEntryChanges(entry, updatedFields),
			DryRun:         true,
			Note:           "No changes were made.",
			Validation:     &validation,
		}, withFinancialMeta(map[string]any{"workspaceId": wsID, "noop": len(updatedFields) == 0}, financialMeta)), nil
	}
	if len(updatedFields) == 0 {
		view, financialMeta := s.enrichEntryView(ctx, wsID, entry)
		return ok(oneUserToolFixEntry, FindAndUpdateEntryData{Entry: view, MatchedBy: matchedBy, UpdatedFields: updatedFields}, withFinancialMeta(map[string]any{"workspaceId": wsID, "noop": true}, financialMeta)), nil
	}
	payload := timeEntryPutPayload(entry)
	path, err := paths.Workspace(wsID, "time-entries", entry.ID)
	if err != nil {
		return nil, err
	}
	var out clockify.TimeEntry
	if err := s.Client.Put(ctx, path, payload, &out); err != nil {
		return nil, err
	}
	view, financialMeta := s.enrichEntryView(ctx, wsID, out)
	return ok(oneUserToolFixEntry, FindAndUpdateEntryData{Entry: view, MatchedBy: matchedBy, UpdatedFields: updatedFields}, withFinancialMeta(map[string]any{"workspaceId": wsID}, financialMeta)), nil
}

func timeEntryUpdatePreview(entry clockify.TimeEntry) *TimeEntryUpdatePreview {
	return &TimeEntryUpdatePreview{
		Description:     entry.Description,
		ProjectID:       entry.ProjectID,
		TaskID:          entry.TaskID,
		TagIDs:          entry.TagIDs,
		Start:           entry.TimeInterval.Start,
		End:             entry.TimeInterval.End,
		Billable:        entry.Billable,
		BillableState:   billableStateFromPresence(entry.Billable, entry.BillablePresent),
		BillablePresent: entry.BillablePresent,
	}
}

func proposedEntryChanges(entry clockify.TimeEntry, fields []string) map[string]any {
	out := make(map[string]any, len(fields))
	for _, field := range fields {
		switch field {
		case "description":
			out["description"] = entry.Description
		case "projectId":
			out["project_id"] = entry.ProjectID
		case "taskId":
			out["task_id"] = entry.TaskID
		case "tagIds":
			out["tag_ids"] = entry.TagIDs
		case "start":
			out["start"] = entry.TimeInterval.Start
		case "end":
			out["end"] = entry.TimeInterval.End
		case "billable":
			out["billable"] = entry.Billable
		}
	}
	return out
}

// SwitchProject stops the current running timer and starts a new one on the
// target project, backing the switch-work timer workflow.
func (s *Service) SwitchProject(ctx context.Context, args map[string]any) (ToolResult, error) {
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectRef == "" {
		projectRef = projectID
	}
	if projectRef == "" {
		return ToolResult{}, fmt.Errorf("project or project_id is required")
	}
	description := stringArg(args, "description")
	if dryrun.Enabled(args) {
		wsID, err := s.ResolveWorkspaceID(ctx)
		if err != nil {
			return ToolResult{}, err
		}
		resolvedProjectID := projectID
		if resolvedProjectID == "" {
			resolvedProjectID, err = s.resolveProjectID(ctx, wsID, projectRef)
			if err != nil {
				return ToolResult{}, err
			}
		}
		payload := map[string]any{
			"description": description,
			"projectId":   resolvedProjectID,
		}
		if customFields, ok, err := entryCustomFieldsFromArgs(args); err != nil {
			return ToolResult{}, err
		} else if ok {
			payload["customFields"] = customFields
		}
		return ok(oneUserToolSwitchWork, dryrunPreviewPayload(oneUserToolSwitchWork, payload), map[string]any{"workspaceId": wsID, "projectId": resolvedProjectID}), nil
	}

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
			return ToolResult{}, fmt.Errorf("stop timer: %w", stopErr)
		}
	} else {
		stoppedEntry = stopResult
	}

	// Start a new timer with the given project.
	startArgs := map[string]any{"project": projectRef, "description": description}
	if projectID != "" {
		startArgs = map[string]any{"project_id": projectID, "description": description}
	}
	if customFields, ok := args["custom_fields"]; ok {
		startArgs["custom_fields"] = customFields
	}
	startResult, startErr := s.StartTimerArgs(ctx, startArgs)
	if startErr != nil {
		return ToolResult{}, fmt.Errorf("start timer: %w", startErr)
	}
	startedEntryID := entryIDFromResultData(startResult.Data)

	return ok(oneUserToolSwitchWork, map[string]any{
		"id":                    startedEntryID,
		"entryId":               startedEntryID,
		"stop_outcome":          stopOutcome,
		"previous_entry_action": stopOutcome,
		"previous_entry":        stoppedEntry,
		"stopped":               stoppedEntry,
		"started":               startResult.Data,
	}, startResult.Meta), nil
}

func entryIDFromResultData(data any) string {
	switch v := data.(type) {
	case EntryView:
		return v.ID
	case clockify.TimeEntry:
		return v.ID
	case map[string]any:
		return oneUserStringFromMap(v, "id", "_id", "entryId", "entry_id", "timeEntryId", "time_entry_id")
	default:
		return ""
	}
}

func parseFindAndUpdateArgs(args map[string]any, loc *time.Location) (findAndUpdateArgs, error) {
	out := findAndUpdateArgs{
		DescriptionContains: stringArg(args, "description_contains"),
		ExactDescription:    stringArg(args, "exact_description"),
		EntryID:             stringArg(args, "entry_id"),
		StartAfter:          stringArg(args, "start_after"),
		StartBefore:         stringArg(args, "start_before"),
		NewDescription:      stringArg(args, "new_description"),
		ProjectID:           stringArg(args, "project_id"),
		Project:             stringArg(args, "project"),
		TaskID:              stringArg(args, "task_id"),
		Start:               stringArg(args, "start"),
		End:                 stringArg(args, "end"),
		DryRun:              boolArg(args, "dry_run"),
	}
	if _, ok := args["tag_ids"]; ok {
		tagIDs, _, err := strictStringSliceArg(args, "tag_ids")
		if err != nil {
			return out, err
		}
		out.TagIDs = tagIDs
	}
	if v, ok := args["billable"].(bool); ok {
		out.Billable = &v
	}
	if out.EntryID == "" && out.DescriptionContains == "" && out.ExactDescription == "" {
		return out, fmt.Errorf("provide entry_id, exact_description, or description_contains to find an entry")
	}
	if out.NewDescription == "" && out.ProjectID == "" && out.Project == "" && out.TaskID == "" && out.TagIDs == nil && out.Start == "" && out.End == "" && out.Billable == nil {
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
		normalized, err := normalizeFindAndUpdateDatetime(pair.label, *pair.dst, loc)
		if err != nil {
			return out, err
		}
		*pair.dst = normalized
	}
	return out, nil
}

func normalizeFindAndUpdateDatetime(label, value string, loc *time.Location) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if loc == nil {
		loc = time.UTC
	}
	parsed, err := timeparse.ParseDatetime(value, loc)
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
	maps.Copy(query, baseQuery)
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
					return clockify.TimeEntry{}, nil, fmt.Errorf("multiple entries matched; narrow the filters, or use clockify_tools_guide and returned IDs before retrying if a project/user/client name is ambiguous")
				}
			}
		}
		if len(entries) < pageSize {
			if len(matches) == 0 {
				return clockify.TimeEntry{}, nil, fmt.Errorf("no matching entry found; re-check exact IDs or use clockify_tools_guide and returned IDs before retrying if a project/user/client name is ambiguous")
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
	// Ownership guard — see UpdateEntry/DeleteEntry. The non-entry_id
	// branch of clockify_find_and_update_entry already routes through
	// the user-scoped /workspaces/{ws}/user/{userID}/time-entries
	// path; this branch fetches by ID on the admin path, so it needs
	// the same constraint to keep the contract uniform across both
	// paths.
	if err := s.requireCurrentUserEntry(ctx, entry); err != nil {
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
	if args.DescriptionContains != "" {
		needle := strings.ToLower(strings.TrimSpace(args.DescriptionContains))
		if needle != "" && !strings.Contains(strings.ToLower(entry.Description), needle) {
			return false
		}
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
