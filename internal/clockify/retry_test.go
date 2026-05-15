package clockify

import (
	"context"
	"errors"
	"net/http"
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
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"retryable 503", &APIError{StatusCode: 503}, true},
		{"retryable 429", &APIError{StatusCode: 429}, true},
		{"non-retryable 404", &APIError{StatusCode: 404}, false},
		{"non-retryable 400", &APIError{StatusCode: 400}, false},
		{"transport error", netTimeoutErr{}, true},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"plain (decode) error", errors.New("decode failed"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Fatalf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
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
