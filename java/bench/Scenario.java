package bench;

/**
 * One benchmark and the flag defaults it sets. Mirrors bench.Scenario: same names and defaults.
 *
 * @param name                 scenario name.
 * @param channel              default channel.
 * @param messageLength        default message length in bytes.
 * @param fragmentLimit        default fragment limit per poll.
 * @param isCluster            whether the scenario runs against the Java cluster. Config.parse then takes
 *                             -cluster-members and checks the cluster limits.
 * @param isFixedRate          whether the scenario sends on a fixed schedule. Config.parse then takes -rate and
 *                             -warmup-rate and checks the fixed-rate limits.
 */
record Scenario(
    String name,
    String channel,
    int messageLength,
    int fragmentLimit,
    boolean isCluster,
    boolean isFixedRate)
{
    static final String IPC_CHANNEL = "aeron:ipc";
    static final String UDP_CHANNEL = "aeron:udp?endpoint=127.0.0.1:20121";
    static final int THROUGHPUT_FRAGMENT_LIMIT = 10;
    static final int PING_PONG_FRAGMENT_LIMIT = 1;
    static final int FIXED_RATE_FRAGMENT_LIMIT = 10;

    /**
     * Longest message that fits one frame at the driver default MTU of 1408 bytes. The 32-byte data frame header
     * takes the rest of the frame. Offer splits a longer message into fragments. TryClaim rejects a longer message.
     */
    static final int MAX_PAYLOAD_LENGTH = 1408 - 32;

    static final int ARCHIVE_MESSAGE_LENGTH = 256;
    static final String ARCHIVE_CONTROL_REQUEST_CHANNEL = "aeron:ipc?term-length=64k";
    static final int ARCHIVE_CONTROL_REQUEST_STREAM_ID = 10;
    static final String ARCHIVE_CONTROL_RESPONSE_CHANNEL = "aeron:ipc?term-length=64k";
    static final int ARCHIVE_CONTROL_RESPONSE_STREAM_ID = 20;

    /**
     * Stream on the scenario channel that archive-replay replays to.
     */
    static final int REPLAY_STREAM_ID = 1003;

    /**
     * Default ingress channel of cluster-ping-pong. The client substitutes the endpoint of each member. The stream id
     * names the ingress stream.
     */
    static final String CLUSTER_INGRESS_CHANNEL = "aeron:udp";

    /**
     * Carries the echoes of the cluster to the client. The pong stream id names the egress stream.
     */
    static final String CLUSTER_EGRESS_CHANNEL = "aeron:udp?endpoint=127.0.0.1:20300";

    /**
     * Fragment limit per egress poll of cluster-ping-pong. The Java, Luas and aeron-go cluster clients each fix it
     * at 10.
     */
    static final int CLUSTER_EGRESS_FRAGMENT_LIMIT = 10;

    /**
     * First port of member 0. Member i takes the CLUSTER_MEMBER_PORT_STRIDE ports from CLUSTER_MEMBER_PORT_BASE +
     * CLUSTER_MEMBER_PORT_STRIDE * i.
     */
    static final int CLUSTER_MEMBER_PORT_BASE = 20310;

    /**
     * Port count of one member.
     */
    static final int CLUSTER_MEMBER_PORT_STRIDE = 10;

    /**
     * Offset of the ingress port within the ports of one member.
     */
    static final int CLUSTER_INGRESS_PORT_OFFSET = 2;

    /**
     * Host of every cluster member.
     */
    static final String CLUSTER_HOST = "127.0.0.1";

    static final int PUBLISHER_THREAD = 0;
    static final int SUBSCRIBER_THREAD = 1;
    static final int PING_THREAD = 0;
    static final int PONG_THREAD = 1;

    static final Scenario IPC_THROUGHPUT = new Scenario(
        "ipc-throughput", IPC_CHANNEL, Config.DEFAULT_MESSAGE_LENGTH, THROUGHPUT_FRAGMENT_LIMIT, false, false);
    static final Scenario UDP_THROUGHPUT = new Scenario(
        "udp-throughput", UDP_CHANNEL, Config.DEFAULT_MESSAGE_LENGTH, THROUGHPUT_FRAGMENT_LIMIT, false, false);
    static final Scenario IPC_PING_PONG = new Scenario(
        "ipc-ping-pong", IPC_CHANNEL, Config.DEFAULT_MESSAGE_LENGTH, PING_PONG_FRAGMENT_LIMIT, false, false);
    static final Scenario UDP_PING_PONG = new Scenario(
        "udp-ping-pong", UDP_CHANNEL, Config.DEFAULT_MESSAGE_LENGTH, PING_PONG_FRAGMENT_LIMIT, false, false);
    static final Scenario IPC_FIXED_RATE = new Scenario(
        "ipc-fixed-rate", IPC_CHANNEL, Config.DEFAULT_MESSAGE_LENGTH, FIXED_RATE_FRAGMENT_LIMIT, false, true);
    static final Scenario UDP_FIXED_RATE = new Scenario(
        "udp-fixed-rate", UDP_CHANNEL, Config.DEFAULT_MESSAGE_LENGTH, FIXED_RATE_FRAGMENT_LIMIT, false, true);
    static final Scenario ARCHIVE_RECORD = new Scenario(
        "archive-record", IPC_CHANNEL, ARCHIVE_MESSAGE_LENGTH, THROUGHPUT_FRAGMENT_LIMIT, false, false);
    static final Scenario ARCHIVE_REPLAY = new Scenario(
        "archive-replay", IPC_CHANNEL, ARCHIVE_MESSAGE_LENGTH, THROUGHPUT_FRAGMENT_LIMIT, false, false);
    static final Scenario CLUSTER_PING_PONG = new Scenario(
        "cluster-ping-pong", CLUSTER_INGRESS_CHANNEL, Config.DEFAULT_MESSAGE_LENGTH, CLUSTER_EGRESS_FRAGMENT_LIMIT,
        true, false);

    /**
     * Returns the ingress endpoints of a cluster, for example "0=127.0.0.1:20312,1=127.0.0.1:20322". Mirrors
     * bench.ClusterIngressEndpoints.
     *
     * @param members cluster member count.
     * @return the ingress endpoints.
     */
    static String clusterIngressEndpoints(final int members)
    {
        final StringBuilder endpoints = new StringBuilder();
        for (int id = 0; id < members; id++)
        {
            if (id > 0)
            {
                endpoints.append(',');
            }
            endpoints.append(id).append('=').append(clusterEndpoint(id, CLUSTER_INGRESS_PORT_OFFSET));
        }
        return endpoints.toString();
    }

    /**
     * Returns the host and port of one member port.
     *
     * @param memberId   member id.
     * @param portOffset offset of the port within the ports of the member.
     * @return the endpoint, for example "127.0.0.1:20312".
     */
    static String clusterEndpoint(final int memberId, final int portOffset)
    {
        return CLUSTER_HOST + ":" + (CLUSTER_MEMBER_PORT_BASE + CLUSTER_MEMBER_PORT_STRIDE * memberId + portOffset);
    }
}
