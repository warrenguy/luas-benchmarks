package main

import (
	"context"
	"fmt"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runThroughput measures one publisher goroutine that sends to one
// subscriber on the calling goroutine. The subscriber checks the sequence
// number and length of each message. The clock stops when the subscriber
// holds every message.
func runThroughput(ctx context.Context, scenario bench.Scenario, args []string) (err error) {
	cfg, err := bench.ParseFlags(scenario, args)
	if err != nil {
		return err
	}
	setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	client, err := connect(cfg)
	if err != nil {
		return err
	}
	defer closeClient(client, &err)
	pub, err := addPublication(client, cfg, cfg.StreamID)
	if err != nil {
		return err
	}
	sub, err := client.AddSubscription(cfg.Channel, cfg.StreamID)
	if err != nil {
		return fmt.Errorf("add subscription: %w", err)
	}
	if err := waitUntilConnected(
		setupCtx,
		[]*aeron.Publication{pub},
		[]*aeron.Subscription{sub},
	); err != nil {
		return err
	}
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()

	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	if err := cfg.PinHotThread(bench.SubscriberThread); err != nil {
		return err
	}

	var worker bench.Worker
	sender := newSender(pub, cfg, &worker)
	publisherDone := make(chan struct{})
	go func() {
		defer close(publisherDone)
		publish(cfg, sender, &worker)
	}()
	defer func() {
		worker.Stop()
		<-publisherDone
	}()

	checker := bench.NewSequenceChecker(cfg.MessageLength)
	handler := func(buffer *atomic.Buffer, offset, length int32, _ *logbuffer.Header) {
		checker.Check(buffer.GetInt64(offset), int(length))
	}
	if cfg.IsFragmented() {
		handler = aeron.NewFragmentAssembler(
			handler,
			aeron.DefaultFragmentAssemblyBufferLength,
		).OnFragment
	}
	pass := func(count int64) error {
		target := checker.Received() + count
		worker.Request(count)
		for checker.Received() < target {
			if err := checker.Err(); err != nil {
				return err
			}
			fragments := sub.Poll(handler, cfg.FragmentLimit)
			if fragments == 0 {
				if err := worker.Err(); err != nil {
					return fmt.Errorf("publisher: %w", err)
				}
			}
			idle.Idle(fragments)
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
// after Stop, the first send error or a panic.
func publish(cfg bench.Config, sender *sender, worker *bench.Worker) {
	defer failOnPanic(worker)
	if err := cfg.PinHotThread(bench.PublisherThread); err != nil {
		worker.Fail(err)
		return
	}
	message := atomic.MakeBuffer(make([]byte, cfg.MessageLength))
	length := message.Capacity()
	var sent int64
	for {
		target, ok := worker.Next(sent, spinIdle)
		if !ok {
			return
		}
		for ; sent < target; sent++ {
			message.PutInt64(0, sent)
			if err := sender.send(message, 0, length); err != nil {
				worker.Fail(err)
				return
			}
		}
	}
}
