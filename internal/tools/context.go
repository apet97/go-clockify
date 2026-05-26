package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func (s *Service) ResolveName(ctx context.Context, args map[string]any) (ToolResult, error) {
	return s.resolveName(ctx, args, oneUserToolGuide)
}

func (s *Service) resolveName(ctx context.Context, args map[string]any, action string) (ToolResult, error) {
	entityType := stringArg(args, "entity_type")
	nameOrID := stringArg(args, "name_or_id")
	if entityType == "" || nameOrID == "" {
		return ToolResult{}, fmt.Errorf("entity_type and name_or_id are required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	var resolvedID string
	var resolveErr error

	switch strings.ToLower(entityType) {
	case "project":
		resolvedID, resolveErr = s.resolveProjectID(ctx, wsID, nameOrID)
	case "client":
		resolvedID, resolveErr = s.resolveClientID(ctx, wsID, nameOrID)
	case "tag":
		resolvedID, resolveErr = s.resolveTagID(ctx, wsID, nameOrID)
	case "user":
		resolvedID, resolveErr = s.resolveUserID(ctx, wsID, nameOrID)
	case "task":
		projectID := stringArg(args, "project_id")
		if projectID == "" {
			projectRef := stringArg(args, "project")
			if projectRef == "" {
				return ToolResult{}, fmt.Errorf("project or project_id is required when entity_type is task")
			}
			projectID, resolveErr = s.resolveProjectID(ctx, wsID, projectRef)
		}
		if resolveErr == nil {
			resolvedID, resolveErr = s.resolveTaskID(ctx, wsID, projectID, nameOrID)
		}
	default:
		return ToolResult{}, fmt.Errorf("entity_type must be project, client, tag, user, or task; got %q", entityType)
	}

	status := "exact_match"
	errMsg := ""
	if resolveErr != nil {
		resolvedID = ""
		errMsg = resolveErr.Error()
		switch {
		case strings.Contains(errMsg, "not found"):
			status = "not_found"
		case strings.Contains(errMsg, "multiple"):
			status = "multiple_matches"
		default:
			status = "error"
		}
	}

	data := map[string]any{
		"entity_type": entityType,
		"input":       nameOrID,
		"resolved_id": resolvedID,
		"status":      status,
		"error":       errMsg,
	}
	if status == "multiple_matches" {
		candidates, err := s.resolveNameCandidates(ctx, wsID, entityType, nameOrID)
		if err == nil && len(candidates) > 0 {
			data["candidates"] = candidates
		}
	}

	return ok(action, data, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) resolveNameCandidates(ctx context.Context, workspaceID, entityType, ref string) ([]map[string]any, error) {
	const pageSize = 200
	entity := strings.ToLower(entityType)
	path := "/workspaces/" + workspaceID + "/" + entityCandidateCollection(entity)
	query := map[string]string{"page-size": strconv.Itoa(pageSize)}
	if entity == "user" {
		if looksLikeEmailForResolveCache(ref) {
			query["email"] = ref
		} else {
			query["name"] = ref
			query["strict-name-search"] = "true"
		}
	} else {
		query["name"] = ref
		query["strict-name-search"] = "true"
	}

	out := make([]map[string]any, 0, 4)
	for page := 1; page <= aggregatePageSafetyStop; page++ {
		query["page"] = strconv.Itoa(page)
		var items []map[string]any
		if err := s.Client.Get(ctx, path, query, &items); err != nil {
			return nil, err
		}
		for _, item := range items {
			if !candidateMatches(entity, item, ref) {
				continue
			}
			candidate := map[string]any{}
			if id, _ := item["id"].(string); id != "" {
				candidate["id"] = id
			}
			if name, _ := item["name"].(string); name != "" {
				candidate["name"] = name
			}
			if email, _ := item["email"].(string); email != "" {
				candidate["email"] = email
			}
			if len(candidate) > 0 {
				out = append(out, candidate)
			}
		}
		if len(items) < pageSize {
			return out, nil
		}
	}
	return out, fmt.Errorf("candidate pagination safety stop reached: path=%s page-size=%d", path, pageSize)
}

func entityCandidateCollection(entity string) string {
	switch entity {
	case "user":
		return "users"
	case "project":
		return "projects"
	case "client":
		return "clients"
	case "tag":
		return "tags"
	default:
		return entity + "s"
	}
}

func candidateMatches(entity string, item map[string]any, ref string) bool {
	name, _ := item["name"].(string)
	email, _ := item["email"].(string)
	if entity == "user" {
		if looksLikeEmailForResolveCache(ref) {
			return strings.EqualFold(email, ref)
		}
		return strings.EqualFold(name, ref) || strings.EqualFold(email, ref)
	}
	return strings.EqualFold(name, ref)
}
