package bench;

/**
 * Runs ipc-throughput: one publisher and one subscriber on one IPC stream.
 */
final class IpcThroughput
{
    private IpcThroughput()
    {
    }

    static void run(final String[] args) throws Exception
    {
        ThroughputRunner.run(Scenario.IPC_THROUGHPUT, args);
    }
}
