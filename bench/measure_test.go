package bench_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

var allocationSink []*[64]byte

func TestMeasureRunsWarmupThenMeasured(t *testing.T) {
	cfg := bench.Config{Messages: 1000, WarmupMessages: 10}
	var calls []int64
	warmup := func(count int64) error {
		calls = append(calls, -count)
		return nil
	}
	measured := func(count int64) error {
		calls = append(calls, count)
		allocationSink = make([]*[64]byte, count)
		for i := range allocationSink {
			allocationSink[i] = new([64]byte)
		}
		runtime.GC()
		return nil
	}

	measurement, err := bench.Measure(cfg, warmup, measured)
	require.NoError(t, err)
	require.Equal(t, []int64{-10, 1000}, calls)
	require.Positive(t, measurement.ElapsedNs)
	require.GreaterOrEqual(t, measurement.GoRuntime.Allocations, uint64(1001))
	require.GreaterOrEqual(t, measurement.GoRuntime.AllocationsPerMessage, 1.0)
	require.GreaterOrEqual(t, measurement.GoRuntime.GCCycles, uint64(1))
	require.Positive(t, measurement.GoRuntime.GCPauseNs)
}

func TestMeasureRecordsGOMAXPROCS(t *testing.T) {
	cfg := bench.Config{Messages: 1}
	measurement, err := bench.Measure(cfg, nil, func(int64) error { return nil })
	require.NoError(t, err)
	require.Equal(t, runtime.GOMAXPROCS(0), measurement.GoRuntime.GOMAXPROCS)
}

func TestMeasureCountsAllocationsBelowOneSpan(t *testing.T) {
	cfg := bench.Config{Messages: 10}
	measured := func(count int64) error {
		allocationSink = make([]*[64]byte, 0, count)
		for range count {
			allocationSink = append(allocationSink, new([64]byte))
		}
		return nil
	}

	measurement, err := bench.Measure(cfg, nil, measured)
	require.NoError(t, err)
	require.GreaterOrEqual(t, measurement.GoRuntime.Allocations, uint64(10))
}

func TestMeasureEmptyPassAllocatesNothing(t *testing.T) {
	cfg := bench.Config{Messages: 1}
	measurement, err := bench.Measure(cfg, nil, func(int64) error { return nil })
	require.NoError(t, err)
	require.Zero(t, measurement.GoRuntime.Allocations)
}

func TestMeasureSkipsZeroWarmup(t *testing.T) {
	cfg := bench.Config{Messages: 1}
	warmup := func(int64) error { return errors.New("warmup ran") }
	measured := func(int64) error { return nil }

	_, err := bench.Measure(cfg, warmup, measured)
	require.NoError(t, err)
}

func TestMeasureReturnsPassError(t *testing.T) {
	cfg := bench.Config{Messages: 1, WarmupMessages: 1}
	failing := func(int64) error { return errors.New("offer failed") }
	passing := func(int64) error { return nil }

	_, err := bench.Measure(cfg, failing, passing)
	require.ErrorContains(t, err, "warmup: offer failed")
	_, err = bench.Measure(cfg, passing, failing)
	require.ErrorContains(t, err, "measured: offer failed")
}

func TestNewThroughput(t *testing.T) {
	require.Equal(t, bench.Throughput{
		Messages:           2_000_000,
		Bytes:              64_000_000,
		ElapsedNs:          500_000_000,
		MessagesPerSecond:  4_000_000,
		MegabytesPerSecond: 128,
	}, bench.NewThroughput(2_000_000, 32, 500_000_000))
	require.Equal(t, bench.Throughput{Messages: 1, Bytes: 32}, bench.NewThroughput(1, 32, 0))
}
