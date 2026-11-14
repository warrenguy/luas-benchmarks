package bench;

import io.aeron.archive.Archive;
import io.aeron.archive.client.AeronArchive;
import io.aeron.cluster.ConsensusModule;
import io.aeron.cluster.ElectionState;
import io.aeron.cluster.service.Cluster;
import io.aeron.cluster.service.ClusteredServiceContainer;
import io.aeron.exceptions.AeronException;
import org.agrona.ErrorHandler;
import org.agrona.SystemUtil;
import org.agrona.concurrent.AgentTerminationException;
import org.agrona.concurrent.NoOpLock;
import org.agrona.concurrent.ShutdownSignalBarrier;

import java.util.Arrays;
import java.util.List;
import java.util.Locale;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Runs one Java cluster member on the C driver in aeron.dir until SIGINT, SIGTERM or a fatal error. scripts/cluster
 * starts one per member. The member runs an archive, a consensus module and a clustered service container with
 * EchoService in one JVM. aeron.archive.dir and aeron.cluster.dir name the member directories.
 * <p>
 * Arguments: member id, member count, then properties files that set the contexts. The bench.server.cpus property
 * pins the archive, consensus module and service threads to its three CPUs. Prints one ready line once the
 * first election closes. Prints each error to stderr. Exits with status 1 when a fatal error or a termination hook
 * stops the member before the first election closes. A signal before the first election closes ends the JVM with the
 * signal status.
 */
public final class ClusterServer
{
    private static final int ARCHIVE_CONTROL_PORT_OFFSET = 1;
    private static final int CONSENSUS_PORT_OFFSET = 3;
    private static final int LOG_PORT_OFFSET = 4;
    private static final int CATCHUP_PORT_OFFSET = 5;
    private static final String LOCAL_CONTROL_CHANNEL = "aeron:ipc?term-length=64k";
    private static final String REPLICATION_CHANNEL = "aeron:udp?endpoint=" + Scenario.CLUSTER_HOST + ":0";
    private static final long READY_POLL_MS = 10;

    private ClusterServer()
    {
    }

    public static void main(final String[] args) throws InterruptedException
    {
        if (args.length < 2)
        {
            System.err.println("usage: ClusterServer <member-id> <member-count> [properties-file ...]");
            System.exit(2);
        }
        final int memberId = Integer.parseInt(args[0]);
        final int memberCount = Integer.parseInt(args[1]);
        SystemUtil.loadPropertiesFiles(Arrays.copyOfRange(args, 2, args.length));
        final List<Integer> cpus = ServerCpus.parse(3);

        final AtomicBoolean stopped = new AtomicBoolean();
        boolean ready = false;
        try (ShutdownSignalBarrier barrier = new ShutdownSignalBarrier(() -> stopped.set(true)))
        {
            final ErrorHandler errorHandler =
                (throwable) ->
                {
                    throwable.printStackTrace();
                    if (throwable instanceof AgentTerminationException || AeronException.isFatal(throwable))
                    {
                        barrier.signal();
                    }
                };

            final Archive.Context archiveContext = new Archive.Context()
                .controlChannel("aeron:udp?endpoint=" +
                    Scenario.clusterEndpoint(memberId, ARCHIVE_CONTROL_PORT_OFFSET))
                .localControlChannel(LOCAL_CONTROL_CHANNEL)
                .replicationChannel(REPLICATION_CHANNEL)
                .archiveClientContext(new AeronArchive.Context().controlResponseChannel(REPLICATION_CHANNEL))
                .recordingEventsEnabled(false)
                .errorHandler(errorHandler);

            final AeronArchive.Context localArchiveContext = new AeronArchive.Context()
                .lock(NoOpLock.INSTANCE)
                .controlRequestChannel(LOCAL_CONTROL_CHANNEL)
                .controlRequestStreamId(archiveContext.localControlStreamId())
                .controlResponseChannel(LOCAL_CONTROL_CHANNEL)
                .errorHandler(errorHandler);

            final ConsensusModule.Context consensusModuleContext = new ConsensusModule.Context()
                .clusterMemberId(memberId)
                .clusterMembers(clusterMembers(memberCount))
                .ingressChannel(Scenario.CLUSTER_INGRESS_CHANNEL)
                .ingressStreamId(Config.DEFAULT_STREAM_ID)
                .replicationChannel(REPLICATION_CHANNEL)
                .archiveContext(localArchiveContext.clone())
                .serviceCount(1)
                .terminationHook(barrier::signal)
                .errorHandler(errorHandler);

            final ClusteredServiceContainer.Context serviceContext = new ClusteredServiceContainer.Context()
                .clusteredService(new EchoService())
                .archiveContext(localArchiveContext.clone())
                .terminationHook(barrier::signal)
                .errorHandler(errorHandler);
            if (!cpus.isEmpty())
            {
                archiveContext.threadFactory(Pinning.threadFactory(cpus.get(0)));
                consensusModuleContext.threadFactory(Pinning.threadFactory(cpus.get(1)));
                serviceContext.threadFactory(Pinning.threadFactory(cpus.get(2)));
            }

            try (Archive ignoreArchive = Archive.launch(archiveContext);
                ConsensusModule consensusModule = ConsensusModule.launch(consensusModuleContext);
                ClusteredServiceContainer ignoreContainer = ClusteredServiceContainer.launch(serviceContext))
            {
                final ConsensusModule.Context context = consensusModule.context();
                ready = awaitElection(context, stopped);
                if (ready)
                {
                    final Cluster.Role role = Cluster.Role.get(context.clusterNodeRoleCounter());
                    System.out.println(
                        "cluster: member " + memberId + " ready: " + role.name().toLowerCase(Locale.ROOT));
                    System.out.flush();
                    barrier.await();
                }
            }
        }
        if (!ready)
        {
            System.err.println("cluster: member " + memberId + " stopped before the first election closed");
            System.exit(1);
        }
    }

    /**
     * Returns the cluster members string: id, then the ingress, consensus, log, catchup and archive endpoints of
     * each member.
     */
    private static String clusterMembers(final int memberCount)
    {
        final StringBuilder members = new StringBuilder();
        for (int id = 0; id < memberCount; id++)
        {
            members.append(id)
                .append(',').append(Scenario.clusterEndpoint(id, Scenario.CLUSTER_INGRESS_PORT_OFFSET))
                .append(',').append(Scenario.clusterEndpoint(id, CONSENSUS_PORT_OFFSET))
                .append(',').append(Scenario.clusterEndpoint(id, LOG_PORT_OFFSET))
                .append(',').append(Scenario.clusterEndpoint(id, CATCHUP_PORT_OFFSET))
                .append(',').append(Scenario.clusterEndpoint(id, ARCHIVE_CONTROL_PORT_OFFSET))
                .append('|');
        }
        return members.toString();
    }

    /**
     * Waits until the first election closes and the consensus module turns active, or until stopped turns true.
     *
     * @return true when the election closed, false when stopped turned true first.
     */
    private static boolean awaitElection(final ConsensusModule.Context context, final AtomicBoolean stopped)
        throws InterruptedException
    {
        while (ElectionState.CLOSED != ElectionState.get(context.electionStateCounter()) ||
            ConsensusModule.State.ACTIVE != ConsensusModule.State.get(context.moduleStateCounter()))
        {
            if (stopped.get())
            {
                return false;
            }
            Thread.sleep(READY_POLL_MS);
        }
        return true;
    }
}
