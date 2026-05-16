package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func userAdminHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List user groups (RO)
		{
			Tool:         toolRO("clockify_list_user_groups", "List user groups in the workspace", userGroupListSchema()),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListUserGroups(ctx, args)
			},
		},
		// 2. Create user group (RW)
		{
			Tool: toolRW("clockify_create_user_group", "Create a new user group. Supports dry_run:true.", map[string]any{
				"type":     "object",
				"required": []string{"name"},
				"properties": map[string]any{
					"name":    map[string]any{"type": "string", "description": "Name for the new user group"},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview the user group payload without creating it"},
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
			Tool: toolDestructive("clockify_delete_user_group", "Permanently delete a user group. Admin scope; destructive; supports dry_run preview.", map[string]any{
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
		// 5. Invite user to workspace (RW + external side effect)
		{
			Tool: toolRW("clockify_invite_user", "Invite/add a user to the workspace by email. Supports dry_run:true before sending an invitation email.", map[string]any{
				"type":     "object",
				"required": []string{"email"},
				"properties": map[string]any{
					"email":      map[string]any{"type": "string", "format": "email", "description": "Email address to add to the workspace"},
					"send_email": map[string]any{"type": "boolean", "description": "Whether Clockify should send an invitation email. Defaults to true."},
					"dry_run":    map[string]any{"type": "boolean", "description": "Preview the invitation payload without adding a user or sending email"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.InviteUser(ctx, args)
			},
		},
		// 6. Add user to group (RW)
		{
			Tool: toolRW("clockify_add_user_to_group", "Add a user to a user group", map[string]any{
				"type":     "object",
				"required": []string{"group_id", "user_id"},
				"properties": map[string]any{
					"group_id": map[string]any{"type": "string", "description": "User group ID"},
					"user_id":  map[string]any{"type": "string", "description": "User ID to add"},
					"dry_run":  map[string]any{"type": "boolean", "description": "Preview adding the user without making changes"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.AddUserToGroup(ctx, args)
			},
		},
		// 7. Remove user from group (destructive)
		{
			Tool: toolDestructive("clockify_remove_user_from_group", "Remove a user from a user group. Admin and permission-change impact; destructive; supports dry_run preview.", map[string]any{
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
			Tool: toolRW("clockify_update_user_role", "Update a user's workspace role. Supports dry_run:true.", map[string]any{
				"type":     "object",
				"required": []string{"user_id", "role"},
				"properties": map[string]any{
					"user_id": map[string]any{"type": "string", "description": "User ID"},
					"role": map[string]any{
						"type":        "string",
						"description": "Role to assign: WORKSPACE_ADMIN, PROJECT_MANAGER, TEAM_MANAGER, or REGULAR",
						"enum":        []string{"WORKSPACE_ADMIN", "PROJECT_MANAGER", "TEAM_MANAGER", "REGULAR"},
					},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview the role change without applying it"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateUserRole(ctx, args)
			},
		},
		// 8. Deactivate user (RW, confirm pattern dry-run)
		{
			Tool: toolRW("clockify_deactivate_user", "Deactivate a workspace user and remove access. Supports dry_run preview.", map[string]any{
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
		// 9. List user managers (RO)
		{
			Tool: toolRO("clockify_list_user_managers", "List managers assigned to a user", map[string]any{
				"type":     "object",
				"required": []string{"user_id"},
				"properties": map[string]any{
					"user_id":     map[string]any{"type": "string", "description": "User ID"},
					"page":        map[string]any{"type": "integer", "description": "Page number (default 1)"},
					"page_size":   map[string]any{"type": "integer", "description": "Items per page (default 50)"},
					"sort_column": map[string]any{"type": "string", "enum": []string{"ID", "EMAIL", "NAME", "NAME_LOWERCASE", "ACCESS", "HOURLYRATE", "COSTRATE"}},
					"sort_order":  map[string]any{"type": "string", "enum": []string{"ASCENDING", "DESCENDING"}},
				},
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.ListUserManagers(ctx, args)
			},
		},
		// 10. Get member profile (RO)
		{
			Tool: toolRO("clockify_get_member_profile", "Get a user's workspace-scoped member profile", map[string]any{
				"type":     "object",
				"required": []string{"user_id"},
				"properties": map[string]any{
					"user_id": map[string]any{"type": "string", "description": "User ID"},
				},
			}),
			ReadOnlyHint: true, IdempotentHint: true,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.GetMemberProfile(ctx, args)
			},
		},
		// 11. Update member profile (RW)
		{
			Tool: toolRW("clockify_update_member_profile", "Update a user's workspace-scoped member profile. Supports dry_run:true.", map[string]any{
				"type":     "object",
				"required": []string{"user_id"},
				"properties": map[string]any{
					"user_id":              map[string]any{"type": "string", "description": "User ID"},
					"name":                 map[string]any{"type": "string", "description": "Profile display name"},
					"image_url":            map[string]any{"type": "string", "description": "Profile image URL"},
					"remove_profile_image": map[string]any{"type": "boolean", "description": "Remove the profile image"},
					"week_start":           map[string]any{"type": "string", "enum": memberProfileDayEnums(), "description": "First day of the week"},
					"work_capacity":        map[string]any{"type": "string", "description": "Daily work capacity, for example PT7H"},
					"working_days": map[string]any{
						"type":        "array",
						"description": "Array of working day enum strings; live Clockify rejects a JSON-encoded string here.",
						"items":       map[string]any{"type": "string", "enum": memberProfileDayEnums()},
					},
					"user_custom_fields": map[string]any{
						"type":        "array",
						"description": "Member profile custom field values",
						"items":       map[string]any{"type": "object", "additionalProperties": true},
					},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview the profile payload without applying it"},
				},
			}),
			ReadOnlyHint: false,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return s.UpdateMemberProfile(ctx, args)
			},
		},
	}
}

var validMemberProfileDays = map[string]bool{
	"MONDAY":    true,
	"TUESDAY":   true,
	"WEDNESDAY": true,
	"THURSDAY":  true,
	"FRIDAY":    true,
	"SATURDAY":  true,
	"SUNDAY":    true,
}

func memberProfileDayEnums() []string {
	return []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY"}
}

func normalizeMemberProfileDay(raw, field string) (string, error) {
	day := strings.ToUpper(strings.TrimSpace(raw))
	if day == "" {
		return "", nil
	}
	if !validMemberProfileDays[day] {
		return "", fmt.Errorf("%s must be one of MONDAY, TUESDAY, WEDNESDAY, THURSDAY, FRIDAY, SATURDAY, SUNDAY; got %q", field, raw)
	}
	return day, nil
}

func normalizeMemberProfileDays(raw []string, field string) ([]string, error) {
	days := make([]string, 0, len(raw))
	for _, item := range raw {
		day, err := normalizeMemberProfileDay(item, field)
		if err != nil {
			return nil, err
		}
		if day != "" {
			days = append(days, day)
		}
	}
	return days, nil
}

// ListUserGroups returns user groups for the workspace.
func (s *Service) ListUserGroups(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	query, page, pageSize := userGroupListQuery(args)

	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var groups []map[string]any
	if err := s.Client.Get(ctx, path, query, &groups); err != nil {
		return ResultEnvelope{}, err
	}

	return ok("clockify_list_user_groups", groups, emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(groups),
		"page":        page,
		"pageSize":    pageSize,
	}, "clockify_groups_create")), nil
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
	if dryrun.Enabled(args) {
		return ok("clockify_create_user_group", dryrunPreviewPayload("clockify_create_user_group", payload), map[string]any{"workspaceId": wsID}), nil
	}
	path, err := paths.Workspace(wsID, "user-groups")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	if gid, _ := result["id"].(string); gid != "" {
		s.emitResourceUpdateWithState(groupResourceURI(wsID, gid), result)
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
	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Put(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceUpdateWithState(groupResourceURI(wsID, groupID), result)
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

	path, err := paths.Workspace(wsID, "user-groups", groupID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceDeleted(groupResourceURI(wsID, groupID))
	return ok("clockify_delete_user_group", map[string]any{"deleted": true, "groupId": groupID}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) InviteUser(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	email := strings.TrimSpace(stringArg(args, "email"))
	if email == "" {
		return ResultEnvelope{}, fmt.Errorf("email is required")
	}
	sendEmail := true
	if v, ok := args["send_email"].(bool); ok {
		sendEmail = v
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	payload := map[string]any{"email": email}
	if dryrun.Enabled(args) {
		preview := dryrunPreviewPayload("clockify_invite_user", payload)
		preview["send_email"] = sendEmail
		return ok("clockify_invite_user", preview, map[string]any{"workspaceId": wsID, "sendEmail": sendEmail}), nil
	}
	path, err := paths.Workspace(wsID, "users")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.PostWithQuery(ctx, path, map[string]string{"send-email": strconv.FormatBool(sendEmail)}, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_invite_user", result, map[string]any{"workspaceId": wsID, "sendEmail": sendEmail}), nil
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
	path, err := paths.Workspace(wsID, "user-groups", groupID, "users")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if dryrun.Enabled(args) {
		return ok("clockify_add_user_to_group", dryrunPreviewPayload("clockify_add_user_to_group", payload), map[string]any{
			"workspaceId": wsID,
			"groupId":     groupID,
			"userId":      userID,
		}), nil
	}
	var result map[string]any
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceUpdate(ctx, groupResourceURI(wsID, groupID))
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

	path, err := paths.Workspace(wsID, "user-groups", groupID, "users", userID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceUpdate(ctx, groupResourceURI(wsID, groupID))
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
	if role == "REGULAR" {
		return ResultEnvelope{}, fmt.Errorf(
			"setting role to REGULAR strips the user's elevated grants " +
				"(WORKSPACE_ADMIN/PROJECT_MANAGER/TEAM_MANAGER) while keeping them " +
				"in the workspace as an active member; " +
				"the MCP does not yet expose a role-strip helper because the live API " +
				"requires DELETE on /v1/workspaces/{ws}/users/{userId}/roles with a " +
				"JSON body, which the internal Clockify client's Delete method does not " +
				"support today — manage this via the Clockify web UI or a direct API call; " +
				"clockify_deactivate_user is NOT a substitute: it removes the user from " +
				"the workspace entirely",
		)
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// The live Clockify API rejects PUT on this path with code 3000
	// ("Request method 'PUT' is not supported"). The contract is POST
	// with a body containing both entityId (the workspace ID) and role.
	// The REGULAR case is handled by the early-return guard above
	// (DELETE-with-body wire contract; see ADR 0020).
	payload := map[string]any{"entityId": wsID, "role": role}
	if dryrun.Enabled(args) {
		return ok("clockify_update_user_role", dryrunPreviewPayload("clockify_update_user_role", payload), map[string]any{
			"workspaceId": wsID,
			"userId":      userID,
			"role":        role,
		}), nil
	}

	// Self-modification guard: defense in depth on top of the upstream
	// 403. The Clockify web UI is the right surface for an operator to
	// change their own role; doing it via an API key risks the operator
	// stranding themselves (e.g. demoting from WORKSPACE_ADMIN to
	// TEAM_MANAGER and losing the surface to undo). Run only on the
	// real-action path so dry-run previews stay HTTP-free; the audit
	// trail records the reject before any role POST issues.
	if self, lookupErr := s.getCurrentUser(ctx); lookupErr == nil && self.ID != "" && self.ID == userID {
		return ResultEnvelope{}, fmt.Errorf(
			"refusing to change the API key owner's own workspace role at " +
				"the MCP layer; use the Clockify web UI if this is intentional " +
				"(self-modification can strip the operator's ability to undo)",
		)
	}
	path, err := paths.Workspace(wsID, "users", userID, "roles")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Post(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceUpdateWithState(userResourceURI(wsID, userID), result)
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

	// Self-deactivation guard: defense in depth on top of the upstream
	// 403. Deactivating the API key owner is a fast path to a workspace
	// lockout — the web UI undo requires a still-active admin login.
	// Skip the lookup on the dry-run preview path so previews remain
	// HTTP-free; the real action path always checks before issuing
	// the PUT.
	if self, lookupErr := s.getCurrentUser(ctx); lookupErr == nil && self.ID != "" && self.ID == userID {
		return ResultEnvelope{}, fmt.Errorf(
			"refusing to deactivate the API key owner at the MCP layer; " +
				"use the Clockify web UI if this is intentional " +
				"(self-deactivation can lock the operator out of the workspace)",
		)
	}

	payload := map[string]any{"status": "INACTIVE"}
	path, err := paths.Workspace(wsID, "users", userID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Put(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}

	s.emitResourceUpdateWithState(userResourceURI(wsID, userID), result)
	return ok("clockify_deactivate_user", result, map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
	}), nil
}

// ListUserManagers returns managers assigned to a workspace user.
func (s *Service) ListUserManagers(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	if v := strings.TrimSpace(stringArg(args, "sort_column")); v != "" {
		query["sort-column"] = v
	}
	if v := strings.TrimSpace(stringArg(args, "sort_order")); v != "" {
		query["sort-order"] = v
	}
	path, err := paths.Workspace(wsID, "users", userID, "managers")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result []map[string]any
	if err := s.Client.Get(ctx, path, query, &result); err != nil {
		return ResultEnvelope{}, err
	}
	meta := addPaginationMeta(map[string]any{"workspaceId": wsID, "userId": userID, "count": len(result)}, args, page, pageSize)
	return ok("clockify_list_user_managers", userManagerViewsFromRaw(result), meta), nil
}

// GetMemberProfile returns a workspace-scoped member profile for a user.
func (s *Service) GetMemberProfile(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "member-profile", userID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Get(ctx, path, nil, &result); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_member_profile", memberProfileViewFromRaw(result), map[string]any{"workspaceId": wsID, "userId": userID}), nil
}

// UpdateMemberProfile updates live-supported workspace member profile fields.
func (s *Service) UpdateMemberProfile(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	userID := stringArg(args, "user_id")
	if err := resolve.ValidateID(userID, "user_id"); err != nil {
		return ResultEnvelope{}, err
	}

	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// image_url and remove_profile_image are mutually exclusive — sending
	// both leaves the upstream's resulting image undefined.
	if strings.TrimSpace(stringArg(args, "image_url")) != "" && boolArg(args, "remove_profile_image") {
		return ResultEnvelope{}, fmt.Errorf("image_url and remove_profile_image are mutually exclusive: set one or the other, not both")
	}

	payload := map[string]any{}
	setIfString(payload, args, "name", "name")
	setIfString(payload, args, "image_url", "imageUrl")
	setIfBool(payload, args, "remove_profile_image", "removeProfileImage")
	setIfString(payload, args, "work_capacity", "workCapacity")
	if weekStart := stringArg(args, "week_start"); strings.TrimSpace(weekStart) != "" {
		day, err := normalizeMemberProfileDay(weekStart, "week_start")
		if err != nil {
			return ResultEnvelope{}, err
		}
		payload["weekStart"] = day
	}
	if workingDays, ok, err := strictStringSliceArg(args, "working_days"); err != nil {
		return ResultEnvelope{}, err
	} else if ok {
		days, err := normalizeMemberProfileDays(workingDays, "working_days")
		if err != nil {
			return ResultEnvelope{}, err
		}
		payload["workingDays"] = days
	}
	if customFields, ok, err := mapSliceArg(args, "user_custom_fields"); err != nil {
		return ResultEnvelope{}, err
	} else if ok {
		payload["userCustomFields"] = customFields
	}
	if len(payload) == 0 {
		return ResultEnvelope{}, fmt.Errorf("at least one member profile field must be provided")
	}

	if dryrun.Enabled(args) {
		return ok("clockify_update_member_profile", dryrunPreviewPayload("clockify_update_member_profile", payload), map[string]any{
			"workspaceId": wsID,
			"userId":      userID,
		}), nil
	}

	path, err := paths.Workspace(wsID, "member-profile", userID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var result map[string]any
	if err := s.Client.Patch(ctx, path, payload, &result); err != nil {
		return ResultEnvelope{}, err
	}
	s.emitResourceUpdateWithState(userResourceURI(wsID, userID), result)
	return ok("clockify_update_member_profile", memberProfileViewFromRaw(result), map[string]any{
		"workspaceId": wsID,
		"userId":      userID,
	}), nil
}
