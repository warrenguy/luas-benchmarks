package bench;

import io.aeron.FragmentAssembler;
import io.aeron.Subscription;
import io.aeron.logbuffer.FragmentHandler;
import org.agrona.BufferUtil;
import org.agrona.concurrent.UnsafeBuffer;

import static org.agrona.BitUtil.CACHE_LINE_LENGTH;

/**
 * Measures round trips between ping and pong at a fixed send rate. Ping sends on the schedule of FixedRateLoop. More
 * than one message can be in flight. Mirrors runFixedRate in luas/fixed_rate.go and aerongo/fixed_rate.go.
 */
final class FixedRateRunner
{
    private FixedRateRunner()
    {
    }

    static void run(final Scenario scenario, final String[] args) throws Exception
    {
        final Config config = Config.parse(scenario, args);
        PingPongRunner.withPingPong(config, (ping) ->
        {
            final LatencyHistogram histogram = new LatencyHistogram();
            final FixedRateTracker tracker = new FixedRateTracker(histogram, config.messageLength());
            final FragmentHandler messageHandler = (buffer, offset, length, header) ->
                tracker.reply(System.nanoTime(), buffer.getLong(offset), buffer.getLong(offset + 8), length);
            final FragmentHandler handler = config.isFragmented() ?
                new FragmentAssembler(messageHandler) : messageHandler;
            final Transceiver transceiver = new Transceiver(ping, handler, config);
            final FixedRateLoop loop = new FixedRateLoop(transceiver, tracker, ping.worker());
            final FixedRate[] fixedRate = new FixedRate[1];
            final Measure.Pass warmup = (count) -> loop.warmup(config.warmupRate(), count);
            final Measure.Pass measured = (count) -> fixedRate[0] = loop.run(config.rate(), count);

            final Measure.Measurement measurement = Measure.run(config, warmup, measured);
            new Result(
                scenario.name(),
                config.variant(),
                config,
                null,
                histogram.latency(),
                fixedRate[0].withHistogram(histogram.encode()),
                measurement.jvmRuntime()).write();
        });
    }

    /**
     * Sends fixed-rate messages through the ping sender and polls the echoes on the pong subscription.
     */
    private static final class Transceiver implements FixedRateLoop.Transceiver
    {
        private final Transport.Sender sender;
        private final Subscription subscription;
        private final FragmentHandler handler;
        private final int fragmentLimit;
        private final UnsafeBuffer message;

        Transceiver(final PingPongRunner.PingSide ping, final FragmentHandler handler, final Config config)
        {
            sender = ping.sender();
            subscription = ping.subscription();
            this.handler = handler;
            fragmentLimit = config.fragmentLimit();
            message = new UnsafeBuffer(BufferUtil.allocateDirectAligned(config.messageLength(), CACHE_LINE_LENGTH));
        }

        /**
         * Makes one send attempt. Does not allocate.
         */
        public boolean send(final long timestampNs, final long sequence)
        {
            message.putLong(0, timestampNs);
            message.putLong(8, sequence);
            return sender.trySend(message, 0, message.capacity());
        }

        public int poll()
        {
            return subscription.poll(handler, fragmentLimit);
        }

        public void idle(final int workCount)
        {
            Transport.IDLE.idle(workCount);
        }
    }
}
