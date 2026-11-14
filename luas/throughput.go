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

// runThroughput measures one publisher goroutine that sends to one
// subscriber on the calling goroutine. The subscriber checks the sequence
// number and length of each message. The clock stops when the subscriber
// holds every message.
func runThroughput(ctx context.Context, scenario bench.Scenario, args []string) (err error) {
	cfg, err := bench.ParseFlagsWithUnlock(scenario, args)
	if err != nil {
		return err
	}
	setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	client, err := connect(setupCtx, cfg)
	if err != nil {
		return err
	}
	defer closeClient(client, &err)
	pub, err := addPublication(client, cfg, cfg.StreamID)
	if err != nil {
		return err
	}
	sub, err := client.AddSubscription(cfg.StreamID, cfg.Channel)
	if err != nil {
		return fmt.Errorf("add subscription: %w", err)
	}
	if err := waitUntilConnected(
		setupCtx,
		[]publication{pub},
		[]*aeron.Subscription{sub},
	); err != nil {
		return err
	}
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()

	var worker bench.Worker
	sender, err := newSender(pub, cfg, &worker)
	if err != nil {
		return err
	}
	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	publisherDone := make(chan struct{})
	if err := cfg.RunPinnedPair(bench.SubscriberThread, bench.PublisherThread, func() {
		defer close(publisherDone)
		publish(cfg, sender, &worker)
	}); err != nil {
		return err
	}
	defer func() {
		worker.Stop()
		<-publisherDone
	}()

	checker := bench.NewSequenceChecker(cfg.MessageLength)
	handler := func(buffer atomicbuffer.AtomicBuffer, offset, length int, _ *aeron.Header) {
		checker.Check(int64(binary.LittleEndian.Uint64(buffer.Bytes()[offset:])), length)
	}
	if cfg.IsFragmented() {
		handler = aeron.NewFragmentAssembler(handler).Handler()
	}
	duty := newConductorDuty(client, cfg, bench.SubscriberThread, bench.HotRepin{})
	pass := func(count int64) error {
		target := checker.Received() + count
		worker.Request(count)
		for checker.Received() < target {
			if err := checker.Err(); err != nil {
				return err
			}
			fragments := sub.Poll(handler, cfg.FragmentLimit)
			duty.tick()
			if fragments == 0 {
				if err := worker.Err(); err != nil {
					return fmt.Errorf("publisher: %w", err)
				}
			}
			idlestrategy.BusySpin.Idle(fragments)
		}
		return nil
	}

	measurement, err := bench.Measure(cfg, pass, pass)
	if err != nil {
		return err
	}
	throughput := bench.NewThroughput(cfg.Messages, cfg.MessageLength, measurement.ElapsedNs)
	return bench.WriteJSON(cfg.Output, bench.Result{
		Scenario:   scenario.Name,
		Client:     clientName,
		Variant:    cfg.Variant(),
		Config:     cfg,
		Throughput: &throughput,
		GoRuntime:  &measurement.GoRuntime,
	})
}

// publish runs on the publisher goroutine. publish sends each requested
// message with its sequence number in the first 8 bytes. publish returns
// after Stop or the first send error.
func publish(cfg bench.Config, sender *sender, worker *bench.Worker) {
	message := make([]byte, cfg.MessageLength)
	var sent int64
	for {
		target, ok := worker.Next(sent, spinIdle)
		if !ok {
			return
		}
		for ; sent < target; sent++ {
			binary.LittleEndian.PutUint64(message, uint64(sent))
			if err := sender.send(message); err != nil {
				worker.Fail(err)
				return
			}
		}
	}
}
