package bench;

import java.util.concurrent.TimeUnit;

/**
 * Sends messages on a fixed schedule and polls the replies between sends. Mirrors bench.FixedRateLoop. Ports
 * LoadTestRig.send of the Aeron load test rig with a burst of 1.
 * <p>
 * Message k carries its scheduled send time start + k * interval. A late sender sends at once with that time, so the
 * recorded round trip includes the delay.
 */
final class FixedRateLoop
{
    /**
     * How long the loop keeps sending after the scheduled end of the run.
     */
    private static final long SEND_GRACE_NS = TimeUnit.MILLISECONDS.toNanos(100);

    /**
     * How long the loop waits for the last replies after it stops sending.
     */
    private static final long RECEIVE_DEADLINE_NS = TimeUnit.SECONDS.toNanos(3);

    private static final long NANOS_PER_SECOND = TimeUnit.SECONDS.toNanos(1);

    /**
     * Sends the messages of a fixed-rate run and polls the replies. Mirrors bench.FixedRateTransceiver.
     */
    interface Transceiver
    {
        /**
         * Makes one offer or tryClaim attempt. The message carries the scheduled send time timestampNs in bytes 0-7
         * and sequence in bytes 8-15, little endian.
         *
         * @param timestampNs scheduled send time.
         * @param sequence    sequence number.
         * @return false on back pressure, admin action or not connected.
         * @throws IllegalStateException when the publication returns a terminal code.
         */
        boolean send(long timestampNs, long sequence);

        /**
         * Polls the replies once. The reply handler passes each reply to the FixedRateTracker.
         *
         * @return the fragment count.
         */
        int poll();

        /**
         * Idles one busy-spin step after a pass that handled workCount fragments.
         *
         * @param workCount fragments of the pass.
         */
        void idle(int workCount);
    }

    private final Transceiver transceiver;
    private final FixedRateTracker tracker;
    private final Worker worker;

    /**
     * Returns a loop over transceiver.
     *
     * @param transceiver sends and polls.
     * @param tracker     records the replies.
     * @param worker      reports a pong failure.
     */
    FixedRateLoop(final Transceiver transceiver, final FixedRateTracker tracker, final Worker worker)
    {
        this.transceiver = transceiver;
        this.tracker = tracker;
        this.worker = worker;
    }

    /**
     * Sends messages at rate messages per second. Starts from an empty tracker. A shortfall is a result, not an
     * error. Mirrors bench.FixedRateLoop.Run: same order of sends, polls and deadlines.
     *
     * @param rate     messages per second.
     * @param messages message count of the schedule.
     * @return the counts, with an empty histogram.
     * @throws IllegalStateException when a send fails, a reply does not match or pong fails.
     */
    FixedRate run(final long rate, final long messages)
    {
        tracker.reset();
        final long intervalNs = NANOS_PER_SECOND / rate;
        final long startNs = System.nanoTime();
        final long stopNs = startNs + scheduleLengthNs(rate, messages);
        final long deadlineNs = stopNs + SEND_GRACE_NS;
        long sent = 0;
        long timestampNs = startNs;
        while (sent < messages)
        {
            long nowNs;
            if (transceiver.send(timestampNs, sent))
            {
                sent++;
                if (sent == messages)
                {
                    break;
                }
                timestampNs += intervalNs;
                nowNs = System.nanoTime();
                while (nowNs < timestampNs && nowNs < stopNs)
                {
                    final int fragments = tracker.received() < sent ? poll() : 0;
                    transceiver.idle(fragments);
                    nowNs = System.nanoTime();
                }
            }
            else
            {
                transceiver.idle(poll());
                nowNs = System.nanoTime();
            }
            if (nowNs >= deadlineNs)
            {
                break;
            }
            if (nowNs >= stopNs)
            {
                poll();
            }
        }

        final long receiveDeadlineNs = System.nanoTime() + RECEIVE_DEADLINE_NS;
        while (tracker.received() < sent)
        {
            final int fragments = poll();
            transceiver.idle(fragments);
            if (0 == fragments && System.nanoTime() >= receiveDeadlineNs)
            {
                break;
            }
        }
        return new FixedRate(messages, sent, tracker.received(), "");
    }

    /**
     * Runs run at rate over messages and drops the result.
     *
     * @param rate     messages per second.
     * @param messages message count of the schedule.
     * @throws IllegalStateException when run fails, or when a reply did not come back. A late reply would break the
     *                               sequence of the next run.
     */
    void warmup(final long rate, final long messages)
    {
        final FixedRate result = run(rate, messages);
        if (result.received() < result.sent())
        {
            throw new IllegalStateException("warmup: " + result.received() + " of " + result.sent() + " replies");
        }
    }

    private int poll()
    {
        final int fragments = transceiver.poll();
        tracker.throwIfFailed();
        if (0 == fragments)
        {
            worker.throwIfFailed("pong");
        }
        return fragments;
    }

    /**
     * Returns messages * 1e9 / rate, rounded down, without overflow for a rate up to Config.MAX_RATE.
     */
    private static long scheduleLengthNs(final long rate, final long messages)
    {
        return messages / rate * NANOS_PER_SECOND + messages % rate * NANOS_PER_SECOND / rate;
    }
}
