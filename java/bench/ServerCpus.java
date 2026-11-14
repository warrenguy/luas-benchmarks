package bench;

import java.util.List;

/**
 * Reads the bench.server.cpus system property: the CPUs a server pins its agent threads to, one per agent.
 * scripts/cluster and scripts/archive set it from SERVER_CPUS.
 */
final class ServerCpus
{
    static final String PROPERTY = "bench.server.cpus";

    private ServerCpus()
    {
    }

    /**
     * Parses the property.
     *
     * @param count agent threads the server pins.
     * @return the CPUs in property order, or an empty list when the property is unset.
     * @throws IllegalArgumentException when the property lists another count.
     */
    static List<Integer> parse(final int count)
    {
        final String text = System.getProperty(PROPERTY);
        if (null == text)
        {
            return List.of();
        }
        final List<Integer> cpus = CpuList.parse(text);
        if (cpus.size() != count)
        {
            throw new IllegalArgumentException(PROPERTY + " " + text + ": want " + count + " cpus");
        }
        return cpus;
    }
}
