//go:build postgres

package postgres

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// advisoryLockKey is the fixed 64-bit key claimed by the applier. It
// serialises concurrent clockify-mcp startups against the same database
// so only one process runs a given migration. Value chosen to be
// application-specific and unlikely to collide with other tenants.
const advisoryLockKey int64 = 0x434c4d435f4350 // "CLMC_CP"

type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations enumerates the embedded migrations/*.sql files, parses
// their leading `NNN_` prefix as the version number, and returns them
// in ascending version order. Files that do not match the NNN_* shape
// are reported as an error rather than silently skipped so accidental
// misnames fail loudly.
func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			return nil, fmt.Errorf("migration %q has no NNN_ prefix", name)
		}
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q prefix %q is not an integer: %w", name, prefix, err)
		}
		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", name, err)
		}
		out = append(out, migration{version: v, name: name, sql: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	for i := 1; i < len(out); i++ {
		if out[i].version == out[i-1].version {
			return nil, fmt.Errorf("duplicate migration version %d (%q, %q)", out[i].version, out[i-1].name, out[i].name)
		}
	}
	return out, nil
}

// applyMigrations ensures schema_migrations exists, then applies any
// embedded migrations whose version is not yet recorded. Each migration
// runs in its own transaction. A pg_advisory_lock guards the whole
// sequence so concurrent startups against the same database are safe.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() {
		// Best-effort unlock; the connection is released back to the
		// pool either way so at worst the lock is held until the conn
		// closes, which is fine because another startup will queue.
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := readAppliedVersions(ctx, conn.Conn())
	if err != nil {
		return err
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	// E2.1: refuse to boot when the database is ahead of the binary.
	// A version in schema_migrations that the embedded set does not
	// know means we would silently ignore a schema the binary cannot
	// interpret — which is how "mysterious missing column" incidents
	// happen. Surface the mismatch instead.
	knownMax := 0
	if len(migrations) > 0 {
		knownMax = migrations[len(migrations)-1].version
	}
	for v := range applied {
		if v > knownMax {
			return fmt.Errorf("controlplane/postgres: database schema is at version %d but this binary only knows up to %d; upgrade the binary or roll the database back", v, knownMax)
		}
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := applyOne(ctx, conn.Conn(), m); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
	}
	return nil
}

func readAppliedVersions(ctx context.Context, conn *pgx.Conn) (map[int]bool, error) {
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func applyOne(ctx context.Context, conn *pgx.Conn, m migration) error {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, m.sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
