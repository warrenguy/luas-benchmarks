package bench_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

func TestHotYieldTick(t *testing.T) {
	const interval = bench.DefaultYieldIntervalNs
	tests := []struct {
		name  string
		cfg   bench.Config
		index int
		ticks []int64
		want  []bool
	}{
		{
			name:  "locked threads never yield",
			cfg:   bench.Config{CPUs: []int{4, 5}, YieldIntervalNs: interval},
			index: bench.PingThread,
			ticks: []int64{0, interval, 10 * interval},
			want:  []bool{false, false, false},
		},
		{
			name:  "unpinned threads never yield",
			cfg:   bench.Config{UnlockThreads: true, YieldIntervalNs: interval},
			index: bench.PingThread,
			ticks: []int64{0, interval, 10 * interval},
			want:  []bool{false, false, false},
		},
		{
			name:  "zero interval never yields",
			cfg:   bench.Config{CPUs: []int{4, 5}, UnlockThreads: true},
			index: bench.PingThread,
			ticks: []int64{0, interval, 10 * interval},
			want:  []bool{false, false, false},
		},
		{
			name:  "first thread yields at the start of each interval",
			cfg:   bench.Config{CPUs: []int{4, 5}, UnlockThreads: true, YieldIntervalNs: interval},
			index: bench.PingThread,
			ticks: []int64{
				0,
				1,
				interval - 1,
				interval,
				interval + 1,
				4*interval + 7,
				5*interval - 1,
				5 * interval,
			},
			want: []bool{true, false, false, true, false, true, false, true},
		},
		{
			name:  "second thread yields half an interval later",
			cfg:   bench.Config{CPUs: []int{4, 5}, UnlockThreads: true, YieldIntervalNs: interval},
			index: bench.PongThread,
			ticks: []int64{
				0,
				interval/2 - 1,
				interval / 2,
				interval,
				3*interval/2 - 1,
				3 * interval / 2,
				9 * interval,
				19*interval/2 - 1,
				19 * interval / 2,
			},
			want: []bool{false, false, true, false, false, true, true, false, true},
		},
		{
			name:  "single thread yields once per interval",
			cfg:   bench.Config{CPUs: []int{4}, UnlockThreads: true, YieldIntervalNs: interval},
			index: bench.PingThread,
			ticks: []int64{interval / 2, interval - 1, interval},
			want:  []bool{true, false, true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yield := tt.cfg.HotYield(tt.index)
			got := make([]bool, len(tt.ticks))
			for i, nowNs := range tt.ticks {
				got[i] = yield.Tick(nowNs)
			}
			require.Equal(t, tt.want, got)
		})
	}
}
