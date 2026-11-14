package bench

import (
	"fmt"
	"time"
)

// Fixed-rate send loop limits. java/bench/FixedRateLoop.java uses the same
// values. They match the Aeron load test rig.
const (
	// FixedRateSendGrace is how long the loop keeps sending after the
	// scheduled end of the run.
	FixedRateSendGrace = 100 * time.Millisecond

	// FixedRateReceiveDeadline is how long the loop waits for the last
	// replies after it stops sending.
	FixedRateReceiveDeadline = 3 * time.Second
)

// FixedRate holds the message counts and the latency histogram of one
// fixed-rate run. Java writes the same JSON fields.
type FixedRate struct {
	// Expected is the message count of the schedule.
	Expected int64 `json:"expected"`

	// Sent counts the messages that went out before the deadline.
	Sent int64 `json:"sent"`

	// Received counts the replies.
	Received int64 `json:"received"`

	// Histogram holds the round trip histogram in the HdrHistogram V2
	// compressed format, base64 encoded.
	Histogram string `json:"histogram"`
}

// HasShortfall reports whether the run sent fewer messages than its
// schedule, or received fewer replies than it sent.
func (r FixedRate) HasShortfall() bool {
	return r.Sent < r.Expected || r.Received < r.Sent
}

// FixedRateTransceiver sends the messages of a fixed-rate run and polls the
// replies. Each client implements it over its ping publication and pong
// subscription.
type FixedRateTransceiver interface {
	// Send makes one Offer or TryClaim attempt. The message carries the
	// scheduled send time timestampNs in bytes 0-7 and sequence in bytes
	// 8-15, little endian. Send returns false on back pressure, admin action
	// or not connected.
	Send(timestampNs, sequence int64) (bool, error)

	// Poll polls the replies once and returns the fragment count. The reply
	// handler passes each reply to the FixedRateTracker.
	Poll() int

	// Idle idles one busy-spin step after a pass that handled workCount
	// fragments.
	Idle(workCount int)
}

// FixedRateTracker records the round trips of a fixed-rate run. Reply
// accepts only the next sequence at the configured length. The first
// mismatch stops the count. The paddings keep other heap objects off the
// cache lines of the fields.
//
// Concurrency: one goroutine at a time.
type FixedRateTracker struct {
	_             CacheLinePadding
	histogram     *Histogram
	messageLength int
	received      int64
	err           error
	_             CacheLinePadding
}

// NewFixedRateTracker returns a FixedRateTracker that records into
// histogram.
func NewFixedRateTracker(histogram *Histogram, messageLength int) *FixedRateTracker {
	return &FixedRateTracker{histogram: histogram, messageLength: messageLength}
}

// Reply checks one reply and records nowNs minus timestampNs. nowNs is the
// current time on the clock of the scheduled send times. Reply stores an
// error when the length differs from the message length, or when sequence
// differs from the reply count. Reply does not allocate while the replies
// match.
func (t *FixedRateTracker) Reply(nowNs, timestampNs, sequence int64, length int) {
	switch {
	case t.err != nil:
		return
	case length != t.messageLength:
		t.err = fmt.Errorf("reply %d: length %d, want %d", t.received, length, t.messageLength)
	case sequence != t.received:
		t.err = fmt.Errorf("reply %d: sequence %d", t.received, sequence)
	default:
		if err := t.histogram.Record(nowNs - timestampNs); err != nil {
			t.err = fmt.Errorf("record round trip: %w", err)
			return
		}
		t.received++
	}
}

// Received returns the count of matching replies.
func (t *FixedRateTracker) Received() int64 {
	return t.received
}

// Err returns the first error Reply stored, or nil.
func (t *FixedRateTracker) Err() error {
	return t.err
}

// reset removes the recorded values and zeroes the reply count.
func (t *FixedRateTracker) reset() {
	t.histogram.Reset()
	t.received = 0
}

// FixedRateLoop sends messages on a fixed schedule and polls the replies
// between sends. It ports LoadTestRig.send of the Aeron load test rig with a
// burst of 1. Message k carries its scheduled send time
// start + k * interval. A late sender sends at once with that time, so the
// recorded round trip includes the delay.
//
// Concurrency: one goroutine at a time.
type FixedRateLoop struct {
	Transceiver FixedRateTransceiver
	Tracker     *FixedRateTracker

	// Worker reports a pong failure.
	Worker *Worker

	// Clock returns the current time in nanoseconds: NanoTime outside tests.
	Clock func() int64
}

// Run sends messages at rate messages per second. Run starts from an empty
// tracker. The returned FixedRate holds the counts and no histogram. A
// shortfall is a result, not an error.
//
// Run follows this order:
//  1. Each send attempt carries the next message.
//  2. After a send, Run spins until the next scheduled time or the scheduled
//     end, and polls while replies are outstanding.
//  3. After back pressure, Run polls once and retries the same message.
//  4. Past the scheduled end, Run polls once per attempt.
//  5. Run stops sending after every message, or FixedRateSendGrace past the
//     scheduled end.
//  6. Run polls the last replies for up to FixedRateReceiveDeadline.
//
// Returns an error when a send fails, a reply does not match or pong fails.
func (l FixedRateLoop) Run(rate, messages int64) (FixedRate, error) {
	l.Tracker.reset()
	intervalNs := int64(time.Second) / rate
	startNs := l.Clock()
	stopNs := startNs + scheduleLengthNs(rate, messages)
	deadlineNs := stopNs + int64(FixedRateSendGrace)
	result := FixedRate{Expected: messages}
	timestampNs := startNs
	for result.Sent < messages {
		var nowNs int64
		isSent, err := l.Transceiver.Send(timestampNs, result.Sent)
		if err != nil {
			return FixedRate{}, err
		}
		if isSent {
			result.Sent++
			if result.Sent == messages {
				break
			}
			timestampNs += intervalNs
			nowNs = l.Clock()
			for nowNs < timestampNs && nowNs < stopNs {
				fragments := 0
				if l.Tracker.Received() < result.Sent {
					if fragments, err = l.poll(); err != nil {
						return FixedRate{}, err
					}
				}
				l.Transceiver.Idle(fragments)
				nowNs = l.Clock()
			}
		} else {
			fragments, err := l.poll()
			if err != nil {
				return FixedRate{}, err
			}
			l.Transceiver.Idle(fragments)
			nowNs = l.Clock()
		}
		if nowNs >= deadlineNs {
			break
		}
		if nowNs >= stopNs {
			if _, err := l.poll(); err != nil {
				return FixedRate{}, err
			}
		}
	}

	receiveDeadlineNs := l.Clock() + int64(FixedRateReceiveDeadline)
	for l.Tracker.Received() < result.Sent {
		fragments, err := l.poll()
		if err != nil {
			return FixedRate{}, err
		}
		l.Transceiver.Idle(fragments)
		if fragments == 0 && l.Clock() >= receiveDeadlineNs {
			break
		}
	}
	result.Received = l.Tracker.Received()
	return result, nil
}

// Warmup runs Run at rate over messages and drops the result.
//
// Returns an error when Run fails, or when a reply did not come back. A
// late reply would break the sequence of the next run.
func (l FixedRateLoop) Warmup(rate, messages int64) error {
	result, err := l.Run(rate, messages)
	if err != nil {
		return err
	}
	if result.Received < result.Sent {
		return fmt.Errorf("%d of %d replies", result.Received, result.Sent)
	}
	return nil
}

// poll polls the replies once.
//
// Returns an error when a reply does not match, or when the poll finds no
// fragment and pong has failed.
func (l FixedRateLoop) poll() (int, error) {
	fragments := l.Transceiver.Poll()
	if err := l.Tracker.Err(); err != nil {
		return 0, err
	}
	if fragments == 0 {
		if err := l.Worker.Err(); err != nil {
			return 0, fmt.Errorf("pong: %w", err)
		}
	}
	return fragments, nil
}

// scheduleLengthNs returns messages * 1e9 / rate, rounded down, without
// overflow for a rate up to MaxRate.
func scheduleLengthNs(rate, messages int64) int64 {
	second := int64(time.Second)
	return messages/rate*second + messages%rate*second/rate
}
