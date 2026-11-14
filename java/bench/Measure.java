package bench;

/**
 * Runs the warmup and the measured section of a scenario. Mirrors bench.Measure.
 */
final class Measure
{
    /**
     * One pass of a scenario over count messages.
     */
    @FunctionalInterface
    interface Pass
    {
        void run(long count) throws Exception;
    }

    /**
     * Clock and GC totals of one measured section.
     *
     * @param elapsedNs  measured section length.
     * @param jvmRuntime GC totals over the measured section.
     */
    record Measurement(long elapsedNs, JvmRuntime jvmRuntime)
    {
    }

    private Measure()
    {
    }

    /**
     * Runs warmup over the warmup count. Then reads the GC totals and starts the clock. Runs measured over the
     * message count. Stops the clock before it reads the GC totals again. Skips warmup when the warmup count is 0.
     *
     * @param config   scenario flags.
     * @param warmup   warmup pass.
     * @param measured measured pass.
     * @return the measurement.
     * @throws Exception when a pass throws.
     */
    static Measurement run(final Config config, final Pass warmup, final Pass measured) throws Exception
    {
        if (config.warmupMessages() > 0)
        {
            warmup.run(config.warmupMessages());
        }
        final JvmRuntime before = JvmRuntime.read();
        final long start = System.nanoTime();
        measured.run(config.messages());
        final long elapsedNs = System.nanoTime() - start;
        final JvmRuntime after = JvmRuntime.read();
        return new Measurement(elapsedNs, after.minus(before));
    }
}
