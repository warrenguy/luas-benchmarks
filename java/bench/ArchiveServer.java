package bench;

import io.aeron.archive.Archive;
import io.aeron.exceptions.AeronException;
import org.agrona.SystemUtil;
import org.agrona.concurrent.AgentTerminationException;
import org.agrona.concurrent.ShutdownSignalBarrier;

import java.util.List;

/**
 * Runs the Java archive on the C driver in aeron.dir until SIGINT, SIGTERM or a fatal archive error. scripts/archive
 * starts it. Each argument names a properties file that sets the archive context. The bench.server.cpus property
 * pins the archive thread to its one CPU. Prints one ready line once the archive runs. Prints each archive error to
 * stderr.
 */
public final class ArchiveServer
{
    private ArchiveServer()
    {
    }

    public static void main(final String[] args)
    {
        SystemUtil.loadPropertiesFiles(args);
        final List<Integer> cpus = ServerCpus.parse(1);
        try (ShutdownSignalBarrier barrier = new ShutdownSignalBarrier())
        {
            final Archive.Context context = new Archive.Context().errorHandler(
                (throwable) ->
                {
                    throwable.printStackTrace();
                    if (throwable instanceof AgentTerminationException || AeronException.isFatal(throwable))
                    {
                        barrier.signal();
                    }
                });
            if (!cpus.isEmpty())
            {
                context.threadFactory(Pinning.threadFactory(cpus.get(0)));
            }
            try (Archive archive = Archive.launch(context))
            {
                System.out.println("archive: ready: archive id " + archive.context().archiveId());
                System.out.flush();
                barrier.await();
            }
        }
    }
}
