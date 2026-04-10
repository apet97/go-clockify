// Package metrics provides a stdlib-only Prometheus text exposition format
// encoder. It implements the minimum Counter, Histogram, and Gauge types
// needed to expose server-level instrumentation over an HTTP /metrics
// endpoint. No external dependencies.
//
// The implementation targets Prometheus text format version 0.0.4
// (https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md).
package metrics

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// DefaultBuckets mirrors Prometheus client_golang's default histogram buckets.
var DefaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

var nameRegexp = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Counter is a monotonically increasing value with optional labels.
type Counter struct {
	name, help string
	labels     []string
	values     sync.Map // map[string]*counterSample
}

type counterSample struct {
	labelValues []string
	value       atomic.Uint64
}

// Histogram records observations into cumulative bucket counts plus sum and count.
type Histogram struct {
	name, help string
	labels     []string
	buckets    []float64
	values     sync.Map // map[string]*histSample
}

type histSample struct {
	labelValues []string
	counts      []atomic.Uint64 // one per bucket plus one for +Inf
	sum         atomic.Uint64   // float64 bits (CAS loop)
	count       atomic.Uint64
}

// Gauge exposes a sampled value provided by the caller via SetFunc.
type Gauge struct {
	name, help string
	labels     []string
	mu         sync.Mutex
	valueFns   []gaugeEntry
}

type gaugeEntry struct {
	labelValues []string
	fn          func() float64
}

// Registry owns a set of metrics and writes them in Prometheus text format.
type Registry struct {
	mu         sync.Mutex
	counters   []*Counter
	histograms []*Histogram
	gauges     []*Gauge
	names      map[string]struct{}
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{names: map[string]struct{}{}}
}

// NewCounter registers a new counter. Panics on invalid names or duplicates.
func (r *Registry) NewCounter(name, help string, labels ...string) *Counter {
	validate(name, labels)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimName(name)
	c := &Counter{name: name, help: help, labels: append([]string(nil), labels...)}
	r.counters = append(r.counters, c)
	return c
}

// NewHistogram registers a new histogram. Panics on invalid names or duplicates.
func (r *Registry) NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	validate(name, labels)
	if len(buckets) == 0 {
		buckets = DefaultBuckets
	}
	// Ensure sorted ascending.
	sorted := append([]float64(nil), buckets...)
	sort.Float64s(sorted)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimName(name)
	h := &Histogram{
		name:    name,
		help:    help,
		labels:  append([]string(nil), labels...),
		buckets: sorted,
	}
	r.histograms = append(r.histograms, h)
	return h
}

// NewGauge registers a new gauge. Panics on invalid names or duplicates.
func (r *Registry) NewGauge(name, help string, labels ...string) *Gauge {
	validate(name, labels)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimName(name)
	g := &Gauge{name: name, help: help, labels: append([]string(nil), labels...)}
	r.gauges = append(r.gauges, g)
	return g
}

func (r *Registry) claimName(name string) {
	if _, dup := r.names[name]; dup {
		panic("metrics: duplicate metric name: " + name)
	}
	r.names[name] = struct{}{}
}

func validate(name string, labels []string) {
	if !nameRegexp.MatchString(name) {
		panic("metrics: invalid metric name: " + name)
	}
	for _, l := range labels {
		if !nameRegexp.MatchString(l) {
			panic("metrics: invalid label name: " + l)
		}
	}
}

// Counter API.

// Inc increments the counter by 1 for the given label set.
func (c *Counter) Inc(labelValues ...string) { c.Add(1, labelValues...) }

// Add increments the counter by delta. Silently dropped on label mismatch.
func (c *Counter) Add(delta uint64, labelValues ...string) {
	if len(labelValues) != len(c.labels) {
		return
	}
	key := labelKey(labelValues)
	v, ok := c.values.Load(key)
	if !ok {
		v, _ = c.values.LoadOrStore(key, &counterSample{
			labelValues: append([]string(nil), labelValues...),
		})
	}
	v.(*counterSample).value.Add(delta)
}

// Get returns the current value for a label set. Primarily used in tests.
func (c *Counter) Get(labelValues ...string) uint64 {
	if len(labelValues) != len(c.labels) {
		return 0
	}
	v, ok := c.values.Load(labelKey(labelValues))
	if !ok {
		return 0
	}
	return v.(*counterSample).value.Load()
}

// Histogram API.

// Observe records a single value under the given label set.
func (h *Histogram) Observe(value float64, labelValues ...string) {
	if len(labelValues) != len(h.labels) {
		return
	}
	key := labelKey(labelValues)
	v, ok := h.values.Load(key)
	if !ok {
		counts := make([]atomic.Uint64, len(h.buckets)+1)
		v, _ = h.values.LoadOrStore(key, &histSample{
			labelValues: append([]string(nil), labelValues...),
			counts:      counts,
		})
	}
	s := v.(*histSample)
	for i, ub := range h.buckets {
		if value <= ub {
			s.counts[i].Add(1)
		}
	}
	s.counts[len(h.buckets)].Add(1) // +Inf
	s.count.Add(1)
	for {
		old := s.sum.Load()
		next := math.Float64bits(math.Float64frombits(old) + value)
		if s.sum.CompareAndSwap(old, next) {
			break
		}
	}
}

// Gauge API.

// SetFunc registers (or replaces) the sampling closure for a label set.
// The closure is invoked at /metrics scrape time.
func (g *Gauge) SetFunc(fn func() float64, labelValues ...string) {
	if len(labelValues) != len(g.labels) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.valueFns {
		if equalStrings(g.valueFns[i].labelValues, labelValues) {
			g.valueFns[i].fn = fn
			return
		}
	}
	g.valueFns = append(g.valueFns, gaugeEntry{
		labelValues: append([]string(nil), labelValues...),
		fn:          fn,
	})
}

// Registry writer.

// WriteTo emits all registered metrics in Prometheus text format v0.0.4.
func (r *Registry) WriteTo(w io.Writer) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var total int64
	write := func(n int, err error) error {
		total += int64(n)
		return err
	}

	for _, c := range r.counters {
		if err := writeHelpType(w, c.name, c.help, "counter", write); err != nil {
			return total, err
		}
		samples := collectCounterSamples(c)
		for _, s := range samples {
			line := fmt.Sprintf("%s%s %s\n",
				c.name,
				formatLabels(c.labels, s.labelValues),
				formatFloat(float64(s.value)),
			)
			if err := write(io.WriteString(w, line)); err != nil {
				return total, err
			}
		}
	}

	for _, h := range r.histograms {
		if err := writeHelpType(w, h.name, h.help, "histogram", write); err != nil {
			return total, err
		}
		samples := collectHistSamples(h)
		for _, s := range samples {
			// Bucket lines.
			for i, ub := range h.buckets {
				labels := append([]string(nil), s.labelValues...)
				line := fmt.Sprintf("%s_bucket%s %s\n",
					h.name,
					formatLabelsWithLE(h.labels, labels, formatFloat(ub)),
					strconv.FormatUint(s.counts[i], 10),
				)
				if err := write(io.WriteString(w, line)); err != nil {
					return total, err
				}
			}
			// +Inf bucket.
			line := fmt.Sprintf("%s_bucket%s %s\n",
				h.name,
				formatLabelsWithLE(h.labels, s.labelValues, "+Inf"),
				strconv.FormatUint(s.counts[len(h.buckets)], 10),
			)
			if err := write(io.WriteString(w, line)); err != nil {
				return total, err
			}
			// _sum and _count.
			sumLine := fmt.Sprintf("%s_sum%s %s\n",
				h.name, formatLabels(h.labels, s.labelValues), formatFloat(s.sum),
			)
			if err := write(io.WriteString(w, sumLine)); err != nil {
				return total, err
			}
			countLine := fmt.Sprintf("%s_count%s %s\n",
				h.name, formatLabels(h.labels, s.labelValues), strconv.FormatUint(s.count, 10),
			)
			if err := write(io.WriteString(w, countLine)); err != nil {
				return total, err
			}
		}
	}

	for _, g := range r.gauges {
		if err := writeHelpType(w, g.name, g.help, "gauge", write); err != nil {
			return total, err
		}
		samples := collectGaugeSamples(g)
		for _, s := range samples {
			line := fmt.Sprintf("%s%s %s\n",
				g.name,
				formatLabels(g.labels, s.labelValues),
				formatFloat(s.value),
			)
			if err := write(io.WriteString(w, line)); err != nil {
				return total, err
			}
		}
	}

	return total, nil
}

func writeHelpType(w io.Writer, name, help, typ string, write func(int, error) error) error {
	if err := write(fmt.Fprintf(w, "# HELP %s %s\n", name, escapeHelp(help))); err != nil {
		return err
	}
	if err := write(fmt.Fprintf(w, "# TYPE %s %s\n", name, typ)); err != nil {
		return err
	}
	return nil
}

// Sample collection with deterministic ordering.

type counterSnapshot struct {
	labelValues []string
	value       uint64
	key         string
}

func collectCounterSamples(c *Counter) []counterSnapshot {
	var out []counterSnapshot
	c.values.Range(func(k, v any) bool {
		cs := v.(*counterSample)
		out = append(out, counterSnapshot{
			labelValues: cs.labelValues,
			value:       cs.value.Load(),
			key:         k.(string),
		})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

type histSnapshot struct {
	labelValues []string
	counts      []uint64
	sum         float64
	count       uint64
	key         string
}

func collectHistSamples(h *Histogram) []histSnapshot {
	var out []histSnapshot
	h.values.Range(func(k, v any) bool {
		hs := v.(*histSample)
		counts := make([]uint64, len(hs.counts))
		for i := range hs.counts {
			counts[i] = hs.counts[i].Load()
		}
		out = append(out, histSnapshot{
			labelValues: hs.labelValues,
			counts:      counts,
			sum:         math.Float64frombits(hs.sum.Load()),
			count:       hs.count.Load(),
			key:         k.(string),
		})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

type gaugeSnapshot struct {
	labelValues []string
	value       float64
	key         string
}

func collectGaugeSamples(g *Gauge) []gaugeSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]gaugeSnapshot, 0, len(g.valueFns))
	for _, e := range g.valueFns {
		out = append(out, gaugeSnapshot{
			labelValues: e.labelValues,
			value:       e.fn(),
			key:         labelKey(e.labelValues),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// Formatting helpers.

func formatLabels(names, values []string) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(n)
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(values[i]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func formatLabelsWithLE(names, values []string, le string) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, n := range names {
		b.WriteString(n)
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(values[i]))
		b.WriteString(`",`)
	}
	b.WriteString(`le="`)
	b.WriteString(le)
	b.WriteString(`"}`)
	return b.String()
}

func escapeLabelValue(s string) string {
	if !strings.ContainsAny(s, `\"`+"\n") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeHelp(s string) string {
	if !strings.ContainsAny(s, "\\\n") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatFloat(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "+Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func labelKey(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, "\x00")
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Default registry and pre-registered metrics.

// Default is the package-level registry used by the Clockify MCP server.
var Default = NewRegistry()

var (
	// ToolCallsTotal counts tools/call dispatches by tool name and outcome.
	ToolCallsTotal *Counter
	// ToolCallDuration records dispatch duration in seconds.
	ToolCallDuration *Histogram
	// RateLimitRejections counts rate limiter rejections by kind.
	RateLimitRejections *Counter
	// HTTPRequestsTotal counts HTTP requests by path, method, and status.
	HTTPRequestsTotal *Counter
	// ReadyState is 1 when the server is ready, 0 otherwise.
	ReadyState *Gauge
	// BuildInfo exposes build metadata; value is always 1.
	BuildInfo *Gauge
	// InFlightToolCalls reports the dispatch-layer in-flight goroutine count.
	InFlightToolCalls *Gauge
)

func init() {
	ToolCallsTotal = Default.NewCounter(
		"clockify_mcp_tool_calls_total",
		"Total tools/call invocations by tool name and outcome.",
		"tool", "outcome",
	)
	ToolCallDuration = Default.NewHistogram(
		"clockify_mcp_tool_call_duration_seconds",
		"Duration of tools/call dispatch in seconds.",
		DefaultBuckets,
		"tool",
	)
	RateLimitRejections = Default.NewCounter(
		"clockify_mcp_rate_limit_rejections_total",
		"Rate limiter rejections by kind (concurrency, window).",
		"kind",
	)
	HTTPRequestsTotal = Default.NewCounter(
		"clockify_mcp_http_requests_total",
		"HTTP requests by path, method, and status.",
		"path", "method", "status",
	)
	ReadyState = Default.NewGauge(
		"clockify_mcp_ready_state",
		"1 when the server reports ready, 0 otherwise.",
	)
	BuildInfo = Default.NewGauge(
		"clockify_mcp_build_info",
		"Build metadata. Value is always 1.",
		"version",
	)
	InFlightToolCalls = Default.NewGauge(
		"clockify_mcp_inflight_tool_calls",
		"Current in-flight tools/call dispatch goroutines.",
	)
}
