package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
)

func projectListQueryValues(args map[string]any, page, pageSize int) (url.Values, error) {
	query := pageQuery(page, pageSize)
	addStringQuery(query, args, "name", "name")
	addBoolQuery(query, args, "strict_name_search", "strict-name-search")
	addBoolQuery(query, args, "archived", "archived")
	addBoolQuery(query, args, "billable", "billable")
	addBoolQuery(query, args, "contains_client", "contains-client")
	addStringQuery(query, args, "client_status", "client-status")
	addBoolQuery(query, args, "contains_user", "contains-user")
	addStringQuery(query, args, "user_status", "user-status")
	addBoolQuery(query, args, "is_template", "is-template")
	addStringQuery(query, args, "sort_column", "sort-column")
	addStringQuery(query, args, "sort_order", "sort-order")
	addBoolQuery(query, args, "hydrated", "hydrated")
	if _, ok := query["hydrated"]; !ok {
		query["hydrated"] = "false"
	}
	addStringQuery(query, args, "access", "access")
	addIntQuery(query, args, "expense_limit", "expense-limit")
	addStringQuery(query, args, "expense_date", "expense-date")
	addBoolQuery(query, args, "contains_group", "contains-group")
	values := valuesFromQueryMap(query)
	if err := addRepeatedStringQuery(values, args, "clients", "clients"); err != nil {
		return nil, err
	}
	if err := addRepeatedStringQuery(values, args, "users", "users"); err != nil {
		return nil, err
	}
	if err := addRepeatedStringQuery(values, args, "user_groups", "userGroups"); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *Service) GetProject(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
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
	path, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out clockify.Project
	query := map[string]string{"hydrated": "true"}
	addStringQuery(query, args, "custom_field_entity_type", "custom-field-entity-type")
	addIntQuery(query, args, "expense_limit", "expense-limit")
	addStringQuery(query, args, "expense_date", "expense-date")
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ResultEnvelope{}, err
	}
	view, financialMeta := s.enrichProjectView(ctx, wsID, out, args)
	return ok("clockify_projects_get", view, withFinancialMeta(map[string]any{"workspaceId": wsID, "projectId": projectID}, financialMeta)), nil
}

func (s *Service) ListProjectMemberships(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	projectID := strings.TrimSpace(stringArg(args, "project_id"))
	if projectID == "" {
		projectID = strings.TrimSpace(stringArg(args, "project"))
	}
	if projectID == "" {
		return ResultEnvelope{}, fmt.Errorf("project_id is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	projectID, err = s.resolveProjectID(ctx, wsID, projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "projects", projectID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var project map[string]any
	if err := s.Client.Get(ctx, path, map[string]string{"hydrated": "true"}, &project); err != nil {
		return ResultEnvelope{}, err
	}
	memberships := mapSlice(project["memberships"])
	data := map[string]any{
		"id":          projectID,
		"projectId":   projectID,
		"memberships": memberships,
	}
	if groups := firstPresent(project, "userGroups", "user_groups"); groups != nil {
		data["userGroups"] = groups
	}
	return ok("clockify_projects_memberships_list", data, map[string]any{
		"workspaceId": wsID,
		"projectId":   projectID,
		"count":       len(memberships),
	}), nil
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
	if clientID := strings.TrimSpace(stringArg(args, "client_id")); clientID != "" && clientID != existing.ClientID {
		existing.ClientID = clientID
		changedFields = append(changedFields, "client_id")
	} else if clientRef := strings.TrimSpace(stringArg(args, "client")); clientRef != "" {
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
			Action: "clockify_projects_update",
			Data:   dryrun.Preview("clockify_projects_update", args),
			Meta:   meta,
		}, nil
	}

	payload := projectPutPayload(existing)
	if err := applyProjectRequestFields(payload, args); err != nil {
		return ResultEnvelope{}, err
	}
	var updated clockify.Project
	if err := s.Client.Put(ctx, projectPath, payload, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	s.emitResourceUpdate(ctx, projectResourceURI(wsID, projectID))
	view, financialMeta := s.enrichProjectView(ctx, wsID, updated, args)
	return ok("clockify_projects_update", view, withFinancialMeta(meta, financialMeta)), nil
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

func applyProjectRequestFields(payload map[string]any, args map[string]any) error {
	setIfString(payload, args, "color", "color")
	setIfString(payload, args, "note", "note")
	setIfBool(payload, args, "billable", "billable")
	setIfBool(payload, args, "is_public", "isPublic")
	setIfBool(payload, args, "archived", "archived")
	if rate, ok, err := mapArg(args, "cost_rate"); err != nil {
		return err
	} else if ok {
		payload["costRate"] = normalizeRateRequest(rate)
	}
	if rate, ok, err := mapArg(args, "hourly_rate"); err != nil {
		return err
	} else if ok {
		payload["hourlyRate"] = normalizeRateRequest(rate)
	}
	if estimate, ok, err := mapArg(args, "estimate"); err != nil {
		return err
	} else if ok {
		payload["estimate"] = normalizeEstimateRequest(estimate)
	}
	if budgetEstimate, ok, err := mapArg(args, "budget_estimate"); err != nil {
		return err
	} else if ok {
		payload["budgetEstimate"] = normalizeEstimateWithOptionsRequest(budgetEstimate)
	}
	if estimateReset, ok, err := mapArg(args, "estimate_reset"); err != nil {
		return err
	} else if ok {
		payload["estimateReset"] = normalizeEstimateResetRequest(estimateReset)
	}
	if timeEstimate, ok, err := mapArg(args, "time_estimate"); err != nil {
		return err
	} else if ok {
		payload["timeEstimate"] = normalizeTimeEstimateRequest(timeEstimate)
	}
	if memberships, ok, err := mapSliceArg(args, "memberships"); err != nil {
		return err
	} else if ok {
		out := make([]map[string]any, 0, len(memberships))
		for _, item := range memberships {
			out = append(out, normalizeMembershipRequest(item))
		}
		payload["memberships"] = out
	}
	if tasks, ok, err := mapSliceArg(args, "tasks"); err != nil {
		return err
	} else if ok {
		out := make([]map[string]any, 0, len(tasks))
		for _, item := range tasks {
			out = append(out, normalizeTaskRequest(item))
		}
		payload["tasks"] = out
	}
	return nil
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
			Action: "clockify_projects_delete",
			Data:   dryrun.WrapResult(existing, "clockify_projects_delete"),
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
	return ok("clockify_projects_delete", map[string]any{"deleted": true, "projectId": projectID}, map[string]any{"workspaceId": wsID}), nil
}
