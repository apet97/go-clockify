package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

// ResolveWorkspaceID returns the pinned workspace ID, validating it; when none
// is configured it auto-detects from /workspaces (caching the result) and errors
// if zero or multiple workspaces are available.
func (s *Service) ResolveWorkspaceID(ctx context.Context) (string, error) {
	if s.WorkspaceID != "" {
		if err := resolve.ValidateID(s.WorkspaceID, "workspace_id"); err != nil {
			return "", err
		}
		return s.WorkspaceID, nil
	}
	if wsID, ok := s.identity.cachedWorkspaceID(); ok {
		return wsID, nil
	}
	var workspaces []clockify.Workspace
	if err := s.Client.Get(ctx, "/workspaces", nil, &workspaces); err != nil {
		return "", err
	}
	if len(workspaces) == 1 {
		s.identity.storeWorkspaceID(workspaces[0].ID)
		return workspaces[0].ID, nil
	}
	if len(workspaces) == 0 {
		return "", fmt.Errorf("no workspaces available for this API key")
	}
	return "", fmt.Errorf("multiple workspaces found; set CLOCKIFY_WORKSPACE_ID")
}

// GetWorkspace handles clockify_workspace_settings: it reads the pinned
// workspace's settings.
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
