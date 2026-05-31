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
// docs/policy/production-tool-scope.md: clockify_entries_update,
// clockify_entries_delete, and the entry_id branch of
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

// CurrentUser handles clockify_users_profile: it returns the current Clockify
// user.
func (s *Service) CurrentUser(ctx context.Context) (ToolResult, error) {
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_users_profile", userViewFromUser(user), nil), nil
}

func (s *Service) getCurrentUser(ctx context.Context) (clockify.User, error) {
	if user, ok := s.identity.cachedUserSnapshot(); ok {
		return user, nil
	}
	var user clockify.User
	if err := s.Client.Get(ctx, "/user", nil, &user); err != nil {
		return clockify.User{}, err
	}
	s.identity.storeUser(user)
	return user, nil
}

// ListUsers handles clockify_users_list: it lists users in the pinned workspace
// with pagination.
func (s *Service) ListUsers(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	path, err := paths.Workspace(wsID, "users")
	if err != nil {
		return ToolResult{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	addStringQuery(query, args, "email", "email")
	addStringQuery(query, args, "project_id", "project-id")
	addStringQuery(query, args, "status", "status")
	addStringQuery(query, args, "account_statuses", "account-statuses")
	addStringQuery(query, args, "name", "name")
	addStringQuery(query, args, "sort_column", "sort-column")
	addStringQuery(query, args, "sort_order", "sort-order")
	addStringQuery(query, args, "memberships", "memberships")
	addBoolQuery(query, args, "include_roles", "include-roles")
	var users []clockify.User
	if err := s.Client.Get(ctx, path, query, &users); err != nil {
		return ToolResult{}, err
	}
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(users),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	return ok("clockify_users_list", compactUserViewsFromUsers(users), emptyListMeta(meta, "clockify_users_invite")), nil
}
