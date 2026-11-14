package bench;

import org.HdrHistogram.Histogram;

import java.nio.ByteBuffer;
import java.util.Arrays;
import java.util.Base64;
import java.util.concurrent.TimeUnit;

/**
 * Records latencies in nanoseconds with 3 significant digits. Mirrors bench.Histogram.
 */
final class LatencyHistogram
{
    static final long LOWEST_DISCERNIBLE_NS = 1;
    static final long HIGHEST_TRACKABLE_NS = TimeUnit.HOURS.toNanos(1);
    static final int SIGNIFICANT_DIGITS = 3;

    private final Histogram histogram =
        new Histogram(LOWEST_DISCERNIBLE_NS, HIGHEST_TRACKABLE_NS, SIGNIFICANT_DIGITS);

    /**
     * Records one value. Does not allocate.
     *
     * @param valueNs latency in nanoseconds, from 0 to HIGHEST_TRACKABLE_NS.
     * @throws ArrayIndexOutOfBoundsException when valueNs is out of range.
     */
    void record(final long valueNs)
    {
        histogram.recordValue(valueNs);
    }

    void reset()
    {
        histogram.reset();
    }

    Latency latency()
    {
        return new Latency(
            histogram.getTotalCount(),
            histogram.getValueAtPercentile(50.0),
            histogram.getValueAtPercentile(90.0),
            histogram.getValueAtPercentile(99.0),
            histogram.getValueAtPercentile(99.9),
            histogram.getValueAtPercentile(99.99),
            histogram.getValueAtPercentile(99.999),
            histogram.getMaxValue(),
            null);
    }

    /**
     * Returns the histogram in the HdrHistogram V2 compressed format, base64 encoded. Mirrors bench.Histogram.Encode
     *. Allocates.
     *
     * @return the encoded histogram.
     */
    String encode()
    {
        final ByteBuffer buffer = ByteBuffer.allocate(histogram.getNeededByteBufferCapacity());
        final int length = histogram.encodeIntoCompressedByteBuffer(buffer);
        return Base64.getEncoder().encodeToString(Arrays.copyOf(buffer.array(), length));
    }
}
