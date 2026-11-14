package bench

import (
	"errors"
	"fmt"
	"sync/atomic"
)

// ErrStopped reports that the measuring goroutine stopped the worker.
var ErrStopped = errors.New("worker stopped")

// CacheLineLength is the cache line length of the bench hosts in bytes.
const CacheLineLength = 64

// CacheLinePadding keeps other heap objects off the cache lines of the
// fields between two paddings. Pad state that one hot goroutine writes on
// every message. Without it, the allocator can place an object the other
// hot goroutine reads in the same line.
type CacheLinePadding [2 * CacheLineLength]byte

// Worker links the measuring goroutine with one hot worker goroutine: the
// throughput publisher or the ping-pong responder. The worker reports its
// failure back.
//
// Concurrency: any goroutine may call any method. The measuring goroutine
// calls Stop. The worker goroutine calls Fail. In throughput, the subscriber
// calls Request and the publisher calls Next. Either goroutine may call
// IsStopped and Err.
type Worker struct {
	requested atomic.Int64
	stopped   atomic.Bool
	failure   atomic.Pointer[error]
}

// Request raises the worker target by count messages.
func (w *Worker) Request(count int64) {
	w.requested.Add(count)
}

// Stop ends Next and makes IsStopped true. Stop is idempotent.
func (w *Worker) Stop() {
	w.stopped.Store(true)
}

// IsStopped reports whether Stop ran.
func (w *Worker) IsStopped() bool {
	return w.stopped.Load()
}

// Next spins until the target rises above done or Stop runs. Next calls
// idle between checks.
//
// Returns the target and true, or 0 and false after Stop.
func (w *Worker) Next(done int64, idle func()) (int64, bool) {
	for {
		if w.stopped.Load() {
			return 0, false
		}
		if target := w.requested.Load(); target > done {
			return target, true
		}
		idle()
	}
}

// Fail stores the first worker error. Later calls do nothing.
func (w *Worker) Fail(err error) {
	w.failure.CompareAndSwap(nil, &err)
}

// Err returns the error Fail stored, or nil.
func (w *Worker) Err() error {
	if err := w.failure.Load(); err != nil {
		return *err
	}
	return nil
}

// SequenceChecker counts throughput messages on the subscriber. The
// publisher writes the sequence number 0, 1, 2 and so on into the first 8
// bytes of each message. Check accepts only the next number at the
// configured length. The first mismatch stops the count.
// The paddings keep other heap objects off the cache lines of the fields.
//
// Concurrency: one goroutine at a time.
type SequenceChecker struct {
	_             CacheLinePadding
	messageLength int
	received      int64
	err           error
	_             CacheLinePadding
}

// NewSequenceChecker returns a SequenceChecker for messages of messageLength
// bytes.
func NewSequenceChecker(messageLength int) *SequenceChecker {
	return &SequenceChecker{messageLength: messageLength}
}

// Check counts one message. Check does not allocate while the messages
// match.
func (c *SequenceChecker) Check(sequence int64, length int) {
	if c.err != nil {
		return
	}
	if length != c.messageLength {
		c.err = fmt.Errorf("message %d: length %d, want %d", c.received, length, c.messageLength)
		return
	}
	if sequence != c.received {
		c.err = fmt.Errorf("message %d: sequence %d", c.received, sequence)
		return
	}
	c.received++
}

// Received returns the count of matching messages.
func (c *SequenceChecker) Received() int64 {
	return c.received
}

// Err returns the first mismatch, or nil.
func (c *SequenceChecker) Err() error {
	return c.err
}

// PingTracker records ping-pong round trips. Ping writes the send timestamp
// into the first 8 bytes of each message. Pong echoes the message. Reply
// checks the echo and records the round trip time.
//
// Concurrency: one goroutine at a time.
type PingTracker struct {
	histogram     *Histogram
	messageLength int
	sentNs        int64
	waiting       bool
	err           error
}

// NewPingTracker returns a PingTracker that records into histogram.
func NewPingTracker(histogram *Histogram, messageLength int) *PingTracker {
	return &PingTracker{histogram: histogram, messageLength: messageLength}
}

// Send marks one ping in flight with send timestamp sentNs from NanoTime.
func (p *PingTracker) Send(sentNs int64) {
	p.sentNs = sentNs
	p.waiting = true
}

// Reset removes every recorded value.
func (p *PingTracker) Reset() {
	p.histogram.Reset()
}

// Reply checks one echo and records NanoTime minus timestampNs. Reply
// stores an error when no ping is in flight, or when the echo length or
// timestamp differs from the ping. Reply does not allocate while the echoes
// match.
func (p *PingTracker) Reply(timestampNs int64, length int) {
	nowNs := NanoTime()
	switch {
	case p.err != nil:
		return
	case !p.waiting:
		p.err = errors.New("reply without a ping in flight")
	case length != p.messageLength:
		p.err = fmt.Errorf("reply length %d, want %d", length, p.messageLength)
	case timestampNs != p.sentNs:
		p.err = fmt.Errorf("reply timestamp %d, want %d", timestampNs, p.sentNs)
	default:
		p.waiting = false
		if err := p.histogram.Record(nowNs - timestampNs); err != nil {
			p.err = fmt.Errorf("record round trip: %w", err)
		}
	}
}

// IsWaiting reports whether a ping is in flight and Reply has not stored an
// error.
func (p *PingTracker) IsWaiting() bool {
	return p.waiting && p.err == nil
}

// Err returns the first error Reply stored, or nil.
func (p *PingTracker) Err() error {
	return p.err
}
