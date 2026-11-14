package bench;

import io.aeron.Aeron;
import io.aeron.Publication;
import io.aeron.archive.client.AeronArchive;
import io.aeron.archive.codecs.SourceLocation;
import io.aeron.archive.status.RecordingPos;
import org.agrona.BufferUtil;
import org.agrona.concurrent.UnsafeBuffer;
import org.agrona.concurrent.status.CountersReader;

import java.util.List;

import static org.agrona.BitUtil.CACHE_LINE_LENGTH;

/**
 * Offers messages on a publication that the archive records locally. Each message holds its sequence number in the
 * first 8 bytes. The sequence continues over every record call. Mirrors the recorder in luas/archive.go and
 * aerongo/archive.go.
 */
final class Recorder implements AutoCloseable
{
    private final AeronArchive archive;
    private final Publication publication;
    private final Transport.Sender sender;
    private final String channel;
    private final int streamId;
    private final CountersReader counters;
    private final int counterId;
    private final long recordingId;
    private final long startPosition;
    private final UnsafeBuffer message;
    private final int length;
    private long sent;

    /**
     * Adds a publication on the config channel and stream. Then starts a local recording of the publication. Waits for
     * the recording position counter. Stops the recording when a later step fails.
     *
     * @param config  scenario flags.
     * @param aeron   client.
     * @param archive archive control session.
     * @throws Exception when the publication, the recording or the counter does not come up.
     */
    Recorder(final Config config, final Aeron aeron, final AeronArchive archive) throws Exception
    {
        this.archive = archive;
        channel = config.channel();
        streamId = config.streamId();
        length = config.messageLength();
        message = new UnsafeBuffer(BufferUtil.allocateDirectAligned(length, CACHE_LINE_LENGTH));
        counters = aeron.countersReader();
        publication = Transport.addPublication(aeron, config, streamId);
        try
        {
            sender = new Transport.Sender(publication, config, new Worker());
            archive.startRecording(channel, streamId, SourceLocation.LOCAL);
            try
            {
                counterId = awaitCounter(counters, publication.sessionId());
                recordingId = RecordingPos.getRecordingId(counters, counterId);
                Transport.awaitConnected(List.of(publication), List.of());
                startPosition = counters.getCounterValue(counterId);
            }
            catch (final Exception ex)
            {
                try
                {
                    archive.stopRecording(channel, streamId);
                }
                catch (final Exception stopEx)
                {
                    ex.addSuppressed(stopEx);
                }
                throw ex;
            }
        }
        catch (final Exception ex)
        {
            try
            {
                publication.close();
            }
            catch (final Exception closeEx)
            {
                ex.addSuppressed(closeEx);
            }
            throw ex;
        }
    }

    /**
     * Connects an archive control session on aeron. Every client uses the same control channels and a busy-spin idle
     * strategy.
     *
     * @param aeron client. The caller closes it after the session.
     * @return the archive control session.
     */
    static AeronArchive connectArchive(final Aeron aeron)
    {
        return AeronArchive.connect(new AeronArchive.Context()
            .aeron(aeron)
            .controlRequestChannel(Scenario.ARCHIVE_CONTROL_REQUEST_CHANNEL)
            .controlRequestStreamId(Scenario.ARCHIVE_CONTROL_REQUEST_STREAM_ID)
            .controlResponseChannel(Scenario.ARCHIVE_CONTROL_RESPONSE_CHANNEL)
            .controlResponseStreamId(Scenario.ARCHIVE_CONTROL_RESPONSE_STREAM_ID)
            .idleStrategy(Transport.IDLE));
    }

    /**
     * Returns the archive recording id of the publication.
     *
     * @return the recording id.
     */
    long recordingId()
    {
        return recordingId;
    }

    /**
     * Returns the recorded position before the first message.
     *
     * @return the start position.
     */
    long startPosition()
    {
        return startPosition;
    }

    /**
     * Returns the count of messages that record offered.
     *
     * @return the sent message count.
     */
    long sent()
    {
        return sent;
    }

    /**
     * Offers count messages. Then waits until the recorded position reaches the publication position. Does not
     * allocate.
     *
     * @param count messages to offer.
     * @return the recorded position after the last message.
     */
    long record(final long count)
    {
        for (long i = 0; i < count; i++)
        {
            message.putLong(0, sent);
            sender.send(message, 0, length);
            sent++;
        }
        final long target = publication.position();
        while (counters.getCounterValue(counterId) < target)
        {
            Transport.IDLE.idle();
        }
        return target;
    }

    /**
     * Stops the recording and closes the publication.
     */
    public void close()
    {
        try
        {
            archive.stopRecording(channel, streamId);
        }
        finally
        {
            publication.close();
        }
    }

    private static int awaitCounter(final CountersReader counters, final int sessionId) throws InterruptedException
    {
        final long deadlineNs = System.nanoTime() + Transport.SETUP_TIMEOUT_NS;
        while (true)
        {
            // Aeron.NULL_VALUE matches any archive. One archive runs on the driver.
            final int counterId = RecordingPos.findCounterIdBySession(counters, sessionId, Aeron.NULL_VALUE);
            if (CountersReader.NULL_COUNTER_ID != counterId)
            {
                return counterId;
            }
            if (System.nanoTime() - deadlineNs > 0)
            {
                throw new IllegalStateException("wait for recording position counter: timeout");
            }
            Thread.sleep(Transport.SETUP_IDLE_MS);
        }
    }
}
