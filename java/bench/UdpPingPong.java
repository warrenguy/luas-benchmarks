package bench;

/**
 * Runs udp-ping-pong: round trips over two UDP loopback streams.
 */
final class UdpPingPong
{
    private UdpPingPong()
    {
    }

    static void run(final String[] args) throws Exception
    {
        PingPongRunner.run(Scenario.UDP_PING_PONG, args);
    }
}
