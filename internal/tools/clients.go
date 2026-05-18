package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/paths"
)

func clientListQuery(args map[string]any, page, pageSize int) map[string]string {
	query := map[string]string{
		"page":      strconv.Itoa(page),
		"page-size": strconv.Itoa(pageSize),
	}
	addStringQuery(query, args, "name", "name")
	addBoolQuery(query, args, "archived", "archived")
	addStringQuery(query, args, "address", "address")
	addStringQuery(query, args, "note", "note")
	addStringQuery(query, args, "sort_column", "sort-column")
	addStringQuery(query, args, "sort_order", "sort-order")
	return query
}

func (s *Service) GetClientWithArgs(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	clientRef := stringArg(args, "client")
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
	view, financialMeta := s.enrichClientView(ctx, wsID, out, args, false)
	return ok("clockify_clients_get", view, withFinancialMeta(map[string]any{"workspaceId": wsID, "clientId": clientID}, financialMeta)), nil
}

// UpdateClient performs a fetch-then-merge update of a client.
// Clockify's PUT /clients/{id} is a full replacement (omitted fields
// get nulled server-side), so we GET the existing client, layer
// caller-provided changes over the fetched object, and PUT the
// complete merged shape back. Caller-supplied empty strings are
// treated as "do not change" (use the dedicated `archived` boolean
// flag to flip archival state).
func (s *Service) UpdateClient(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	clientRef := stringArg(args, "client")
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
	clientPath, err := paths.Workspace(wsID, "clients", clientID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing clockify.ClientEntity
	if err := s.Client.Get(ctx, clientPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	changedFields := make([]string, 0, 5)
	if v := stringArg(args, "name"); v != "" && v != existing.Name {
		existing.Name = v
		changedFields = append(changedFields, "name")
	}
	if v := stringArg(args, "address"); v != "" && v != existing.Address {
		existing.Address = v
		changedFields = append(changedFields, "address")
	}
	if v := stringArg(args, "email"); v != "" && v != existing.Email {
		existing.Email = v
		changedFields = append(changedFields, "email")
	}
	if v := stringArg(args, "note"); v != "" && v != existing.Note {
		existing.Note = v
		changedFields = append(changedFields, "note")
	}
	if ccEmails, ok, err := strictStringSliceArg(args, "cc_emails"); err != nil {
		return ResultEnvelope{}, err
	} else if ok {
		existing.CCEmails = ccEmails
		changedFields = append(changedFields, "cc_emails")
	}
	if v := stringArg(args, "currency_id"); v != "" && v != existing.CurrencyID {
		existing.CurrencyID = v
		changedFields = append(changedFields, "currency_id")
	}
	if v, ok := args["archived"].(bool); ok && v != existing.Archived {
		existing.Archived = v
		changedFields = append(changedFields, "archived")
	}

	meta := map[string]any{
		"workspaceId":   wsID,
		"clientId":      clientID,
		"changedFields": changedFields,
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_clients_update",
			Data:   dryrun.Preview("clockify_clients_update", args),
			Meta:   meta,
		}, nil
	}

	payload := clientPutPayload(existing)
	var updated clockify.ClientEntity
	query := map[string]string{}
	addBoolQuery(query, args, "archive_projects", "archive-projects")
	addBoolQuery(query, args, "mark_tasks_as_done", "mark-tasks-as-done")
	if err := s.Client.PutWithQuery(ctx, clientPath, query, payload, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	view, financialMeta := s.enrichClientView(ctx, wsID, updated, args, false)
	return ok("clockify_clients_update", view, withFinancialMeta(meta, financialMeta)), nil
}

// clientPutPayload builds the full-replacement body for PUT /clients/{id}.
// Clockify requires `name` in the body for the PUT validator and treats
// omitted optional fields as "clear me", so every non-empty field on the
// fetched entity is forwarded explicitly to preserve current state.
func clientPutPayload(c clockify.ClientEntity) map[string]any {
	p := map[string]any{
		"name":     c.Name,
		"archived": c.Archived,
	}
	if c.Address != "" {
		p["address"] = c.Address
	}
	if c.Email != "" {
		p["email"] = c.Email
	}
	if c.Note != "" {
		p["note"] = c.Note
	}
	if c.CCEmails != nil {
		p["ccEmails"] = c.CCEmails
	}
	if c.CurrencyCode != "" {
		p["currencyCode"] = c.CurrencyCode
	}
	if c.CurrencyID != "" {
		p["currencyId"] = c.CurrencyID
	}
	return p
}

// DeleteClient archives the client if it is still active, then deletes
// it. Clockify rejects DELETE on active clients (the active rule is the
// same as for projects), and the PUT archive validator additionally
// requires the existing name in the body. The implementation mirrors
// the rawArchiveAndDeleteClient cleanup helper in tests/.
func (s *Service) DeleteClient(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	clientRef := stringArg(args, "client")
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
	clientPath, err := paths.Workspace(wsID, "clients", clientID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing clockify.ClientEntity
	if err := s.Client.Get(ctx, clientPath, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_clients_delete",
			Data:   dryrun.WrapResult(existing, "clockify_clients_delete"),
			Meta:   map[string]any{"workspaceId": wsID, "clientId": clientID},
		}, nil
	}

	projects, err := s.listAllProjects(ctx, map[string]any{"clients": []any{clientID}, "archived": false})
	if err == nil && len(projects) > 0 {
		return ResultEnvelope{}, fmt.Errorf("client has %d active projects; reassign or archive them first", len(projects))
	}

	if !existing.Archived {
		archivePayload := map[string]any{"name": existing.Name, "archived": true}
		var archived clockify.ClientEntity
		if err := s.Client.Put(ctx, clientPath, archivePayload, &archived); err != nil {
			return ResultEnvelope{}, fmt.Errorf("archive client %s before delete: %w", clientID, err)
		}
	}

	if err := s.Client.Delete(ctx, clientPath); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_clients_delete", map[string]any{"deleted": true, "clientId": clientID}, map[string]any{"workspaceId": wsID}), nil
}
