package bench_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// TestHotYieldMatchesModel draws a hot thread and rising tick times. The
// model yields when a yield slot of the thread falls after the previous
// yield and at or before the tick. Slots of hot thread i of n sit at
// i*YieldIntervalNs/n within each interval.
func TestHotYieldMatchesModel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		threads := rapid.IntRange(1, 8).Draw(rt, "threads")
		index := rapid.IntRange(0, threads-1).Draw(rt, "index")
		intervalNs := rapid.Int64Range(1, 2*bench.DefaultYieldIntervalNs).Draw(rt, "intervalNs")
		cfg := bench.Config{CPUs: make([]int, threads), UnlockThreads: true, YieldIntervalNs: intervalNs}
		phaseNs := intervalNs * int64(index) / int64(threads)
		steps := rapid.SliceOfN(rapid.Int64Range(0, 3*intervalNs), 1, 64).Draw(rt, "steps")

		yield := cfg.HotYield(index)
		slot := func(nowNs int64) int64 {
			return (nowNs - phaseNs) / intervalNs
		}
		var nowNs int64
		lastSlot := int64(-1)
		for i, step := range steps {
			nowNs += step
			want := nowNs >= phaseNs && slot(nowNs) > lastSlot
			require.Equal(rt, want, yield.Tick(nowNs), "tick %d at %d ns", i, nowNs)
			if want {
				lastSlot = slot(nowNs)
			}
		}
	})
}

// TestHotYieldSlotsStayApart checks that the yield slots of two hot threads
// of one process sit at least YieldIntervalNs/n apart, less the 1 us step
// of firstYield.
func TestHotYieldSlotsStayApart(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		threads := rapid.IntRange(2, 8).Draw(rt, "threads")
		first := rapid.IntRange(0, threads-1).Draw(rt, "first")
		second := rapid.IntRange(0, threads-1).
			Filter(func(i int) bool { return i != first }).
			Draw(rt, "second")
		cfg := bench.Config{
			CPUs:            make([]int, threads),
			UnlockThreads:   true,
			YieldIntervalNs: bench.DefaultYieldIntervalNs,
		}

		firstSlot := firstYield(rt, cfg, first)
		secondSlot := firstYield(rt, cfg, second)
		gap := (secondSlot - firstSlot + bench.DefaultYieldIntervalNs) % bench.DefaultYieldIntervalNs
		gap = min(gap, bench.DefaultYieldIntervalNs-gap)
		require.GreaterOrEqual(rt, gap, bench.DefaultYieldIntervalNs/int64(threads)-yieldSearchStepNs)
	})
}

// yieldSearchStepNs is the tick step of firstYield.
const yieldSearchStepNs = 1000

// firstYield returns the first tick time, in steps of yieldSearchStepNs, at
// which hot thread index yields.
func firstYield(rt *rapid.T, cfg bench.Config, index int) int64 {
	rt.Helper()
	yield := cfg.HotYield(index)
	for nowNs := int64(0); nowNs <= cfg.YieldIntervalNs; nowNs += yieldSearchStepNs {
		if yield.Tick(nowNs) {
			return nowNs
		}
	}
	rt.Fatalf("hot thread %d did not yield within one interval", index)
	return 0
}
