//go:build livee2e

package e2e_test

import (
	"sync/atomic"
	"testing"
)

// liveTestsRan is incremented by every live-contract test that clears the
// env-var skip gate. A zero value means every test in the suite skipped —
// typically because CLOCKIFY_RUN_LIVE_E2E=1 or CLOCKIFY_API_KEY is missing.
var liveTestsRan int32

// MarkLiveTestRan increments the live-contract evidence counter. Call this
// from setup helpers after the env-var skip gates pass, before the test
// body runs. A test that calls t.Skip does NOT count as "ran."
func MarkLiveTestRan() { atomic.AddInt32(&liveTestsRan, 1) }

// TestLiveContractSkipSentinel fails when every live-contract test in the
// package skipped because required env vars (CLOCKIFY_RUN_LIVE_E2E=1 +
// CLOCKIFY_API_KEY) were missing. This prevents local-shell output from
// being mistaken for live-contract evidence.
//
// CI workflows use explicit -run patterns that do NOT match this test, so
// it only fires on unfiltered local runs (the exact case where a human or
// agent might misread a silent skip as a pass).
func TestLiveContractSkipSentinel(t *testing.T) {
	if atomic.LoadInt32(&liveTestsRan) == 0 {
		t.Fatal(
			"All live-contract tests skipped — no evidence was collected.\n" +
				"Set CLOCKIFY_RUN_LIVE_E2E=1 and CLOCKIFY_API_KEY (pointing at\n" +
				"the sacrificial workspace named in docs/live-tests.md) and\n" +
				"re-run. Skipped local tests are NOT live-contract evidence.",
		)
	}
}
