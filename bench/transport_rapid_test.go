package bench_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/warrenguy/luas-benchmarks/bench"
)

const rapidMessageLength = 32

// TestSequenceCheckerMatchesModel draws messages that mostly match. The model
// counts matches up to the first mismatch. The checker must agree with the
// model after each message.
func TestSequenceCheckerMatchesModel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		checker := bench.NewSequenceChecker(rapidMessageLength)
		var received int64
		mismatched := false
		for range rapid.IntRange(0, 64).Draw(rt, "count") {
			sequence := drawMostly(rt, received, rapid.Int64Range(-1, received+2), "sequence")
			length := drawMostly(rt, rapidMessageLength, rapid.IntRange(0, 2*rapidMessageLength), "length")
			isMatch := sequence == received && length == rapidMessageLength
			if !mismatched && isMatch {
				received++
			} else {
				mismatched = true
			}

			checker.Check(sequence, length)
			require.Equal(rt, received, checker.Received())
			require.Equal(rt, mismatched, checker.Err() != nil)
		}
	})
}

// TestPingTrackerMatchesModel draws a mix of sends and replies. The model
// tracks the ping in flight, the first error and the recorded count. The
// tracker must agree with the model after each step.
func TestPingTrackerMatchesModel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		histogram := bench.NewHistogram()
		tracker := bench.NewPingTracker(histogram, rapidMessageLength)
		var sentNs, recorded int64
		waiting, failed := false, false
		for range rapid.IntRange(0, 32).Draw(rt, "steps") {
			if rapid.Bool().Draw(rt, "send") {
				sentNs = bench.NanoTime() - rapid.Int64Range(0, 1_000_000).Draw(rt, "age")
				waiting = true
				tracker.Send(sentNs)
			} else {
				timestampNs := drawMostly(rt, sentNs, rapid.Int64Range(sentNs-1, sentNs+1), "timestamp")
				length := drawMostly(rt, rapidMessageLength, rapid.IntRange(0, 2*rapidMessageLength), "length")
				isMatch := waiting && timestampNs == sentNs && length == rapidMessageLength
				switch {
				case failed:
				case isMatch:
					waiting = false
					recorded++
				default:
					failed = true
				}
				tracker.Reply(timestampNs, length)
			}

			require.Equal(rt, waiting && !failed, tracker.IsWaiting())
			require.Equal(rt, failed, tracker.Err() != nil)
			require.Equal(rt, recorded, histogram.Latency().Count)
		}
	})
}

// drawMostly returns want in about nine draws of ten. Otherwise it returns a
// draw from other, which may still equal want.
func drawMostly[V any](rt *rapid.T, want V, other *rapid.Generator[V], label string) V {
	rt.Helper()
	if rapid.IntRange(0, 9).Draw(rt, label+" is other") > 0 {
		return want
	}
	return other.Draw(rt, label)
}
