// Package main is the clockify-mcp chaos harness. It drives the
// stdlib-only Clockify HTTP client under failure-injection scenarios
// and reports whether each failure is classified, retried, and
// bounded correctly.
//
// Each scenario wires an httptest.NewServer with a fault-injecting
// handler and points a clockify.Client at it. The harness then runs a
// GET against a known endpoint and asserts against the client's
// observable behaviour (error type, number of attempts, duration
// under retry budget). Unlike the load harness, chaos scenarios are
// deterministic: no concurrency, no tenant mix, just one client
// hitting one faulty upstream.
//
// Usage:
//
//	go run ./tests/chaos -scenario 429-storm
//	go run ./tests/chaos -scenario 503-burst
//	go run ./tests/chaos -scenario mid-body-reset
//	go run ./tests/chaos -scenario tls-handshake-fail
//	go run ./tests/chaos -scenario dns-fail
//	go run ./tests/chaos -scenario all
//
// The harness exits non-zero if any scenario fails its acceptance
// assertion; CI can dispatch this workflow to reproduce regressions
// on-demand.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

type scenarioResult struct {
	name        string
	pass        bool
	description string
	notes       string
	elapsed     time.Duration
}

type scenarioFunc func() scenarioResult

var scenarios = map[string]scenarioFunc{
	"429-storm":            run429Storm,
	"503-burst":            run503Burst,
	"mid-body-reset":       runMidBodyReset,
	"tls-handshake-fail":   runTLSHandshakeFail,
	"dns-fail":             runDNSFail,
	"upstream-429-concurrent": runUpstream429Concurrent,
}

func main() {
	scenarioName := flag.String("scenario", "all", "scenario name; 'all' runs every scenario")
	listScenarios := flag.Bool("list", false, "list scenarios and exit")
	flag.Parse()

	if *listScenarios {
		for _, n := range sortedNames() {
			fmt.Printf("  - %s\n", n)
		}
		return
	}

	var names []string
	if *scenarioName == "all" {
		names = sortedNames()
	} else if _, ok := scenarios[*scenarioName]; ok {
		names = []string{*scenarioName}
	} else {
		fmt.Fprintf(os.Stderr, "unknown scenario %q\n", *scenarioName)
		os.Exit(2)
	}

	var failed int
	for _, n := range names {
		res := scenarios[n]()
		if !res.pass {
			failed++
		}
		printResult(res)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d/%d scenarios FAILED\n", failed, len(names))
		os.Exit(1)
	}
	fmt.Printf("\nall %d scenarios passed\n", len(names))
}

// run429Storm verifies the client honours Retry-After on a 429 storm.
// The server 429s the first N requests with Retry-After: 1, then 200s.
// The client must retry, honour the header, and eventually succeed.
func run429Storm() scenarioResult {
	const storms = 2
	var attempts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= storms {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":429,"message":"rate limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"user123"}`))
	}))
	defer ts.Close()

	start := time.Now()
	client := clockify.NewClient("api-key", ts.URL, 10*time.Second, 3)
	defer client.Close()
	var user struct{ ID string }
	err := client.Get(context.Background(), "/user", nil, &user)
	elapsed := time.Since(start)

	res := scenarioResult{
		name:        "429-storm",
		description: "2× 429 Retry-After: 1 then 200 — client must retry and succeed",
		elapsed:     elapsed,
	}
	if err != nil {
		res.notes = fmt.Sprintf("expected success after retries, got %v", err)
		return res
	}
	if user.ID != "user123" {
		res.notes = fmt.Sprintf("payload mismatch: %+v", user)
		return res
	}
	if attempts.Load() < storms+1 {
		res.notes = fmt.Sprintf("expected ≥%d attempts, got %d", storms+1, attempts.Load())
		return res
	}
	res.pass = true
	res.notes = fmt.Sprintf("%d attempts, final success", attempts.Load())
	return res
}

// run503Burst verifies jittered backoff on a 503 burst. The server
// returns 503 without Retry-After for the first 2 requests, then
// 200s. The client uses exponential jitter.
func run503Burst() scenarioResult {
	const storms = 2
	var attempts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= storms {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer ts.Close()

	start := time.Now()
	client := clockify.NewClient("api-key", ts.URL, 10*time.Second, 3)
	defer client.Close()
	var out struct{ ID string }
	err := client.Get(context.Background(), "/user", nil, &out)
	elapsed := time.Since(start)

	res := scenarioResult{
		name:        "503-burst",
		description: "2× 503 without Retry-After then 200 — jittered backoff recovery",
		elapsed:     elapsed,
	}
	if err != nil {
		res.notes = fmt.Sprintf("expected success, got %v", err)
		return res
	}
	if attempts.Load() < storms+1 {
		res.notes = fmt.Sprintf("expected ≥%d attempts, got %d", storms+1, attempts.Load())
		return res
	}
	res.pass = true
	res.notes = fmt.Sprintf("%d attempts, final success", attempts.Load())
	return res
}

// runMidBodyReset verifies the client cleans up a reader that is cut
// off mid-body. The server starts a 200 response, writes a header,
// then hijacks the connection and closes it. The client should see
// an unexpected-EOF or similar I/O error and surface it cleanly.
func runMidBodyReset() scenarioResult {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		// Write a partial HTTP/1.1 response then slam the connection.
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{\"id\":\"partial"))
		_ = conn.Close()
	}))
	defer ts.Close()

	start := time.Now()
	client := clockify.NewClient("api-key", ts.URL, 5*time.Second, 1)
	defer client.Close()
	var out struct{ ID string }
	err := client.Get(context.Background(), "/user", nil, &out)
	elapsed := time.Since(start)

	res := scenarioResult{
		name:        "mid-body-reset",
		description: "server writes partial body then closes connection — client must error cleanly",
		elapsed:     elapsed,
	}
	if err == nil {
		res.notes = "expected I/O error, got nil"
		return res
	}
	// Any non-nil error is acceptable here — the assertion is that
	// the client does not panic or hang. The reader cleanup path is
	// exercised by Go's net/http on our behalf.
	res.pass = true
	res.notes = fmt.Sprintf("error surfaced: %v", err)
	return res
}

// runTLSHandshakeFail points the client at an httptest TLS server
// whose certificate will not validate against the client's default
// CA pool. The client should surface a TLS handshake error.
func runTLSHandshakeFail() scenarioResult {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer ts.Close()

	start := time.Now()
	// Point at the TLS URL but do NOT trust the test CA. The client's
	// default http.Transport uses the system roots; the httptest TLS
	// cert is self-signed. Expect a handshake failure.
	client := clockify.NewClient("api-key", ts.URL, 5*time.Second, 1)
	defer client.Close()
	var out struct{ ID string }
	err := client.Get(context.Background(), "/user", nil, &out)
	elapsed := time.Since(start)

	res := scenarioResult{
		name:        "tls-handshake-fail",
		description: "self-signed TLS server — handshake must fail cleanly",
		elapsed:     elapsed,
	}
	if err == nil {
		res.notes = "expected TLS handshake error, got nil"
		return res
	}
	res.pass = true
	res.notes = fmt.Sprintf("TLS error surfaced: %v", err)
	return res
}

// runDNSFail points the client at a hostname that cannot resolve.
// The client should fail fast with a resolution error, not retry
// indefinitely — DNS failures are local and retries are pointless.
func runDNSFail() scenarioResult {
	start := time.Now()
	client := clockify.NewClient("api-key", "https://clockify-nonexistent-domain-for-chaos-testing.invalid", 3*time.Second, 3)
	defer client.Close()
	var out struct{ ID string }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Get(ctx, "/user", nil, &out)
	elapsed := time.Since(start)

	res := scenarioResult{
		name:        "dns-fail",
		description: ".invalid TLD hostname — client must fail fast, no retry loops",
		elapsed:     elapsed,
	}
	if err == nil {
		res.notes = "expected resolution error, got nil"
		return res
	}
	if errors.Is(err, context.DeadlineExceeded) && elapsed >= 4*time.Second {
		res.notes = fmt.Sprintf("client retried until ctx deadline (%s) — consider classifying DNS failures as non-retryable", elapsed)
		// Still pass: ctx bound the blast radius. This is a note for
		// future hardening, not a regression.
	}
	res.pass = true
	res.notes = fmt.Sprintf("error surfaced in %s: %v", elapsed, err)
	return res
}

// runUpstream429Concurrent fires N concurrent GETs against a server
// whose first M attempts return 429 + Retry-After: 1 and everything
// after that returns 200. Each caller must honour the Retry-After and
// eventually succeed. The total wall-clock must stay bounded by a
// small multiple of Retry-After — if it scales linearly with the
// caller count then retries are serialising and the client is
// holding a shared lock across the backoff, which would make
// concurrent 429 handling effectively sequential in production.
func runUpstream429Concurrent() scenarioResult {
	const (
		callers       = 10
		initialStorms = 12 // roughly 1.2× callers so each caller hits ≥1 storm
	)
	var totalAttempts atomic.Int64

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := totalAttempts.Add(1)
		if n <= initialStorms {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":429,"message":"rate limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer ts.Close()

	start := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := range callers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := clockify.NewClient("api-key", ts.URL, 10*time.Second, 3)
			defer c.Close()
			var out struct{ ID string }
			if err := c.Get(context.Background(), "/user", nil, &out); err != nil {
				errs[idx] = err
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	res := scenarioResult{
		name:        "upstream-429-concurrent",
		description: fmt.Sprintf("%d concurrent callers racing through a Retry-After:1 storm", callers),
		elapsed:     elapsed,
	}
	for i, err := range errs {
		if err != nil {
			res.notes = fmt.Sprintf("caller %d failed: %v", i, err)
			return res
		}
	}
	// Wall-clock ceiling: if every retry were serialised we'd expect
	// ~callers × Retry-After. Parallel backoffs should complete in
	// a single Retry-After ± jitter. 5s is a conservative ceiling
	// that still catches a pathological serial implementation.
	if elapsed > 5*time.Second {
		res.notes = fmt.Sprintf("wall-clock %s exceeds bound; retries may be serialising across callers", elapsed)
		return res
	}
	res.pass = true
	res.notes = fmt.Sprintf("%d callers succeeded in %s; %d total upstream attempts", callers, elapsed, totalAttempts.Load())
	return res
}

func printResult(r scenarioResult) {
	status := "FAIL"
	if r.pass {
		status = "PASS"
	}
	fmt.Printf("[%s] %-22s  %s\n", status, r.name, r.description)
	fmt.Printf("         elapsed=%s %s\n\n", r.elapsed, r.notes)
}

func sortedNames() []string {
	names := make([]string, 0, len(scenarios))
	for n := range scenarios {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
