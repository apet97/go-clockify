package tools

import (
	"context"
	"strconv"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
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

func (s *Service) ListUsers(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "users")
	if err != nil {
		return ResultEnvelope{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	var users []clockify.User
	if err := s.Client.Get(ctx, path, query, &users); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_users", users, map[string]any{
		"workspaceId": wsID,
		"count":       len(users),
		"page":        page,
		"pageSize":    pageSize,
	}), nil
}
