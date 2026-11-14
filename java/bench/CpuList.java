package bench;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/**
 * Parses and formats Linux CPU lists such as "2-4,7". Mirrors ParseCPUList and FormatCPUList.
 */
final class CpuList
{
    /**
     * Highest CPU number that a Go unix.CPUSet holds. The Go clients use the same cap.
     */
    static final int MAX_CPU = 1023;

    private CpuList()
    {
    }

    /**
     * Parses a CPU list. Each entry is one CPU or an ascending range. Keeps the input order. Trims leading and
     * trailing white space. Returns an empty list for blank text.
     *
     * @param text CPU list.
     * @return the CPUs in input order.
     * @throws IllegalArgumentException when an entry is empty or a descending range, a CPU is not an integer from 0
     *                                  to MAX_CPU, or a CPU appears twice.
     */
    static List<Integer> parse(final String text)
    {
        final String trimmed = text.strip();
        final List<Integer> cpus = new ArrayList<>();
        if (trimmed.isEmpty())
        {
            return cpus;
        }
        final Set<Integer> seen = new HashSet<>();
        for (final String entry : trimmed.split(",", -1))
        {
            final int dash = entry.indexOf('-');
            final int first = parseCpu(trimmed, dash < 0 ? entry : entry.substring(0, dash));
            final int last = dash < 0 ? first : parseCpu(trimmed, entry.substring(dash + 1));
            if (last < first)
            {
                throw new IllegalArgumentException(
                    "parse cpu list \"" + trimmed + "\": descending range \"" + entry + "\"");
            }
            for (int cpu = first; cpu <= last; cpu++)
            {
                if (!seen.add(cpu))
                {
                    throw new IllegalArgumentException(
                        "parse cpu list \"" + trimmed + "\": cpu " + cpu + " appears twice");
                }
                cpus.add(cpu);
            }
        }
        return cpus;
    }

    /**
     * Formats cpus as a CPU list. Writes each ascending run of consecutive CPUs as a range.
     *
     * @param cpus CPUs in thread order.
     * @return the CPU list. For distinct CPUs from 0 to MAX_CPU, parse(format(cpus)) returns cpus.
     */
    static String format(final List<Integer> cpus)
    {
        final StringBuilder sb = new StringBuilder();
        int i = 0;
        while (i < cpus.size())
        {
            int end = i;
            while (end + 1 < cpus.size() && cpus.get(end + 1) == cpus.get(end) + 1)
            {
                end++;
            }
            if (i > 0)
            {
                sb.append(',');
            }
            sb.append(cpus.get(i).intValue());
            if (end > i)
            {
                sb.append('-').append(cpus.get(end).intValue());
            }
            i = end + 1;
        }
        return sb.toString();
    }

    private static int parseCpu(final String list, final String text)
    {
        if (text.isEmpty())
        {
            throw new IllegalArgumentException("parse cpu list \"" + list + "\": empty entry");
        }
        for (int i = 0; i < text.length(); i++)
        {
            final char c = text.charAt(i);
            if (c < '0' || c > '9')
            {
                throw new IllegalArgumentException(
                    "parse cpu list \"" + list + "\": cpu \"" + text + "\": want a non-negative integer");
            }
        }
        int cpu;
        try
        {
            cpu = Integer.parseInt(text);
        }
        catch (final NumberFormatException ex)
        {
            cpu = Integer.MAX_VALUE;
        }
        if (cpu > MAX_CPU)
        {
            throw new IllegalArgumentException(
                "parse cpu list \"" + list + "\": cpu \"" + text + "\": above " + MAX_CPU);
        }
        return cpu;
    }
}
