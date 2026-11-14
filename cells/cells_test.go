package cells

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// writeRun returns a new run directory with an empty run record and one
// result file per entry of results, by file name.
func writeRun(t *testing.T, results map[string]bench.Result) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, bench.WriteJSON(filepath.Join(dir, HostFile), bench.RunRecord{}))
	for name, result := range results {
		require.NoError(t, bench.WriteJSON(filepath.Join(dir, name), result))
	}
	return dir
}

// encodeHistogram returns the encoded histogram of valuesNs.
func encodeHistogram(t *testing.T, valuesNs ...int64) string {
	t.Helper()
	histogram := bench.NewHistogram()
	for _, value := range valuesNs {
		require.NoError(t, histogram.Record(value))
	}
	encoded, err := histogram.Encode()
	require.NoError(t, err)
	return encoded
}

var (
	throughputResult = bench.Result{
		Scenario:   "ipc-throughput",
		Client:     "luas",
		Variant:    "concurrent-offer-32",
		Throughput: &bench.Throughput{Messages: 1},
	}
	fixedRateConfig = bench.Config{Rate: 1000}
)

func TestLoadRunRejects(t *testing.T) {
	tests := []struct {
		name    string
		edit    func(r *bench.Result)
		wantErr string
	}{
		{
			name:    "unknown client",
			edit:    func(r *bench.Result) { r.Client = "cpp" },
			wantErr: `unknown client "cpp"`,
		},
		{
			name:    "throughput and latency",
			edit:    func(r *bench.Result) { r.Latency = &bench.Latency{Count: 1} },
			wantErr: "want exactly one of throughput and latency",
		},
		{
			name:    "neither throughput nor latency",
			edit:    func(r *bench.Result) { r.Throughput = nil },
			wantErr: "want exactly one of throughput and latency",
		},
		{
			name: "fixed_rate without a rate",
			edit: func(r *bench.Result) {
				r.Throughput, r.Latency, r.FixedRate = nil, &bench.Latency{}, &bench.FixedRate{}
			},
			wantErr: "want fixed_rate exactly when the config sets a rate",
		},
		{
			name: "rate without fixed_rate",
			edit: func(r *bench.Result) {
				r.Throughput, r.Latency, r.Config = nil, &bench.Latency{}, fixedRateConfig
			},
			wantErr: "want fixed_rate exactly when the config sets a rate",
		},
		{
			name:    "fixed_rate without latency",
			edit:    func(r *bench.Result) { r.Config, r.FixedRate = fixedRateConfig, &bench.FixedRate{} },
			wantErr: "want latency with fixed_rate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := throughputResult
			tt.edit(&result)
			dir := writeRun(t, map[string]bench.Result{"result-1.json": result})
			_, _, err := LoadRun(dir)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestLoadRunRejectsRunWithoutResult(t *testing.T) {
	_, _, err := LoadRun(writeRun(t, nil))
	require.ErrorContains(t, err, "no result file")
}

func TestLoadRunRejectsRunWithoutRecord(t *testing.T) {
	_, _, err := LoadRun(t.TempDir())
	require.ErrorContains(t, err, "load run record")
}

// TestLoadRunOrdersRuns writes runs 1, 2 and 10 of one variant. LoadRun
// must return them in run order, not name order.
func TestLoadRunOrdersRuns(t *testing.T) {
	results := make(map[string]bench.Result)
	for _, run := range []int64{10, 2, 1} {
		result := throughputResult
		result.Throughput = &bench.Throughput{Messages: run}
		results["ipc-throughput-luas-concurrent-offer-32-"+strconv.FormatInt(run, 10)+".json"] = result
	}
	_, got, err := LoadRun(writeRun(t, results))
	require.NoError(t, err)
	var messages []int64
	for _, result := range got {
		messages = append(messages, result.Throughput.Messages)
	}
	require.Equal(t, []int64{1, 2, 10}, messages)
}

func TestGroupRejects(t *testing.T) {
	pingPong := bench.Result{Scenario: "ipc-ping-pong", Client: "luas", Variant: "concurrent-offer-32"}
	tests := []struct {
		name    string
		results func() []bench.Result
		wantErr string
	}{
		{
			name: "bad histogram",
			results: func() []bench.Result {
				result := pingPong
				result.Latency = &bench.Latency{Histogram: "!"}
				return []bench.Result{result}
			},
			wantErr: "decode histogram",
		},
		{
			name: "ping-pong without a histogram",
			results: func() []bench.Result {
				result := pingPong
				result.Latency = &bench.Latency{Count: 1, P50Ns: 1000}
				return []bench.Result{result}
			},
			wantErr: "ipc-ping-pong concurrent-offer-32: luas: no histogram",
		},
		{
			name: "latency in a throughput row",
			results: func() []bench.Result {
				result := throughputResult
				result.Throughput, result.Latency = nil, &bench.Latency{Histogram: encodeHistogram(t, 1)}
				return []bench.Result{throughputResult, result}
			},
			wantErr: "mixes throughput and latency results",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Group(tt.results())
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestGroupReadsBothHistogramFields groups a ping-pong result, which holds
// its histogram in latency, and a fixed-rate result, which holds it in
// fixed_rate.
func TestGroupReadsBothHistogramFields(t *testing.T) {
	pingPong := bench.Result{
		Scenario: "ipc-ping-pong",
		Client:   "luas",
		Variant:  "concurrent-offer-32",
		Latency:  &bench.Latency{Histogram: encodeHistogram(t, 1000, 2000)},
	}
	fixedRate := bench.Result{
		Scenario: "ipc-fixed-rate",
		Client:   "luas",
		Variant:  "exclusive-try-claim-32-rate-1000",
		Config:   fixedRateConfig,
		Latency:  &bench.Latency{},
		FixedRate: &bench.FixedRate{
			Expected:  3,
			Sent:      3,
			Received:  2,
			Histogram: encodeHistogram(t, 3000),
		},
	}
	rows, err := Group([]bench.Result{fixedRate, pingPong})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	require.Equal(t, Latency, rows[0].Kind)
	pingPongCell := rows[0].Latency["luas"]
	merged, runs := pingPongCell.Percentiles([]float64{50, 100})
	require.Equal(t, []int64{1000, 2000}, merged)
	require.Equal(t, [][]int64{{1000, 2000}}, runs)
	require.False(t, pingPongCell.HasShortfall)

	require.Equal(t, FixedRate, rows[1].Kind)
	fixedRateCell := rows[1].Latency["luas"]
	merged, _ = fixedRateCell.Percentiles([]float64{50})
	require.Equal(t, []int64{3001}, merged)
	require.True(t, fixedRateCell.HasShortfall)
	require.Equal(
		t,
		[3]int64{3, 3, 2},
		[3]int64{fixedRateCell.Expected, fixedRateCell.Sent, fixedRateCell.Received},
	)
}

func TestSummarizeEmpty(t *testing.T) {
	require.Equal(t, Summary{}, Summarize(nil))
}
