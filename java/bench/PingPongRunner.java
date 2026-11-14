package bench;

import io.aeron.Aeron;
import io.aeron.FragmentAssembler;
import io.aeron.Publication;
import io.aeron.Subscription;
import io.aeron.logbuffer.FragmentHandler;
import org.agrona.BufferUtil;
import org.agrona.concurrent.UnsafeBuffer;

import java.util.List;

import static org.agrona.BitUtil.CACHE_LINE_LENGTH;

/**
 * Measures round trips between ping and pong. One message is in flight at a time. Mirrors runPingPong in
 * luas/ping_pong.go and aerongo/ping_pong.go.
 */
final class PingPongRunner
{
    /**
     * What the ping thread of a ping-pong pair uses: its sender on the stream id, its subscription to the echoes on
     * the pong stream id, and the worker that reports a pong failure.
     *
     * @param sender       ping sender.
     * @param subscription subscription to the echoes.
     * @param worker       reports a pong failure.
     */
    record PingSide(Transport.Sender sender, Subscription subscription, Worker worker)
    {
    }

    /**
     * Measures on the ping thread of a running ping-pong pair.
     */
    @FunctionalInterface
    interface PingRun
    {
        void run(PingSide ping) throws Exception;
    }

    private PingPongRunner()
    {
    }

    static void run(final Scenario scenario, final String[] args) throws Exception
    {
        final Config config = Config.parse(scenario, args);
        withPingPong(config, (ping) ->
        {
            final LatencyHistogram histogram = new LatencyHistogram();
            final PingTracker tracker = new PingTracker(histogram, config.messageLength());
            final FragmentHandler handler =
                (buffer, offset, length, header) -> tracker.reply(buffer.getLong(offset), length);
            final int length = config.messageLength();
            final UnsafeBuffer message =
                new UnsafeBuffer(BufferUtil.allocateDirectAligned(length, CACHE_LINE_LENGTH));
            final int fragmentLimit = config.fragmentLimit();
            final Transport.Sender pingSender = ping.sender();
            final Subscription pongSubscription = ping.subscription();
            final Worker worker = ping.worker();
            final Measure.Pass pass = (count) ->
            {
                for (long i = 0; i < count; i++)
                {
                    final long sentNs = System.nanoTime();
                    message.putLong(0, sentNs);
                    tracker.send(sentNs);
                    pingSender.send(message, 0, length);
                    while (tracker.isWaiting())
                    {
                        final int fragments = pongSubscription.poll(handler, fragmentLimit);
                        if (0 == fragments)
                        {
                            worker.throwIfFailed("pong");
                        }
                        Transport.IDLE.idle(fragments);
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
                scenario.name(),
                config.variant(),
                config,
                null,
                histogram.latency().withHistogram(histogram.encode()),
                measurement.jvmRuntime()).write();
        });
    }

    /**
     * Runs ping on the calling thread and pong on a second thread. Ping sends on the stream id. Pong echoes each
     * message on the pong stream id. Ping and pong each own one client, to match Luas. Connects both, pins ping,
     * starts pong and calls measure. Stops pong after measure returns. Mirrors withPingPong in luas/ping_pong.go and
     * aerongo/ping_pong.go.
     *
     * @param config  scenario flags.
     * @param measure measures on the ping thread.
     * @throws Exception when the setup or measure fails.
     */
    static void withPingPong(final Config config, final PingRun measure) throws Exception
    {
        try (Aeron pingAeron = Transport.connect(config);
            Aeron pongAeron = Transport.connect(config);
            Publication pingPublication = Transport.addPublication(pingAeron, config, config.streamId());
            Subscription pongSubscription = pingAeron.addSubscription(config.channel(), config.pongStreamId());
            Subscription pingSubscription = pongAeron.addSubscription(config.channel(), config.streamId());
            Publication pongPublication = Transport.addPublication(pongAeron, config, config.pongStreamId()))
        {
            Transport.awaitConnected(
                List.of(pingPublication, pongPublication), List.of(pingSubscription, pongSubscription));

            // Pin after connect in every client. A thread the client starts takes the CPU set of the thread that
            // starts it.
            config.pinHotThread(Scenario.PING_THREAD);

            final Worker worker = new Worker();
            final Transport.Sender pingSender = new Transport.Sender(pingPublication, config, worker);
            final Transport.Sender pongSender = new Transport.Sender(pongPublication, config, worker);
            final Thread pong = new Thread(() -> pong(config, pingSubscription, pongSender, worker), "pong");
            pong.start();
            try
            {
                measure.run(new PingSide(pingSender, pongSubscription, worker));
            }
            finally
            {
                worker.stop();
                pong.join();
            }
        }
    }

    /**
     * Runs on the pong thread. Echoes each ping message on sender. Returns after stop or the first send failure.
     */
    private static void pong(
        final Config config, final Subscription subscription, final Transport.Sender sender, final Worker worker)
    {
        try
        {
            config.pinHotThread(Scenario.PONG_THREAD);
            final FragmentHandler echo =
                (buffer, offset, length, header) -> sender.send(buffer, offset, length);
            final FragmentHandler handler = config.isFragmented() ? new FragmentAssembler(echo) : echo;
            final int fragmentLimit = config.fragmentLimit();
            while (!worker.isStopped())
            {
                Transport.IDLE.idle(subscription.poll(handler, fragmentLimit));
            }
        }
        catch (final Throwable ex)
        {
            worker.fail(ex);
        }
    }
}
