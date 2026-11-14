package bench;

import io.aeron.Aeron;
import io.aeron.Publication;
import io.aeron.cluster.client.AeronCluster;
import io.aeron.cluster.client.EgressListener;
import org.agrona.BufferUtil;
import org.agrona.concurrent.UnsafeBuffer;

import static org.agrona.BitUtil.CACHE_LINE_LENGTH;

/**
 * Runs cluster-ping-pong: round trips through the Java echo cluster. The calling thread offers each ping on cluster
 * ingress. The calling thread then polls egress for the echo. One message is in flight at a time. Mirrors
 * runClusterPingPong in luas/cluster_ping_pong.go and aerongo/cluster_ping_pong.go.
 */
final class ClusterPingPong
{
    private ClusterPingPong()
    {
    }

    static void run(final String[] args) throws Exception
    {
        final Config config = Config.parse(Scenario.CLUSTER_PING_PONG, args);
        final LatencyHistogram histogram = new LatencyHistogram();
        final PingTracker tracker = new PingTracker(histogram, config.messageLength());
        final EgressListener listener =
            (clusterSessionId, timestamp, buffer, offset, length, header) ->
            tracker.reply(buffer.getLong(offset), length);

        try (Aeron aeron = Transport.connect(config);
            AeronCluster cluster = AeronCluster.connect(new AeronCluster.Context()
                .aeron(aeron)
                .ownsAeronClient(false)
                .ingressChannel(config.channel())
                .ingressEndpoints(Scenario.clusterIngressEndpoints(config.clusterMembers()))
                .ingressStreamId(config.streamId())
                .isIngressExclusive(config.isExclusive())
                .egressChannel(Scenario.CLUSTER_EGRESS_CHANNEL)
                .egressStreamId(config.pongStreamId())
                .idleStrategy(Transport.IDLE)
                .egressListener(listener)))
        {
            // Pin after connect in every client. A thread the client starts takes the CPU set of the thread that
            // starts it.
            config.pinHotThread(Scenario.PING_THREAD);

            final int length = config.messageLength();
            final UnsafeBuffer message = new UnsafeBuffer(BufferUtil.allocateDirectAligned(length, CACHE_LINE_LENGTH));
            final Measure.Pass pass = (count) ->
            {
                for (long i = 0; i < count; i++)
                {
                    final long sentNs = System.nanoTime();
                    message.putLong(0, sentNs);
                    tracker.send(sentNs);
                    offerIngress(cluster, message, length);
                    while (tracker.isWaiting())
                    {
                        final int work = cluster.pollEgress();
                        if (0 == work && cluster.isClosed())
                        {
                            throw new IllegalStateException("cluster session closed");
                        }
                        Transport.IDLE.idle(work);
                    }
                    tracker.throwIfFailed();
                }
            };
            final Measure.Pass warmup = (count) ->
            {
                try
                {
                    pass.run(count);
                }
                finally
                {
                    tracker.reset();
                }
            };

            final Measure.Measurement measurement = Measure.run(config, warmup, pass);
            new Result(
                Scenario.CLUSTER_PING_PONG.name(),
                config.variant(),
                config,
                null,
                histogram.latency().withHistogram(histogram.encode()),
                measurement.jvmRuntime()).write();
        }
    }

    /**
     * Offers length bytes of message on cluster ingress. Retries back pressure and admin action with a busy spin.
     * Polls egress between attempts. Does not allocate.
     *
     * @throws IllegalStateException when offer returns any other failure code.
     */
    private static void offerIngress(final AeronCluster cluster, final UnsafeBuffer message, final int length)
    {
        while (true)
        {
            final long position = cluster.offer(message, 0, length);
            if (position > 0)
            {
                return;
            }
            if (position != Publication.BACK_PRESSURED && position != Publication.ADMIN_ACTION)
            {
                throw new IllegalStateException("offer: " + Publication.errorString(position));
            }
            cluster.pollEgress();
            Transport.IDLE.idle();
        }
    }
}
