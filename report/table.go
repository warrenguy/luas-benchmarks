package main

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/warrenguy/luas-benchmarks/bench"
	"github.com/warrenguy/luas-benchmarks/cells"
)

// metric names one table row of a scenario variant: a statistic such as
// "p50", and a unit. value reads the row value of a throughput metric from
// one result. A latency metric reads percentile from the histograms of a
// latency cell instead.
type metric struct {
	statistic  string
	unit       string
	value      func(bench.Result) float64
	percentile float64
}

// name returns the statistic and the unit, for example "p50 us".
func (m metric) name() string {
	return strings.TrimSpace(m.statistic + " " + m.unit)
}

var throughputMetrics = []metric{
	{
		unit:  "Mmsg/s",
		value: func(r bench.Result) float64 { return r.Throughput.MessagesPerSecond / 1e6 },
	},
}

// latencyPercentiles lists the latency rows of the comparison tables.
var latencyPercentiles = []metric{
	{statistic: "p50", unit: "us", percentile: 50},
	{statistic: "p90", unit: "us", percentile: 90},
	{statistic: "p99", unit: "us", percentile: 99},
	{statistic: "p99.9", unit: "us", percentile: 99.9},
	{statistic: "p99.99", unit: "us", percentile: 99.99},
	{statistic: "p99.999", unit: "us", percentile: 99.999},
}

// latencyMetrics adds the max to latencyPercentiles. Only the full table
// shows it.
var latencyMetrics = append(slices.Clone(latencyPercentiles),
	metric{statistic: "max", unit: "us", percentile: 100},
)

func microseconds(ns int64) float64 {
	return float64(ns) / 1e3
}

// metrics returns the rows of the full table of a row of kind.
func metrics(kind cells.Kind) []metric {
	if kind.IsLatency() {
		return latencyMetrics
	}
	return throughputMetrics
}

// comparisonMetrics returns the lines of the comparison table of a row of
// kind.
func comparisonMetrics(kind cells.Kind) []metric {
	if kind.IsLatency() {
		return latencyPercentiles
	}
	return throughputMetrics
}

// variantRow holds every run of one scenario variant, and the latency
// values of each client of a latency or fixed-rate row.
type variantRow struct {
	*cells.Row

	// percentiles holds the latency values of each client, by client name.
	// Nil in a throughput row.
	percentiles map[string]*latencyValues
}

// latencyValues holds the value of each metric in latencyMetrics of one
// latency cell in microseconds, by percentile.
type latencyValues struct {
	// merged holds the values of the merged histogram of every run.
	merged map[float64]float64

	// runs holds the values of the histogram of each run.
	runs []map[float64]float64
}

// newLatencyValues reads the value of each metric in latencyMetrics from
// cell.
func newLatencyValues(cell *cells.LatencyCell) *latencyValues {
	percentiles := make([]float64, len(latencyMetrics))
	for i, m := range latencyMetrics {
		percentiles[i] = m.percentile
	}
	merged, runs := cell.Percentiles(percentiles)
	values := &latencyValues{merged: byPercentile(percentiles, merged)}
	for _, run := range runs {
		values.runs = append(values.runs, byPercentile(percentiles, run))
	}
	return values
}

// byPercentile maps each of percentiles to its value in microseconds.
func byPercentile(percentiles []float64, valuesNs []int64) map[float64]float64 {
	values := make(map[float64]float64, len(percentiles))
	for i, percentile := range percentiles {
		values[percentile] = microseconds(valuesNs[i])
	}
	return values
}

// cellValue returns the value of metric m in the cell of client name, and
// the summary of the per-run values. In a throughput row the value is the
// median of the per-run values. In a latency or fixed-rate row it is the
// value of the merged histogram.
func (r *variantRow) cellValue(name string, m metric) (float64, cells.Summary) {
	if !r.Kind.IsLatency() {
		s := summarizeMetric(r.Runs[name], m)
		return s.Median, s
	}
	cell := r.percentiles[name]
	values := make([]float64, len(cell.runs))
	for i, run := range cell.runs {
		values[i] = run[m.percentile]
	}
	return cell.merged[m.percentile], cells.Summarize(values)
}

// mark returns " !" when a fixed-rate run of client name missed its send
// rate or lost replies. Returns "" otherwise.
func (r *variantRow) mark(name string) string {
	if cell := r.Latency[name]; cell != nil && cell.HasShortfall {
		return " !"
	}
	return ""
}

// groupRows groups results into one row per scenario and variant, in table
// order, and reads the latency values of each latency cell.
//
// Returns an error when the results of one row differ in kind, or a
// histogram is missing or does not decode.
func groupRows(results []bench.Result) ([]*variantRow, error) {
	grouped, err := cells.Group(results)
	if err != nil {
		return nil, err
	}
	rows := make([]*variantRow, len(grouped))
	for i, row := range grouped {
		rows[i] = &variantRow{Row: row}
		if row.Latency == nil {
			continue
		}
		rows[i].percentiles = make(map[string]*latencyValues, len(row.Latency))
		for name, cell := range row.Latency {
			rows[i].percentiles[name] = newLatencyValues(cell)
		}
	}
	return rows, nil
}

// scenarioRuns returns the highest run count of any client in any row of
// each scenario, by scenario name.
func scenarioRuns(rows []*variantRow) map[string]int {
	most := make(map[string]int)
	for _, row := range rows {
		for _, runs := range row.Runs {
			most[row.Scenario] = max(most[row.Scenario], len(runs))
		}
	}
	return most
}

// formatRuns returns the run count of every scenario in expectedRuns. One
// count for every scenario prints alone. Otherwise each count names its
// scenarios, in scenario order.
func formatRuns(expectedRuns map[string]int) string {
	scenarios := slices.SortedFunc(maps.Keys(expectedRuns), func(a, b string) int {
		return cmp.Or(cmp.Compare(cells.ScenarioRank(a), cells.ScenarioRank(b)), cmp.Compare(a, b))
	})
	var counts []int
	byCount := make(map[int][]string)
	for _, scenario := range scenarios {
		count := expectedRuns[scenario]
		if _, ok := byCount[count]; !ok {
			counts = append(counts, count)
		}
		byCount[count] = append(byCount[count], scenario)
	}
	switch len(counts) {
	case 0:
		return "0"
	case 1:
		return strconv.Itoa(counts[0])
	}
	groups := make([]string, len(counts))
	for i, count := range counts {
		groups[i] = strconv.Itoa(count) + " in " + strings.Join(byCount[count], ", ")
	}
	return strings.Join(groups, "; ")
}

// writeTable writes one markdown table line per row and metric. A cell with
// fewer runs than expectedRuns holds for its scenario names its run count.
func writeTable(b *strings.Builder, rows []*variantRow, expectedRuns map[string]int) {
	b.WriteString("| Scenario | Variant | Metric |")
	for _, c := range cells.Clients {
		b.WriteString(" " + c.Title + " |")
	}
	b.WriteString("\n|---|---|---|" + strings.Repeat("---|", len(cells.Clients)) + "\n")
	for _, row := range rows {
		for _, m := range metrics(row.Kind) {
			fmt.Fprintf(b, "| %s | %s | %s |", row.Scenario, row.Variant, m.name())
			for _, c := range cells.Clients {
				b.WriteString(" " + row.formatCell(c.Name, m, expectedRuns[row.Scenario]) + " |")
			}
			b.WriteByte('\n')
		}
	}
}

// formatCell returns the cell value of metric m for client name, then the
// min-max spread over its runs. A cell with fewer than expectedRuns runs
// names its run count. A fixed-rate cell with a shortfall ends in "!".
// Returns "-" when the client has no run.
func (r *variantRow) formatCell(name string, m metric, expectedRuns int) string {
	if len(r.Runs[name]) == 0 {
		return "-"
	}
	value, s := r.cellValue(name, m)
	text := fmt.Sprintf("%s (%s-%s)", formatValue(value), formatValue(s.Min), formatValue(s.Max))
	if s.Runs < expectedRuns {
		text += " n=" + strconv.Itoa(s.Runs)
	}
	return text + r.mark(name)
}

// summarizeMetric returns the summary of metric m over runs.
func summarizeMetric(runs []bench.Result, m metric) cells.Summary {
	values := make([]float64, len(runs))
	for i, run := range runs {
		values[i] = m.value(run)
	}
	return cells.Summarize(values)
}

// formatValue formats v with 3 significant digits. Values of 100 and more
// keep every integer digit.
func formatValue(v float64) string {
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	// The exponent of the 3-digit scientific form already reflects the
	// rounding: 9.996 becomes 1.00e+01.
	scientific := strconv.FormatFloat(v, 'e', 2, 64)
	exponent, _ := strconv.Atoi(scientific[strings.IndexByte(scientific, 'e')+1:])
	return strconv.FormatFloat(v, 'f', max(0, 2-exponent), 64)
}
