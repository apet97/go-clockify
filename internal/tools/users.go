package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
)

// requireCurrentUserEntry enforces the personal-time-tracking
// ownership contract documented in
// docs/policy/production-tool-scope.md: clockify_update_entry,
// clockify_delete_entry, and the entry_id branch of
// clockify_find_and_update_entry must reject any entry that is
// not owned by the API key's user. The handlers fetch entries via
// the admin path /workspaces/{ws}/time-entries/{id}, which an
// elevated key can read across users — the policy gate alone is
// not sufficient.
//
// Fail-closed posture: the live Clockify API populates userId on
// every time entry (see internal/clockify/models.go TimeEntry and
// the live schema-diff test tests/e2e_live_schema_test.go). An
// entry returned without userId is anomalous — possibly an
// upstream regression, possibly a deliberate API change, possibly
// a malicious proxy stripping the field. Either way the handler
// cannot prove ownership, so it must refuse the mutation. The
// alternative ("permissive on empty userId") would silently allow
// cross-user mutation in any scenario where the field is missing
// — exactly the threat model we are trying to defeat.
func (s *Service) requireCurrentUserEntry(ctx context.Context, entry clockify.TimeEntry) error {
	currentUser, err := s.getCurrentUser(ctx)
	if err != nil {
		return err
	}
	if entry.UserID == "" {
		return fmt.Errorf("permission denied: time entry %s has no userId; refusing to mutate ambiguous ownership", entry.ID)
	}
	if entry.UserID != currentUser.ID {
		return fmt.Errorf("permission denied: time entry %s is not owned by current user", entry.ID)
	}
	return nil
}

func (s *Service) WhoAmI(ctx context.Context) (ResultEnvelope, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	resolvedWorkspaceID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	view := userViewFromUser(user)
	return ok("clockify_whoami", IdentityData{User: user, UserView: &view, WorkspaceID: resolvedWorkspaceID}, nil), nil
}

func (s *Service) CurrentUser(ctx context.Context) (ResultEnvelope, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_current_user", userViewFromUser(user), nil), nil
}

func (s *Service) getCurrentUser(ctx context.Context) (clockify.User, error) {
	s.mu.RLock()
	if s.cachedUser != nil {
		u := *s.cachedUser
		s.mu.RUnlock()
		return u, nil
	}
	s.mu.RUnlock()
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
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(users),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	return ok("clockify_list_users", userViewsFromUsers(users), meta), nil
}
