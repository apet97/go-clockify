package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
)

func (s *Service) ListProjects(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "projects")
	if err != nil {
		return ResultEnvelope{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	var projects []clockify.Project
	if err := s.Client.Get(ctx, path, query, &projects); err != nil {
		return ResultEnvelope{}, err
	}
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(projects),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	return ok("clockify_list_projects", projects, meta), nil
}

func (s *Service) GetProject(ctx context.Context, projectRef string) (ResultEnvelope, error) {
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out clockify.Project
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_project", out, map[string]any{"workspaceId": wsID, "projectId": projectID}), nil
}

func (s *Service) CreateProject(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{"name": name}

	clientRef := stringArg(args, "client")
	if clientRef != "" {
		clientID, err := s.resolveClientID(ctx, wsID, clientRef)
		if err != nil {
			return ResultEnvelope{}, err
		}
		payload["clientId"] = clientID
	}
	if color := stringArg(args, "color"); color != "" {
		payload["color"] = color
	}
	if billable, ok := args["billable"].(bool); ok {
		payload["billable"] = billable
	}
	if isPublic, ok := args["is_public"].(bool); ok {
		payload["isPublic"] = isPublic
	}
	if dryrun.Enabled(args) {
		return ok("clockify_create_project", dryrun.Preview("clockify_create_project", payload), map[string]any{"workspaceId": wsID}), nil
	}

	path, err := paths.Workspace(wsID, "projects")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var project clockify.Project
	if err := s.Client.Post(ctx, path, payload, &project); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceUpdate(ctx, projectResourceURI(wsID, project.ID))
	meta := map[string]any{"workspaceId": wsID}
	return ok("clockify_create_project", project, meta), nil
}

// UpdateProject performs a fetch-then-merge update of a project.
// Clockify's PUT /projects/{id} replaces the full record (omitted
// optional fields are nulled), so we GET the existing project, layer
// caller-provided changes over it, and PUT the merged shape back.
// Caller-supplied empty strings are treated as "do not change"; flip
// archival state through the dedicated `archived` boolean instead.
func (s *Service) UpdateProject(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectRef := stringArg(args, "project")
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectPath, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing clockify.Project
	if err := s.Client.Get(ctx, projectPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	changedFields := make([]string, 0, 6)
	if v := strings.TrimSpace(stringArg(args, "name")); v != "" && v != existing.Name {
		existing.Name = v
		changedFields = append(changedFields, "name")
	}
	if v := strings.TrimSpace(stringArg(args, "color")); v != "" && v != existing.Color {
		existing.Color = v
		changedFields = append(changedFields, "color")
	}
	if v := strings.TrimSpace(stringArg(args, "note")); v != "" && v != existing.Note {
		existing.Note = v
		changedFields = append(changedFields, "note")
	}
	if clientRef := strings.TrimSpace(stringArg(args, "client")); clientRef != "" {
		clientID, resolveErr := s.resolveClientID(ctx, wsID, clientRef)
		if resolveErr != nil {
			return ResultEnvelope{}, resolveErr
		}
		if clientID != existing.ClientID {
			existing.ClientID = clientID
			changedFields = append(changedFields, "client")
		}
	}
	if v, ok := args["billable"].(bool); ok && v != existing.Billable {
		existing.Billable = v
		changedFields = append(changedFields, "billable")
	}
	if v, ok := args["is_public"].(bool); ok && v != existing.Public {
		existing.Public = v
		changedFields = append(changedFields, "is_public")
	}
	if v, ok := args["archived"].(bool); ok && v != existing.Archived {
		existing.Archived = v
		changedFields = append(changedFields, "archived")
	}

	meta := map[string]any{
		"workspaceId":   wsID,
		"projectId":     projectID,
		"changedFields": changedFields,
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_update_project",
			Data:   dryrun.Preview("clockify_update_project", args),
			Meta:   meta,
		}, nil
	}

	payload := projectPutPayload(existing)
	var updated clockify.Project
	if err := s.Client.Put(ctx, projectPath, payload, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	s.emitResourceUpdate(ctx, projectResourceURI(wsID, projectID))
	return ok("clockify_update_project", updated, meta), nil
}

// projectPutPayload builds the full-replacement body for
// PUT /projects/{id}. Clockify's PUT validator requires `name` and
// reads optional fields explicitly; omitted optional fields are
// treated as "clear me", so every non-empty field on the fetched
// project is forwarded explicitly to preserve current state.
func projectPutPayload(p clockify.Project) map[string]any {
	out := map[string]any{
		"name":     p.Name,
		"archived": p.Archived,
		"billable": p.Billable,
		"isPublic": p.Public,
	}
	if p.ClientID != "" {
		out["clientId"] = p.ClientID
	}
	if p.Color != "" {
		out["color"] = p.Color
	}
	if p.Note != "" {
		out["note"] = p.Note
	}
	return out
}

// DeleteProject archives the project if still active, then deletes
// it. Clockify rejects DELETE on active projects (the archive
// validator additionally requires the existing name in the body).
// Mirrors clients.DeleteClient's archive-first pattern; both
// resources share the same upstream constraint.
func (s *Service) DeleteProject(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectRef := stringArg(args, "project")
	if projectRef == "" {
		return ResultEnvelope{}, fmt.Errorf("project is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err := s.resolveProjectID(ctx, wsID, projectRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectPath, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing clockify.Project
	if err := s.Client.Get(ctx, projectPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_project",
			Data:   dryrun.WrapResult(existing, "clockify_delete_project"),
			Meta:   map[string]any{"workspaceId": wsID, "projectId": projectID},
		}, nil
	}

	if !existing.Archived {
		archivePayload := map[string]any{"name": existing.Name, "archived": true}
		var archived clockify.Project
		if err := s.Client.Put(ctx, projectPath, archivePayload, &archived); err != nil {
			return ResultEnvelope{}, fmt.Errorf("archive project %s before delete: %w", projectID, err)
		}
	}

	if err := s.Client.Delete(ctx, projectPath); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_project", map[string]any{"deleted": true, "projectId": projectID}, map[string]any{"workspaceId": wsID}), nil
}
