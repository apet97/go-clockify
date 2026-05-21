package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

const demoDefaultRunID = "phase1"

func (s *Service) ClockifyDemoSeed(ctx context.Context, args map[string]any) (any, error) {
	runID := demoRunID(args)
	prefix := demoPrefix(args)
	reviewDate := strings.TrimSpace(stringArg(args, "date"))
	if reviewDate == "" {
		reviewDate = "2026-01-02"
	}
	upsert := true
	if v, ok := args["upsert"].(bool); ok {
		upsert = v
	}

	changed := ChangeSet{}
	warnings := []Warning{}
	client, reused, err := s.ensureDemoClient(ctx, prefix, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, clientRef(client))

	project, reused, err := s.ensureDemoProject(ctx, prefix, client.ID, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, projectRef(project))

	task, reused, err := s.ensureDemoTask(ctx, prefix, project.ID, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, taskRef(task))

	tag, reused, err := s.ensureDemoTag(ctx, prefix, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, tagRef(tag))

	entry, reused, err := s.ensureDemoEntry(ctx, args, prefix, project.ID, task.ID, tag.ID, upsert)
	if err != nil {
		return nil, err
	}
	addChanged(&changed, reused, entryRef(entry))

	out := result("clockify_demo_seed", "demo", map[string]string{
		"workspaceId": s.WorkspaceID,
		"clientId":    client.ID,
		"projectId":   project.ID,
		"taskId":      task.ID,
		"tagId":       tag.ID,
		"entryId":     entry.ID,
	}, map[string]any{
		"runId":      runID,
		"prefix":     prefix,
		"client":     client,
		"project":    project,
		"task":       task,
		"tag":        tag,
		"entry":      entry,
		"usableWith": []string{"clockify_log_work", "clockify_start_work", "clockify_review_day", "clockify_demo_cleanup"},
	}, changed, warnings, []NextAction{
		{
			Tool:   "clockify_review_day",
			Args:   map[string]any{"date": reviewDate},
			Reason: "Review the seeded demo entry and verify report totals.",
		},
		{
			Tool:   "clockify_demo_cleanup",
			Args:   map[string]any{"prefix": prefix},
			Reason: "Clean up the deterministic demo objects when finished.",
		},
	})
	s.updateDemoResource(runID, prefix, "seeded", out)
	return out, nil
}

func (s *Service) ClockifyDemoCleanup(ctx context.Context, args map[string]any) (any, error) {
	runID := demoRunID(args)
	prefix := demoPrefix(args)
	changed := ChangeSet{}
	warnings := []Warning{}

	entries, _, err := s.listAllCurrentUserEntries(ctx, cleanupEntryRangeArgs(args))
	if err != nil {
		warnings = append(warnings, Warning{Code: "entries_list_failed", Message: err.Error()})
	} else {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Description, prefix) {
				s.deleteDemo(ctx, "entry", entry.ID, entry.Description, &changed, &warnings)
			}
		}
	}

	projects, err := s.listAllProjectsIncludingArchived(ctx, map[string]any{"hydrated": false})
	if err != nil {
		warnings = append(warnings, Warning{Code: "projects_list_failed", Message: err.Error()})
	} else {
		for _, project := range projects {
			if strings.HasPrefix(project.Name, prefix) {
				tasks, taskErr := s.listAllTasks(ctx, project.ID, nil)
				if taskErr != nil {
					warnings = append(warnings, Warning{Code: "tasks_list_failed", Message: taskErr.Error()})
				} else {
					for _, task := range tasks {
						if strings.HasPrefix(task.Name, prefix) {
							s.deleteDemoTask(ctx, project.ID, task, &changed, &warnings)
						}
					}
				}
			}
		}
	}

	tags, err := s.listAllTags(ctx, nil)
	if err != nil {
		warnings = append(warnings, Warning{Code: "tags_list_failed", Message: err.Error()})
	} else {
		for _, tag := range tags {
			if strings.HasPrefix(tag.Name, prefix) {
				s.deleteDemo(ctx, "tag", tag.ID, tag.Name, &changed, &warnings)
			}
		}
	}

	for _, project := range projects {
		if strings.HasPrefix(project.Name, prefix) {
			s.deleteDemo(ctx, "project", project.ID, project.Name, &changed, &warnings)
		}
	}

	clients, err := s.listAllClients(ctx, nil)
	if err != nil {
		warnings = append(warnings, Warning{Code: "clients_list_failed", Message: err.Error()})
	} else {
		for _, client := range clients {
			if strings.HasPrefix(client.Name, prefix) {
				s.deleteDemo(ctx, "client", client.ID, client.Name, &changed, &warnings)
			}
		}
	}

	out := result("clockify_demo_cleanup", "demo", map[string]string{"workspaceId": s.WorkspaceID}, map[string]any{
		"runId":         runID,
		"prefix":        prefix,
		"deletedCount":  len(changed.Deleted),
		"warningsCount": len(warnings),
	}, changed, warnings, []NextAction{{
		Tool:   "clockify_demo_seed",
		Args:   map[string]any{"prefix": prefix},
		Reason: "Recreate the deterministic demo fixture if another smoke pass is needed.",
	}})
	s.updateDemoResource(runID, prefix, "cleaned", out)
	return out, nil
}

func (s *Service) ensureDemoClient(ctx context.Context, prefix string, upsert bool) (clockify.ClientEntity, bool, error) {
	name := prefix + " Client"
	if upsert {
		clients, err := s.listAllClients(ctx, nil)
		if err != nil {
			return clockify.ClientEntity{}, false, err
		}
		for _, c := range clients {
			if c.Name == name {
				return c, true, nil
			}
		}
	}
	client, err := s.createClient(ctx, name)
	return client, false, err
}

func (s *Service) ensureDemoProject(ctx context.Context, prefix, clientID string, upsert bool) (clockify.Project, bool, error) {
	name := prefix + " Project"
	if upsert {
		projects, err := s.listAllProjectsIncludingArchived(ctx, map[string]any{"hydrated": false})
		if err != nil {
			return clockify.Project{}, false, err
		}
		var archivedMatch *clockify.Project
		for _, p := range projects {
			if p.Name != name {
				continue
			}
			if clientID != "" && p.ClientID != "" && p.ClientID != clientID {
				continue
			}
			if !p.Archived {
				return p, true, nil
			}
			project := p
			archivedMatch = &project
		}
		if archivedMatch != nil {
			project, err := s.restoreArchivedProject(ctx, *archivedMatch, map[string]any{"client_id": clientID, "billable": true})
			if err != nil {
				return clockify.Project{}, false, err
			}
			return project, true, nil
		}
	}
	project, _, err := s.createProject(ctx, map[string]any{"client_id": clientID, "billable": true}, name)
	return project, false, err
}

func (s *Service) listAllProjectsIncludingArchived(ctx context.Context, args map[string]any) ([]clockify.Project, error) {
	activeArgs := copyArgs(args)
	delete(activeArgs, "archived")
	projects, err := s.listAllProjects(ctx, activeArgs)
	if err != nil {
		return nil, err
	}
	archivedArgs := copyArgs(args)
	archivedArgs["archived"] = true
	archived, err := s.listAllProjects(ctx, archivedArgs)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(projects)+len(archived))
	out := make([]clockify.Project, 0, len(projects)+len(archived))
	for _, project := range append(projects, archived...) {
		if project.ID != "" && seen[project.ID] {
			continue
		}
		if project.ID != "" {
			seen[project.ID] = true
		}
		out = append(out, project)
	}
	return out, nil
}

func (s *Service) restoreArchivedProject(ctx context.Context, project clockify.Project, args map[string]any) (clockify.Project, error) {
	path, err := paths.Workspace(s.WorkspaceID, "projects", project.ID)
	if err != nil {
		return clockify.Project{}, err
	}
	var existing clockify.Project
	if err := s.Client.Get(ctx, path, nil, &existing); err != nil {
		return clockify.Project{}, err
	}
	existing.Archived = false
	if clientID := strings.TrimSpace(stringArg(args, "client_id")); clientID != "" {
		existing.ClientID = clientID
	}
	payload := projectPutPayload(existing)
	if err := applyProjectRequestFields(payload, args); err != nil {
		return clockify.Project{}, err
	}
	var updated clockify.Project
	if err := s.Client.Put(ctx, path, payload, &updated); err != nil {
		return clockify.Project{}, err
	}
	s.emitResourceUpdate(ctx, projectResourceURI(s.WorkspaceID, project.ID))
	return updated, nil
}

func (s *Service) ensureDemoTask(ctx context.Context, prefix, projectID string, upsert bool) (clockify.Task, bool, error) {
	name := prefix + " Task"
	if upsert {
		tasks, err := s.listAllTasks(ctx, projectID, nil)
		if err != nil {
			return clockify.Task{}, false, err
		}
		for _, t := range tasks {
			if t.Name == name {
				return t, true, nil
			}
		}
	}
	task, err := s.createTask(ctx, projectID, name, map[string]any{"billable": true})
	return task, false, err
}

func (s *Service) ensureDemoTag(ctx context.Context, prefix string, upsert bool) (clockify.Tag, bool, error) {
	name := prefix + " Tag"
	if upsert {
		tags, err := s.listAllTags(ctx, nil)
		if err != nil {
			return clockify.Tag{}, false, err
		}
		for _, t := range tags {
			if t.Name == name {
				return t, true, nil
			}
		}
	}
	tag, err := s.createTag(ctx, name)
	return tag, false, err
}

func (s *Service) ensureDemoEntry(ctx context.Context, args map[string]any, prefix, projectID, taskID, tagID string, upsert bool) (clockify.TimeEntry, bool, error) {
	date := strings.TrimSpace(stringArg(args, "date"))
	if date == "" {
		date = "2026-01-02"
	}
	description := prefix + " Time Entry"
	entryArgs := map[string]any{
		"start":       date + " 09:00",
		"end":         date + " 10:00",
		"description": description,
		"project_id":  projectID,
		"task_id":     taskID,
		"tag_ids":     []any{tagID},
		"billable":    true,
	}
	if customFields, ok := args["custom_fields"]; ok {
		entryArgs["custom_fields"] = customFields
	}
	if upsert {
		entries, _, _, _, err := s.listCurrentUserEntries(ctx, map[string]any{
			"start": date + " 00:00",
			"end":   date + " 23:59",
		})
		if err != nil {
			return clockify.TimeEntry{}, false, err
		}
		for _, e := range entries {
			if e.Description == description {
				return e, true, nil
			}
		}
	}
	entry, _, err := s.createEntry(ctx, entryArgs)
	return entry, false, err
}

func addChanged(changed *ChangeSet, reused bool, ref EntityRef) {
	if reused {
		changed.Reused = append(changed.Reused, ref)
		return
	}
	changed.Created = append(changed.Created, ref)
}

func (s *Service) deleteDemoTask(ctx context.Context, projectID string, task clockify.Task, changed *ChangeSet, warnings *[]Warning) {
	if err := resolve.ValidateID(projectID, "project_id"); err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("task %s: %v", task.ID, err)})
		return
	}
	if err := resolve.ValidateID(task.ID, "task_id"); err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("task %s: %v", task.ID, err)})
		return
	}
	path, err := paths.Workspace(s.WorkspaceID, "projects", projectID, "tasks", task.ID)
	if err == nil {
		if !strings.EqualFold(task.Status, "DONE") {
			task.Status = "DONE"
			var updated clockify.Task
			if putErr := s.Client.Put(ctx, path, taskPutPayload(task), &updated); putErr != nil {
				err = fmt.Errorf("mark task DONE before delete: %w", putErr)
			}
		}
	}
	if err == nil {
		err = s.Client.Delete(ctx, path)
	}
	if err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("task %s: %v", task.ID, err)})
		return
	}
	changed.Deleted = append(changed.Deleted, taskRef(task))
}

func (s *Service) deleteDemo(ctx context.Context, entity, id, name string, changed *ChangeSet, warnings *[]Warning) {
	if err := resolve.ValidateID(id, entity+"_id"); err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("%s %s: %v", entity, id, err)})
		return
	}
	var path string
	var err error
	switch entity {
	case "entry":
		path, err = paths.Workspace(s.WorkspaceID, "time-entries", id)
	case "tag":
		path, err = paths.Workspace(s.WorkspaceID, "tags", id)
	case "project":
		path, err = paths.Workspace(s.WorkspaceID, "projects", id)
		if err == nil {
			var existing clockify.Project
			if getErr := s.Client.Get(ctx, path, nil, &existing); getErr != nil {
				err = getErr
			} else if !existing.Archived {
				if putErr := s.Client.Put(ctx, path, map[string]any{"name": existing.Name, "archived": true}, &existing); putErr != nil {
					err = fmt.Errorf("archive project before delete: %w", putErr)
				}
			}
		}
	case "client":
		path, err = paths.Workspace(s.WorkspaceID, "clients", id)
		if err == nil {
			var existing clockify.ClientEntity
			if getErr := s.Client.Get(ctx, path, nil, &existing); getErr != nil {
				err = getErr
			} else if !existing.Archived {
				if putErr := s.Client.Put(ctx, path, map[string]any{"name": existing.Name, "archived": true}, &existing); putErr != nil {
					err = fmt.Errorf("archive client before delete: %w", putErr)
				}
			}
		}
	default:
		err = fmt.Errorf("unknown cleanup entity %q", entity)
	}
	if err == nil {
		err = s.Client.Delete(ctx, path)
	}
	if err != nil {
		*warnings = append(*warnings, Warning{Code: "delete_failed", Message: fmt.Sprintf("%s %s: %v", entity, id, err)})
		return
	}
	changed.Deleted = append(changed.Deleted, EntityRef{Type: entity, ID: id, Name: name})
}

func cleanupEntryRangeArgs(args map[string]any) map[string]any {
	out := map[string]any{"page_size": float64(200)}
	if start := strings.TrimSpace(stringArg(args, "start")); start != "" {
		out["start"] = start
	} else {
		out["start"] = "2026-01-01 00:00"
	}
	if end := strings.TrimSpace(stringArg(args, "end")); end != "" {
		out["end"] = end
	} else {
		out["end"] = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	}
	return out
}

func demoPrefix(args map[string]any) string {
	if prefix := strings.TrimSpace(stringArg(args, "prefix")); prefix != "" {
		return prefix
	}
	runID := strings.TrimSpace(stringArg(args, "run_id"))
	if runID == "" {
		runID = demoDefaultRunID
	}
	return "DEMO-" + runID
}
