package bench

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// Histogram range and precision. java/bench/LatencyHistogram.java uses the
// same values.
const (
	HistogramLowestDiscernibleNs = 1
	HistogramHighestTrackableNs  = int64(time.Hour)
	HistogramSignificantDigits   = 3
)

// Latency holds round trip percentiles in nanoseconds. Java writes the same
// JSON fields.
type Latency struct {
	Count    int64 `json:"count"`
	P50Ns    int64 `json:"p50_ns"`
	P90Ns    int64 `json:"p90_ns"`
	P99Ns    int64 `json:"p99_ns"`
	P999Ns   int64 `json:"p99_9_ns"`
	P9999Ns  int64 `json:"p99_99_ns"`
	P99999Ns int64 `json:"p99_999_ns"`
	MaxNs    int64 `json:"max_ns"`

	// Histogram holds the round trip histogram of a ping-pong or cluster
	// scenario in the HdrHistogram V2 compressed format, base64 encoded.
	// Empty in a fixed-rate result, where FixedRate holds it.
	Histogram string `json:"histogram,omitempty"`
}

// Histogram records latencies in nanoseconds with 3 significant digits.
//
// Concurrency: one goroutine at a time.
type Histogram struct {
	histogram *hdrhistogram.Histogram
}

// NewHistogram returns an empty Histogram.
func NewHistogram() *Histogram {
	return &Histogram{histogram: hdrhistogram.New(
		HistogramLowestDiscernibleNs,
		HistogramHighestTrackableNs,
		HistogramSignificantDigits,
	)}
}

// Record adds one value. Record does not allocate.
//
// Returns an error when valueNs is below 0 or beyond the histogram range. The
// range covers at least HistogramHighestTrackableNs.
func (h *Histogram) Record(valueNs int64) error {
	return h.histogram.RecordValue(valueNs)
}

// Reset removes every recorded value.
func (h *Histogram) Reset() {
	h.histogram.Reset()
}

// Latency returns the count, the percentiles and the max. Latency allocates.
// Call it outside the measured section.
//
// The percentiles use the Java HdrHistogram rank rule, not the Go one. The Go
// port rounds the rank to the nearest count. The Java port rounds the rank up.
func (h *Histogram) Latency() Latency {
	values := h.ValuesAtPercentiles([]float64{50, 90, 99, 99.9, 99.99, 99.999})
	return Latency{
		Count:    h.histogram.TotalCount(),
		P50Ns:    values[0],
		P90Ns:    values[1],
		P99Ns:    values[2],
		P999Ns:   values[3],
		P9999Ns:  values[4],
		P99999Ns: values[5],
		MaxNs:    h.histogram.Max(),
	}
}

// ValuesAtPercentiles returns the value at each of percentiles, from above
// 0 to 100, by the Java HdrHistogram rank rule. ValuesAtPercentiles reads
// the buckets once. ValuesAtPercentiles allocates.
//
// Returns 0 for each percentile when the histogram is empty.
func (h *Histogram) ValuesAtPercentiles(percentiles []float64) []int64 {
	bars := h.histogram.Distribution()
	total := h.histogram.TotalCount()
	values := make([]int64, len(percentiles))
	for i, percentile := range percentiles {
		values[i] = valueAtPercentile(bars, total, percentile)
	}
	return values
}

// Encode returns the histogram in the HdrHistogram V2 compressed format,
// base64 encoded. Java encodeIntoCompressedByteBuffer and a base64 encoder
// write the same format. Encode allocates.
func (h *Histogram) Encode() (string, error) {
	encoded, err := h.histogram.Encode(hdrhistogram.V2CompressedEncodingCookieBase)
	if err != nil {
		return "", fmt.Errorf("encode histogram: %w", err)
	}
	return string(encoded), nil
}

// DecodeHistogram returns the histogram that Encode or the Java encoder
// wrote into encoded.
//
// Returns an error when encoded does not decode, or holds a histogram of
// another range or precision.
func DecodeHistogram(encoded string) (*Histogram, error) {
	if encoded == "" {
		return nil, errors.New("decode histogram: empty")
	}
	decoded, err := hdrhistogram.Decode([]byte(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode histogram: %w", err)
	}
	lowest := decoded.LowestTrackableValue()
	highest := decoded.HighestTrackableValue()
	digits := decoded.SignificantFigures()
	isSameGeometry := lowest == HistogramLowestDiscernibleNs &&
		highest == HistogramHighestTrackableNs &&
		digits == HistogramSignificantDigits
	if !isSameGeometry {
		return nil, fmt.Errorf(
			"decode histogram: range %d-%d ns at %d digits, want %d-%d ns at %d digits",
			lowest,
			highest,
			digits,
			HistogramLowestDiscernibleNs,
			HistogramHighestTrackableNs,
			HistogramSignificantDigits,
		)
	}
	return &Histogram{histogram: decoded}, nil
}

// Merge adds every value of other. NewHistogram and DecodeHistogram give
// every histogram the same range, so Merge drops no value.
func (h *Histogram) Merge(other *Histogram) {
	h.histogram.Merge(other.histogram)
}

// valueAtPercentile ports AbstractHistogram.getValueAtPercentile from Java
// HdrHistogram 2.2.2 for a percentile above 0. bars holds the buckets in
// value order up to the highest recorded value, as Histogram.Distribution
// returns them.
//
// Returns 0 when total is 0.
func valueAtPercentile(bars []hdrhistogram.Bar, total int64, percentile float64) int64 {
	requested := math.Min(math.Max(math.Nextafter(percentile, math.Inf(-1)), 0), 100)
	rank := max(int64(math.Ceil(requested*float64(total)/100)), 1)
	var count int64
	for _, bar := range bars {
		count += bar.Count
		if count >= rank {
			return bar.To
		}
	}
	return 0
}
