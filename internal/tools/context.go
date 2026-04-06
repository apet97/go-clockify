package tools

import (
	"context"
	"fmt"
	"strings"

	"goclmcp/internal/bootstrap"
	"goclmcp/internal/resolve"
)

func (s *Service) ResolveDebug(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	entityType := stringArg(args, "entity_type")
	nameOrID := stringArg(args, "name_or_id")
	if entityType == "" || nameOrID == "" {
		return ResultEnvelope{}, fmt.Errorf("entity_type and name_or_id are required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var resolvedID string
	var resolveErr error

	switch strings.ToLower(entityType) {
	case "project":
		resolvedID, resolveErr = resolve.ResolveProjectID(ctx, s.Client, wsID, nameOrID)
	case "client":
		resolvedID, resolveErr = resolve.ResolveClientID(ctx, s.Client, wsID, nameOrID)
	case "tag":
		resolvedID, resolveErr = resolve.ResolveTagID(ctx, s.Client, wsID, nameOrID)
	case "user":
		resolvedID, resolveErr = resolve.ResolveUserID(ctx, s.Client, wsID, nameOrID)
	default:
		return ResultEnvelope{}, fmt.Errorf("entity_type must be project, client, tag, or user; got %q", entityType)
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

	return ok("clockify_resolve_debug", map[string]any{
		"entity_type": entityType,
		"input":       nameOrID,
		"resolved_id": resolvedID,
		"status":      status,
		"error":       errMsg,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) PolicyInfo(ctx context.Context) (ResultEnvelope, error) {
	if s.PolicyDescribe == nil {
		return ok("clockify_policy_info", map[string]any{
			"message": "policy info not available",
		}, nil), nil
	}
	return ok("clockify_policy_info", s.PolicyDescribe(), nil), nil
}

func (s *Service) SearchTools(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	activateGroup := stringArg(args, "activate_group")
	activateTool := stringArg(args, "activate_tool")
	if activateGroup != "" || activateTool != "" {
		return ok("clockify_search_tools", map[string]any{
			"message": "Tool activation will be available after server pipeline wiring.",
		}, nil), nil
	}

	query := stringArg(args, "query")
	results := bootstrap.SearchCatalog(query)

	// Group results by domain.
	grouped := map[string][]bootstrap.CatalogEntry{}
	for _, entry := range results {
		grouped[entry.Domain] = append(grouped[entry.Domain], entry)
	}

	return ok("clockify_search_tools", map[string]any{
		"query":       query,
		"count":       len(results),
		"by_domain":   grouped,
		"all_results": results,
	}, nil), nil
}
