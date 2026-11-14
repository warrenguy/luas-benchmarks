package bench_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// fixtureValues matches HistogramFixture.VALUES in java/bench.
const fixtureValues = 10_789

// javaFixture holds what java/bench/HistogramFixture.java prints: the
// percentiles of the fixed input and the encoded histogram.
type javaFixture struct {
	Latency   bench.Latency `json:"latency"`
	Histogram string        `json:"histogram"`
}

// readJavaFixture reads testdata/histogram-java.json. Regenerate it with
// make histogram-fixture after a change to the input or to the Java
// HdrHistogram pin.
func readJavaFixture(t *testing.T) javaFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/histogram-java.json")
	require.NoError(t, err)
	var fixture javaFixture
	require.NoError(t, json.Unmarshal(data, &fixture))
	return fixture
}

// fixtureHistogram records the fixed input of
// java/bench/HistogramFixture.java.
func fixtureHistogram(t *testing.T) *bench.Histogram {
	t.Helper()
	histogram := bench.NewHistogram()
	state := uint64(1)
	for range fixtureValues {
		state = state*6364136223846793005 + 1442695040888963407
		value := (100 + int64((state>>33)%10_000)) << ((state >> 60) % 12)
		require.NoError(t, histogram.Record(value))
	}
	return histogram
}

// TestHistogramMatchesJavaPercentiles records the same fixed input as
// java/bench/HistogramFixture.java. The test expects the percentiles that the
// Java HdrHistogram computed for that input.
func TestHistogramMatchesJavaPercentiles(t *testing.T) {
	require.Equal(t, readJavaFixture(t).Latency, fixtureHistogram(t).Latency())
}

// TestDecodeHistogramReadsJavaEncoding decodes the histogram that the Java
// HdrHistogram encoded for the fixed input. The decoded histogram must give
// the Java percentiles and hold the same values as the Go histogram.
func TestDecodeHistogramReadsJavaEncoding(t *testing.T) {
	fixture := readJavaFixture(t)
	decoded, err := bench.DecodeHistogram(fixture.Histogram)
	require.NoError(t, err)

	require.Equal(t, fixture.Latency, decoded.Latency())
	percentiles := []float64{0.1, 50, 99.99, 99.999, 100}
	require.Equal(
		t,
		fixtureHistogram(t).ValuesAtPercentiles(percentiles),
		decoded.ValuesAtPercentiles(percentiles),
	)
}

func TestHistogramValuesAtPercentilesMatchLatency(t *testing.T) {
	histogram := fixtureHistogram(t)
	latency := histogram.Latency()
	require.Equal(
		t,
		[]int64{
			latency.P99Ns,
			latency.P50Ns,
			latency.MaxNs,
			latency.P90Ns,
			latency.P999Ns,
			latency.P9999Ns,
			latency.P99999Ns,
		},
		histogram.ValuesAtPercentiles([]float64{99, 50, 100, 90, 99.9, 99.99, 99.999}),
	)
}

func TestHistogramEncodeRoundTrip(t *testing.T) {
	histogram := fixtureHistogram(t)
	encoded, err := histogram.Encode()
	require.NoError(t, err)

	decoded, err := bench.DecodeHistogram(encoded)
	require.NoError(t, err)
	require.Equal(t, histogram.Latency(), decoded.Latency())
	percentiles := []float64{99.999}
	require.Equal(
		t,
		histogram.ValuesAtPercentiles(percentiles),
		decoded.ValuesAtPercentiles(percentiles),
	)
}

func TestHistogramMergeAddsValues(t *testing.T) {
	histogram := bench.NewHistogram()
	require.NoError(t, histogram.Record(1000))
	other := bench.NewHistogram()
	require.NoError(t, other.Record(3000))
	require.NoError(t, other.Record(5000))

	otherLatency := other.Latency()
	histogram.Merge(other)
	latency := histogram.Latency()
	require.Equal(t, int64(3), latency.Count)
	require.Equal(t, otherLatency.MaxNs, latency.MaxNs)
	require.Equal(t, otherLatency.P50Ns, histogram.ValuesAtPercentiles([]float64{60})[0])
}

func TestDecodeHistogramRejects(t *testing.T) {
	encode := func(highest int64, digits int) string {
		histogram := hdrhistogram.New(bench.HistogramLowestDiscernibleNs, highest, digits)
		encoded, err := histogram.Encode(hdrhistogram.V2CompressedEncodingCookieBase)
		require.NoError(t, err)
		return string(encoded)
	}
	tests := []struct {
		name    string
		encoded string
	}{
		{name: "not base64", encoded: "!"},
		{name: "empty", encoded: ""},
		{
			name:    "other precision",
			encoded: encode(bench.HistogramHighestTrackableNs, bench.HistogramSignificantDigits-1),
		},
		{
			name:    "other range",
			encoded: encode(bench.HistogramHighestTrackableNs/2, bench.HistogramSignificantDigits),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := bench.DecodeHistogram(tt.encoded)
			require.Error(t, err)
		})
	}
}

func TestHistogramResetClearsValues(t *testing.T) {
	histogram := bench.NewHistogram()
	require.NoError(t, histogram.Record(1000))
	histogram.Reset()
	require.NoError(t, histogram.Record(2000))

	latency := histogram.Latency()
	require.Equal(t, int64(1), latency.Count)
	require.Equal(t, latency.MaxNs, latency.P50Ns)
	require.GreaterOrEqual(t, latency.MaxNs, int64(2000))
}

func TestHistogramRecordDoesNotAllocate(t *testing.T) {
	histogram := bench.NewHistogram()
	require.Zero(t, testing.AllocsPerRun(1000, func() { _ = histogram.Record(12345) }))
}

func TestHistogramRecordRejectsOutOfRange(t *testing.T) {
	histogram := bench.NewHistogram()
	require.Error(t, histogram.Record(-1))
	require.Error(t, histogram.Record(bench.HistogramHighestTrackableNs*2))
	require.Equal(t, int64(0), histogram.Latency().Count)
}
