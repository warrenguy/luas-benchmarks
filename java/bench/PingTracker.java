package bench;

/**
 * Records ping-pong round trips. Mirrors bench.PingTracker.
 * <p>
 * Ping writes the send timestamp into the first 8 bytes of each message. Pong echoes the message. reply checks the
 * echo and records the round trip time.
 */
final class PingTracker
{
    private final LatencyHistogram histogram;
    private final int messageLength;
    private long sentNs;
    private boolean waiting;
    private String error;

    PingTracker(final LatencyHistogram histogram, final int messageLength)
    {
        this.histogram = histogram;
        this.messageLength = messageLength;
    }

    /**
     * Marks one ping in flight.
     *
     * @param sentNs send timestamp from System.nanoTime.
     */
    void send(final long sentNs)
    {
        this.sentNs = sentNs;
        waiting = true;
    }

    /**
     * Removes every recorded value.
     */
    void reset()
    {
        histogram.reset();
    }

    /**
     * Checks one echo and records System.nanoTime minus timestampNs. Stores an error when no ping is in flight, or
     * when the echo length or timestamp differs from the ping. Does not allocate while the echoes match.
     *
     * @param timestampNs timestamp in the echo.
     * @param length      echo length.
     */
    void reply(final long timestampNs, final int length)
    {
        final long nowNs = System.nanoTime();
        if (null != error)
        {
            return;
        }
        if (!waiting)
        {
            error = "reply without a ping in flight";
        }
        else if (length != messageLength)
        {
            error = "reply length " + length + ", want " + messageLength;
        }
        else if (timestampNs != sentNs)
        {
            error = "reply timestamp " + timestampNs + ", want " + sentNs;
        }
        else
        {
            waiting = false;
            final long roundTripNs = nowNs - timestampNs;
            try
            {
                histogram.record(roundTripNs);
            }
            catch (final ArrayIndexOutOfBoundsException ex)
            {
                error = "record round trip: " + roundTripNs + " ns out of range";
            }
        }
    }

    /**
     * Reports whether a ping is in flight and reply has not stored an error.
     *
     * @return true while ping waits for the echo.
     */
    boolean isWaiting()
    {
        return waiting && null == error;
    }

    /**
     * Throws the first error reply stored.
     *
     * @throws IllegalStateException when an echo did not match.
     */
    void throwIfFailed()
    {
        if (null != error)
        {
            throw new IllegalStateException(error);
        }
    }
}
