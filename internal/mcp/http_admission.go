package mcp

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/metrics"
)

const httpAdmissionWindow = time.Minute

// HTTPAdmissionLimits configures the app-layer HTTP admission guard used by
// legacy HTTP and streamable HTTP before JSON-RPC dispatch. These limits are
// intentionally process-local; hosted deployments that need cross-replica
// quotas must still enforce them at the gateway/load-balancer layer.
type HTTPAdmissionLimits struct {
	PerIPPerMinute        int
	PerPrincipalPerMinute int
	SSEPerSession         int
}

type httpAdmissionLimiter struct {
	limits HTTPAdmissionLimits
	now    func() time.Time

	mu          sync.Mutex
	ips         map[string]*admissionBucket
	principals  map[string]*admissionBucket
	sseSessions map[string]int
	lastCleanup time.Time
}

type admissionBucket struct {
	windowStart time.Time
	count       int
}

func newHTTPAdmissionLimiter(limits HTTPAdmissionLimits) *httpAdmissionLimiter {
	if limits.PerIPPerMinute <= 0 && limits.PerPrincipalPerMinute <= 0 && limits.SSEPerSession <= 0 {
		return nil
	}
	return &httpAdmissionLimiter{
		limits:      limits,
		now:         time.Now,
		ips:         map[string]*admissionBucket{},
		principals:  map[string]*admissionBucket{},
		sseSessions: map[string]int{},
	}
}

func optionalHTTPAdmission(args []*httpAdmissionLimiter) *httpAdmissionLimiter {
	if len(args) == 0 {
		return nil
	}
	return args[0]
}

func (l *httpAdmissionLimiter) allowIP(r *http.Request, behindHTTPSProxy bool) (int, bool) {
	if l == nil || l.limits.PerIPPerMinute <= 0 {
		return 0, true
	}
	return l.allow(l.ips, httpAdmissionSourceKey(r, behindHTTPSProxy), l.limits.PerIPPerMinute)
}

func (l *httpAdmissionLimiter) allowPrincipal(p authn.Principal) (int, bool) {
	if l == nil || l.limits.PerPrincipalPerMinute <= 0 {
		return 0, true
	}
	key := p.Subject + "\x00" + p.TenantID
	return l.allow(l.principals, key, l.limits.PerPrincipalPerMinute)
}

func (l *httpAdmissionLimiter) allow(buckets map[string]*admissionBucket, key string, max int) (int, bool) {
	if key == "" {
		key = "unknown"
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.cleanupExpiredLocked(now)
	b := buckets[key]
	if b == nil || now.Sub(b.windowStart) >= httpAdmissionWindow {
		b = &admissionBucket{windowStart: now}
		buckets[key] = b
	}
	if b.count >= max {
		return retryAfterSeconds(now, b.windowStart.Add(httpAdmissionWindow)), false
	}
	b.count++
	return 0, true
}

func (l *httpAdmissionLimiter) acquireSSE(sessionID string) (func(), bool) {
	if l == nil || l.limits.SSEPerSession <= 0 {
		return func() {}, true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.sseSessions[sessionID] >= l.limits.SSEPerSession {
		return nil, false
	}
	l.sseSessions[sessionID]++
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.sseSessions[sessionID] <= 1 {
			delete(l.sseSessions, sessionID)
			return
		}
		l.sseSessions[sessionID]--
	}, true
}

func (l *httpAdmissionLimiter) cleanupExpiredLocked(now time.Time) {
	if now.Sub(l.lastCleanup) < httpAdmissionWindow {
		return
	}
	l.lastCleanup = now
	for key, b := range l.ips {
		if now.Sub(b.windowStart) >= httpAdmissionWindow {
			delete(l.ips, key)
		}
	}
	for key, b := range l.principals {
		if now.Sub(b.windowStart) >= httpAdmissionWindow {
			delete(l.principals, key)
		}
	}
}

func retryAfterSeconds(now, until time.Time) int {
	if !until.After(now) {
		return 1
	}
	secs := int(until.Sub(now).Round(time.Second) / time.Second)
	if secs < 1 {
		return 1
	}
	return secs
}

func applyHTTPAdmissionIP(w http.ResponseWriter, r *http.Request, limiter *httpAdmissionLimiter, behindHTTPSProxy bool) bool {
	retryAfter, ok := limiter.allowIP(r, behindHTTPSProxy)
	if !ok {
		writeHTTPAdmissionRejected(w, r, "ip", retryAfter)
		return false
	}
	return true
}

func applyHTTPAdmissionPrincipal(w http.ResponseWriter, r *http.Request, limiter *httpAdmissionLimiter, principal authn.Principal) bool {
	retryAfter, ok := limiter.allowPrincipal(principal)
	if !ok {
		writeHTTPAdmissionRejected(w, r, "principal", retryAfter)
		return false
	}
	return true
}

func applyHTTPAdmissionSSE(w http.ResponseWriter, r *http.Request, limiter *httpAdmissionLimiter, sessionID string) (func(), bool) {
	release, ok := limiter.acquireSSE(sessionID)
	if !ok {
		writeHTTPAdmissionRejected(w, r, "sse_session", 1)
		return nil, false
	}
	return release, true
}

func writeHTTPAdmissionRejected(w http.ResponseWriter, r *http.Request, reason string, retryAfter int) {
	if retryAfter < 1 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	metrics.HTTPAdmissionRejectionsTotal.Inc(normalizeHTTPAdmissionPath(r.URL.Path), reason)
	writeJSONRPCError(w, http.StatusTooManyRequests, RPCCodeRateLimited, "rate limited")
}

func normalizeHTTPAdmissionPath(path string) string {
	switch path {
	case "/mcp", "/mcp/events":
		return path
	default:
		return "/other"
	}
}

func httpAdmissionSourceKey(r *http.Request, behindHTTPSProxy bool) string {
	if behindHTTPSProxy {
		forwardedFor := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
		if ip := net.ParseIP(forwardedFor); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
