package bench;

/**
 * Round trip latency percentiles in nanoseconds. Mirrors bench.Latency.
 *
 * @param count    recorded values.
 * @param p50Ns    50th percentile.
 * @param p90Ns    90th percentile.
 * @param p99Ns    99th percentile.
 * @param p999Ns   99.9th percentile.
 * @param p9999Ns  99.99th percentile.
 * @param p99999Ns 99.999th percentile.
 * @param maxNs    highest recorded value.
 * @param histogram round trip histogram of a ping-pong or cluster scenario in the HdrHistogram V2 compressed format,
 *                  base64 encoded. Null in a fixed-rate result, where FixedRate holds it.
 */
record Latency(
    long count,
    long p50Ns,
    long p90Ns,
    long p99Ns,
    long p999Ns,
    long p9999Ns,
    long p99999Ns,
    long maxNs,
    String histogram)
{
    Latency withHistogram(final String histogram)
    {
        return new Latency(count, p50Ns, p90Ns, p99Ns, p999Ns, p9999Ns, p99999Ns, maxNs, histogram);
    }

    Json toJson()
    {
        final Json json = new Json()
            .field("count", count)
            .field("p50_ns", p50Ns)
            .field("p90_ns", p90Ns)
            .field("p99_ns", p99Ns)
            .field("p99_9_ns", p999Ns)
            .field("p99_99_ns", p9999Ns)
            .field("p99_999_ns", p99999Ns)
            .field("max_ns", maxNs);
        if (null != histogram)
        {
            json.field("histogram", histogram);
        }
        return json;
    }
}
