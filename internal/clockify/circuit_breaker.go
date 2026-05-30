package clockify

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrCircuitBreakerOpen is the sentinel that CircuitBreakerOpenError matches
// via errors.Is, letting callers detect a tripped breaker without depending on
// the concrete error type.
var ErrCircuitBreakerOpen = errors.New("clockify upstream circuit breaker open")

// CircuitBreakerConfig configures a CircuitBreaker. Enabled gates the breaker;
// FailureThreshold is the consecutive-failure count that opens it; OpenDuration
// is how long it stays open before probing; HalfOpenProbes is the number of
// concurrent trial requests allowed while half-open. Zero/negative numeric
// fields fall back to defaults in NewCircuitBreaker.
type CircuitBreakerConfig struct {
	Enabled          bool
	FailureThreshold int
	OpenDuration     time.Duration
	HalfOpenProbes   int
}

// CircuitBreakerOpenError is returned by CircuitBreaker.Before when a request
// is rejected because the per-endpoint breaker is open. RetryAfter is the
// remaining cool-down before the next probe is allowed.
type CircuitBreakerOpenError struct {
	Endpoint   string
	Method     string
	RetryAfter time.Duration
}

func (e *CircuitBreakerOpenError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("clockify upstream circuit breaker open: method=%s endpoint=%s retry_after=%s", e.Method, e.Endpoint, e.RetryAfter.Truncate(time.Millisecond))
}

// Is reports whether target is ErrCircuitBreakerOpen, enabling
// errors.Is(err, ErrCircuitBreakerOpen) on a *CircuitBreakerOpenError.
func (e *CircuitBreakerOpenError) Is(target error) bool {
	return target == ErrCircuitBreakerOpen
}

type breakerStateName string

const (
	breakerClosed   breakerStateName = "closed"
	breakerOpen     breakerStateName = "open"
	breakerHalfOpen breakerStateName = "half_open"
)

type breakerKey struct {
	endpoint string
	method   string
}

type breakerEndpointState struct {
	state            breakerStateName
	failures         int
	openedAt         time.Time
	halfOpenInFlight int
}

// CircuitBreaker is a per-endpoint (method+path) circuit breaker for the
// Clockify HTTP client. It transitions closed -> open after FailureThreshold
// consecutive upstream failures, half-open after OpenDuration, and back to
// closed on a successful probe. A nil or disabled breaker is a no-op.
type CircuitBreaker struct {
	enabled          bool
	failureThreshold int
	openDuration     time.Duration
	halfOpenProbes   int
	now              func() time.Time

	mu     sync.Mutex
	states map[breakerKey]*breakerEndpointState
}

// NewCircuitBreaker builds a CircuitBreaker from cfg, substituting defaults for
// any zero/negative numeric field (5 failures, 45s open, 1 half-open probe).
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	threshold := cfg.FailureThreshold
	if threshold <= 0 {
		threshold = 5
	}
	openDuration := cfg.OpenDuration
	if openDuration <= 0 {
		openDuration = 45 * time.Second
	}
	halfOpenProbes := cfg.HalfOpenProbes
	if halfOpenProbes <= 0 {
		halfOpenProbes = 1
	}
	return &CircuitBreaker{
		enabled:          cfg.Enabled,
		failureThreshold: threshold,
		openDuration:     openDuration,
		halfOpenProbes:   halfOpenProbes,
		now:              time.Now,
		states:           map[breakerKey]*breakerEndpointState{},
	}
}

// Before is called prior to an upstream request for (endpoint, method). It
// returns a *CircuitBreakerOpenError when the breaker is open (or half-open and
// already saturated with probes), and nil when the request may proceed.
func (b *CircuitBreaker) Before(endpoint, method string) error {
	if b == nil || !b.enabled {
		return nil
	}
	key := breakerKey{endpoint: endpoint, method: method}
	b.mu.Lock()
	defer b.mu.Unlock()

	st := b.ensureLocked(key)
	now := b.now()
	if st.state == breakerOpen {
		elapsed := now.Sub(st.openedAt)
		if elapsed < b.openDuration {
			retryAfter := b.openDuration - elapsed
			return &CircuitBreakerOpenError{Endpoint: endpoint, Method: method, RetryAfter: retryAfter}
		}
		b.transitionLocked(st, breakerHalfOpen, now)
	}
	if st.state == breakerHalfOpen {
		if st.halfOpenInFlight >= b.halfOpenProbes {
			return &CircuitBreakerOpenError{Endpoint: endpoint, Method: method, RetryAfter: b.openDuration}
		}
		st.halfOpenInFlight++
	}
	return nil
}

// After records the outcome of a request admitted by Before. upstreamFailure
// advances the failure count (and may open the breaker); a success resets it
// and closes the breaker. Endpoint and method identify the per-endpoint state.
func (b *CircuitBreaker) After(endpoint, method string, upstreamFailure bool) {
	if b == nil || !b.enabled {
		return
	}
	key := breakerKey{endpoint: endpoint, method: method}
	b.mu.Lock()
	defer b.mu.Unlock()

	st := b.ensureLocked(key)
	now := b.now()
	if st.state == breakerHalfOpen && st.halfOpenInFlight > 0 {
		st.halfOpenInFlight--
	}
	if !upstreamFailure {
		st.failures = 0
		if st.state != breakerClosed {
			b.transitionLocked(st, breakerClosed, now)
		}
		return
	}
	switch st.state {
	case breakerHalfOpen:
		b.transitionLocked(st, breakerOpen, now)
	case breakerOpen:
		st.openedAt = now
	default:
		st.failures++
		if st.failures >= b.failureThreshold {
			b.transitionLocked(st, breakerOpen, now)
		}
	}
}

func (b *CircuitBreaker) ensureLocked(key breakerKey) *breakerEndpointState {
	st, ok := b.states[key]
	if ok {
		return st
	}
	st = &breakerEndpointState{state: breakerClosed}
	b.states[key] = st
	return st
}

func (b *CircuitBreaker) transitionLocked(st *breakerEndpointState, next breakerStateName, now time.Time) {
	if st.state == next {
		return
	}
	st.state = next
	st.failures = 0
	if next == breakerOpen {
		st.openedAt = now
	}
	if next != breakerHalfOpen {
		st.halfOpenInFlight = 0
	}
}
