package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
)

func (s *Service) ListClients(ctx context.Context) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out []clockify.ClientEntity
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/clients", map[string]string{"page-size": "50"}, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_clients", out, map[string]any{"workspaceId": wsID, "count": len(out)}), nil
}

func (s *Service) CreateClient(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var client clockify.ClientEntity
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/clients", map[string]any{"name": name}, &client); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_client", client, map[string]any{"workspaceId": wsID}), nil
}
