package bench;

import io.aeron.ExclusivePublication;
import io.aeron.Image;
import io.aeron.Publication;
import io.aeron.cluster.codecs.CloseReason;
import io.aeron.cluster.service.ClientSession;
import io.aeron.cluster.service.Cluster;
import io.aeron.cluster.service.ClusteredService;
import io.aeron.logbuffer.Header;
import org.agrona.DirectBuffer;
import org.agrona.concurrent.IdleStrategy;

/**
 * Echoes each session message back to its session. The leader sends the echo. On a follower, the
 * offer returns ClientSession.MOCKED_OFFER without sending. The service does not keep state, so
 * its snapshot is empty.
 */
final class EchoService implements ClusteredService
{
    private IdleStrategy idleStrategy;

    public void onStart(final Cluster cluster, final Image snapshotImage)
    {
        idleStrategy = cluster.idleStrategy();
    }

    public void onSessionOpen(final ClientSession session, final long timestamp)
    {
    }

    public void onSessionClose(final ClientSession session, final long timestamp, final CloseReason closeReason)
    {
    }

    /**
     * Offers the message back to session. Retries back pressure and admin action with the cluster idle strategy.
     * Drops the echo on any other failed offer, for example after the client disconnects.
     */
    public void onSessionMessage(
        final ClientSession session,
        final long timestamp,
        final DirectBuffer buffer,
        final int offset,
        final int length,
        final Header header)
    {
        idleStrategy.reset();
        while (true)
        {
            final long position = session.offer(buffer, offset, length);
            if (position > 0 || (position != Publication.BACK_PRESSURED && position != Publication.ADMIN_ACTION))
            {
                return;
            }
            idleStrategy.idle();
        }
    }

    public void onTimerEvent(final long correlationId, final long timestamp)
    {
    }

    public void onTakeSnapshot(final ExclusivePublication snapshotPublication)
    {
    }

    public void onRoleChange(final Cluster.Role newRole)
    {
    }

    public void onTerminate(final Cluster cluster)
    {
    }
}
