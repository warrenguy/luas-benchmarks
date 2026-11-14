package bench;

/**
 * Rate of one measured section. Mirrors bench.Throughput.
 *
 * @param messages           measured messages.
 * @param bytes              measured payload bytes.
 * @param elapsedNs          measured section length.
 * @param messagesPerSecond  message rate.
 * @param megabytesPerSecond payload rate, 10^6 bytes per megabyte.
 */
record Throughput(long messages, long bytes, long elapsedNs, double messagesPerSecond, double megabytesPerSecond)
{
    /**
     * Returns the rate of messages of messageLength bytes over elapsedNs. The rates are 0 when elapsedNs is not
     * positive.
     */
    static Throughput of(final long messages, final int messageLength, final long elapsedNs)
    {
        final long bytes = messages * messageLength;
        if (elapsedNs <= 0)
        {
            return new Throughput(messages, bytes, elapsedNs, 0, 0);
        }
        final double seconds = elapsedNs / 1e9;
        return new Throughput(messages, bytes, elapsedNs, messages / seconds, bytes / 1e6 / seconds);
    }

    Json toJson()
    {
        return new Json()
            .field("messages", messages)
            .field("bytes", bytes)
            .field("elapsed_ns", elapsedNs)
            .field("messages_per_second", messagesPerSecond)
            .field("megabytes_per_second", megabytesPerSecond);
    }
}
