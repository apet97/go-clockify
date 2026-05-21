package tools

import (
	"context"
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

func (s *Service) getWorkspace(ctx context.Context, wsID string) (clockify.Workspace, error) {
	path, err := paths.Workspace(wsID)
	if err != nil {
		return clockify.Workspace{}, err
	}
	var workspace clockify.Workspace
	if err := s.Client.Get(ctx, path, nil, &workspace); err != nil {
		return clockify.Workspace{}, err
	}
	return workspace, nil
}

func (s *Service) currentTimer(ctx context.Context) (any, error) {
	entries, _, userID, err := s.listEntriesWithQuery(ctx, map[string]string{"in-progress": "true", "page-size": "1"})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 || !entries[0].IsRunning() {
		return map[string]any{"running": false, "entry": nil, "userId": userID}, nil
	}
	entry := entries[0]
	start, err := entry.StartTime()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"running":        true,
		"entry":          entry,
		"userId":         userID,
		"elapsedSeconds": int64(time.Since(start).Seconds()),
	}, nil
}

func (s *Service) listClients(ctx context.Context, args map[string]any) ([]clockify.ClientEntity, int, int, error) {
	page, pageSize := paginationFromArgs(args)
	path, err := paths.Workspace(s.WorkspaceID, "clients")
	if err != nil {
		return nil, 0, 0, err
	}
	var out []clockify.ClientEntity
	err = s.Client.Get(ctx, path, clientListQuery(args, page, pageSize), &out)
	return out, page, pageSize, err
}

func (s *Service) listAllClients(ctx context.Context, args map[string]any) ([]clockify.ClientEntity, error) {
	return listAllPages(ctx, args, s.listClients)
}

func (s *Service) createClient(ctx context.Context, name string) (clockify.ClientEntity, error) {
	path, err := paths.Workspace(s.WorkspaceID, "clients")
	if err != nil {
		return clockify.ClientEntity{}, err
	}
	var client clockify.ClientEntity
	err = s.Client.Post(ctx, path, map[string]any{"name": name}, &client)
	return client, err
}

func (s *Service) listProjects(ctx context.Context, args map[string]any) ([]clockify.Project, int, int, error) {
	page, pageSize := paginationFromArgs(args)
	path, err := paths.Workspace(s.WorkspaceID, "projects")
	if err != nil {
		return nil, 0, 0, err
	}
	var out []clockify.Project
	values, err := projectListQueryValues(args, page, pageSize)
	if err != nil {
		return nil, 0, 0, err
	}
	err = s.Client.GetValues(ctx, path, values, &out)
	return out, page, pageSize, err
}

func (s *Service) listAllProjects(ctx context.Context, args map[string]any) ([]clockify.Project, error) {
	return listAllPages(ctx, args, s.listProjects)
}

func (s *Service) createProject(ctx context.Context, args map[string]any, name string) (clockify.Project, string, error) {
	payload := map[string]any{"name": name}
	clientID := strings.TrimSpace(stringArg(args, "client_id"))
	if clientID == "" {
		clientRef := strings.TrimSpace(stringArg(args, "client"))
		if clientRef != "" {
			var err error
			clientID, err = s.resolveClientID(ctx, s.WorkspaceID, clientRef)
			if err != nil {
				return clockify.Project{}, "", err
			}
		}
	}
	if clientID != "" {
		if err := resolve.ValidateID(clientID, "client_id"); err != nil {
			return clockify.Project{}, "", err
		}
		payload["clientId"] = clientID
	}
	if color := strings.TrimSpace(stringArg(args, "color")); color != "" {
		payload["color"] = color
	}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	if isPublic, ok := args["is_public"].(bool); ok {
		payload["isPublic"] = isPublic
	}
	path, err := paths.Workspace(s.WorkspaceID, "projects")
	if err != nil {
		return clockify.Project{}, "", err
	}
	var project clockify.Project
	err = s.Client.Post(ctx, path, payload, &project)
	return project, clientID, err
}

func (s *Service) listTasks(ctx context.Context, projectID string, args map[string]any) ([]clockify.Task, int, int, error) {
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return nil, 0, 0, err
	}
	page, pageSize := paginationFromArgs(args)
	path, err := paths.Workspace(s.WorkspaceID, "projects", projectID, "tasks")
	if err != nil {
		return nil, 0, 0, err
	}
	var out []clockify.Task
	err = s.Client.Get(ctx, path, taskListQuery(args, page, pageSize), &out)
	return out, page, pageSize, err
}

func (s *Service) listAllTasks(ctx context.Context, projectID string, args map[string]any) ([]clockify.Task, error) {
	return listAllPages(ctx, args, func(ctx context.Context, args map[string]any) ([]clockify.Task, int, int, error) {
		return s.listTasks(ctx, projectID, args)
	})
}

func (s *Service) createTask(ctx context.Context, projectID, name string, args map[string]any) (clockify.Task, error) {
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		return clockify.Task{}, err
	}
	payload := map[string]any{"name": name}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	} else if projectID != "" {
		wsID := s.WorkspaceID
		if wsID == "" {
			if resolved, err := s.ResolveWorkspaceID(ctx); err == nil {
				wsID = resolved
			}
		}
		if wsID != "" {
			if projPath, perr := paths.Workspace(wsID, "projects", projectID); perr == nil {
				var proj clockify.Project
				if gerr := s.Client.Get(ctx, projPath, nil, &proj); gerr == nil {
					payload["billable"] = proj.Billable
				}
			}
		}
	}
	path, err := paths.Workspace(s.WorkspaceID, "projects", projectID, "tasks")
	if err != nil {
		return clockify.Task{}, err
	}
	var task clockify.Task
	err = s.Client.Post(ctx, path, payload, &task)
	return task, err
}

func (s *Service) listTags(ctx context.Context, args map[string]any) ([]clockify.Tag, int, int, error) {
	page, pageSize := paginationFromArgs(args)
	path, err := paths.Workspace(s.WorkspaceID, "tags")
	if err != nil {
		return nil, 0, 0, err
	}
	var out []clockify.Tag
	values, err := tagListQueryValues(args, page, pageSize)
	if err != nil {
		return nil, 0, 0, err
	}
	err = s.Client.GetValues(ctx, path, values, &out)
	return out, page, pageSize, err
}

func (s *Service) listAllTags(ctx context.Context, args map[string]any) ([]clockify.Tag, error) {
	return listAllPages(ctx, args, s.listTags)
}

func (s *Service) createTag(ctx context.Context, name string) (clockify.Tag, error) {
	path, err := paths.Workspace(s.WorkspaceID, "tags")
	if err != nil {
		return clockify.Tag{}, err
	}
	var tag clockify.Tag
	err = s.Client.Post(ctx, path, map[string]any{"name": name}, &tag)
	return tag, err
}

func (s *Service) listCurrentUserEntries(ctx context.Context, args map[string]any) ([]clockify.TimeEntry, string, int, int, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return nil, "", 0, 0, err
	}
	page, pageSize := paginationFromArgs(args)
	query := pageQuery(page, pageSize)
	loc := s.location()
	if start := strings.TrimSpace(stringArg(args, "start")); start != "" {
		t, err := timeparse.ParseDatetime(start, loc)
		if err != nil {
			return nil, "", 0, 0, fmt.Errorf("invalid start: %w", err)
		}
		query["start"] = timeparse.FormatISO(t)
	}
	if end := strings.TrimSpace(stringArg(args, "end")); end != "" {
		t, err := timeparse.ParseDatetime(end, loc)
		if err != nil {
			return nil, "", 0, 0, fmt.Errorf("invalid end: %w", err)
		}
		// A bare YYYY-MM-DD end means "through the end of that day"; without
		// this a same-day start==end range is a zero-width window that
		// matches nothing. ParseDatetime treats a 10-char input as date-only.
		if len(end) == 10 {
			local := t.In(loc)
			t = time.Date(local.Year(), local.Month(), local.Day(), 23, 59, 59, int(999*time.Millisecond), loc)
		}
		query["end"] = timeparse.FormatISO(t)
	}
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	if projectID == "" {
		projectRef := strings.TrimSpace(stringArg(args, "project"))
		if projectRef != "" {
			projectID, err = s.resolveProjectID(ctx, s.WorkspaceID, projectRef)
			if err != nil {
				return nil, "", 0, 0, err
			}
		}
	}
	if projectID != "" {
		if err := resolve.ValidateID(projectID, "project_id"); err != nil {
			return nil, "", 0, 0, err
		}
		query["project"] = projectID
	}
	addEntryListQueryParams(query, args)
	values, err := valuesFromEntryListQuery(query, args)
	if err != nil {
		return nil, "", 0, 0, err
	}
	if err := resolve.ValidateID(user.ID, "user_id"); err != nil {
		return nil, "", 0, 0, err
	}
	path, err := paths.Workspace(s.WorkspaceID, "user", user.ID, "time-entries")
	if err != nil {
		return nil, "", 0, 0, err
	}
	var entries []clockify.TimeEntry
	err = s.Client.GetValues(ctx, path, values, &entries)
	return entries, user.ID, page, pageSize, err
}

func (s *Service) listAllCurrentUserEntries(ctx context.Context, args map[string]any) ([]clockify.TimeEntry, string, error) {
	scanArgs := copyArgs(args)
	scanArgs["page_size"] = float64(200)
	var all []clockify.TimeEntry
	var userID string
	for page := 1; ; page++ {
		scanArgs["page"] = float64(page)
		batch, currentUserID, _, pageSize, err := s.listCurrentUserEntries(ctx, scanArgs)
		if err != nil {
			return nil, "", err
		}
		if userID == "" {
			userID = currentUserID
		}
		all = append(all, batch...)
		if len(batch) < pageSize {
			return all, userID, nil
		}
	}
}

func listAllPages[T any](ctx context.Context, args map[string]any, list func(context.Context, map[string]any) ([]T, int, int, error)) ([]T, error) {
	scanArgs := copyArgs(args)
	scanArgs["page_size"] = float64(200)
	var all []T
	for page := 1; ; page++ {
		scanArgs["page"] = float64(page)
		batch, _, pageSize, err := list(ctx, scanArgs)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < pageSize {
			return all, nil
		}
	}
}

// buildEntryPayload assembles the time-entry POST body and the pre-create
// id map from caller args. It is shared by createEntry (the real POST) and
// the EntriesCreate dry-run preview so both derive the payload identically.
func (s *Service) buildEntryPayload(ctx context.Context, args map[string]any) (map[string]any, map[string]string, error) {
	startRaw := strings.TrimSpace(stringArg(args, "start"))
	if startRaw == "" {
		return nil, nil, fmt.Errorf("start is required")
	}
	loc := s.location()
	startTime, err := timeparse.ParseDatetime(startRaw, loc)
	if err != nil {
		return nil, nil, fmt.Errorf("could not parse date %q for start — use YYYY-MM-DD or RFC3339", startRaw)
	}
	payload := map[string]any{"start": timeparse.FormatISO(startTime)}
	if endRaw := strings.TrimSpace(stringArg(args, "end")); endRaw != "" {
		endTime, err := timeparse.ParseDatetime(endRaw, loc)
		if err != nil {
			return nil, nil, fmt.Errorf("could not parse date %q for end — use YYYY-MM-DD or RFC3339", endRaw)
		}
		if !endTime.After(startTime) {
			return nil, nil, fmt.Errorf("end must be after start")
		}
		payload["end"] = timeparse.FormatISO(endTime)
	}
	if desc := strings.TrimSpace(stringArg(args, "description")); desc != "" {
		payload["description"] = desc
	}
	if entryType := strings.TrimSpace(stringArg(args, "type")); entryType != "" {
		payload["type"] = entryType
	}
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	if projectID == "" {
		projectRef := strings.TrimSpace(stringArg(args, "project"))
		if projectRef != "" {
			projectID, err = s.resolveProjectID(ctx, s.WorkspaceID, projectRef)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if projectID != "" {
		if err := resolve.ValidateID(projectID, "project_id"); err != nil {
			return nil, nil, err
		}
		payload["projectId"] = projectID
	}
	taskID := strings.TrimSpace(stringArg(args, "task_id"))
	if taskID == "" {
		taskRef := strings.TrimSpace(stringArg(args, "task"))
		if taskRef != "" {
			if projectID == "" {
				return nil, nil, fmt.Errorf("project_id or project is required when resolving task by name")
			}
			taskID, err = s.resolveTaskID(ctx, s.WorkspaceID, projectID, taskRef)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if taskID != "" {
		if err := resolve.ValidateID(taskID, "task_id"); err != nil {
			return nil, nil, err
		}
		payload["taskId"] = taskID
	}
	tagIDs, err := s.tagIDsFromArgs(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	if len(tagIDs) > 0 {
		payload["tagIds"] = tagIDs
	}
	if customFields, ok, err := entryCustomFieldsFromArgs(args); err != nil {
		return nil, nil, err
	} else if ok {
		payload["customFields"] = customFields
	}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	} else if projectID != "" && !dryrun.Enabled(args) {
		if projPath, perr := paths.Workspace(s.WorkspaceID, "projects", projectID); perr == nil {
			var proj clockify.Project
			if gerr := s.Client.Get(ctx, projPath, nil, &proj); gerr == nil {
				payload["billable"] = proj.Billable
			}
		}
	}
	ids := map[string]string{
		"workspaceId": s.WorkspaceID,
		"projectId":   projectID,
		"taskId":      taskID,
	}
	if len(tagIDs) == 1 {
		ids["tagId"] = tagIDs[0]
	}
	return payload, ids, nil
}

func (s *Service) futureEntryWarningsFromArgs(args map[string]any, now time.Time) []Warning {
	loc := s.location()
	startRaw := strings.TrimSpace(stringArg(args, "start"))
	if startRaw == "" {
		return nil
	}
	start, err := timeparse.ParseDatetime(startRaw, loc)
	if err != nil {
		return nil
	}
	if endRaw := strings.TrimSpace(stringArg(args, "end")); endRaw != "" {
		end, err := timeparse.ParseDatetime(endRaw, loc)
		if err != nil || !end.After(now) {
			return nil
		}
		return []Warning{futureDatedWarning("end", end, now)}
	}
	if start.After(now) {
		return []Warning{futureDatedWarning("start", start, now)}
	}
	return nil
}

func futureDatedWarning(bound string, resolved, now time.Time) Warning {
	return Warning{
		Code: "future_dated",
		Message: fmt.Sprintf("The entry %s (%s) is in the future relative to server now (%s). Confirm this is planned work, not already-finished work.",
			bound, resolved.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339)),
	}
}

func (s *Service) createEntry(ctx context.Context, args map[string]any) (clockify.TimeEntry, map[string]string, error) {
	payload, ids, err := s.buildEntryPayload(ctx, args)
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	if ids["projectId"] == "" && s.workspaceForcesProjects(ctx, s.WorkspaceID) {
		return clockify.TimeEntry{}, nil, fmt.Errorf("this workspace requires a project on every time entry — create one with clockify_create_work_package, then retry with the returned project_id, or pass an existing project name or ID as project")
	}
	path, err := paths.Workspace(s.WorkspaceID, "time-entries")
	if err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	var entry clockify.TimeEntry
	if err := s.Client.Post(ctx, path, payload, &entry); err != nil {
		return clockify.TimeEntry{}, nil, err
	}
	ids["entryId"] = entry.ID
	return entry, ids, nil
}

func entryCustomFieldsFromArgs(args map[string]any) ([]map[string]any, bool, error) {
	items, ok, err := mapSliceArg(args, "custom_fields")
	if err != nil || !ok {
		return nil, ok, err
	}
	out := make([]map[string]any, 0, len(items))
	for i, item := range items {
		fieldID := firstNonEmptyString(
			stringFromAny(item["field_id"]),
			stringFromAny(item["custom_field_id"]),
			stringFromAny(item["customFieldId"]),
			stringFromAny(item["id"]),
		)
		if strings.TrimSpace(fieldID) == "" {
			return nil, true, fmt.Errorf("custom_fields[%d].field_id is required", i)
		}
		if err := resolve.ValidateID(fieldID, fmt.Sprintf("custom_fields[%d].field_id", i)); err != nil {
			return nil, true, err
		}
		value, hasValue := item["value"]
		if !hasValue {
			return nil, true, fmt.Errorf("custom_fields[%d].value is required", i)
		}
		out = append(out, map[string]any{"customFieldId": fieldID, "value": value})
	}
	return out, true, nil
}

func (s *Service) projectIDFromArgs(ctx context.Context, args map[string]any) (string, error) {
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	if projectID != "" {
		if err := resolve.ValidateID(projectID, "project_id"); err != nil {
			return "", err
		}
		return projectID, nil
	}
	projectRef := strings.TrimSpace(stringArg(args, "project"))
	if projectRef == "" {
		return "", fmt.Errorf("project_id or project is required")
	}
	return s.resolveProjectID(ctx, s.WorkspaceID, projectRef)
}

func (s *Service) tagIDsFromArgs(ctx context.Context, args map[string]any) ([]string, error) {
	out, _, err := strictStringSliceArg(args, "tag_ids")
	if err != nil {
		return nil, err
	}
	for _, id := range out {
		if err := resolve.ValidateID(id, "tag_id"); err != nil {
			return nil, err
		}
	}
	if tag := strings.TrimSpace(stringArg(args, "tag")); tag != "" {
		tagID, err := s.resolveTagID(ctx, s.WorkspaceID, tag)
		if err != nil {
			return nil, err
		}
		out = append(out, tagID)
	}
	return out, nil
}

func pageQuery(page, pageSize int) map[string]string {
	return map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
}

func (s *Service) location() *time.Location {
	if s.DefaultTimezone != nil {
		return s.DefaultTimezone
	}
	return time.UTC
}

func (s *Service) timezoneName() string {
	if s.DefaultTimezone != nil {
		return s.DefaultTimezone.String()
	}
	return "UTC"
}

func clientRef(c clockify.ClientEntity) EntityRef {
	return EntityRef{Type: "client", ID: c.ID, Name: c.Name}
}

func projectRef(p clockify.Project) EntityRef {
	return EntityRef{Type: "project", ID: p.ID, Name: p.Name}
}

func taskRef(t clockify.Task) EntityRef {
	return EntityRef{Type: "task", ID: t.ID, Name: t.Name}
}

func tagRef(t clockify.Tag) EntityRef {
	return EntityRef{Type: "tag", ID: t.ID, Name: t.Name}
}

func entryRef(e clockify.TimeEntry) EntityRef {
	return EntityRef{Type: "entry", ID: e.ID, Name: e.Description}
}
