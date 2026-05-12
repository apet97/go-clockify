package mcp

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/apet97/go-clockify/internal/authn"
)

// TestSessionCap_PerReplicaRejectsBeyondCap pins the per-replica gate
// added so a single replica cannot accept unbounded initialize calls.
// At MaxSessionsPerReplica=2, the third create returns
// errSessionReplicaCapReached and the handler maps it to HTTP 503 +
// Retry-After. Sessions already in flight stay alive.
//
// Drift check: comment out the maxPerReplica branch in create and the
// third create succeeds; this test fails with "expected
// errSessionReplicaCapReached on third create".
func TestSessionCap_PerReplicaRejectsBeyondCap(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	mgr.maxPerReplica = 2
	mgr.perPrincipal = map[string]int{}

	principal := authn.Principal{Subject: "alice", TenantID: "tenant-a"}
	for i := range 2 {
		id := "session-" + strconv.Itoa(i)
		if _, err := mgr.create(context.Background(), id, principal, opts); err != nil {
			t.Fatalf("create %d failed: %v", i, err)
		}
	}
	_, err := mgr.create(context.Background(), "session-overflow", principal, opts)
	if !errors.Is(err, errSessionReplicaCapReached) {
		t.Fatalf("expected errSessionReplicaCapReached on third create, got %v", err)
	}
}

// TestSessionCap_PerPrincipalRejectsTenantOverrun pins the per-
// principal gate: with MaxSessionsPerPrincipal=2, principal-A can open
// at most 2 sessions; principal-B is unaffected by A's quota. The
// per-replica cap is disabled here so the only constraint is the
// principal bucketing.
//
// Drift check: remove the maxPerPrincipal branch in create and B's
// session creation still succeeds, but A's third no longer fails.
func TestSessionCap_PerPrincipalRejectsTenantOverrun(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	mgr.maxPerPrincipal = 2
	mgr.perPrincipal = map[string]int{}

	a := authn.Principal{Subject: "alice", TenantID: "tenant-a"}
	b := authn.Principal{Subject: "bob", TenantID: "tenant-b"}

	for i := range 2 {
		if _, err := mgr.create(context.Background(), "a-"+strconv.Itoa(i), a, opts); err != nil {
			t.Fatalf("a-%d failed: %v", i, err)
		}
	}
	if _, err := mgr.create(context.Background(), "a-3", a, opts); !errors.Is(err, errSessionPrincipalCapReached) {
		t.Fatalf("expected errSessionPrincipalCapReached for principal A, got %v", err)
	}
	// Principal B must still be able to open a session — quota is
	// per-principal, not global.
	if _, err := mgr.create(context.Background(), "b-1", b, opts); err != nil {
		t.Fatalf("principal B blocked by A's quota: %v", err)
	}
}

// TestSessionCap_DestroyDecrementsPrincipalCount confirms that an
// evicted session releases its principal slot so a re-initialise
// after destroy succeeds. Without the decrement in destroy() (or in
// reapOnce's downstream destroy call), the principal cap would behave
// as a lifetime cap instead of a concurrent-session cap.
func TestSessionCap_DestroyDecrementsPrincipalCount(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	mgr.maxPerPrincipal = 1
	mgr.perPrincipal = map[string]int{}

	p := authn.Principal{Subject: "alice", TenantID: "tenant-a"}

	sess, err := mgr.create(context.Background(), "alice-1", p, opts)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	mgr.destroy("alice-1", sess)

	if _, err := mgr.create(context.Background(), "alice-2", p, opts); err != nil {
		t.Fatalf("create after destroy should have succeeded, got %v", err)
	}
}

// TestSessionCap_ZeroDefaultsAreUnlimited pins backwards-compatibility:
// with both caps at 0 (the spec.go default), every create succeeds
// regardless of count. Operators that have not enabled the gates must
// see the historical unbounded behaviour.
func TestSessionCap_ZeroDefaultsAreUnlimited(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	// Defaults: maxPerReplica=0, maxPerPrincipal=0.
	mgr.perPrincipal = map[string]int{}

	p := authn.Principal{Subject: "alice", TenantID: "tenant-a"}
	for i := range 100 {
		if _, err := mgr.create(context.Background(), "alice-"+strconv.Itoa(i), p, opts); err != nil {
			t.Fatalf("create %d with caps=0 failed: %v", i, err)
		}
	}
}
