package authn

import (
	"fmt"
	"testing"
	"time"
)

// TestOIDCVerifyCache_HitAndMiss walks the happy path: a put followed
// by an in-TTL get returns the cached principal, and a get for an
// unknown token returns a miss.
func TestOIDCVerifyCache_HitAndMiss(t *testing.T) {
	cache := newOIDCVerifyCache(16)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	p := Principal{Subject: "alice", TenantID: "acme", AuthMode: ModeOIDC}
	cache.put("token-a", p, now.Add(10*time.Second).Unix(), now)

	got, ok := cache.get("token-a", now)
	if !ok {
		t.Fatal("expected cache hit for token-a")
	}
	if got.Subject != p.Subject || got.TenantID != p.TenantID {
		t.Errorf("principal diverges: got %+v want %+v", got, p)
	}

	if _, ok := cache.get("token-unknown", now); ok {
		t.Error("expected miss for unknown token")
	}
}

// TestOIDCVerifyCache_ExpiresOnTokenExp documents that the cache honours
// the token's own exp claim: once `now` passes tokenExp, subsequent gets
// miss and delete the stale entry. Critical for revocation semantics —
// an expired token must not be served from cache past its stated
// lifetime.
func TestOIDCVerifyCache_ExpiresOnTokenExp(t *testing.T) {
	cache := newOIDCVerifyCache(16)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	tokenExp := now.Add(5 * time.Second).Unix()
	cache.put("token-a", Principal{Subject: "alice"}, tokenExp, now)

	// Inside the window: hit.
	if _, ok := cache.get("token-a", now.Add(3*time.Second)); !ok {
		t.Fatal("expected hit at 3s (token valid until 5s)")
	}

	// Past the token's exp: miss (entry also deleted).
	if _, ok := cache.get("token-a", now.Add(6*time.Second)); ok {
		t.Fatal("expected miss at 6s (past tokenExp)")
	}

	// Subsequent get for the same key must still miss — the prior
	// get should have evicted the stale entry.
	if _, ok := cache.get("token-a", now.Add(6*time.Second)); ok {
		t.Fatal("expected miss on subsequent lookup after eviction")
	}
}

// TestOIDCVerifyCache_CeilingTTL asserts the hard ceiling on cache TTL.
// Even if a token's exp is an hour from now, the cache must not hold
// the entry past oidcVerifyCacheMaxTTL — this is the JWKS rotation
// safety margin.
func TestOIDCVerifyCache_CeilingTTL(t *testing.T) {
	cache := newOIDCVerifyCache(16)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	// Token exp = 1 hour from now. Cache TTL should be capped at
	// oidcVerifyCacheMaxTTL regardless.
	tokenExp := now.Add(1 * time.Hour).Unix()
	cache.put("token-a", Principal{Subject: "alice"}, tokenExp, now)

	// At ceiling - 1s: hit.
	if _, ok := cache.get("token-a", now.Add(oidcVerifyCacheMaxTTL-time.Second)); !ok {
		t.Fatal("expected hit just inside the ceiling")
	}

	// At ceiling + 1s: miss (ceiling enforced even though tokenExp is much later).
	if _, ok := cache.get("token-a", now.Add(oidcVerifyCacheMaxTTL+time.Second)); ok {
		t.Fatal("expected miss past the ceiling")
	}
}

// TestOIDCVerifyCache_BoundedSize asserts the cache refuses to grow
// beyond its configured max. Eviction is random (Go map iteration
// order) so the test asserts the invariant, not which specific entry
// survives.
func TestOIDCVerifyCache_BoundedSize(t *testing.T) {
	const max = 4
	cache := newOIDCVerifyCache(max)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	tokenExp := now.Add(10 * time.Minute).Unix()

	for i := 0; i < max*3; i++ {
		cache.put(fmt.Sprintf("token-%d", i), Principal{Subject: "s"}, tokenExp, now)
	}

	// Expect the map never exceeded max during the puts. We can only
	// observe the final state, so walk the entries via reflection-free
	// len() under the cache's own mutex.
	cache.mu.Lock()
	size := len(cache.entries)
	cache.mu.Unlock()
	if size > max {
		t.Fatalf("cache exceeded max size: got %d, want ≤ %d", size, max)
	}
}

// TestOIDCVerifyCache_NilReceiverSafe documents the nil-safe contract
// so call sites do not need guards. Both get and put on a nil receiver
// must not panic.
func TestOIDCVerifyCache_NilReceiverSafe(t *testing.T) {
	var cache *oidcVerifyCache
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	if _, ok := cache.get("anything", now); ok {
		t.Error("nil cache get should miss")
	}
	// Must not panic.
	cache.put("anything", Principal{Subject: "x"}, now.Add(time.Minute).Unix(), now)
}

// TestOIDCVerifyCache_RefusesExpiredWrite confirms that put silently
// drops an already-expired token. Prevents accidentally seeding the
// cache with a stale entry that would be immediately evicted on the
// next get — wasted work for no benefit.
func TestOIDCVerifyCache_RefusesExpiredWrite(t *testing.T) {
	cache := newOIDCVerifyCache(16)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-1 * time.Minute).Unix()

	cache.put("token-a", Principal{Subject: "alice"}, expired, now)

	cache.mu.Lock()
	size := len(cache.entries)
	cache.mu.Unlock()
	if size != 0 {
		t.Fatalf("expected expired put to be dropped, cache size = %d", size)
	}
}
