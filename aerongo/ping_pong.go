package main

import (
	"context"
	"fmt"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// pingSide holds what the ping goroutine of a ping-pong pair uses: its
// sender on cfg.StreamID, its subscription to the echoes on
// cfg.PongStreamID, and the worker that reports a pong failure.
type pingSide struct {
	sender       *sender
	subscription *aeron.Subscription
	worker       *bench.Worker
}

// runPingPong measures round trips between ping and pong. One message is in
// flight at a time.
func runPingPong(ctx context.Context, scenario bench.Scenario, args []string) error {
	cfg, err := bench.ParseFlags(scenario, args)
	if err != nil {
		return err
	}
	return withPingPong(ctx, cfg, func(ping pingSide) error {
		histogram := bench.NewHistogram()
		tracker := bench.NewPingTracker(histogram, cfg.MessageLength)
		handler := func(buffer *atomic.Buffer, offset, length int32, _ *logbuffer.Header) {
			tracker.Reply(buffer.GetInt64(offset), int(length))
		}
		message := atomic.MakeBuffer(make([]byte, cfg.MessageLength))
		length := message.Capacity()
		pass := func(count int64) (err error) {
			defer recoverPanic(&err)
			for range count {
				sentNs := bench.NanoTime()
				message.PutInt64(0, sentNs)
				tracker.Send(sentNs)
				if err := ping.sender.send(message, 0, length); err != nil {
					return err
				}
				for tracker.IsWaiting() {
					fragments := ping.subscription.Poll(handler, cfg.FragmentLimit)
					if fragments == 0 {
						if err := ping.worker.Err(); err != nil {
							return fmt.Errorf("pong: %w", err)
						}
					}
					idle.Idle(fragments)
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
// cfg.PongStreamID. Ping and pong each own one client, to match Luas.
// withPingPong connects both, pins ping, starts pong and calls measure.
// withPingPong stops pong after measure returns.
func withPingPong(ctx context.Context, cfg bench.Config, measure func(pingSide) error) (err error) {
	setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	pingClient, err := connect(cfg)
	if err != nil {
		return err
	}
	defer closeClient(pingClient, &err)
	pongClient, err := connect(cfg)
	if err != nil {
		return err
	}
	defer closeClient(pongClient, &err)

	pingPub, err := addPublication(pingClient, cfg, cfg.StreamID)
	if err != nil {
		return err
	}
	pongSub, err := pingClient.AddSubscription(cfg.Channel, cfg.PongStreamID)
	if err != nil {
		return fmt.Errorf("add pong subscription: %w", err)
	}
	pingSub, err := pongClient.AddSubscription(cfg.Channel, cfg.StreamID)
	if err != nil {
		return fmt.Errorf("add ping subscription: %w", err)
	}
	pongPub, err := addPublication(pongClient, cfg, cfg.PongStreamID)
	if err != nil {
		return err
	}
	if err := waitUntilConnected(
		setupCtx,
		[]*aeron.Publication{pingPub, pongPub},
		[]*aeron.Subscription{pingSub, pongSub},
	); err != nil {
		return err
	}
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()

	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	if err := cfg.PinHotThread(bench.PingThread); err != nil {
		return err
	}

	var worker bench.Worker
	pingSender := newSender(pingPub, cfg, &worker)
	pongSender := newSender(pongPub, cfg, &worker)
	pongDone := make(chan struct{})
	go func() {
		defer close(pongDone)
		pong(cfg, pingSub, pongSender, &worker)
	}()
	defer func() {
		worker.Stop()
		<-pongDone
	}()

	return measure(pingSide{sender: pingSender, subscription: pongSub, worker: &worker})
}

// pong runs on the pong goroutine. pong echoes each ping message on sender.
// pong returns after Stop, the first send error or a panic.
func pong(cfg bench.Config, sub *aeron.Subscription, sender *sender, worker *bench.Worker) {
	defer failOnPanic(worker)
	if err := cfg.PinHotThread(bench.PongThread); err != nil {
		worker.Fail(err)
		return
	}
	var sendErr error
	handler := func(buffer *atomic.Buffer, offset, length int32, _ *logbuffer.Header) {
		if sendErr == nil {
			sendErr = sender.send(buffer, offset, length)
		}
	}
	if cfg.IsFragmented() {
		handler = aeron.NewFragmentAssembler(
			handler,
			aeron.DefaultFragmentAssemblyBufferLength,
		).OnFragment
	}
	for !worker.IsStopped() {
		fragments := sub.Poll(handler, cfg.FragmentLimit)
		if sendErr != nil {
			worker.Fail(sendErr)
			return
		}
		idle.Idle(fragments)
	}
}
