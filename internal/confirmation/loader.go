package confirmation

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// EnvVarTokensEnabled is the on/off knob for the confirmation gate.
const EnvVarTokensEnabled = "CLOCKIFY_CONFIRMATION_TOKENS"

// EnvVarTokenSecret is the optional explicit secret. Hex- or
// base64-encoded; >=32 raw bytes after decode.
const EnvVarTokenSecret = "CLOCKIFY_CONFIRMATION_TOKEN_SECRET"

// EnvVarTokenTTL is the token lifetime. Parsed by time.ParseDuration;
// clamped to [1m,1h].
const EnvVarTokenTTL = "CLOCKIFY_CONFIRMATION_TOKEN_TTL"

// EnvVarReplayProtection controls nonce replay tracking. Empty defaults
// to enabled for hosted profiles and disabled for local/self-hosted profiles.
const EnvVarReplayProtection = "CLOCKIFY_CONFIRMATION_REPLAY_PROTECTION"

const (
	// MinTokenTTL is the lower bound on the configured token lifetime.
	// Anything shorter would make the dry-run-then-execute UX impossibly
	// race-prone (the agent's round trip plus network latency).
	MinTokenTTL = time.Minute
	// MaxTokenTTL is the upper bound. Confirmation tokens are a "you
	// previewed this recently" signal — letting them live for hours
	// would erode the value of the dry-run-first flow.
	MaxTokenTTL = time.Hour
)

// ErrHostedSecretRequired is returned by ConfigFromEnv when the
// caller indicates a hosted/multi-replica deployment but
// CLOCKIFY_CONFIRMATION_TOKEN_SECRET is unset. A process-local
// random secret cannot survive a cross-replica request, so falling
// back silently would produce a confirmation_invalid error on every
// pod hop — worse than declining to boot.
var ErrHostedSecretRequired = errors.New("CLOCKIFY_CONFIRMATION_TOKEN_SECRET is required for hosted profiles (multi-replica deployments need a shared HMAC secret)")

// LoadResult bundles the parsed Config with operator-facing
// diagnostic notes the runtime should surface as startup log lines.
// Returning notes alongside the Config keeps the package free of
// logging dependencies; the caller decides whether to log, expose
// in /metrics, or echo to a doctor command.
type LoadResult struct {
	Config Config
	Notes  []string
}

// ConfigFromEnv reads the three CLOCKIFY_CONFIRMATION_TOKEN_* env
// vars into a Config plus diagnostic notes. The `hosted` flag drives
// the hosted-secret-required policy: hosted profiles refuse to boot
// without an explicit shared secret because process-local randomness
// cannot satisfy the stateless verification contract across replicas.
//
// Errors surface as boot-time config errors so a misconfigured
// hosted deployment fails closed instead of silently falling back to
// an ephemeral secret nobody else can verify against.
func ConfigFromEnv(hosted bool) (LoadResult, error) {
	enabled, enabledRaw := readEnabledFlag()
	if !enabled {
		return LoadResult{
			Config: Config{Enabled: false},
			Notes: []string{
				fmt.Sprintf("%s=%q: confirmation-token gate is inert; high-risk tool calls execute without a preview-first contract.", EnvVarTokensEnabled, enabledRaw),
			},
		}, nil
	}

	var notes []string
	replayProtection, replayRaw := readReplayProtection(hosted)
	if replayProtection && replayRaw == "" && hosted {
		notes = append(notes, fmt.Sprintf("%s unset; enabled by default for hosted profiles.", EnvVarReplayProtection))
	}
	secretRaw := strings.TrimSpace(os.Getenv(EnvVarTokenSecret))
	var (
		secret    []byte
		ephemeral bool
	)
	switch {
	case secretRaw == "" && hosted:
		return LoadResult{}, ErrHostedSecretRequired
	case secretRaw == "":
		s, err := NewRandomSecret()
		if err != nil {
			return LoadResult{}, fmt.Errorf("confirmation: generate random secret: %w", err)
		}
		secret = s
		ephemeral = true
		notes = append(notes,
			fmt.Sprintf("%s unset; minted an ephemeral 32-byte random secret. Tokens will NOT survive a process restart or a cross-replica request.", EnvVarTokenSecret),
		)
	default:
		s, err := decodeConfigSecret(secretRaw)
		if err != nil {
			return LoadResult{}, err
		}
		secret = s
	}
	if len(secret) < MinSecretBytes {
		return LoadResult{}, fmt.Errorf("%s decoded to %d bytes; need >=%d", EnvVarTokenSecret, len(secret), MinSecretBytes)
	}

	ttl, err := readTTL()
	if err != nil {
		return LoadResult{}, err
	}

	return LoadResult{
		Config: Config{
			Enabled:          true,
			Secret:           secret,
			TTL:              ttl,
			ReplayProtection: replayProtection,
			Ephemeral:        ephemeral,
		},
		Notes: notes,
	}, nil
}

// readEnabledFlag reads CLOCKIFY_CONFIRMATION_TOKENS, defaulting to
// enabled, and accepts the same "off/disabled/0/false" pattern the
// dryrun package uses for symmetry. Empty defaults to enabled so a
// stock deployment carries the gate.
func readEnabledFlag() (enabled bool, raw string) {
	raw = strings.TrimSpace(os.Getenv(EnvVarTokensEnabled))
	switch strings.ToLower(raw) {
	case "off", "disabled", "0", "false":
		return false, raw
	}
	return true, raw
}

// readTTL parses CLOCKIFY_CONFIRMATION_TOKEN_TTL and clamps it to
// the allowed [MinTokenTTL, MaxTokenTTL] band. Returns the default
// when the env var is unset.
func readTTL() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(EnvVarTokenTTL))
	if raw == "" {
		return DefaultTTL, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", EnvVarTokenTTL, err)
	}
	if d < MinTokenTTL || d > MaxTokenTTL {
		return 0, fmt.Errorf("%s=%s out of allowed range [%s, %s]", EnvVarTokenTTL, d, MinTokenTTL, MaxTokenTTL)
	}
	return d, nil
}

func readReplayProtection(hosted bool) (enabled bool, raw string) {
	raw = strings.TrimSpace(os.Getenv(EnvVarReplayProtection))
	switch strings.ToLower(raw) {
	case "":
		return hosted, raw
	case "on", "enabled", "1", "true":
		return true, raw
	case "off", "disabled", "0", "false":
		return false, raw
	default:
		// Unknown values fail closed for hosted by enabling protection; local
		// callers still see a note-free default through the config spec/tests.
		return hosted, raw
	}
}

// decodeConfigSecret accepts hex or any of the four common base64
// encodings (standard, raw-standard, URL, raw-URL). Hex is preferred
// for shell-pasted secrets; base64 is accepted so a vault that emits
// material in either form Just Works. Returns the first successful
// decode whose result meets MinSecretBytes — preventing a short hex
// string from accidentally decoding as a valid (but too short)
// base64 buffer.
func decodeConfigSecret(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil && len(b) >= MinSecretBytes {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) >= MinSecretBytes {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(b) >= MinSecretBytes {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil && len(b) >= MinSecretBytes {
		return b, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil && len(b) >= MinSecretBytes {
		return b, nil
	}
	return nil, fmt.Errorf("%s must be hex or base64 and decode to >=%d bytes", EnvVarTokenSecret, MinSecretBytes)
}
