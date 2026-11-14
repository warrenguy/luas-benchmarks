package bench;

import io.aeron.Aeron;
import io.aeron.FragmentAssembler;
import io.aeron.Subscription;
import io.aeron.archive.client.AeronArchive;
import io.aeron.logbuffer.FragmentHandler;
import org.agrona.collections.Long2LongHashMap;

/**
 * Runs archive-replay. The archive first records the warmup and measured messages of one IPC publication, outside the
 * clock. Each pass then replays its part of the finished recording to one subscription on the calling thread. The
 * clock starts before the replay request. The clock stops when the subscriber holds the last message.
 */
final class ArchiveReplay
{
    private ArchiveReplay()
    {
    }

    static void run(final String[] args) throws Exception
    {
        final Config config = Config.parse(Scenario.ARCHIVE_REPLAY, args);

        try (Aeron aeron = Transport.connect(config);
            AeronArchive archive = Recorder.connectArchive(aeron))
        {
            // Recorded position at each pass boundary, keyed by message count: 0, the warmup count and the total.
            final Long2LongHashMap positions = new Long2LongHashMap(Aeron.NULL_VALUE);
            final long recordingId;
            try (Recorder recorder = new Recorder(config, aeron, archive))
            {
                recordingId = recorder.recordingId();
                positions.put(0, recorder.startPosition());
                final long[] counts = { config.warmupMessages(), config.warmupMessages() + config.messages() };
                for (final long count : counts)
                {
                    positions.put(count, recorder.record(count - recorder.sent()));
                }
            }

            try (Subscription subscription = aeron.addSubscription(config.channel(), Scenario.REPLAY_STREAM_ID))
            {
                // Pin after connect in every client. A thread the client starts takes the CPU set of the thread
                // that starts it.
                config.pinHotThread(Scenario.SUBSCRIBER_THREAD);

                final SequenceChecker checker = new SequenceChecker(config.messageLength());
                final FragmentHandler messageHandler =
                    (buffer, offset, length, header) -> checker.check(buffer.getLong(offset), length);
                final FragmentHandler handler = config.isFragmented() ?
                    new FragmentAssembler(messageHandler) : messageHandler;
                final int fragmentLimit = config.fragmentLimit();
                final Measure.Pass pass = (count) ->
                {
                    final long received = checker.received();
                    final long from = positions.get(received);
                    final long target = received + count;
                    archive.startReplay(
                        recordingId, from, positions.get(target) - from, config.channel(), Scenario.REPLAY_STREAM_ID);
                    while (checker.received() < target)
                    {
                        checker.throwIfFailed();
                        Transport.IDLE.idle(subscription.poll(handler, fragmentLimit));
                    }
                };

                final Measure.Measurement measurement = Measure.run(config, pass, pass);
                new Result(
                    Scenario.ARCHIVE_REPLAY.name(),
                    config.variant(),
                    config,
                    Throughput.of(config.messages(), config.messageLength(), measurement.elapsedNs()),
                    null,
                    measurement.jvmRuntime()).write();
            }
        }
    }
}
