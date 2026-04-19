package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/metrics"
)

// retainAuditLoop is the background reaper for the control-plane
// audit log. It wakes on a fixed interval, asks the store to drop
// events older than maxAge, and updates
// clockify_mcp_audit_events_retained_total. Mirrors the session reaper
// pattern in internal/mcp/transport_streamable_http.go:515-526 — one
// ticker, one ctx.Done() branch, one synchronous step per tick.
//
// Exits silently when ctx is cancelled. Returning from main.run()
// waits on no one, so the reaper goroutine is orphaned at shutdown by
// design; its last in-flight RetainAudit is bounded by the store's
// own per-op timeout and never holds open external resources.
//
// Folds into internal/runtime/retain.go when C2.2 extracts the
// runtime; keeping it next to main.go for now so the B2 commit is
// legible in isolation.
func retainAuditLoop(ctx context.Context, store controlplane.Store, maxAge, interval time.Duration) {
	if maxAge <= 0 || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// Run once immediately so a restart after a long outage reaps
	// backlog without waiting for the first interval.
	retainAuditOnce(ctx, store, maxAge)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			retainAuditOnce(ctx, store, maxAge)
		}
	}
}

// retainAuditOnce is a single tick of the reaper. Split out so tests
// can exercise the outcome metric without standing up a ticker.
func retainAuditOnce(ctx context.Context, store controlplane.Store, maxAge time.Duration) {
	deleted, err := store.RetainAudit(ctx, maxAge)
	if err != nil {
		slog.Warn("audit_retention_failed", "error", err.Error(), "max_age", maxAge.String())
		metrics.AuditEventsRetainedTotal.Inc("error")
		return
	}
	if deleted > 0 {
		slog.Info("audit_retention_reaped", "deleted", deleted, "max_age", maxAge.String())
		metrics.AuditEventsRetainedTotal.Add(uint64(deleted), "deleted")
	}
}
