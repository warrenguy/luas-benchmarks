package bench;

import io.aeron.Aeron;
import io.aeron.archive.client.AeronArchive;

/**
 * Runs archive-record: the archive records one IPC publication locally. The calling thread offers the messages. The
 * clock stops when the recorded position reaches the publication position.
 */
final class ArchiveRecord
{
    private ArchiveRecord()
    {
    }

    static void run(final String[] args) throws Exception
    {
        final Config config = Config.parse(Scenario.ARCHIVE_RECORD, args);

        try (Aeron aeron = Transport.connect(config);
            AeronArchive archive = Recorder.connectArchive(aeron);
            Recorder recorder = new Recorder(config, aeron, archive))
        {
            // Pin after connect in every client. A thread the client starts takes the CPU set of the thread that
            // starts it.
            config.pinHotThread(Scenario.PUBLISHER_THREAD);

            final Measure.Pass pass = recorder::record;
            final Measure.Measurement measurement = Measure.run(config, pass, pass);
            new Result(
                Scenario.ARCHIVE_RECORD.name(),
                config.variant(),
                config,
                Throughput.of(config.messages(), config.messageLength(), measurement.elapsedNs()),
                null,
                measurement.jvmRuntime()).write();
        }
    }
}
