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

const DefaultTokenTTL = 5 * time.Minute

type Payload struct {
	ToolName    string
	WorkspaceID string
	RiskClass   string
	ArgsHash    string
	PreviewHash string
}

type TokenStoreOptions struct {
	TTL time.Duration
	Now func() time.Time
}

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
