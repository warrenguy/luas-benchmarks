package bench;

/**
 * Runs ipc-ping-pong: round trips over two IPC streams.
 */
final class IpcPingPong
{
    private IpcPingPong()
    {
    }

    static void run(final String[] args) throws Exception
    {
        PingPongRunner.run(Scenario.IPC_PING_PONG, args);
    }
}
