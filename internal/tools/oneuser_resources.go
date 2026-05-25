package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
)

const defaultDemoResourceRunID = demoDefaultRunID

type demoResourceState struct {
	RunID     string            `json:"runId"`
	URI       string            `json:"uri"`
	Exists    bool              `json:"exists"`
	Status    string            `json:"status"`
	Prefix    string            `json:"prefix,omitempty"`
	IDs       map[string]string `json:"ids,omitempty"`
	Changed   ChangeSet         `json:"changed"`
	Warnings  []Warning         `json:"warnings,omitempty"`
	Next      []NextAction      `json:"next,omitempty"`
	UpdatedAt string            `json:"updatedAt,omitempty"`
	Data      any               `json:"data,omitempty"`
}

func oneUserResources() []mcp.Resource {
	return []mcp.Resource{
		{
			URI:         "clockify://status",
			Name:        "Status",
			Title:       "Clockify Status",
			Description: "Current user, pinned workspace, timezone, and running timer.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://workspace",
			Name:        "Workspace",
			Title:       "Pinned Workspace",
			Description: "The pinned Clockify workspace for this one-user MCP.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://user",
			Name:        "User",
			Title:       "Current User",
			Description: "The Clockify user for the configured API key.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://features",
			Name:        "Features",
			Title:       "Workspace Features",
			Description: "Workspace feature list and subscription feature label when returned by Clockify.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://tools",
			Name:        "Tools",
			Title:       "Loaded Tools",
			Description: "All tools loaded at startup, grouped for one-user workflow use.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://mcp/tool-catalog",
			Name:        "MCP tool catalog",
			Title:       "MCP Tool Catalog",
			Description: "Live tool catalog generated from the startup registry.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://mcp/default-toolset",
			Name:        "MCP default toolset",
			Title:       "Default Toolset",
			Description: "The 16 everyday tools advertised by CLOCKIFY_TOOLSET=default.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://mcp/api-parity",
			Name:        "MCP API parity matrix",
			Title:       "API Parity Matrix",
			Description: "Committed API parity matrix for the current MCP surface.",
			MimeType:    "text/markdown",
		},
		{
			URI:         "clockify://mcp/coverage-dashboard",
			Name:        "MCP coverage dashboard",
			Title:       "Coverage Dashboard",
			Description: "Committed coverage dashboard split by ready, recovery-only, paid-feature, and raw-fallback buckets.",
			MimeType:    "text/markdown",
		},
		{
			URI:         "clockify://mcp/live-evidence",
			Name:        "MCP live-test evidence",
			Title:       "Live Test Evidence",
			Description: "Committed live-test evidence and gates for the current MCP surface.",
			MimeType:    "text/markdown",
		},
		{
			URI:         "clockify://mcp/doctor",
			Name:        "MCP doctor",
			Title:       "Doctor",
			Description: "Local diagnostic receipt for config posture, registry counts, toolset, raw gates, and audit state. Secrets are not exposed.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://mcp/audit-tail",
			Name:        "MCP audit tail",
			Title:       "Audit Tail",
			Description: "Tail of the optional local Clockify MCP audit JSONL log.",
			MimeType:    "application/jsonl",
		},
		{
			URI:         "clockify://workflows",
			Name:        "Workflows",
			Title:       "Workflow Guide",
			Description: "Workflow-first guide for common Clockify MCP tasks.",
			MimeType:    "application/json",
		},
		{
			URI:         demoResourceURI(defaultDemoResourceRunID),
			Name:        "Demo run",
			Title:       "Demo Run State",
			Description: "Current state for the default deterministic demo run.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://recent/entries",
			Name:        "Recent entries",
			Title:       "Recent Time Entries",
			Description: "Recent current-user time entries from the pinned workspace.",
			MimeType:    "application/json",
		},
		{
			URI:         "clockify://recent/projects",
			Name:        "Recent projects",
			Title:       "Recent Projects",
			Description: "Recent projects from the pinned workspace.",
			MimeType:    "application/json",
		},
	}
}

// maxDemoResourcesListed caps how many per-run demo resources resources/list
// surfaces. The backing map is unbounded over a process lifetime; this keeps
// the listing bounded and predictable.
const maxDemoResourcesListed = 50

func (s *Service) demoResourcesList() []mcp.Resource {
	if s == nil {
		return nil
	}
	s.resources.mu.RLock()
	states := make([]demoResourceState, 0, len(s.resources.demoResources))
	for _, state := range s.resources.demoResources {
		states = append(states, state)
	}
	s.resources.mu.RUnlock()
	sort.Slice(states, func(i, j int) bool { return states[i].RunID < states[j].RunID })
	out := make([]mcp.Resource, 0, len(states))
	for _, state := range states {
		if state.RunID == defaultDemoResourceRunID {
			continue
		}
		out = append(out, mcp.Resource{
			URI:         state.URI,
			Name:        fmt.Sprintf("Demo run %s", state.RunID),
			Title:       "Demo Run State",
			Description: fmt.Sprintf("Current state for deterministic demo run %s.", state.RunID),
			MimeType:    "application/json",
		})
	}
	if len(out) > maxDemoResourcesListed {
		out = out[len(out)-maxDemoResourcesListed:]
	}
	return out
}

func (s *Service) readOneUserResource(ctx context.Context, uri string) ([]mcp.ResourceContents, bool, error) {
	switch {
	case uri == "clockify://status":
		if err := s.requireClockifyClient(); err != nil {
			return nil, true, err
		}
		out, err := s.ClockifyStatus(ctx, nil)
		if err != nil {
			return nil, true, err
		}
		contents, err := encodeResource(uri, out)
		return contents, true, err
	case uri == "clockify://workspace":
		if err := s.requireClockifyClient(); err != nil {
			return nil, true, err
		}
		wsID, err := s.ResolveWorkspaceID(ctx)
		if err != nil {
			return nil, true, err
		}
		workspace, err := s.getWorkspace(ctx, wsID)
		if err != nil {
			return nil, true, err
		}
		contents, err := encodeResource(uri, workspace)
		return contents, true, err
	case uri == "clockify://user":
		if err := s.requireClockifyClient(); err != nil {
			return nil, true, err
		}
		user, err := s.getCurrentUser(ctx)
		if err != nil {
			return nil, true, err
		}
		contents, err := encodeResource(uri, user)
		return contents, true, err
	case uri == "clockify://features":
		if err := s.requireClockifyClient(); err != nil {
			return nil, true, err
		}
		wsID, err := s.ResolveWorkspaceID(ctx)
		if err != nil {
			return nil, true, err
		}
		workspace, err := s.getWorkspace(ctx, wsID)
		if err != nil {
			return nil, true, err
		}
		contents, err := encodeResource(uri, map[string]any{
			"workspaceId":                workspace.ID,
			"workspaceName":              workspace.Name,
			"features":                   workspace.Features,
			"featureSubscriptionType":    workspace.FeatureSubscriptionType,
			"featureUnavailableHandling": "return ok=false with error.code=feature_unavailable and a recovery hint",
		})
		return contents, true, err
	case uri == "clockify://tools":
		contents, err := encodeResource(uri, s.toolsResourceData())
		return contents, true, err
	case uri == "clockify://mcp/tool-catalog":
		contents, err := encodeResource(uri, s.toolsResourceData())
		return contents, true, err
	case uri == "clockify://mcp/default-toolset":
		contents, err := encodeResource(uri, s.defaultToolsetResourceData())
		return contents, true, err
	case uri == "clockify://mcp/api-parity":
		return []mcp.ResourceContents{{URI: uri, MimeType: "text/markdown", Text: selfInspectAPIParityMatrix}}, true, nil
	case uri == "clockify://mcp/coverage-dashboard":
		return []mcp.ResourceContents{{URI: uri, MimeType: "text/markdown", Text: selfInspectCoverageDashboard}}, true, nil
	case uri == "clockify://mcp/live-evidence":
		return []mcp.ResourceContents{{URI: uri, MimeType: "text/markdown", Text: selfInspectLiveTests}}, true, nil
	case uri == "clockify://mcp/audit-tail":
		text, err := s.auditTailText()
		if err != nil {
			return nil, true, err
		}
		return []mcp.ResourceContents{{URI: uri, MimeType: "application/jsonl", Text: text}}, true, nil
	case uri == "clockify://mcp/doctor":
		contents, err := encodeResource(uri, s.doctorResourceData())
		return contents, true, err
	case uri == "clockify://workflows":
		out, err := s.ClockifyToolsGuide(ctx, nil)
		if err != nil {
			return nil, true, err
		}
		data := out
		if envelope, ok := out.(ToolResult); ok {
			data = envelope.Data
		}
		contents, err := encodeResource(uri, data)
		return contents, true, err
	case uri == "clockify://recent/entries":
		if err := s.requireClockifyClient(); err != nil {
			return nil, true, err
		}
		out, err := s.EntriesList(ctx, map[string]any{"page_size": float64(10)})
		if err != nil {
			return nil, true, err
		}
		contents, err := encodeResource(uri, out)
		return contents, true, err
	case uri == "clockify://recent/projects":
		if err := s.requireClockifyClient(); err != nil {
			return nil, true, err
		}
		out, err := s.ProjectsList(ctx, map[string]any{"page_size": float64(10)})
		if err != nil {
			return nil, true, err
		}
		contents, err := encodeResource(uri, out)
		return contents, true, err
	case strings.HasPrefix(uri, "clockify://demo/"):
		runID := strings.TrimPrefix(uri, "clockify://demo/")
		if runID == "" {
			return nil, true, fmt.Errorf("invalid demo resource URI: %q", uri)
		}
		contents, err := encodeResource(uri, s.demoResourceState(runID))
		return contents, true, err
	default:
		return nil, false, nil
	}
}

func (s *Service) auditTailText() (string, error) {
	if s == nil || strings.TrimSpace(s.AuditLogPath) == "" {
		return "", nil
	}
	raw, err := os.ReadFile(s.AuditLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	const maxTailBytes = 64 * 1024
	if len(raw) > maxTailBytes {
		raw = raw[len(raw)-maxTailBytes:]
		if idx := strings.IndexByte(string(raw), '\n'); idx >= 0 && idx+1 < len(raw) {
			raw = raw[idx+1:]
		}
	}
	return string(raw), nil
}

func (s *Service) doctorResourceData() map[string]any {
	if s == nil {
		return map[string]any{
			"ok": false,
			"issues": []map[string]any{{
				"code":    "service_unconfigured",
				"message": "Clockify service is nil.",
			}},
		}
	}
	toolset := strings.ToLower(strings.TrimSpace(s.Toolset))
	if toolset == "" {
		toolset = "all"
	}
	confirmationMode := strings.TrimSpace(s.ConfirmationMode)
	if confirmationMode == "" {
		confirmationMode = "required"
	}
	full, err := s.FullAccessRegistryChecked()
	if err != nil {
		return map[string]any{
			"ok": false,
			"configuration": map[string]any{
				"client_configured": s.Client != nil,
				"workspace_id":      redactResourceIdentifier(s.WorkspaceID),
				"toolset":           toolset,
			},
			"issues": []map[string]any{{
				"code":    "registry_invalid",
				"message": err.Error(),
			}},
		}
	}
	advertised := s.RegistryForToolset(toolset)
	issues := make([]map[string]any, 0, 2)
	if s.Client == nil {
		issues = append(issues, map[string]any{
			"code":    "client_missing",
			"message": "Clockify client is not configured.",
		})
	}
	if strings.TrimSpace(s.WorkspaceID) == "" {
		issues = append(issues, map[string]any{
			"code":    "workspace_missing",
			"message": "CLOCKIFY_WORKSPACE_ID is required for the one-user runtime.",
		})
	}
	return map[string]any{
		"ok": len(issues) == 0,
		"configuration": map[string]any{
			"client_configured":         s.Client != nil,
			"workspace_id":              redactResourceIdentifier(s.WorkspaceID),
			"toolset":                   toolset,
			"confirmation_mode":         confirmationMode,
			"raw_tools_enabled":         s.EnableRawTools,
			"raw_get_enabled":           s.EnableRawGet,
			"raw_writes_enabled":        s.EnableRawWrites,
			"raw_documented_only":       s.RawWriteDocumentedOnly,
			"audit_log_configured":      strings.TrimSpace(s.AuditLogPath) != "",
			"rate_limits":               s.ToolRateLimits,
			"rate_limit_explicitly_off": s.ToolRateLimitDisabled,
			"default_timezone":          timezoneName(s.DefaultTimezone),
		},
		"registry": map[string]any{
			"loaded_tools":               len(full),
			"advertised_tools":           len(advertised),
			"default_advertised_tools":   len(s.RegistryForToolset("default")),
			"raw_tools_last":             rawToolsLast(full),
			"high_risk_advertised_tools": countHighRiskTools(advertised),
		},
		"resources": map[string]any{
			"doctor_uri":          "clockify://mcp/doctor",
			"default_toolset_uri": "clockify://mcp/default-toolset",
			"coverage_uri":        "clockify://mcp/coverage-dashboard",
			"audit_tail_uri":      "clockify://mcp/audit-tail",
		},
		"issues": issues,
		"next": []string{
			"Call clockify_status first for live workspace/user/timer state.",
			"Use clockify://mcp/default-toolset for the everyday advertised surface.",
			"Use clockify://mcp/coverage-dashboard before release-readiness claims.",
		},
	}
}

func redactResourceIdentifier(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if len(id) <= 6 {
		return "[redacted]"
	}
	return "[redacted]..." + id[len(id)-6:]
}

func timezoneName(loc *time.Location) string {
	if loc == nil {
		return ""
	}
	return loc.String()
}

func rawToolsLast(reg []mcp.ToolDescriptor) bool {
	firstRaw := -1
	for i, descriptor := range reg {
		name := descriptor.Tool.Name
		isRaw := name == "clockify_api_get" || name == "clockify_api_request"
		if isRaw && firstRaw == -1 {
			firstRaw = i
		}
		if !isRaw && firstRaw != -1 {
			return false
		}
	}
	return firstRaw != -1
}

func (s *Service) requireClockifyClient() error {
	if s == nil || s.Client == nil {
		return fmt.Errorf("clockify client is not configured")
	}
	return nil
}

func (s *Service) toolsResourceData() map[string]any {
	s.registry.toolsResourceOnce.Do(func() {
		s.registry.toolsResourceCache = s.buildToolsResourceData()
	})
	return cloneToolsResourceData(s.registry.toolsResourceCache)
}

func (s *Service) defaultToolsetResourceData() map[string]any {
	descriptors := s.RegistryForToolset("default")
	sortDescriptorsForToolsList(descriptors)
	tools := make([]mcp.Tool, 0, len(descriptors))
	for _, descriptor := range descriptors {
		tools = append(tools, descriptor.Tool)
	}
	return map[string]any{
		"toolset":       "default",
		"count":         len(tools),
		"tools":         tools,
		"rawFallback":   []string{},
		"loadedAtStart": true,
		"guidance": []string{
			"Start with clockify_status and clockify_tools_guide.",
			"Use workflow tools for common actions, domain tools when workflow output points there, and raw fallback only as an explicit escape hatch.",
			"CLOCKIFY_TOOLSET=all advertises every loaded tool for expert/debug sessions.",
		},
	}
}

func (s *Service) buildToolsResourceData() map[string]any {
	descriptors := s.RegistryForToolset(s.Toolset)
	sortDescriptorsForToolsList(descriptors)
	tools := make([]mcp.Tool, 0, len(descriptors))
	workflowTools := make([]string, 0, 20)
	domainTools := make(map[string][]string)
	rawTools := make([]string, 0, 2)
	for _, descriptor := range descriptors {
		tool := descriptor.Tool
		tools = append(tools, tool)
		if tool.Name == "clockify_api_get" || tool.Name == "clockify_api_request" {
			rawTools = append(rawTools, tool.Name)
			continue
		}
		if tool.Annotations != nil {
			if category, _ := tool.Annotations["category"].(string); category == "workflow" {
				workflowTools = append(workflowTools, tool.Name)
				continue
			}
			if domain, _ := tool.Annotations["domain"].(string); domain != "" {
				domainTools[domain] = append(domainTools[domain], tool.Name)
			}
		}
	}
	return map[string]any{
		"count":         len(tools),
		"tools":         tools,
		"workflows":     workflowTools,
		"domains":       domainTools,
		"rawFallback":   rawTools,
		"loadedAtStart": true,
	}
}

func cloneToolsResourceData(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		switch v := value.(type) {
		case []mcp.Tool:
			out[key] = append([]mcp.Tool(nil), v...)
		case []string:
			out[key] = append([]string(nil), v...)
		case map[string][]string:
			cp := make(map[string][]string, len(v))
			for domain, tools := range v {
				cp[domain] = append([]string(nil), tools...)
			}
			out[key] = cp
		default:
			out[key] = value
		}
	}
	return out
}

func sortDescriptorsForToolsList(descriptors []mcp.ToolDescriptor) {
	priorityAware := false
	for _, descriptor := range descriptors {
		if _, ok := descriptorPriority(descriptor.Tool); ok {
			priorityAware = true
			break
		}
	}
	sort.Slice(descriptors, func(i, j int) bool {
		if !priorityAware {
			return descriptors[i].Tool.Name < descriptors[j].Tool.Name
		}
		left, lok := descriptorPriority(descriptors[i].Tool)
		right, rok := descriptorPriority(descriptors[j].Tool)
		if !lok {
			left = 50
		}
		if !rok {
			right = 50
		}
		if left != right {
			return left < right
		}
		return descriptors[i].Tool.Name < descriptors[j].Tool.Name
	})
}

func descriptorPriority(tool mcp.Tool) (int, bool) {
	if tool.Annotations == nil {
		return 0, false
	}
	switch v := tool.Annotations["priority"].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func demoRunID(args map[string]any) string {
	runID := strings.TrimSpace(stringArg(args, "run_id"))
	if runID == "" {
		runID = defaultDemoResourceRunID
	}
	return cleanDemoRunID(runID)
}

func demoResourceURI(runID string) string {
	runID = cleanDemoRunID(runID)
	if runID == "" {
		runID = defaultDemoResourceRunID
	}
	return fmt.Sprintf("clockify://demo/%s", runID)
}

func cleanDemoRunID(runID string) string {
	runID = strings.TrimSpace(runID)
	runID = strings.ReplaceAll(runID, "/", "-")
	runID = strings.ReplaceAll(runID, "\\", "-")
	return runID
}

func (s *Service) updateDemoResource(runID, prefix, status string, out ToolResult) {
	if s == nil {
		return
	}
	runID = cleanDemoRunID(runID)
	if runID == "" {
		runID = defaultDemoResourceRunID
	}
	state := demoResourceState{
		RunID:     runID,
		URI:       demoResourceURI(runID),
		Exists:    true,
		Status:    status,
		Prefix:    prefix,
		IDs:       out.IDs,
		Changed:   out.Changed,
		Warnings:  out.Warnings,
		Next:      out.Next,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Data:      out.Data,
	}
	s.resources.mu.Lock()
	if s.resources.demoResources == nil {
		s.resources.demoResources = map[string]demoResourceState{}
	}
	_, existed := s.resources.demoResources[runID]
	s.resources.demoResources[runID] = state
	s.resources.mu.Unlock()
	s.emitResourceUpdateWithState(state.URI, state)
	// A previously-unseen non-default run id is a new entry in
	// ListResources()/demoResourcesList(), so the resource list changed.
	if !existed && runID != defaultDemoResourceRunID && s.EmitResourceListChanged != nil {
		s.EmitResourceListChanged()
	}
}

func (s *Service) demoResourceState(runID string) demoResourceState {
	runID = cleanDemoRunID(runID)
	if runID == "" {
		runID = defaultDemoResourceRunID
	}
	s.resources.mu.RLock()
	state, ok := s.resources.demoResources[runID]
	s.resources.mu.RUnlock()
	if ok {
		return state
	}
	return demoResourceState{
		RunID:   runID,
		URI:     demoResourceURI(runID),
		Exists:  false,
		Status:  "not_seeded",
		Changed: ChangeSet{},
	}
}
