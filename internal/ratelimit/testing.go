package ratelimit

// SetPerTokenLimitsForTest mutates the per-token sub-layer on a RateLimiter.
// Production code configures these via CLOCKIFY_PER_TOKEN_* env vars through
// FromEnv(); this helper exists so other packages' tests can drive the
// per-subject path without exporting the fields themselves.
func SetPerTokenLimitsForTest(rl *RateLimiter, maxConcurrent int, maxPerWindow int64) {
	if rl == nil {
		return
	}
	rl.perTokenMaxConcurrent = maxConcurrent
	rl.perTokenMaxPerWindow = maxPerWindow
	rl.subjects = nil
}
