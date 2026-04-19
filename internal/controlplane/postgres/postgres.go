//go:build postgres

// Package postgres provides a Postgres-backed controlplane.Store
// implementation. It is selected at runtime by the DSN scheme
// ("postgres://..." or "postgresql://...") and requires the binary to
// be built with -tags=postgres — the default build deliberately omits
// pgx to keep the top-level go.mod stdlib-only (ADR 0001).
//
// Registration happens in init.go. Use the parent package's Open
// function (controlplane.Open) to construct a store; it dispatches to
// the opener registered here.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apet97/go-clockify/internal/controlplane"
)

// Store is the pgx-backed controlplane.Store implementation. The pool
// is owned by the store and released on Close.
type Store struct {
	pool *pgxpool.Pool
}

// open is the factory registered with controlplane.RegisterOpener. It
// parses the DSN, builds a pool, applies embedded migrations, and
// returns the store. Options configured for DevFileStore (WithAuditCap)
// do not apply to Postgres — retention is handled via RetainAudit (B2)
// rather than an in-memory cap.
func open(dsn string, _ ...controlplane.Option) (controlplane.Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("controlplane/postgres: parse DSN: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("controlplane/postgres: new pool: %w", err)
	}
	if err := applyMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool. Subsequent method calls will return errors.
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

const storeOpTimeout = 15 * time.Second

func (s *Store) Tenant(id string) (controlplane.TenantRecord, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		SELECT id, credential_ref_id, workspace_id, base_url, timezone,
		       policy_mode, deny_tools, deny_groups, allow_groups, metadata
		  FROM tenants WHERE id = $1`, id)
	var (
		rec        controlplane.TenantRecord
		denyTools  []byte
		denyGroups []byte
		allowGrps  []byte
		metadata   []byte
	)
	if err := row.Scan(&rec.ID, &rec.CredentialRefID, &rec.WorkspaceID, &rec.BaseURL,
		&rec.Timezone, &rec.PolicyMode, &denyTools, &denyGroups, &allowGrps, &metadata); err != nil {
		return controlplane.TenantRecord{}, false
	}
	_ = json.Unmarshal(denyTools, &rec.DenyTools)
	_ = json.Unmarshal(denyGroups, &rec.DenyGroups)
	_ = json.Unmarshal(allowGrps, &rec.AllowGroups)
	_ = json.Unmarshal(metadata, &rec.Metadata)
	return rec, true
}

func (s *Store) PutTenant(rec controlplane.TenantRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	denyTools, _ := json.Marshal(sliceOrEmpty(rec.DenyTools))
	denyGroups, _ := json.Marshal(sliceOrEmpty(rec.DenyGroups))
	allowGroups, _ := json.Marshal(sliceOrEmpty(rec.AllowGroups))
	metadata, _ := json.Marshal(mapOrEmpty(rec.Metadata))
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tenants (id, credential_ref_id, workspace_id, base_url, timezone,
		                    policy_mode, deny_tools, deny_groups, allow_groups, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			credential_ref_id = EXCLUDED.credential_ref_id,
			workspace_id      = EXCLUDED.workspace_id,
			base_url          = EXCLUDED.base_url,
			timezone          = EXCLUDED.timezone,
			policy_mode       = EXCLUDED.policy_mode,
			deny_tools        = EXCLUDED.deny_tools,
			deny_groups       = EXCLUDED.deny_groups,
			allow_groups      = EXCLUDED.allow_groups,
			metadata          = EXCLUDED.metadata`,
		rec.ID, rec.CredentialRefID, rec.WorkspaceID, rec.BaseURL, rec.Timezone,
		rec.PolicyMode, denyTools, denyGroups, allowGroups, metadata)
	return err
}

func (s *Store) CredentialRef(id string) (controlplane.CredentialRef, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		SELECT id, backend, reference, workspace_id, base_url, metadata, modified_at
		  FROM credential_refs WHERE id = $1`, id)
	var (
		rec      controlplane.CredentialRef
		metadata []byte
		modified *time.Time
	)
	if err := row.Scan(&rec.ID, &rec.Backend, &rec.Reference, &rec.Workspace,
		&rec.BaseURL, &metadata, &modified); err != nil {
		return controlplane.CredentialRef{}, false
	}
	_ = json.Unmarshal(metadata, &rec.Metadata)
	if modified != nil {
		rec.ModifiedAt = modified.UTC()
	}
	return rec, true
}

func (s *Store) PutCredentialRef(rec controlplane.CredentialRef) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	metadata, _ := json.Marshal(mapOrEmpty(rec.Metadata))
	var modified any
	if !rec.ModifiedAt.IsZero() {
		modified = rec.ModifiedAt.UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO credential_refs (id, backend, reference, workspace_id, base_url, metadata, modified_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET
			backend      = EXCLUDED.backend,
			reference    = EXCLUDED.reference,
			workspace_id = EXCLUDED.workspace_id,
			base_url     = EXCLUDED.base_url,
			metadata     = EXCLUDED.metadata,
			modified_at  = EXCLUDED.modified_at`,
		rec.ID, rec.Backend, rec.Reference, rec.Workspace, rec.BaseURL, metadata, modified)
	return err
}

func (s *Store) Session(id string) (controlplane.SessionRecord, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, subject, transport, protocol_version, client_name,
		       client_version, created_at, expires_at, last_seen_at, workspace_id,
		       clockify_base_url, session_affinity_id
		  FROM sessions WHERE id = $1`, id)
	var rec controlplane.SessionRecord
	if err := row.Scan(&rec.ID, &rec.TenantID, &rec.Subject, &rec.Transport,
		&rec.ProtocolVersion, &rec.ClientName, &rec.ClientVersion, &rec.CreatedAt,
		&rec.ExpiresAt, &rec.LastSeenAt, &rec.WorkspaceID, &rec.ClockifyBaseURL,
		&rec.SessionAffinityID); err != nil {
		return controlplane.SessionRecord{}, false
	}
	rec.CreatedAt = rec.CreatedAt.UTC()
	rec.ExpiresAt = rec.ExpiresAt.UTC()
	rec.LastSeenAt = rec.LastSeenAt.UTC()
	return rec, true
}

func (s *Store) PutSession(rec controlplane.SessionRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, tenant_id, subject, transport, protocol_version,
		                     client_name, client_version, created_at, expires_at,
		                     last_seen_at, workspace_id, clockify_base_url,
		                     session_affinity_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			tenant_id           = EXCLUDED.tenant_id,
			subject             = EXCLUDED.subject,
			transport           = EXCLUDED.transport,
			protocol_version    = EXCLUDED.protocol_version,
			client_name         = EXCLUDED.client_name,
			client_version      = EXCLUDED.client_version,
			created_at          = EXCLUDED.created_at,
			expires_at          = EXCLUDED.expires_at,
			last_seen_at        = EXCLUDED.last_seen_at,
			workspace_id        = EXCLUDED.workspace_id,
			clockify_base_url   = EXCLUDED.clockify_base_url,
			session_affinity_id = EXCLUDED.session_affinity_id`,
		rec.ID, rec.TenantID, rec.Subject, rec.Transport, rec.ProtocolVersion,
		rec.ClientName, rec.ClientVersion, rec.CreatedAt.UTC(), rec.ExpiresAt.UTC(),
		rec.LastSeenAt.UTC(), rec.WorkspaceID, rec.ClockifyBaseURL, rec.SessionAffinityID)
	return err
}

func (s *Store) DeleteSession(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

// RetainAudit deletes audit events older than maxAge. Returns the row
// count removed. maxAge <= 0 is a no-op (matches DevFileStore so the
// reaper can safely drive both backends through the same interface
// method).
func (s *Store) RetainAudit(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-maxAge).UTC()
	tag, err := s.pool.Exec(ctx, `DELETE FROM audit_events WHERE at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (s *Store) AppendAuditEvent(event controlplane.AuditEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	resourceIDs, _ := json.Marshal(mapOrEmpty(event.ResourceIDs))
	metadata, _ := json.Marshal(mapOrEmpty(event.Metadata))
	externalID := event.ID
	if externalID == "" {
		externalID = fmt.Sprintf("%d-%s-%s", event.At.UnixNano(), event.SessionID, event.Tool)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_events (external_id, at, tenant_id, subject, session_id,
		                        transport, tool, action, outcome, reason,
		                        resource_ids, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (external_id) DO NOTHING`,
		externalID, event.At.UTC(), event.TenantID, event.Subject, event.SessionID,
		event.Transport, event.Tool, event.Action, event.Outcome, event.Reason,
		resourceIDs, metadata)
	return err
}

// sliceOrEmpty guarantees JSON-encodes to `[]` rather than `null` for
// nil inputs, keeping the `deny_tools JSONB NOT NULL` column honest.
func sliceOrEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// mapOrEmpty mirrors sliceOrEmpty for the map-shaped JSONB columns.
func mapOrEmpty(v map[string]string) map[string]string {
	if v == nil {
		return map[string]string{}
	}
	return v
}

// Compile-time assertion that Store satisfies controlplane.Store.
// Keeps the two from drifting without failing tests.
var _ controlplane.Store = (*Store)(nil)
