package bench;

/**
 * Fields of SequenceChecker. The subscriber writes them on every message.
 */
abstract class SequenceCheckerFields extends LhsPadding
{
    final int messageLength;
    long received;
    String error;

    SequenceCheckerFields(final int messageLength)
    {
        this.messageLength = messageLength;
    }
}

/**
 * Counts throughput messages on the subscriber. Mirrors bench.SequenceChecker.
 * <p>
 * The publisher writes the sequence number 0, 1, 2 and so on into the first 8 bytes of each message. check accepts
 * only the next number at the configured length. The first mismatch stops the count.
 * <p>
 * The paddings keep other heap objects off the cache lines of the fields. Mirrors bench.CacheLinePadding.
 */
final class SequenceChecker extends SequenceCheckerFields
{
    byte p128, p129, p130, p131, p132, p133, p134, p135, p136, p137, p138, p139, p140, p141, p142, p143;
    byte p144, p145, p146, p147, p148, p149, p150, p151, p152, p153, p154, p155, p156, p157, p158, p159;
    byte p160, p161, p162, p163, p164, p165, p166, p167, p168, p169, p170, p171, p172, p173, p174, p175;
    byte p176, p177, p178, p179, p180, p181, p182, p183, p184, p185, p186, p187, p188, p189, p190, p191;
    byte p192, p193, p194, p195, p196, p197, p198, p199, p200, p201, p202, p203, p204, p205, p206, p207;
    byte p208, p209, p210, p211, p212, p213, p214, p215, p216, p217, p218, p219, p220, p221, p222, p223;
    byte p224, p225, p226, p227, p228, p229, p230, p231, p232, p233, p234, p235, p236, p237, p238, p239;
    byte p240, p241, p242, p243, p244, p245, p246, p247, p248, p249, p250, p251, p252, p253, p254, p255;

    SequenceChecker(final int messageLength)
    {
        super(messageLength);
    }

    /**
     * Counts one message. Does not allocate while the messages match.
     *
     * @param sequence sequence number in the message.
     * @param length   message length.
     */
    void check(final long sequence, final int length)
    {
        if (null != error)
        {
            return;
        }
        if (length != messageLength)
        {
            error = "message " + received + ": length " + length + ", want " + messageLength;
            return;
        }
        if (sequence != received)
        {
            error = "message " + received + ": sequence " + sequence;
            return;
        }
        received++;
    }

    long received()
    {
        return received;
    }

    /**
     * Throws the first mismatch.
     *
     * @throws IllegalStateException when a message did not match.
     */
    void throwIfFailed()
    {
        if (null != error)
        {
            throw new IllegalStateException(error);
        }
    }
}
