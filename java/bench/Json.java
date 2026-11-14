package bench;

import java.util.List;

/**
 * Builds one JSON object. Matches the field names of the Go structs.
 */
final class Json
{
    private final StringBuilder out = new StringBuilder();
    private boolean first = true;

    Json()
    {
        out.append('{');
    }

    Json field(final String name, final long value)
    {
        name(name).append(value);
        return this;
    }

    Json field(final String name, final double value)
    {
        if (!Double.isFinite(value))
        {
            throw new IllegalArgumentException("json field " + name + ": non-finite value " + value);
        }
        name(name).append(value);
        return this;
    }

    Json field(final String name, final String value)
    {
        quote(name(name), value);
        return this;
    }

    Json field(final String name, final List<Integer> values)
    {
        final StringBuilder sb = name(name).append('[');
        for (int i = 0; i < values.size(); i++)
        {
            if (i > 0)
            {
                sb.append(',');
            }
            sb.append(values.get(i).intValue());
        }
        sb.append(']');
        return this;
    }

    Json field(final String name, final Json value)
    {
        name(name).append(value);
        return this;
    }

    Json field(final String name, final Json[] values)
    {
        final StringBuilder sb = name(name).append('[');
        for (int i = 0; i < values.length; i++)
        {
            if (i > 0)
            {
                sb.append(',');
            }
            sb.append(values[i]);
        }
        sb.append(']');
        return this;
    }

    public String toString()
    {
        return out + "}";
    }

    private StringBuilder name(final String name)
    {
        if (!first)
        {
            out.append(',');
        }
        first = false;
        quote(out, name);
        return out.append(':');
    }

    private static void quote(final StringBuilder sb, final String value)
    {
        sb.append('"');
        for (int i = 0; i < value.length(); i++)
        {
            final char c = value.charAt(i);
            switch (c)
            {
                case '"' -> sb.append("\\\"");
                case '\\' -> sb.append("\\\\");
                case '\n' -> sb.append("\\n");
                case '\r' -> sb.append("\\r");
                case '\t' -> sb.append("\\t");
                default ->
                {
                    if (c < 0x20 || c > 0x7e)
                    {
                        sb.append(String.format("\\u%04x", (int)c));
                    }
                    else
                    {
                        sb.append(c);
                    }
                }
            }
        }
        sb.append('"');
    }
}
