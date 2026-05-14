//go:build legacy_platform

package confirmation

import (
	"errors"
	"testing"
	"time"
)

func newTestSigner(t *testing.T) *Signer {
	t.Helper()
	secret, err := NewRandomSecret()
	if err != nil {
		t.Fatalf("NewRandomSecret: %v", err)
	}
	s, err := NewSigner(Config{Enabled: true, Secret: secret, TTL: 5 * time.Minute})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

func sampleMintInput() MintInput {
	return MintInput{
		Tool:      "clockify_delete_invoice",
		ArgsHash:  BuildArgumentFingerprint(map[string]any{"invoice_id": "inv-1"}),
		RiskClass: uint32(0x40), // RiskDestructive bit
		Tenant:    "tenant-1",
		Subject:   "subject-1",
		Session:   "session-1",
	}
}

// TestMintVerifyRoundTrip is the primary positive path. A token
// minted for a binding must verify cleanly against the same binding.
func TestMintVerifyRoundTrip(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()

	tok, exp, err := s.Mint(in)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if tok == "" {
		t.Fatal("Mint returned empty token")
	}
	if !exp.After(time.Now()) {
		t.Fatalf("Mint expiry %s should be after now %s", exp, time.Now())
	}

	claims, err := s.Verify(VerifyInput{
		Tool:      in.Tool,
		ArgsHash:  in.ArgsHash,
		RiskClass: in.RiskClass,
		Tenant:    in.Tenant,
		Subject:   in.Subject,
		Session:   in.Session,
		Token:     tok,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Tool != in.Tool {
		t.Errorf("claims.Tool = %q, want %q", claims.Tool, in.Tool)
	}
	if claims.ArgsHash != in.ArgsHash {
		t.Errorf("claims.ArgsHash = %q, want %q", claims.ArgsHash, in.ArgsHash)
	}
	if claims.RiskClass != in.RiskClass {
		t.Errorf("claims.RiskClass = %d, want %d", claims.RiskClass, in.RiskClass)
	}
	if claims.Nonce == "" {
		t.Error("claims.Nonce must not be empty")
	}
	if claims.Expires != exp.Unix() {
		t.Errorf("claims.Expires %d != exp.Unix() %d", claims.Expires, exp.Unix())
	}
}

// TestMintProducesDistinctTokens verifies the per-call nonce makes
// repeated mints of identical bindings yield distinct on-wire tokens.
// Required so log scraping cannot fingerprint a known-args call.
func TestMintProducesDistinctTokens(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()
	tok1, _, err := s.Mint(in)
	if err != nil {
		t.Fatalf("Mint #1: %v", err)
	}
	tok2, _, err := s.Mint(in)
	if err != nil {
		t.Fatalf("Mint #2: %v", err)
	}
	if tok1 == tok2 {
		t.Fatalf("expected distinct tokens, got identical: %s", tok1)
	}
}

// TestVerifyFailsOnChangedTool guards the tool binding. A token
// minted for invoice deletion must not verify against a user-delete
// call even if every other field matches.
func TestVerifyFailsOnChangedTool(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()
	tok, _, err := s.Mint(in)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	vi := verifyInputFromMint(in, tok)
	vi.Tool = "clockify_delete_user_group"
	if _, err := s.Verify(vi); !errors.Is(err, ErrTokenBindingMismatch) {
		t.Fatalf("Verify err = %v, want ErrTokenBindingMismatch", err)
	}
}

// TestVerifyFailsOnChangedArgsFingerprint guards the args binding.
// The dry-run preview returns a token bound to the previewed args;
// re-submitting with mutated args must invalidate the token.
func TestVerifyFailsOnChangedArgsFingerprint(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()
	tok, _, err := s.Mint(in)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	vi := verifyInputFromMint(in, tok)
	vi.ArgsHash = BuildArgumentFingerprint(map[string]any{"invoice_id": "inv-2"})
	if _, err := s.Verify(vi); !errors.Is(err, ErrTokenBindingMismatch) {
		t.Fatalf("Verify err = %v, want ErrTokenBindingMismatch", err)
	}
}

// TestVerifyFailsOnChangedSubject / Tenant / Session each pin a
// principal-binding field. A token issued to one user/tenant/session
// must not verify when replayed from another.
func TestVerifyFailsOnChangedSubject(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()
	tok, _, _ := s.Mint(in)
	vi := verifyInputFromMint(in, tok)
	vi.Subject = "intruder"
	if _, err := s.Verify(vi); !errors.Is(err, ErrTokenBindingMismatch) {
		t.Fatalf("Verify err = %v, want ErrTokenBindingMismatch", err)
	}
}

func TestVerifyFailsOnChangedTenant(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()
	tok, _, _ := s.Mint(in)
	vi := verifyInputFromMint(in, tok)
	vi.Tenant = "other-tenant"
	if _, err := s.Verify(vi); !errors.Is(err, ErrTokenBindingMismatch) {
		t.Fatalf("Verify err = %v, want ErrTokenBindingMismatch", err)
	}
}

func TestVerifyFailsOnChangedSession(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()
	tok, _, _ := s.Mint(in)
	vi := verifyInputFromMint(in, tok)
	vi.Session = "other-session"
	if _, err := s.Verify(vi); !errors.Is(err, ErrTokenBindingMismatch) {
		t.Fatalf("Verify err = %v, want ErrTokenBindingMismatch", err)
	}
}

// TestVerifyFailsOnChangedRiskClass guards against a token issued
// for one risk class being replayed against another. An attacker who
// observes a billing token must not be able to replay it for a
// permission-change call.
func TestVerifyFailsOnChangedRiskClass(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()
	tok, _, _ := s.Mint(in)
	vi := verifyInputFromMint(in, tok)
	vi.RiskClass = 0x10 // RiskPermissionChange bit
	if _, err := s.Verify(vi); !errors.Is(err, ErrTokenBindingMismatch) {
		t.Fatalf("Verify err = %v, want ErrTokenBindingMismatch", err)
	}
}

// TestVerifyFailsOnExpiredToken uses the test-only clock hook to age
// a token past its TTL.
func TestVerifyFailsOnExpiredToken(t *testing.T) {
	s := newTestSigner(t)
	frozen := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	mintingSigner := s.withClock(func() time.Time { return frozen })
	in := sampleMintInput()
	tok, _, err := mintingSigner.Mint(in)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	verifyingSigner := s.withClock(func() time.Time {
		return frozen.Add(s.TTL() + time.Second)
	})
	vi := verifyInputFromMint(in, tok)
	if _, err := verifyingSigner.Verify(vi); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("Verify err = %v, want ErrTokenExpired", err)
	}
}

// TestVerifyFailsOnMalformedToken covers the on-wire shape errors:
// empty string, missing '.', bad base64, bad JSON.
func TestVerifyFailsOnMalformedToken(t *testing.T) {
	s := newTestSigner(t)
	in := sampleMintInput()

	cases := map[string]string{
		"empty":         "",
		"no-dot":        "justonething",
		"trailing-dot":  "header.",
		"bad-base64":    "!!!.zzz",
		"bad-base64-2":  "abcd.!!!",
		"truncated-mac": "Y2xhaW1z.YQ",
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			vi := verifyInputFromMint(in, tok)
			_, err := s.Verify(vi)
			if err == nil {
				t.Fatalf("Verify on %q must fail", tok)
			}
			// Allow either malformed or signature-mismatch — the
			// header-base64-decodable-but-mac-bad path returns
			// signature mismatch, which is also acceptable here.
			if !errors.Is(err, ErrTokenMalformed) && !errors.Is(err, ErrTokenSignatureMismatch) {
				t.Fatalf("Verify err = %v, want malformed or signature mismatch", err)
			}
		})
	}
}

// TestVerifyFailsOnWrongSecret guards the HMAC. A token minted by
// one signer must not verify against another with a different secret.
func TestVerifyFailsOnWrongSecret(t *testing.T) {
	secret1, _ := NewRandomSecret()
	secret2, _ := NewRandomSecret()
	s1, _ := NewSigner(Config{Secret: secret1})
	s2, _ := NewSigner(Config{Secret: secret2})
	in := sampleMintInput()
	tok, _, _ := s1.Mint(in)
	vi := verifyInputFromMint(in, tok)
	if _, err := s2.Verify(vi); !errors.Is(err, ErrTokenSignatureMismatch) {
		t.Fatalf("Verify err = %v, want ErrTokenSignatureMismatch", err)
	}
}

// TestNewSignerRejectsShortSecret pins the secret-length requirement.
func TestNewSignerRejectsShortSecret(t *testing.T) {
	for _, length := range []int{0, 1, 8, 16, MinSecretBytes - 1} {
		_, err := NewSigner(Config{Secret: make([]byte, length)})
		if !errors.Is(err, ErrSecretTooShort) {
			t.Errorf("NewSigner with %d-byte secret err = %v, want ErrSecretTooShort", length, err)
		}
	}
	_, err := NewSigner(Config{Secret: make([]byte, MinSecretBytes)})
	if err != nil {
		t.Errorf("NewSigner with %d-byte secret should succeed, got %v", MinSecretBytes, err)
	}
}

// TestFingerprintExcludesDryRunAndConfirmationToken pins the
// FingerprintExcludedKeys contract: adding/removing/changing those
// two keys must not change the fingerprint, so a dry-run preview and
// the subsequent execution call (which adds confirmation_token and
// removes dry_run) produce the same hash.
func TestFingerprintExcludesDryRunAndConfirmationToken(t *testing.T) {
	base := map[string]any{
		"invoice_id": "inv-1",
		"reason":     "test",
	}
	dryRun := map[string]any{
		"invoice_id": "inv-1",
		"reason":     "test",
		"dry_run":    true,
	}
	withToken := map[string]any{
		"invoice_id":         "inv-1",
		"reason":             "test",
		"confirmation_token": "made-up-token",
	}
	bothFlags := map[string]any{
		"invoice_id":         "inv-1",
		"reason":             "test",
		"dry_run":            false,
		"confirmation_token": "another-token",
	}

	want := BuildArgumentFingerprint(base)
	if got := BuildArgumentFingerprint(dryRun); got != want {
		t.Errorf("dry_run changed fingerprint: %q vs %q", got, want)
	}
	if got := BuildArgumentFingerprint(withToken); got != want {
		t.Errorf("confirmation_token changed fingerprint: %q vs %q", got, want)
	}
	if got := BuildArgumentFingerprint(bothFlags); got != want {
		t.Errorf("dry_run+confirmation_token changed fingerprint: %q vs %q", got, want)
	}
}

// TestFingerprintStableAcrossKeyOrdering verifies that whatever
// insertion order the caller uses, the canonical fingerprint is
// identical. encoding/json sorts map keys alphabetically, so this is
// a regression pin rather than a new contract.
func TestFingerprintStableAcrossKeyOrdering(t *testing.T) {
	a := map[string]any{"a": 1, "b": 2, "c": 3}
	b := map[string]any{"c": 3, "a": 1, "b": 2}
	if BuildArgumentFingerprint(a) != BuildArgumentFingerprint(b) {
		t.Fatal("fingerprint must be order-independent")
	}
}

// TestFingerprintStableForNestedMaps pins the same property for
// nested objects, which is the more interesting case in practice
// (e.g. the tag_ids array containing object values).
func TestFingerprintStableForNestedMaps(t *testing.T) {
	a := map[string]any{
		"outer": map[string]any{"a": 1, "b": 2, "c": 3},
		"list":  []any{"x", "y"},
	}
	b := map[string]any{
		"list":  []any{"x", "y"},
		"outer": map[string]any{"c": 3, "b": 2, "a": 1},
	}
	if BuildArgumentFingerprint(a) != BuildArgumentFingerprint(b) {
		t.Fatal("nested fingerprint must be order-independent")
	}
}

// TestFingerprintDifferentiatesContent is the contrapositive: when
// substantive fields change, the fingerprint must change.
func TestFingerprintDifferentiatesContent(t *testing.T) {
	a := map[string]any{"invoice_id": "inv-1"}
	b := map[string]any{"invoice_id": "inv-2"}
	if BuildArgumentFingerprint(a) == BuildArgumentFingerprint(b) {
		t.Fatal("fingerprint must change when invoice_id changes")
	}
}

// TestFingerprintHandlesNilArgs returns a deterministic hash for the
// nil-args case (some tools have empty argument shape).
func TestFingerprintHandlesNilArgs(t *testing.T) {
	want := BuildArgumentFingerprint(map[string]any{})
	if got := BuildArgumentFingerprint(nil); got != want {
		t.Fatalf("nil args = %q, want empty-args %q", got, want)
	}
}

func verifyInputFromMint(in MintInput, token string) VerifyInput {
	return VerifyInput{
		Tool:      in.Tool,
		ArgsHash:  in.ArgsHash,
		RiskClass: in.RiskClass,
		Tenant:    in.Tenant,
		Subject:   in.Subject,
		Session:   in.Session,
		Token:     token,
	}
}
