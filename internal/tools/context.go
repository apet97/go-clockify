package tools

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/bootstrap"
)

func (s *Service) ResolveName(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.resolveName(ctx, args, "clockify_resolve_name")
}

func (s *Service) ResolveDebug(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.resolveName(ctx, args, "clockify_resolve_debug")
}

func (s *Service) resolveName(ctx context.Context, args map[string]any, action string) (ResultEnvelope, error) {
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
		resolvedID, resolveErr = s.resolveProjectID(ctx, wsID, nameOrID)
	case "client":
		resolvedID, resolveErr = s.resolveClientID(ctx, wsID, nameOrID)
	case "tag":
		resolvedID, resolveErr = s.resolveTagID(ctx, wsID, nameOrID)
	case "user":
		resolvedID, resolveErr = s.resolveUserID(ctx, wsID, nameOrID)
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

func (s *Service) PolicyInfo(ctx context.Context) (ResultEnvelope, error) {
	data := map[string]any{}
	if s.PolicyDescribe == nil {
		data["message"] = "policy info not available"
	} else {
		maps.Copy(data, s.PolicyDescribe())
	}
	if bootstrapInfo := s.bootstrapInfo(); bootstrapInfo != nil {
		maps.Copy(data, bootstrapInfo)
	}
	if s.GroupActivation != nil {
		blocked := make([]string, 0)
		for _, name := range tier2GroupNames() {
			allowed, _ := s.groupActivationStatus(name)
			if !allowed {
				blocked = append(blocked, name)
			}
		}
		data["tier2_groups_blocked_by_policy"] = blocked
	}
	return ok("clockify_policy_info", data, nil), nil
}

func (s *Service) SearchTools(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	activateGroup := stringArg(args, "activate_group")
	activateTool := stringArg(args, "activate_tool")
	replacement := "clockify_list_tools"
	var result ResultEnvelope
	var err error
	if activateGroup != "" {
		replacement = "clockify_activate_group"
		result, err = s.activateGroupByName(ctx, activateGroup)
	} else if activateTool != "" {
		replacement = "clockify_activate_tool"
		result, err = s.activateToolByName(ctx, activateTool)
	} else {
		result, err = s.ListTools(ctx, args)
	}
	if err != nil {
		return ResultEnvelope{}, err
	}
	result.Action = "clockify_search_tools"
	if result.Meta == nil {
		result.Meta = map[string]any{}
	}
	result.Meta["deprecated"] = true
	result.Meta["replacement"] = replacement
	return result, nil
}

func (s *Service) ListTools(_ context.Context, args map[string]any) (ResultEnvelope, error) {
	query := stringArg(args, "query")
	results := make([]map[string]any, 0)
	for _, entry := range bootstrap.SearchCatalog(query) {
		results = append(results, map[string]any{
			"type":         "tool",
			"name":         entry.Name,
			"domain":       entry.Domain,
			"description":  entry.Description,
			"keywords":     entry.Keywords,
			"availability": s.tier1Availability(entry.Name),
		})
	}

	q := strings.ToLower(strings.TrimSpace(query))
	for _, name := range tier2GroupNames() {
		group := Tier2Groups[name]
		if q != "" &&
			!strings.Contains(strings.ToLower(group.Name), q) &&
			!strings.Contains(strings.ToLower(group.Description), q) &&
			!containsKeyword(group.Keywords, q) {
			continue
		}
		entry := map[string]any{
			"type":         "group",
			"name":         group.Name,
			"domain":       group.Name,
			"description":  group.Description,
			"keywords":     group.Keywords,
			"availability": "tier2",
			"tool_count":   len(group.ToolNames),
		}
		allowed, reason := s.groupActivationStatus(group.Name)
		entry["activatable"] = allowed
		if !allowed {
			entry["block_reason"] = reason
		}
		results = append(results, entry)
	}

	// Group results by domain.
	grouped := map[string][]map[string]any{}
	for _, entry := range results {
		domain, _ := entry["domain"].(string)
		grouped[domain] = append(grouped[domain], entry)
	}

	return ok("clockify_list_tools", map[string]any{
		"query":       query,
		"count":       len(results),
		"by_domain":   grouped,
		"all_results": results,
	}, nil), nil
}

func (s *Service) ActivateToolGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.activateGroupByName(ctx, stringArg(args, "name"))
}

func (s *Service) ActivateNamedTool(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.activateToolByName(ctx, stringArg(args, "name"))
}

func (s *Service) DeactivateToolGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := stringArg(args, "name")
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	if s.DeactivateGroup == nil {
		return ResultEnvelope{}, fmt.Errorf("tool deactivation is not configured")
	}
	result, err := s.DeactivateGroup(ctx, name)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_deactivate_group", deactivationPayload(result, deactivationMessage(result)), nil), nil
}

func (s *Service) activateGroupByName(ctx context.Context, name string) (ResultEnvelope, error) {
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	if s.ActivateGroup == nil {
		return ResultEnvelope{}, fmt.Errorf("tool activation is not configured")
	}
	result, err := s.ActivateGroup(ctx, name)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_activate_group", activationPayload(result, activationMessage(result)), nil), nil
}

func (s *Service) activateToolByName(ctx context.Context, name string) (ResultEnvelope, error) {
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	if s.ActivateTool == nil {
		return ResultEnvelope{}, fmt.Errorf("tool activation is not configured")
	}
	result, err := s.ActivateTool(ctx, name)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_activate_tool", activationPayload(result, activationMessage(result)), nil), nil
}

func (s *Service) groupActivationStatus(name string) (bool, string) {
	if s.GroupActivation == nil {
		return true, ""
	}
	return s.GroupActivation(name)
}

func (s *Service) bootstrapInfo() map[string]any {
	if s.Bootstrap == nil {
		return nil
	}
	visible := s.Bootstrap.VisibleCount()
	total := len(s.Bootstrap.Tier1Names)
	hidden := total - visible
	if hidden < 0 {
		hidden = 0
	}
	return map[string]any{
		"bootstrap_mode":      s.Bootstrap.Mode.String(),
		"tier1_visible_count": visible,
		"tier1_hidden_count":  hidden,
		"tier1_total_count":   total,
	}
}

func (s *Service) tier1Availability(name string) string {
	if s.Bootstrap == nil || len(s.Bootstrap.Tier1Names) == 0 {
		return "tier1"
	}
	if s.Bootstrap.Tier1Names[name] && !s.Bootstrap.IsVisible(name) {
		return "tier1_hidden_by_bootstrap"
	}
	return "tier1"
}

// activationPayload assembles the data envelope returned by SearchTools
// for any activation request. Centralised so the activate_group and
// activate_tool branches stay schema-aligned and so the activated_tools
// list is always present (empty for Tier-1 single-tool activation, full
// group enumeration for Tier-2).
func activationPayload(result ActivationResult, message string) map[string]any {
	tools := result.VisibleActivatedTools
	if tools == nil {
		tools = result.ActivatedTools
	}
	if tools == nil {
		tools = []string{}
	}
	payload := map[string]any{
		"activated":          result.Name,
		"activation_type":    result.Kind,
		"group":              result.Group,
		"tool_count":         result.ToolCount,
		"activated_tools":    tools,
		"activation_message": message,
	}
	if result.TotalVisibleTools > 0 {
		payload["total_visible_tools"] = result.TotalVisibleTools
	}
	if len(result.ActivatedToolsHiddenByBootstrap) > 0 {
		payload["activated_tools_hidden_by_bootstrap"] = result.ActivatedToolsHiddenByBootstrap
	}
	if len(result.ActivatedToolsBlockedByPolicy) > 0 {
		payload["activated_tools_blocked_by_policy"] = result.ActivatedToolsBlockedByPolicy
	}
	return payload
}

func activationMessage(result ActivationResult) string {
	if result.Group != "" && result.Kind == "group" {
		if result.TotalVisibleTools > 0 {
			return fmt.Sprintf("Activated group %q (%d tools in group; %d total visible tools)", result.Group, result.ToolCount, result.TotalVisibleTools)
		}
		return fmt.Sprintf("Activated group %q (%d tools in group)", result.Group, result.ToolCount)
	}
	if result.Group != "" {
		if result.TotalVisibleTools > 0 {
			return fmt.Sprintf("Activated tool %q via group %q (%d tools in group; %d total visible tools)", result.Name, result.Group, result.ToolCount, result.TotalVisibleTools)
		}
		return fmt.Sprintf("Activated tool %q via group %q (%d tools in group)", result.Name, result.Group, result.ToolCount)
	}
	if result.TotalVisibleTools > 0 {
		return fmt.Sprintf("Activated tool %q (%d total visible tools)", result.Name, result.TotalVisibleTools)
	}
	return fmt.Sprintf("Activated tool %q", result.Name)
}

func deactivationPayload(result DeactivationResult, message string) map[string]any {
	tools := result.DeactivatedTools
	if tools == nil {
		tools = []string{}
	}
	payload := map[string]any{
		"deactivated":          result.Name,
		"deactivation_type":    result.Kind,
		"group":                result.Group,
		"tool_count":           result.ToolCount,
		"deactivated_tools":    tools,
		"deactivation_message": message,
	}
	if result.TotalVisibleTools > 0 {
		payload["total_visible_tools"] = result.TotalVisibleTools
	}
	return payload
}

func deactivationMessage(result DeactivationResult) string {
	if result.TotalVisibleTools > 0 {
		return fmt.Sprintf("Deactivated group %q (%d tools removed; %d total visible tools)", result.Group, result.ToolCount, result.TotalVisibleTools)
	}
	return fmt.Sprintf("Deactivated group %q (%d tools removed)", result.Group, result.ToolCount)
}

func containsKeyword(keywords []string, query string) bool {
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(keyword), query) {
			return true
		}
	}
	return false
}
