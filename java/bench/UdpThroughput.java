package bench;

/**
 * Runs udp-throughput: one publisher and one subscriber on one UDP loopback stream.
 */
final class UdpThroughput
{
    private UdpThroughput()
    {
    }

    static void run(final String[] args) throws Exception
    {
        ThroughputRunner.run(Scenario.UDP_THROUGHPUT, args);
    }
}
