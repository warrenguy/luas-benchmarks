package bench;

import net.openhft.affinity.Affinity;

import java.util.BitSet;
import java.util.concurrent.ThreadFactory;

/**
 * Binds the calling thread to one CPU through the OpenHFT affinity library. Mirrors PinThread.
 */
final class Pinning
{
    private Pinning()
    {
    }

    /**
     * Binds the calling thread to cpu.
     *
     * @param cpu CPU number from 0 to CpuList.MAX_CPU.
     * @throws IllegalArgumentException when cpu is out of range.
     * @throws IllegalStateException    when the thread CPU set does not read back as cpu alone.
     */
    static void pinThread(final int cpu)
    {
        if (cpu < 0 || cpu > CpuList.MAX_CPU)
        {
            throw new IllegalArgumentException("pin thread: cpu " + cpu + " outside 0-" + CpuList.MAX_CPU);
        }
        Affinity.setAffinity(cpu);
        final BitSet got = Affinity.getAffinity();
        if (got.cardinality() != 1 || !got.get(cpu))
        {
            throw new IllegalStateException("pin thread to cpu " + cpu + ": thread cpus read back as " + got);
        }
    }

    /**
     * Returns a thread factory whose threads bind themselves to cpu before they run their task.
     *
     * @param cpu CPU number from 0 to CpuList.MAX_CPU.
     * @return the thread factory.
     */
    static ThreadFactory threadFactory(final int cpu)
    {
        return (task) -> new Thread(() ->
        {
            pinThread(cpu);
            task.run();
        });
    }
}
