package bench;

/**
 * Fields of FixedRateTracker. Ping writes them on every reply.
 */
abstract class FixedRateTrackerFields extends LhsPadding
{
    final LatencyHistogram histogram;
    final int messageLength;
    long received;
    String error;

    FixedRateTrackerFields(final LatencyHistogram histogram, final int messageLength)
    {
        this.histogram = histogram;
        this.messageLength = messageLength;
    }
}

/**
 * Records the round trips of a fixed-rate run. Mirrors bench.FixedRateTracker.
 * <p>
 * reply accepts only the next sequence at the configured length. The first mismatch stops the count.
 * <p>
 * The paddings keep other heap objects off the cache lines of the fields. Mirrors bench.CacheLinePadding.
 */
final class FixedRateTracker extends FixedRateTrackerFields
{
    byte p128, p129, p130, p131, p132, p133, p134, p135, p136, p137, p138, p139, p140, p141, p142, p143;
    byte p144, p145, p146, p147, p148, p149, p150, p151, p152, p153, p154, p155, p156, p157, p158, p159;
    byte p160, p161, p162, p163, p164, p165, p166, p167, p168, p169, p170, p171, p172, p173, p174, p175;
    byte p176, p177, p178, p179, p180, p181, p182, p183, p184, p185, p186, p187, p188, p189, p190, p191;
    byte p192, p193, p194, p195, p196, p197, p198, p199, p200, p201, p202, p203, p204, p205, p206, p207;
    byte p208, p209, p210, p211, p212, p213, p214, p215, p216, p217, p218, p219, p220, p221, p222, p223;
    byte p224, p225, p226, p227, p228, p229, p230, p231, p232, p233, p234, p235, p236, p237, p238, p239;
    byte p240, p241, p242, p243, p244, p245, p246, p247, p248, p249, p250, p251, p252, p253, p254, p255;

    /**
     * Returns a tracker that records into histogram.
     *
     * @param histogram     round trip histogram.
     * @param messageLength message length.
     */
    FixedRateTracker(final LatencyHistogram histogram, final int messageLength)
    {
        super(histogram, messageLength);
    }

    /**
     * Checks one reply and records nowNs minus timestampNs. Stores an error when the length differs from the message
     * length, or when sequence differs from the reply count. Does not allocate while the replies match.
     *
     * @param nowNs       current time from System.nanoTime.
     * @param timestampNs scheduled send time in the reply.
     * @param sequence    sequence number in the reply.
     * @param length      reply length.
     */
    void reply(final long nowNs, final long timestampNs, final long sequence, final int length)
    {
        if (null != error)
        {
            return;
        }
        if (length != messageLength)
        {
            error = "reply " + received + ": length " + length + ", want " + messageLength;
            return;
        }
        if (sequence != received)
        {
            error = "reply " + received + ": sequence " + sequence;
            return;
        }
        final long roundTripNs = nowNs - timestampNs;
        try
        {
            histogram.record(roundTripNs);
        }
        catch (final ArrayIndexOutOfBoundsException ex)
        {
            error = "record round trip: " + roundTripNs + " ns out of range";
            return;
        }
        received++;
    }

    long received()
    {
        return received;
    }

    /**
     * Throws the first error reply stored.
     *
     * @throws IllegalStateException when a reply did not match.
     */
    void throwIfFailed()
    {
        if (null != error)
        {
            throw new IllegalStateException(error);
        }
    }

    /**
     * Removes the recorded values and zeroes the reply count.
     */
    void reset()
    {
        histogram.reset();
        received = 0;
    }
}
