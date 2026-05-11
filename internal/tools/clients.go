package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/paths"
)

func (s *Service) ListClients(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "clients")
	if err != nil {
		return ResultEnvelope{}, err
	}
	page, pageSize := paginationFromArgs(args)
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	var out []clockify.ClientEntity
	if err := s.Client.Get(ctx, path, query, &out); err != nil {
		return ResultEnvelope{}, err
	}
	meta := addPaginationMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(out),
		"page":        page,
		"pageSize":    pageSize,
	}, args, page, pageSize)
	return ok("clockify_list_clients", out, meta), nil
}

func (s *Service) GetClient(ctx context.Context, clientRef string) (ResultEnvelope, error) {
	if clientRef == "" {
		return ResultEnvelope{}, fmt.Errorf("client is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	clientID, err := s.resolveClientID(ctx, wsID, clientRef)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "clients", clientID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var out clockify.ClientEntity
	if err := s.Client.Get(ctx, path, nil, &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_client", out, map[string]any{"workspaceId": wsID, "clientId": clientID}), nil
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
	payload := map[string]any{"name": name}
	if boolArg(args, "dry_run") {
		return ok("clockify_create_client", dryrunPreviewPayload("clockify_create_client", payload), map[string]any{"workspaceId": wsID}), nil
	}
	path, err := paths.Workspace(wsID, "clients")
	if err != nil {
		return ResultEnvelope{}, err
	}

	var client clockify.ClientEntity
	if err := s.Client.Post(ctx, path, payload, &client); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_client", client, map[string]any{"workspaceId": wsID}), nil
}
