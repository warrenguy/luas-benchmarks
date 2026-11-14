package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/warrenguy/luas/aeron"
	"github.com/warrenguy/luas/aeron/logbuffer"
	"github.com/warrenguy/luas/atomicbuffer"
	"github.com/warrenguy/luas/idlestrategy"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// setupTimeout bounds the connect and the waits for connected publications
// and images.
const setupTimeout = 10 * time.Second

// doWorkInterval is the count of conductorDuty ticks per Client.DoWork call.
const doWorkInterval = 1024

// publication holds the methods that aeron.Publication and
// aeron.ExclusivePublication share.
type publication interface {
	OfferWithOffset(buffer []byte, offset, length int) int64
	TryClaim(length int) (termIndex, frameOffset int, position int64, err error)
	TermBuffer(termIndex int) atomicbuffer.AtomicBuffer
	Commit(termIndex, frameOffset int) error
	MaxPayloadLength() int
	Position() int64
	WaitUntilConnected(ctx context.Context) error
	Close() error
}

// connect connects to the driver in cfg.AeronDir.
func connect(ctx context.Context, cfg bench.Config) (*aeron.Client, error) {
	client, err := aeron.Connect(ctx, aeron.Config{AeronDir: cfg.AeronDir})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return client, nil
}

// addPublication adds the publication kind that cfg.Publication names.
func addPublication(client *aeron.Client, cfg bench.Config, streamID int32) (publication, error) {
	if cfg.Publication == bench.ExclusivePublication {
		exclusive, err := client.AddExclusivePublication(streamID, cfg.Channel)
		if err != nil {
			return nil, fmt.Errorf("add exclusive publication: %w", err)
		}
		return exclusive, nil
	}
	concurrent, err := client.AddPublication(streamID, cfg.Channel)
	if err != nil {
		return nil, fmt.Errorf("add publication: %w", err)
	}
	return concurrent, nil
}

// waitUntilConnected waits until each publication has a subscriber and each
// subscription has an image.
func waitUntilConnected(
	ctx context.Context,
	publications []publication,
	subscriptions []*aeron.Subscription,
) error {
	for _, pub := range publications {
		if err := pub.WaitUntilConnected(ctx); err != nil {
			return fmt.Errorf("wait for publication: %w", err)
		}
	}
	for _, sub := range subscriptions {
		if err := sub.WaitUntilImageCount(ctx, 1); err != nil {
			return fmt.Errorf("wait for image: %w", err)
		}
	}
	return nil
}

// conductorDuty calls Client.DoWork from one hot goroutine of the client.
// Luas does not start a conductor goroutine, so that goroutine must call
// DoWork. Java and aeron-go run their conductor on a thread of its own. One
// DoWork call per doWorkInterval ticks keeps the client alive. The interval
// keeps conductor work out of most ticks. The same ticks drive the yields
// and the CPU checks of an unlocked hot goroutine.
type conductorDuty struct {
	client *aeron.Client
	yield  bench.HotYield
	repin  bench.HotRepin
	ticks  uint32
}

// newConductorDuty returns the duty of client on hot thread index of cfg.
// The duty re-pins a thread off its CPU with repin. The zero HotRepin
// re-pins nothing.
func newConductorDuty(
	client *aeron.Client,
	cfg bench.Config,
	index int,
	repin bench.HotRepin,
) conductorDuty {
	return conductorDuty{client: client, yield: cfg.HotYield(index), repin: repin}
}

// tick counts one poll, offer or wait spin. tick calls DoWork and checks
// for a due yield and a due CPU check on every doWorkInterval-th call.
func (d *conductorDuty) tick() {
	d.ticks++
	if d.ticks%doWorkInterval == 0 {
		d.client.DoWork()
		nowNs := bench.NanoTime()
		d.yield.Tick(nowNs)
		d.repin.Tick(nowNs, bench.OSThread{})
	}
}

// sender sends whole messages through Offer or TryClaim, as cfg.SendMethod
// names. sender retries back pressure, admin action and not connected with a
// busy spin.
type sender struct {
	publication publication
	isTryClaim  bool
	worker      *bench.Worker
}

// newSender returns a sender on pub. The worker ends the retries: a stored
// worker error or Stop makes send return.
//
// Returns an error when cfg.SendMethod is SendTryClaim and the message does
// not fit one frame.
func newSender(pub publication, cfg bench.Config, worker *bench.Worker) (*sender, error) {
	isTryClaim := cfg.SendMethod == bench.SendTryClaim
	if maxPayload := pub.MaxPayloadLength(); isTryClaim && cfg.MessageLength > maxPayload {
		return nil, fmt.Errorf(
			"try-claim: message length %d above max payload length %d",
			cfg.MessageLength,
			maxPayload,
		)
	}
	return &sender{publication: pub, isTryClaim: isTryClaim, worker: worker}, nil
}

// send copies message into the publication. send does not allocate.
//
// Returns an error when the publication returns a terminal code, a claim
// fails, or the worker stops or fails during a retry.
func (s *sender) send(message []byte) error {
	for {
		isSent, err := s.trySend(message)
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
// Returns an error when the publication returns a terminal code or a claim
// fails.
func (s *sender) trySend(message []byte) (bool, error) {
	var position int64
	if s.isTryClaim {
		termIndex, frameOffset, claimed, err := s.publication.TryClaim(len(message))
		if err != nil {
			return false, fmt.Errorf("try claim: %w", err)
		}
		if claimed > 0 {
			s.publication.TermBuffer(termIndex).PutBytes(frameOffset+logbuffer.OffsetFramePayload, message)
			if err := s.publication.Commit(termIndex, frameOffset); err != nil {
				return false, fmt.Errorf("commit claim: %w", err)
			}
		}
		position = claimed
	} else {
		position = s.publication.OfferWithOffset(message, 0, len(message))
	}
	if position > 0 {
		return true, nil
	}
	if !aeron.IsOfferRetriable(position) {
		return false, fmt.Errorf("send: %s", aeron.PositionCodeName(position))
	}
	return false, nil
}

// spinIdle idles one busy-spin step.
func spinIdle() {
	idlestrategy.BusySpin.Idle(0)
}

// closeClient closes client and joins its error to err.
func closeClient(client *aeron.Client, err *error) {
	*err = errors.Join(*err, client.Close())
}
