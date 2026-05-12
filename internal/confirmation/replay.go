package confirmation

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrTokenReplayed is returned when a previously accepted confirmation-token
// nonce is presented again before its expiry.
var ErrTokenReplayed = errors.New("confirmation token replayed")

// ReplayRecord is the durable fingerprint stored after token verification.
// ArgsHash is included for audit/debug correlation; uniqueness is enforced on
// tenant, subject, session, tool, and nonce.
type ReplayRecord struct {
	Nonce     string    `json:"nonce"`
	Tool      string    `json:"tool"`
	ArgsHash  string    `json:"args_hash"`
	Tenant    string    `json:"tenant,omitempty"`
	Subject   string    `json:"subject,omitempty"`
	Session   string    `json:"session,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	UsedAt    time.Time `json:"used_at"`
}

// ReplayStore atomically records a nonce on first use and returns
// ErrTokenReplayed for subsequent uses before expiry.
type ReplayStore interface {
	UseConfirmationNonce(context.Context, ReplayRecord) error
}

// MemoryReplayStore is the stdio/local replay backend. It is process-local,
// bounded by token TTL, and concurrency-safe.
type MemoryReplayStore struct {
	mu   sync.Mutex
	seen map[string]ReplayRecord
	now  func() time.Time
}

// NewMemoryReplayStore builds an empty in-memory replay cache.
func NewMemoryReplayStore() *MemoryReplayStore {
	return &MemoryReplayStore{seen: map[string]ReplayRecord{}}
}

// UseConfirmationNonce records rec unless the same nonce binding is already
// present and unexpired.
func (s *MemoryReplayStore) UseConfirmationNonce(ctx context.Context, rec ReplayRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if rec.Nonce == "" {
		return ErrTokenMalformed
	}
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	if rec.UsedAt.IsZero() {
		rec.UsedAt = now
	}
	key := replayKey(rec)
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, old := range s.seen {
		if !old.ExpiresAt.IsZero() && !old.ExpiresAt.After(now) {
			delete(s.seen, k)
		}
	}
	if old, ok := s.seen[key]; ok && (old.ExpiresAt.IsZero() || old.ExpiresAt.After(now)) {
		return ErrTokenReplayed
	}
	s.seen[key] = rec
	return nil
}

func replayKey(rec ReplayRecord) string {
	return rec.Tenant + "\x00" + rec.Subject + "\x00" + rec.Session + "\x00" + rec.Tool + "\x00" + rec.Nonce
}
