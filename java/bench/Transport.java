package bench;

import io.aeron.Aeron;
import io.aeron.Publication;
import io.aeron.Subscription;
import io.aeron.logbuffer.BufferClaim;
import org.agrona.DirectBuffer;
import org.agrona.concurrent.BusySpinIdleStrategy;
import org.agrona.concurrent.IdleStrategy;

import java.util.List;
import java.util.concurrent.TimeUnit;

/**
 * Connect, publication and send code that the transport, archive and cluster scenarios share. Mirrors
 * luas/publication.go and aerongo/publication.go.
 */
final class Transport
{
    /**
     * Busy-spin idle strategy of every hot loop.
     */
    static final IdleStrategy IDLE = BusySpinIdleStrategy.INSTANCE;

    /**
     * Bound on the connect and the waits for connected publications and images.
     */
    static final long SETUP_TIMEOUT_NS = TimeUnit.SECONDS.toNanos(10);

    /**
     * Sleep between polls of a setup wait.
     */
    static final long SETUP_IDLE_MS = 1;

    private Transport()
    {
    }

    static Aeron connect(final Config config)
    {
        return Aeron.connect(new Aeron.Context()
            .aeronDirectoryName(config.aeronDir())
            .driverTimeoutMs(TimeUnit.NANOSECONDS.toMillis(SETUP_TIMEOUT_NS)));
    }

    /**
     * Adds the publication kind that config.publication names.
     *
     * @param aeron    client.
     * @param config   scenario flags.
     * @param streamId stream id.
     * @return the publication.
     */
    static Publication addPublication(final Aeron aeron, final Config config, final int streamId)
    {
        return config.isExclusive() ?
            aeron.addExclusivePublication(config.channel(), streamId) :
            aeron.addPublication(config.channel(), streamId);
    }

    /**
     * Waits until each publication has a subscriber and each subscription has an image.
     *
     * @param publications  publications.
     * @param subscriptions subscriptions.
     * @throws InterruptedException  when the thread is interrupted.
     * @throws IllegalStateException after SETUP_TIMEOUT_NS.
     */
    static void awaitConnected(final List<Publication> publications, final List<Subscription> subscriptions)
        throws InterruptedException
    {
        final long deadlineNs = System.nanoTime() + SETUP_TIMEOUT_NS;
        while (!publications.stream().allMatch(Publication::isConnected) ||
            !subscriptions.stream().allMatch(Subscription::isConnected))
        {
            if (System.nanoTime() - deadlineNs > 0)
            {
                throw new IllegalStateException("wait for connected publications and images: timeout");
            }
            Thread.sleep(SETUP_IDLE_MS);
        }
    }

    /**
     * Sends whole messages through offer or tryClaim, as config.sendMethod names. Retries back pressure, admin action
     * and not connected with a busy spin.
     */
    static final class Sender
    {
        private final Publication publication;
        private final boolean isTryClaim;
        private final BufferClaim claim = new BufferClaim();
        private final Worker worker;

        /**
         * Returns a sender on publication. The worker ends the retries: a stored worker failure or stop makes send
         * throw.
         *
         * @param publication publication.
         * @param config      scenario flags.
         * @param worker      worker that ends the retries.
         * @throws IllegalArgumentException when the send method is try-claim and the message does not fit one frame.
         */
        Sender(final Publication publication, final Config config, final Worker worker)
        {
            final int maxPayloadLength = publication.maxPayloadLength();
            isTryClaim = config.isTryClaim();
            if (isTryClaim && config.messageLength() > maxPayloadLength)
            {
                throw new IllegalArgumentException("try-claim: message length " + config.messageLength() +
                    " above max payload length " + maxPayloadLength);
            }
            this.publication = publication;
            this.worker = worker;
        }

        /**
         * Copies length bytes at offset in buffer into the publication. Does not allocate.
         *
         * @param buffer message buffer.
         * @param offset message offset.
         * @param length message length.
         * @throws IllegalStateException when the publication returns a terminal code, or the worker stops or fails
         *                               during a retry.
         */
        void send(final DirectBuffer buffer, final int offset, final int length)
        {
            while (!trySend(buffer, offset, length))
            {
                worker.throwIfFailed("worker");
                if (worker.isStopped())
                {
                    throw new IllegalStateException("worker stopped");
                }
                IDLE.idle();
            }
        }

        /**
         * Makes one offer or tryClaim call. Does not allocate.
         *
         * @param buffer message buffer.
         * @param offset message offset.
         * @param length message length.
         * @return false on back pressure, admin action or not connected.
         * @throws IllegalStateException when the publication returns a terminal code.
         */
        boolean trySend(final DirectBuffer buffer, final int offset, final int length)
        {
            final long position;
            if (isTryClaim)
            {
                position = publication.tryClaim(length, claim);
                if (position > 0)
                {
                    claim.putBytes(buffer, offset, length).commit();
                }
            }
            else
            {
                position = publication.offer(buffer, offset, length);
            }
            if (position > 0)
            {
                return true;
            }
            if (position == Publication.CLOSED || position == Publication.MAX_POSITION_EXCEEDED)
            {
                throw new IllegalStateException("send: " + Publication.errorString(position));
            }
            return false;
        }
    }
}
