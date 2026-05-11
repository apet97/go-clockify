package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
)

func (s *Service) ListTags(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "tags")
	if err != nil {
		return ResultEnvelope{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	var out []clockify.Tag
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ResultEnvelope{}, err
	}
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	return ok("clockify_list_tags", out, meta), nil
}

func (s *Service) CreateTag(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	if len(name) > 100 {
		return ResultEnvelope{}, fmt.Errorf("tag name cannot be longer than 100 characters")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	payload := map[string]any{"name": name}
	if dryrun.Enabled(args) {
		return ok("clockify_create_tag", dryrun.Preview("clockify_create_tag", payload), map[string]any{"workspaceId": wsID}), nil
	}
	path, err := paths.Workspace(wsID, "tags")
	if err != nil {
		return ResultEnvelope{}, err
	}

	var tag clockify.Tag
	if err := s.Client.Post(ctx, path, payload, &tag); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_tag", tag, map[string]any{"workspaceId": wsID}), nil
}

// GetTag fetches a single tag by ID or exact name.
func (s *Service) GetTag(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	tagRef := strings.TrimSpace(stringArg(args, "tag"))
	if tagRef == "" {
		return ResultEnvelope{}, fmt.Errorf("tag is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	tagID, err := s.resolveTagID(ctx, wsID, tagRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	tagPath, err := paths.Workspace(wsID, "tags", tagID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out clockify.Tag
	if err := s.Client.Get(ctx, tagPath, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_tag", out, map[string]any{"workspaceId": wsID, "tagId": tagID}), nil
}

// UpdateTag performs a fetch-then-merge update of a tag.
// Clockify's PUT /workspaces/{ws}/tags/{id} is a full replacement;
// we GET the existing tag, layer caller changes on top, then PUT the
// merged shape back. The archived boolean can be toggled explicitly.
func (s *Service) UpdateTag(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	tagRef := strings.TrimSpace(stringArg(args, "tag"))
	if tagRef == "" {
		return ResultEnvelope{}, fmt.Errorf("tag is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	tagID, err := s.resolveTagID(ctx, wsID, tagRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	tagPath, err := paths.Workspace(wsID, "tags", tagID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing clockify.Tag
	if err := s.Client.Get(ctx, tagPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	changedFields := make([]string, 0, 2)
	if v := stringArg(args, "name"); v != "" && v != existing.Name {
		existing.Name = v
		changedFields = append(changedFields, "name")
	}
	if v, ok := args["archived"].(bool); ok && v != existing.Archived {
		existing.Archived = v
		changedFields = append(changedFields, "archived")
	}

	meta := map[string]any{
		"workspaceId":   wsID,
		"tagId":         tagID,
		"changedFields": changedFields,
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_update_tag",
			Data:   dryrun.Preview("clockify_update_tag", args),
			Meta:   meta,
		}, nil
	}

	payload := tagPutPayload(existing)
	var updated clockify.Tag
	if err := s.Client.Put(ctx, tagPath, payload, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_tag", updated, meta), nil
}

// tagPutPayload builds the full-replacement body for PUT /workspaces/{ws}/tags/{id}.
// Clockify requires name in the body and uses full-replacement semantics.
// archived must be sent explicitly (even false) since Go's zero value would
// otherwise be silently omitted with omitempty — Tag.Archived has no omitempty tag.
func tagPutPayload(t clockify.Tag) map[string]any {
	return map[string]any{
		"name":     t.Name,
		"archived": t.Archived,
	}
}

// DeleteTag deletes a tag by ID or exact name.
// Clockify's DELETE /workspaces/{ws}/tags/{id} works directly on active tags —
// no archive step is required (unlike clients). Supports dry_run.
func (s *Service) DeleteTag(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	tagRef := strings.TrimSpace(stringArg(args, "tag"))
	if tagRef == "" {
		return ResultEnvelope{}, fmt.Errorf("tag is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	tagID, err := s.resolveTagID(ctx, wsID, tagRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	tagPath, err := paths.Workspace(wsID, "tags", tagID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing clockify.Tag
	if err := s.Client.Get(ctx, tagPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_tag",
			Data:   dryrun.WrapResult(existing, "clockify_delete_tag"),
			Meta:   map[string]any{"workspaceId": wsID, "tagId": tagID},
		}, nil
	}

	if err := s.Client.Delete(ctx, tagPath); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_tag", map[string]any{"deleted": true, "tagId": tagID}, map[string]any{"workspaceId": wsID}), nil
}
