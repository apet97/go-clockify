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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/enforcement"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/ratelimit"
	"github.com/apet97/go-clockify/internal/tools"
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
	"ratelimit-reap-correctness": {
		description: "2 tenants; noisy tenant[0] saturates its per-token budget, " +
			"idles past one rate-limit window, then resumes. After the reap, " +
			"the noisy tenant must regain full budget; the cold tenant must " +
			"be unaffected throughout. Uses reapTwoPhase below.",
		tenants:             2,
		callsPerQuiet:       20,
		pacingQuiet:         2 * time.Millisecond,
		noisyIdx:            0,
		noisyFactor:         5,
		pacingNoisy:         1 * time.Millisecond,
		globalMaxConcurrent: 50,
		globalMaxPerWindow:  1000,
		// Short window so the reap completes in seconds, not minutes.
		windowMillis:        1_500,
		perTokMaxConcurrent: 8,
		perTokMaxPerWindow:  20,
	},
	"tenant-churn": {
		description: "50 short-lived tenants run in waves, then idle long enough " +
			"to be reaped. Verifies the per-subject limiter map drains instead " +
			"of growing without bound under owner-key multi-workspace churn.",
		tenants:             50,
		callsPerQuiet:       10,
		pacingQuiet:         1 * time.Millisecond,
		noisyIdx:            -1,
		globalMaxConcurrent: 50,
		globalMaxPerWindow:  1000,
		windowMillis:        500,
		perTokMaxConcurrent: 8,
		perTokMaxPerWindow:  20,
	},
	"transport-fan-out": {
		description: "8 goroutines issue real MCP tools/call messages for a read-only " +
			"Clockify tool against a fake upstream. Exercises JSON-RPC dispatch, " +
			"schema validation, enforcement, rate limiting, tool handling, and HTTP.",
		tenants:             8,
		callsPerQuiet:       25,
		pacingQuiet:         0,
		noisyIdx:            -1,
		globalMaxConcurrent: 32,
		globalMaxPerWindow:  1000,
		windowMillis:        60_000,
		perTokMaxConcurrent: 8,
		perTokMaxPerWindow:  100,
	},
	"upstream-slow": {
		description: "4 tenants share a slow 50 ms upstream; tenant[0] has a " +
			"larger concurrent worker pool. Verifies slow successful calls do " +
			"not let one subject pin the global semaphore or starve quiet tenants.",
		tenants:             4,
		callsPerQuiet:       4,
		pacingQuiet:         0,
		noisyIdx:            0,
		globalMaxConcurrent: 32,
		globalMaxPerWindow:  1000,
		windowMillis:        60_000,
		perTokMaxConcurrent: 2,
		perTokMaxPerWindow:  100,
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

	if *scenarioName == "ratelimit-reap-correctness" {
		runReapTwoPhase(*scenarioName, rl, &sc)
		return
	}
	if *scenarioName == "tenant-churn" {
		runTenantChurn(*scenarioName, rl, &sc)
		return
	}
	if *scenarioName == "transport-fan-out" {
		runTransportFanOut(*scenarioName, rl, &sc)
		return
	}
	if *scenarioName == "upstream-slow" {
		runUpstreamSlow(*scenarioName, rl, &sc)
		return
	}

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

func runUpstreamSlow(name string, rl *ratelimit.RateLimiter, sc *scenario) {
	const (
		quietWorkers = 2
		noisyWorkers = 12
		workDuration = 50 * time.Millisecond
	)
	results := make([]*tenantResult, sc.tenants)
	for i := range results {
		results[i] = &tenantResult{subject: fmt.Sprintf("tenant-%d", i)}
	}

	var wg sync.WaitGroup
	start := time.Now()
	for tenant := 0; tenant < sc.tenants; tenant++ {
		workers := quietWorkers
		if tenant == sc.noisyIdx {
			workers = noisyWorkers
		}
		for worker := 0; worker < workers; worker++ {
			tenant := tenant
			wg.Add(1)
			go func() {
				defer wg.Done()
				runSlowWorker(rl, sc, results[tenant], workDuration)
			}()
		}
	}
	wg.Wait()
	elapsed := time.Since(start)
	for _, result := range results {
		result.effectiveDuration = elapsed
		if elapsed > 0 {
			result.observedQPS = float64(result.success) / elapsed.Seconds()
		}
	}

	printResults(name, elapsed, results)
	fmt.Println("=== acceptance check (upstream-slow) ===")
	noisy := results[sc.noisyIdx]
	quietSuccess := int64(0)
	quietRejected := int64(0)
	for idx, result := range results {
		if idx == sc.noisyIdx {
			continue
		}
		quietSuccess += result.success
		quietRejected += result.rejectedGlobal + result.rejectedPerToken + result.rejectedOther
	}
	fmt.Printf("noisy tenant per-token rejections: %d\n", noisy.rejectedPerToken)
	fmt.Printf("quiet tenants success/rejected: %d/%d\n", quietSuccess, quietRejected)
	if noisy.rejectedPerToken > 0 && quietSuccess == int64((sc.tenants-1)*quietWorkers*sc.callsPerQuiet) && quietRejected == 0 {
		fmt.Println("PASS — slow noisy tenant was isolated; quiet tenants completed")
		return
	}
	fmt.Println("FAIL — slow-upstream isolation not observed")
	log.Fatal("upstream-slow acceptance check failed")
}

func runSlowWorker(rl *ratelimit.RateLimiter, sc *scenario, out *tenantResult, workDuration time.Duration) {
	for i := 0; i < sc.callsPerQuiet; i++ {
		atomic.AddInt64(&out.totalAttempts, 1)
		release, scope, err := rl.AcquireForSubject(context.Background(), out.subject)
		if err != nil {
			recordRejection(out, scope, err)
			continue
		}
		time.Sleep(workDuration)
		release()
		atomic.AddInt64(&out.success, 1)
	}
}

func runTransportFanOut(name string, rl *ratelimit.RateLimiter, sc *scenario) {
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(previousLogger)

	var upstreamHits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws-load/tags" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"tag-load","name":"load"}]`))
	}))
	defer upstream.Close()

	client := clockify.NewClient("test-api-key", upstream.URL, 5*time.Second, 0)
	defer client.Close()

	svc := tools.New(client, "ws-load")
	server := mcp.NewServer("load-transport-fan-out", svc.Registry(), &enforcement.Pipeline{
		Policy:    &policy.Policy{Mode: policy.TimeTrackingSafe},
		RateLimit: rl,
	}, nil)
	initMsg := mustMarshalLoad(mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": mcp.SupportedProtocolVersions[0],
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "load-transport-fan-out", "version": "0"},
		},
	})
	if raw, err := server.DispatchMessage(context.Background(), initMsg); err != nil {
		log.Fatalf("initialize dispatch failed: %v", err)
	} else if hasRPCError(raw) {
		log.Fatalf("initialize returned error: %s", raw)
	}

	type callResult struct {
		subject string
		latency time.Duration
		err     string
	}
	totalCalls := sc.tenants * sc.callsPerQuiet
	results := make(chan callResult, totalCalls)
	var requestID atomic.Int64
	requestID.Store(1)

	start := time.Now()
	var wg sync.WaitGroup
	for worker := 0; worker < sc.tenants; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			subject := fmt.Sprintf("tenant-%d", worker)
			ctx := authn.WithPrincipal(context.Background(), &authn.Principal{
				Subject:  subject,
				TenantID: "ws-load",
				AuthMode: authn.ModeStaticBearer,
			})
			for i := 0; i < sc.callsPerQuiet; i++ {
				id := requestID.Add(1)
				msg := mustMarshalLoad(mcp.Request{
					JSONRPC: "2.0",
					ID:      id,
					Method:  "tools/call",
					Params: mcp.ToolCallParams{
						Name:      "clockify_list_tags",
						Arguments: map[string]any{"page": 1, "page_size": 50},
					},
				})
				t0 := time.Now()
				raw, err := server.DispatchMessage(ctx, msg)
				latency := time.Since(t0)
				if err != nil {
					results <- callResult{subject: subject, latency: latency, err: err.Error()}
					continue
				}
				if rpcErr := responseError(raw); rpcErr != "" {
					results <- callResult{subject: subject, latency: latency, err: rpcErr}
					continue
				}
				results <- callResult{subject: subject, latency: latency}
			}
		}()
	}
	wg.Wait()
	close(results)
	elapsed := time.Since(start)

	latencies := make([]time.Duration, 0, totalCalls)
	failures := 0
	perTenant := map[string]int{}
	for result := range results {
		perTenant[result.subject]++
		latencies = append(latencies, result.latency)
		if result.err != "" {
			failures++
			if failures <= 5 {
				fmt.Printf("failure[%d] subject=%s err=%s\n", failures, result.subject, result.err)
			}
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := percentileDuration(latencies, 0.50)
	p95 := percentileDuration(latencies, 0.95)
	p99 := percentileDuration(latencies, 0.99)

	fmt.Printf("scenario=%s duration=%s calls=%d failures=%d upstream_hits=%d\n",
		name, elapsed, totalCalls, failures, upstreamHits.Load())
	fmt.Printf("latency p50=%s p95=%s p99=%s\n", p50, p95, p99)
	fmt.Println("\nper-tenant calls:")
	names := make([]string, 0, len(perTenant))
	for subject := range perTenant {
		names = append(names, subject)
	}
	sort.Strings(names)
	for _, subject := range names {
		fmt.Printf("  %-12s %8d\n", subject, perTenant[subject])
	}
	fmt.Println("\n=== acceptance check (transport-fan-out) ===")
	if failures == 0 && int(upstreamHits.Load()) == totalCalls {
		fmt.Println("PASS — concurrent tools/call fan-out completed through fake upstream")
		return
	}
	fmt.Println("FAIL — transport fan-out had failed dispatches or missing upstream hits")
	log.Fatal("transport-fan-out acceptance check failed")
}

func mustMarshalLoad(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("marshal load request: %v", err)
	}
	return b
}

func hasRPCError(raw []byte) bool {
	return responseError(raw) != ""
}

func responseError(raw []byte) string {
	var resp struct {
		Error  *mcp.RPCError  `json:"error,omitempty"`
		Result map[string]any `json:"result,omitempty"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("decode response: %v", err)
	}
	if resp.Error != nil {
		return fmt.Sprintf("rpc %d: %s", resp.Error.Code, resp.Error.Message)
	}
	if isErr, _ := resp.Result["isError"].(bool); isErr {
		return fmt.Sprintf("tool error: %v", resp.Result["content"])
	}
	return ""
}

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	idx := int(float64(len(values)-1) * percentile)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

// runTenantChurn runs short-lived tenants in waves and explicitly reaps
// between waves. This models owner-key deployments where many workspace or
// principal subjects appear briefly, issue a few calls, then disappear.
func runTenantChurn(name string, rl *ratelimit.RateLimiter, sc *scenario) {
	const waveSize = 10
	maxIdle := 120 * time.Millisecond
	start := time.Now()
	var allResults []*tenantResult
	totalEvicted := 0
	maxSubjects := 0

	for offset := 0; offset < sc.tenants; offset += waveSize {
		limit := offset + waveSize
		if limit > sc.tenants {
			limit = sc.tenants
		}
		waveResults := make([]*tenantResult, 0, limit-offset)
		var wg sync.WaitGroup
		for i := offset; i < limit; i++ {
			result := &tenantResult{subject: fmt.Sprintf("tenant-%d", i)}
			waveResults = append(waveResults, result)
			wg.Add(1)
			go runTenant(&wg, rl, sc, i, result)
		}
		wg.Wait()
		allResults = append(allResults, waveResults...)
		if count := rl.SubjectCount(); count > maxSubjects {
			maxSubjects = count
		}

		time.Sleep(maxIdle + 50*time.Millisecond)
		totalEvicted += rl.ReapIdleSubjects(time.Now(), maxIdle)
	}

	elapsed := time.Since(start)
	printResults(name, elapsed, allResults)
	finalSubjects := rl.SubjectCount()
	fmt.Println("=== acceptance check (tenant-churn) ===")
	fmt.Printf("max tracked subjects: %d\n", maxSubjects)
	fmt.Printf("evicted subjects: %d\n", totalEvicted)
	fmt.Printf("final tracked subjects: %d\n", finalSubjects)
	if totalEvicted >= sc.tenants && finalSubjects == 0 {
		fmt.Println("PASS — short-lived tenant subjects were reaped")
		return
	}
	fmt.Println("FAIL — tenant subject map did not drain")
	log.Fatal("tenant-churn acceptance check failed")
}

// runReapTwoPhase runs the scenario in two phases separated by an
// idle window so the per-token budget reaper can expire the noisy
// tenant's window. Phase 1: noisy tenant saturates; cold tenant runs
// unaffected. Idle: everyone sleeps for >1 window. Phase 2: noisy
// tenant runs again and must observe substantially fewer per-token
// rejections (budget reaped) while the cold tenant remains unaffected.
func runReapTwoPhase(name string, rl *ratelimit.RateLimiter, sc *scenario) {
	phase := func(label string) []*tenantResult {
		res := make([]*tenantResult, sc.tenants)
		var wg sync.WaitGroup
		for i := 0; i < sc.tenants; i++ {
			res[i] = &tenantResult{subject: fmt.Sprintf("tenant-%d", i)}
			wg.Add(1)
			go runTenant(&wg, rl, sc, i, res[i])
		}
		wg.Wait()
		fmt.Printf("--- %s ---\n", label)
		printResults(name+":"+label, 0, res)
		return res
	}

	p1 := phase("phase1")
	// Sleep past one full window so the rate-limit window slides
	// forward and the noisy tenant's exhausted budget re-opens.
	idle := time.Duration(sc.windowMillis)*time.Millisecond + 250*time.Millisecond
	fmt.Printf("idling %s for reap window ...\n\n", idle)
	time.Sleep(idle)
	p2 := phase("phase2")

	var p1Noisy, p2Noisy int64
	var p1Cold, p2Cold int64
	for _, r := range p1 {
		if r.subject == "tenant-0" {
			p1Noisy = r.rejectedPerToken
		} else {
			p1Cold += r.rejectedPerToken
		}
	}
	for _, r := range p2 {
		if r.subject == "tenant-0" {
			p2Noisy = r.rejectedPerToken
		} else {
			p2Cold += r.rejectedPerToken
		}
	}
	fmt.Println("=== acceptance check (ratelimit-reap-correctness) ===")
	fmt.Printf("phase1 noisy per-token rejections: %d\n", p1Noisy)
	fmt.Printf("phase2 noisy per-token rejections: %d (should be <= phase1)\n", p2Noisy)
	fmt.Printf("phase1+phase2 cold per-token rejections: %d / %d (should stay near 0)\n", p1Cold, p2Cold)
	if p1Noisy > 0 && p2Noisy <= p1Noisy && p1Cold == 0 && p2Cold == 0 {
		fmt.Println("PASS — noisy tenant's budget reaped after idle window; cold tenant unaffected")
		return
	}
	fmt.Println("FAIL — reap/isolation not observed")
	log.Fatal("ratelimit-reap-correctness acceptance check failed")
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
			recordRejection(out, scope, err)
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

func recordRejection(out *tenantResult, scope ratelimit.PerTokenScope, err error) {
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
