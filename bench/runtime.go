package bench

import "runtime"

// GoRuntime holds the GOMAXPROCS setting and the Go runtime counters for
// the measured section.
//
// Measure reads the counters with runtime.ReadMemStats. ReadMemStats stops
// the world. ReadMemStats flushes the per-P allocation caches first, so
// Allocations counts every object.
type GoRuntime struct {
	GOMAXPROCS int `json:"gomaxprocs"`

	// Allocations counts heap objects, tiny objects included.
	Allocations           uint64  `json:"allocations"`
	AllocationsPerMessage float64 `json:"allocations_per_message"`
	GCCycles              uint64  `json:"gc_cycles"`

	// GCPauseNs sums the stop-the-world GC pauses.
	GCPauseNs uint64 `json:"gc_pause_ns"`
}

// JVMRuntime holds the GarbageCollectorMXBean totals for the measured section.
// The totals count GC pauses only. The Java client writes JVMRuntime. The Go
// side reads it back.
type JVMRuntime struct {
	GCCount  int64 `json:"gc_count"`
	GCTimeMs int64 `json:"gc_time_ms"`
}

// goRuntimeDelta returns the counters between before and after.
func goRuntimeDelta(before, after *runtime.MemStats, messages int64) GoRuntime {
	allocations := after.Mallocs - before.Mallocs
	stats := GoRuntime{
		Allocations: allocations,
		GCCycles:    uint64(after.NumGC - before.NumGC),
		GCPauseNs:   after.PauseTotalNs - before.PauseTotalNs,
	}
	if messages > 0 {
		stats.AllocationsPerMessage = float64(allocations) / float64(messages)
	}
	return stats
}
