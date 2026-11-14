package cells

import (
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// drawnCell holds the results of one drawn cell and the round trips of each
// run.
type drawnCell struct {
	results    []bench.Result
	runSamples [][]int64
	pooled     *bench.Histogram
}

// drawCell draws the round trips of up to 5 runs. isFixedRate puts each
// histogram in fixed_rate, with drawn message counts. Otherwise latency
// holds it.
func drawCell(rt *rapid.T, isFixedRate bool) drawnCell {
	rt.Helper()
	cell := drawnCell{pooled: bench.NewHistogram()}
	for range rapid.IntRange(1, 5).Draw(rt, "runs") {
		run := bench.NewHistogram()
		received := rapid.Int64Range(1, 64).Draw(rt, "received")
		var samples []int64
		for range received {
			// The exponent spreads the values over the whole histogram range.
			exponent := rapid.IntRange(0, 41).Draw(rt, "exponent")
			value := rapid.Int64Range(0, 1<<exponent).Draw(rt, "value")
			require.NoError(rt, run.Record(value))
			require.NoError(rt, cell.pooled.Record(value))
			samples = append(samples, value)
		}
		encoded, err := run.Encode()
		require.NoError(rt, err)
		result := bench.Result{Latency: &bench.Latency{Histogram: encoded}}
		if isFixedRate {
			sent := received + rapid.Int64Range(0, 1).Draw(rt, "lost")
			expected := sent + rapid.Int64Range(0, 1).Draw(rt, "unsent")
			result.Latency = &bench.Latency{}
			result.FixedRate = &bench.FixedRate{
				Expected:  expected,
				Sent:      sent,
				Received:  received,
				Histogram: encoded,
			}
		}
		cell.results = append(cell.results, result)
		cell.runSamples = append(cell.runSamples, samples)
	}
	return cell
}

// TestNewLatencyCellMatchesPooledHistogram draws the round trips of up to 5
// runs. The merged values must equal the values of one histogram of every
// round trip. The values of each run must equal the values of its own round
// trips. The counts must sum over the runs.
func TestNewLatencyCellMatchesPooledHistogram(t *testing.T) {
	percentiles := []float64{50, 90, 99, 99.9, 99.99, 99.999, 100}
	rapid.Check(t, func(rt *rapid.T) {
		isFixedRate := rapid.Bool().Draw(rt, "fixed rate")
		drawn := drawCell(rt, isFixedRate)
		var wantRuns [][]int64
		var expected, sent, received int64
		hasShortfall := false
		for _, result := range drawn.results {
			histogram, err := bench.DecodeHistogram(encodedHistogram(result))
			require.NoError(rt, err)
			wantRuns = append(wantRuns, histogram.ValuesAtPercentiles(percentiles))
			if fixedRate := result.FixedRate; fixedRate != nil {
				expected += fixedRate.Expected
				sent += fixedRate.Sent
				received += fixedRate.Received
				hasShortfall = hasShortfall || fixedRate.HasShortfall()
			}
		}

		cell, err := newLatencyCell(drawn.results)
		require.NoError(rt, err)
		merged, runs := cell.Percentiles(percentiles)
		require.Equal(rt, drawn.pooled.ValuesAtPercentiles(percentiles), merged)
		require.Equal(rt, wantRuns, runs)
		require.Equal(rt, hasShortfall, cell.HasShortfall)
		require.Equal(
			rt,
			[3]int64{expected, sent, received},
			[3]int64{cell.Expected, cell.Sent, cell.Received},
		)
	})
}

// encodedHistogram returns the encoded histogram of result.
func encodedHistogram(result bench.Result) string {
	if result.FixedRate != nil {
		return result.FixedRate.Histogram
	}
	return result.Latency.Histogram
}

// TestCurveMatchesPooledSamples draws the round trips of up to 5 runs. Each
// curve point must hold the round trip of its rank among the pooled round
// trips, within the histogram precision. Lowest and Highest must bound the
// rank values of the runs. The grid must rise to the round trip count and
// end at the max.
func TestCurveMatchesPooledSamples(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		drawn := drawCell(rt, rapid.Bool().Draw(rt, "fixed rate"))
		var pooled []int64
		for _, samples := range drawn.runSamples {
			pooled = append(pooled, samples...)
		}
		count := float64(len(pooled))

		cell, err := newLatencyCell(drawn.results)
		require.NoError(rt, err)
		points := cell.Curve()
		require.GreaterOrEqual(rt, len(points), 2)

		last := points[len(points)-1]
		require.InDelta(rt, 100.0, last.Percentile, 0)
		require.InDelta(rt, count, last.Inverse, 0)
		grid := points[:len(points)-1]
		for k, point := range grid {
			require.InDelta(rt, max(100*(1-math.Pow(2, -float64(k)/5)), 1e-6), point.Percentile, 1e-9)
			// The grid rule reads the percentile before the clamp to 1e-6.
			require.LessOrEqual(rt, math.Pow(2, float64(k)/5), count*(1+1e-9))
		}
		require.Greater(rt, math.Pow(2, float64(len(grid))/5), count*(1-1e-9), "grid stops early")

		for _, point := range points {
			requireNear(rt, rankValue(pooled, point.Percentile), point.Value)
			lowest, highest := int64(math.MaxInt64), int64(math.MinInt64)
			for _, samples := range drawn.runSamples {
				value := rankValue(samples, point.Percentile)
				lowest, highest = min(lowest, value), max(highest, value)
			}
			requireNear(rt, lowest, point.Lowest)
			requireNear(rt, highest, point.Highest)
			require.LessOrEqual(rt, point.Lowest, point.Value)
			require.LessOrEqual(rt, point.Value, point.Highest)
		}
	})
}

// rankValue returns the sample at the rank of percentile by the Java
// HdrHistogram rule: the percentile just below percentile, times the count,
// rounded up, and at least 1.
func rankValue(samples []int64, percentile float64) int64 {
	sorted := slices.Sorted(slices.Values(samples))
	requested := math.Min(math.Max(math.Nextafter(percentile, math.Inf(-1)), 0), 100)
	rank := max(int64(math.Ceil(requested*float64(len(sorted))/100)), 1)
	return sorted[rank-1]
}

// requireNear requires got to hold sample at 3 significant digits: the
// highest value of the histogram bucket of sample.
func requireNear(rt *rapid.T, sample, got int64) {
	rt.Helper()
	require.GreaterOrEqual(rt, got, sample)
	require.LessOrEqual(rt, got, sample+sample/1024+1)
}
