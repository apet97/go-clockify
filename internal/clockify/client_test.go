package clockify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientGetEmpty200LeavesOutZero(t *testing.T) {
	// Some Clockify endpoints (notably scheduling per-user totals)
	// answer 200 with a zero-byte body when the query matches no
	// rows. Earlier client behaviour bubbled "unexpected end of JSON
	// input" up to callers; this asserts the new tolerance contract:
	// no error, out left at its zero value.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out map[string]any
	if err := c.Get(context.Background(), "/empty", nil, &out); err != nil {
		t.Fatalf("expected nil error on empty 200, got %v", err)
	}
	if out != nil {
		t.Fatalf("expected out to remain nil, got %#v", out)
	}
}

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

func TestClientRefusesCrossHostRedirect(t *testing.T) {
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Store(true)
		if got := r.Header.Get("X-Api-Key"); got != "" {
			t.Fatalf("redirect target received X-Api-Key: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/capture", http.StatusFound)
	}))
	defer source.Close()

	c := NewClient("test-key", source.URL, 5*time.Second, 0)
	var out map[string]any
	if err := c.Get(context.Background(), "/redirect", nil, &out); err == nil {
		t.Fatal("expected cross-host redirect to be refused")
	}
	if redirected.Load() {
		t.Fatal("cross-host redirect target should not have been contacted")
	}
}

func TestClientRefusesSchemeDowngradeRedirect(t *testing.T) {
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+r.Host+"/capture", http.StatusFound)
	}))
	defer source.Close()

	c := NewClient("test-key", source.URL, 5*time.Second, 0)
	c.httpClient.Transport = source.Client().Transport
	var out map[string]any
	err := c.Get(context.Background(), "/redirect", nil, &out)
	if err == nil {
		t.Fatal("expected scheme downgrade redirect to be refused")
	}
	if !strings.Contains(err.Error(), "scheme downgrade") {
		t.Fatalf("error = %v, want scheme downgrade", err)
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

// TestClientErrorBodyReadHonoursContextCancellation proves that when the
// error-body read is cut short by a context deadline, the client returns the
// context error rather than an APIError built from a truncated body.
func TestClientErrorBodyReadHonoursContextCancellation(t *testing.T) {
	// The server sends a 400 with a Content-Length far larger than the bytes
	// it actually writes, then blocks — so the client's error-body read
	// stalls until its context deadline fires mid-read.
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "100000")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"partial`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release // hold the body open past the client's deadline
	}))
	defer ts.Close()
	defer close(release)

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	var out map[string]any
	err := c.Get(ctx, "/x", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v (%T), want context.DeadlineExceeded — a truncated "+
			"error body must not surface as an APIError", err, err)
	}
}

func TestClientAPIErrorBodyRedactsSecretCanaries(t *testing.T) {
	const (
		apiKeyCanary      = "canary-clockify-api-key-1234567890"
		webhookAuthCanary = "canary-webhook-auth-token-1234567890"
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"message":"bad request",
			"apiKey":"` + apiKeyCanary + `",
			"details":{"authToken":"` + webhookAuthCanary + `"}
		}`))
	}))
	defer ts.Close()

	c := NewClient(apiKeyCanary, ts.URL, 5*time.Second, 0)
	var out map[string]any
	err := c.Get(context.Background(), "/user", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	for _, got := range []struct {
		name  string
		value string
	}{
		{"APIError.Body", apiErr.Body},
		{"APIError.Error", apiErr.Error()},
		{"APIError translation", fmt.Sprint(apiErr.ErrorTranslation())},
	} {
		if strings.Contains(got.value, apiKeyCanary) || strings.Contains(got.value, webhookAuthCanary) {
			t.Fatalf("%s leaked secret canary: %s", got.name, got.value)
		}
	}
	if !strings.Contains(apiErr.Body, "[REDACTED]") {
		t.Fatalf("expected redacted marker in APIError.Body, got %s", apiErr.Body)
	}
}

func TestClientAPIErrorBodyRedactsBeforeTruncating(t *testing.T) {
	const apiKeyCanary = "canary-clockify-api-key-before-truncate"
	body := strings.Repeat("x", 900) + `{"apiKey":"` + apiKeyCanary + `","message":"bad request"}` + strings.Repeat("y", 200)
	if len(body) <= 1000 {
		t.Fatal("test body must exercise the truncation path")
	}
	got := trimBody(body)
	if strings.Contains(got, apiKeyCanary) {
		t.Fatalf("trimBody leaked secret canary after truncation: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected redacted marker before truncation, got %s", got)
	}
	if len(got) > 1003 {
		t.Fatalf("trimmed body length=%d want <=1003", len(got))
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

func TestCircuitBreakerOpensAfterFinal5xx(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"down"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	c.SetCircuitBreaker(CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 2,
		OpenDuration:     time.Minute,
		HalfOpenProbes:   1,
	})
	for i := range 2 {
		var out map[string]any
		if err := c.Get(context.Background(), "/test", nil, &out); err == nil {
			t.Fatalf("call %d: expected upstream error", i+1)
		}
	}
	var out map[string]any
	err := c.Get(context.Background(), "/test", nil, &out)
	if !errors.Is(err, ErrCircuitBreakerOpen) {
		t.Fatalf("third call error = %v, want ErrCircuitBreakerOpen", err)
	}
	if got := count.Load(); got != 2 {
		t.Fatalf("breaker should fast-fail before wire call, upstream requests = %d", got)
	}
}

func TestCircuitBreakerHalfOpenSuccessCloses(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"down"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	c.SetCircuitBreaker(CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 2,
		OpenDuration:     10 * time.Second,
		HalfOpenProbes:   1,
	})
	now := time.Unix(1700000000, 0)
	c.breaker.now = func() time.Time { return now }

	for i := range 2 {
		var out map[string]any
		if err := c.Get(context.Background(), "/test", nil, &out); err == nil {
			t.Fatalf("call %d: expected upstream error", i+1)
		}
	}
	now = now.Add(11 * time.Second)
	var out map[string]any
	if err := c.Get(context.Background(), "/test", nil, &out); err != nil {
		t.Fatalf("half-open probe should close breaker on success: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("unexpected response: %#v", out)
	}
}

func TestEncodeMultipartWithFiles(t *testing.T) {
	payload, contentType, err := encodeMultipartWithFiles(url.Values{"kind": []string{"avatar"}}, []MultipartFile{{
		FieldName:   "file",
		Filename:    "avatar.png",
		ContentType: "image/png",
		Data:        []byte("png"),
	}})
	if err != nil {
		t.Fatalf("encodeMultipartWithFiles: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("contentType = %q", contentType)
	}
	body := string(payload)
	for _, want := range []string{`name="kind"`, "avatar", `name="file"; filename="avatar.png"`, "Content-Type: image/png", "png"} {
		if !strings.Contains(body, want) {
			t.Fatalf("multipart body missing %q: %s", want, body)
		}
	}
}

func TestEncodeExpenseUpdateMultipartWithoutFile(t *testing.T) {
	form := url.Values{
		"amount":       {"12.50"},
		"categoryId":   {"cat-1"},
		"changeFields": {"AMOUNT", "NOTES"},
		"date":         {"2026-07-19T00:00:00Z"},
		"notes":        {"Taxi"},
		"userId":       {"user-1"},
	}
	payload, contentType, err := encodeMultipartWithFiles(form, nil)
	if err != nil {
		t.Fatalf("encode expense update multipart without file: %v", err)
	}

	request := httptest.NewRequest(http.MethodPut, "/expenses/expense-1", bytes.NewReader(payload))
	request.Header.Set("Content-Type", contentType)
	if err := request.ParseMultipartForm(int64(len(payload))); err != nil {
		t.Fatalf("parse expense update multipart without file: %v", err)
	}
	assertExactMultipartValues(t, request.MultipartForm.Value, form)
	if len(request.MultipartForm.File) != 0 {
		t.Fatalf("expense update without file emitted file parts: %#v", request.MultipartForm.File)
	}
}

func TestEncodeExpenseUpdateMultipartWithFile(t *testing.T) {
	form := url.Values{
		"amount":       {"12.50"},
		"categoryId":   {"cat-1"},
		"changeFields": {"AMOUNT", "FILE"},
		"date":         {"2026-07-19T00:00:00Z"},
		"userId":       {"user-1"},
	}
	payload, contentType, err := encodeMultipartWithFiles(form, []MultipartFile{{
		FieldName:   "file",
		Filename:    "receipt.png",
		ContentType: "image/png",
		Data:        []byte("png-receipt"),
	}})
	if err != nil {
		t.Fatalf("encode expense update multipart with file: %v", err)
	}

	request := httptest.NewRequest(http.MethodPut, "/expenses/expense-1", bytes.NewReader(payload))
	request.Header.Set("Content-Type", contentType)
	if err := request.ParseMultipartForm(int64(len(payload))); err != nil {
		t.Fatalf("parse expense update multipart with file: %v", err)
	}
	assertExactMultipartValues(t, request.MultipartForm.Value, form)
	if len(request.MultipartForm.File) != 1 || len(request.MultipartForm.File["file"]) != 1 {
		t.Fatalf("expense update file parts = %#v, want exactly one field named file", request.MultipartForm.File)
	}
	fileHeader := request.MultipartForm.File["file"][0]
	if fileHeader.Filename != "receipt.png" || fileHeader.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("expense update file metadata = filename %q content-type %q", fileHeader.Filename, fileHeader.Header.Get("Content-Type"))
	}
	file, err := fileHeader.Open()
	if err != nil {
		t.Fatalf("open expense update file part: %v", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read expense update file part: %v", err)
	}
	if string(data) != "png-receipt" {
		t.Fatalf("expense update file bytes = %q, want png-receipt", data)
	}
}

func assertExactMultipartValues(t *testing.T, got map[string][]string, want url.Values) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("multipart scalar field count = %d, want %d; got=%#v", len(got), len(want), got)
	}
	for key, wantValues := range want {
		gotValues, ok := got[key]
		if !ok {
			t.Fatalf("multipart scalar fields missing %q; got=%#v", key, got)
		}
		if len(gotValues) != len(wantValues) {
			t.Fatalf("multipart scalar field %q values=%#v want=%#v", key, gotValues, wantValues)
		}
		for index, wantValue := range wantValues {
			if gotValues[index] != wantValue {
				t.Fatalf("multipart scalar field %q value[%d]=%q want=%q", key, index, gotValues[index], wantValue)
			}
		}
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

func TestParseRetryAfterAdditionalFormats(t *testing.T) {
	future := time.Now().Add(30 * time.Second).UTC()
	cases := []struct {
		name string
		raw  string
		ok   bool
	}{
		{name: "empty", raw: "", ok: false},
		{name: "seconds", raw: "30", ok: true},
		{name: "rfc1123", raw: future.Format(time.RFC1123), ok: true},
		{name: "rfc1123z", raw: future.Format(time.RFC1123Z), ok: true},
		{name: "rfc3339", raw: future.Format(time.RFC3339), ok: true},
		{name: "garbage", raw: "not a retry-after", ok: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseRetryAfter(tt.raw)
			if ok != tt.ok {
				t.Fatalf("parseRetryAfter(%q) ok=%v, want %v", tt.raw, ok, tt.ok)
			}
			if ok && got == 0 {
				t.Fatalf("parseRetryAfter(%q) returned zero duration", tt.raw)
			}
		})
	}
}

func TestRetryAfterMalformedLeavesRetryAfterUnset(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "not a retry-after")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"slow down"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out map[string]any
	err := c.Get(context.Background(), "/test", nil, &out)
	if err == nil {
		t.Fatal("expected API error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.RetryAfter != 0 {
		t.Fatalf("malformed Retry-After should not set retry duration, got %s", apiErr.RetryAfter)
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
	// page-size=200 (default), only 2 items returned -> single page
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

func TestListAllMaxRowsExceededReturnsTypedError(t *testing.T) {
	type item struct {
		ID string `json:"id"`
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page-size"))
		if pageSize <= 0 {
			pageSize = 2
		}
		items := make([]item, 0, pageSize)
		for i := 0; i < pageSize; i++ {
			items = append(items, item{ID: fmt.Sprintf("item-%d", i)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	result, err := ListAllWithOptions[item](
		context.Background(),
		c,
		"/items",
		map[string]string{"page-size": "2"},
		ListAllOptions{MaxRows: 3},
	)
	if err == nil {
		t.Fatal("expected max-row pagination error")
	}
	if result != nil {
		t.Fatalf("expected no partial result on error, got %+v", result)
	}
	var capErr *PaginationCapError
	if !errors.As(err, &capErr) {
		t.Fatalf("expected PaginationCapError, got %T: %v", err, err)
	}
	if capErr.Path != "/items" || capErr.RowsScanned != 4 || capErr.MaxRows != 3 || capErr.Page != 2 || capErr.PageSize != 2 {
		t.Fatalf("unexpected cap error fields: %+v", capErr)
	}
	for _, want := range []string{"pagination row cap exceeded", "path=/items", "rows_scanned=4", "max_rows=3"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q should contain %q", err, want)
		}
	}
}

func TestListAllFuncStreamsPages(t *testing.T) {
	type item struct {
		ID string `json:"id"`
	}

	allItems := []item{
		{ID: "1"},
		{ID: "2"},
		{ID: "3"},
		{ID: "4"},
		{ID: "5"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page-size"))
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
	var pageLengths []int
	var seen []string
	err := ListAllFuncWithOptions[item](
		context.Background(),
		c,
		"/items",
		map[string]string{"page-size": "2"},
		ListAllOptions{MaxRows: 10},
		func(batch []item) error {
			pageLengths = append(pageLengths, len(batch))
			for _, it := range batch {
				seen = append(seen, it.ID)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := strings.Join(seen, ","), "1,2,3,4,5"; got != want {
		t.Fatalf("streamed ids: got %q want %q", got, want)
	}
	if got, want := fmt.Sprint(pageLengths), "[2 2 1]"; got != want {
		t.Fatalf("page lengths: got %s want %s", got, want)
	}
}

func TestListAllSafetyStopErrorIncludesPath(t *testing.T) {
	type item struct {
		ID string `json:"id"`
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"x"}]`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	_, err := ListAll[item](context.Background(), c, "/items", map[string]string{"page-size": "1"})
	if err == nil {
		t.Fatal("expected pagination safety stop error")
	}
	for _, want := range []string{"pagination safety stop reached", "path=/items", "page-size=1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q should contain %q", err, want)
		}
	}
}

// --- Edge Cases ---

func TestBackoffIncreasing(t *testing.T) {
	// Run multiple samples to account for jitter and confirm the trend.
	const samples = 20
	for s := range samples {
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

func TestPostAuditLog(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/workspaces/ws-1/audit-log" {
			t.Fatalf("path = %s, want /workspaces/ws-1/audit-log", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := body["actions"]; !ok {
			t.Fatalf("request body missing actions: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"response":[{"action":"CREATE_PROJECT"}]}`)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out map[string]any
	if err := c.PostAuditLog(context.Background(), "/workspaces/ws-1/audit-log", map[string]any{"actions": []string{"CREATE_PROJECT"}}, &out); err != nil {
		t.Fatalf("PostAuditLog: %v", err)
	}
	rows, ok := out["response"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("response = %v, want one audit-log row", out["response"])
	}
}

func TestAuditLogBaseURLResolvesDocumentedHost(t *testing.T) {
	canonical := NewClient("test-key", DocumentedHostMain, 5*time.Second, 0)
	if got := canonical.AuditLogBaseURL(); got != DocumentedHostAuditLog {
		t.Fatalf("AuditLogBaseURL() = %s, want %s", got, DocumentedHostAuditLog)
	}
	if got := canonical.DocumentedBaseURL(DocumentedHostAuditLog); got != DocumentedHostAuditLog {
		t.Fatalf("DocumentedBaseURL(audit-log) = %s, want %s", got, DocumentedHostAuditLog)
	}
	// A non-canonical base URL (test stub or proxy) stays pinned so fake
	// servers do not need per-host wiring.
	stub := NewClient("test-key", "http://127.0.0.1:1", 5*time.Second, 0)
	if got := stub.AuditLogBaseURL(); got != "http://127.0.0.1:1" {
		t.Fatalf("AuditLogBaseURL() for stub = %s, want unchanged base", got)
	}
}

func TestClientNoOutputSuccessDrainsResponseBodyBounded(t *testing.T) {
	body := &countingBody{remaining: responseDrainLimit * 2}
	c := NewClient("test-key", "http://clockify.test", 5*time.Second, 0)
	c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       body,
			Request:    r,
		}, nil
	})

	if err := c.Delete(context.Background(), "/items/delete-me"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if body.read != responseDrainLimit {
		t.Fatalf("drained %d bytes, want bounded drain of %d", body.read, responseDrainLimit)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestDeleteWithQueryCaptureBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/items/deleted-123" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "deleted-123", "status": "DELETED"})
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out map[string]any
	if err := c.DeleteWithQueryCapture(context.Background(), "/items/deleted-123", nil, &out); err != nil {
		t.Fatalf("delete capture: %v", err)
	}
	if out["id"] != "deleted-123" || out["status"] != "DELETED" {
		t.Fatalf("captured body = %+v", out)
	}
}

func TestDeleteWithQueryCaptureEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/items/deleted-123" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	var out any
	if err := c.DeleteWithQueryCapture(context.Background(), "/items/deleted-123", nil, &out); err != nil {
		t.Fatalf("delete capture empty: %v", err)
	}
	if out != nil {
		t.Fatalf("empty body populated out: %#v", out)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type countingBody struct {
	remaining int64
	read      int64
	closed    bool
}

func (b *countingBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	for i := range p {
		p[i] = 'x'
	}
	n := len(p)
	b.remaining -= int64(n)
	b.read += int64(n)
	return n, nil
}

func (b *countingBody) Close() error {
	b.closed = true
	return nil
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
	for i := range n {
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

func TestClientResponseBodyLimitIsConfigurable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":"1234567890"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	defer c.Close()
	c.SetMaxResponseBodyBytes(8)

	var out map[string]any
	err := c.Get(context.Background(), "/items", nil, &out)
	if err == nil {
		t.Fatal("expected response-too-large error")
	}
	if !contains(err.Error(), "response too large: > 8 bytes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientRawResponseBodyLimitIsConfigurable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	defer c.Close()
	c.SetMaxResponseBodyBytes(4)

	_, err := c.RequestRawValues(context.Background(), false, http.MethodGet, "/export", nil, nil)
	if err == nil {
		t.Fatal("expected raw response-too-large error")
	}
	if !contains(err.Error(), "response too large: > 4 bytes") {
		t.Fatalf("unexpected error: %v", err)
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

// ---------------------------------------------------------------------------
// RequestRawValues / RawResponse coverage. The shared-reports and
// invoice export handlers depend on the raw branch in doOnce; the
// JSON-mode siblings are well-covered via the Get/Post/Put/Delete
// tests above.
// ---------------------------------------------------------------------------

func TestRequestRawValuesSuccessReturnsBytesAndHeaders(t *testing.T) {
	pdfMagic := []byte{'%', 'P', 'D', 'F'}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shared-reports/abc" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("exportType") != "PDF" {
			t.Fatalf("missing ?exportType=PDF query: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "filename=foo.pdf")
		_, _ = w.Write(pdfMagic)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	raw, err := c.RequestRawValues(context.Background(), false, http.MethodGet,
		"/shared-reports/abc", url.Values{"exportType": {"PDF"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := raw.Header.Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type want application/pdf, got %q", got)
	}
	if got := raw.Header.Get("Content-Disposition"); got != "filename=foo.pdf" {
		t.Fatalf("Content-Disposition want filename=foo.pdf, got %q", got)
	}
	if string(raw.Body) != string(pdfMagic) {
		t.Fatalf("body bytes mismatch: want %v got %v", pdfMagic, raw.Body)
	}
}

func TestRequestRawValuesAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"NOT FOUND"}`))
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	raw, err := c.RequestRawValues(context.Background(), false, http.MethodGet,
		"/shared-reports/missing", nil, nil)
	if err == nil {
		t.Fatalf("expected error on 404, got nil (raw=%v)", raw)
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestRequestRawValuesEmptyBodyOK(t *testing.T) {
	// Some Reports endpoints reply 200 with no body (e.g. zero-byte
	// CSV — rare but observed). The raw branch must tolerate it,
	// returning an empty Body slice rather than erroring.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("test-key", ts.URL, 5*time.Second, 0)
	raw, err := c.RequestRawValues(context.Background(), false, http.MethodGet,
		"/shared-reports/empty", url.Values{"exportType": {"CSV"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error on empty body: %v", err)
	}
	if len(raw.Body) != 0 {
		t.Fatalf("expected empty body, got %d bytes", len(raw.Body))
	}
	if got := raw.Header.Get("Content-Type"); got != "text/csv" {
		t.Fatalf("Content-Type want text/csv, got %q", got)
	}
}
