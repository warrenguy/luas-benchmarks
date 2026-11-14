package bench;

import java.util.ArrayList;
import java.util.List;

/**
 * Flags of a scenario. Mirrors bench.Config and bench.ParseFlags: same flag names, defaults and
 * checks. Only a cluster scenario takes -cluster-members. Only a fixed-rate scenario takes -rate and -warmup-rate.
 *
 * @param aeronDir       driver directory.
 * @param channel        channel.
 * @param streamId       stream id.
 * @param pongStreamId   stream of the ping-pong replies. The throughput scenarios do not use it. validate still
 *                       rejects it when it equals streamId, in every scenario.
 * @param messageLength  message length in bytes.
 * @param messages       measured message count.
 * @param warmupMessages warmup message count.
 * @param fragmentLimit  fragment limit per poll.
 * @param publication    CONCURRENT_PUBLICATION or EXCLUSIVE_PUBLICATION.
 * @param sendMethod     SEND_OFFER or SEND_TRY_CLAIM.
 * @param clusterMembers member count of the cluster that cluster-ping-pong runs against. 0 in every other scenario.
 * @param rate           send rate of a fixed-rate scenario in messages per second. 0 in every other scenario.
 * @param warmupRate     warmup send rate of a fixed-rate scenario in messages per second. 0 in every other scenario.
 * @param cpus           one CPU per hot thread. Empty leaves every thread unpinned.
 * @param output         result JSON file. Empty writes to stdout.
 */
record Config(
    String aeronDir,
    String channel,
    int streamId,
    int pongStreamId,
    int messageLength,
    long messages,
    long warmupMessages,
    int fragmentLimit,
    String publication,
    String sendMethod,
    int clusterMembers,
    long rate,
    long warmupRate,
    List<Integer> cpus,
    String output)
{
    static final String CONCURRENT_PUBLICATION = "concurrent";
    static final String EXCLUSIVE_PUBLICATION = "exclusive";
    static final String SEND_OFFER = "offer";
    static final String SEND_TRY_CLAIM = "try-claim";

    static final int DEFAULT_STREAM_ID = 1001;
    static final int DEFAULT_PONG_STREAM_ID = 1002;
    static final int DEFAULT_MESSAGE_LENGTH = 32;
    static final long DEFAULT_MESSAGES = 1_000_000;
    static final long DEFAULT_WARMUP_MESSAGES = 100_000;
    static final String DEFAULT_PUBLICATION = CONCURRENT_PUBLICATION;
    static final String DEFAULT_SEND_METHOD = SEND_OFFER;
    static final int DEFAULT_CLUSTER_MEMBERS = 1;
    static final long DEFAULT_RATE = 501_000;
    static final long DEFAULT_WARMUP_RATE = 25_000;

    /**
     * Shortest message. Each message starts with an 8-byte sequence number or timestamp.
     */
    static final int MIN_MESSAGE_LENGTH = 8;

    /**
     * Shortest fixed-rate message. Each fixed-rate message starts with an 8-byte scheduled send time and an 8-byte
     * sequence number.
     */
    static final int MIN_FIXED_RATE_MESSAGE_LENGTH = 16;

    /**
     * Highest fixed-rate send rate in messages per second. The send interval is then 1 ns.
     */
    static final long MAX_RATE = 1_000_000_000;

    /**
     * Parses args, the arguments after the scenario name. Each flag takes "-name value" or "-name=value". A double
     * dash works as a single dash. A bare "--" ends the flags, as in the Go flag package. AERON_DIR from the
     * environment sets the default driver directory. The scenario sets the channel, message length and fragment
     * limit defaults. Only a cluster scenario takes -cluster-members. Only a fixed-rate scenario takes -rate and
     * -warmup-rate.
     *
     * @param scenario scenario that sets the defaults.
     * @param args     flags.
     * @return the checked config.
     * @throws IllegalArgumentException when a flag does not parse, a positional argument appears, or a check fails.
     */
    static Config parse(final Scenario scenario, final String[] args)
    {
        final String envAeronDir = System.getenv("AERON_DIR");
        String aeronDir = null == envAeronDir ? "" : envAeronDir;
        String channel = scenario.channel();
        int streamId = DEFAULT_STREAM_ID;
        int pongStreamId = DEFAULT_PONG_STREAM_ID;
        int messageLength = scenario.messageLength();
        long messages = DEFAULT_MESSAGES;
        long warmupMessages = DEFAULT_WARMUP_MESSAGES;
        int fragmentLimit = scenario.fragmentLimit();
        String publication = DEFAULT_PUBLICATION;
        String sendMethod = DEFAULT_SEND_METHOD;
        int clusterMembers = scenario.isCluster() ? DEFAULT_CLUSTER_MEMBERS : 0;
        long rate = scenario.isFixedRate() ? DEFAULT_RATE : 0;
        long warmupRate = scenario.isFixedRate() ? DEFAULT_WARMUP_RATE : 0;
        List<Integer> cpus = new ArrayList<>();
        String output = "";

        for (int i = 0; i < args.length; i++)
        {
            final String arg = args[i];
            if (arg.equals("--"))
            {
                if (i + 1 == args.length)
                {
                    break;
                }
                throw new IllegalArgumentException("parse flags: unexpected argument \"" + args[i + 1] + "\"");
            }
            if (!arg.startsWith("-") || arg.equals("-"))
            {
                throw new IllegalArgumentException("parse flags: unexpected argument \"" + arg + "\"");
            }
            String name = arg.substring(arg.startsWith("--") ? 2 : 1);
            final String value;
            final int equals = name.indexOf('=');
            if (equals >= 0)
            {
                value = name.substring(equals + 1);
                name = name.substring(0, equals);
            }
            else if (i + 1 < args.length)
            {
                value = args[++i];
            }
            else
            {
                throw new IllegalArgumentException("parse flags: flag needs an argument: -" + name);
            }

            if (name.equals("cluster-members") && scenario.isCluster())
            {
                clusterMembers = parseInt(name, value);
                continue;
            }
            if (name.equals("rate") && scenario.isFixedRate())
            {
                rate = parseLong(name, value);
                continue;
            }
            if (name.equals("warmup-rate") && scenario.isFixedRate())
            {
                warmupRate = parseLong(name, value);
                continue;
            }
            switch (name)
            {
                case "aeron-dir" -> aeronDir = value;
                case "channel" -> channel = value;
                case "stream-id" -> streamId = parseInt(name, value);
                case "pong-stream-id" -> pongStreamId = parseInt(name, value);
                case "message-length" -> messageLength = parseInt(name, value);
                case "messages" -> messages = parseLong(name, value);
                case "warmup-messages" -> warmupMessages = parseLong(name, value);
                case "fragment-limit" -> fragmentLimit = parseInt(name, value);
                case "publication" -> publication = value;
                case "send-method" -> sendMethod = value;
                case "cpus" -> cpus = CpuList.parse(value);
                case "output" -> output = value;
                default -> throw new IllegalArgumentException("parse flags: flag provided but not defined: -" + name);
            }
        }

        final Config config = new Config(
            aeronDir, channel, streamId, pongStreamId, messageLength, messages, warmupMessages, fragmentLimit,
            publication, sendMethod, clusterMembers, rate, warmupRate, List.copyOf(cpus), output);
        config.validate();
        if (scenario.isCluster())
        {
            config.validateCluster();
        }
        if (scenario.isFixedRate())
        {
            config.validateFixedRate();
        }
        return config;
    }

    /**
     * Checks the flag values.
     *
     * @throws IllegalArgumentException when aeronDir or channel is empty, pongStreamId equals streamId,
     *                                  messageLength is below MIN_MESSAGE_LENGTH, messages or fragmentLimit is below 1,
     *                                  warmupMessages is below 0, or publication or sendMethod does not name a known
     *                                  value.
     */
    void validate()
    {
        final List<String> errors = new ArrayList<>();
        if (aeronDir.isEmpty())
        {
            errors.add("aeron dir empty: set -aeron-dir or AERON_DIR");
        }
        if (channel.isEmpty())
        {
            errors.add("channel empty");
        }
        if (pongStreamId == streamId)
        {
            errors.add("pong stream id " + pongStreamId + " equals stream id");
        }
        if (messageLength < MIN_MESSAGE_LENGTH)
        {
            errors.add("message length " + messageLength + " below " + MIN_MESSAGE_LENGTH);
        }
        if (messages < 1)
        {
            errors.add("messages " + messages + " below 1");
        }
        if (warmupMessages < 0)
        {
            errors.add("warmup messages " + warmupMessages + " below 0");
        }
        if (fragmentLimit < 1)
        {
            errors.add("fragment limit " + fragmentLimit + " below 1");
        }
        if (!publication.equals(CONCURRENT_PUBLICATION) && !publication.equals(EXCLUSIVE_PUBLICATION))
        {
            errors.add("publication \"" + publication + "\": want " + CONCURRENT_PUBLICATION + " or " +
                EXCLUSIVE_PUBLICATION);
        }
        if (!sendMethod.equals(SEND_OFFER) && !sendMethod.equals(SEND_TRY_CLAIM))
        {
            errors.add("send method \"" + sendMethod + "\": want " + SEND_OFFER + " or " + SEND_TRY_CLAIM);
        }
        if (!errors.isEmpty())
        {
            throw new IllegalArgumentException("invalid config: " + String.join("; ", errors));
        }
    }

    /**
     * Checks the limits that every cluster client shares. Mirrors bench.Config.ValidateCluster.
     *
     * @throws IllegalArgumentException when clusterMembers is below 1, streamId is not DEFAULT_STREAM_ID, sendMethod
     *                                  is not SEND_OFFER, or fragmentLimit is not
     *                                  Scenario.CLUSTER_EGRESS_FRAGMENT_LIMIT. ClusterServer fixes the ingress
     *                                  stream at DEFAULT_STREAM_ID.
     */
    void validateCluster()
    {
        final List<String> errors = new ArrayList<>();
        if (clusterMembers < 1)
        {
            errors.add("cluster members " + clusterMembers + " below 1");
        }
        if (streamId != DEFAULT_STREAM_ID)
        {
            errors.add(
                "stream id " + streamId + ": the cluster takes ingress on stream " + DEFAULT_STREAM_ID + " only");
        }
        if (!sendMethod.equals(SEND_OFFER))
        {
            errors.add("send method \"" + sendMethod + "\": cluster ingress takes " + SEND_OFFER + " only");
        }
        if (fragmentLimit != Scenario.CLUSTER_EGRESS_FRAGMENT_LIMIT)
        {
            errors.add("fragment limit " + fragmentLimit + ": cluster clients poll egress " +
                Scenario.CLUSTER_EGRESS_FRAGMENT_LIMIT + " fragments at a time");
        }
        if (!errors.isEmpty())
        {
            throw new IllegalArgumentException("invalid cluster config: " + String.join("; ", errors));
        }
    }

    /**
     * Checks the limits of a fixed-rate scenario. Mirrors bench.Config.ValidateFixedRate.
     *
     * @throws IllegalArgumentException when rate or warmupRate is below 1 or above MAX_RATE, or messageLength is
     *                                  below MIN_FIXED_RATE_MESSAGE_LENGTH.
     */
    void validateFixedRate()
    {
        final List<String> errors = new ArrayList<>();
        if (rate < 1 || rate > MAX_RATE)
        {
            errors.add("rate " + rate + ": want 1 to " + MAX_RATE);
        }
        if (warmupRate < 1 || warmupRate > MAX_RATE)
        {
            errors.add("warmup rate " + warmupRate + ": want 1 to " + MAX_RATE);
        }
        if (messageLength < MIN_FIXED_RATE_MESSAGE_LENGTH)
        {
            errors.add("message length " + messageLength + " below " + MIN_FIXED_RATE_MESSAGE_LENGTH);
        }
        if (!errors.isEmpty())
        {
            throw new IllegalArgumentException("invalid fixed-rate config: " + String.join("; ", errors));
        }
    }

    /**
     * Returns the result variant label: publication kind, send method and message length, for example
     * "exclusive-try-claim-256". A cluster config adds the member count, for example "exclusive-offer-32-members-3".
     * A fixed-rate config adds the rate, for example "exclusive-try-claim-32-rate-501000".
     *
     * @return the variant label.
     */
    String variant()
    {
        String variant = publication + "-" + sendMethod + "-" + messageLength;
        if (clusterMembers > 0)
        {
            variant += "-members-" + clusterMembers;
        }
        if (rate > 0)
        {
            variant += "-rate-" + rate;
        }
        return variant;
    }

    boolean isExclusive()
    {
        return publication.equals(EXCLUSIVE_PUBLICATION);
    }

    boolean isTryClaim()
    {
        return sendMethod.equals(SEND_TRY_CLAIM);
    }

    /**
     * Reports whether messageLength exceeds Scenario.MAX_PAYLOAD_LENGTH. The throughput subscriber reassembles each
     * message. Fixed-rate ping and pong do the same, because each side has to see the whole message.
     *
     * @return true when each message spans more than one frame.
     */
    boolean isFragmented()
    {
        return messageLength > Scenario.MAX_PAYLOAD_LENGTH;
    }

    /**
     * Binds the calling thread to cpus[index] through Pinning.pinThread. Does nothing when cpus is empty.
     *
     * @param index hot thread index.
     * @throws IllegalArgumentException when cpus is not empty and does not have an entry at index.
     */
    void pinHotThread(final int index)
    {
        if (cpus.isEmpty())
        {
            return;
        }
        if (index < 0 || index >= cpus.size())
        {
            throw new IllegalArgumentException(
                "pin hot thread " + index + ": -cpus lists " + cpus.size() + " cpus");
        }
        Pinning.pinThread(cpus.get(index));
    }

    Json toJson()
    {
        final Json json = new Json()
            .field("aeron_dir", aeronDir)
            .field("channel", channel)
            .field("stream_id", streamId)
            .field("pong_stream_id", pongStreamId)
            .field("message_length", messageLength)
            .field("messages", messages)
            .field("warmup_messages", warmupMessages)
            .field("fragment_limit", fragmentLimit)
            .field("publication", publication)
            .field("send_method", sendMethod);
        if (clusterMembers > 0)
        {
            json.field("cluster_members", clusterMembers);
        }
        if (rate > 0)
        {
            json.field("rate", rate);
        }
        if (warmupRate > 0)
        {
            json.field("warmup_rate", warmupRate);
        }
        return json.field("cpus", cpus);
    }

    private static int parseInt(final String name, final String value)
    {
        try
        {
            return Integer.parseInt(value);
        }
        catch (final NumberFormatException ex)
        {
            throw new IllegalArgumentException("parse flags: invalid value \"" + value + "\" for flag -" + name, ex);
        }
    }

    private static long parseLong(final String name, final String value)
    {
        try
        {
            return Long.parseLong(value);
        }
        catch (final NumberFormatException ex)
        {
            throw new IllegalArgumentException("parse flags: invalid value \"" + value + "\" for flag -" + name, ex);
        }
    }
}
