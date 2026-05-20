package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/dryrun"
)

func (s *Service) ClientsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, err := s.listClients(ctx, args)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(items)}, args, page, pageSize)
	return result("clockify_clients_list", "client", map[string]string{"workspaceId": s.WorkspaceID}, withListPaginationData(map[string]any{
		"clients":  items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

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

func (s *Service) ProjectsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, err := s.listProjects(ctx, args)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(items)}, args, page, pageSize)
	return result("clockify_projects_list", "project", map[string]string{"workspaceId": s.WorkspaceID}, withListPaginationData(map[string]any{
		"projects": compactProjectViewsFromProjects(items),
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

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

func (s *Service) TasksList(ctx context.Context, args map[string]any) (any, error) {
	projectID, err := s.projectIDFromArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	items, page, pageSize, err := s.listTasks(ctx, projectID, args)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(items)}, args, page, pageSize)
	return result("clockify_tasks_list", "task", map[string]string{"workspaceId": s.WorkspaceID, "projectId": projectID}, withListPaginationData(map[string]any{
		"tasks":    items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

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

func (s *Service) TagsList(ctx context.Context, args map[string]any) (any, error) {
	items, page, pageSize, err := s.listTags(ctx, args)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(items)}, args, page, pageSize)
	return result("clockify_tags_list", "tag", map[string]string{"workspaceId": s.WorkspaceID}, withListPaginationData(map[string]any{
		"tags":     items,
		"count":    len(items),
		"page":     page,
		"pageSize": pageSize,
	}, meta), ChangeSet{}, nil, nil, meta), nil
}

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

func (s *Service) EntriesList(ctx context.Context, args map[string]any) (any, error) {
	entries, userID, page, pageSize, err := s.listCurrentUserEntries(ctx, args)
	if err != nil {
		return nil, err
	}
	meta := addPaginationMeta(map[string]any{"count": len(entries)}, args, page, pageSize)
	return result("clockify_entries_list", "entry", map[string]string{"workspaceId": s.WorkspaceID, "userId": userID}, withListPaginationData(map[string]any{
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
