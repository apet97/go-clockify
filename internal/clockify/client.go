package clockify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxResponseBody is the maximum number of bytes read from API responses
// to prevent OOM on unexpectedly large or malicious responses.
const maxResponseBody = 10 * 1024 * 1024 // 10 MB

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
	return &Client{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
		userAgent:  "clockify-mcp-go/0.2.0",
	}
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
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	var lastErr error
	var explicitRetryAfter time.Duration
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			waitDur := explicitRetryAfter
			if waitDur <= 0 {
				waitDur = backoff(attempt)
			}
			if err := sleepWithContext(ctx, waitDur); err != nil {
				return err
			}
			explicitRetryAfter = 0
		}

		err = c.doOnce(ctx, method, path, query, payload, out)
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

func (c *Client) doOnce(ctx context.Context, method, path string, query map[string]string, payload []byte, out any) error {
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Limit body reads to prevent OOM on oversized responses.
	limitedBody := io.LimitReader(resp.Body, maxResponseBody)

	if resp.StatusCode >= 400 {
		// Limit error body read to 64KB for error messages
		errorReader := io.LimitReader(resp.Body, 64*1024)
		bodyBytes, _ := io.ReadAll(errorReader)

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
	return json.NewDecoder(limitedBody).Decode(out)
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout ||
		(code >= 500 && code <= 599)
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
	for k, v := range in {
		out[k] = v
	}
	return out
}

func atoiDefault(s string, fallback int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
