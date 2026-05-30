package clockify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// netTimeoutErr is a minimal net.Error simulating a transport-level failure
// (dial/read timeout) that never produced an HTTP response.
type netTimeoutErr struct{}

func (netTimeoutErr) Error() string   { return "simulated transport timeout" }
func (netTimeoutErr) Timeout() bool   { return true }
func (netTimeoutErr) Temporary() bool { return true }

// failTransport is an http.RoundTripper that always fails with err and
// counts how many attempts reached it.
type failTransport struct {
	calls atomic.Int32
	err   error
}

func (t *failTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return nil, t.err
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name   string
		method string
		err    error
		want   bool
	}{
		// Idempotent/safe methods retry on retryable statuses and transport errors.
		{"GET nil", http.MethodGet, nil, false},
		{"GET 503", http.MethodGet, &APIError{StatusCode: 503}, true},
		{"GET 429", http.MethodGet, &APIError{StatusCode: 429}, true},
		{"GET 404", http.MethodGet, &APIError{StatusCode: 404}, false},
		{"GET 400", http.MethodGet, &APIError{StatusCode: 400}, false},
		{"GET transport", http.MethodGet, netTimeoutErr{}, true},
		{"GET context canceled", http.MethodGet, context.Canceled, false},
		{"GET context deadline", http.MethodGet, context.DeadlineExceeded, false},
		{"GET decode error", http.MethodGet, errors.New("decode failed"), false},
		{"HEAD 503", http.MethodHead, &APIError{StatusCode: 503}, true},
		{"OPTIONS 503", http.MethodOptions, &APIError{StatusCode: 503}, true},
		{"PUT 503", http.MethodPut, &APIError{StatusCode: 503}, true},
		{"PUT transport", http.MethodPut, netTimeoutErr{}, true},
		// Mutating methods are never auto-retried, even on retryable statuses
		// or transport errors, to avoid duplicate writes.
		{"POST 429", http.MethodPost, &APIError{StatusCode: 429}, false},
		{"POST 502", http.MethodPost, &APIError{StatusCode: 502}, false},
		{"POST 503", http.MethodPost, &APIError{StatusCode: 503}, false},
		{"POST 504", http.MethodPost, &APIError{StatusCode: 504}, false},
		{"POST transport", http.MethodPost, netTimeoutErr{}, false},
		{"PATCH 429", http.MethodPatch, &APIError{StatusCode: 429}, false},
		{"PATCH 502", http.MethodPatch, &APIError{StatusCode: 502}, false},
		{"PATCH 503", http.MethodPatch, &APIError{StatusCode: 503}, false},
		{"PATCH 504", http.MethodPatch, &APIError{StatusCode: 504}, false},
		{"PATCH transport", http.MethodPatch, netTimeoutErr{}, false},
		{"DELETE 429", http.MethodDelete, &APIError{StatusCode: 429}, false},
		{"DELETE 502", http.MethodDelete, &APIError{StatusCode: 502}, false},
		{"DELETE 503", http.MethodDelete, &APIError{StatusCode: 503}, false},
		{"DELETE 504", http.MethodDelete, &APIError{StatusCode: 504}, false},
		{"DELETE transport", http.MethodDelete, netTimeoutErr{}, false},
		// Unknown/empty method denies retry as a safety default.
		{"unknown method 503", "FROBNICATE", &APIError{StatusCode: 503}, false},
		{"empty method 503", "", &APIError{StatusCode: 503}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.method, tt.err); got != tt.want {
				t.Fatalf("isRetryableError(%q, %v) = %v, want %v", tt.method, tt.err, got, tt.want)
			}
		})
	}
}

func TestIsUpstreamFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil success", nil, false},
		{"5xx", &APIError{StatusCode: 502}, true},
		{"4xx", &APIError{StatusCode: 404}, false},
		{"transport error", netTimeoutErr{}, true},
		{"context canceled", context.Canceled, false},
		{"breaker open", ErrCircuitBreakerOpen, false},
		{"decode error", errors.New("bad json"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUpstreamFailure(tt.err); got != tt.want {
				t.Fatalf("isUpstreamFailure(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestClientRetriesTransportErrorsAndTripsBreaker proves transport-level
// failures are retried (previously they returned after a single attempt) and
// that a sustained run of them opens the circuit breaker (previously every
// transport error was recorded as a breaker success and reset the count).
func TestClientRetriesTransportErrorsAndTripsBreaker(t *testing.T) {
	ft := &failTransport{err: netTimeoutErr{}}
	c := NewClient("test-key", "https://api.clockify.test/api/v1", time.Second, 1)
	c.httpClient.Transport = ft
	c.breaker = NewCircuitBreaker(CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 2,
		OpenDuration:     time.Minute,
	})

	var out map[string]any
	// Call 1: 1 attempt + 1 retry = 2 transport hits, 1 breaker failure.
	if err := c.Get(context.Background(), "/user", nil, &out); err == nil {
		t.Fatal("call 1: expected transport error")
	}
	// Call 2: 2 more hits; the 2nd breaker failure reaches the threshold.
	if err := c.Get(context.Background(), "/user", nil, &out); err == nil {
		t.Fatal("call 2: expected transport error")
	}
	if got := ft.calls.Load(); got != 4 {
		t.Fatalf("transport attempts = %d, want 4 (2 calls x [1 try + 1 retry])", got)
	}
	// Call 3: the breaker is open, so it is rejected before any transport hit.
	err := c.Get(context.Background(), "/user", nil, &out)
	if !errors.Is(err, ErrCircuitBreakerOpen) {
		t.Fatalf("call 3 err = %v, want circuit breaker open", err)
	}
	if got := ft.calls.Load(); got != 4 {
		t.Fatalf("transport attempts after breaker open = %d, want 4 (no new hits)", got)
	}
}

// statusSeqTransport is an http.RoundTripper that returns each status in seq
// in order (repeating the last entry once exhausted), counting attempts. A
// status of 0 is treated as a transport-level failure (returns netTimeoutErr).
type statusSeqTransport struct {
	calls atomic.Int32
	seq   []int
}

func (t *statusSeqTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	idx := int(t.calls.Add(1)) - 1
	code := t.seq[len(t.seq)-1]
	if idx < len(t.seq) {
		code = t.seq[idx]
	}
	if code == 0 {
		return nil, netTimeoutErr{}
	}
	body := "{}"
	return &http.Response{
		StatusCode: code,
		Status:     http.StatusText(code),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func newSeqClient(t *testing.T, seq []int) (*Client, *statusSeqTransport) {
	t.Helper()
	st := &statusSeqTransport{seq: seq}
	c := NewClient("test-key", "https://api.clockify.test/api/v1", time.Second, 3)
	c.httpClient.Transport = st
	return c, st
}

// TestPostRequestsNeverRetried proves POST is not auto-retried even when the
// upstream returns a retryable 503: exactly one transport hit, error surfaced.
func TestPostRequestsNeverRetried(t *testing.T) {
	c, st := newSeqClient(t, []int{503, 503, 200})
	var out map[string]any
	if err := c.Post(context.Background(), "/projects", map[string]any{"name": "x"}, &out); err == nil {
		t.Fatal("expected error from POST 503, got nil")
	}
	if got := st.calls.Load(); got != 1 {
		t.Fatalf("POST transport attempts = %d, want 1 (no retry)", got)
	}
}

// TestPatchRequestsNeverRetried proves PATCH is not auto-retried on a 503.
func TestPatchRequestsNeverRetried(t *testing.T) {
	c, st := newSeqClient(t, []int{503, 503, 200})
	var out map[string]any
	if err := c.Patch(context.Background(), "/projects/1", map[string]any{"name": "x"}, &out); err == nil {
		t.Fatal("expected error from PATCH 503, got nil")
	}
	if got := st.calls.Load(); got != 1 {
		t.Fatalf("PATCH transport attempts = %d, want 1 (no retry)", got)
	}
}

// TestDeleteRequestsNeverRetried proves DELETE is not auto-retried on a 503.
func TestDeleteRequestsNeverRetried(t *testing.T) {
	c, st := newSeqClient(t, []int{503, 503, 200})
	if err := c.Delete(context.Background(), "/projects/1"); err == nil {
		t.Fatal("expected error from DELETE 503, got nil")
	}
	if got := st.calls.Load(); got != 1 {
		t.Fatalf("DELETE transport attempts = %d, want 1 (no retry)", got)
	}
}

// TestGetRequestsAlwaysRetried proves GET retries through transient 503s until
// it succeeds: three attempts (503 → 503 → 200) for maxRetries=3.
func TestGetRequestsAlwaysRetried(t *testing.T) {
	c, st := newSeqClient(t, []int{503, 503, 200})
	var out map[string]any
	if err := c.Get(context.Background(), "/user", nil, &out); err != nil {
		t.Fatalf("GET expected success after retries, got %v", err)
	}
	if got := st.calls.Load(); got != 3 {
		t.Fatalf("GET transport attempts = %d, want 3 (2 retries then success)", got)
	}
}

// TestClientDoesNotRetryPostTransportTimeout proves a POST that hits a
// transport-level net timeout is not auto-retried: with maxRetries=2 the
// transport is invoked exactly once and the error is surfaced. A retry here
// would risk a duplicate write.
func TestClientDoesNotRetryPostTransportTimeout(t *testing.T) {
	ft := &failTransport{err: netTimeoutErr{}}
	c := NewClient("test-key", "https://api.clockify.test/api/v1", time.Second, 2)
	c.httpClient.Transport = ft

	var out map[string]any
	if err := c.Post(context.Background(), "/projects", map[string]any{"name": "x"}, &out); err == nil {
		t.Fatal("expected transport error from POST, got nil")
	}
	if got := ft.calls.Load(); got != 1 {
		t.Fatalf("POST transport attempts on timeout = %d, want 1 (no retry)", got)
	}
}
