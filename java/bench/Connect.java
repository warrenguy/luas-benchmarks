package bench;

import io.aeron.Aeron;
import io.aeron.Publication;
import io.aeron.Subscription;
import io.aeron.logbuffer.FragmentHandler;
import org.agrona.concurrent.UnsafeBuffer;

import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.concurrent.TimeUnit;
import java.util.function.BooleanSupplier;

/**
 * Connects to the driver in AERON_DIR. Sends one message over IPC. Polls the same stream for that message.
 */
final class Connect
{
    private static final String CHANNEL = "aeron:ipc";
    private static final int STREAM_ID = 1001;
    private static final long TIMEOUT_NS = TimeUnit.SECONDS.toNanos(10);
    private static final long IDLE_MS = 1;
    private static final byte[] PAYLOAD = "luas-benchmarks connect check".getBytes(StandardCharsets.US_ASCII);

    private Connect()
    {
    }

    /**
     * Runs the connect check. The connect check does not take flags.
     *
     * @param args flags after the scenario name.
     * @throws InterruptedException     when the thread is interrupted.
     * @throws IllegalArgumentException when args is not empty.
     */
    static void run(final String[] args) throws InterruptedException
    {
        if (args.length > 0)
        {
            throw new IllegalArgumentException("unexpected argument \"" + args[0] + "\"");
        }
        final String aeronDir = System.getenv("AERON_DIR");
        if (null == aeronDir || aeronDir.isEmpty())
        {
            throw new IllegalStateException("AERON_DIR not set");
        }
        final long deadlineNs = System.nanoTime() + TIMEOUT_NS;

        try (Aeron aeron = Aeron.connect(new Aeron.Context()
            .aeronDirectoryName(aeronDir)
            .driverTimeoutMs(TimeUnit.NANOSECONDS.toMillis(TIMEOUT_NS)));
            Publication publication = aeron.addPublication(CHANNEL, STREAM_ID);
            Subscription subscription = aeron.addSubscription(CHANNEL, STREAM_ID))
        {
            waitUntil(deadlineNs, "connected publication and image",
                () -> publication.isConnected() && subscription.isConnected());

            final UnsafeBuffer buffer = new UnsafeBuffer(PAYLOAD);
            final long[] lastResult = new long[1];
            try
            {
                waitUntil(deadlineNs, "offer", () ->
                {
                    lastResult[0] = publication.offer(buffer, 0, buffer.capacity());
                    return lastResult[0] > 0;
                });
            }
            catch (final IllegalStateException ex)
            {
                throw new IllegalStateException(ex.getMessage() + ": last result " + lastResult[0], ex);
            }

            final byte[][] received = new byte[1][];
            final FragmentHandler handler = (fragment, offset, length, header) ->
            {
                received[0] = new byte[length];
                fragment.getBytes(offset, received[0]);
            };
            waitUntil(deadlineNs, "poll", () -> subscription.poll(handler, 1) > 0);

            if (!Arrays.equals(received[0], PAYLOAD))
            {
                throw new IllegalStateException("payload mismatch: got \"" +
                    new String(received[0], StandardCharsets.US_ASCII) + "\", want \"" +
                    new String(PAYLOAD, StandardCharsets.US_ASCII) + "\"");
            }
            System.out.println("java connect: round-tripped " + received[0].length + " bytes");
        }
    }

    private static void waitUntil(final long deadlineNs, final String what, final BooleanSupplier isDone)
        throws InterruptedException
    {
        while (!isDone.getAsBoolean())
        {
            if (System.nanoTime() - deadlineNs > 0)
            {
                throw new IllegalStateException("wait for " + what + ": timeout");
            }
            Thread.sleep(IDLE_MS);
        }
    }
}
