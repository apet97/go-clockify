package runtime

import (
	"log/slog"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/auditbridge"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
)

// Defaults capture the size/interval the wrapper uses unless tests
// override them via newBatchedAuditorWithSettings. ADR 0022 Q3
// documents the rationale: hardcoded, no operator-facing env var.
const (
	defaultAuditBatchSize     = 64
	defaultAuditBatchInterval = 250 * time.Millisecond
)

// batchedAuditor consolidates non-strict outcome audit events into
// pgx.SendBatch-sized writes while keeping intent records,
// fail_closed_strict outcomes, legacy single-shot rows, and handler
// panic records on the synchronous single-row path. The strict-rule
// guard lives in shouldFlushSync and is pinned by the property
// matrix test in audit_batch_test.go.
//
// The wrapper holds an embedded controlPlaneAuditor for the
// synchronous path; both paths share the same underlying
// controlplane.Store, with the batched path calling
// AppendAuditEventBatch and the sync path calling AppendAuditEvent.
//
// Close() must run before the underlying Store closes. The runtime
// service wires it via defer at the call site (see streamable.go).
type batchedAuditor struct {
	inner          controlPlaneAuditor
	durabilityMode string

	flushSize     int
	flushInterval time.Duration

	bufMu  sync.Mutex
	buf    []controlplane.AuditEvent
	closed bool

	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup
}

// newBatchedAuditor returns a batchedAuditor with the production
// defaults (64 events / 250 ms). If store is nil the wrapper is a
// no-op (RecordAudit returns nil); no goroutine spawns.
func newBatchedAuditor(store controlplane.Store, durabilityMode string) *batchedAuditor {
	return newBatchedAuditorWithSettings(store, durabilityMode, defaultAuditBatchSize, defaultAuditBatchInterval)
}

// newBatchedAuditorWithSettings is the test entry point: it lets a
// suite drive flushSize and flushInterval explicitly. flushInterval
// <= 0 disables the ticker goroutine (size-based flush only).
func newBatchedAuditorWithSettings(store controlplane.Store, durabilityMode string, flushSize int, flushInterval time.Duration) *batchedAuditor {
	if flushSize <= 0 {
		flushSize = defaultAuditBatchSize
	}
	b := &batchedAuditor{
		inner:          controlPlaneAuditor{store: store},
		durabilityMode: durabilityMode,
		flushSize:      flushSize,
		flushInterval:  flushInterval,
		done:           make(chan struct{}),
	}
	if store != nil && flushInterval > 0 {
		b.wg.Add(1)
		go b.flushLoop()
	}
	return b
}

// RecordAudit dispatches the event to the synchronous path or
// enqueues it for batched flush according to shouldFlushSync.
// A nil store short-circuits to nil, matching the documented
// controlPlaneAuditor.RecordAudit behaviour.
func (b *batchedAuditor) RecordAudit(event mcp.AuditEvent) error {
	if b.inner.store == nil {
		return nil
	}
	if shouldFlushSync(event.Phase, b.durabilityMode) {
		return b.inner.RecordAudit(event)
	}
	cpEvent := auditbridge.ToControlPlaneEvent(event, time.Now().UTC())

	var toFlush []controlplane.AuditEvent
	b.bufMu.Lock()
	if b.closed {
		// Post-Close fallback: write synchronously so the event is
		// not silently dropped if a caller races with shutdown.
		b.bufMu.Unlock()
		return b.inner.store.AppendAuditEvent(cpEvent)
	}
	b.buf = append(b.buf, cpEvent)
	if len(b.buf) >= b.flushSize {
		toFlush = b.buf
		b.buf = nil
	}
	b.bufMu.Unlock()
	if toFlush != nil {
		b.flush(toFlush)
	}
	return nil
}

// flush writes the slice via AppendAuditEventBatch. Errors are
// logged with the canonical audit_outcome=not_durable field so
// operators filter the same way they do today; they are NOT
// propagated to the caller because the events that take this path
// are always ones whose errors are silenced at the call site (per
// the strict-rule guard in shouldFlushSync).
func (b *batchedAuditor) flush(events []controlplane.AuditEvent) {
	if len(events) == 0 {
		return
	}
	if err := b.inner.store.AppendAuditEventBatch(events); err != nil {
		slog.Error("audit_batch_persist_failed",
			"count", len(events),
			"audit_outcome", "not_durable",
			"durability_mode", b.durabilityMode,
			"error", err)
	}
}

// flushLoop drains the buffer every flushInterval, plus once at
// shutdown when done is closed.
func (b *batchedAuditor) flushLoop() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.drainAndFlush()
		case <-b.done:
			b.drainAndFlush()
			return
		}
	}
}

func (b *batchedAuditor) drainAndFlush() {
	b.bufMu.Lock()
	toFlush := b.buf
	b.buf = nil
	b.bufMu.Unlock()
	b.flush(toFlush)
}

// Close stops the flush goroutine and drains any remaining events.
// Idempotent via closeOnce. After Close returns, any subsequent
// RecordAudit calls fall back to the synchronous single-row path
// so events are not silently lost.
func (b *batchedAuditor) Close() error {
	b.closeOnce.Do(func() {
		b.bufMu.Lock()
		b.closed = true
		b.bufMu.Unlock()
		close(b.done)
		b.wg.Wait()
	})
	return nil
}

// shouldFlushSync is the strict-rule guard: it returns true for any
// event that must take the synchronous single-row path, false for
// events safe to batch. ADR 0022 Q2 documents the matrix:
//
//   - PhaseIntent           → always sync (gates the mutation).
//   - "" (legacy)           → always sync (preserve historical
//     contract).
//   - PhaseHandlerPanic     → always sync (panic events are
//     operationally critical; a delayed flush could be lost if the
//     process exits).
//   - PhaseOutcome          → sync only under fail_closed_strict
//     where the persistence error must surface to the client.
//   - any unknown phase     → sync (fail closed on unexpected
//     input).
//
// The function is pure so the property matrix test can exercise it
// directly without any wrapper state.
func shouldFlushSync(phase mcp.AuditPhase, mode string) bool {
	switch phase {
	case mcp.PhaseIntent, mcp.PhaseHandlerPanic:
		return true
	case "":
		return true
	case mcp.PhaseOutcome:
		return mode == "fail_closed_strict"
	default:
		return true
	}
}
