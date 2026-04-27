package tools

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
	"github.com/apet97/go-clockify/internal/timeparse"
)

// ListEntries returns recent time entries with optional filtering by date range,
// project, and pagination.
func (s *Service) ListEntries(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	page := intArg(args, "page", 1)
	pageSize := min(intArg(args, "page_size", 50), 200)

	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	if startRaw != "" {
		t, err := timeparse.ParseDatetime(startRaw, time.UTC)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("invalid start: %w", err)
		}
		query["start"] = timeparse.FormatISO(t)
	}
	if endRaw != "" {
		t, err := timeparse.ParseDatetime(endRaw, time.UTC)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("invalid end: %w", err)
		}
		query["end"] = timeparse.FormatISO(t)
	}

	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, query)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Optional client-side project filter
	projectFilter := stringArg(args, "project")
	if projectFilter != "" {
		lower := strings.ToLower(projectFilter)
		filtered := make([]clockify.TimeEntry, 0, len(entries))
		for _, e := range entries {
			if strings.EqualFold(e.ProjectID, projectFilter) ||
				strings.Contains(strings.ToLower(e.ProjectName), lower) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	meta := map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"count":       len(entries),
		"page":        page,
		"pageSize":    pageSize,
	}
	return ok("clockify_list_entries", entries, meta), nil
}

// GetEntry retrieves a single time entry by ID.
func (s *Service) GetEntry(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	entryID := stringArg(args, "entry_id")
	if err := resolve.ValidateID(entryID, "entry_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "time-entries", entryID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var entry clockify.TimeEntry
	if err := s.Client.Get(ctx, path, nil, &entry); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_entry", entry, map[string]any{"workspaceId": wsID}), nil
}

// TodayEntries returns time entries for the current day.
func (s *Service) TodayEntries(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	loc := time.Now().Location()
	startOfDay, err := timeparse.ParseDatetime("today", loc)
	if err != nil {
		return ResultEnvelope{}, fmt.Errorf("failed to parse today: %w", err)
	}
	nowTime, err := timeparse.ParseDatetime("now", loc)
	if err != nil {
		return ResultEnvelope{}, fmt.Errorf("failed to parse now: %w", err)
	}

	query := map[string]string{
		"start":     timeparse.FormatISO(startOfDay),
		"end":       timeparse.FormatISO(nowTime),
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}

	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, query)
	if err != nil {
		return ResultEnvelope{}, err
	}
	meta := map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"count":       len(entries),
		"page":        page,
		"pageSize":    pageSize,
		"rangeStart":  timeparse.FormatISO(startOfDay),
		"rangeEnd":    timeparse.FormatISO(nowTime),
	}
	return ok("clockify_today_entries", entries, meta), nil
}

// AddEntry creates a new time entry.
func (s *Service) AddEntry(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	startRaw := stringArg(args, "start")
	if startRaw == "" {
		return ResultEnvelope{}, fmt.Errorf("start is required")
	}
	startTime, err := timeparse.ParseDatetime(startRaw, time.UTC)
	if err != nil {
		return ResultEnvelope{}, fmt.Errorf("invalid start: %w", err)
	}

	payload := map[string]any{
		"start": timeparse.FormatISO(startTime),
	}

	endRaw := stringArg(args, "end")
	if endRaw != "" {
		endTime, err := timeparse.ParseDatetime(endRaw, time.UTC)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("invalid end: %w", err)
		}
		payload["end"] = timeparse.FormatISO(endTime)
	}

	desc := stringArg(args, "description")
	if desc != "" {
		payload["description"] = desc
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	projectID := stringArg(args, "project_id")
	projectRef := stringArg(args, "project")
	if projectID == "" && projectRef != "" {
		projectID, err = resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
	}
	if projectID != "" {
		payload["projectId"] = projectID
	}

	taskID := stringArg(args, "task_id")
	if taskID != "" {
		payload["taskId"] = taskID
	}

	if billable, hasBillable := args["billable"].(bool); hasBillable {
		payload["billable"] = billable
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{OK: true, Action: "clockify_add_entry", Data: dryrun.Preview("clockify_add_entry", args)}, nil
	}

	path, err := paths.Workspace(wsID, "time-entries")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var entry clockify.TimeEntry
	if err := s.Client.Post(ctx, path, payload, &entry); err != nil {
		return ResultEnvelope{}, err
	}

	meta := map[string]any{"workspaceId": wsID}
	if projectID != "" {
		meta["projectId"] = projectID
	}
	s.emitEntryAndWeeklyWithState(ctx, wsID, entry)
	return ok("clockify_add_entry", entry, meta), nil
}

// UpdateEntry performs a fetch-then-update of a time entry, merging caller fields
// over the existing values.
func (s *Service) UpdateEntry(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	entryID := stringArg(args, "entry_id")
	if err := resolve.ValidateID(entryID, "entry_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	entryPath, err := paths.Workspace(wsID, "time-entries", entryID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Fetch existing entry
	var existing clockify.TimeEntry
	if err := s.Client.Get(ctx, entryPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	loc := s.DefaultTimezone
	if loc == nil {
		loc = time.UTC
	}
	oldWeeklyURIs := weeklyReportURIsForEntry(wsID, existing.TimeInterval.Start, existing.TimeInterval.End, loc)

	// Track changes
	changedFields := make([]string, 0, 6)

	// Merge description
	if desc := stringArg(args, "description"); desc != "" && desc != existing.Description {
		existing.Description = desc
		changedFields = append(changedFields, "description")
	}

	// Merge project
	projectID := stringArg(args, "project_id")
	projectRef := stringArg(args, "project")
	if projectID == "" && projectRef != "" {
		projectID, err = resolve.ResolveProjectID(ctx, s.Client, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
	}
	if projectID != "" && projectID != existing.ProjectID {
		existing.ProjectID = projectID
		changedFields = append(changedFields, "projectId")
	}

	// Merge start
	if startRaw := stringArg(args, "start"); startRaw != "" {
		t, err := timeparse.ParseDatetime(startRaw, time.UTC)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("invalid start: %w", err)
		}
		formatted := timeparse.FormatISO(t)
		if formatted != existing.TimeInterval.Start {
			existing.TimeInterval.Start = formatted
			changedFields = append(changedFields, "start")
		}
	}

	// Merge end
	if endRaw := stringArg(args, "end"); endRaw != "" {
		t, err := timeparse.ParseDatetime(endRaw, time.UTC)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("invalid end: %w", err)
		}
		formatted := timeparse.FormatISO(t)
		if formatted != existing.TimeInterval.End {
			existing.TimeInterval.End = formatted
			changedFields = append(changedFields, "end")
		}
	}

	// Merge billable
	if billable, hasBillable := args["billable"].(bool); hasBillable && billable != existing.Billable {
		existing.Billable = billable
		changedFields = append(changedFields, "billable")
	}

	meta := map[string]any{
		"workspaceId":   wsID,
		"changedFields": changedFields,
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{OK: true, Action: "clockify_update_entry", Data: dryrun.Preview("clockify_update_entry", args), Meta: meta}, nil
	}

	// Build full PUT payload from merged entry
	putPayload := map[string]any{
		"start":       existing.TimeInterval.Start,
		"description": existing.Description,
		"projectId":   existing.ProjectID,
		"billable":    existing.Billable,
	}
	if existing.TimeInterval.End != "" {
		putPayload["end"] = existing.TimeInterval.End
	}
	if existing.TaskID != "" {
		putPayload["taskId"] = existing.TaskID
	}

	var updated clockify.TimeEntry
	if err := s.Client.Put(ctx, entryPath, putPayload, &updated); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitEntryAndWeeklyWithState(ctx, wsID, updated)
	newWeeklyURIs := weeklyReportURIsForEntry(wsID, updated.TimeInterval.Start, updated.TimeInterval.End, loc)
	for _, oldURI := range oldWeeklyURIs {
		if !slices.Contains(newWeeklyURIs, oldURI) {
			s.emitResourceUpdate(ctx, oldURI)
		}
	}
	return ok("clockify_update_entry", updated, meta), nil
}

// DeleteEntry deletes a time entry by ID.
func (s *Service) DeleteEntry(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	entryID := stringArg(args, "entry_id")
	if err := resolve.ValidateID(entryID, "entry_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	entryPath, err := paths.Workspace(wsID, "time-entries", entryID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var entry clockify.TimeEntry
	if err := s.Client.Get(ctx, entryPath, nil, &entry); err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_entry",
			Data:   dryrun.WrapResult(entry, "clockify_delete_entry"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, entryPath); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceDeleted(entryResourceURI(wsID, entryID))
	loc := s.DefaultTimezone
	if loc == nil {
		loc = time.UTC
	}
	for _, uri := range weeklyReportURIsForEntry(wsID, entry.TimeInterval.Start, entry.TimeInterval.End, loc) {
		s.emitResourceUpdate(ctx, uri)
	}
	return ok("clockify_delete_entry", map[string]any{"deleted": true, "entryId": entryID}, map[string]any{"workspaceId": wsID}), nil
}

// listEntriesWithQuery is the shared helper for fetching time entries with query parameters.
func (s *Service) listEntriesWithQuery(ctx context.Context, query map[string]string) ([]clockify.TimeEntry, string, string, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return nil, "", "", err
	}
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return nil, "", "", err
	}
	if query == nil {
		query = map[string]string{}
	}
	if _, ok := query["page-size"]; !ok {
		query["page-size"] = "100"
	}
	path, err := paths.Workspace(wsID, "user", user.ID, "time-entries")
	if err != nil {
		return nil, "", "", err
	}
	var entries []clockify.TimeEntry
	if err := s.Client.Get(ctx, path, query, &entries); err != nil {
		return nil, "", "", err
	}
	return entries, wsID, user.ID, nil
}
