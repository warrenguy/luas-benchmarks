package cells

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestSummarizeMatchesModel draws the per-run values of one cell. Summarize
// must match a model that counts the values below and above each candidate.
func TestSummarizeMatchesModel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		values := rapid.SliceOfN(rapid.Float64Range(0, 1e12), 1, 16).Draw(rt, "values")
		original := slices.Clone(values)

		got := Summarize(values)
		require.Equal(rt, original, values, "Summarize changed its input")
		require.Equal(rt, len(values), got.Runs)
		require.InDelta(rt, modelMin(values), got.Min, 0)
		require.InDelta(rt, modelMax(values), got.Max, 0)
		require.InDelta(rt, modelMedian(rt, values), got.Median, 0)
		require.LessOrEqual(rt, got.Min, got.Median)
		require.LessOrEqual(rt, got.Median, got.Max)
	})
}

func modelMin(values []float64) float64 {
	low := values[0]
	for _, value := range values {
		if value < low {
			low = value
		}
	}
	return low
}

func modelMax(values []float64) float64 {
	high := values[0]
	for _, value := range values {
		if value > high {
			high = value
		}
	}
	return high
}

// modelMedian returns the value with rank len/2 for an odd count. It returns
// the mean of the values with ranks len/2-1 and len/2 for an even count. A
// value holds rank k when at most k values lie below it and more than k lie
// at or below it.
func modelMedian(rt *rapid.T, values []float64) float64 {
	rt.Helper()
	rank := func(k int) float64 {
		for _, candidate := range values {
			below, atOrBelow := 0, 0
			for _, value := range values {
				if value < candidate {
					below++
				}
				if value <= candidate {
					atOrBelow++
				}
			}
			if below <= k && k < atOrBelow {
				return candidate
			}
		}
		rt.Fatalf("no value holds rank %d", k)
		return 0
	}
	n := len(values)
	if n%2 == 1 {
		return rank(n / 2)
	}
	return (rank(n/2-1) + rank(n/2)) / 2
}
