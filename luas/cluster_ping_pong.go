package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/warrenguy/luas/aeron"
	"github.com/warrenguy/luas/atomicbuffer"
	clusterclient "github.com/warrenguy/luas/cluster/client"
	"github.com/warrenguy/luas/idlestrategy"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runClusterPingPong runs cluster-ping-pong: round trips through the Java echo
// cluster. The calling goroutine offers each ping on cluster ingress. The
// calling goroutine then polls egress for the echo. One message is in flight
// at a time.
func runClusterPingPong(ctx context.Context, args []string) (err error) {
	cfg, err := bench.ParseFlagsWithUnlock(bench.ClusterPingPong, args)
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

	histogram := bench.NewHistogram()
	tracker := bench.NewPingTracker(histogram, cfg.MessageLength)
	exclusive := cfg.Publication == bench.ExclusivePublication
	cluster, err := clusterclient.Connect(setupCtx, clusterclient.Config{
		Aeron:            client,
		IngressChannel:   cfg.Channel,
		IngressEndpoints: bench.ClusterIngressEndpoints(cfg.ClusterMembers),
		IngressStreamID:  cfg.StreamID,
		IngressExclusive: &exclusive,
		EgressChannel:    bench.ClusterEgressChannel,
		EgressStreamID:   cfg.PongStreamID,
		IdleStrategy:     idlestrategy.BusySpin,
		EgressHandler: &clusterclient.EgressHandlers{
			MessageFunc: func(
				_ int64,
				_ int64,
				buffer atomicbuffer.AtomicBuffer,
				offset int,
				length int,
				_ *aeron.Header,
			) {
				tracker.Reply(int64(binary.LittleEndian.Uint64(buffer.Bytes()[offset:])), length)
			},
		},
	})
	if err != nil {
		return fmt.Errorf("connect cluster: %w", err)
	}
	defer func() { err = errors.Join(err, cluster.Close()) }()
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()

	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	if err := cfg.PinHotThread(bench.PingThread); err != nil {
		return err
	}

	message := make([]byte, cfg.MessageLength)
	duty := newConductorDuty(client, cfg, bench.PingThread, bench.HotRepin{})
	pass := func(count int64) error {
		for range count {
			sentNs := bench.NanoTime()
			binary.LittleEndian.PutUint64(message, uint64(sentNs))
			tracker.Send(sentNs)
			if err := offerIngress(cluster, &duty, message); err != nil {
				return err
			}
			for tracker.IsWaiting() {
				work := cluster.PollEgress()
				duty.tick()
				if work == 0 && cluster.IsClosed() {
					return errors.New("cluster session closed")
				}
				idlestrategy.BusySpin.Idle(work)
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
		Scenario:  bench.ClusterPingPong.Name,
		Client:    clientName,
		Variant:   cfg.Variant(),
		Config:    cfg,
		Latency:   &latency,
		GoRuntime: &measurement.GoRuntime,
	})
}

// offerIngress offers message on cluster ingress. offerIngress retries back
// pressure and admin action with a busy spin. offerIngress polls egress
// between attempts. offerIngress does not allocate.
//
// Returns an error when Offer returns any other failure code.
func offerIngress(cluster *clusterclient.AeronCluster, duty *conductorDuty, message []byte) error {
	for {
		position := cluster.Offer(message)
		if position > 0 {
			return nil
		}
		if !aeron.IsIngressRetriable(position) {
			return fmt.Errorf("offer: %s", aeron.PositionCodeName(position))
		}
		cluster.PollEgress()
		duty.tick()
		spinIdle()
	}
}
