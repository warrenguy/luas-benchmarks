package bench;

/**
 * Runs ipc-fixed-rate: round trips at a fixed send rate over two IPC streams.
 */
final class IpcFixedRate
{
    private IpcFixedRate()
    {
    }

    static void run(final String[] args) throws Exception
    {
        FixedRateRunner.run(Scenario.IPC_FIXED_RATE, args);
    }
}
