package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
)

// ClientsList handles clockify_clients_list: it lists workspace clients with
// pagination and optional auto-pagination, returning compact pagination meta.
func (s *Service) ClientsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, autoPaginated, truncated, err := runListWithAutoPaginate(ctx, args, s.listClients)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(items)}, args, page, pageSize)
	meta = addAutoPaginateMeta(meta, autoPaginated, truncated, maxRowsArg(args))
	return result("clockify_clients_list", "client", map[string]string{"workspaceId": s.WorkspaceID}, withListPaginationData(map[string]any{
		"clients":  items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

// ClientsCreate handles clockify_clients_create: it creates a client by name and
// returns its ID plus a next-action suggestion to create a project for it.
func (s *Service) ClientsCreate(ctx context.Context, args map[string]any) (any, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	client, err := s.createClient(ctx, name)
	if err != nil {
		return nil, err
	}
	return result("clockify_clients_create", "client", map[string]string{
		"workspaceId": s.WorkspaceID,
		"clientId":    client.ID,
	}, client, ChangeSet{Created: []EntityRef{clientRef(client)}}, nil, []NextAction{{
		Tool:   "clockify_projects_create",
		Args:   map[string]any{"client_id": client.ID},
		Reason: "Create a project for this client.",
	}}), nil
}

// ProjectsList handles clockify_projects_list: it lists workspace projects
// (compact view) with pagination and optional auto-pagination.
func (s *Service) ProjectsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, autoPaginated, truncated, err := runListWithAutoPaginate(ctx, args, s.listProjects)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(items)}, args, page, pageSize)
	meta = addAutoPaginateMeta(meta, autoPaginated, truncated, maxRowsArg(args))
	return result("clockify_projects_list", "project", map[string]string{"workspaceId": s.WorkspaceID}, withListPaginationData(map[string]any{
		"projects": compactProjectViewsFromProjects(items),
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

// ProjectsCreate handles clockify_projects_create: it creates a project (with
// optional client linkage) and returns its ID plus a next-action suggestion to
// add a task.
func (s *Service) ProjectsCreate(ctx context.Context, args map[string]any) (any, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	project, clientID, err := s.createProject(ctx, args, name)
	if err != nil {
		return nil, err
	}
	return result("clockify_projects_create", "project", map[string]string{
		"workspaceId": s.WorkspaceID,
		"projectId":   project.ID,
		"clientId":    clientID,
	}, project, ChangeSet{Created: []EntityRef{projectRef(project)}}, nil, []NextAction{{
		Tool:   "clockify_tasks_create",
		Args:   map[string]any{"project_id": project.ID},
		Reason: "Add a task to this project.",
	}}), nil
}

// TasksList handles clockify_tasks_list: it resolves the project from args and
// lists its tasks with pagination and optional auto-pagination.
func (s *Service) TasksList(ctx context.Context, args map[string]any) (any, error) {
	projectID, err := s.projectIDFromArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	tasksList := func(ctx context.Context, args map[string]any) ([]clockify.Task, int, int, error) {
		return s.listTasks(ctx, projectID, args)
	}
	items, page, pageSize, autoPaginated, truncated, err := runListWithAutoPaginate(ctx, args, tasksList)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(items)}, args, page, pageSize)
	meta = addAutoPaginateMeta(meta, autoPaginated, truncated, maxRowsArg(args))
	return result("clockify_tasks_list", "task", map[string]string{"workspaceId": s.WorkspaceID, "projectId": projectID}, withListPaginationData(map[string]any{
		"tasks":    items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

// TasksCreate handles clockify_tasks_create: it resolves the project from args
// and creates a task by name under it, returning the new task ID.
func (s *Service) TasksCreate(ctx context.Context, args map[string]any) (any, error) {
	projectID, err := s.projectIDFromArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	task, err := s.createTask(ctx, projectID, name, args)
	if err != nil {
		return nil, err
	}
	return result("clockify_tasks_create", "task", map[string]string{
		"workspaceId": s.WorkspaceID,
		"projectId":   projectID,
		"taskId":      task.ID,
	}, task, ChangeSet{Created: []EntityRef{taskRef(task)}}, nil, nil), nil
}

// TagsList handles clockify_tags_list: it lists workspace tags with pagination
// and optional auto-pagination.
func (s *Service) TagsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, autoPaginated, truncated, err := runListWithAutoPaginate(ctx, args, s.listTags)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(items)}, args, page, pageSize)
	meta = addAutoPaginateMeta(meta, autoPaginated, truncated, maxRowsArg(args))
	return result("clockify_tags_list", "tag", map[string]string{"workspaceId": s.WorkspaceID}, withListPaginationData(map[string]any{
		"tags":     items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

// TagsCreate handles clockify_tags_create: it creates a tag by name and returns
// its ID.
func (s *Service) TagsCreate(ctx context.Context, args map[string]any) (any, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	tag, err := s.createTag(ctx, name)
	if err != nil {
		return nil, err
	}
	return result("clockify_tags_create", "tag", map[string]string{
		"workspaceId": s.WorkspaceID,
		"tagId":       tag.ID,
	}, tag, ChangeSet{Created: []EntityRef{tagRef(tag)}}, nil, nil), nil
}

// EntriesList handles clockify_entries_list: it lists the current user's time
// entries with pagination and optional auto-pagination, capturing the resolved
// user ID into the result IDs.
func (s *Service) EntriesList(ctx context.Context, args map[string]any) (any, error) {
	var capturedUserID string
	entriesList := func(ctx context.Context, args map[string]any) ([]clockify.TimeEntry, int, int, error) {
		batch, userID, page, pageSize, err := s.listCurrentUserEntries(ctx, args)
		if userID != "" {
			capturedUserID = userID
		}
		return batch, page, pageSize, err
	}
	entries, page, pageSize, autoPaginated, truncated, err := runListWithAutoPaginate(ctx, args, entriesList)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(entries)}, args, page, pageSize)
	meta = addAutoPaginateMeta(meta, autoPaginated, truncated, maxRowsArg(args))
	return result("clockify_entries_list", "entry", map[string]string{"workspaceId": s.WorkspaceID, "userId": capturedUserID}, withListPaginationData(map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

func withListPaginationData(data map[string]any, meta map[string]any) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	for _, key := range []string{"total", "has_more"} {
		if value, ok := meta[key]; ok {
			data[key] = value
		}
	}
	return data
}

// EntriesCreate handles clockify_entries_create: it creates a time entry,
// honoring dry_run with a validated payload preview and emitting future-entry
// warnings, then returns the created entry and the affected IDs.
func (s *Service) EntriesCreate(ctx context.Context, args map[string]any) (any, error) {
	if dryrun.Enabled(args) {
		payload, ids, err := s.buildEntryPayload(ctx, args)
		if err != nil {
			return nil, err
		}
		preview := dryrunPreviewPayloadValidated("clockify_entries_create", payload, validationOK("payload_check"))
		warnings := s.futureEntryWarningsFromArgs(args, time.Now().UTC())
		if len(warnings) > 0 {
			preview["warnings"] = warnings
		}
		return result("clockify_entries_create", "entry", ids, preview, ChangeSet{}, warnings, nil), nil
	}
	warnings := s.futureEntryWarningsFromArgs(args, time.Now().UTC())
	entry, ids, err := s.createEntry(ctx, args)
	if err != nil {
		return nil, err
	}
	s.emitEntryAndWeeklyWithState(ctx, s.WorkspaceID, entry)
	return result("clockify_entries_create", "entry", ids, entry, ChangeSet{Created: []EntityRef{entryRef(entry)}}, warnings, nil), nil
}
