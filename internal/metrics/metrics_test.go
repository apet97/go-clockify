package metrics

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestCounter_Format(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("test_counter_total", "a test counter", "path", "method")
	c.Inc("/foo", "GET")
	c.Inc("/foo", "GET")
	c.Add(5, "/bar", "POST")

	// Escaping: a label value containing quote and backslash.
	c.Inc(`he said "hi"\back`, "GET")

	var buf bytes.Buffer
	if _, err := r.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "# HELP test_counter_total a test counter") {
		t.Errorf("missing HELP line: %s", out)
	}
	if !strings.Contains(out, "# TYPE test_counter_total counter") {
		t.Errorf("missing TYPE line: %s", out)
	}
	if !strings.Contains(out, `test_counter_total{path="/foo",method="GET"} 2`) {
		t.Errorf("missing counter sample for /foo GET: %s", out)
	}
	if !strings.Contains(out, `test_counter_total{path="/bar",method="POST"} 5`) {
		t.Errorf("missing counter sample for /bar POST: %s", out)
	}
	if !strings.Contains(out, `path="he said \"hi\"\\back"`) {
		t.Errorf("label value not properly escaped: %s", out)
	}
}

func TestHistogram_Format(t *testing.T) {
	r := NewRegistry()
	h := r.NewHistogram("req_duration_seconds", "request duration", []float64{0.1, 1.0, 10.0}, "route")
	h.Observe(0.05, "a")
	h.Observe(0.5, "a")
	h.Observe(5.0, "a")

	var buf bytes.Buffer
	if _, err := r.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()

	checks := []string{
		`req_duration_seconds_bucket{route="a",le="0.1"} 1`,
		`req_duration_seconds_bucket{route="a",le="1"} 2`,
		`req_duration_seconds_bucket{route="a",le="10"} 3`,
		`req_duration_seconds_bucket{route="a",le="+Inf"} 3`,
		`req_duration_seconds_count{route="a"} 3`,
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("missing line %q in output:\n%s", want, out)
		}
	}

	// Sum line: 0.05 + 0.5 + 5.0 = 5.55
	sumLine := findLine(out, `req_duration_seconds_sum{route="a"}`)
	if sumLine == "" {
		t.Fatalf("no _sum line: %s", out)
	}
	fields := strings.Fields(sumLine)
	if len(fields) != 2 {
		t.Fatalf("bad sum line: %q", sumLine)
	}
	got := parseFloat(t, fields[1])
	if math.Abs(got-5.55) > 1e-9 {
		t.Errorf("sum = %v, want 5.55", got)
	}
}

func TestGauge_Format(t *testing.T) {
	r := NewRegistry()
	g := r.NewGauge("inflight_calls", "current in-flight calls")
	counter := 0
	g.SetFunc(func() float64 {
		counter++
		return float64(counter)
	})

	var buf bytes.Buffer
	if _, err := r.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "# TYPE inflight_calls gauge") {
		t.Errorf("missing gauge TYPE: %s", out)
	}
	if !strings.Contains(out, "inflight_calls 1") {
		t.Errorf("expected gauge value 1: %s", out)
	}

	// Second scrape picks up updated value.
	var buf2 bytes.Buffer
	if _, err := r.WriteTo(&buf2); err != nil {
		t.Fatalf("write2: %v", err)
	}
	if !strings.Contains(buf2.String(), "inflight_calls 2") {
		t.Errorf("expected gauge value 2 on second scrape: %s", buf2.String())
	}
}

func TestGauge_Labels(t *testing.T) {
	r := NewRegistry()
	g := r.NewGauge("build_info", "build metadata", "version")
	g.SetFunc(func() float64 { return 1 }, "1.2.3")

	var buf bytes.Buffer
	if _, err := r.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), `build_info{version="1.2.3"} 1`) {
		t.Errorf("missing labelled gauge: %s", buf.String())
	}
}

func TestWriteTo_DeterministicOrder(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("ordered_total", "ordered counter", "k")
	c.Inc("z")
	c.Inc("a")
	c.Inc("m")

	var a, b bytes.Buffer
	if _, err := r.WriteTo(&a); err != nil {
		t.Fatal(err)
	}
	if _, err := r.WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Errorf("non-deterministic output:\n--- a ---\n%s\n--- b ---\n%s", a.String(), b.String())
	}
}

func TestCounter_Concurrent(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("race_total", "concurrent counter", "k")

	const goroutines = 100
	const perGoroutine = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range perGoroutine {
				c.Inc("x")
			}
		}()
	}
	wg.Wait()
	if got := c.Get("x"); got != goroutines*perGoroutine {
		t.Errorf("counter = %d, want %d", got, goroutines*perGoroutine)
	}
}

func TestLabelEscape(t *testing.T) {
	tests := map[string]string{
		"plain":              "plain",
		`has "quote"`:        `has \"quote\"`,
		`back\slash`:         `back\\slash`,
		"line\nbreak":        `line\nbreak`,
		`all "\\` + "\n end": `all \"\\\\\n end`,
	}
	for in, want := range tests {
		if got := escapeLabelValue(in); got != want {
			t.Errorf("escapeLabelValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHistogram_DefaultBuckets(t *testing.T) {
	want := []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	if len(DefaultBuckets) != len(want) {
		t.Fatalf("DefaultBuckets length %d, want %d", len(DefaultBuckets), len(want))
	}
	for i, b := range DefaultBuckets {
		if b != want[i] {
			t.Errorf("bucket[%d] = %v, want %v", i, b, want[i])
		}
	}
}

func TestInvalidName_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on invalid name")
		}
	}()
	r := NewRegistry()
	r.NewCounter("bad name!", "help")
}

func TestDuplicateName_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate name")
		}
	}()
	r := NewRegistry()
	r.NewCounter("dup_total", "help")
	r.NewCounter("dup_total", "help")
}

func TestDefaultRegistryPreRegistered(t *testing.T) {
	// Exercise the pre-registered metrics.
	ToolCallsTotal.Inc("some_tool", "success")
	ToolCallDuration.Observe(0.2, "some_tool")
	RateLimitRejections.Inc("window")
	HTTPRequestsTotal.Inc("/mcp", "POST", "200")
	BuildInfo.SetFunc(func() float64 { return 1 }, "test-version")
	ReadyState.SetFunc(func() float64 { return 1 })
	InFlightToolCalls.SetFunc(func() float64 { return 3 })

	var buf bytes.Buffer
	if _, err := Default.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"clockify_mcp_tool_calls_total",
		"clockify_mcp_tool_call_duration_seconds",
		"clockify_mcp_rate_limit_rejections_total",
		"clockify_mcp_http_requests_total",
		"clockify_mcp_build_info",
		"clockify_mcp_ready_state",
		"clockify_mcp_inflight_tool_calls",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("default registry missing %s", want)
		}
	}
}

func findLine(out, prefix string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func parseFloat(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("parseFloat %q: %v", s, err)
	}
	return f
}
