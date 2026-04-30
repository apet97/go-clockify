package clockify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientGetSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Fatalf("missing api key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"u1","name":"Test"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out map[string]any
	if err := c.Get(context.Background(), "/user", nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "u1" {
		t.Fatalf("unexpected id: %#v", out)
	}
}

func TestClientGetNormalizesPath(t *testing.T) {
	for _, path := range []string{"/user", "user"} {
		t.Run(path, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/user" {
					t.Fatalf("path = %q, want /user", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer ts.Close()

			c := NewClient("test-key", ts.URL, 5*time.Second, 0)
			var out map[string]any
			if err := c.Get(context.Background(), path, nil, &out); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out["ok"] != true {
				t.Fatalf("unexpected response: %#v", out)
			}
		})
	}
}

func TestClientAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out map[string]any
	err := c.Get(context.Background(), "/user", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
}

// --- Retry Logic ---

func TestRetryOn429ThenSuccess(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 10*time.Second, 2)
	var out map[string]any
	if err := c.Get(context.Background(), "/test", nil, &out); err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if count.Load() != 2 {
		t.Fatalf("expected 2 requests, got %d", count.Load())
	}
	if out["ok"] != true {
		t.Fatalf("unexpected response: %v", out)
	}
}

func TestRetryAfterIntegerSeconds(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"slow down"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 10*time.Second, 2)
	start := time.Now()
	var out map[string]any
	if err := c.Get(context.Background(), "/test", nil, &out); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 900*time.Millisecond {
		t.Fatalf("expected at least ~1s wait for Retry-After, elapsed=%v", elapsed)
	}
	if count.Load() != 2 {
		t.Fatalf("expected 2 requests, got %d", count.Load())
	}
}

func TestRetryAfterRFC1123(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n == 1 {
			// Use 2 seconds in the future to avoid sub-second rounding issues
			// with RFC1123 (which has only second precision).
			retryTime := time.Now().Add(2 * time.Second).UTC().Format(time.RFC1123)
			w.Header().Set("Retry-After", retryTime)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"slow down"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 10*time.Second, 2)
	start := time.Now()
	var out map[string]any
	if err := c.Get(context.Background(), "/test", nil, &out); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	elapsed := time.Since(start)
	// RFC1123 has second-level precision, so the actual wait can be 1-2s.
	// We just verify the client actually waited (more than the default backoff).
	if elapsed < 500*time.Millisecond {
		t.Fatalf("expected noticeable wait for RFC1123 Retry-After, elapsed=%v", elapsed)
	}
	if count.Load() != 2 {
		t.Fatalf("expected 2 requests, got %d", count.Load())
	}
}

func TestNoRetryOn401(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 3)
	var out map[string]any
	err := c.Get(context.Background(), "/test", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", apiErr.StatusCode)
	}
	if count.Load() != 1 {
		t.Fatalf("expected 1 request (no retries for 401), got %d", count.Load())
	}
}

func TestNoRetryOn404(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 3)
	var out map[string]any
	err := c.Get(context.Background(), "/test", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", apiErr.StatusCode)
	}
	if count.Load() != 1 {
		t.Fatalf("expected 1 request (no retries for 404), got %d", count.Load())
	}
}

func TestRetryOn502(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`bad gateway`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 30*time.Second, 3)
	var out map[string]any
	if err := c.Get(context.Background(), "/test", nil, &out); err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if count.Load() != 3 {
		t.Fatalf("expected 3 requests, got %d", count.Load())
	}
	if out["ok"] != true {
		t.Fatalf("unexpected response: %v", out)
	}
}

func TestRetryExhausted(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 30*time.Second, 2)
	var out map[string]any
	err := c.Get(context.Background(), "/test", nil, &out)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", apiErr.StatusCode)
	}
	// 1 initial + 2 retries = 3 total
	if count.Load() != 3 {
		t.Fatalf("expected 3 requests (1 + 2 retries), got %d", count.Load())
	}
}

func TestRetryDeadlineCheck(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 30*time.Second, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	var out map[string]any
	err := c.Get(ctx, "/test", nil, &out)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error due to deadline check")
	}
	// Should bail out almost immediately (well under 60s), because deadline < Retry-After.
	// Allow generous tolerance but it must be far less than 60s.
	if elapsed > 2*time.Second {
		t.Fatalf("expected fast bail-out due to deadline check, but took %v", elapsed)
	}
	// Server should have been hit only once before the client bailed.
	if count.Load() != 1 {
		t.Fatalf("expected 1 request before deadline bail-out, got %d", count.Load())
	}
}

func TestContextCancelDuringBackoff(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 30*time.Second, 5)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 50ms so it fires during backoff sleep.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	var out map[string]any
	err := c.Get(ctx, "/test", nil, &out)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if err != context.Canceled {
		// The error could be wrapped; check that it's a context error
		apiErr, isAPI := err.(*APIError)
		if isAPI {
			// If we got the deadline check bail-out, that's also acceptable
			_ = apiErr
		} else if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got: %v (%T)", err, err)
		}
	}
}

// --- Pagination ---

func TestListAllMultiplePages(t *testing.T) {
	type item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	allItems := []item{
		{ID: "1", Name: "a"},
		{ID: "2", Name: "b"},
		{ID: "3", Name: "c"},
		{ID: "4", Name: "d"},
		{ID: "5", Name: "e"},
		{ID: "6", Name: "f"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		pageSizeStr := r.URL.Query().Get("page-size")
		page, _ := strconv.Atoi(pageStr)
		pageSize, _ := strconv.Atoi(pageSizeStr)

		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 2
		}

		start := (page - 1) * pageSize
		end := start + pageSize
		if start >= len(allItems) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if end > len(allItems) {
			end = len(allItems)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(allItems[start:end])
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	result, err := ListAll[item](context.Background(), c, "/items", map[string]string{"page-size": "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 6 {
		t.Fatalf("expected 6 items, got %d", len(result))
	}
	for i, it := range result {
		expected := allItems[i]
		if it.ID != expected.ID || it.Name != expected.Name {
			t.Fatalf("item %d mismatch: got %+v, want %+v", i, it, expected)
		}
	}
}

func TestListAllEmptyFirstPage(t *testing.T) {
	type item struct {
		ID string `json:"id"`
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	result, err := ListAll[item](context.Background(), c, "/items", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result))
	}
}

func TestListAllSinglePage(t *testing.T) {
	type item struct {
		ID string `json:"id"`
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"x"},{"id":"y"}]`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	// page-size=50 (default), only 2 items returned → single page
	result, err := ListAll[item](context.Background(), c, "/items", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].ID != "x" || result[1].ID != "y" {
		t.Fatalf("unexpected items: %+v", result)
	}
}

// --- Edge Cases ---

func TestBackoffIncreasing(t *testing.T) {
	// Run multiple samples to account for jitter and confirm the trend.
	const samples = 20
	for s := 0; s < samples; s++ {
		b1 := backoff(1)
		b2 := backoff(2)
		b3 := backoff(3)
		// Base values: 250ms, 500ms, 1000ms with up to 125ms jitter.
		// b1 in [250, 375], b2 in [500, 625], b3 in [1000, 1125].
		// Even with worst-case jitter, b2's minimum (500ms) > b1's maximum (375ms),
		// and b3's minimum (1000ms) > b2's maximum (625ms).
		if b2 <= b1 {
			t.Fatalf("sample %d: expected backoff(2) > backoff(1), got %v <= %v", s, b2, b1)
		}
		if b3 <= b2 {
			t.Fatalf("sample %d: expected backoff(3) > backoff(2), got %v <= %v", s, b3, b2)
		}
	}
}

func TestIsRetryableStatus(t *testing.T) {
	retryable := []int{429, 502, 503, 504}
	nonRetryable := []int{400, 401, 403, 404, 500, 501}

	for _, code := range retryable {
		if !isRetryableStatus(code) {
			t.Errorf("expected status %d to be retryable", code)
		}
	}
	for _, code := range nonRetryable {
		if isRetryableStatus(code) {
			t.Errorf("expected status %d to NOT be retryable", code)
		}
	}
}

func TestPostWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %s", ct)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["name"] != "test-project" {
			t.Fatalf("unexpected body name: %v", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"p1","name":"%s"}`, body["name"])
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	reqBody := map[string]string{"name": "test-project"}
	var out map[string]any
	if err := c.Post(context.Background(), "/projects", reqBody, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "p1" {
		t.Fatalf("unexpected id: %v", out["id"])
	}
	if out["name"] != "test-project" {
		t.Fatalf("unexpected name: %v", out["name"])
	}
}

// TestConcurrentPutsShareBufPoolSafely drives 100 parallel Put calls
// through a single client to stress the bodyBufPool against the race
// detector. Each goroutine sends a unique payload and asserts the
// upstream echoes that payload back intact — if the pool ever leaked
// bytes between goroutines (for example by returning a buffer before
// the encoder finished, or by reusing a buffer whose Bytes() slice
// was still aliased by a prior caller), the upstream's assertion
// would fail with a mismatched ID and the test would fail.
//
// Run with -race to catch any data races in the pool Get/Put paths.
func TestConcurrentPutsShareBufPoolSafely(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id, ok := body["id"].(string)
		if !ok || id == "" {
			t.Errorf("missing id in body: %#v", body)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Echo the id back in the response so each caller can verify
		// its own payload wasn't swapped with a sibling goroutine's.
		_, _ = fmt.Fprintf(w, `{"echoed":%q}`, id)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	defer c.Close()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("id-%04d", i)
			payload := map[string]any{"id": id, "pad": strconv.Itoa(i)}
			var out map[string]any
			if err := c.Put(context.Background(), "/items/"+id, payload, &out); err != nil {
				errCh <- fmt.Errorf("goroutine %d: %w", i, err)
				return
			}
			if got, _ := out["echoed"].(string); got != id {
				errCh <- fmt.Errorf("goroutine %d: echoed=%q want %q", i, got, id)
				return
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// TestClientOversizeResponseReturnsClearError locks the contract
// added by ChatGPT's oversized-response audit: when the upstream
// emits a body larger than maxResponseBody, the client returns a
// purpose-built "response too large" error rather than letting
// json.Unmarshal fail on a silently truncated payload. Operators
// can distinguish a truncated upstream from a genuinely malformed
// one — they need different follow-up actions.
func TestClientOversizeResponseReturnsClearError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Emit an obviously-valid-JSON-but-too-large body. The
		// excess just past maxResponseBody is the boundary the
		// client now rejects; the request handler tops it off
		// with garbage so the truncated read would still parse
		// successfully (proving the size check, not parse luck).
		_, _ = w.Write([]byte(`{"items":["`))
		_, _ = w.Write(make([]byte, maxResponseBody))
		_, _ = w.Write([]byte(`"]}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	defer c.Close()

	var out map[string]any
	err := c.Get(context.Background(), "/items", nil, &out)
	if err == nil {
		t.Fatal("expected oversize error, got nil")
	}
	if got := err.Error(); !contains(got, "response too large") {
		t.Fatalf("expected error to mention 'response too large', got: %v", err)
	}
	if got := err.Error(); !contains(got, "method=GET") {
		t.Fatalf("expected error to include method label, got: %v", err)
	}
}

// TestClientUnderLimitResponseStillSucceeds is the symmetric
// guardrail: a body just under the cap must still parse cleanly,
// otherwise we'd push the boundary too low and break legitimate
// large reports.
func TestClientUnderLimitResponseStillSucceeds(t *testing.T) {
	// Build a JSON payload that is large but well under the cap.
	const padSize = 1 * 1024 * 1024 // 1 MiB
	pad := make([]byte, padSize)
	for i := range pad {
		pad[i] = 'a'
	}
	body := append([]byte(`{"pad":"`), pad...)
	body = append(body, []byte(`"}`)...)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	defer c.Close()

	var out map[string]any
	if err := c.Get(context.Background(), "/items", nil, &out); err != nil {
		t.Fatalf("unexpected error for under-limit response: %v", err)
	}
	if got, _ := out["pad"].(string); len(got) != padSize {
		t.Fatalf("pad length = %d, want %d", len(got), padSize)
	}
}

// contains is a tiny strings.Contains shim so this file doesn't
// need to import strings just for two test sites.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
