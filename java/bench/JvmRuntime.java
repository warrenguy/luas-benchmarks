package bench;

import java.lang.management.GarbageCollectorMXBean;
import java.lang.management.ManagementFactory;

/**
 * GarbageCollectorMXBean totals over the GC pauses. Mirrors bench.JVMRuntime.
 * <p>
 * The totals skip the ZGC and Shenandoah beans whose name ends with "Cycles". Those beans time concurrent cycles.
 * Every other bean counts pauses. The "G1 Concurrent GC" bean counts the G1 Remark and Cleanup pauses, so the
 * totals keep it.
 *
 * @param gcCount  GC pauses.
 * @param gcTimeMs accumulated GC pause time in milliseconds.
 */
record JvmRuntime(long gcCount, long gcTimeMs)
{
    /**
     * Reads the totals since JVM start. A pause bean that reports -1 does not add to the totals.
     *
     * @return the totals.
     */
    static JvmRuntime read()
    {
        long count = 0;
        long timeMs = 0;
        for (final GarbageCollectorMXBean bean : ManagementFactory.getGarbageCollectorMXBeans())
        {
            if (isCycleBean(bean.getName()))
            {
                continue;
            }
            count += Math.max(bean.getCollectionCount(), 0);
            timeMs += Math.max(bean.getCollectionTime(), 0);
        }
        return new JvmRuntime(count, timeMs);
    }

    private static boolean isCycleBean(final String beanName)
    {
        return beanName.endsWith(" Cycles");
    }

    JvmRuntime minus(final JvmRuntime before)
    {
        return new JvmRuntime(gcCount - before.gcCount, gcTimeMs - before.gcTimeMs);
    }

    Json toJson()
    {
        return new Json().field("gc_count", gcCount).field("gc_time_ms", gcTimeMs);
    }
}
