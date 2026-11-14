package bench_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// modelMessageLength is the message length of the model transceiver.
const modelMessageLength = 32

// modelMessage holds one message that the model transceiver sent.
type modelMessage struct {
	timestampNs int64
	sequence    int64
	sentAtNs    int64
}

// modelTransceiver queues each sent message and echoes it on a later poll.
// Its clock moves only when the loop idles without work.
type modelTransceiver struct {
	nowNs         int64
	messageLength int
	tracker       *bench.FixedRateTracker

	// lostFrom is the first sequence whose reply never comes.
	lostFrom int64

	// isBackPressured reports whether the next send attempt fails.
	isBackPressured func() bool

	// replies returns how many of pending queued replies the next poll
	// delivers.
	replies func(pending int) int

	// idleStepNs returns how far the clock moves on an idle without work.
	idleStepNs func() int64

	// sendErr makes each send attempt fail.
	sendErr error

	sent    []modelMessage
	pending []modelMessage
}

func (m *modelTransceiver) clock() int64 {
	return m.nowNs
}

func (m *modelTransceiver) Send(timestampNs, sequence int64) (bool, error) {
	if m.sendErr != nil {
		return false, m.sendErr
	}
	if m.isBackPressured != nil && m.isBackPressured() {
		return false, nil
	}
	message := modelMessage{timestampNs: timestampNs, sequence: sequence, sentAtNs: m.nowNs}
	m.sent = append(m.sent, message)
	if sequence < m.lostFrom {
		m.pending = append(m.pending, message)
	}
	return true, nil
}

func (m *modelTransceiver) Poll() int {
	count := len(m.pending)
	if m.replies != nil {
		count = m.replies(count)
	}
	for _, message := range m.pending[:count] {
		m.tracker.Reply(m.nowNs, message.timestampNs, message.sequence, m.messageLength)
	}
	m.pending = m.pending[count:]
	return count
}

func (m *modelTransceiver) Idle(workCount int) {
	if workCount == 0 {
		m.nowNs += m.idleStepNs()
	}
}

// newModelLoop returns a loop over model. The loop records into a new
// histogram.
func newModelLoop(model *modelTransceiver) (bench.FixedRateLoop, *bench.Histogram) {
	histogram := bench.NewHistogram()
	model.tracker = bench.NewFixedRateTracker(histogram, modelMessageLength)
	return bench.FixedRateLoop{
		Transceiver: model,
		Tracker:     model.tracker,
		Worker:      &bench.Worker{},
		Clock:       model.clock,
	}, histogram
}

// TestFixedRateLoopSendsOnSchedule runs 4 messages at 1 M msg/s. The clock
// moves 400 ns per idle. Each message carries its slot time and goes out at
// the first idle step at or after it.
func TestFixedRateLoopSendsOnSchedule(t *testing.T) {
	model := &modelTransceiver{
		nowNs:         5000,
		messageLength: modelMessageLength,
		lostFrom:      4,
		idleStepNs:    func() int64 { return 400 },
	}
	loop, histogram := newModelLoop(model)

	got, err := loop.Run(1_000_000, 4)
	require.NoError(t, err)
	require.Equal(t, bench.FixedRate{Expected: 4, Sent: 4, Received: 4}, got)
	require.Equal(t, []modelMessage{
		{timestampNs: 5000, sequence: 0, sentAtNs: 5000},
		{timestampNs: 6000, sequence: 1, sentAtNs: 6200},
		{timestampNs: 7000, sequence: 2, sentAtNs: 7000},
		{timestampNs: 8000, sequence: 3, sentAtNs: 8200},
	}, model.sent)
	require.Equal(t, int64(4), histogram.Latency().Count)
}

func TestFixedRateLoopRejects(t *testing.T) {
	tests := []struct {
		name    string
		model   func(model *modelTransceiver)
		pongErr error
		wantErr string
	}{
		{
			name:    "send error",
			model:   func(model *modelTransceiver) { model.sendErr = errors.New("send: closed") },
			wantErr: "send: closed",
		},
		{
			name:    "reply length",
			model:   func(model *modelTransceiver) { model.messageLength = modelMessageLength + 1 },
			wantErr: "reply 0: length 33, want 32",
		},
		{
			name:    "pong failure",
			model:   func(model *modelTransceiver) { model.lostFrom = 0 },
			pongErr: errors.New("pong send failed"),
			wantErr: "pong: pong send failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &modelTransceiver{
				messageLength: modelMessageLength,
				lostFrom:      4,
				idleStepNs:    func() int64 { return 400 },
			}
			loop, _ := newModelLoop(model)
			if tt.model != nil {
				tt.model(model)
			}
			if tt.pongErr != nil {
				loop.Worker.Fail(tt.pongErr)
			}

			_, err := loop.Run(1_000_000, 4)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestFixedRateLoopWarmupRejectsLostReplies loses the reply of the last
// warmup message. A late reply would break the sequence of the measured
// run, so the warmup fails.
func TestFixedRateLoopWarmupRejectsLostReplies(t *testing.T) {
	model := &modelTransceiver{messageLength: modelMessageLength, lostFrom: 3}
	// Small steps keep every send on schedule. Steps of 1 s then end the
	// wait for the lost reply.
	model.idleStepNs = func() int64 {
		if len(model.sent) < 4 {
			return 400
		}
		return int64(time.Second)
	}
	loop, _ := newModelLoop(model)

	require.ErrorContains(t, loop.Warmup(1_000_000, 4), "3 of 4 replies")
}

// TestFixedRateLoopRunStartsEmpty runs twice. The second run must count its
// replies from 0 and record only its own values.
func TestFixedRateLoopRunStartsEmpty(t *testing.T) {
	model := &modelTransceiver{
		messageLength: modelMessageLength,
		lostFrom:      math.MaxInt64,
		idleStepNs:    func() int64 { return 400 },
	}
	loop, histogram := newModelLoop(model)
	require.NoError(t, loop.Warmup(1_000_000, 4))

	got, err := loop.Run(1_000_000, 2)
	require.NoError(t, err)
	require.Equal(t, bench.FixedRate{Expected: 2, Sent: 2, Received: 2}, got)
	require.Equal(t, int64(2), histogram.Latency().Count)
}

func TestFixedRateTrackerRejects(t *testing.T) {
	tests := []struct {
		name     string
		sequence int64
		length   int
		wantErr  string
	}{
		{
			name:     "length",
			sequence: 0,
			length:   modelMessageLength - 1,
			wantErr:  "reply 0: length 31, want 32",
		},
		{name: "sequence", sequence: 1, length: modelMessageLength, wantErr: "reply 0: sequence 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := bench.NewFixedRateTracker(bench.NewHistogram(), modelMessageLength)
			tracker.Reply(1000, 0, tt.sequence, tt.length)
			require.EqualError(t, tracker.Err(), tt.wantErr)
			require.Zero(t, tracker.Received())
		})
	}
}

func TestFixedRateTrackerReplyDoesNotAllocate(t *testing.T) {
	tracker := bench.NewFixedRateTracker(bench.NewHistogram(), modelMessageLength)
	var sequence int64
	require.Zero(t, testing.AllocsPerRun(1000, func() {
		tracker.Reply(1000, 0, sequence, modelMessageLength)
		sequence++
	}))
	require.NoError(t, tracker.Err())
}

func TestFixedRateHasShortfall(t *testing.T) {
	require.False(t, bench.FixedRate{Expected: 4, Sent: 4, Received: 4}.HasShortfall())
	require.True(t, bench.FixedRate{Expected: 4, Sent: 3, Received: 3}.HasShortfall())
	require.True(t, bench.FixedRate{Expected: 4, Sent: 4, Received: 3}.HasShortfall())
}
