package clockify

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/metrics"
)

var ErrCircuitBreakerOpen = errors.New("clockify upstream circuit breaker open")

type CircuitBreakerConfig struct {
	Enabled          bool
	FailureThreshold int
	OpenDuration     time.Duration
	HalfOpenProbes   int
}

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

type CircuitBreaker struct {
	enabled          bool
	failureThreshold int
	openDuration     time.Duration
	halfOpenProbes   int
	now              func() time.Time

	mu     sync.Mutex
	states map[breakerKey]*breakerEndpointState
	gauges sync.Map
}

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

func (b *CircuitBreaker) Before(endpoint, method string) error {
	if b == nil || !b.enabled {
		return nil
	}
	key := breakerKey{endpoint: endpoint, method: method}
	b.registerGauge(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	st := b.ensureLocked(key)
	now := b.now()
	if st.state == breakerOpen {
		elapsed := now.Sub(st.openedAt)
		if elapsed < b.openDuration {
			retryAfter := b.openDuration - elapsed
			metrics.UpstreamCircuitBreakerRejectionsTotal.Inc(endpoint, method)
			return &CircuitBreakerOpenError{Endpoint: endpoint, Method: method, RetryAfter: retryAfter}
		}
		b.transitionLocked(key, st, breakerHalfOpen, now)
	}
	if st.state == breakerHalfOpen {
		if st.halfOpenInFlight >= b.halfOpenProbes {
			metrics.UpstreamCircuitBreakerRejectionsTotal.Inc(endpoint, method)
			return &CircuitBreakerOpenError{Endpoint: endpoint, Method: method, RetryAfter: b.openDuration}
		}
		st.halfOpenInFlight++
	}
	return nil
}

func (b *CircuitBreaker) After(endpoint, method string, upstream5xx bool) {
	if b == nil || !b.enabled {
		return
	}
	key := breakerKey{endpoint: endpoint, method: method}
	b.registerGauge(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	st := b.ensureLocked(key)
	now := b.now()
	if st.state == breakerHalfOpen && st.halfOpenInFlight > 0 {
		st.halfOpenInFlight--
	}
	if !upstream5xx {
		st.failures = 0
		if st.state != breakerClosed {
			b.transitionLocked(key, st, breakerClosed, now)
		}
		return
	}
	switch st.state {
	case breakerHalfOpen:
		b.transitionLocked(key, st, breakerOpen, now)
	case breakerOpen:
		st.openedAt = now
	default:
		st.failures++
		if st.failures >= b.failureThreshold {
			b.transitionLocked(key, st, breakerOpen, now)
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

func (b *CircuitBreaker) registerGauge(key breakerKey) {
	if _, loaded := b.gauges.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	metrics.UpstreamCircuitBreakerState.SetFunc(func() float64 {
		return b.StateValue(key.endpoint, key.method)
	}, key.endpoint, key.method)
}

func (b *CircuitBreaker) transitionLocked(key breakerKey, st *breakerEndpointState, next breakerStateName, now time.Time) {
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
	metrics.UpstreamCircuitBreakerTransitionsTotal.Inc(key.endpoint, key.method, string(next))
}

func (b *CircuitBreaker) StateValue(endpoint, method string) float64 {
	if b == nil || !b.enabled {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.states[breakerKey{endpoint: endpoint, method: method}]
	if !ok {
		return 0
	}
	switch st.state {
	case breakerOpen:
		return 1
	case breakerHalfOpen:
		return 0.5
	default:
		return 0
	}
}
