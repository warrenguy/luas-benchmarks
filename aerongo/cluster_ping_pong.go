package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	clusterclient "github.com/lirm/aeron-go/cluster/client"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runClusterPingPong runs cluster-ping-pong: round trips through the Java echo
// cluster. The calling goroutine offers each ping on cluster ingress. The
// calling goroutine then polls egress for the echo. One message is in flight
// at a time.
func runClusterPingPong(ctx context.Context, args []string) (err error) {
	cfg, err := bench.ParseFlags(bench.ClusterPingPong, args)
	if err != nil {
		return err
	}
	setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	histogram := bench.NewHistogram()
	listener := &pingListener{tracker: bench.NewPingTracker(histogram, cfg.MessageLength)}
	options := clusterclient.NewOptions()
	options.IngressChannel = cfg.Channel
	options.IngressEndpoints = bench.ClusterIngressEndpoints(cfg.ClusterMembers)
	options.IngressStreamId = cfg.StreamID
	options.IsIngressExclusive = cfg.Publication == bench.ExclusivePublication
	options.EgressChannel = bench.ClusterEgressChannel
	options.EgressStreamId = cfg.PongStreamID
	options.IdleStrategy = idle
	cluster, err := clusterclient.NewAeronCluster(
		aeron.NewContext().AeronDir(cfg.AeronDir).MediaDriverTimeout(setupTimeout),
		options,
		listener,
	)
	if err != nil {
		return fmt.Errorf("connect cluster: %w", err)
	}
	defer cluster.Close()
	if err := waitUntil(setupCtx, "cluster session", func() bool {
		cluster.Poll()
		return cluster.IsConnected() || listener.err != nil
	}); err != nil {
		return err
	}
	if listener.err != nil {
		return fmt.Errorf("connect cluster: %w", listener.err)
	}
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()

	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	if err := cfg.PinHotThread(bench.PingThread); err != nil {
		return err
	}

	tracker := listener.tracker
	message := atomic.MakeBuffer(make([]byte, cfg.MessageLength))
	length := message.Capacity()
	pass := func(count int64) (err error) {
		defer recoverPanic(&err)
		for range count {
			sentNs := bench.NanoTime()
			message.PutInt64(0, sentNs)
			tracker.Send(sentNs)
			if err := offerIngress(cluster, message, length); err != nil {
				return err
			}
			for tracker.IsWaiting() {
				work := cluster.Poll()
				if work == 0 {
					if err := listener.failure(cluster); err != nil {
						return err
					}
				}
				idle.Idle(work)
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

// offerIngress offers length bytes of message on cluster ingress.
// offerIngress retries back pressure and admin action with a busy spin.
// offerIngress polls egress between attempts.
//
// Returns an error when Offer returns any other failure code.
func offerIngress(cluster *clusterclient.AeronCluster, message *atomic.Buffer, length int32) error {
	for {
		position := cluster.Offer(message, 0, length)
		if position > 0 {
			return nil
		}
		if position != aeron.BackPressured && position != aeron.AdminAction {
			return fmt.Errorf("offer: result %d", position)
		}
		cluster.Poll()
		spinIdle()
	}
}

// pingListener passes each echo to the tracker. pingListener stores the first
// disconnect or error event.
//
// Concurrency: invoker-only. The goroutine that calls AeronCluster.Poll
// receives every event.
type pingListener struct {
	tracker *bench.PingTracker
	err     error
}

var _ clusterclient.EgressListener = (*pingListener)(nil)

func (l *pingListener) OnConnect(*clusterclient.AeronCluster) {}

func (l *pingListener) OnDisconnect(_ *clusterclient.AeronCluster, details string) {
	if l.err == nil {
		l.err = fmt.Errorf("cluster disconnected: %s", details)
	}
}

func (l *pingListener) OnMessage(
	_ *clusterclient.AeronCluster,
	_ int64,
	buffer *atomic.Buffer,
	offset int32,
	length int32,
	_ *logbuffer.Header,
) {
	l.tracker.Reply(buffer.GetInt64(offset), int(length))
}

func (l *pingListener) OnNewLeader(*clusterclient.AeronCluster, int64, int32) {}

func (l *pingListener) OnError(_ *clusterclient.AeronCluster, details string) {
	if l.err == nil {
		l.err = fmt.Errorf("cluster error: %s", details)
	}
}

// failure returns the stored event error, or an error when cluster closed.
// Returns nil otherwise.
func (l *pingListener) failure(cluster *clusterclient.AeronCluster) error {
	if l.err != nil {
		return l.err
	}
	if cluster.IsClosed() {
		return errors.New("cluster session closed")
	}
	return nil
}
