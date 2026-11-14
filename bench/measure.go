package bench

import (
	"fmt"
	"runtime"
)

// Pass runs a scenario over count messages.
type Pass func(count int64) error

// Measurement holds the clock and runtime counters of one measured section.
type Measurement struct {
	ElapsedNs int64
	GoRuntime GoRuntime
}

// Measure runs warmup over cfg.WarmupMessages. Measure then reads the runtime
// counters and starts the clock. Measure runs measured over cfg.Messages.
// Measure stops the clock before it reads the runtime counters again.
//
// Measure skips warmup when cfg.WarmupMessages is 0. Measure itself does
// not allocate between the two counter reads. Measure also records
// GOMAXPROCS.
func Measure(cfg Config, warmup, measured Pass) (Measurement, error) {
	if cfg.WarmupMessages > 0 {
		if err := warmup(cfg.WarmupMessages); err != nil {
			return Measurement{}, fmt.Errorf("warmup: %w", err)
		}
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := NanoTime()
	err := measured(cfg.Messages)
	elapsed := NanoTime() - start
	runtime.ReadMemStats(&after)
	if err != nil {
		return Measurement{}, fmt.Errorf("measured: %w", err)
	}
	goRuntime := goRuntimeDelta(&before, &after, cfg.Messages)
	goRuntime.GOMAXPROCS = runtime.GOMAXPROCS(0)
	return Measurement{ElapsedNs: elapsed, GoRuntime: goRuntime}, nil
}

// Throughput holds the rate of one measured section. Java writes the same
// JSON fields.
type Throughput struct {
	Messages  int64 `json:"messages"`
	Bytes     int64 `json:"bytes"`
	ElapsedNs int64 `json:"elapsed_ns"`

	MessagesPerSecond float64 `json:"messages_per_second"`

	// MegabytesPerSecond counts 10^6 bytes per megabyte.
	MegabytesPerSecond float64 `json:"megabytes_per_second"`
}

// NewThroughput returns the rate of messages of messageLength bytes over
// elapsedNs. The rates are 0 when elapsedNs is not positive.
func NewThroughput(messages int64, messageLength int, elapsedNs int64) Throughput {
	bytes := messages * int64(messageLength)
	t := Throughput{Messages: messages, Bytes: bytes, ElapsedNs: elapsedNs}
	if elapsedNs > 0 {
		seconds := float64(elapsedNs) / 1e9
		t.MessagesPerSecond = float64(messages) / seconds
		t.MegabytesPerSecond = float64(bytes) / 1e6 / seconds
	}
	return t
}
