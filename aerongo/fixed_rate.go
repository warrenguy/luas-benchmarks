package main

import (
	"context"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	"github.com/lirm/aeron-go/aeron/logbuffer/term"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runFixedRate measures round trips between ping and pong at a fixed send
// rate. Ping sends on the schedule of bench.FixedRateLoop. More than one
// message can be in flight.
func runFixedRate(ctx context.Context, scenario bench.Scenario, args []string) error {
	cfg, err := bench.ParseFlags(scenario, args)
	if err != nil {
		return err
	}
	return withPingPong(ctx, cfg, func(ping pingSide) error {
		histogram := bench.NewHistogram()
		tracker := bench.NewFixedRateTracker(histogram, cfg.MessageLength)
		handler := func(buffer *atomic.Buffer, offset, length int32, _ *logbuffer.Header) {
			tracker.Reply(bench.NanoTime(), buffer.GetInt64(offset), buffer.GetInt64(offset+8), int(length))
		}
		if cfg.IsFragmented() {
			handler = aeron.NewFragmentAssembler(
				handler,
				aeron.DefaultFragmentAssemblyBufferLength,
			).OnFragment
		}
		transceiver := &fixedRateTransceiver{
			sender:        ping.sender,
			subscription:  ping.subscription,
			fragmentLimit: cfg.FragmentLimit,
			message:       atomic.MakeBuffer(make([]byte, cfg.MessageLength)),
			handler:       handler,
		}
		loop := bench.FixedRateLoop{
			Transceiver: transceiver,
			Tracker:     tracker,
			Worker:      ping.worker,
			Clock:       bench.NanoTime,
		}
		var fixedRate bench.FixedRate
		warmup := func(count int64) (err error) {
			defer recoverPanic(&err)
			return loop.Warmup(cfg.WarmupRate, count)
		}
		measured := func(count int64) (err error) {
			defer recoverPanic(&err)
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
	handler       term.FragmentHandler
	fragmentLimit int
	message       *atomic.Buffer
}

// Send makes one send attempt. Send does not allocate.
//
// Returns an error when the publication returns a terminal code.
func (t *fixedRateTransceiver) Send(timestampNs, sequence int64) (bool, error) {
	t.message.PutInt64(0, timestampNs)
	t.message.PutInt64(8, sequence)
	return t.sender.trySend(t.message, 0, t.message.Capacity())
}

func (t *fixedRateTransceiver) Poll() int {
	return t.subscription.Poll(t.handler, t.fragmentLimit)
}

func (t *fixedRateTransceiver) Idle(workCount int) {
	idle.Idle(workCount)
}
