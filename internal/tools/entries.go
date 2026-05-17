package tools

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
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
	addEntryListQueryParams(baseQuery, args)

	projectFilter := strings.TrimSpace(stringArg(args, "project"))
	if projectFilter != "" {
		entries, wsID, userID, filteredCount, pagesFetched, entriesScanned, resolvedProjectID, err := s.listEntriesWithProjectFilter(ctx, baseQuery, args, projectFilter, page, pageSize)
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
		compactEntryListViews(views)
		return ok("clockify_entries_list", views, emptyListMeta(withFinancialMeta(meta, financialMeta), "clockify_log_work")), nil
	}

	query := make(map[string]string, len(baseQuery)+2)
	for k, v := range baseQuery {
		query[k] = v
	}
	query["page"] = strconv.Itoa(page)
	query["page-size"] = strconv.Itoa(pageSize)

	values, err := valuesFromEntryListQuery(query, args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	entries, wsID, userID, err := s.listEntriesWithQueryValues(ctx, values)
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
	compactEntryListViews(views)
	return ok("clockify_entries_list", views, emptyListMeta(withFinancialMeta(meta, financialMeta), "clockify_log_work")), nil
}

func compactEntryListViews(views []EntryView) {
	for i := range views {
		views[i].CustomFieldValues = nil
		if len(views[i].CustomFields) == 0 {
			continue
		}
		filtered := views[i].CustomFields[:0]
		for _, field := range views[i].CustomFields {
			if field.Value != nil {
				filtered = append(filtered, field)
			}
		}
		if len(filtered) == 0 {
			views[i].CustomFields = nil
			continue
		}
		views[i].CustomFields = filtered
	}
}

// GetEntry retrieves a single time entry by ID.
func (s *Service) GetEntry(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	entryID := stringArg(args, "entry_id")
	if err := resolve.ValidateID(entryID, "entry_id"); err != nil {
		return ResultEnvelope{}, err
	}
	if !looksLikeClockifyEntryID(entryID) {
		return ResultEnvelope{}, entryNotFoundError(entryID)
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "time-entries", entryID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	query := entryDetailQuery(args)
	var entry clockify.TimeEntry
	if err := s.Client.Get(ctx, path, query, &entry); err != nil {
		if entryLookupMiss(err) {
			return ResultEnvelope{}, entryNotFoundError(entryID)
		}
		return ResultEnvelope{}, err
	}
	view, financialMeta := s.enrichEntryView(ctx, wsID, entry)
	return ok("clockify_entries_get", view, withFinancialMeta(map[string]any{"workspaceId": wsID}, financialMeta)), nil
}

func entryNotFoundError(entryID string) error {
	return fmt.Errorf("entry '%s' not found. List entries with clockify_entries_list, or narrow the date range and retry with a returned entry_id", entryID)
}

func entryLookupMiss(err error) bool {
	var apiErr *clockify.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode == http.StatusNotFound {
		return true
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		return false
	}
	body := strings.ToLower(apiErr.Body)
	return strings.Contains(body, "doesn't belong to workspace") ||
		strings.Contains(body, "doesn’t belong to workspace") ||
		strings.Contains(body, "not found")
}

func looksLikeClockifyEntryID(id string) bool {
	if len(id) == 24 || len(id) == 32 {
		for i := 0; i < len(id); i++ {
			if !isHexByte(id[i]) {
				return false
			}
		}
		return true
	}
	if len(id) != 36 {
		return false
	}
	for i := 0; i < len(id); i++ {
		switch i {
		case 8, 13, 18, 23:
			if id[i] != '-' {
				return false
			}
		default:
			if !isHexByte(id[i]) {
				return false
			}
		}
	}
	return true
}

func isHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
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

	query := entryDetailQuery(args)

	// Fetch existing entry
	var existing clockify.TimeEntry
	if err := s.Client.Get(ctx, entryPath, query, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	// Ownership guard. clockify_entries_update is constrained to the API
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
			return ResultEnvelope{}, fmt.Errorf("%w; use clockify_tools_guide and returned IDs to disambiguate project names before retrying", err)
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
			return ResultEnvelope{}, fmt.Errorf("could not parse date %q for start — use YYYY-MM-DD or RFC3339", startRaw)
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
			return ResultEnvelope{}, fmt.Errorf("could not parse date %q for end — use YYYY-MM-DD or RFC3339", endRaw)
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
		preview := dryrunPreviewPayloadValidated("clockify_entries_update", timeEntryPutPayload(existing), validation)
		preview["args"] = args
		preview["current"] = timeEntryUpdatePreview(previous)
		preview["proposed_changes"] = proposedEntryChanges(existing, changedFields)
		return ResultEnvelope{OK: true, Action: "clockify_entries_update", Data: preview, Meta: meta}, nil
	}

	putPayload := timeEntryPutPayload(existing)

	var updated clockify.TimeEntry
	if err := s.Client.PutWithQuery(ctx, entryPath, query, putPayload, &updated); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceUpdateWithState(entryResourceURI(wsID, updated.ID), updated)
	s.emitWeeklyReportsForEntryChange(ctx, wsID, &previous, &updated)
	view, financialMeta := s.enrichEntryView(ctx, wsID, updated)
	return ok("clockify_entries_update", view, withFinancialMeta(meta, financialMeta)), nil
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
			Action: "clockify_entries_delete",
			Data:   dryrun.WrapResult(view, "clockify_entries_delete"),
			Meta:   withFinancialMeta(map[string]any{"workspaceId": wsID}, financialMeta),
		}, nil
	}

	if err := s.Client.Delete(ctx, entryPath); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceDeleted(entryResourceURI(wsID, entryID))
	s.emitWeeklyReportsForEntryChange(ctx, wsID, &entry, nil)
	return ok("clockify_entries_delete", map[string]any{"deleted": true, "entryId": entryID}, map[string]any{"workspaceId": wsID}), nil
}

// listEntriesWithQuery is the shared helper for fetching time entries with query parameters.
func (s *Service) listEntriesWithQuery(ctx context.Context, query map[string]string) ([]clockify.TimeEntry, string, string, error) {
	return s.listEntriesWithQueryValues(ctx, valuesFromQueryMap(query))
}

func (s *Service) listEntriesWithQueryValues(ctx context.Context, query url.Values) ([]clockify.TimeEntry, string, string, error) {
	wsID, userID, path, err := s.currentUserEntriesPath(ctx)
	if err != nil {
		return nil, "", "", err
	}
	if query == nil {
		query = url.Values{}
	}
	if _, ok := query["page-size"]; !ok {
		query.Set("page-size", "100")
	}
	var entries []clockify.TimeEntry
	if err := s.Client.GetValues(ctx, path, query, &entries); err != nil {
		return nil, "", "", err
	}
	return entries, wsID, userID, nil
}

func addEntryListQueryParams(query map[string]string, args map[string]any) {
	addStringQuery(query, args, "description", "description")
	addStringQuery(query, args, "task", "task")
	addBoolQuery(query, args, "project_required", "project-required")
	addBoolQuery(query, args, "task_required", "task-required")
	addBoolQuery(query, args, "hydrated", "hydrated")
	addStringQuery(query, args, "in_progress", "in-progress")
	addStringQuery(query, args, "get_week_before", "get-week-before")
}

func valuesFromEntryListQuery(query map[string]string, args map[string]any) (url.Values, error) {
	values := valuesFromQueryMap(query)
	if err := addRepeatedStringQuery(values, args, "tags", "tags"); err != nil {
		return nil, err
	}
	return values, nil
}

func entryDetailQuery(args map[string]any) map[string]string {
	query := map[string]string{}
	addBoolQuery(query, args, "hydrated", "hydrated")
	addBoolQuery(query, args, "consider_duration_format", "consider-duration-format")
	if len(query) == 0 {
		return nil
	}
	return query
}

func (s *Service) listEntriesWithProjectFilter(ctx context.Context, baseQuery map[string]string, args map[string]any, projectFilter string, page, pageSize int) ([]clockify.TimeEntry, string, string, int, int, int, string, error) {
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

	query, err := valuesFromEntryListQuery(baseQuery, args)
	if err != nil {
		return nil, "", "", 0, 0, 0, "", err
	}
	query.Set("page-size", strconv.Itoa(upstreamPageSize))
	if resolvedProjectID != "" {
		query.Set("project", resolvedProjectID)
	}
	for upstreamPage := 1; upstreamPage <= aggregatePageSafetyStop; upstreamPage++ {
		query.Set("page", strconv.Itoa(upstreamPage))

		var batch []clockify.TimeEntry
		if err := s.Client.GetValues(ctx, path, query, &batch); err != nil {
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
