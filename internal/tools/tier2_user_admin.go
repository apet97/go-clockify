package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "user_admin",
		Description: "User and group management",
		Keywords:    []string{"user", "group", "role", "admin", "deactivate", "permission"},
		Builder:     userAdminHandlers,
	})
}

func userAdminHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List user groups (RO)
		{
			Tool: toolRO("clockify_list_user_groups", "List user groups in the workspace", map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
					"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
				},
			}),
			ReadOnlyHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListUserGroups(ctx, args)
			},
		},
		// 2. Create user group (RW)
		{
			Tool: toolRW("clockify_create_user_group", "Create a new user group", map[string]any{
				"type":     "object",
				"required": []string{"name"},
				"properties": map[string]any{
					"name": map[string]any{"type": "string", "description": "Name for the new user group"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.CreateUserGroup(ctx, args)
			},
		},
		// 3. Update user group (RW)
		{
			Tool: toolRW("clockify_update_user_group", "Update a user group name", map[string]any{
				"type":     "object",
				"required": []string{"group_id", "name"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID"},
					"name":     map[string]any{"type": "string", "description": "New name for the user group"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateUserGroup(ctx, args)
			},
		},
		// 4. Delete user group (destructive)
		{
			Tool: toolDestructive("clockify_delete_user_group", "Delete a user group", map[string]any{
				"type":     "object",
				"required": []string{"group_id"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID to delete"},
					"dry_run":  map[string]any{"type": "boolean", "description": "Preview without making changes"},
				},
			}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeleteUserGroup(ctx, args)
			},
		},
		// 5. Add user to group (RW)
		{
			Tool: toolRW("clockify_add_user_to_group", "Add a user to a user group", map[string]any{
				"type":     "object",
				"required": []string{"group_id", "user_id"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID"},
					"user_id":  map[string]any{"type": "string", "description": "User ID to add"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.AddUserToGroup(ctx, args)
			},
		},
		// 6. Remove user from group (destructive)
		{
			Tool: toolDestructive("clockify_remove_user_from_group", "Remove a user from a user group", map[string]any{
				"type":     "object",
				"required": []string{"group_id", "user_id"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID"},
					"user_id":  map[string]any{"type": "string", "description": "User ID to remove"},
					"dry_run":  map[string]any{"type": "boolean", "description": "Preview without making changes"},
				},
			}),
			ReadOnlyHint:    false,
			DestructiveHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.RemoveUserFromGroup(ctx, args)
			},
		},
		// 7. Update user role (RW)
		{
			Tool: toolRW("clockify_update_user_role", "Update a user's workspace role", map[string]any{
				"type":     "object",
				"required": []string{"user_id", "role"},
				"properties": map[string]any{
					"user_id": map[string]any{"type": "string", "description": "User ID"},
					"role": map[string]any{
						"type":        "string",
						"description": "Role to assign: WORKSPACE_ADMIN, PROJECT_MANAGER, TEAM_MANAGER, or REGULAR",
						"enum":        []string{"WORKSPACE_ADMIN", "PROJECT_MANAGER", "TEAM_MANAGER", "REGULAR"},
					},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateUserRole(ctx, args)
			},
		},
		// 8. Deactivate user (RW, confirm pattern dry-run)
		{
			Tool: toolRW("clockify_deactivate_user", "Deactivate a user in the workspace", map[string]any{
				"type":     "object",
				"required": []string{"user_id"},
				"properties": map[string]any{
					"user_id": map[string]any{"type": "string", "description": "User ID to deactivate"},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview without making changes"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.DeactivateUser(ctx, args)
			},
		},
	}
}

// ListUserGroups returns user groups for the workspace.
func (s *Service) ListUserGroups(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}

	var groups []map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/user-groups", query, &groups); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_user_groups", groups, map[string]any{
		"workspaceId": wsID,
		"count":       len(groups),
		"page":        page,
		"pageSize":    pageSize,
	}), nil
}

// CreateUserGroup creates a new user group.
func (s *Service) CreateUserGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := stringArg(args, "name")
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{"name": name}
	var result map[string]any
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/user-groups", payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_create_user_group", result, map[string]any{"workspaceId": wsID}), nil
}

// UpdateUserGroup updates a user group's name.
func (s *Service) UpdateUserGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	name := stringArg(args, "name")
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{"name": name}
	var result map[string]any
	if err := s.Client.Put(ctx, "/workspaces/"+wsID+"/user-groups/"+groupID, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_update_user_group", result, map[string]any{"workspaceId": wsID}), nil
}

// DeleteUserGroup deletes a user group. Supports dry-run (minimal fallback).
func (s *Service) DeleteUserGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_user_group",
			Data:   dryrun.MinimalResult("clockify_delete_user_group", map[string]any{"group_id": groupID}),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, "/workspaces/"+wsID+"/user-groups/"+groupID); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_delete_user_group", map[string]any{"deleted": true, "groupId": groupID}, map[string]any{"workspaceId": wsID}), nil
}

// AddUserToGroup adds a user to a user group.
func (s *Service) AddUserToGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{"userId": userID}
	var result map[string]any
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/user-groups/"+groupID+"/users", payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_add_user_to_group", result, map[string]any{
		"workspaceId": wsID,
		"groupId":     groupID,
		"userId":      userID,
	}), nil
}

// RemoveUserFromGroup removes a user from a user group. Supports dry-run (minimal fallback).
func (s *Service) RemoveUserFromGroup(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	groupID := stringArg(args, "group_id")
	if err := resolve.ValidateID(groupID, "group_id"); err != nil {
		return ResultEnvelope{}, err
	}
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_remove_user_from_group",
			Data:   dryrun.MinimalResult("clockify_remove_user_from_group", map[string]any{"group_id": groupID, "user_id": userID}),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, "/workspaces/"+wsID+"/user-groups/"+groupID+"/users/"+userID); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_remove_user_from_group", map[string]any{"removed": true, "groupId": groupID, "userId": userID}, map[string]any{"workspaceId": wsID}), nil
}

// UpdateUserRole updates a user's workspace role.
func (s *Service) UpdateUserRole(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ResultEnvelope{}, err
	}
	role := stringArg(args, "role")
	validRoles := map[string]bool{
		"WORKSPACE_ADMIN": true,
		"PROJECT_MANAGER": true,
		"TEAM_MANAGER":    true,
		"REGULAR":         true,
	}
	if !validRoles[role] {
		return ResultEnvelope{}, fmt.Errorf("role must be one of WORKSPACE_ADMIN, PROJECT_MANAGER, TEAM_MANAGER, REGULAR; got %q", role)
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	payload := map[string]any{"role": role}
	var result map[string]any
	if err := s.Client.Put(ctx, "/workspaces/"+wsID+"/users/"+userID+"/roles", payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_update_user_role", result, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
		"role":        role,
	}), nil
}

// DeactivateUser deactivates a user. Supports dry-run (confirm pattern).
func (s *Service) DeactivateUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_deactivate_user",
			Data: map[string]any{
				"dry_run": true,
				"tool":    "clockify_deactivate_user",
				"args":    map[string]any{"user_id": userID},
				"note":    "This is a dry-run preview. The user would be deactivated. No changes were made.",
			},
			Meta: map[string]any{"workspaceId": wsID, "userId": userID},
		}, nil
	}

	payload := map[string]any{"status": "INACTIVE"}
	var result map[string]any
	if err := s.Client.Put(ctx, "/workspaces/"+wsID+"/users/"+userID, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_deactivate_user", result, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
	}), nil
}
