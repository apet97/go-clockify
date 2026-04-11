// Package main is the clockify-mcp load harness. It drives the
// per-token rate limiter under configurable scenarios and reports
// aggregate throughput + per-tenant rejection rates.
//
// Unlike the e2e tests under tests/e2e_live_test.go the harness does
// not need real Clockify credentials. It exercises the three-layer
// rate limiter (global semaphore, global window, per-subject sub-layer)
// directly via RateLimiter.AcquireForSubject, which is the same entry
// point enforcement.Pipeline.BeforeCall uses in production after
// reading the Principal off the request context.
//
// Usage:
//
//	go run ./tests/load -scenario per-token-saturation
//
// Scenarios are defined below; add new rows to the `scenarios` map to
// explore custom mixes. Every scenario prints:
//
//   - total runtime
//   - total successes / rejections
//   - per-tenant success + rejection counters
//   - observed global QPS
//
// The acceptance criterion for W2-09 is that the per-token-saturation
// scenario shows the noisy tenant getting a large share of the
// rejections while quiet tenants keep flowing.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apet97/go-clockify/internal/ratelimit"
)

// scenario describes a synthetic workload. All durations are in the
// driver's reference frame — the rate limiter operates on wall time.
type scenario struct {
	description   string
	tenants       int           // number of concurrent tenants
	callsPerQuiet int           // calls each quiet tenant attempts
	pacingQuiet   time.Duration // delay between calls for quiet tenants
	noisyIdx      int           // 0-based index of the noisy tenant; -1 for none
	noisyFactor   int           // noisy tenant fires N× the quiet call count
	pacingNoisy   time.Duration // delay between calls for the noisy tenant

	// RateLimiter configuration.
	globalMaxConcurrent int   // global semaphore size
	globalMaxPerWindow  int64 // global window cap (calls per window)
	windowMillis        int64 // rate-limit window length
	perTokMaxConcurrent int   // per-subject concurrency cap
	perTokMaxPerWindow  int64 // per-subject window cap
}

var scenarios = map[string]scenario{
	"steady": {
		description:         "5 tenants at a flat 20 calls each; no noisy tenant",
		tenants:             5,
		callsPerQuiet:       20,
		pacingQuiet:         5 * time.Millisecond,
		noisyIdx:            -1,
		globalMaxConcurrent: 50,
		globalMaxPerWindow:  500,
		windowMillis:        60_000,
		perTokMaxConcurrent: 10,
		perTokMaxPerWindow:  100,
	},
	"burst": {
		description:         "5 tenants fire 50 calls each back-to-back",
		tenants:             5,
		callsPerQuiet:       50,
		pacingQuiet:         0,
		noisyIdx:            -1,
		globalMaxConcurrent: 20,
		globalMaxPerWindow:  400,
		windowMillis:        60_000,
		perTokMaxConcurrent: 8,
		perTokMaxPerWindow:  100,
	},
	"tenant-mix": {
		description:         "10 tenants; tenant[0] fires 5× the call volume of others",
		tenants:             10,
		callsPerQuiet:       30,
		pacingQuiet:         5 * time.Millisecond,
		noisyIdx:            0,
		noisyFactor:         5,
		pacingNoisy:         2 * time.Millisecond,
		globalMaxConcurrent: 30,
		globalMaxPerWindow:  600,
		windowMillis:        60_000,
		perTokMaxConcurrent: 8,
		perTokMaxPerWindow:  80,
	},
	"per-token-saturation": {
		description: "4 tenants; noisy tenant[0] fires 10× the volume and " +
			"is expected to exhaust its per-token budget while the other " +
			"three tenants keep flowing. This is the W2-09 acceptance scenario.",
		tenants:             4,
		callsPerQuiet:       30,
		pacingQuiet:         10 * time.Millisecond,
		noisyIdx:            0,
		noisyFactor:         10,
		pacingNoisy:         1 * time.Millisecond,
		globalMaxConcurrent: 50,
		globalMaxPerWindow:  1000,
		windowMillis:        60_000,
		perTokMaxConcurrent: 4,
		perTokMaxPerWindow:  40,
	},
}

type tenantResult struct {
	subject           string
	success           int64
	rejectedGlobal    int64
	rejectedPerToken  int64
	rejectedOther     int64
	totalAttempts     int64
	observedQPS       float64
	effectiveDuration time.Duration
}

func main() {
	scenarioName := flag.String("scenario", "steady", "scenario name; see the `scenarios` map in source for the full list")
	listScenarios := flag.Bool("list", false, "print the scenario catalog and exit")
	flag.Parse()

	if *listScenarios {
		printScenarios()
		return
	}

	sc, ok := scenarios[*scenarioName]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown scenario %q\n\n", *scenarioName)
		printScenarios()
		os.Exit(2)
	}

	fmt.Printf("=== scenario: %s ===\n%s\n\n", *scenarioName, sc.description)

	// Build the rate limiter with the scenario-specific caps. Using
	// NewWithAcquireTimeout with a short timeout so the noisy tenant
	// sees concurrency rejections fast instead of blocking on the
	// global semaphore.
	rl := ratelimit.NewWithAcquireTimeout(
		sc.globalMaxConcurrent,
		sc.globalMaxPerWindow,
		sc.windowMillis,
		50*time.Millisecond,
	)
	rl.SetPerTokenLimits(sc.perTokMaxConcurrent, sc.perTokMaxPerWindow)

	results := make([]*tenantResult, sc.tenants)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < sc.tenants; i++ {
		results[i] = &tenantResult{subject: fmt.Sprintf("tenant-%d", i)}
		wg.Add(1)
		go runTenant(&wg, rl, &sc, i, results[i])
	}
	wg.Wait()
	elapsed := time.Since(start)

	printResults(*scenarioName, elapsed, results)
	checkAcceptance(*scenarioName, results)
}

func runTenant(wg *sync.WaitGroup, rl *ratelimit.RateLimiter, sc *scenario, idx int, out *tenantResult) {
	defer wg.Done()
	calls := sc.callsPerQuiet
	pacing := sc.pacingQuiet
	if idx == sc.noisyIdx {
		if sc.noisyFactor > 0 {
			calls *= sc.noisyFactor
		}
		if sc.pacingNoisy > 0 {
			pacing = sc.pacingNoisy
		}
	}
	tenantStart := time.Now()
	for j := 0; j < calls; j++ {
		if pacing > 0 {
			time.Sleep(pacing)
		}
		atomic.AddInt64(&out.totalAttempts, 1)
		ctx := context.Background()
		release, scope, err := rl.AcquireForSubject(ctx, out.subject)
		if err != nil {
			// Classify the rejection.
			var rle *ratelimit.RateLimitError
			var cle *ratelimit.ConcurrencyLimitError
			switch {
			case scope == ratelimit.ScopePerToken:
				atomic.AddInt64(&out.rejectedPerToken, 1)
			case errors.As(err, &rle) || errors.As(err, &cle):
				atomic.AddInt64(&out.rejectedGlobal, 1)
			default:
				atomic.AddInt64(&out.rejectedOther, 1)
			}
			continue
		}
		// Simulate a tiny amount of work so goroutines overlap in the
		// concurrency layer — without this the calls would race through
		// the rate limiter so fast the semaphore is a no-op.
		time.Sleep(200 * time.Microsecond)
		release()
		atomic.AddInt64(&out.success, 1)
	}
	out.effectiveDuration = time.Since(tenantStart)
	if out.effectiveDuration > 0 {
		out.observedQPS = float64(out.success) / out.effectiveDuration.Seconds()
	}
}

func printResults(name string, elapsed time.Duration, results []*tenantResult) {
	var totalSuccess, totalRejected int64
	for _, r := range results {
		totalSuccess += r.success
		totalRejected += r.rejectedGlobal + r.rejectedPerToken + r.rejectedOther
	}
	fmt.Printf("scenario=%s duration=%s success=%d rejected=%d\n",
		name, elapsed, totalSuccess, totalRejected)
	fmt.Println("\nper-tenant breakdown:")
	fmt.Printf("  %-12s %8s %8s %8s %8s %10s\n",
		"tenant", "attempts", "success", "rej(pt)", "rej(gl)", "obs_qps")
	sort.Slice(results, func(i, j int) bool { return results[i].subject < results[j].subject })
	for _, r := range results {
		fmt.Printf("  %-12s %8d %8d %8d %8d %10.2f\n",
			r.subject, r.totalAttempts, r.success, r.rejectedPerToken, r.rejectedGlobal, r.observedQPS)
	}
	fmt.Println()
}

func checkAcceptance(name string, results []*tenantResult) {
	if name != "per-token-saturation" {
		return
	}
	var noisyPerTok int64
	quietPerTok := int64(0)
	quietCount := 0
	for _, r := range results {
		if r.subject == "tenant-0" {
			noisyPerTok = r.rejectedPerToken
			continue
		}
		quietPerTok += r.rejectedPerToken
		quietCount++
	}
	fmt.Printf("=== acceptance check (per-token-saturation) ===\n")
	fmt.Printf("noisy tenant-0 per-token rejections: %d\n", noisyPerTok)
	avgQuiet := float64(0)
	if quietCount > 0 {
		avgQuiet = float64(quietPerTok) / float64(quietCount)
	}
	fmt.Printf("quiet tenants avg per-token rejections: %.2f\n", avgQuiet)
	if noisyPerTok > int64(avgQuiet*3) && noisyPerTok > 10 {
		fmt.Println("PASS — noisy tenant isolated; quiet tenants kept flowing")
		return
	}
	fmt.Println("FAIL — isolation not observed")
	log.Fatal("per-token isolation acceptance check failed")
}

func printScenarios() {
	names := make([]string, 0, len(scenarios))
	for n := range scenarios {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Println("available scenarios:")
	for _, n := range names {
		fmt.Printf("  - %-22s %s\n", n, scenarios[n].description)
	}
}
