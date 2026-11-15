package bench;

import java.util.Arrays;
import java.util.Map;
import java.util.SortedMap;
import java.util.TreeMap;

/**
 * Runs one benchmark scenario on the Aeron (Java) client.
 * Usage: java -jar bench.jar &lt;scenario&gt; [flags]
 */
public final class Main
{
    private static final SortedMap<String, Runner> SCENARIOS = new TreeMap<>(Map.ofEntries(
        Map.entry("connect", Connect::run),
        Map.entry("ipc-throughput", IpcThroughput::run),
        Map.entry("udp-throughput", UdpThroughput::run),
        Map.entry("ipc-ping-pong", IpcPingPong::run),
        Map.entry("udp-ping-pong", UdpPingPong::run),
        Map.entry("ipc-fixed-rate", IpcFixedRate::run),
        Map.entry("udp-fixed-rate", UdpFixedRate::run),
        Map.entry("archive-record", ArchiveRecord::run),
        Map.entry("archive-replay", ArchiveReplay::run),
        Map.entry("cluster-ping-pong", ClusterPingPong::run)));

    private Main()
    {
    }

    public static void main(final String[] args)
    {
        if (args.length < 1)
        {
            System.err.println(
                "usage: java -jar bench.jar <scenario> [flags]\nscenarios: " + String.join(", ", SCENARIOS.keySet()));
            System.exit(2);
        }
        final String name = args[0];
        final String[] flags = Arrays.copyOfRange(args, 1, args.length);
        final Runner runner = SCENARIOS.get(name);
        if (null == runner)
        {
            System.err.println("bench: unknown scenario \"" + name + "\"");
            System.exit(2);
        }
        try
        {
            runner.run(flags);
        }
        catch (final Exception ex)
        {
            System.err.println("java " + name + ": " + ex);
            System.exit(1);
        }
    }

    /**
     * Runs one scenario with the flags after its name.
     */
    @FunctionalInterface
    private interface Runner
    {
        void run(String[] flags) throws Exception;
    }
}
