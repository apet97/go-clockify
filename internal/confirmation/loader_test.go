package confirmation

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func resetEnv(t *testing.T) {
	t.Helper()
	t.Setenv(EnvVarTokensEnabled, "")
	t.Setenv(EnvVarTokenSecret, "")
	t.Setenv(EnvVarTokenTTL, "")
}

// TestConfigFromEnvDefaultsEnabled documents the safe default: any
// fresh deployment carries the gate even when the operator has not
// set any of the three env vars. Self-hosted callers get an
// ephemeral secret + a diagnostic note.
func TestConfigFromEnvDefaultsEnabled(t *testing.T) {
	resetEnv(t)
	res, err := ConfigFromEnv(false /* hosted */)
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if !res.Config.Enabled {
		t.Fatal("default Config.Enabled = false, want true")
	}
	if !res.Config.Ephemeral {
		t.Fatal("missing secret should produce an ephemeral signer for self-hosted")
	}
	if len(res.Config.Secret) < MinSecretBytes {
		t.Fatalf("ephemeral secret length = %d, want >=%d", len(res.Config.Secret), MinSecretBytes)
	}
	if res.Config.TTL != DefaultTTL {
		t.Fatalf("TTL = %s, want %s", res.Config.TTL, DefaultTTL)
	}
	if len(res.Notes) == 0 {
		t.Fatal("ephemeral fallback must emit an operator-facing note")
	}
}

// TestConfigFromEnvHostedRequiresExplicitSecret pins the
// shared-service contract: a hosted profile without an explicit
// secret refuses to boot rather than silently falling back to a
// process-local random key.
func TestConfigFromEnvHostedRequiresExplicitSecret(t *testing.T) {
	resetEnv(t)
	_, err := ConfigFromEnv(true /* hosted */)
	if !errors.Is(err, ErrHostedSecretRequired) {
		t.Fatalf("err = %v, want ErrHostedSecretRequired", err)
	}
}

// TestConfigFromEnvHostedAcceptsExplicitHexSecret verifies the hex
// path on the hosted gate.
func TestConfigFromEnvHostedAcceptsExplicitHexSecret(t *testing.T) {
	resetEnv(t)
	raw := make([]byte, MinSecretBytes)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	t.Setenv(EnvVarTokenSecret, hex.EncodeToString(raw))
	res, err := ConfigFromEnv(true)
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if res.Config.Ephemeral {
		t.Fatal("explicit secret must not be marked Ephemeral")
	}
	if string(res.Config.Secret) != string(raw) {
		t.Fatalf("decoded secret mismatch: got %x want %x", res.Config.Secret, raw)
	}
}

// TestConfigFromEnvAcceptsBase64Secret pins the base64 alternative.
// Vault material that ships in standard base64 must not need
// pre-decoding by the operator.
func TestConfigFromEnvAcceptsBase64Secret(t *testing.T) {
	resetEnv(t)
	raw := make([]byte, MinSecretBytes)
	for i := range raw {
		raw[i] = byte(0xa0 + i)
	}
	t.Setenv(EnvVarTokenSecret, base64.StdEncoding.EncodeToString(raw))
	res, err := ConfigFromEnv(false)
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if string(res.Config.Secret) != string(raw) {
		t.Fatalf("decoded secret mismatch: got %x want %x", res.Config.Secret, raw)
	}
}

// TestConfigFromEnvRejectsShortSecret guards against a paste error
// that produces a too-short decoded buffer.
func TestConfigFromEnvRejectsShortSecret(t *testing.T) {
	resetEnv(t)
	t.Setenv(EnvVarTokenSecret, "deadbeef") // 4 bytes, way below 32
	_, err := ConfigFromEnv(false)
	if err == nil {
		t.Fatal("expected short-secret error")
	}
	if !strings.Contains(err.Error(), ">=32") {
		t.Fatalf("error should mention minimum length, got %v", err)
	}
}

// TestConfigFromEnvAcceptsExplicitTTL parses
// CLOCKIFY_CONFIRMATION_TOKEN_TTL and applies it.
func TestConfigFromEnvAcceptsExplicitTTL(t *testing.T) {
	resetEnv(t)
	t.Setenv(EnvVarTokenSecret, hex.EncodeToString(make([]byte, MinSecretBytes)))
	t.Setenv(EnvVarTokenTTL, "15m")
	res, err := ConfigFromEnv(false)
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if res.Config.TTL != 15*time.Minute {
		t.Fatalf("TTL = %s, want 15m", res.Config.TTL)
	}
}

// TestConfigFromEnvRejectsOutOfRangeTTL pins the [1m,1h] clamp.
func TestConfigFromEnvRejectsOutOfRangeTTL(t *testing.T) {
	resetEnv(t)
	t.Setenv(EnvVarTokenSecret, hex.EncodeToString(make([]byte, MinSecretBytes)))
	for _, raw := range []string{"30s", "24h", "0s"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv(EnvVarTokenTTL, raw)
			_, err := ConfigFromEnv(false)
			if err == nil {
				t.Fatalf("TTL=%s must be rejected", raw)
			}
		})
	}
}

// TestConfigFromEnvDisabledFlag honours the explicit opt-out.
func TestConfigFromEnvDisabledFlag(t *testing.T) {
	resetEnv(t)
	for _, raw := range []string{"off", "disabled", "0", "false", "FALSE"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv(EnvVarTokensEnabled, raw)
			res, err := ConfigFromEnv(true /* hosted, but disabled overrides the secret requirement */)
			if err != nil {
				t.Fatalf("ConfigFromEnv: %v", err)
			}
			if res.Config.Enabled {
				t.Fatalf("Config.Enabled = true with flag=%q", raw)
			}
		})
	}
}
