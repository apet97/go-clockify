package authn

import (
	"crypto/sha256"
	"sync"
	"time"
)

// oidcVerifyCacheMaxTTL is the hard ceiling on how long a cached verify
// result may live. Kept shorter than any realistic JWKS key rotation
// window so a rotated key naturally invalidates within one tick — we
// do not hook the JWKS refresh path directly to avoid cross-component
// coupling; the ceiling provides the same correctness guarantee.
const oidcVerifyCacheMaxTTL = 60 * time.Second

// oidcVerifyCacheSize is the bounded entry cap. At 1024 entries of
// Principal+timestamp (~200B each) the cache occupies ~200KB steady
// state. Random eviction (Go map iteration is randomised) keeps the
// footprint hard-capped without LRU bookkeeping.
const oidcVerifyCacheSize = 1024

// oidcVerifyCacheEntry pairs a validated Principal with the wall-clock
// time at which the cached entry must be refreshed. The cache key
// implicitly includes the JWT kid because we hash the entire token
// string (header+claims+signature) — a key rotation produces a token
// with a different header bytes, hence a different sha256, hence no
// cache hit under the old kid.
type oidcVerifyCacheEntry struct {
	principal Principal
	expiresAt time.Time
}

// oidcVerifyCache memoises OIDC Authenticate results keyed by
// sha256(token). Present only on the OIDC auth path — other modes
// (static bearer, forward-auth, mTLS) are cheap enough that caching
// adds no value. The cache is populated after a successful verify
// and consulted before every subsequent decode/verify cycle, so the
// 53.8µs RSA verify path (per BenchmarkOIDCVerifyCached) collapses
// to a single map lookup on repeat calls with the same token.
//
// Correctness guarantees:
//   - TTL capped at oidcVerifyCacheMaxTTL. Set at populate time to
//     min(token.exp, now + ceiling).
//   - Expired entries are deleted on observation, not via a reaper —
//     this keeps the hot-path Get wait-free in the common case.
//   - Key rotation is handled by hashing the full token: any change
//     in header.kid produces a fresh key, so a rotated kid's cached
//     entries become unreachable.
//   - Token revocation is NOT modelled; revocations propagate after
//     the TTL ceiling expires. A shorter ceiling should be chosen if
//     the deployment requires revocation faster than 60s.
type oidcVerifyCache struct {
	mu      sync.Mutex
	entries map[[sha256.Size]byte]oidcVerifyCacheEntry
	max     int
}

func newOIDCVerifyCache(max int) *oidcVerifyCache {
	if max <= 0 {
		max = oidcVerifyCacheSize
	}
	return &oidcVerifyCache{
		entries: make(map[[sha256.Size]byte]oidcVerifyCacheEntry, max),
		max:     max,
	}
}

// get returns the cached principal for token if present and not
// expired. A miss (absent OR expired) returns the zero Principal and
// ok=false so callers fall through to the full verify path. A nil
// receiver is safe and returns a miss so call sites do not need nil
// guards.
func (c *oidcVerifyCache) get(token string, now time.Time) (Principal, bool) {
	if c == nil {
		return Principal{}, false
	}
	key := sha256.Sum256([]byte(token))
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return Principal{}, false
	}
	if !now.Before(entry.expiresAt) {
		delete(c.entries, key)
		return Principal{}, false
	}
	return entry.principal, true
}

// put stores a validated principal for token. tokenExp is the `exp`
// claim from the JWT (or zero if absent); the effective cache TTL is
// min(tokenExp - now, oidcVerifyCacheMaxTTL). A nil receiver is safe
// and discards the write.
func (c *oidcVerifyCache) put(token string, principal Principal, tokenExp int64, now time.Time) {
	if c == nil {
		return
	}
	expiresAt := now.Add(oidcVerifyCacheMaxTTL)
	if tokenExp > 0 {
		tokenExpires := time.Unix(tokenExp, 0)
		if tokenExpires.Before(expiresAt) {
			expiresAt = tokenExpires
		}
	}
	// Refuse to cache an already-expired token — the subsequent get()
	// would return a miss anyway and we would have wasted a write.
	if !now.Before(expiresAt) {
		return
	}
	key := sha256.Sum256([]byte(token))
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		// Random eviction via Go's randomised map iteration order.
		// Evicting one entry per write keeps the cap stable without
		// the per-entry linked-list overhead an LRU would impose.
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}
	c.entries[key] = oidcVerifyCacheEntry{
		principal: principal,
		expiresAt: expiresAt,
	}
}
