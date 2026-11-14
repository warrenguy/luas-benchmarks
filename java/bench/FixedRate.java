package bench;

/**
 * Message counts and latency histogram of one fixed-rate run. Mirrors bench.FixedRate.
 *
 * @param expected  message count of the schedule.
 * @param sent      messages that went out before the deadline.
 * @param received  replies.
 * @param histogram round trip histogram in the HdrHistogram V2 compressed format, base64 encoded.
 */
record FixedRate(long expected, long sent, long received, String histogram)
{
    FixedRate withHistogram(final String histogram)
    {
        return new FixedRate(expected, sent, received, histogram);
    }

    Json toJson()
    {
        return new Json()
            .field("expected", expected)
            .field("sent", sent)
            .field("received", received)
            .field("histogram", histogram);
    }
}
