package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func (s *Service) ResolveWorkspaceID(ctx context.Context) (string, error) {
	if s.WorkspaceID != "" {
		if err := resolve.ValidateID(s.WorkspaceID, "workspace_id"); err != nil {
			return "", err
		}
		return s.WorkspaceID, nil
	}
	s.identity.mu.RLock()
	if s.identity.cachedWSID != "" {
		wsID := s.identity.cachedWSID
		s.identity.mu.RUnlock()
		return wsID, nil
	}
	s.identity.mu.RUnlock()
	var workspaces []clockify.Workspace
	if err := s.Client.Get(ctx, "/workspaces", nil, &workspaces); err != nil {
		return "", err
	}
	if len(workspaces) == 1 {
		s.identity.mu.Lock()
		s.identity.cachedWSID = workspaces[0].ID
		s.identity.mu.Unlock()
		return workspaces[0].ID, nil
	}
	if len(workspaces) == 0 {
		return "", fmt.Errorf("no workspaces available for this API key")
	}
	return "", fmt.Errorf("multiple workspaces found; set CLOCKIFY_WORKSPACE_ID")
}

func (s *Service) GetWorkspace(ctx context.Context) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	// paths.Workspace runs resolve.ValidateID and url.PathEscape per
	// segment. Defence-in-depth: config.LoadOneUser already validates an
	// env-supplied CLOCKIFY_WORKSPACE_ID, but ResolveWorkspaceID can
	// also return an auto-detected ID from a /workspaces response.
	path, err := paths.Workspace(wsID)
	if err != nil {
		return ToolResult{}, err
	}
	var out map[string]any
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_workspace_settings", workspaceViewFromRaw(out), map[string]any{"workspaceId": wsID}), nil
}
