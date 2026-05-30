package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
)

// ListTags handles clockify_tags_list: it lists tags in the pinned workspace
// with pagination.
func (s *Service) ListTags(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	path, err := paths.Workspace(wsID, "tags")
	if err != nil {
		return ToolResult{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query, err := tagListQueryValues(args, page, pageSize)
	if err != nil {
		return ToolResult{}, err
	}
	var out []clockify.Tag
	if err := s.Client.GetValues(ctx, path, query, &out); err != nil {
		return ToolResult{}, err
	}
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	return ok("clockify_tags_list", out, emptyListMeta(meta, "clockify_tags_create")), nil
}

func tagListQuery(args map[string]any, page, pageSize int) map[string]string {
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	addStringQuery(query, args, "name", "name")
	addBoolQuery(query, args, "archived", "archived")
	addBoolQuery(query, args, "strict_name_search", "strict-name-search")
	addStringQuery(query, args, "sort_column", "sort-column")
	addStringQuery(query, args, "sort_order", "sort-order")
	return query
}

func tagListQueryValues(args map[string]any, page, pageSize int) (url.Values, error) {
	values := valuesFromQueryMap(tagListQuery(args, page, pageSize))
	if err := addRepeatedStringQuery(values, args, "excluded_ids", "excluded-ids"); err != nil {
		return nil, err
	}
	return values, nil
}

// CreateTag handles clockify_tags_create: it creates a tag by name in the pinned
// workspace.
func (s *Service) CreateTag(ctx context.Context, args map[string]any) (ToolResult, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ToolResult{}, fmt.Errorf("name is required")
	}
	if len(name) > 100 {
		return ToolResult{}, fmt.Errorf("tag name cannot be longer than 100 characters")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	payload := map[string]any{"name": name}
	if dryrun.Enabled(args) {
		return ok("clockify_tags_create", dryrun.Preview("clockify_tags_create", payload), map[string]any{"workspaceId": wsID}), nil
	}
	path, err := paths.Workspace(wsID, "tags")
	if err != nil {
		return ToolResult{}, err
	}

	var tag clockify.Tag
	if err := s.Client.Post(ctx, path, payload, &tag); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_tags_create", tag, map[string]any{"workspaceId": wsID}), nil
}

// GetTag fetches a single tag by ID or exact name.
func (s *Service) GetTag(ctx context.Context, args map[string]any) (ToolResult, error) {
	tagRef := strings.TrimSpace(stringArg(args, "tag"))
	if tagRef == "" {
		return ToolResult{}, fmt.Errorf("tag is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	tagID, err := s.resolveTagID(ctx, wsID, tagRef)
	if err != nil {
		return ToolResult{}, err
	}
	tagPath, err := paths.Workspace(wsID, "tags", tagID)
	if err != nil {
		return ToolResult{}, err
	}
	var out clockify.Tag
	if err := s.Client.Get(ctx, tagPath, nil, &out); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_tags_get", out, map[string]any{"workspaceId": wsID, "tagId": tagID}), nil
}

// UpdateTag performs a fetch-then-merge update of a tag.
// Clockify's PUT /workspaces/{ws}/tags/{id} is a full replacement;
// we GET the existing tag, layer caller changes on top, then PUT the
// merged shape back. The archived boolean can be toggled explicitly.
func (s *Service) UpdateTag(ctx context.Context, args map[string]any) (ToolResult, error) {
	tagRef := strings.TrimSpace(stringArg(args, "tag"))
	if tagRef == "" {
		return ToolResult{}, fmt.Errorf("tag is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	tagID, err := s.resolveTagID(ctx, wsID, tagRef)
	if err != nil {
		return ToolResult{}, err
	}
	tagPath, err := paths.Workspace(wsID, "tags", tagID)
	if err != nil {
		return ToolResult{}, err
	}

	var existing clockify.Tag
	if err := s.Client.Get(ctx, tagPath, nil, &existing); err != nil {
		return ToolResult{}, err
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
		return ToolResult{
			OK:     true,
			Action: "clockify_tags_update",
			Data:   dryrun.Preview("clockify_tags_update", args),
			Meta:   meta,
		}, nil
	}

	payload := tagPutPayload(existing)
	var updated clockify.Tag
	if err := s.Client.Put(ctx, tagPath, payload, &updated); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_tags_update", updated, meta), nil
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
func (s *Service) DeleteTag(ctx context.Context, args map[string]any) (ToolResult, error) {
	tagRef := strings.TrimSpace(stringArg(args, "tag"))
	if tagRef == "" {
		return ToolResult{}, fmt.Errorf("tag is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	tagID, err := s.resolveTagID(ctx, wsID, tagRef)
	if err != nil {
		return ToolResult{}, err
	}
	tagPath, err := paths.Workspace(wsID, "tags", tagID)
	if err != nil {
		return ToolResult{}, err
	}

	var existing clockify.Tag
	if err := s.Client.Get(ctx, tagPath, nil, &existing); err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		return ToolResult{
			OK:     true,
			Action: "clockify_tags_delete",
			Data:   dryrun.WrapResult(existing, "clockify_tags_delete"),
			Meta:   map[string]any{"workspaceId": wsID, "tagId": tagID},
		}, nil
	}

	if err := s.Client.Delete(ctx, tagPath); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_tags_delete", map[string]any{"deleted": true, "tagId": tagID}, map[string]any{"workspaceId": wsID}), nil
}
