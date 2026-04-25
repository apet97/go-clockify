//go:build postgres && integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apet97/go-clockify/internal/controlplane"
)

// doctorChecker is the same private interface cmd/clockify-mcp uses to
// invoke DoctorCheck without the test importing the postgres package
// directly (that would force a build-tag dance for the assertion type).
type doctorChecker interface {
	DoctorCheck(context.Context) error
}

// asDoctorChecker pulls the DoctorCheck method off the opened store. It
// fails the test if the store does not implement it — which would mean
// the postgres backend silently dropped the doctor surface.
func asDoctorChecker(t *testing.T, s controlplane.Store) doctorChecker {
	t.Helper()
	c, ok := s.(doctorChecker)
	if !ok {
		t.Fatal("postgres store does not implement DoctorCheck")
	}
	return c
}

// withDirectPool returns a raw pgx pool against the shared test
// database. The caller is responsible for restoring any state it
// mutates, since the container is reused across tests in the package.
func withDirectPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn(t))
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestDoctorCheckPersistsAuditPhase is the happy path: a freshly-opened
// store has migration 002 recorded and audit_events.phase present, and
// DoctorCheck succeeds and cleans up its probe row.
func TestDoctorCheckPersistsAuditPhase(t *testing.T) {
	store := openStore(t)
	checker := asDoctorChecker(t, store)
	if err := checker.DoctorCheck(context.Background()); err != nil {
		t.Fatalf("DoctorCheck on healthy db failed: %v", err)
	}
	// Probe row must be cleaned up — no doctor-backend rows should
	// linger after a successful run.
	pool := withDirectPool(t)
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_events WHERE external_id LIKE 'doctor-backend-%'`).Scan(&count)
	if err != nil {
		t.Fatalf("count probe rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("DoctorCheck left %d probe rows; expected 0", count)
	}
}

// TestDoctorCheckRequiresMigration002Row plants the database in a state
// where audit_events.phase exists but schema_migrations row 2 is gone
// (e.g. somebody ran `ALTER TABLE` by hand without recording the
// migration). DoctorCheck must refuse — both conditions are required.
func TestDoctorCheckRequiresMigration002Row(t *testing.T) {
	store := openStore(t)
	checker := asDoctorChecker(t, store)
	pool := withDirectPool(t)
	ctx := context.Background()

	// Snapshot the row so we can restore it; the container is shared
	// with later tests.
	if _, err := pool.Exec(ctx, `DELETE FROM schema_migrations WHERE version = 2`); err != nil {
		t.Fatalf("delete row: %v", err)
	}
	t.Cleanup(func() {
		// Re-insert row 2 so subsequent tests in the package see a
		// healthy schema. The applied_at default fills itself in.
		_, _ = pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES (2) ON CONFLICT DO NOTHING`)
	})

	err := checker.DoctorCheck(ctx)
	if err == nil {
		t.Fatal("DoctorCheck succeeded with migration 002 row missing; expected failure")
	}
	if !strings.Contains(err.Error(), "migration 002_audit_phase is not recorded") {
		t.Fatalf("error %q did not name the missing migration row", err.Error())
	}
}

// TestDoctorCheckRequiresAuditPhaseColumn plants the inverse: the
// migration row exists but the phase column has been dropped (e.g. a
// destructive ALTER TABLE by an operator). DoctorCheck must refuse.
func TestDoctorCheckRequiresAuditPhaseColumn(t *testing.T) {
	store := openStore(t)
	checker := asDoctorChecker(t, store)
	pool := withDirectPool(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `ALTER TABLE audit_events DROP COLUMN IF EXISTS phase`); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	t.Cleanup(func() {
		// Recreate the column so subsequent tests in the package see a
		// healthy schema. Migration 002's body is the source of truth
		// for the column shape.
		_, _ = pool.Exec(ctx, `ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS phase TEXT NOT NULL DEFAULT ''`)
		_, _ = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_audit_events_phase ON audit_events (phase)`)
	})

	err := checker.DoctorCheck(ctx)
	if err == nil {
		t.Fatal("DoctorCheck succeeded with phase column missing; expected failure")
	}
	if !strings.Contains(err.Error(), "audit_events.phase column is missing") {
		t.Fatalf("error %q did not name the missing column", err.Error())
	}
}
