package bench_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

func TestWorkerNextReturnsRaisedTarget(t *testing.T) {
	var worker bench.Worker
	worker.Request(3)
	worker.Request(2)

	target, ok := worker.Next(0, func() { t.Fatal("idle with a pending target") })
	require.True(t, ok)
	require.Equal(t, int64(5), target)
}

func TestWorkerNextIdlesUntilStop(t *testing.T) {
	var worker bench.Worker
	worker.Request(5)
	idles := 0

	_, ok := worker.Next(5, func() {
		idles++
		worker.Stop()
	})
	require.False(t, ok)
	require.Equal(t, 1, idles)
	require.True(t, worker.IsStopped())
}

func TestWorkerKeepsFirstError(t *testing.T) {
	var worker bench.Worker
	require.NoError(t, worker.Err())

	first := errors.New("first")
	worker.Fail(first)
	worker.Fail(errors.New("second"))
	require.ErrorIs(t, worker.Err(), first)
}

func TestSequenceChecker(t *testing.T) {
	tests := []struct {
		name         string
		sequences    []int64
		lengths      []int
		wantReceived int64
		wantErr      string
	}{
		{name: "in order", sequences: []int64{0, 1, 2}, lengths: []int{32, 32, 32}, wantReceived: 3},
		{
			name:         "gap",
			sequences:    []int64{0, 2, 3},
			lengths:      []int{32, 32, 32},
			wantReceived: 1,
			wantErr:      "message 1: sequence 2",
		},
		{
			name:         "duplicate",
			sequences:    []int64{0, 0},
			lengths:      []int{32, 32},
			wantReceived: 1,
			wantErr:      "message 1: sequence 0",
		},
		{
			name:         "short message",
			sequences:    []int64{0, 1},
			lengths:      []int{32, 16},
			wantReceived: 1,
			wantErr:      "message 1: length 16, want 32",
		},
		{
			name:      "stops at first mismatch",
			sequences: []int64{1, 0, 1},
			lengths:   []int{32, 32, 32},
			wantErr:   "message 0: sequence 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := bench.NewSequenceChecker(32)
			for i, sequence := range tt.sequences {
				checker.Check(sequence, tt.lengths[i])
			}
			require.Equal(t, tt.wantReceived, checker.Received())
			if tt.wantErr == "" {
				require.NoError(t, checker.Err())
				return
			}
			require.EqualError(t, checker.Err(), tt.wantErr)
		})
	}
}

// TestHotStatePadsFields keeps other heap objects off the cache lines of
// the fields of each type. One hot goroutine writes them on every message.
func TestHotStatePadsFields(t *testing.T) {
	for _, padded := range []reflect.Type{
		reflect.TypeFor[bench.SequenceChecker](),
		reflect.TypeFor[bench.FixedRateTracker](),
	} {
		t.Run(padded.Name(), func(t *testing.T) {
			for field := range padded.Fields() {
				if field.Name == "_" {
					continue
				}
				require.GreaterOrEqual(t, field.Offset, uintptr(bench.CacheLineLength), field.Name)
				require.LessOrEqual(
					t,
					field.Offset+field.Type.Size(),
					padded.Size()-bench.CacheLineLength,
					field.Name,
				)
			}
		})
	}
}

func TestPingTrackerRecordsMatchingReply(t *testing.T) {
	histogram := bench.NewHistogram()
	tracker := bench.NewPingTracker(histogram, 32)
	require.False(t, tracker.IsWaiting())

	sentNs := bench.NanoTime()
	tracker.Send(sentNs)
	require.True(t, tracker.IsWaiting())
	tracker.Reply(sentNs, 32)

	require.False(t, tracker.IsWaiting())
	require.NoError(t, tracker.Err())
	require.Equal(t, int64(1), histogram.Latency().Count)
}

func TestPingTrackerRejectsBadReply(t *testing.T) {
	tests := []struct {
		name    string
		sent    bool
		offset  int64
		length  int
		wantErr string
	}{
		{name: "no ping in flight", length: 32, wantErr: "reply without a ping in flight"},
		{name: "length", sent: true, length: 16, wantErr: "reply length 16, want 32"},
		{name: "timestamp", sent: true, offset: 1, length: 32, wantErr: "reply timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			histogram := bench.NewHistogram()
			tracker := bench.NewPingTracker(histogram, 32)
			sentNs := bench.NanoTime()
			if tt.sent {
				tracker.Send(sentNs)
			}
			tracker.Reply(sentNs+tt.offset, tt.length)

			require.ErrorContains(t, tracker.Err(), tt.wantErr)
			require.False(t, tracker.IsWaiting())
			require.Zero(t, histogram.Latency().Count)
		})
	}
}
