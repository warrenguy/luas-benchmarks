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
 * Measures one publisher thread that sends to one subscriber on the calling thread. The subscriber checks the
 * sequence number and length of each message. The clock stops when the subscriber holds every message. Mirrors
 * runThroughput in luas/throughput.go and aerongo/throughput.go.
 */
final class ThroughputRunner
{
    private ThroughputRunner()
    {
    }

    static void run(final Scenario scenario, final String[] args) throws Exception
    {
        final Config config = Config.parse(scenario, args);

        try (Aeron aeron = Transport.connect(config);
            Publication publication = Transport.addPublication(aeron, config, config.streamId());
            Subscription subscription = aeron.addSubscription(config.channel(), config.streamId()))
        {
            Transport.awaitConnected(List.of(publication), List.of(subscription));

            // Pin after connect in every client. A thread the client starts takes the CPU set of the thread that
            // starts it.
            config.pinHotThread(Scenario.SUBSCRIBER_THREAD);

            final Worker worker = new Worker();
            final Transport.Sender sender = new Transport.Sender(publication, config, worker);
            final Thread publisher = new Thread(() -> publish(config, sender, worker), "publisher");
            publisher.start();
            try
            {
                final SequenceChecker checker = new SequenceChecker(config.messageLength());
                final FragmentHandler messageHandler =
                    (buffer, offset, length, header) -> checker.check(buffer.getLong(offset), length);
                final FragmentHandler handler = config.isFragmented() ?
                    new FragmentAssembler(messageHandler) : messageHandler;
                final int fragmentLimit = config.fragmentLimit();
                final Measure.Pass pass = (count) ->
                {
                    final long target = checker.received() + count;
                    worker.request(count);
                    while (checker.received() < target)
                    {
                        checker.throwIfFailed();
                        final int fragments = subscription.poll(handler, fragmentLimit);
                        if (0 == fragments)
                        {
                            worker.throwIfFailed("publisher");
                        }
                        Transport.IDLE.idle(fragments);
                    }
                };

                final Measure.Measurement measurement = Measure.run(config, pass, pass);
                new Result(
                    scenario.name(),
                    config.variant(),
                    config,
                    Throughput.of(config.messages(), config.messageLength(), measurement.elapsedNs()),
                    null,
                    measurement.jvmRuntime()).write();
            }
            finally
            {
                worker.stop();
                publisher.join();
            }
        }
    }

    /**
     * Runs on the publisher thread. Sends each requested message with its sequence number in the first 8 bytes.
     * Returns after stop or the first send failure.
     */
    private static void publish(final Config config, final Transport.Sender sender, final Worker worker)
    {
        try
        {
            config.pinHotThread(Scenario.PUBLISHER_THREAD);
            final int length = config.messageLength();
            final UnsafeBuffer message = new UnsafeBuffer(BufferUtil.allocateDirectAligned(length, CACHE_LINE_LENGTH));
            long sent = 0;
            while (true)
            {
                final long target = worker.next(sent);
                if (target < 0)
                {
                    return;
                }
                for (; sent < target; sent++)
                {
                    message.putLong(0, sent);
                    sender.send(message, 0, length);
                }
            }
        }
        catch (final Throwable ex)
        {
            worker.fail(ex);
        }
    }
}
