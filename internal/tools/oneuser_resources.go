package tools

import (
	"context"
	"fmt"
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
			URI:         "clockify://mcp/api-parity",
			Name:        "MCP API parity matrix",
			Title:       "API Parity Matrix",
			Description: "Committed API parity matrix for the current MCP surface.",
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
	s.mu.RLock()
	states := make([]demoResourceState, 0, len(s.demoResources))
	for _, state := range s.demoResources {
		states = append(states, state)
	}
	s.mu.RUnlock()
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
	case uri == "clockify://mcp/api-parity":
		return []mcp.ResourceContents{{URI: uri, MimeType: "text/markdown", Text: selfInspectAPIParityMatrix}}, true, nil
	case uri == "clockify://mcp/live-evidence":
		return []mcp.ResourceContents{{URI: uri, MimeType: "text/markdown", Text: selfInspectLiveTests}}, true, nil
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

func (s *Service) requireClockifyClient() error {
	if s == nil || s.Client == nil {
		return fmt.Errorf("clockify client is not configured")
	}
	return nil
}

func (s *Service) toolsResourceData() map[string]any {
	s.toolsResourceOnce.Do(func() {
		s.toolsResourceCache = s.buildToolsResourceData()
	})
	return cloneToolsResourceData(s.toolsResourceCache)
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
	s.mu.Lock()
	if s.demoResources == nil {
		s.demoResources = map[string]demoResourceState{}
	}
	_, existed := s.demoResources[runID]
	s.demoResources[runID] = state
	s.mu.Unlock()
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
	s.mu.RLock()
	state, ok := s.demoResources[runID]
	s.mu.RUnlock()
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
