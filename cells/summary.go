package cells

import "slices"

// Summary holds the median, min and max of the runs of one cell.
type Summary struct {
	Runs   int
	Median float64
	Min    float64
	Max    float64
}

// Summarize returns the summary of values, one value per run. The median of
// an even run count is the mean of the two middle values. Summarize does not
// change values.
//
// Returns the zero summary when values is empty.
func Summarize(values []float64) Summary {
	if len(values) == 0 {
		return Summary{}
	}
	sorted := slices.Sorted(slices.Values(values))
	middle := len(sorted) / 2
	median := sorted[middle]
	if len(sorted)%2 == 0 {
		median = (sorted[middle-1] + sorted[middle]) / 2
	}
	return Summary{
		Runs:   len(sorted),
		Median: median,
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
	}
}
