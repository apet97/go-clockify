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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apet97/go-clockify/internal/confirmation"
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

// DoctorCheck is an explicit health probe for `clockify-mcp doctor
// --check-backends`. Opening the store has already parsed the DSN,
// connected to Postgres, and applied embedded migrations; this method
// verifies those effects are visible, that the audit write path can
// round-trip the 002_audit_phase column, and that removed schema
// leftovers are actually gone.
func (s *Store) DoctorCheck(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, storeOpTimeout)
	defer cancel()

	if err := s.pool.Ping(checkCtx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	var hasMigrationsTable bool
	if err := s.pool.QueryRow(checkCtx, `
		SELECT EXISTS (
			SELECT 1
			  FROM information_schema.tables
			 WHERE table_schema = current_schema()
			   AND table_name = 'schema_migrations'
		)`).Scan(&hasMigrationsTable); err != nil {
		return fmt.Errorf("check schema_migrations table: %w", err)
	}
	if !hasMigrationsTable {
		return fmt.Errorf("schema_migrations table does not exist")
	}

	var migration002Applied bool
	if err := s.pool.QueryRow(checkCtx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 2)`,
	).Scan(&migration002Applied); err != nil {
		return fmt.Errorf("check migration 002_audit_phase: %w", err)
	}
	// Both checks are launch-required and reported separately so the
	// operator can tell whether the binary, the database, or both
	// drifted. A previous version returned success when only one of the
	// two was true (e.g. somebody hand-applied the column without
	// recording the migration). That was too permissive: the doctor
	// report drives the public hosted launch checklist, which contracts
	// on "migration 002 row present AND phase column present".
	if !migration002Applied {
		return fmt.Errorf("migration 002_audit_phase is not recorded in schema_migrations")
	}
	var migration003Applied bool
	if err := s.pool.QueryRow(checkCtx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 3)`,
	).Scan(&migration003Applied); err != nil {
		return fmt.Errorf("check migration 003_drop_session_affinity_id: %w", err)
	}
	if !migration003Applied {
		return fmt.Errorf("migration 003_drop_session_affinity_id is not recorded in schema_migrations")
	}

	var hasAuditPhaseColumn bool
	if err := s.pool.QueryRow(checkCtx, `
		SELECT EXISTS (
			SELECT 1
			  FROM information_schema.columns
			 WHERE table_schema = current_schema()
			   AND table_name = 'audit_events'
			   AND column_name = 'phase'
		)`).Scan(&hasAuditPhaseColumn); err != nil {
		return fmt.Errorf("check audit_events.phase column: %w", err)
	}
	if !hasAuditPhaseColumn {
		return fmt.Errorf("audit_events.phase column is missing")
	}
	var hasSessionAffinityColumn bool
	if err := s.pool.QueryRow(checkCtx, `
		SELECT EXISTS (
			SELECT 1
			  FROM information_schema.columns
			 WHERE table_schema = current_schema()
			   AND table_name = 'sessions'
			   AND column_name = 'session_affinity_id'
		)`).Scan(&hasSessionAffinityColumn); err != nil {
		return fmt.Errorf("check sessions.session_affinity_id column: %w", err)
	}
	if hasSessionAffinityColumn {
		return fmt.Errorf("sessions.session_affinity_id column is still present")
	}

	externalID := fmt.Sprintf("doctor-backend-%d", time.Now().UnixNano())
	appended := false
	defer func() {
		if !appended {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
		defer cancel()
		_, _ = s.pool.Exec(cleanupCtx,
			`DELETE FROM audit_events WHERE external_id = $1`,
			externalID,
		)
	}()
	event := controlplane.AuditEvent{
		ID:        externalID,
		At:        time.Now().UTC(),
		TenantID:  "doctor",
		Subject:   "doctor",
		SessionID: "doctor",
		Transport: "doctor",
		Tool:      "doctor",
		Action:    "backend_check",
		Outcome:   "ok",
		Phase:     "outcome",
		Metadata: map[string]string{
			"source": "clockify-mcp doctor --check-backends",
		},
	}
	if err := s.AppendAuditEvent(event); err != nil {
		return fmt.Errorf("append audit health event: %w", err)
	}
	appended = true
	var phase string
	if err := s.pool.QueryRow(checkCtx,
		`SELECT phase FROM audit_events WHERE external_id = $1`,
		externalID,
	).Scan(&phase); err != nil {
		return fmt.Errorf("read audit health event: %w", err)
	}
	if phase != "outcome" {
		return fmt.Errorf("read audit health event phase %q, want outcome", phase)
	}
	if _, err := s.pool.Exec(checkCtx,
		`DELETE FROM audit_events WHERE external_id = $1`,
		externalID,
	); err != nil {
		return fmt.Errorf("delete audit health event: %w", err)
	}
	appended = false

	return nil
}

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
		       clockify_base_url
		  FROM sessions WHERE id = $1`, id)
	var rec controlplane.SessionRecord
	if err := row.Scan(&rec.ID, &rec.TenantID, &rec.Subject, &rec.Transport,
		&rec.ProtocolVersion, &rec.ClientName, &rec.ClientVersion, &rec.CreatedAt,
		&rec.ExpiresAt, &rec.LastSeenAt, &rec.WorkspaceID, &rec.ClockifyBaseURL,
	); err != nil {
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
		                     last_seen_at, workspace_id, clockify_base_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
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
			clockify_base_url   = EXCLUDED.clockify_base_url`,
		rec.ID, rec.TenantID, rec.Subject, rec.Transport, rec.ProtocolVersion,
		rec.ClientName, rec.ClientVersion, rec.CreatedAt.UTC(), rec.ExpiresAt.UTC(),
		rec.LastSeenAt.UTC(), rec.WorkspaceID, rec.ClockifyBaseURL)
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

// UseConfirmationNonce atomically records a confirmation-token nonce. The
// primary key is the same tenant/subject/session/tool/nonce tuple used by the
// in-memory store, so a replay across replicas fails on ON CONFLICT.
func (s *Store) UseConfirmationNonce(ctx context.Context, rec confirmation.ReplayRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if rec.Nonce == "" {
		return confirmation.ErrTokenMalformed
	}
	usedAt := rec.UsedAt
	if usedAt.IsZero() {
		usedAt = time.Now().UTC()
	}
	key := postgresConfirmationReplayKey(rec)
	tag, err := s.pool.Exec(ctx, `
		WITH expired AS (
			DELETE FROM confirmation_nonces WHERE expires_at <= now()
		)
		INSERT INTO confirmation_nonces
			(replay_key, nonce, tool, args_hash, tenant_id, subject, session_id, expires_at, used_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (replay_key) DO NOTHING`,
		key, rec.Nonce, rec.Tool, rec.ArgsHash, rec.Tenant, rec.Subject, rec.Session,
		rec.ExpiresAt.UTC(), usedAt.UTC(),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return confirmation.ErrTokenReplayed
	}
	return nil
}

func postgresConfirmationReplayKey(rec confirmation.ReplayRecord) string {
	return rec.Tenant + "\x00" + rec.Subject + "\x00" + rec.Session + "\x00" + rec.Tool + "\x00" + rec.Nonce
}

// auditInsertSQL is the single canonical INSERT statement used by both
// AppendAuditEvent (single-row Exec) and AppendAuditEventBatch
// (pgx.SendBatch). Keeping the SQL and column order in one place
// guarantees the two paths cannot drift on schema migrations.
//
// Migration 002_audit_phase.sql added the phase column; legacy rows
// default to ”. The ON CONFLICT (external_id) DO NOTHING clause
// matches AppendAuditEvent's idempotency contract and pairs with the
// intent/outcome external_id synthesis below.
const auditInsertSQL = `
		INSERT INTO audit_events (external_id, at, tenant_id, subject, session_id,
		                        transport, tool, action, outcome, phase, reason,
		                        resource_ids, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (external_id) DO NOTHING`

// auditInsertArgs prepares the 13 positional arguments for the INSERT
// statement. Shared by AppendAuditEvent and AppendAuditEventBatch so
// the two paths cannot drift on JSON column encoding or external_id
// synthesis.
//
// The two-phase audit emits intent + outcome with the same (At,
// SessionID, Tool) tuple, so the synthesised external_id includes
// Phase to keep ON CONFLICT (external_id) DO NOTHING from collapsing
// the pair into a single row.
func auditInsertArgs(event controlplane.AuditEvent) []any {
	resourceIDs, _ := json.Marshal(mapOrEmpty(event.ResourceIDs))
	metadata, _ := json.Marshal(mapOrEmpty(event.Metadata))
	externalID := event.ID
	if externalID == "" {
		externalID = fmt.Sprintf("%d-%s-%s-%s", event.At.UnixNano(), event.SessionID, event.Tool, event.Phase)
	}
	return []any{
		externalID, event.At.UTC(), event.TenantID, event.Subject, event.SessionID,
		event.Transport, event.Tool, event.Action, event.Outcome, event.Phase, event.Reason,
		resourceIDs, metadata,
	}
}

func (s *Store) AppendAuditEvent(event controlplane.AuditEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	_, err := s.pool.Exec(ctx, auditInsertSQL, auditInsertArgs(event)...)
	return err
}

// AppendAuditEventBatch consolidates len(events) round trips into one
// pgx.SendBatch. Each row reuses the same auditInsertSQL + ON CONFLICT
// dedupe as AppendAuditEvent, so the batched path is semantically
// indistinguishable from a sequence of single-row inserts.
//
// Errors short-circuit on first failure (matching AppendAuditEvent's
// contract; partial-success accounting belongs in the runtime layer).
// The empty-slice case is a no-op and returns nil so callers don't
// have to guard against it.
//
// ADR 0022 captures the rationale: this method is invoked only from
// the runtime-layer batchedAuditor wrapper, which gates non-strict
// outcome events through it. Intent records and fail_closed_strict
// outcomes still take the AppendAuditEvent single-row path.
func (s *Store) AppendAuditEventBatch(events []controlplane.AuditEvent) error {
	if len(events) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()

	batch := &pgx.Batch{}
	for _, event := range events {
		batch.Queue(auditInsertSQL, auditInsertArgs(event)...)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range events {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
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
var _ confirmation.ReplayStore = (*Store)(nil)
