package bench;

/**
 * Prints the Java HdrHistogram percentiles and the encoded histogram for the fixed input of
 * bench/histogram_test.go. make histogram-fixture runs it and writes
 * bench/testdata/histogram-java.json.
 */
public final class HistogramFixture
{
    /**
     * Input size. Each of p90, p99 and p99.9 times this count has a fraction below one half, so ceil and round pick
     * different ranks.
     */
    static final int VALUES = 10_789;

    private HistogramFixture()
    {
    }

    public static void main(final String[] args)
    {
        final LatencyHistogram histogram = new LatencyHistogram();
        long state = 1;
        for (int i = 0; i < VALUES; i++)
        {
            state = state * 6364136223846793005L + 1442695040888963407L;
            histogram.record((100 + (state >>> 33) % 10_000) << ((state >>> 60) % 12));
        }
        System.out.println(new Json()
            .field("latency", histogram.latency().toJson())
            .field("histogram", histogram.encode()));
    }
}
