package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/apet97/go-clockify/internal/bootstrap"
	"github.com/apet97/go-clockify/internal/resolve"
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
	if activateGroup != "" {
		if s.ActivateGroup == nil {
			return ResultEnvelope{}, fmt.Errorf("tool activation is not configured")
		}
		result, err := s.ActivateGroup(ctx, activateGroup)
		if err != nil {
			return ResultEnvelope{}, err
		}
		return ok("clockify_search_tools", activationPayload(result, fmt.Sprintf(
			"Activated group %q (%d tools now available): %s",
			result.Group, result.ToolCount, strings.Join(result.ActivatedTools, ", "),
		)), nil), nil
	}
	if activateTool != "" {
		if s.ActivateTool == nil {
			return ResultEnvelope{}, fmt.Errorf("tool activation is not configured")
		}
		result, err := s.ActivateTool(ctx, activateTool)
		if err != nil {
			return ResultEnvelope{}, err
		}
		// Tier-1 activation has no Group; Tier-2 tool-name activation
		// brings the entire containing group online. The enumerated
		// list makes that contract visible to the LLM (audit finding 1).
		var message string
		if result.Group != "" {
			message = fmt.Sprintf(
				"Activated tool %q via group %q — the entire group is now available (%d tools): %s",
				result.Name, result.Group, result.ToolCount, strings.Join(result.ActivatedTools, ", "),
			)
		} else {
			message = fmt.Sprintf("Activated tool %q", result.Name)
		}
		return ok("clockify_search_tools", activationPayload(result, message), nil), nil
	}

	query := stringArg(args, "query")
	results := make([]map[string]any, 0)
	for _, entry := range bootstrap.SearchCatalog(query) {
		results = append(results, map[string]any{
			"type":         "tool",
			"name":         entry.Name,
			"domain":       entry.Domain,
			"description":  entry.Description,
			"keywords":     entry.Keywords,
			"availability": "tier1",
		})
	}

	tier2Names := make([]string, 0, len(Tier2Groups))
	for name := range Tier2Groups {
		tier2Names = append(tier2Names, name)
	}
	sort.Strings(tier2Names)
	q := strings.ToLower(strings.TrimSpace(query))
	for _, name := range tier2Names {
		group := Tier2Groups[name]
		if q != "" &&
			!strings.Contains(strings.ToLower(group.Name), q) &&
			!strings.Contains(strings.ToLower(group.Description), q) &&
			!containsKeyword(group.Keywords, q) {
			continue
		}
		descriptors, ok := s.Tier2Handlers(name)
		toolCount := 0
		if ok {
			toolCount = len(descriptors)
		}
		results = append(results, map[string]any{
			"type":         "group",
			"name":         group.Name,
			"domain":       group.Name,
			"description":  group.Description,
			"keywords":     group.Keywords,
			"availability": "tier2",
			"tool_count":   toolCount,
		})
	}

	// Group results by domain.
	grouped := map[string][]map[string]any{}
	for _, entry := range results {
		domain, _ := entry["domain"].(string)
		grouped[domain] = append(grouped[domain], entry)
	}

	return ok("clockify_search_tools", map[string]any{
		"query":       query,
		"count":       len(results),
		"by_domain":   grouped,
		"all_results": results,
	}, nil), nil
}

// activationPayload assembles the data envelope returned by SearchTools
// for any activation request. Centralised so the activate_group and
// activate_tool branches stay schema-aligned and so the activated_tools
// list is always present (empty for Tier-1 single-tool activation, full
// group enumeration for Tier-2).
func activationPayload(result ActivationResult, message string) map[string]any {
	tools := result.ActivatedTools
	if tools == nil {
		tools = []string{}
	}
	return map[string]any{
		"activated":          result.Name,
		"activation_type":    result.Kind,
		"group":              result.Group,
		"tool_count":         result.ToolCount,
		"activated_tools":    tools,
		"activation_message": message,
	}
}

func containsKeyword(keywords []string, query string) bool {
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(keyword), query) {
			return true
		}
	}
	return false
}
