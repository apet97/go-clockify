package tools

import (
	"context"
	"fmt"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
)

func (s *Service) ListWorkspaces(ctx context.Context) (ResultEnvelope, error) {
	var workspaces []clockify.Workspace
	if err := s.Client.Get(ctx, "/workspaces", nil, &workspaces); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_workspaces", workspaces, map[string]any{"count": len(workspaces)}), nil
}

func (s *Service) ResolveWorkspaceID(ctx context.Context) (string, error) {
	if s.WorkspaceID != "" {
		return s.WorkspaceID, nil
	}
	s.mu.Lock()
	if s.cachedWSID != "" {
		wsID := s.cachedWSID
		s.mu.Unlock()
		return wsID, nil
	}
	s.mu.Unlock()
	var workspaces []clockify.Workspace
	if err := s.Client.Get(ctx, "/workspaces", nil, &workspaces); err != nil {
		return "", err
	}
	if len(workspaces) == 1 {
		s.mu.Lock()
		s.cachedWSID = workspaces[0].ID
		s.mu.Unlock()
		return workspaces[0].ID, nil
	}
	if len(workspaces) == 0 {
		return "", fmt.Errorf("no workspaces available for this API key")
	}
	return "", fmt.Errorf("multiple workspaces found; set CLOCKIFY_WORKSPACE_ID")
}

func (s *Service) GetWorkspace(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	// paths.Workspace runs resolve.ValidateID and url.PathEscape per
	// segment. Defence-in-depth: config.Load already validates an
	// env-supplied CLOCKIFY_WORKSPACE_ID, but ResolveWorkspaceID can
	// also return an auto-detected ID from a /workspaces response.
	path, err := paths.Workspace(wsID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out map[string]any
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_workspace", out, map[string]any{"workspaceId": wsID}), nil
}
