package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dedupe"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
	"github.com/apet97/go-clockify/internal/timeparse"
)

// ListEntries returns recent time entries with optional filtering by date range,
// project, and pagination.
func (s *Service) ListEntries(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	page, pageSize := paginationFromArgs(args)

	baseQuery := map[string]string{}

	startRaw := stringArg(args, "start")
	endRaw := stringArg(args, "end")
	loc, err := s.locationFromArgs(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if startRaw != "" {
		t, err := timeparse.ParseDatetime(startRaw, loc)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("invalid start: %w", err)
		}
		baseQuery["start"] = timeparse.FormatISO(t)
	}
	if endRaw != "" {
		t, err := timeparse.ParseDatetime(endRaw, loc)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("invalid end: %w", err)
		}
		baseQuery["end"] = timeparse.FormatISO(t)
	}

	projectFilter := strings.TrimSpace(stringArg(args, "project"))
	if projectFilter != "" {
		entries, wsID, userID, filteredCount, pagesFetched, entriesScanned, resolvedProjectID, err := s.listEntriesWithProjectFilter(ctx, baseQuery, projectFilter, page, pageSize)
		if err != nil {
			return ResultEnvelope{}, err
		}

		meta := addPaginationMeta(map[string]any{
			"workspaceId":    wsID,
			"userId":         userID,
			"count":          len(entries),
			"page":           page,
			"pageSize":       pageSize,
			"projectFilter":  projectFilter,
			"filteredCount":  filteredCount,
			"pagesFetched":   pagesFetched,
			"entriesScanned": entriesScanned,
		}, args, page, pageSize)
		if resolvedProjectID != "" {
			meta["projectFilterResolvedId"] = resolvedProjectID
		}
		views, financialMeta := s.enrichEntryViews(ctx, wsID, entries)
		return ok("clockify_list_entries", views, withFinancialMeta(meta, financialMeta)), nil
	}

	query := make(map[string]string, len(baseQuery)+2)
	for k, v := range baseQuery {
		query[k] = v
	}
	query["page"] = strconv.Itoa(page)
	query["page-size"] = strconv.Itoa(pageSize)

	entries, wsID, userID, err := s.listEntriesWithQuery(ctx, query)
	if err != nil {
		return ResultEnvelope{}, err
	}

	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"count":       len(entries),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	views, financialMeta := s.enrichEntryViews(ctx, wsID, entries)
	return ok("clockify_list_entries", views, withFinancialMeta(meta, financialMeta)), nil
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
	view, financialMeta := s.enrichEntryView(ctx, wsID, entry)
	return ok("clockify_get_entry", view, withFinancialMeta(map[string]any{"workspaceId": wsID}, financialMeta)), nil
}

// TodayEntries returns time entries for the current day.
func (s *Service) TodayEntries(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	page, pageSize := paginationFromArgs(args)

	loc, err := s.locationFromArgs(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
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
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"count":       len(entries),
		"page":        page,
		"pageSize":    pageSize,
		"rangeStart":  timeparse.FormatISO(startOfDay),
		"rangeEnd":    timeparse.FormatISO(nowTime),
	}, args, page, pageSize)
	views, financialMeta := s.enrichEntryViews(ctx, wsID, entries)
	return ok("clockify_today_entries", views, withFinancialMeta(meta, financialMeta)), nil
}

// ListInProgressTimeEntries returns workspace-wide running timers.
func (s *Service) ListInProgressTimeEntries(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	page, pageSize := paginationFromArgs(args)
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "time-entries", "status", "in-progress")
	if err != nil {
		return ResultEnvelope{}, err
	}
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	var entries []clockify.TimeEntry
	if err := s.Client.Get(ctx, path, query, &entries); err != nil {
		return ResultEnvelope{}, err
	}
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(entries),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	views, financialMeta := s.enrichEntryViews(ctx, wsID, entries)
	return ok("clockify_list_in_progress_time_entries", views, withFinancialMeta(meta, financialMeta)), nil
}

// AddEntry creates a new time entry.
func (s *Service) AddEntry(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	startRaw := stringArg(args, "start")
	if startRaw == "" {
		return ResultEnvelope{}, fmt.Errorf("start is required")
	}
	loc, err := s.locationFromArgs(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	startTime, err := timeparse.ParseDatetime(startRaw, loc)
	if err != nil {
		return ResultEnvelope{}, fmt.Errorf("invalid start: %w", err)
	}

	payload := map[string]any{
		"start": timeparse.FormatISO(startTime),
	}

	endRaw := stringArg(args, "end")
	var endTime time.Time
	if endRaw != "" {
		endTime, err = timeparse.ParseDatetime(endRaw, loc)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("invalid end: %w", err)
		}
		if !endTime.After(startTime) {
			return ResultEnvelope{}, fmt.Errorf("end must be after start")
		}
		payload["end"] = timeparse.FormatISO(endTime)
	}

	desc := stringArg(args, "description")
	if desc != "" {
		payload["description"] = desc
	}

	if entryType := stringArg(args, "type"); entryType != "" {
		payload["type"] = entryType
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	projectID := stringArg(args, "project_id")
	projectRef := stringArg(args, "project")
	if projectID == "" && projectRef != "" {
		projectID, err = s.resolveProjectID(ctx, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("%w; use clockify_resolve_name to disambiguate project names before retrying", err)
		}
	}
	if projectID != "" {
		payload["projectId"] = projectID
	}

	taskID := stringArg(args, "task_id")
	if taskID != "" {
		payload["taskId"] = taskID
	}
	if tagIDs := stringSliceArg(args, "tag_ids"); len(tagIDs) > 0 {
		payload["tagIds"] = tagIDs
	}

	if billable, hasBillable := args["billable"].(bool); hasBillable {
		payload["billable"] = billable
	}

	if !endTime.IsZero() {
		if err := s.rejectEntryOverlap(ctx, startTime, endTime, args); err != nil {
			if dryrun.Enabled(args) {
				validation := validationFailed("overlap_check", ValidationProblem{
					Code:        "overlap",
					Field:       "start,end",
					Message:     err.Error(),
					Remediation: "Pass allow_overlap=true only after manually confirming the overlap is intentional.",
				})
				return ResultEnvelope{OK: true, Action: "clockify_add_entry", Data: attachValidationToPreview(dryrun.Preview("clockify_add_entry", args), validation)}, nil
			}
			return ResultEnvelope{}, err
		}
	}

	dedupeMeta, err := s.addEntryDedupeMeta(ctx, desc, projectID, payload)
	if err != nil {
		if dryrun.Enabled(args) {
			validation := validationFailed("dedupe_check", ValidationProblem{
				Code:        "duplicate_or_overlap",
				Message:     err.Error(),
				Remediation: "Change the time entry details, or adjust dedupe/overlap settings only after manual review.",
			})
			return ResultEnvelope{OK: true, Action: "clockify_add_entry", Data: attachValidationToPreview(dryrun.Preview("clockify_add_entry", args), validation)}, nil
		}
		return ResultEnvelope{}, err
	}
	if dryrun.Enabled(args) {
		validation := validationOK("overlap_and_dedupe_check")
		preview := dryrunPreviewPayloadValidated("clockify_add_entry", payload, validation)
		preview["args"] = args
		if len(dedupeMeta) > 0 {
			preview["dedupe"] = dedupeMeta
		}
		return ResultEnvelope{OK: true, Action: "clockify_add_entry", Data: preview}, nil
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
	if len(dedupeMeta) > 0 {
		meta["dedupe"] = dedupeMeta
	}
	s.emitEntryAndWeeklyWithState(ctx, wsID, entry)
	view, financialMeta := s.enrichEntryView(ctx, wsID, entry)
	return ok("clockify_add_entry", view, withFinancialMeta(meta, financialMeta)), nil
}

func (s *Service) addEntryDedupeMeta(ctx context.Context, description, projectID string, payload map[string]any) (map[string]any, error) {
	if s.DedupeConfig == nil || s.DedupeConfig.Mode == dedupe.Off {
		return nil, nil
	}
	cfg := *s.DedupeConfig
	lookback := cfg.LookbackCount
	if lookback <= 0 {
		lookback = 25
	}
	if lookback > 200 {
		lookback = 200
	}
	entries, _, _, err := s.listEntriesWithQuery(ctx, map[string]string{
		"page":      "1",
		"page-size": strconv.Itoa(lookback),
	})
	if err != nil {
		return nil, fmt.Errorf("dedupe lookback failed: %w", err)
	}

	meta := map[string]any{}
	startISO, _ := payload["start"].(string)
	if dup := dedupe.CheckDuplicate(entries, description, projectID, startISO); dup.IsDuplicate {
		if cfg.Mode == dedupe.Block {
			return nil, fmt.Errorf("duplicate time entry blocked: existing entry %s matches description, project, and start minute", dup.ExistingEntryID)
		}
		meta["duplicateOf"] = dup.ExistingEntryID
	}

	if cfg.OverlapCheck {
		endISO, _ := payload["end"].(string)
		if endISO != "" {
			if overlap := dedupe.CheckOverlap(entries, projectID, startISO, endISO); overlap.HasOverlap {
				if cfg.Mode == dedupe.Block {
					return nil, fmt.Errorf("overlapping time entry blocked: existing entry %s overlaps the requested range", overlap.OverlapEntryID)
				}
				meta["overlapWith"] = overlap.OverlapEntryID
				if overlap.Description != "" {
					meta["overlapDescription"] = overlap.Description
				}
			}
		}
	}

	if len(meta) == 0 {
		return nil, nil
	}
	return meta, nil
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

	// Ownership guard. clockify_update_entry is constrained to the API
	// key owner's own entries; the admin path
	// /workspaces/{ws}/time-entries/{id} would otherwise let an elevated
	// key mutate another user's entry. Fail-closed when userId is
	// missing; see requireCurrentUserEntry in users.go.
	if err := s.requireCurrentUserEntry(ctx, existing); err != nil {
		return ResultEnvelope{}, err
	}
	previous := existing

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
		projectID, err = s.resolveProjectID(ctx, wsID, projectRef)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("%w; use clockify_resolve_name to disambiguate project names before retrying", err)
		}
	}
	if projectID != "" && projectID != existing.ProjectID {
		existing.ProjectID = projectID
		changedFields = append(changedFields, "projectId")
	}
	if taskID := stringArg(args, "task_id"); taskID != "" && taskID != existing.TaskID {
		existing.TaskID = taskID
		changedFields = append(changedFields, "taskId")
	}
	if _, ok := args["tag_ids"]; ok {
		tagIDs := stringSliceArg(args, "tag_ids")
		if !stringSlicesEqual(tagIDs, existing.TagIDs) {
			existing.TagIDs = tagIDs
			changedFields = append(changedFields, "tagIds")
		}
	}

	// Merge start
	loc, err := s.locationFromArgs(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if startRaw := stringArg(args, "start"); startRaw != "" {
		t, err := timeparse.ParseDatetime(startRaw, loc)
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
		t, err := timeparse.ParseDatetime(endRaw, loc)
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
		existing.BillablePresent = true
		changedFields = append(changedFields, "billable")
	}

	// Merge type (REGULAR or BREAK). Skip if unchanged so changedFields
	// reflects an actual mutation, not a no-op echo of the existing
	// value the API would silently re-accept anyway.
	if entryType := stringArg(args, "type"); entryType != "" && entryType != existing.Type {
		existing.Type = entryType
		changedFields = append(changedFields, "type")
	}

	meta := map[string]any{
		"workspaceId":   wsID,
		"changedFields": changedFields,
	}

	if dryrun.Enabled(args) {
		validation := validationOK("ownership_and_payload_check")
		preview := dryrunPreviewPayloadValidated("clockify_update_entry", timeEntryPutPayload(existing), validation)
		preview["args"] = args
		preview["current"] = timeEntryUpdatePreview(previous)
		preview["proposed_changes"] = proposedEntryChanges(existing, changedFields)
		return ResultEnvelope{OK: true, Action: "clockify_update_entry", Data: preview, Meta: meta}, nil
	}

	putPayload := timeEntryPutPayload(existing)

	var updated clockify.TimeEntry
	if err := s.Client.Put(ctx, entryPath, putPayload, &updated); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceUpdateWithState(entryResourceURI(wsID, updated.ID), updated)
	s.emitWeeklyReportsForEntryChange(ctx, wsID, &previous, &updated)
	view, financialMeta := s.enrichEntryView(ctx, wsID, updated)
	return ok("clockify_update_entry", view, withFinancialMeta(meta, financialMeta)), nil
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

	// Ownership guard — see the matching comment in UpdateEntry.
	// DeleteEntry is destructive, so the guard runs even for dry-run
	// previews to avoid leaking another user's entry payload into a
	// "what would happen if I deleted X" response.
	if err := s.requireCurrentUserEntry(ctx, entry); err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		view, financialMeta := s.enrichEntryView(ctx, wsID, entry)
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_entry",
			Data:   dryrun.WrapResult(view, "clockify_delete_entry"),
			Meta:   withFinancialMeta(map[string]any{"workspaceId": wsID}, financialMeta),
		}, nil
	}

	if err := s.Client.Delete(ctx, entryPath); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceDeleted(entryResourceURI(wsID, entryID))
	s.emitWeeklyReportsForEntryChange(ctx, wsID, &entry, nil)
	return ok("clockify_delete_entry", map[string]any{"deleted": true, "entryId": entryID}, map[string]any{"workspaceId": wsID}), nil
}

// listEntriesWithQuery is the shared helper for fetching time entries with query parameters.
func (s *Service) listEntriesWithQuery(ctx context.Context, query map[string]string) ([]clockify.TimeEntry, string, string, error) {
	wsID, userID, path, err := s.currentUserEntriesPath(ctx)
	if err != nil {
		return nil, "", "", err
	}
	if query == nil {
		query = map[string]string{}
	}
	if _, ok := query["page-size"]; !ok {
		query["page-size"] = "100"
	}
	var entries []clockify.TimeEntry
	if err := s.Client.Get(ctx, path, query, &entries); err != nil {
		return nil, "", "", err
	}
	return entries, wsID, userID, nil
}

func (s *Service) listEntriesWithProjectFilter(ctx context.Context, baseQuery map[string]string, projectFilter string, page, pageSize int) ([]clockify.TimeEntry, string, string, int, int, int, string, error) {
	const upstreamPageSize = 200

	wsID, userID, path, err := s.currentUserEntriesPath(ctx)
	if err != nil {
		return nil, "", "", 0, 0, 0, "", err
	}

	resolvedProjectID, resolveErr := s.resolveProjectID(ctx, wsID, projectFilter)
	if resolveErr != nil {
		resolvedProjectID = ""
	}

	selected := make([]clockify.TimeEntry, 0, pageSize)
	filteredCount := 0
	entriesScanned := 0
	pagesFetched := 0
	startOffset := (page - 1) * pageSize
	endOffset := startOffset + pageSize

	query := make(map[string]string, len(baseQuery)+3)
	for k, v := range baseQuery {
		query[k] = v
	}
	query["page-size"] = strconv.Itoa(upstreamPageSize)
	if resolvedProjectID != "" {
		query["project"] = resolvedProjectID
	}
	for upstreamPage := 1; upstreamPage <= aggregatePageSafetyStop; upstreamPage++ {
		query["page"] = strconv.Itoa(upstreamPage)

		var batch []clockify.TimeEntry
		if err := s.Client.Get(ctx, path, query, &batch); err != nil {
			return nil, "", "", 0, 0, 0, "", err
		}
		pagesFetched++
		entriesScanned += len(batch)

		for _, entry := range batch {
			if resolvedProjectID == "" && !entryMatchesProjectFilter(entry, projectFilter) {
				continue
			}
			if filteredCount >= startOffset && filteredCount < endOffset {
				selected = append(selected, entry)
			}
			filteredCount++
		}

		if len(batch) < upstreamPageSize {
			return selected, wsID, userID, filteredCount, pagesFetched, entriesScanned, resolvedProjectID, nil
		}
	}

	return nil, "", "", 0, 0, 0, "", fmt.Errorf("list entries project filter pagination safety stop reached at %d pages; narrow the date range", aggregatePageSafetyStop)
}

func entryMatchesProjectFilter(entry clockify.TimeEntry, projectFilter string) bool {
	projectFilter = strings.TrimSpace(projectFilter)
	if projectFilter == "" {
		return true
	}
	return strings.EqualFold(entry.ProjectID, projectFilter) ||
		strings.Contains(strings.ToLower(entry.ProjectName), strings.ToLower(projectFilter))
}

func (s *Service) currentUserEntriesPath(ctx context.Context) (string, string, string, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return "", "", "", err
	}
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return "", "", "", err
	}
	path, err := paths.Workspace(wsID, "user", user.ID, "time-entries")
	if err != nil {
		return "", "", "", err
	}
	return wsID, user.ID, path, nil
}
