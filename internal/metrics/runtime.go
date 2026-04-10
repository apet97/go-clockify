package metrics

import (
	"os"
	"runtime"
	"runtime/metrics"
	"sync/atomic"
	"time"
)

// registerRuntimeMetrics registers Go runtime and process metrics as lazy
// gauges on the given registry. Metric names mirror the well-known Prometheus
// `go_*` / `process_*` families so existing dashboards and alerts work.
//
// Values are read at scrape time via Gauge.SetFunc, so there is zero overhead
// between scrapes. We use runtime/metrics.Read (lock-free for the metrics we
// sample) instead of runtime.ReadMemStats (stop-the-world).
func registerRuntimeMetrics(r *Registry) {
	// go_goroutines: current number of goroutines.
	r.NewGauge("go_goroutines", "Number of goroutines that currently exist.").
		SetFunc(func() float64 { return float64(runtime.NumGoroutine()) })

	// go_gomaxprocs: current GOMAXPROCS value.
	r.NewGauge("go_gomaxprocs", "Current GOMAXPROCS setting.").
		SetFunc(func() float64 { return float64(runtime.GOMAXPROCS(0)) })

	// go_memstats_heap_alloc_bytes: heap bytes allocated and still in use.
	r.NewGauge("go_memstats_heap_alloc_bytes",
		"Number of heap bytes allocated and still in use.").
		SetFunc(func() float64 {
			return float64(readUint64("/memory/classes/heap/objects:bytes"))
		})

	// go_memstats_heap_inuse_bytes
	r.NewGauge("go_memstats_heap_inuse_bytes",
		"Number of heap bytes that are in use.").
		SetFunc(func() float64 {
			return float64(
				readUint64("/memory/classes/heap/objects:bytes") +
					readUint64("/memory/classes/heap/unused:bytes"),
			)
		})

	// go_memstats_heap_released_bytes
	r.NewGauge("go_memstats_heap_released_bytes",
		"Number of heap bytes released to the OS.").
		SetFunc(func() float64 {
			return float64(readUint64("/memory/classes/heap/released:bytes"))
		})

	// go_memstats_sys_bytes: total bytes obtained from the OS.
	r.NewGauge("go_memstats_sys_bytes", "Total bytes obtained from the OS.").
		SetFunc(func() float64 {
			return float64(readUint64("/memory/classes/total:bytes"))
		})

	// go_memstats_stack_inuse_bytes
	r.NewGauge("go_memstats_stack_inuse_bytes",
		"Number of stack bytes that are in use.").
		SetFunc(func() float64 {
			return float64(readUint64("/memory/classes/heap/stacks:bytes"))
		})

	// go_gc_runs_total: count of completed GC cycles since process start.
	r.NewGauge("go_gc_runs_total", "Total number of completed GC cycles.").
		SetFunc(func() float64 {
			return float64(readUint64("/gc/cycles/total:gc-cycles"))
		})

	// go_info exposes the Go runtime version.
	goInfo := r.NewGauge("go_info", "Information about the Go runtime.", "version")
	goInfo.SetFunc(func() float64 { return 1 }, runtime.Version())

	// process_start_time_seconds: unix epoch seconds at start.
	startTime := float64(time.Now().Unix())
	r.NewGauge("process_start_time_seconds",
		"Start time of the process since unix epoch in seconds.").
		SetFunc(func() float64 { return startTime })

	// process_resident_memory_bytes: approximation from runtime/metrics.
	// total - released gives a reasonable RSS proxy on platforms where
	// runtime/metrics doesn't expose it directly.
	r.NewGauge("process_resident_memory_bytes",
		"Resident memory size in bytes (approximation).").
		SetFunc(func() float64 {
			total := readUint64("/memory/classes/total:bytes")
			released := readUint64("/memory/classes/heap/released:bytes")
			if released > total {
				return float64(total)
			}
			return float64(total - released)
		})

	// process_open_fds: number of open file descriptors. Best-effort via
	// /dev/fd, cached to avoid walking the directory every scrape.
	r.NewGauge("process_open_fds",
		"Number of open file descriptors (approximation).").
		SetFunc(openFDs)
}

// readUint64 reads a single runtime/metrics Uint64 sample, returning 0 on
// unknown metric or kind mismatch.
func readUint64(name string) uint64 {
	var sample [1]metrics.Sample
	sample[0].Name = name
	metrics.Read(sample[:])
	if sample[0].Value.Kind() == metrics.KindUint64 {
		return sample[0].Value.Uint64()
	}
	return 0
}

// openFDs caches the result of walking /dev/fd so scrapes stay O(1) between
// refreshes. Safe to call concurrently.
var (
	cachedOpenFDs  atomic.Int64
	lastFDCheck    atomic.Int64 // unix nanos
	minFDScrapeGap = int64(5 * time.Second)
)

func openFDs() float64 {
	now := time.Now().UnixNano()
	last := lastFDCheck.Load()
	if last != 0 && now-last < minFDScrapeGap {
		return float64(cachedOpenFDs.Load())
	}
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return 0
	}
	count := int64(len(entries))
	cachedOpenFDs.Store(count)
	lastFDCheck.Store(now)
	return float64(count)
}
