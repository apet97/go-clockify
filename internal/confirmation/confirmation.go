// Package confirmation implements server-issued HMAC confirmation
// tokens for high-risk tool calls as specified by
// docs/adr/0018-risk-class-confirmation-tokens.md (Q1 option B).
//
// The contract: a high-risk tool call (RiskBilling | RiskAdmin |
// RiskPermissionChange | RiskExternalSideEffect | RiskDestructive)
// arriving with dry_run:true returns a stateless HMAC token bound to
// the tool name, the argument fingerprint (with dry_run and
// confirmation_token excluded), the risk class bitmask, the
// principal's tenant/subject/session if present, and a server-side
// expiry. A non-dry-run call to the same tool must include the
// returned token; the verifier recomputes the binding and rejects
// mismatches, expirations, and tampering. This gate is independent
// from the policy gate — policy decides whether the tool is callable
// at all; this gate decides whether the caller has previewed the
// effect first.
//
// Layout:
//   - Config carries the HMAC secret, TTL, and enable flag.
//   - Signer wraps Mint/Verify; Config.Signer() returns one.
//   - TokenClaims is the on-wire payload (compact JSON, base64url +
//     "." + base64url(HMAC(...))).
//   - BuildArgumentFingerprint stripping is in fingerprint.go.
//
// The package is stdlib-only — no new module dependencies.
package confirmation

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenVersion is the on-wire version byte. Older tokens decoded with
// a different version are rejected so a future binding-format change
// can roll forward without ambiguity.
const TokenVersion = 1

// MinSecretBytes is the minimum raw-byte length of the HMAC secret
// the signer accepts. 32 bytes (256 bits) is the SHA-256 block-fit
// minimum; rejecting shorter secrets prevents accidental misuse of a
// short literal.
const MinSecretBytes = 32

// DefaultTTL is the token expiry window when Config.TTL is unset.
const DefaultTTL = 10 * time.Minute

// Sentinel errors. Verify returns one of these so callers (the
// enforcement gate, the audit recorder) can branch on the failure
// mode without parsing strings.
var (
	// ErrTokenMalformed signals a token whose on-wire shape is wrong
	// (missing "." separator, undecodable base64, undecodable JSON).
	ErrTokenMalformed = errors.New("confirmation token malformed")
	// ErrTokenSignatureMismatch signals a token whose HMAC does not
	// match the configured secret. Includes wrong-secret and
	// payload-tampering cases.
	ErrTokenSignatureMismatch = errors.New("confirmation token signature mismatch")
	// ErrTokenExpired signals a token whose exp is at or before
	// time.Now (per the Signer's clock).
	ErrTokenExpired = errors.New("confirmation token expired")
	// ErrTokenBindingMismatch signals a token whose embedded claims
	// disagree with the verifier's expected binding (different tool
	// name, different args fingerprint, different
	// tenant/subject/session, different risk class).
	ErrTokenBindingMismatch = errors.New("confirmation token binding mismatch")
	// ErrTokenVersionUnsupported signals a token whose on-wire
	// version byte differs from TokenVersion. Rolling forward
	// requires accepting older versions explicitly; the default is
	// to reject.
	ErrTokenVersionUnsupported = errors.New("confirmation token version unsupported")
	// ErrSecretTooShort signals a Config whose secret has fewer than
	// MinSecretBytes raw bytes. Returned by Signer construction.
	ErrSecretTooShort = errors.New("confirmation token secret too short")
)

// Config carries the operator-facing knobs. Build a Signer with
// Config.Signer() to mint/verify tokens; a zero-Config Signer rejects
// every verification with ErrSecretTooShort so misuse is loud.
type Config struct {
	// Enabled toggles the gate. When false the enforcement layer
	// should skip Verify entirely and never call Mint. The package
	// itself does not inspect this — it's a contract for callers.
	Enabled bool
	// Secret is the HMAC-SHA256 key. Required when Enabled. Use
	// crypto/rand for production; a short literal is rejected at
	// Signer construction.
	Secret []byte
	// TTL is the lifetime applied to Mint'd tokens. Zero defaults to
	// DefaultTTL.
	TTL time.Duration
	// ReplayProtection enables single-use nonce tracking at the
	// enforcement layer. The signer stays stateless; callers wire a
	// ReplayStore appropriate for the runtime profile.
	ReplayProtection bool
	// Ephemeral marks a secret that was generated at startup rather
	// than supplied by the operator. The enforcement layer logs a
	// warning so operators on hosted multi-replica deployments see
	// that cross-replica verification will not survive a pod hop.
	Ephemeral bool
}

// NewRandomSecret generates a cryptographically random secret of
// MinSecretBytes bytes. Used by the runtime fallback when an operator
// has not configured CLOCKIFY_CONFIRMATION_TOKEN_SECRET.
func NewRandomSecret() ([]byte, error) {
	buf := make([]byte, MinSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("confirmation: random secret: %w", err)
	}
	return buf, nil
}

// TokenClaims is the JSON payload signed by Mint and re-derived by
// Verify. All fields are lowercase + short so the on-wire form is
// compact.
type TokenClaims struct {
	Version   int    `json:"v"`
	Tool      string `json:"tool"`
	ArgsHash  string `json:"args"`
	RiskClass uint32 `json:"risk"`
	Tenant    string `json:"tenant,omitempty"`
	Subject   string `json:"subject,omitempty"`
	Session   string `json:"session,omitempty"`
	Expires   int64  `json:"exp"`
	Nonce     string `json:"n"`
}

// MintInput names the contract for a Mint call. Tool, ArgsHash, and
// RiskClass are required; Tenant/Subject/Session are populated when
// the call has a principal in context.
type MintInput struct {
	Tool      string
	ArgsHash  string
	RiskClass uint32
	Tenant    string
	Subject   string
	Session   string
}

// VerifyInput names the contract for a Verify call. The verifier
// receives the expected binding and the on-wire token; mismatch in
// any field produces ErrTokenBindingMismatch (so the caller can log a
// single audit outcome without enumerating which field drifted).
type VerifyInput struct {
	Tool      string
	ArgsHash  string
	RiskClass uint32
	Tenant    string
	Subject   string
	Session   string
	Token     string
}

// Signer mints and verifies confirmation tokens with a fixed secret +
// TTL. It does not own the Enabled flag — that belongs to Config and
// is enforced by the call site.
type Signer struct {
	secret []byte
	ttl    time.Duration
	// now is overridable for tests; production callers leave it nil
	// and the signer uses time.Now.
	now func() time.Time
}

// NewSigner builds a Signer from a Config, returning ErrSecretTooShort
// when the secret is too short to be safe. Pass a nil time-source to
// use time.Now; tests inject their own.
func NewSigner(cfg Config) (*Signer, error) {
	if len(cfg.Secret) < MinSecretBytes {
		return nil, fmt.Errorf("%w: need %d bytes, got %d", ErrSecretTooShort, MinSecretBytes, len(cfg.Secret))
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Signer{secret: append([]byte(nil), cfg.Secret...), ttl: ttl}, nil
}

// withClock returns a copy of s using the supplied clock. Test-only.
func (s *Signer) withClock(now func() time.Time) *Signer {
	clone := *s
	clone.now = now
	return &clone
}

// clock returns the signer's clock or time.Now.
func (s *Signer) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// TTL returns the configured token lifetime; useful for logging the
// `confirmation_expires_at` field on a dry-run preview.
func (s *Signer) TTL() time.Duration { return s.ttl }

// Mint returns a compact token of the form `b64url(claims).b64url(mac)`
// bound to the input. ExpiresAt is the wall-clock instant the token
// becomes invalid; surface it to the client so the client knows when
// to re-issue the dry-run.
func (s *Signer) Mint(in MintInput) (token string, expiresAt time.Time, err error) {
	if in.Tool == "" || in.ArgsHash == "" {
		return "", time.Time{}, fmt.Errorf("confirmation: mint requires Tool and ArgsHash")
	}
	nonce, err := randomNonce()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = s.clock().Add(s.ttl).UTC()
	claims := TokenClaims{
		Version:   TokenVersion,
		Tool:      in.Tool,
		ArgsHash:  in.ArgsHash,
		RiskClass: in.RiskClass,
		Tenant:    in.Tenant,
		Subject:   in.Subject,
		Session:   in.Session,
		Expires:   expiresAt.Unix(),
		Nonce:     nonce,
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("confirmation: marshal claims: %w", err)
	}
	header := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(header))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return header + "." + signature, expiresAt, nil
}

// Verify checks a token against the expected binding. The contract is
// "constant-time on signature; field-by-field on binding". Returns
// the decoded claims when verification succeeds so callers can record
// the nonce for replay tracking if they wish; today we treat tokens
// as single-use-implicit via TTL but emit the nonce for diagnostics.
func (s *Signer) Verify(in VerifyInput) (TokenClaims, error) {
	if in.Token == "" {
		return TokenClaims{}, ErrTokenMalformed
	}
	dot := strings.LastIndexByte(in.Token, '.')
	if dot <= 0 || dot == len(in.Token)-1 {
		return TokenClaims{}, ErrTokenMalformed
	}
	header := in.Token[:dot]
	gotSig, err := base64.RawURLEncoding.DecodeString(in.Token[dot+1:])
	if err != nil {
		return TokenClaims{}, ErrTokenMalformed
	}

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(header))
	wantSig := mac.Sum(nil)
	if !hmac.Equal(wantSig, gotSig) {
		return TokenClaims{}, ErrTokenSignatureMismatch
	}

	payload, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return TokenClaims{}, ErrTokenMalformed
	}
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return TokenClaims{}, ErrTokenMalformed
	}
	if claims.Version != TokenVersion {
		return TokenClaims{}, ErrTokenVersionUnsupported
	}
	if s.clock().Unix() >= claims.Expires {
		return TokenClaims{}, ErrTokenExpired
	}
	if claims.Tool != in.Tool ||
		claims.ArgsHash != in.ArgsHash ||
		claims.RiskClass != in.RiskClass ||
		claims.Tenant != in.Tenant ||
		claims.Subject != in.Subject ||
		claims.Session != in.Session {
		return TokenClaims{}, ErrTokenBindingMismatch
	}
	return claims, nil
}

// randomNonce returns a base64url-encoded 8-byte random string.
// Nonces are not part of the binding (the args fingerprint already
// gives uniqueness within a TTL window); they are emitted purely to
// make repeated mints of the same input produce distinct tokens so
// log scraping cannot fingerprint a known-args call.
func randomNonce() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("confirmation: random nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
