package clockify

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"math/big"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
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

// responseDrainLimit caps best-effort connection-reuse drains once the caller
// no longer needs the response body. Past this, close the connection instead
// of pulling unbounded bytes from a bad upstream.
const responseDrainLimit = 1 << 20 // 1 MiB

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
// Production effect: reduces per-call allocations on the hot
// time-entry write path by ~1 allocation + ~400 bytes for the request
// payload and another ~1 allocation for the response decoder state that
// used to come from json.NewDecoder.
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

func valuesFromQueryMap(query map[string]string) url.Values {
	values := url.Values{}
	for k, v := range query {
		if v != "" {
			values.Add(k, v)
		}
	}
	return values
}

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
	userAgent  string
	breaker    *CircuitBreaker
}

const (
	DocumentedHostMain     = "https://api.clockify.me/api/v1"
	DocumentedHostReports  = "https://reports.api.clockify.me/v1"
	DocumentedHostAuditLog = "https://auditlog-api.api.clockify.me/v1"
)

func NewClient(apiKey, baseURL string, timeout time.Duration, maxRetries int) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
					return fmt.Errorf("refusing cross-host redirect from %s to %s", via[0].URL.Host, req.URL.Host)
				}
				if len(via) > 0 && strings.EqualFold(via[0].URL.Scheme, "https") && !strings.EqualFold(req.URL.Scheme, "https") {
					return fmt.Errorf("refusing scheme downgrade from %s to %s", via[0].URL.Scheme, req.URL.Scheme)
				}
				return nil
			},
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

func (c *Client) SetCircuitBreaker(cfg CircuitBreakerConfig) {
	if !cfg.Enabled {
		c.breaker = nil
		return
	}
	c.breaker = NewCircuitBreaker(cfg)
}

func (c *Client) Get(ctx context.Context, path string, query map[string]string, out any) error {
	return c.doJSON(ctx, c.baseURL, http.MethodGet, path, query, nil, out)
}

func (c *Client) GetValues(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSONValues(ctx, c.baseURL, http.MethodGet, path, query, nil, out)
}

func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, c.baseURL, http.MethodPost, path, nil, body, out)
}

func (c *Client) PostWithQuery(ctx context.Context, path string, query map[string]string, body any, out any) error {
	return c.doJSON(ctx, c.baseURL, http.MethodPost, path, query, body, out)
}

func (c *Client) Put(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, c.baseURL, http.MethodPut, path, nil, body, out)
}

func (c *Client) PutWithQuery(ctx context.Context, path string, query map[string]string, body any, out any) error {
	return c.doJSON(ctx, c.baseURL, http.MethodPut, path, query, body, out)
}

func (c *Client) Patch(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, c.baseURL, http.MethodPatch, path, nil, body, out)
}

func (c *Client) Delete(ctx context.Context, path string) error {
	return c.doJSON(ctx, c.baseURL, http.MethodDelete, path, nil, nil, nil)
}

func (c *Client) DeleteWithQuery(ctx context.Context, path string, query map[string]string) error {
	return c.doJSON(ctx, c.baseURL, http.MethodDelete, path, query, nil, nil)
}

// RequestRawValues runs a request against a documented host and returns the
// raw response bytes and headers instead of unmarshalling JSON.
func (c *Client) RequestRawValues(ctx context.Context, reportsHost bool, method, path string, query url.Values, body any) (*RawResponse, error) {
	host := DocumentedHostMain
	if reportsHost {
		host = DocumentedHostReports
	}
	return c.RequestRawValuesForHost(ctx, host, method, path, query, body)
}

// RequestRawValuesForHost is the host-aware binary-aware request transport.
// Non-canonical test/custom base URLs stay pinned to the configured base so
// fake servers and proxies do not need per-host wiring.
func (c *Client) RequestRawValuesForHost(ctx context.Context, host, method, path string, query url.Values, body any) (*RawResponse, error) {
	raw := &RawResponse{}
	if err := c.doJSONValues(ctx, c.DocumentedBaseURL(host), method, path, query, body, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ReportsBaseURL returns the base URL for endpoints that live on
// Clockify's reports host (reports.api.clockify.me/v1). The reports
// API is a separate host from api.clockify.me — hitting the reports
// paths on the primary host returns 404 ("No static resource"), and
// hitting them on the reports host with the /api/v1 prefix is also
// 404. See findings/shared-reports.md.
//
// Derivation: when the primary base URL matches the canonical
// production host, swap to reports.api.clockify.me and drop the /api
// segment. Otherwise (test stubs, custom proxies, on-prem mirrors),
// return the primary base URL unchanged so existing fixtures continue
// to work without per-test wiring.
func (c *Client) ReportsBaseURL() string {
	if c.baseURL == DocumentedHostMain {
		return DocumentedHostReports
	}
	return c.baseURL
}

// AuditLogBaseURL returns the dedicated Clockify audit-log API base URL.
// Like ReportsBaseURL, custom/test base URLs remain unchanged.
func (c *Client) AuditLogBaseURL() string {
	if c.baseURL == DocumentedHostMain {
		return DocumentedHostAuditLog
	}
	return c.baseURL
}

// DocumentedBaseURL resolves a generated OpenAPI host to the concrete base URL
// this client should use.
func (c *Client) DocumentedBaseURL(host string) string {
	switch strings.TrimRight(host, "/") {
	case DocumentedHostReports:
		return c.ReportsBaseURL()
	case DocumentedHostAuditLog:
		return c.AuditLogBaseURL()
	default:
		return c.baseURL
	}
}

// GetReports performs a GET against the reports host. Use this for
// shared-report endpoints; everything else should stay on Get.
func (c *Client) GetReports(ctx context.Context, path string, query map[string]string, out any) error {
	return c.doJSON(ctx, c.ReportsBaseURL(), http.MethodGet, path, query, nil, out)
}

// PostReports performs a POST against the reports host.
func (c *Client) PostReports(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, c.ReportsBaseURL(), http.MethodPost, path, nil, body, out)
}

// PostReportsRaw is the binary-aware sibling of PostReports. It is used for
// report exportType values such as PDF/CSV/XLSX/ZIP where the reports API may
// return a file body rather than JSON.
func (c *Client) PostReportsRaw(ctx context.Context, path string, body any) (*RawResponse, error) {
	raw := &RawResponse{}
	if err := c.doJSON(ctx, c.ReportsBaseURL(), http.MethodPost, path, nil, body, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// PutReports performs a PUT against the reports host.
func (c *Client) PutReports(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, c.ReportsBaseURL(), http.MethodPut, path, nil, body, out)
}

// DeleteReports performs a DELETE against the reports host.
func (c *Client) DeleteReports(ctx context.Context, path string) error {
	return c.doJSON(ctx, c.ReportsBaseURL(), http.MethodDelete, path, nil, nil, nil)
}

// RawResponse is the binary-aware envelope returned by RequestRawValues and
// PostReportsRaw — the raw response bytes plus enough header context for
// callers to recover Content-Type and Content-Disposition (filename). Used by
// export endpoints where the upstream returns binary (PDF/XLSX) or non-JSON
// text (CSV).
type RawResponse struct {
	Header http.Header
	Body   []byte
}

// MultipartFile is a single file part for multipart/form-data requests.
// Data is already decoded by the caller; callers must not pass filesystem
// paths because MCP clients should not grant arbitrary server file access.
type MultipartFile struct {
	FieldName   string
	Filename    string
	ContentType string
	Data        []byte
}

const maxMultipartFileBytes = 10 << 20 // 10 MiB

// PostMultipart performs a POST with a multipart/form-data body. Each
// form key maps to one or more values; multi-value keys are written as
// repeated form fields under the same name.
func (c *Client) PostMultipart(ctx context.Context, path string, form url.Values, out any) error {
	payload, contentType, err := encodeMultipartWithFiles(form, nil)
	if err != nil {
		return err
	}
	return c.doRequest(ctx, c.baseURL, http.MethodPost, path, nil, contentType, payload, out)
}

// PostMultipartWithFiles performs a POST with form fields plus binary file
// parts. File payloads are bounded to keep MCP requests from becoming an
// unbounded memory sink.
func (c *Client) PostMultipartWithFiles(ctx context.Context, path string, form url.Values, files []MultipartFile, out any) error {
	payload, contentType, err := encodeMultipartWithFiles(form, files)
	if err != nil {
		return err
	}
	return c.doRequest(ctx, c.baseURL, http.MethodPost, path, nil, contentType, payload, out)
}

// PutMultipart performs a PUT with a multipart/form-data body. Same
// encoding rules as PostMultipart.
func (c *Client) PutMultipart(ctx context.Context, path string, form url.Values, out any) error {
	payload, contentType, err := encodeMultipartWithFiles(form, nil)
	if err != nil {
		return err
	}
	return c.doRequest(ctx, c.baseURL, http.MethodPut, path, nil, contentType, payload, out)
}

func encodeMultipartWithFiles(form url.Values, files []MultipartFile) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, vs := range form {
		for _, v := range vs {
			if err := w.WriteField(k, v); err != nil {
				return nil, "", err
			}
		}
	}
	for _, file := range files {
		field := strings.TrimSpace(file.FieldName)
		if field == "" {
			return nil, "", fmt.Errorf("multipart file field name is required")
		}
		filename := strings.TrimSpace(file.Filename)
		if filename == "" {
			return nil, "", fmt.Errorf("multipart file filename is required")
		}
		if len(file.Data) == 0 {
			return nil, "", fmt.Errorf("multipart file %q is empty", filename)
		}
		if len(file.Data) > maxMultipartFileBytes {
			return nil, "", fmt.Errorf("multipart file %q exceeds %d bytes", filename, maxMultipartFileBytes)
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
		if ct := strings.TrimSpace(file.ContentType); ct != "" {
			header.Set("Content-Type", ct)
		} else {
			header.Set("Content-Type", "application/octet-stream")
		}
		part, err := w.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(file.Data); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

const (
	defaultListAllPageSize = 200
	defaultListAllMaxRows  = 5000
	listAllSafetyStopPages = 1000
)

type ListAllOptions struct {
	// MaxRows bounds total decoded rows before ListAll/ListAllFunc fail
	// closed. 0 uses the default production cap.
	MaxRows int
}

// PaginationCapError reports that a paginated scan exceeded its row cap.
type PaginationCapError struct {
	Path        string
	RowsScanned int
	MaxRows     int
	Page        int
	PageSize    int
}

func (e *PaginationCapError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf(
		"pagination row cap exceeded: path=%s rows_scanned=%d max_rows=%d page=%d page-size=%d",
		e.Path,
		e.RowsScanned,
		e.MaxRows,
		e.Page,
		e.PageSize,
	)
}

// ListAll fetches every page into a single slice, bounded by the default
// pagination row cap.
func ListAll[T any](ctx context.Context, c *Client, path string, baseQuery map[string]string) ([]T, error) {
	return ListAllWithOptions[T](ctx, c, path, baseQuery, ListAllOptions{})
}

// ListAllWithOptions fetches every page into a single slice using opts.
func ListAllWithOptions[T any](ctx context.Context, c *Client, path string, baseQuery map[string]string, opts ListAllOptions) ([]T, error) {
	all := make([]T, 0)
	if err := ListAllFuncWithOptions(ctx, c, path, baseQuery, opts, func(batch []T) error {
		all = append(all, batch...)
		return nil
	}); err != nil {
		return nil, err
	}
	return all, nil
}

// ListAllFuncWithOptions scans pages using opts and invokes onPage for each
// non-empty page without retaining earlier pages.
func ListAllFuncWithOptions[T any](ctx context.Context, c *Client, path string, baseQuery map[string]string, opts ListAllOptions, onPage func([]T) error) error {
	if onPage == nil {
		return fmt.Errorf("pagination page callback is nil: path=%s", path)
	}
	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = defaultListAllMaxRows
	}
	page := 1
	rowsScanned := 0
	for {
		query := cloneQuery(baseQuery)
		query["page"] = strconv.Itoa(page)
		if _, ok := query["page-size"]; !ok {
			query["page-size"] = strconv.Itoa(defaultListAllPageSize)
		}
		pageSize := atoiDefault(query["page-size"], defaultListAllPageSize)

		var batch []T
		if err := c.Get(ctx, path, query, &batch); err != nil {
			return err
		}
		rowsScanned += len(batch)
		if rowsScanned > maxRows {
			return &PaginationCapError{
				Path:        path,
				RowsScanned: rowsScanned,
				MaxRows:     maxRows,
				Page:        page,
				PageSize:    pageSize,
			}
		}
		if len(batch) > 0 {
			if err := onPage(batch); err != nil {
				return err
			}
		}
		if len(batch) == 0 || len(batch) < pageSize {
			break
		}
		if page >= listAllSafetyStopPages {
			return fmt.Errorf("pagination safety stop reached: path=%s page-size=%d", path, pageSize)
		}
		page++
	}

	return nil
}

func (c *Client) doJSON(ctx context.Context, baseURL, method, path string, query map[string]string, body any, out any) error {
	return c.doJSONValues(ctx, baseURL, method, path, valuesFromQueryMap(query), body, out)
}

func (c *Client) doJSONValues(ctx context.Context, baseURL, method, path string, query url.Values, body any, out any) error {
	var payload []byte
	var contentType string
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
		contentType = "application/json"
	}
	return c.doRequestValues(ctx, baseURL, method, path, query, contentType, payload, out)
}

// doRequest is the transport-level entry point used by both JSON and
// multipart callers. Retry, metrics, and tracing all live here so the
// content-type variants share a single hot path.
func (c *Client) doRequest(ctx context.Context, baseURL, method, path string, query map[string]string, contentType string, payload []byte, out any) error {
	return c.doRequestValues(ctx, baseURL, method, path, valuesFromQueryMap(query), contentType, payload, out)
}

func (c *Client) doRequestValues(ctx context.Context, baseURL, method, path string, query url.Values, contentType string, payload []byte, out any) error {
	endpoint := normalizeEndpoint(path)
	if c.breaker != nil {
		if err := c.breaker.Before(endpoint, method); err != nil {
			return err
		}
	}

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
					c.recordCircuitBreakerResult(endpoint, method, lastErr)
					return lastErr
				}
			}
			if err := sleepWithContext(ctx, waitDur); err != nil {
				c.recordCircuitBreakerResult(endpoint, method, lastErr)
				return err
			}
			// explicitRetryAfter is re-read below on retryable errors; no reset needed here.
		}

		err := c.doOnceValues(ctx, baseURL, method, path, endpoint, query, contentType, payload, out)
		if err == nil {
			c.recordCircuitBreakerResult(endpoint, method, nil)
			return nil
		}
		lastErr = err

		if !isRetryableError(err) {
			c.recordCircuitBreakerResult(endpoint, method, err)
			return err
		}
		explicitRetryAfter = 0
		if apiErr, ok := err.(*APIError); ok {
			explicitRetryAfter = apiErr.RetryAfter
		}
	}

	if lastErr != nil {
		c.recordCircuitBreakerResult(endpoint, method, lastErr)
		return lastErr
	}
	err := fmt.Errorf("request failed without specific error")
	c.recordCircuitBreakerResult(endpoint, method, err)
	return err
}

func (c *Client) recordCircuitBreakerResult(endpoint, method string, err error) {
	if c.breaker == nil {
		return
	}
	c.breaker.After(endpoint, method, isUpstreamFailure(err))
}

func (c *Client) doOnceValues(ctx context.Context, baseURL, method, path, endpoint string, query url.Values, contentType string, payload []byte, out any) error {
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
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return err
	}
	q := u.Query()
	for k, vs := range query {
		for _, v := range vs {
			if v != "" {
				q.Add(k, v)
			}
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
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
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
		bodyBytes, readErr := io.ReadAll(errorReader)
		if readErr != nil && ctx.Err() != nil {
			// The body read was cut short by caller cancellation or a
			// deadline; surface that rather than an APIError built from a
			// truncated body that would mislead the caller.
			return ctx.Err()
		}
		// Bound the connection-reuse drain. The previous unbounded
		// io.Copy(io.Discard, resp.Body) would pull a multi-GB error
		// body off the wire just to keep the keepalive connection
		// usable — wasted bytes a misconfigured upstream could
		// weaponise. 1 MiB is well past anything Clockify legitimately
		// returns on an error path; beyond that, throw the connection
		// away rather than copy.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, responseDrainLimit))

		var retryAfter time.Duration
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if d, ok := parseRetryAfter(ra); ok {
				retryAfter = d
			} else {
				metrics.UpstreamRetryAfterUnparseableTotal.Inc(endpoint)
				slog.Warn("retry_after_unparseable", "endpoint", endpoint, "raw", truncateRetryAfterForLog(ra))
			}
		}

		body := trimBody(string(bodyBytes))
		translation := TranslateAPIError(resp.StatusCode, body)
		return &APIError{
			Method:      method,
			Path:        path,
			StatusCode:  resp.StatusCode,
			Status:      resp.Status,
			Body:        body,
			RetryAfter:  retryAfter,
			Translation: &translation,
		}
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, responseDrainLimit))
		return nil
	}
	// Binary-aware path: when the caller wants the raw bytes (e.g.
	// shared-reports PDF/CSV/XLSX export), read into a fresh buffer
	// and stash headers — no JSON unmarshal. Same size cap as the
	// JSON path so an oversize binary still surfaces a clear error.
	if raw, ok := out.(*RawResponse); ok {
		respBuf := getBodyBuf()
		defer putBodyBuf(respBuf)
		n, err := io.Copy(respBuf, io.LimitReader(resp.Body, maxResponseBody+1))
		if err != nil {
			return err
		}
		if n > maxResponseBody {
			metrics.ClockifyResponsesOversizeTotal.Inc(method)
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, responseDrainLimit))
			return fmt.Errorf("clockify response too large: > %d bytes (method=%s path=%s)", maxResponseBody, method, path)
		}
		body := make([]byte, n)
		copy(body, respBuf.Bytes())
		raw.Header = resp.Header.Clone()
		raw.Body = body
		return nil
	}
	// Read the body into a pooled buffer, then unmarshal. Using
	// json.NewDecoder here would allocate fresh internal scan state
	// on every call; io.Copy into a reused buffer is cheaper for the
	// small-response case that dominates tool traffic.
	//
	// LimitReader is sized at maxResponseBody+1 so that a body
	// exceeding the cap surfaces as a clear "response too large"
	// error rather than silently truncating into a generic JSON
	// parse failure (the previous behaviour). ChatGPT's audit
	// flagged this as a defensive correctness gap: a truncated
	// upstream response is a different operator concern than a
	// genuinely malformed one, and operators couldn't distinguish
	// them.
	respBuf := getBodyBuf()
	defer putBodyBuf(respBuf)
	n, err := io.Copy(respBuf, io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return err
	}
	if n > maxResponseBody {
		metrics.ClockifyResponsesOversizeTotal.Inc(method)
		// Drain whatever is left so the connection can be reused
		// (bounded). The 1 MiB ceiling on the drain mirrors the
		// error-path drain — past that, we throw the connection
		// away.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, responseDrainLimit))
		return fmt.Errorf("clockify response too large: > %d bytes (method=%s path=%s)", maxResponseBody, method, path)
	}
	if n == 0 {
		// Some Clockify endpoints (notably the scheduling per-user
		// totals path) reply 200 with a zero-byte body when the query
		// matches no rows. Treat that as a successful empty response
		// rather than letting json.Unmarshal surface "unexpected end
		// of JSON input" as a tool error.
		return nil
	}
	return json.Unmarshal(respBuf.Bytes(), out)
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

// isRetryableError reports whether a failed request attempt should be
// retried: an APIError carrying a retryable HTTP status, or a transport-level
// failure (DNS, connection reset, TLS, dial/read timeout) that never produced
// a response. A caller-cancelled or deadline-exceeded request is never retried
// — that is the caller's decision, not an upstream fault.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return isRetryableStatus(apiErr.StatusCode)
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// isUpstreamFailure reports whether err should count against the circuit
// breaker. A 5xx response and a transport-level failure both mean the
// upstream is unhealthy, so a sustained run of them must open the breaker;
// 4xx responses, decode failures, and caller cancellation must not.
func isUpstreamFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, ErrCircuitBreakerOpen) {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500 && apiErr.StatusCode <= 599
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func parseRetryAfter(raw string) (time.Duration, bool) {
	if raw == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second, true
	}
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, time.RFC3339} {
		if at, err := time.Parse(layout, raw); err == nil {
			return time.Until(at), true
		}
	}
	return 0, false
}

func truncateRetryAfterForLog(raw string) string {
	if len(raw) <= 64 {
		return raw
	}
	return raw[:64]
}

func backoff(attempt int) time.Duration {
	base := 250.0 * math.Pow(2, float64(attempt-1))
	jitter := 0
	if n, err := rand.Int(rand.Reader, big.NewInt(125)); err == nil {
		jitter = int(n.Int64())
	}
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
