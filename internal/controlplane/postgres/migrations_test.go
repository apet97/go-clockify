//go:build postgres && integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apet97/go-clockify/internal/controlplane"
)

// TestSchemaCompatGuard_RefusesFutureVersion forces the scenario that
// the applier is meant to protect against: a binary that only knows
// migrations up to version N runs against a database whose
// schema_migrations table records version N+100, planted out of band.
// applyMigrations must refuse to boot rather than silently ignore the
// unknown future schema (ADR 0011).
func TestSchemaCompatGuard_RefusesFutureVersion(t *testing.T) {
	conn := dsn(t)

	// First open runs migrations normally.
	s, err := controlplane.Open(conn)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_ = s.Close()

	// Plant a future version directly via pgx. We intentionally bypass
	// the store's interface because the whole point is to simulate a
	// newer binary having written this row in the past.
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, conn)
	if err != nil {
		t.Fatalf("pgx pool: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES (9999) ON CONFLICT DO NOTHING`); err != nil {
		pool.Close()
		t.Fatalf("plant future version: %v", err)
	}
	pool.Close()

	// Now a fresh open against the same DB must refuse to boot.
	_, err = controlplane.Open(conn)
	if err == nil {
		t.Fatal("expected Open to refuse against future schema version")
	}
	msg := err.Error()
	if !strings.Contains(msg, "refusing to start") && !strings.Contains(msg, "9999") {
		t.Fatalf("error %q did not identify the version mismatch", msg)
	}

	// Clean up so other tests in the package don't trip the same
	// guard; each test shares the container.
	pool, err = pgxpool.New(ctx, conn)
	if err != nil {
		t.Fatalf("cleanup pool: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM schema_migrations WHERE version = 9999`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}
