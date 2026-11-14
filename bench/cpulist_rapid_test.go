package bench_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// TestParseCPUListMatchesModel draws disjoint entries, writes them as a CPU
// list and expands them in the test. ParseCPUList must return the expansion.
func TestParseCPUListMatchesModel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		starts := rapid.SliceOfNDistinct(rapid.IntRange(0, bench.MaxCPU/8), 0, 16, rapid.ID[int]).
			Draw(rt, "starts")
		var entries []string
		want := []int{}
		for i, start := range starts {
			first := start * 8
			last := first + rapid.IntRange(0, 7).Draw(rt, "width"+strconv.Itoa(i))
			if first == last {
				entries = append(entries, strconv.Itoa(first))
			} else {
				entries = append(entries, strconv.Itoa(first)+"-"+strconv.Itoa(last))
			}
			for cpu := first; cpu <= last; cpu++ {
				want = append(want, cpu)
			}
		}

		got, err := bench.ParseCPUList(strings.Join(entries, ","))
		require.NoError(rt, err)
		require.Equal(rt, want, got)
	})
}

// TestFormatCPUListRoundTrips formats a list of distinct CPUs in any order.
// ParseCPUList must return the same list.
func TestFormatCPUListRoundTrips(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cpus := rapid.SliceOfNDistinct(rapid.IntRange(0, bench.MaxCPU), 1, 32, rapid.ID[int]).
			Draw(rt, "cpus")

		got, err := bench.ParseCPUList(bench.FormatCPUList(cpus))
		require.NoError(rt, err)
		require.Equal(rt, cpus, got)
	})
}
