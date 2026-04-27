package clockify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/metrics"
	"github.com/apet97/go-clockify/internal/tracing"
)

// maxResponseBody is the maximum number of bytes read from API responses
// to prevent OOM on unexpectedly large or malicious responses.
const maxResponseBody = 10 * 1024 * 1024 // 10 MB

// bodyBufPoolMaxCap caps the capacity of a bytes.Buffer we're willing
// to return to the pool. A single oversized response would otherwise
// pin a multi-MB allocation in the pool forever, which hurts working
// set for services that handle one large report followed by many
// small calls.
const bodyBufPoolMaxCap = 64 * 1024

// bodyBufPool reuses bytes.Buffer instances across HTTP request-body
// marshalling and response-body reads. Each doJSON call acquires one
// buffer for the request payload (if any) and one for the response
// decode, returning both when it returns.
//
// Production effect: reduces per-call allocations on the hot tier-1
// write path by ~1 allocation + ~400 bytes for the request payload
// and another ~1 allocation for the response decoder state that used
// to come from json.NewDecoder. See the bench delta in Commit 3's
// message for measured numbers.
var bodyBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func getBodyBuf() *bytes.Buffer {
	// Use the comma-ok form to satisfy errcheck/forcetypeassert. The
	// pool's New returns *bytes.Buffer so the assertion is guaranteed
	// to succeed in practice; the ok=false branch exists for lint
	// compliance and defends against a future New function drift.
	b, ok := bodyBufPool.Get().(*bytes.Buffer)
	if !ok {
		b = new(bytes.Buffer)
	}
	b.Reset()
	return b
}

func putBodyBuf(b *bytes.Buffer) {
	if b.Cap() > bodyBufPoolMaxCap {
		return
	}
	bodyBufPool.Put(b)
}

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
	userAgent  string
}

func NewClient(apiKey, baseURL string, timeout time.Duration, maxRetries int) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     20,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		maxRetries: maxRetries,
		userAgent:  "clockify-mcp-go/dev",
	}
}

// Close releases idle connections held by the client.
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

// SetUserAgent sets the User-Agent string sent with every request.
func (c *Client) SetUserAgent(ua string) {
	c.userAgent = ua
}

func (c *Client) Get(ctx context.Context, path string, query map[string]string, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, out)
}

func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, http.MethodPost, path, nil, body, out)
}

func (c *Client) Put(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, http.MethodPut, path, nil, body, out)
}

func (c *Client) Patch(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, http.MethodPatch, path, nil, body, out)
}

func (c *Client) Delete(ctx context.Context, path string) error {
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func ListAll[T any](ctx context.Context, c *Client, path string, baseQuery map[string]string) ([]T, error) {
	page := 1
	all := make([]T, 0)
	for {
		query := cloneQuery(baseQuery)
		query["page"] = strconv.Itoa(page)
		if _, ok := query["page-size"]; !ok {
			query["page-size"] = "50"
		}

		var batch []T
		if err := c.Get(ctx, path, query, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || len(batch) < atoiDefault(query["page-size"], 50) {
			break
		}
		page++
		if page > 1000 {
			return nil, fmt.Errorf("pagination safety stop reached")
		}
	}

	return all, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, query map[string]string, body any, out any) error {
	var payload []byte
	if body != nil {
		buf := getBodyBuf()
		defer putBodyBuf(buf)
		// json.NewEncoder writes directly into the pooled buffer, avoiding
		// the fresh []byte that json.Marshal would allocate. The encoder
		// appends a trailing newline after the value; we slice it off so
		// the wire format matches the pre-pool json.Marshal output
		// byte-for-byte.
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return err
		}
		payload = buf.Bytes()
		if n := len(payload); n > 0 && payload[n-1] == '\n' {
			payload = payload[:n-1]
		}
	}

	endpoint := normalizeEndpoint(path)

	var lastErr error
	var explicitRetryAfter time.Duration
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Record the retry attempt with the reason derived from the last
			// error's status code. This lets operators see retry storms by
			// endpoint and reason before they escalate into tool errors.
			reason := "error"
			if apiErr, ok := lastErr.(*APIError); ok {
				reason = retryReason(apiErr.StatusCode)
			}
			metrics.UpstreamRetriesTotal.Inc(endpoint, reason)

			waitDur := explicitRetryAfter
			if waitDur <= 0 {
				waitDur = backoff(attempt)
			}
			// Before sleeping for retry, check if we have enough time left.
			if deadline, ok := ctx.Deadline(); ok {
				if time.Until(deadline) < waitDur {
					return lastErr
				}
			}
			if err := sleepWithContext(ctx, waitDur); err != nil {
				return err
			}
			// explicitRetryAfter is re-read below on retryable errors; no reset needed here.
		}

		err := c.doOnce(ctx, method, path, endpoint, query, payload, out)
		if err == nil {
			return nil
		}
		lastErr = err

		apiErr, ok := err.(*APIError)
		if !ok || !isRetryableStatus(apiErr.StatusCode) {
			return err
		}
		explicitRetryAfter = apiErr.RetryAfter
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("request failed without specific error")
}

func (c *Client) doOnce(ctx context.Context, method, path, endpoint string, query map[string]string, payload []byte, out any) error {
	ctx, span := tracing.Default.Start(ctx, "clockify.http")
	span.SetAttribute("upstream.endpoint", endpoint)
	span.SetAttribute("http.method", method)
	defer span.End()

	start := time.Now()
	statusCode := 0
	defer func() {
		span.SetAttribute("http.status_code", statusCode)
		metrics.UpstreamRequestsTotal.Inc(endpoint, method, statusBucket(statusCode))
		metrics.UpstreamRequestDuration.Observe(time.Since(start).Seconds(), endpoint, method)
	}()

	if path != "" && path[0] != '/' {
		path = "/" + path
	}
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	q := u.Query()
	for k, v := range query {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()

	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	tracing.Default.InjectHTTPHeaders(ctx, req.Header)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	statusCode = resp.StatusCode

	if resp.StatusCode >= 400 {
		// Read error body (limited to 64KB) before any other reads.
		errorReader := io.LimitReader(resp.Body, 64*1024)
		bodyBytes, _ := io.ReadAll(errorReader)
		// Drain remaining body to allow connection reuse.
		_, _ = io.Copy(io.Discard, resp.Body)

		var retryAfter time.Duration
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if s, err := strconv.Atoi(ra); err == nil {
				retryAfter = time.Duration(s) * time.Second
			} else if d, err := time.Parse(time.RFC1123, ra); err == nil {
				retryAfter = time.Until(d)
			}
		}

		return &APIError{
			Method:     method,
			Path:       path,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       trimBody(string(bodyBytes)),
			RetryAfter: retryAfter,
		}
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	// Read the body into a pooled buffer, then unmarshal. Using
	// json.NewDecoder here would allocate fresh internal scan state
	// on every call; io.Copy into a reused buffer is cheaper for the
	// small-response case that dominates tool traffic. The body size
	// is still bounded by io.LimitReader(maxResponseBody).
	respBuf := getBodyBuf()
	defer putBodyBuf(respBuf)
	if _, err := io.Copy(respBuf, io.LimitReader(resp.Body, maxResponseBody)); err != nil {
		return err
	}
	return json.Unmarshal(respBuf.Bytes(), out)
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

func backoff(attempt int) time.Duration {
	base := 250.0 * math.Pow(2, float64(attempt-1))
	jitter := rand.IntN(125)
	return time.Duration(base+float64(jitter)) * time.Millisecond
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func cloneQuery(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+2)
	maps.Copy(out, in)
	return out
}

func atoiDefault(s string, fallback int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
