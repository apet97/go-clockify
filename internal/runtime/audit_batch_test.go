package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
)

// batchAuditFakeStore is a minimal controlplane.Store implementation
// purpose-built for the batched-auditor tests. It tracks the path
// each event took (sync AppendAuditEvent vs batch
// AppendAuditEventBatch) so assertions can distinguish the two paths
// without parsing the event slice itself. Errors are injectable per
// path so the strict-outcome surfacing path can be verified.
type batchAuditFakeStore struct {
	mu          sync.Mutex
	singleCalls []controlplane.AuditEvent
	batchCalls  [][]controlplane.AuditEvent
	singleErr   error
	batchErr    error
}

func (s *batchAuditFakeStore) AppendAuditEvent(e controlplane.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.singleErr != nil {
		return s.singleErr
	}
	s.singleCalls = append(s.singleCalls, e)
	return nil
}

func (s *batchAuditFakeStore) AppendAuditEventBatch(events []controlplane.AuditEvent) error {
	if len(events) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.batchErr != nil {
		return s.batchErr
	}
	cp := make([]controlplane.AuditEvent, len(events))
	copy(cp, events)
	s.batchCalls = append(s.batchCalls, cp)
	return nil
}

func (s *batchAuditFakeStore) singleCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.singleCalls)
}

func (s *batchAuditFakeStore) batchCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.batchCalls)
}

func (s *batchAuditFakeStore) totalBatched() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, b := range s.batchCalls {
		n += len(b)
	}
	return n
}

// Stubs — the batched-auditor path never calls these.
func (s *batchAuditFakeStore) Tenant(string) (controlplane.TenantRecord, bool) {
	return controlplane.TenantRecord{}, false
}
func (s *batchAuditFakeStore) PutTenant(controlplane.TenantRecord) error { return nil }
func (s *batchAuditFakeStore) CredentialRef(string) (controlplane.CredentialRef, bool) {
	return controlplane.CredentialRef{}, false
}
func (s *batchAuditFakeStore) PutCredentialRef(controlplane.CredentialRef) error { return nil }
func (s *batchAuditFakeStore) Session(string) (controlplane.SessionRecord, bool) {
	return controlplane.SessionRecord{}, false
}
func (s *batchAuditFakeStore) PutSession(controlplane.SessionRecord) error { return nil }
func (s *batchAuditFakeStore) DeleteSession(string) error                  { return nil }
func (s *batchAuditFakeStore) RetainAudit(context.Context, time.Duration) (int, error) {
	return 0, nil
}
func (s *batchAuditFakeStore) Close() error { return nil }

// mustNewBatchedAuditor returns a batched auditor with size/interval
// tuned for tests and queues t.Cleanup to drain it on test exit.
func mustNewBatchedAuditor(t *testing.T, store controlplane.Store, mode string, size int, interval time.Duration) *batchedAuditor {
	t.Helper()
	b := newBatchedAuditorWithSettings(store, mode, size, interval)
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func sampleEvent(phase mcp.AuditPhase) mcp.AuditEvent {
	return mcp.AuditEvent{
		Tool:    "clockify_update_entry",
		Action:  "tools/call",
		Outcome: "attempted",
		Phase:   phase,
		Metadata: map[string]string{
			"tenant_id":  "acme",
			"subject":    "alice@example.com",
			"session_id": "sess-1",
			"transport":  "streamable_http",
		},
	}
}

// TestBatchedAuditor_IntentAlwaysSync pins that intent records take
// the synchronous AppendAuditEvent path regardless of durability
// mode. This is the load-bearing guarantee from internal/mcp/audit.go:
// in fail_closed mode the intent persistence error short-circuits
// the mutation; if intent were batched, the error would land
// asynchronously and the mutation would already have run.
func TestBatchedAuditor_IntentAlwaysSync(t *testing.T) {
	for _, mode := range []string{"best_effort", "fail_closed", "fail_closed_strict"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			fake := &batchAuditFakeStore{}
			ba := mustNewBatchedAuditor(t, fake, mode, 64, 50*time.Millisecond)
			if err := ba.RecordAudit(sampleEvent(mcp.PhaseIntent)); err != nil {
				t.Fatalf("RecordAudit returned %v", err)
			}
			if got := fake.singleCount(); got != 1 {
				t.Errorf("single-call count = %d, want 1", got)
			}
			if got := fake.batchCount(); got != 0 {
				t.Errorf("batch-call count = %d, want 0 (intent must not batch)", got)
			}
		})
	}
}

// TestBatchedAuditor_LegacyPhaseSync pins that audit rows with an
// empty Phase field (the historical single-shot pattern) take the
// synchronous path. Preserves the contract for any caller still
// emitting non-phase'd audits during a long deprecation window.
func TestBatchedAuditor_LegacyPhaseSync(t *testing.T) {
	fake := &batchAuditFakeStore{}
	ba := mustNewBatchedAuditor(t, fake, "fail_closed", 64, 50*time.Millisecond)
	if err := ba.RecordAudit(sampleEvent("")); err != nil {
		t.Fatalf("RecordAudit returned %v", err)
	}
	if got := fake.singleCount(); got != 1 {
		t.Errorf("single-call count = %d, want 1", got)
	}
	if got := fake.batchCount(); got != 0 {
		t.Errorf("legacy-phase event batched (count=%d)", got)
	}
}

// TestBatchedAuditor_HandlerPanicSync pins that PhaseHandlerPanic
// records always take the synchronous path. Panic events are
// operationally critical and a delayed flush could be lost if the
// process exits before the next ticker fires.
func TestBatchedAuditor_HandlerPanicSync(t *testing.T) {
	for _, mode := range []string{"best_effort", "fail_closed", "fail_closed_strict"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			fake := &batchAuditFakeStore{}
			ba := mustNewBatchedAuditor(t, fake, mode, 64, 50*time.Millisecond)
			if err := ba.RecordAudit(sampleEvent(mcp.PhaseHandlerPanic)); err != nil {
				t.Fatalf("RecordAudit returned %v", err)
			}
			if got := fake.singleCount(); got != 1 {
				t.Errorf("single-call count = %d, want 1", got)
			}
			if got := fake.batchCount(); got != 0 {
				t.Errorf("panic event batched (count=%d)", got)
			}
		})
	}
}

// TestBatchedAuditor_StrictOutcomeSync pins that PhaseOutcome events
// take the synchronous path under fail_closed_strict so the
// persistence error can surface to the client per the audit.go:48
// contract.
func TestBatchedAuditor_StrictOutcomeSync(t *testing.T) {
	fake := &batchAuditFakeStore{}
	ba := mustNewBatchedAuditor(t, fake, "fail_closed_strict", 64, 50*time.Millisecond)
	if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
		t.Fatalf("RecordAudit returned %v", err)
	}
	if got := fake.singleCount(); got != 1 {
		t.Errorf("single-call count = %d, want 1 (strict outcomes must surface sync)", got)
	}
	if got := fake.batchCount(); got != 0 {
		t.Errorf("strict outcome batched (count=%d)", got)
	}
}

// TestBatchedAuditor_NonStrictOutcomeBatches pins that PhaseOutcome
// events take the batched path under best_effort and non-strict
// fail_closed. These cells already silence persistence errors at
// the call site (audit.go:48 only returns an error in strict mode)
// so batching does not change the observable contract.
func TestBatchedAuditor_NonStrictOutcomeBatches(t *testing.T) {
	for _, mode := range []string{"best_effort", "fail_closed"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			fake := &batchAuditFakeStore{}
			// flushSize=1 forces an immediate flush so the test
			// does not need to wait for the ticker.
			ba := mustNewBatchedAuditor(t, fake, mode, 1, 50*time.Millisecond)
			if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
				t.Fatalf("RecordAudit returned %v", err)
			}
			if got := fake.singleCount(); got != 0 {
				t.Errorf("non-strict outcome took sync path (count=%d)", got)
			}
			if got := fake.batchCount(); got != 1 {
				t.Errorf("batch-call count = %d, want 1", got)
			}
			if got := fake.totalBatched(); got != 1 {
				t.Errorf("batched-event count = %d, want 1", got)
			}
		})
	}
}

// TestBatchedAuditor_FlushOnSize pins that the buffer flushes
// synchronously once it reaches flushSize, without waiting for the
// ticker.
func TestBatchedAuditor_FlushOnSize(t *testing.T) {
	fake := &batchAuditFakeStore{}
	const size = 4
	// flushInterval=0 disables the ticker so any flush observed must
	// be size-triggered.
	ba := mustNewBatchedAuditor(t, fake, "fail_closed", size, 0)

	for i := 0; i < size-1; i++ {
		if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
			t.Fatalf("RecordAudit[%d] returned %v", i, err)
		}
	}
	if got := fake.batchCount(); got != 0 {
		t.Fatalf("flush triggered before size threshold (batches=%d)", got)
	}
	if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
		t.Fatalf("RecordAudit(size-th) returned %v", err)
	}
	if got := fake.batchCount(); got != 1 {
		t.Errorf("batch-call count after %d events = %d, want 1", size, got)
	}
	if got := fake.totalBatched(); got != size {
		t.Errorf("batched-event count = %d, want %d", got, size)
	}
}

// TestBatchedAuditor_FlushOnInterval pins that the ticker drains
// the buffer even when it hasn't reached flushSize. A 20ms interval
// + 200ms wait is loose enough to be stable on slow CI runners but
// tight enough that a missing flush would fail the test.
func TestBatchedAuditor_FlushOnInterval(t *testing.T) {
	fake := &batchAuditFakeStore{}
	ba := mustNewBatchedAuditor(t, fake, "fail_closed", 64, 20*time.Millisecond)
	if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
		t.Fatalf("RecordAudit returned %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fake.batchCount() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := fake.batchCount(); got != 1 {
		t.Errorf("batch-call count = %d, want 1 (ticker should drain)", got)
	}
	if got := fake.totalBatched(); got != 1 {
		t.Errorf("batched-event count = %d, want 1", got)
	}
	_ = ba
}

// TestBatchedAuditor_CloseDrains pins that Close flushes the buffer
// before returning. Without this guarantee, in-flight outcome
// events would be lost on shutdown.
func TestBatchedAuditor_CloseDrains(t *testing.T) {
	fake := &batchAuditFakeStore{}
	// Big size + long interval so neither size nor ticker triggers
	// before we call Close; the drain on shutdown is the only thing
	// that can produce a batch.
	ba := newBatchedAuditorWithSettings(fake, "fail_closed", 64, 10*time.Second)

	for i := 0; i < 3; i++ {
		if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
			t.Fatalf("RecordAudit[%d] returned %v", i, err)
		}
	}
	if got := fake.batchCount(); got != 0 {
		t.Fatalf("flush triggered before Close (batches=%d)", got)
	}
	if err := ba.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	if got := fake.batchCount(); got != 1 {
		t.Errorf("batch-call count after Close = %d, want 1", got)
	}
	if got := fake.totalBatched(); got != 3 {
		t.Errorf("batched-event count = %d, want 3", got)
	}
}

// TestBatchedAuditor_CloseIsIdempotent pins that a second Close
// call is a safe no-op. Useful for defer chains that may invoke it
// after an explicit Close.
func TestBatchedAuditor_CloseIsIdempotent(t *testing.T) {
	fake := &batchAuditFakeStore{}
	ba := newBatchedAuditorWithSettings(fake, "fail_closed", 64, 10*time.Second)
	if err := ba.Close(); err != nil {
		t.Fatalf("first Close returned %v", err)
	}
	if err := ba.Close(); err != nil {
		t.Fatalf("second Close returned %v", err)
	}
}

// TestBatchedAuditor_PostCloseSyncFallback pins that RecordAudit
// calls arriving after Close fall back to the synchronous
// AppendAuditEvent path so events are not silently dropped by a
// race with shutdown.
func TestBatchedAuditor_PostCloseSyncFallback(t *testing.T) {
	fake := &batchAuditFakeStore{}
	ba := newBatchedAuditorWithSettings(fake, "fail_closed", 64, 10*time.Second)
	if err := ba.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
		t.Fatalf("post-Close RecordAudit returned %v", err)
	}
	if got := fake.singleCount(); got != 1 {
		t.Errorf("post-Close sync fallback count = %d, want 1", got)
	}
}

// TestBatchedAuditor_NilStoreNoOp pins that a wrapper around a nil
// store accepts RecordAudit calls and returns nil, matching the
// documented controlPlaneAuditor zero-store behaviour. No
// goroutine spawns.
func TestBatchedAuditor_NilStoreNoOp(t *testing.T) {
	ba := newBatchedAuditor(nil, "fail_closed")
	if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
		t.Errorf("nil-store RecordAudit returned %v", err)
	}
	if err := ba.Close(); err != nil {
		t.Errorf("nil-store Close returned %v", err)
	}
}

// TestBatchedAuditor_StrictOutcomeErrorPropagates pins that when a
// strict-outcome event takes the sync path and the store rejects
// it, the error surfaces back to the caller. Without this the
// fail_closed_strict contract (audit.go:48) is silently weakened.
func TestBatchedAuditor_StrictOutcomeErrorPropagates(t *testing.T) {
	fake := &batchAuditFakeStore{singleErr: errors.New("persistence offline")}
	ba := newBatchedAuditorWithSettings(fake, "fail_closed_strict", 64, 10*time.Second)
	t.Cleanup(func() { _ = ba.Close() })

	err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome))
	if err == nil || err.Error() != "persistence offline" {
		t.Fatalf("strict outcome error did not surface: got %v", err)
	}
}

// TestBatchedAuditor_BatchedErrorIsLoggedNotPropagated pins that the
// batched path silences AppendAuditEventBatch errors per the
// contract: events on this path are ones whose error is already
// silenced at the audit.go call site, so the wrapper must not
// invent an error path. Without this guarantee, a transient
// Postgres outage during a flush would surface to handlers in
// fail_closed mode, breaking the existing contract.
func TestBatchedAuditor_BatchedErrorIsLoggedNotPropagated(t *testing.T) {
	fake := &batchAuditFakeStore{batchErr: errors.New("transient pg outage")}
	ba := newBatchedAuditorWithSettings(fake, "fail_closed", 1, 10*time.Second)
	t.Cleanup(func() { _ = ba.Close() })

	if err := ba.RecordAudit(sampleEvent(mcp.PhaseOutcome)); err != nil {
		t.Errorf("non-strict outcome surfaced batched-flush error: %v", err)
	}
}

// TestBatchedAuditor_ConcurrentRecordAndClose verifies the wrapper
// is race-free under concurrent RecordAudit + Close. The strict-
// outcome path is exercised because it takes the synchronous fast
// path that bypasses the buffer entirely; combining it with the
// batched-outcome path forces both code paths to run alongside the
// drain. Run with -race for the actual gate.
func TestBatchedAuditor_ConcurrentRecordAndClose(t *testing.T) {
	fake := &batchAuditFakeStore{}
	ba := newBatchedAuditorWithSettings(fake, "fail_closed", 32, 5*time.Millisecond)

	var wg sync.WaitGroup
	var emitted atomic.Int64
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				phase := mcp.PhaseOutcome
				if i%5 == 0 {
					phase = mcp.PhaseIntent
				}
				if err := ba.RecordAudit(sampleEvent(phase)); err != nil {
					t.Errorf("RecordAudit returned %v", err)
				}
				emitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if err := ba.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}

	total := fake.singleCount() + fake.totalBatched()
	if int64(total) != emitted.Load() {
		t.Errorf("total persisted (%d) != total emitted (%d)", total, emitted.Load())
	}
}

// TestAuditBatch_ShouldFlushSyncMatrix exhaustively pins the
// strict-rule guard at the pure-function layer. Every phase × mode
// cell in the matrix has a known sync/batch verdict; a future
// refactor that quietly reshapes shouldFlushSync will fail loudly
// here rather than silently weakening one of the load-bearing
// audit-durability invariants (see internal/mcp/audit.go:33,48).
//
// This is the commit-3 "guard pin" complement to the commit-2
// scenario tests in TestBatchedAuditor_*: the spot tests prove the
// guard works for the most common cells; this matrix proves no
// cell quietly drifts.
func TestAuditBatch_ShouldFlushSyncMatrix(t *testing.T) {
	type cell struct {
		phase mcp.AuditPhase
		mode  string
		sync  bool // want: true → sync single-row, false → batched
		why   string
	}

	cells := []cell{
		// Intent records gate the mutation; any mode must take sync
		// so a fail_closed pipeline failure short-circuits BEFORE the
		// handler runs (audit.go:33).
		{phase: mcp.PhaseIntent, mode: "best_effort", sync: true, why: "intent gates mutation"},
		{phase: mcp.PhaseIntent, mode: "fail_closed", sync: true, why: "intent gates mutation"},
		{phase: mcp.PhaseIntent, mode: "fail_closed_strict", sync: true, why: "intent gates mutation"},

		// Legacy single-shot records preserve historical contract.
		{phase: "", mode: "best_effort", sync: true, why: "legacy single-shot stays sync"},
		{phase: "", mode: "fail_closed", sync: true, why: "legacy single-shot stays sync"},
		{phase: "", mode: "fail_closed_strict", sync: true, why: "legacy single-shot stays sync"},

		// Handler-panic outcomes are operationally critical; never
		// risk a delayed flush losing them if the process exits.
		{phase: mcp.PhaseHandlerPanic, mode: "best_effort", sync: true, why: "panic events are critical"},
		{phase: mcp.PhaseHandlerPanic, mode: "fail_closed", sync: true, why: "panic events are critical"},
		{phase: mcp.PhaseHandlerPanic, mode: "fail_closed_strict", sync: true, why: "panic events are critical"},

		// Outcome under strict mode must surface the persistence error
		// to the client (audit.go:48); batching it would silently
		// drop that signal.
		{phase: mcp.PhaseOutcome, mode: "fail_closed_strict", sync: true, why: "strict outcome must surface error"},

		// The two cells that ACTUALLY batch — best_effort and
		// non-strict fail_closed outcome events. Both silence
		// persistence errors at the audit.go call site, so the
		// batched path does not change the observable contract.
		{phase: mcp.PhaseOutcome, mode: "best_effort", sync: false, why: "best_effort outcome is the perf cell"},
		{phase: mcp.PhaseOutcome, mode: "fail_closed", sync: false, why: "non-strict fail_closed outcome is the perf cell"},

		// Unknown / malformed phase string falls through to sync to
		// fail closed on unexpected input rather than letting it slip
		// into the buffer where its semantics are undefined.
		{phase: "unrecognised", mode: "fail_closed", sync: true, why: "unknown phase fails closed to sync"},
	}

	for _, c := range cells {
		c := c
		t.Run(string(c.phase)+"/"+c.mode, func(t *testing.T) {
			got := shouldFlushSync(c.phase, c.mode)
			if got != c.sync {
				t.Errorf("shouldFlushSync(%q, %q) = %v, want %v -- %s",
					c.phase, c.mode, got, c.sync, c.why)
			}
		})
	}
}

// TestAuditBatch_PropertyMatrix is the end-to-end counterpart to
// TestAuditBatch_ShouldFlushSyncMatrix: every cell is exercised
// through a real batchedAuditor wrapper so the strict-rule guard is
// also pinned at the integration boundary (wrapper → fake Store).
// A wrapper-side change that ignores shouldFlushSync's verdict (e.g.
// short-circuiting the dispatch) would pass the pure-function
// matrix above but fail this one.
//
// The cells use flushSize=1 so any "batch" verdict produces an
// immediate AppendAuditEventBatch call; flushInterval is large so
// the ticker can't accidentally satisfy a sync expectation.
func TestAuditBatch_PropertyMatrix(t *testing.T) {
	cases := []struct {
		phase mcp.AuditPhase
		mode  string
		sync  bool
	}{
		{mcp.PhaseIntent, "best_effort", true},
		{mcp.PhaseIntent, "fail_closed", true},
		{mcp.PhaseIntent, "fail_closed_strict", true},
		{"", "best_effort", true},
		{"", "fail_closed", true},
		{"", "fail_closed_strict", true},
		{mcp.PhaseHandlerPanic, "best_effort", true},
		{mcp.PhaseHandlerPanic, "fail_closed", true},
		{mcp.PhaseHandlerPanic, "fail_closed_strict", true},
		{mcp.PhaseOutcome, "fail_closed_strict", true},
		{mcp.PhaseOutcome, "best_effort", false},
		{mcp.PhaseOutcome, "fail_closed", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.phase)+"/"+tc.mode, func(t *testing.T) {
			fake := &batchAuditFakeStore{}
			ba := newBatchedAuditorWithSettings(fake, tc.mode, 1, 10*time.Second)
			t.Cleanup(func() { _ = ba.Close() })

			if err := ba.RecordAudit(sampleEvent(tc.phase)); err != nil {
				t.Fatalf("RecordAudit returned %v", err)
			}
			if tc.sync {
				if got := fake.singleCount(); got != 1 {
					t.Errorf("(phase=%q, mode=%q): expected sync, single-count=%d (want 1)",
						tc.phase, tc.mode, got)
				}
				if got := fake.batchCount(); got != 0 {
					t.Errorf("(phase=%q, mode=%q): expected sync, batch-count=%d (want 0)",
						tc.phase, tc.mode, got)
				}
				return
			}
			if got := fake.batchCount(); got != 1 {
				t.Errorf("(phase=%q, mode=%q): expected batch, batch-count=%d (want 1)",
					tc.phase, tc.mode, got)
			}
			if got := fake.singleCount(); got != 0 {
				t.Errorf("(phase=%q, mode=%q): expected batch, single-count=%d (want 0)",
					tc.phase, tc.mode, got)
			}
		})
	}
}
