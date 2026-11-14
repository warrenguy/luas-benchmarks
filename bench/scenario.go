package bench

import (
	"strconv"
	"strings"
)

// Channels, streams, frame limits and per-scenario defaults.
// java/bench/Scenario.java uses the same values.
const (
	IPCChannel = "aeron:ipc"
	UDPChannel = "aeron:udp?endpoint=127.0.0.1:20121"

	// ThroughputFragmentLimit is the fragment limit per poll of the throughput
	// scenarios.
	ThroughputFragmentLimit = 10

	// PingPongFragmentLimit is the fragment limit per poll of the ping-pong
	// scenarios.
	PingPongFragmentLimit = 1

	// FixedRateFragmentLimit is the fragment limit per poll of the
	// fixed-rate scenarios, as in the Aeron load test rig.
	FixedRateFragmentLimit = 10

	// MaxPayloadLength is the longest message that fits one frame at the
	// driver default MTU of 1408 bytes. The 32-byte data frame header takes
	// the rest of the frame. Offer splits a longer message into fragments.
	// TryClaim rejects a longer message.
	MaxPayloadLength = 1408 - 32

	// ArchiveMessageLength is the default message length of archive-record
	// and archive-replay.
	ArchiveMessageLength = 256

	// ArchiveControlRequestChannel is the local control channel of the
	// archive.
	ArchiveControlRequestChannel = "aeron:ipc?term-length=64k"

	// ArchiveControlRequestStreamID is the local control stream of the
	// archive.
	ArchiveControlRequestStreamID = 10

	// ArchiveControlResponseChannel carries the control responses of the
	// archive to the client.
	ArchiveControlResponseChannel = "aeron:ipc?term-length=64k"

	// ArchiveControlResponseStreamID carries the control responses of the
	// archive to the client.
	ArchiveControlResponseStreamID = 20

	// ReplayStreamID is the stream on Config.Channel that archive-replay
	// replays to.
	ReplayStreamID = 1003

	// ClusterIngressChannel is the default ingress channel of
	// cluster-ping-pong. The client substitutes the endpoint of each member.
	// Config.StreamID names the ingress stream.
	ClusterIngressChannel = "aeron:udp"

	// ClusterEgressChannel carries the echoes of the cluster to the client.
	// Config.PongStreamID names the egress stream.
	ClusterEgressChannel = "aeron:udp?endpoint=127.0.0.1:20300"

	// ClusterEgressFragmentLimit is the fragment limit per egress poll of
	// cluster-ping-pong. The Java, Luas and aeron-go cluster clients each fix
	// it at 10.
	ClusterEgressFragmentLimit = 10

	// ClusterMemberPortBase is the first port of member 0. Member i takes the
	// ClusterMemberPortStride ports from ClusterMemberPortBase +
	// ClusterMemberPortStride*i. java/bench/ClusterServer.java assigns the
	// ports within that range.
	ClusterMemberPortBase = 20310

	// ClusterMemberPortStride is the port count of one member.
	ClusterMemberPortStride = 10

	// ClusterIngressPortOffset is the offset of the ingress port within the
	// ports of one member.
	ClusterIngressPortOffset = 2

	// ClusterHost is the host of every cluster member.
	ClusterHost = "127.0.0.1"
)

// Hot thread indexes into Config.CPUs.
const (
	PublisherThread  = 0
	SubscriberThread = 1
	PingThread       = 0
	PongThread       = 1
)

// Scenario names one benchmark and the flag defaults it sets. ParseFlags
// starts from these defaults.
type Scenario struct {
	Name          string
	Channel       string
	MessageLength int
	FragmentLimit int

	// IsCluster marks a scenario that runs against the Java cluster.
	// ParseFlags then takes -cluster-members and checks the cluster limits.
	IsCluster bool

	// IsFixedRate marks a scenario that sends on a fixed schedule. ParseFlags
	// then takes -rate and -warmup-rate and checks the fixed-rate limits.
	IsFixedRate bool
}

// The transport, archive and cluster scenarios. Each client runs each
// scenario from one file.
var (
	IPCThroughput = Scenario{
		Name:          "ipc-throughput",
		Channel:       IPCChannel,
		MessageLength: DefaultMessageLength,
		FragmentLimit: ThroughputFragmentLimit,
	}
	UDPThroughput = Scenario{
		Name:          "udp-throughput",
		Channel:       UDPChannel,
		MessageLength: DefaultMessageLength,
		FragmentLimit: ThroughputFragmentLimit,
	}
	IPCPingPong = Scenario{
		Name:          "ipc-ping-pong",
		Channel:       IPCChannel,
		MessageLength: DefaultMessageLength,
		FragmentLimit: PingPongFragmentLimit,
	}
	UDPPingPong = Scenario{
		Name:          "udp-ping-pong",
		Channel:       UDPChannel,
		MessageLength: DefaultMessageLength,
		FragmentLimit: PingPongFragmentLimit,
	}
	IPCFixedRate = Scenario{
		Name:          "ipc-fixed-rate",
		Channel:       IPCChannel,
		MessageLength: DefaultMessageLength,
		FragmentLimit: FixedRateFragmentLimit,

		IsFixedRate: true,
	}
	UDPFixedRate = Scenario{
		Name:          "udp-fixed-rate",
		Channel:       UDPChannel,
		MessageLength: DefaultMessageLength,
		FragmentLimit: FixedRateFragmentLimit,

		IsFixedRate: true,
	}
	ArchiveRecord = Scenario{
		Name:          "archive-record",
		Channel:       IPCChannel,
		MessageLength: ArchiveMessageLength,
		FragmentLimit: ThroughputFragmentLimit,
	}
	ArchiveReplay = Scenario{
		Name:          "archive-replay",
		Channel:       IPCChannel,
		MessageLength: ArchiveMessageLength,
		FragmentLimit: ThroughputFragmentLimit,
	}
	ClusterPingPong = Scenario{
		Name:          "cluster-ping-pong",
		Channel:       ClusterIngressChannel,
		MessageLength: DefaultMessageLength,
		FragmentLimit: ClusterEgressFragmentLimit,

		IsCluster: true,
	}
)

// ClusterIngressEndpoints returns the ingress endpoint of each member of a
// cluster of the given size, for example
// "0=127.0.0.1:20312,1=127.0.0.1:20322".
func ClusterIngressEndpoints(members int) string {
	endpoints := make([]string, members)
	for id := range members {
		port := ClusterMemberPortBase + ClusterMemberPortStride*id + ClusterIngressPortOffset
		endpoints[id] = strconv.Itoa(id) + "=" + ClusterHost + ":" + strconv.Itoa(port)
	}
	return strings.Join(endpoints, ",")
}
