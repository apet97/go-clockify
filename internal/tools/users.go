package tools

import (
	"context"

	"goclmcp/internal/clockify"
)

func (s *Service) WhoAmI(ctx context.Context) (ResultEnvelope, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	resolvedWorkspaceID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_whoami", IdentityData{User: user, WorkspaceID: resolvedWorkspaceID}, nil), nil
}

func (s *Service) CurrentUser(ctx context.Context) (ResultEnvelope, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_current_user", user, nil), nil
}

func (s *Service) getCurrentUser(ctx context.Context) (clockify.User, error) {
	s.mu.Lock()
	if s.cachedUser != nil {
		u := *s.cachedUser
		s.mu.Unlock()
		return u, nil
	}
	s.mu.Unlock()
	var user clockify.User
	if err := s.Client.Get(ctx, "/user", nil, &user); err != nil {
		return clockify.User{}, err
	}
	s.mu.Lock()
	s.cachedUser = &user
	s.mu.Unlock()
	return user, nil
}

func (s *Service) ListUsers(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var users []clockify.User
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/users", map[string]string{"page-size": "50"}, &users); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_users", users, map[string]any{"workspaceId": wsID, "count": len(users)}), nil
}
