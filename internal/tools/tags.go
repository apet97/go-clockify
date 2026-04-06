package tools

import (
	"context"
	"fmt"
	"strings"

	"goclmcp/internal/clockify"
)

func (s *Service) ListTags(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out []clockify.Tag
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/tags", map[string]string{"page-size": "50"}, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_tags", out, map[string]any{"workspaceId": wsID, "count": len(out)}), nil
}

func (s *Service) CreateTag(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var tag clockify.Tag
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/tags", map[string]any{"name": name}, &tag); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_tag", tag, map[string]any{"workspaceId": wsID}), nil
}
