package main

import (
	"context"
	"encoding/binary"

	"github.com/warrenguy/luas/aeron"
	"github.com/warrenguy/luas/atomicbuffer"
	"github.com/warrenguy/luas/idlestrategy"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runFixedRate measures round trips between ping and pong at a fixed send
// rate. Ping sends on the schedule of bench.FixedRateLoop. More than one
// message can be in flight.
func runFixedRate(ctx context.Context, scenario bench.Scenario, args []string) error {
	cfg, err := bench.ParseFlagsWithUnlock(scenario, args)
	if err != nil {
		return err
	}
	return withPingPong(ctx, cfg, func(ping pingSide) error {
		histogram := bench.NewHistogram()
		tracker := bench.NewFixedRateTracker(histogram, cfg.MessageLength)
		handler := func(buffer atomicbuffer.AtomicBuffer, offset, length int, _ *aeron.Header) {
			payload := buffer.Bytes()[offset : offset+bench.MinFixedRateMessageLength]
			tracker.Reply(
				bench.NanoTime(),
				int64(binary.LittleEndian.Uint64(payload)),
				int64(binary.LittleEndian.Uint64(payload[8:])),
				length,
			)
		}
		if cfg.IsFragmented() {
			handler = aeron.NewFragmentAssembler(handler).Handler()
		}
		transceiver := &fixedRateTransceiver{
			sender:        ping.sender,
			subscription:  ping.subscription,
			fragmentLimit: cfg.FragmentLimit,
			message:       make([]byte, cfg.MessageLength),
			duty: newConductorDuty(
				ping.client,
				cfg,
				bench.PingThread,
				cfg.HotRepin(bench.PingThread),
			),
			handler: handler,
		}
		loop := bench.FixedRateLoop{
			Transceiver: transceiver,
			Tracker:     tracker,
			Worker:      ping.worker,
			Clock:       bench.NanoTime,
		}
		var fixedRate bench.FixedRate
		warmup := func(count int64) error {
			return loop.Warmup(cfg.WarmupRate, count)
		}
		measured := func(count int64) (err error) {
			fixedRate, err = loop.Run(cfg.Rate, count)
			return err
		}

		measurement, err := bench.Measure(cfg, warmup, measured)
		if err != nil {
			return err
		}
		latency := histogram.Latency()
		if fixedRate.Histogram, err = histogram.Encode(); err != nil {
			return err
		}
		return bench.WriteJSON(cfg.Output, bench.Result{
			Scenario:  scenario.Name,
			Client:    clientName,
			Variant:   cfg.Variant(),
			Config:    cfg,
			Latency:   &latency,
			FixedRate: &fixedRate,
			GoRuntime: &measurement.GoRuntime,
		})
	})
}

// fixedRateTransceiver sends fixed-rate messages through the ping sender and
// polls the echoes on the pong subscription.
type fixedRateTransceiver struct {
	sender        *sender
	subscription  *aeron.Subscription
	handler       aeron.FragmentHandler
	fragmentLimit int
	message       []byte
	duty          conductorDuty
}

// Send makes one send attempt. Send does not allocate.
//
// Returns an error when the publication returns a terminal code or a claim
// fails.
func (t *fixedRateTransceiver) Send(timestampNs, sequence int64) (bool, error) {
	header := t.message[:bench.MinFixedRateMessageLength]
	binary.LittleEndian.PutUint64(header, uint64(timestampNs))
	binary.LittleEndian.PutUint64(header[8:], uint64(sequence))
	return t.sender.trySend(t.message)
}

func (t *fixedRateTransceiver) Poll() int {
	return t.subscription.Poll(t.handler, t.fragmentLimit)
}

// Idle counts one conductor duty tick, then idles one busy-spin step.
func (t *fixedRateTransceiver) Idle(workCount int) {
	t.duty.tick()
	idlestrategy.BusySpin.Idle(workCount)
}
