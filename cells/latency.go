package cells

import (
	"errors"
	"math"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// LatencyCell holds the round trip histograms of one client in a latency or
// fixed-rate row.
type LatencyCell struct {
	// Merged holds every round trip of every run, as the Aeron load test rig
	// merges them.
	Merged *bench.Histogram

	// Runs holds the histogram of each run, in run order.
	Runs []*bench.Histogram

	// HasShortfall reports whether a fixed-rate run sent fewer messages than
	// its schedule or received fewer replies than it sent.
	HasShortfall bool

	// Expected, Sent and Received sum the message counts of the fixed-rate
	// runs. Zero in a latency row.
	Expected int64
	Sent     int64
	Received int64
}

// newLatencyCell decodes the histogram of each run and merges them into
// one. A fixed-rate result holds its histogram in fixed_rate, other latency
// results in latency.
//
// Returns an error when a histogram is missing or does not decode.
func newLatencyCell(runs []bench.Result) (*LatencyCell, error) {
	cell := &LatencyCell{Merged: bench.NewHistogram()}
	for _, run := range runs {
		encoded := run.Latency.Histogram
		if fixedRate := run.FixedRate; fixedRate != nil {
			encoded = fixedRate.Histogram
			cell.HasShortfall = cell.HasShortfall || fixedRate.HasShortfall()
			cell.Expected += fixedRate.Expected
			cell.Sent += fixedRate.Sent
			cell.Received += fixedRate.Received
		}
		if encoded == "" {
			return nil, errors.New("no histogram")
		}
		histogram, err := bench.DecodeHistogram(encoded)
		if err != nil {
			return nil, err
		}
		cell.Merged.Merge(histogram)
		cell.Runs = append(cell.Runs, histogram)
	}
	return cell, nil
}

// Percentiles returns the value of the merged histogram at each of
// percentiles, and the values of each run, in nanoseconds. A percentile of
// 100 gives the max.
func (c *LatencyCell) Percentiles(percentiles []float64) (merged []int64, runs [][]int64) {
	runs = make([][]int64, len(c.Runs))
	for i, run := range c.Runs {
		runs[i] = run.ValuesAtPercentiles(percentiles)
	}
	return c.Merged.ValuesAtPercentiles(percentiles), runs
}

// CurvePoint holds one point of the latency curve of a cell. Values are in
// nanoseconds.
type CurvePoint struct {
	Percentile float64

	// Inverse is 1/(1-Percentile/100): one round trip in Inverse lies at or
	// above Value. The max takes the round trip count.
	Inverse float64

	// Value is the value of the merged histogram.
	Value int64

	// Lowest and Highest bound the values of the runs.
	Lowest  int64
	Highest int64
}

// Curve returns the latency curve of the cell. The percentile of point k is
// 100*(1-2^(-k/5)), five points per halving of the tail, while the inverse
// stays within the round trip count. The last point is the max.
func (c *LatencyCell) Curve() []CurvePoint {
	count := float64(c.Merged.Latency().Count)
	var grid []float64
	for k := 0; ; k++ {
		percentile := 100 * (1 - math.Pow(2, -float64(k)/5))
		if 1/(1-percentile/100) > count {
			break
		}
		// The histogram reads percentiles above 0.
		grid = append(grid, max(percentile, 1e-6))
	}
	grid = append(grid, 100)
	merged, runs := c.Percentiles(grid)
	points := make([]CurvePoint, len(grid))
	for i, percentile := range grid {
		inverse := count
		if percentile < 100 {
			inverse = 1 / (1 - percentile/100)
		}
		point := CurvePoint{Percentile: percentile, Inverse: inverse, Value: merged[i]}
		for j, run := range runs {
			if j == 0 || run[i] < point.Lowest {
				point.Lowest = run[i]
			}
			if j == 0 || run[i] > point.Highest {
				point.Highest = run[i]
			}
		}
		points[i] = point
	}
	return points
}
