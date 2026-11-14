package bench;

/**
 * Runs udp-fixed-rate: round trips at a fixed send rate over two UDP loopback streams.
 */
final class UdpFixedRate
{
    private UdpFixedRate()
    {
    }

    static void run(final String[] args) throws Exception
    {
        FixedRateRunner.run(Scenario.UDP_FIXED_RATE, args);
    }
}
