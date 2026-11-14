package bench_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// maxLateStepNs bounds an idle step that makes the sender late.
const maxLateStepNs = int64(2 * time.Second)

// TestFixedRateLoopKeepsSchedule runs the loop against the model
// transceiver. The model draws back pressure, reply delays, lost replies at
// the tail, and idle steps that make the sender late.
func TestFixedRateLoopKeepsSchedule(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		rate := rapid.Int64Range(1, bench.MaxRate).Draw(rt, "rate")
		messages := rapid.Int64Range(1, 48).Draw(rt, "messages")
		intervalNs := int64(time.Second) / rate
		startNs := rapid.Int64Range(0, 1<<40).Draw(rt, "start")
		model := &modelTransceiver{
			nowNs:         startNs,
			messageLength: modelMessageLength,
			lostFrom:      drawMostly(rt, messages, rapid.Int64Range(0, messages), "lost from"),
			isBackPressured: func() bool {
				return rapid.IntRange(0, 9).Draw(rt, "back pressure") == 0
			},
			replies: func(pending int) int {
				return rapid.IntRange(0, pending).Draw(rt, "replies")
			},
			idleStepNs: func() int64 {
				if rapid.IntRange(0, 9).Draw(rt, "late") == 0 {
					return rapid.Int64Range(1, maxLateStepNs).Draw(rt, "late step")
				}
				return rapid.Int64Range(1, 2*intervalNs).Draw(rt, "step")
			},
		}
		loop, histogram := newModelLoop(model)

		got, err := loop.Run(rate, messages)
		require.NoError(rt, err)

		// The oracle computes the scheduled length in exact integers.
		lengthNs := messages * int64(time.Second) / rate
		deadlineNs := startNs + lengthNs + int64(bench.FixedRateSendGrace)
		for i, message := range model.sent {
			sequence := int64(i)
			require.Equal(rt, sequence, message.sequence)
			require.Equal(rt, startNs+sequence*intervalNs, message.timestampNs)
			require.GreaterOrEqual(rt, message.sentAtNs, message.timestampNs, "message %d went out early", i)
			require.Less(rt, message.sentAtNs, deadlineNs, "message %d went out after the deadline", i)
		}
		scheduled := got.Expected
		require.Equal(rt, messages, scheduled)
		require.Equal(rt, int64(len(model.sent)), got.Sent)
		require.LessOrEqual(rt, got.Sent, got.Expected)
		require.Equal(
			rt,
			got.Sent-got.Received,
			int64(len(model.pending))+max(got.Sent-model.lostFrom, 0),
		)
		require.Equal(rt, got.Received, histogram.Latency().Count)
		require.Less(rt, model.nowNs, deadlineNs+int64(bench.FixedRateReceiveDeadline)+2*maxLateStepNs)
	})
}

// TestFixedRateTrackerMatchesModel draws replies that mostly match. The
// model counts replies up to the first mismatch. The tracker must agree
// with the model after each reply. The histogram must hold one value per
// counted reply.
func TestFixedRateTrackerMatchesModel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nowNs := rapid.Int64Range(0, 1<<40).Draw(rt, "now")
		histogram := bench.NewHistogram()
		tracker := bench.NewFixedRateTracker(histogram, modelMessageLength)
		var received int64
		failed := false
		for range rapid.IntRange(0, 64).Draw(rt, "count") {
			nowNs += rapid.Int64Range(0, 1_000_000).Draw(rt, "step")
			timestampNs := nowNs - rapid.Int64Range(0, nowNs).Draw(rt, "age")
			sequence := drawMostly(rt, received, rapid.Int64Range(-1, received+2), "sequence")
			length := drawMostly(rt, modelMessageLength, rapid.IntRange(0, 2*modelMessageLength), "length")
			isMatch := sequence == received && length == modelMessageLength
			if !failed && isMatch {
				received++
			} else {
				failed = true
			}

			tracker.Reply(nowNs, timestampNs, sequence, length)
			require.Equal(rt, received, tracker.Received())
			require.Equal(rt, failed, tracker.Err() != nil)
		}
		require.Equal(rt, received, histogram.Latency().Count)
	})
}
