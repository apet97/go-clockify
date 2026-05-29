package safety

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DefaultTokenTTL is the default lifetime of a confirmation token
// issued by TokenStore.Issue when TokenStoreOptions.TTL is unset or
// non-positive. Short enough that a stale preview cannot be confirmed
// out-of-band; long enough for an operator to read the dry-run
// result and decide whether to proceed.
const DefaultTokenTTL = 5 * time.Minute

// Payload is the immutable per-call identity bound into a
// confirmation token. Equality of all five fields is required for
// TokenStore.Validate to accept the token, so any drift between
// dry-run and confirm (different workspace, args, or preview)
// invalidates the token rather than silently executing the wrong
// operation.
type Payload struct {
	ToolName    string
	WorkspaceID string
	RiskClass   string
	ArgsHash    string
	PreviewHash string
}

// TokenStoreOptions configures TokenStore. TTL overrides the
// default token lifetime (see DefaultTokenTTL). Now is the clock
// source used for issue/expire computations; nil falls back to
// time.Now (tests inject a fake clock here).
type TokenStoreOptions struct {
	TTL time.Duration
	Now func() time.Time
}

// TokenStore issues single-use confirmation tokens bound to a
// Payload and rejects re-use, expiry, or payload mismatch. Safe for
// concurrent use; tokens are held only in memory (the single-user
// stdio product does not persist them across restarts).
type TokenStore struct {
	mu     sync.Mutex
	ttl    time.Duration
	now    func() time.Time
	tokens map[string]issuedToken
}

type issuedToken struct {
	payload   Payload
	expiresAt time.Time
}

// NewTokenStore returns a ready-to-use TokenStore with the given
// options. TTL defaults to DefaultTokenTTL when zero or negative; Now
// defaults to time.Now when nil.
func NewTokenStore(opts TokenStoreOptions) *TokenStore {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = DefaultTokenTTL
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &TokenStore{
		ttl:    ttl,
		now:    now,
		tokens: make(map[string]issuedToken),
	}
}

// Issue generates a new confirmation token bound to payload and
// returns the token string, the canonical preview hash of the
// payload, and the absolute expiry. Callers include the token in
// the dry-run response and the preview hash in the audit log so a
// follow-up confirm call can be matched against the original
// preview. Returns an error when the underlying CSPRNG fails.
func (s *TokenStore) Issue(payload Payload) (token, previewHash string, expiresAt time.Time, err error) {
	if s == nil {
		return "", "", time.Time{}, errors.New("confirmation token store is not configured")
	}
	token, err = randomToken()
	if err != nil {
		return "", "", time.Time{}, err
	}
	previewHash = HashCanonical(payload)
	expiresAt = s.now().Add(s.ttl)
	s.mu.Lock()
	s.tokens[token] = issuedToken{payload: payload, expiresAt: expiresAt}
	s.mu.Unlock()
	return token, previewHash, expiresAt, nil
}

// Validate consumes token if it was issued for an identical
// payload and has not expired. The token is removed atomically so
// any subsequent Validate call with the same string fails — a
// confirmation token is single-use. Returns an error when the
// token is unknown, expired, or bound to a different payload.
func (s *TokenStore) Validate(token string, payload Payload) error {
	if s == nil {
		return errors.New("confirmation token store is not configured")
	}
	s.mu.Lock()
	issued, ok := s.tokens[token]
	if ok {
		delete(s.tokens, token)
	}
	s.mu.Unlock()
	if !ok {
		return errors.New("confirmation token was not issued or was already used")
	}
	if !s.now().Before(issued.expiresAt) {
		return errors.New("confirmation token expired")
	}
	if issued.payload != payload {
		return fmt.Errorf("confirmation token does not match this tool call")
	}
	return nil
}

// TokenAuditSuffix returns a short SHA-256-derived suffix that
// uniquely identifies a confirmation token in audit logs without
// exposing the token's secret bytes. 12 hex chars (48 bits) is
// enough to distinguish concurrent tokens in a single-user store
// while keeping log noise minimal.
func TokenAuditSuffix(token string) string {
	sum := sha256.Sum256([]byte(token))
	encoded := hex.EncodeToString(sum[:])
	return encoded[:12]
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
