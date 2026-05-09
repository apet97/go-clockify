//go:build postgres

package postgres

import (
	"strings"
	"testing"
)

func TestMigrationsDropSessionAffinityColumn(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	for _, m := range migrations {
		if m.version != 3 {
			continue
		}
		if !strings.Contains(m.sql, "ALTER TABLE sessions") {
			t.Fatalf("migration %s does not target sessions table", m.name)
		}
		if !strings.Contains(m.sql, "DROP COLUMN IF EXISTS session_affinity_id") {
			t.Fatalf("migration %s does not drop session_affinity_id", m.name)
		}
		return
	}
	t.Fatal("missing migration 003 to drop session_affinity_id")
}

func TestMigrationsAddAuditEventsTenantAtIndex(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	for _, m := range migrations {
		if m.version != 4 {
			continue
		}
		if !strings.Contains(m.sql, "idx_audit_events_tenant_id_at") {
			t.Fatalf("migration %s does not name tenant/time audit index", m.name)
		}
		if !strings.Contains(m.sql, "ON audit_events (tenant_id, at)") {
			t.Fatalf("migration %s does not index audit_events by tenant_id, at", m.name)
		}
		return
	}
	t.Fatal("missing migration 004 to add audit_events tenant/time index")
}
