package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/idlestrategy"
	"github.com/lirm/aeron-go/aeron/logbuffer"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// setupTimeout bounds the connect and the waits for connected publications
// and images.
const setupTimeout = 10 * time.Second

// idle is the busy-spin idle strategy of every hot loop.
var idle = idlestrategy.Busy{}

// connect connects to the driver in cfg.AeronDir.
func connect(cfg bench.Config) (*aeron.Aeron, error) {
	client, err := aeron.Connect(
		aeron.NewContext().AeronDir(cfg.AeronDir).MediaDriverTimeout(setupTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return client, nil
}

// addPublication adds the publication kind that cfg.Publication names.
// aeron-go returns the same *aeron.Publication type for both kinds.
func addPublication(
	client *aeron.Aeron,
	cfg bench.Config,
	streamID int32,
) (
	*aeron.Publication,
	error,
) {
	if cfg.Publication == bench.ExclusivePublication {
		pub, err := client.AddExclusivePublication(cfg.Channel, streamID)
		if err != nil {
			return nil, fmt.Errorf("add exclusive publication: %w", err)
		}
		return pub, nil
	}
	pub, err := client.AddPublication(cfg.Channel, streamID)
	if err != nil {
		return nil, fmt.Errorf("add publication: %w", err)
	}
	return pub, nil
}

// waitUntilConnected waits until each publication has a subscriber and each
// subscription has an image.
func waitUntilConnected(
	ctx context.Context,
	publications []*aeron.Publication,
	subscriptions []*aeron.Subscription,
) error {
	return waitUntil(ctx, "connected publications and images", func() bool {
		for _, pub := range publications {
			if !pub.IsConnected() {
				return false
			}
		}
		for _, sub := range subscriptions {
			if !sub.IsConnected() {
				return false
			}
		}
		return true
	})
}

// sender sends whole messages through Offer or TryClaim, as cfg.SendMethod
// names. sender retries back pressure, admin action and not connected with a
// busy spin.
type sender struct {
	publication *aeron.Publication
	isTryClaim  bool
	claim       logbuffer.Claim
	worker      *bench.Worker
}

// newSender returns a sender on pub. The worker ends the retries: a stored
// worker error or Stop makes send return.
func newSender(pub *aeron.Publication, cfg bench.Config, worker *bench.Worker) *sender {
	return &sender{
		publication: pub,
		isTryClaim:  cfg.SendMethod == bench.SendTryClaim,
		worker:      worker,
	}
}

// send copies length bytes at offset in buffer into the publication. send
// does not allocate. aeron-go TryClaim panics when length does not fit one
// frame. The hot goroutine recovers that panic through failOnPanic or
// recoverPanic.
//
// Returns an error when the publication returns a terminal code, or the
// worker stops or fails during a retry.
func (s *sender) send(buffer *atomic.Buffer, offset, length int32) error {
	for {
		isSent, err := s.trySend(buffer, offset, length)
		if err != nil || isSent {
			return err
		}
		if err := s.worker.Err(); err != nil {
			return err
		}
		if s.worker.IsStopped() {
			return bench.ErrStopped
		}
		spinIdle()
	}
}

// trySend makes one Offer or TryClaim call. trySend returns false on back
// pressure, admin action or not connected. trySend does not allocate.
//
// Returns an error when the publication returns a terminal code.
func (s *sender) trySend(buffer *atomic.Buffer, offset, length int32) (bool, error) {
	var position int64
	if s.isTryClaim {
		position = s.publication.TryClaim(length, &s.claim)
		if position > 0 {
			s.claim.Buffer().PutBytes(s.claim.Offset(), buffer, offset, length)
			s.claim.Commit()
		}
	} else {
		position = s.publication.Offer(buffer, offset, length, nil)
	}
	if position > 0 {
		return true, nil
	}
	if !isRetriable(position) {
		return false, fmt.Errorf("send: result %d", position)
	}
	return false, nil
}

// isRetriable reports whether a send may succeed on a later attempt.
func isRetriable(position int64) bool {
	return position == aeron.BackPressured || position == aeron.AdminAction ||
		position == aeron.NotConnected
}

// failOnPanic stores a panic of the calling goroutine as a worker error.
// Defer it once at the top of a hot goroutine.
func failOnPanic(worker *bench.Worker) {
	if r := recover(); r != nil {
		worker.Fail(fmt.Errorf("panic: %v", r))
	}
}

// recoverPanic stores a panic of the calling function in err. Defer it once
// at the top of a function with a named error result.
func recoverPanic(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("panic: %v", r)
	}
}

// spinIdle idles one busy-spin step.
func spinIdle() {
	idle.Idle(0)
}

// closeClient closes client and joins its error to err.
func closeClient(client *aeron.Aeron, err *error) {
	*err = errors.Join(*err, client.Close())
}
