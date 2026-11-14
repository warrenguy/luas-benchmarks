package bench;

import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Links the measuring thread with one hot worker thread: the throughput publisher or the ping-pong responder. The
 * worker reports its failure back. Mirrors bench.Worker.
 * <p>
 * Any thread may call any method. The measuring thread calls stop. The worker thread calls fail. In throughput, the
 * subscriber calls request and the publisher calls next. Either thread may call isStopped and throwIfFailed.
 */
final class Worker
{
    private final AtomicLong requested = new AtomicLong();
    private final AtomicReference<Throwable> failure = new AtomicReference<>();
    private volatile boolean stopped;

    /**
     * Raises the worker target by count messages.
     *
     * @param count messages to add.
     */
    void request(final long count)
    {
        requested.addAndGet(count);
    }

    /**
     * Ends next and makes isStopped true. Idempotent.
     */
    void stop()
    {
        stopped = true;
    }

    boolean isStopped()
    {
        return stopped;
    }

    /**
     * Spins until the target rises above done or stop runs.
     *
     * @param done messages the worker has handled.
     * @return the target, or -1 after stop.
     */
    long next(final long done)
    {
        while (!stopped)
        {
            final long target = requested.get();
            if (target > done)
            {
                return target;
            }
            Transport.IDLE.idle();
        }
        return -1;
    }

    /**
     * Stores the first worker failure. Later calls do nothing.
     *
     * @param cause worker failure.
     */
    void fail(final Throwable cause)
    {
        failure.compareAndSet(null, cause);
    }

    /**
     * Throws when fail stored a failure.
     *
     * @param role worker role for the message.
     * @throws IllegalStateException when the worker failed.
     */
    void throwIfFailed(final String role)
    {
        final Throwable cause = failure.get();
        if (null != cause)
        {
            throw new IllegalStateException(role + ": " + cause.getMessage(), cause);
        }
    }
}
