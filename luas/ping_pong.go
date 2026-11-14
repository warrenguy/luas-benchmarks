package main

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/warrenguy/luas/aeron"
	"github.com/warrenguy/luas/atomicbuffer"
	"github.com/warrenguy/luas/idlestrategy"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// pingSide holds what the ping goroutine of a ping-pong pair uses: its
// client, its sender on cfg.StreamID, its subscription to the echoes on
// cfg.PongStreamID, and the worker that reports a pong failure.
type pingSide struct {
	client       *aeron.Client
	sender       *sender
	subscription *aeron.Subscription
	worker       *bench.Worker
}

// runPingPong measures round trips between ping and pong. One message is in
// flight at a time.
func runPingPong(ctx context.Context, scenario bench.Scenario, args []string) error {
	cfg, err := bench.ParseFlagsWithUnlock(scenario, args)
	if err != nil {
		return err
	}
	return withPingPong(ctx, cfg, func(ping pingSide) error {
		histogram := bench.NewHistogram()
		tracker := bench.NewPingTracker(histogram, cfg.MessageLength)
		handler := func(buffer atomicbuffer.AtomicBuffer, offset, length int, _ *aeron.Header) {
			tracker.Reply(int64(binary.LittleEndian.Uint64(buffer.Bytes()[offset:])), length)
		}
		message := make([]byte, cfg.MessageLength)
		duty := newConductorDuty(ping.client, cfg, bench.PingThread, cfg.HotRepin(bench.PingThread))
		pass := func(count int64) error {
			for range count {
				sentNs := bench.NanoTime()
				binary.LittleEndian.PutUint64(message, uint64(sentNs))
				tracker.Send(sentNs)
				if err := ping.sender.send(message); err != nil {
					return err
				}
				for tracker.IsWaiting() {
					fragments := ping.subscription.Poll(handler, cfg.FragmentLimit)
					duty.tick()
					if fragments == 0 {
						if err := ping.worker.Err(); err != nil {
							return fmt.Errorf("pong: %w", err)
						}
					}
					idlestrategy.BusySpin.Idle(fragments)
				}
				if err := tracker.Err(); err != nil {
					return err
				}
			}
			return nil
		}
		warmup := func(count int64) error {
			defer tracker.Reset()
			return pass(count)
		}

		measurement, err := bench.Measure(cfg, warmup, pass)
		if err != nil {
			return err
		}
		latency := histogram.Latency()
		if latency.Histogram, err = histogram.Encode(); err != nil {
			return err
		}
		return bench.WriteJSON(cfg.Output, bench.Result{
			Scenario:  scenario.Name,
			Client:    clientName,
			Variant:   cfg.Variant(),
			Config:    cfg,
			Latency:   &latency,
			GoRuntime: &measurement.GoRuntime,
		})
	})
}

// withPingPong runs ping on the calling goroutine and pong on a second
// goroutine. Ping sends on cfg.StreamID. Pong echoes each message on
// cfg.PongStreamID. Ping and pong each own one client, because a Luas
// client polls on one goroutine only. withPingPong connects both, then pins
// ping and pong. Pong starts before measure. withPingPong stops pong after
// measure returns.
func withPingPong(ctx context.Context, cfg bench.Config, measure func(pingSide) error) (err error) {
	setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	pingClient, err := connect(setupCtx, cfg)
	if err != nil {
		return err
	}
	defer closeClient(pingClient, &err)
	pongClient, err := connect(setupCtx, cfg)
	if err != nil {
		return err
	}
	defer closeClient(pongClient, &err)

	pingPub, err := addPublication(pingClient, cfg, cfg.StreamID)
	if err != nil {
		return err
	}
	pongSub, err := pingClient.AddSubscription(cfg.PongStreamID, cfg.Channel)
	if err != nil {
		return fmt.Errorf("add pong subscription: %w", err)
	}
	pingSub, err := pongClient.AddSubscription(cfg.StreamID, cfg.Channel)
	if err != nil {
		return fmt.Errorf("add ping subscription: %w", err)
	}
	pongPub, err := addPublication(pongClient, cfg, cfg.PongStreamID)
	if err != nil {
		return err
	}
	if err := waitUntilConnected(
		setupCtx,
		[]publication{pingPub, pongPub},
		[]*aeron.Subscription{pingSub, pongSub},
	); err != nil {
		return err
	}
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()

	var worker bench.Worker
	pingSender, err := newSender(pingPub, cfg, &worker)
	if err != nil {
		return err
	}
	pongSender, err := newSender(pongPub, cfg, &worker)
	if err != nil {
		return err
	}
	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	pongDone := make(chan struct{})
	if err := cfg.RunPinnedPair(bench.PingThread, bench.PongThread, func() {
		defer close(pongDone)
		pong(cfg, pongClient, pingSub, pongSender, &worker)
	}); err != nil {
		return err
	}
	defer func() {
		worker.Stop()
		<-pongDone
	}()

	return measure(pingSide{
		client:       pingClient,
		sender:       pingSender,
		subscription: pongSub,
		worker:       &worker,
	})
}

// pong runs on the pong goroutine. pong echoes each ping message on sender.
// pong returns after Stop or the first send error.
func pong(
	cfg bench.Config,
	client *aeron.Client,
	sub *aeron.Subscription,
	sender *sender,
	worker *bench.Worker,
) {
	var sendErr error
	handler := func(buffer atomicbuffer.AtomicBuffer, offset, length int, _ *aeron.Header) {
		if sendErr == nil {
			sendErr = sender.send(buffer.Bytes()[offset : offset+length])
		}
	}
	if cfg.IsFragmented() {
		handler = aeron.NewFragmentAssembler(handler).Handler()
	}
	duty := newConductorDuty(client, cfg, bench.PongThread, cfg.HotRepin(bench.PongThread))
	for !worker.IsStopped() {
		fragments := sub.Poll(handler, cfg.FragmentLimit)
		duty.tick()
		if sendErr != nil {
			worker.Fail(sendErr)
			return
		}
		idlestrategy.BusySpin.Idle(fragments)
	}
}
