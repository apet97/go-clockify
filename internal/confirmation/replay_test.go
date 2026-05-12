package confirmation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryReplayStoreRejectsSecondUse(t *testing.T) {
	store := NewMemoryReplayStore()
	rec := ReplayRecord{
		Nonce:     "nonce-1",
		Tool:      "clockify_delete_invoice",
		ArgsHash:  "args",
		Tenant:    "tenant",
		Subject:   "subject",
		Session:   "session",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := store.UseConfirmationNonce(context.Background(), rec); err != nil {
		t.Fatalf("first UseConfirmationNonce: %v", err)
	}
	if err := store.UseConfirmationNonce(context.Background(), rec); !errors.Is(err, ErrTokenReplayed) {
		t.Fatalf("second UseConfirmationNonce err = %v, want ErrTokenReplayed", err)
	}
}

func TestMemoryReplayStorePurgesExpiredNonce(t *testing.T) {
	now := time.Unix(1000, 0)
	store := NewMemoryReplayStore()
	store.now = func() time.Time { return now }
	rec := ReplayRecord{
		Nonce:     "nonce-1",
		Tool:      "clockify_delete_invoice",
		ExpiresAt: now.Add(time.Second),
	}
	if err := store.UseConfirmationNonce(context.Background(), rec); err != nil {
		t.Fatalf("first UseConfirmationNonce: %v", err)
	}
	now = now.Add(2 * time.Second)
	if err := store.UseConfirmationNonce(context.Background(), rec); err != nil {
		t.Fatalf("expired nonce should be reusable after purge, got %v", err)
	}
}
