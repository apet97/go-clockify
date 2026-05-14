//go:build legacy_platform && postgres && integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apet97/go-clockify/internal/controlplane"
)

// auditBatchVerifyPool opens a separate pgxpool against the same DSN
// so the verification SELECT lives outside the Store under test. The
// pattern mirrors live_audit_phases_test.go::verifyPool and keeps the
// assertion independent of the Store.AppendAuditEventBatch
// implementation it is checking.
func auditBatchVerifyPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, dsn(t))
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func auditBatchCleanup(t *testing.T, pool *pgxpool.Pool, ctx context.Context, sessionID string) {
	t.Helper()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx,
			`DELETE FROM audit_events WHERE session_id = $1`, sessionID); err != nil {
			t.Logf("cleanup audit_events for session %s: %v", sessionID, err)
		}
	})
}

func sampleBatchEvent(sessionID string, phase string, idx int, base time.Time) controlplane.AuditEvent {
	return controlplane.AuditEvent{
		At:        base.Add(time.Duration(idx) * time.Millisecond),
		TenantID:  "acme",
		Subject:   "alice@example.com",
		SessionID: sessionID,
		Transport: "streamable_http",
		Tool:      fmt.Sprintf("clockify_update_entry_%d", idx),
		Action:    "tools/call",
		Outcome:   "succeeded",
		Phase:     phase,
		Reason:    "ok",
		ResourceIDs: map[string]string{
			"entry_id": fmt.Sprintf("entry-%d", idx),
		},
		Metadata: map[string]string{
			"client_name": "test-suite",
		},
	}
}

// TestPostgresAppendAuditEventBatch_RoundTrip writes a 12-event batch
// via the new method and asserts every row lands with the expected
// (tool, phase, outcome) tuple. This is the basic "batch behaves like
// N sequential AppendAuditEvent calls" guarantee.
func TestPostgresAppendAuditEventBatch_RoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := openStore(t)
	verifyPool := auditBatchVerifyPool(t, ctx)
	sessionID := fmt.Sprintf("batch-rt-%d", time.Now().UnixNano())
	auditBatchCleanup(t, verifyPool, ctx, sessionID)

	base := time.Now().UTC().Truncate(time.Microsecond)
	const batchSize = 12
	events := make([]controlplane.AuditEvent, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		events = append(events, sampleBatchEvent(sessionID, "outcome", i, base))
	}

	if err := store.AppendAuditEventBatch(events); err != nil {
		t.Fatalf("AppendAuditEventBatch: %v", err)
	}

	var got int
	if err := verifyPool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE session_id = $1`,
		sessionID,
	).Scan(&got); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if got != batchSize {
		t.Fatalf("audit_events count = %d, want %d", got, batchSize)
	}

	rows, err := verifyPool.Query(ctx,
		`SELECT tool, phase, outcome FROM audit_events
		   WHERE session_id = $1 ORDER BY at ASC`, sessionID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	idx := 0
	for rows.Next() {
		var tool, phase, outcome string
		if err := rows.Scan(&tool, &phase, &outcome); err != nil {
			t.Fatalf("scan: %v", err)
		}
		wantTool := fmt.Sprintf("clockify_update_entry_%d", idx)
		if tool != wantTool {
			t.Errorf("row %d tool = %q, want %q", idx, tool, wantTool)
		}
		if phase != "outcome" {
			t.Errorf("row %d phase = %q, want %q", idx, phase, "outcome")
		}
		if outcome != "succeeded" {
			t.Errorf("row %d outcome = %q, want %q", idx, outcome, "succeeded")
		}
		idx++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if idx != batchSize {
		t.Errorf("scanned %d rows, want %d", idx, batchSize)
	}
}

// TestPostgresAppendAuditEventBatch_ConflictDedup pins the
// ON CONFLICT (external_id) DO NOTHING contract for the batched
// path. Submitting the same batch twice must produce N rows, not 2N.
// This mirrors AppendAuditEvent's idempotency and is the load-bearing
// guarantee that lets the runtime layer flush at-most-once-per-event
// without worrying about double-writes during retries.
func TestPostgresAppendAuditEventBatch_ConflictDedup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := openStore(t)
	verifyPool := auditBatchVerifyPool(t, ctx)
	sessionID := fmt.Sprintf("batch-dedup-%d", time.Now().UnixNano())
	auditBatchCleanup(t, verifyPool, ctx, sessionID)

	base := time.Now().UTC().Truncate(time.Microsecond)
	const batchSize = 8
	events := make([]controlplane.AuditEvent, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		events = append(events, sampleBatchEvent(sessionID, "outcome", i, base))
	}

	if err := store.AppendAuditEventBatch(events); err != nil {
		t.Fatalf("first AppendAuditEventBatch: %v", err)
	}
	if err := store.AppendAuditEventBatch(events); err != nil {
		t.Fatalf("second AppendAuditEventBatch (idempotent): %v", err)
	}

	var got int
	if err := verifyPool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE session_id = $1`,
		sessionID,
	).Scan(&got); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if got != batchSize {
		t.Fatalf("audit_events count after duplicate batch = %d, want %d (ON CONFLICT must dedupe)", got, batchSize)
	}
}

// TestPostgresAppendAuditEventBatch_EmptyIsNoOp pins the empty-slice
// contract: the method short-circuits to nil without touching the
// pool. Without this guarantee the runtime-layer wrapper would have
// to guard every drain call site.
func TestPostgresAppendAuditEventBatch_EmptyIsNoOp(t *testing.T) {
	store := openStore(t)
	if err := store.AppendAuditEventBatch(nil); err != nil {
		t.Errorf("AppendAuditEventBatch(nil) = %v, want nil", err)
	}
	if err := store.AppendAuditEventBatch([]controlplane.AuditEvent{}); err != nil {
		t.Errorf("AppendAuditEventBatch(empty) = %v, want nil", err)
	}
}

// TestPostgresAppendAuditEventBatch_MixedPhasesPreserveExternalIDDistinction
// pins that batched intent + outcome rows do not collide on
// external_id. The synthesised id includes phase per the helper in
// postgres.go (auditInsertArgs), so a single batch carrying two rows
// for the same (At, SessionID, Tool) but different phases must land
// as two rows, not one. This is the same invariant
// TestLiveCreateUpdateDeleteEntryAuditPhases pins for the
// AppendAuditEvent path; pinning it on the batched path defends
// against a future refactor of auditInsertArgs that forgets phase.
func TestPostgresAppendAuditEventBatch_MixedPhasesPreserveExternalIDDistinction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := openStore(t)
	verifyPool := auditBatchVerifyPool(t, ctx)
	sessionID := fmt.Sprintf("batch-mixed-%d", time.Now().UnixNano())
	auditBatchCleanup(t, verifyPool, ctx, sessionID)

	// Same (At, SessionID, Tool) tuple, two different phases.
	base := time.Now().UTC().Truncate(time.Microsecond)
	events := []controlplane.AuditEvent{
		{
			At:        base,
			TenantID:  "acme",
			Subject:   "alice@example.com",
			SessionID: sessionID,
			Tool:      "clockify_update_entry",
			Action:    "tools/call",
			Outcome:   "attempted",
			Phase:     "intent",
		},
		{
			At:        base,
			TenantID:  "acme",
			Subject:   "alice@example.com",
			SessionID: sessionID,
			Tool:      "clockify_update_entry",
			Action:    "tools/call",
			Outcome:   "succeeded",
			Phase:     "outcome",
		},
	}
	if err := store.AppendAuditEventBatch(events); err != nil {
		t.Fatalf("AppendAuditEventBatch: %v", err)
	}

	var got int
	if err := verifyPool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE session_id = $1`,
		sessionID,
	).Scan(&got); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if got != 2 {
		t.Fatalf("intent+outcome rows merged on ON CONFLICT: got %d rows, want 2", got)
	}
}
