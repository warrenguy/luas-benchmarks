package bench_test

import (
	"flag"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

func TestParseFlags(t *testing.T) {
	defaults := bench.Config{
		AeronDir:       "/dev/shm/from-env",
		Channel:        bench.IPCChannel,
		StreamID:       bench.DefaultStreamID,
		PongStreamID:   bench.DefaultPongStreamID,
		MessageLength:  bench.DefaultMessageLength,
		Messages:       bench.DefaultMessages,
		WarmupMessages: bench.DefaultWarmupMessages,
		FragmentLimit:  bench.ThroughputFragmentLimit,
		Publication:    bench.ConcurrentPublication,
		SendMethod:     bench.SendOffer,
		CPUs:           []int{},
	}
	leadingZero := defaults
	leadingZero.StreamID = 10
	minLength := defaults
	minLength.MessageLength = bench.MinMessageLength
	clusterDefaults := defaults
	clusterDefaults.Channel = bench.ClusterIngressChannel
	clusterDefaults.FragmentLimit = bench.ClusterEgressFragmentLimit
	clusterDefaults.ClusterMembers = bench.DefaultClusterMembers
	threeMembers := clusterDefaults
	threeMembers.ClusterMembers = 3
	threeMembers.Publication = bench.ExclusivePublication
	fixedRateDefaults := defaults
	fixedRateDefaults.FragmentLimit = bench.FixedRateFragmentLimit
	fixedRateDefaults.Rate = bench.DefaultRate
	fixedRateDefaults.WarmupRate = bench.DefaultWarmupRate
	udpFixedRateDefaults := fixedRateDefaults
	udpFixedRateDefaults.Channel = bench.UDPChannel
	fixedRates := fixedRateDefaults
	fixedRates.Rate = 101000
	fixedRates.WarmupRate = 1000
	fixedRates.MessageLength = bench.MinFixedRateMessageLength
	scenarioDefaults := func(channel string, messageLength, fragmentLimit int) bench.Config {
		cfg := defaults
		cfg.Channel = channel
		cfg.MessageLength = messageLength
		cfg.FragmentLimit = fragmentLimit
		return cfg
	}
	tests := []struct {
		name     string
		scenario bench.Scenario
		args     []string

		// canUnlock parses with ParseFlagsWithUnlock.
		canUnlock bool
		want      bench.Config
		wantErr   bool
	}{
		{name: "defaults", args: nil, want: defaults},
		{
			name:     "udp-throughput defaults",
			scenario: bench.UDPThroughput,
			want: scenarioDefaults(
				bench.UDPChannel,
				bench.DefaultMessageLength,
				bench.ThroughputFragmentLimit,
			),
		},
		{
			name:     "ipc-ping-pong defaults",
			scenario: bench.IPCPingPong,
			want: scenarioDefaults(
				bench.IPCChannel,
				bench.DefaultMessageLength,
				bench.PingPongFragmentLimit,
			),
		},
		{
			name:     "udp-ping-pong defaults",
			scenario: bench.UDPPingPong,
			want: scenarioDefaults(
				bench.UDPChannel,
				bench.DefaultMessageLength,
				bench.PingPongFragmentLimit,
			),
		},
		{
			name:     "archive-record defaults",
			scenario: bench.ArchiveRecord,
			want: scenarioDefaults(
				bench.IPCChannel,
				bench.ArchiveMessageLength,
				bench.ThroughputFragmentLimit,
			),
		},
		{
			name:     "archive-replay defaults",
			scenario: bench.ArchiveReplay,
			want: scenarioDefaults(
				bench.IPCChannel,
				bench.ArchiveMessageLength,
				bench.ThroughputFragmentLimit,
			),
		},
		{
			name:     "cluster-ping-pong defaults",
			scenario: bench.ClusterPingPong,
			want:     clusterDefaults,
		},
		{
			name:     "cluster-ping-pong of 3 members",
			scenario: bench.ClusterPingPong,
			args:     []string{"-cluster-members", "3", "-publication", "exclusive"},
			want:     threeMembers,
		},
		{
			name:     "cluster-ping-pong of 0 members",
			scenario: bench.ClusterPingPong,
			args:     []string{"-cluster-members", "0"},
			wantErr:  true,
		},
		{
			name:     "cluster-ping-pong try-claim",
			scenario: bench.ClusterPingPong,
			args:     []string{"-send-method", "try-claim"},
			wantErr:  true,
		},
		{
			name:     "cluster-ping-pong fragment limit 1",
			scenario: bench.ClusterPingPong,
			args:     []string{"-fragment-limit", "1"},
			wantErr:  true,
		},
		{
			name:     "cluster-ping-pong stream id 1005",
			scenario: bench.ClusterPingPong,
			args:     []string{"-stream-id", "1005"},
			wantErr:  true,
		},
		{
			name:    "cluster members outside cluster-ping-pong",
			args:    []string{"-cluster-members", "1"},
			wantErr: true,
		},
		{name: "ipc-fixed-rate defaults", scenario: bench.IPCFixedRate, want: fixedRateDefaults},
		{name: "udp-fixed-rate defaults", scenario: bench.UDPFixedRate, want: udpFixedRateDefaults},
		{
			name:     "ipc-fixed-rate rates",
			scenario: bench.IPCFixedRate,
			args:     []string{"-rate", "101000", "-warmup-rate", "1000", "-message-length", "16"},
			want:     fixedRates,
		},
		{name: "zero rate", scenario: bench.IPCFixedRate, args: []string{"-rate", "0"}, wantErr: true},
		{
			name:     "rate above max",
			scenario: bench.IPCFixedRate,
			args:     []string{"-rate", "1000000001"},
			wantErr:  true,
		},
		{
			name:     "zero warmup rate",
			scenario: bench.IPCFixedRate,
			args:     []string{"-warmup-rate", "0"},
			wantErr:  true,
		},
		{
			name:     "fixed-rate message length below 16",
			scenario: bench.IPCFixedRate,
			args:     []string{"-message-length", "15"},
			wantErr:  true,
		},
		{name: "rate outside a fixed-rate scenario", args: []string{"-rate", "1000"}, wantErr: true},
		{
			name:    "warmup rate outside a fixed-rate scenario",
			args:    []string{"-warmup-rate", "1000"},
			wantErr: true,
		},
		{name: "passes", args: []string{"-passes", "3"}, wantErr: true},
		{name: "unlock threads", args: []string{"-unlock-threads"}, wantErr: true},
		{
			name: "every flag",
			args: []string{
				"-aeron-dir", "/dev/shm/flag", "-channel", "aeron:udp?endpoint=localhost:20121",
				"-stream-id", "-7", "-pong-stream-id", "9", "-message-length=4096", "-messages", "500",
				"-warmup-messages", "0", "-fragment-limit", "1", "-publication", "exclusive",
				"-send-method", "try-claim", "-cpus", "4-5", "-unlock-threads", "-output", "out.json",
			},
			canUnlock: true,
			want: bench.Config{
				AeronDir:        "/dev/shm/flag",
				Channel:         "aeron:udp?endpoint=localhost:20121",
				StreamID:        -7,
				PongStreamID:    9,
				MessageLength:   4096,
				Messages:        500,
				WarmupMessages:  0,
				FragmentLimit:   1,
				Publication:     bench.ExclusivePublication,
				SendMethod:      bench.SendTryClaim,
				CPUs:            []int{4, 5},
				UnlockThreads:   true,
				YieldIntervalNs: bench.DefaultYieldIntervalNs,
				Output:          "out.json",
			},
		},
		{name: "empty cpu list", args: []string{"-cpus", ""}, want: defaults},
		{
			name: "leading zero stream id is decimal",
			args: []string{"-stream-id", "010"},
			want: leadingZero,
		},
		{name: "hex stream id", args: []string{"-stream-id", "0x10"}, wantErr: true},
		{name: "underscore messages", args: []string{"-messages", "1_000"}, wantErr: true},
		{
			name:    "message length above int32",
			args:    []string{"-message-length", "2147483648"},
			wantErr: true,
		},
		{
			name:    "fragment limit above int32",
			args:    []string{"-fragment-limit", "2147483648"},
			wantErr: true,
		},
		{name: "unknown flag", args: []string{"-size", "32"}, wantErr: true},
		{name: "positional argument", args: []string{"extra"}, wantErr: true},
		{name: "trailing terminator", args: []string{"--"}, want: defaults},
		{name: "argument after terminator", args: []string{"--", "extra"}, wantErr: true},
		{name: "stream id above int32", args: []string{"-stream-id", "2147483648"}, wantErr: true},
		{name: "bad cpu list", args: []string{"-cpus", "3-1"}, wantErr: true},
		{name: "message length 8", args: []string{"-message-length", "8"}, want: minLength},
		{name: "message length below 8", args: []string{"-message-length", "7"}, wantErr: true},
		{
			name:    "pong stream id equals stream id",
			args:    []string{"-pong-stream-id", "1001"},
			wantErr: true,
		},
		{name: "unknown publication", args: []string{"-publication", "shared"}, wantErr: true},
		{name: "unknown send method", args: []string{"-send-method", "tryclaim"}, wantErr: true},
		{name: "zero messages", args: []string{"-messages", "0"}, wantErr: true},
		{name: "negative warmup", args: []string{"-warmup-messages", "-1"}, wantErr: true},
		{name: "zero fragment limit", args: []string{"-fragment-limit", "0"}, wantErr: true},
		{name: "empty aeron dir", args: []string{"-aeron-dir", ""}, wantErr: true},
		{name: "empty channel", args: []string{"-channel", ""}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AERON_DIR", "/dev/shm/from-env")
			scenario := tt.scenario
			if scenario.Name == "" {
				scenario = bench.IPCThroughput
			}
			parse := bench.ParseFlags
			if tt.canUnlock {
				parse = bench.ParseFlagsWithUnlock
			}
			got, err := parse(scenario, tt.args)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseFlagsHelp(t *testing.T) {
	_, err := bench.ParseFlags(bench.IPCThroughput, []string{"-h"})
	require.ErrorIs(t, err, flag.ErrHelp)
}

func TestConfigVariant(t *testing.T) {
	tests := []struct {
		cfg  bench.Config
		want string
	}{
		{
			cfg: bench.Config{
				Publication:   bench.ExclusivePublication,
				SendMethod:    bench.SendTryClaim,
				MessageLength: 256,
			},
			want: "exclusive-try-claim-256",
		},
		{
			cfg: bench.Config{
				Publication:    bench.ExclusivePublication,
				SendMethod:     bench.SendOffer,
				MessageLength:  32,
				ClusterMembers: 3,
			},
			want: "exclusive-offer-32-members-3",
		},
		{
			cfg: bench.Config{
				Publication:   bench.ExclusivePublication,
				SendMethod:    bench.SendTryClaim,
				MessageLength: 32,
				Rate:          501000,
			},
			want: "exclusive-try-claim-32-rate-501000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			require.Equal(t, tt.want, tt.cfg.Variant())
		})
	}
}

func TestClusterIngressEndpoints(t *testing.T) {
	require.Equal(t, "0=127.0.0.1:20312", bench.ClusterIngressEndpoints(1))
	require.Equal(
		t,
		"0=127.0.0.1:20312,1=127.0.0.1:20322,2=127.0.0.1:20332",
		bench.ClusterIngressEndpoints(3),
	)
}

func TestConfigIsFragmented(t *testing.T) {
	tests := []struct {
		messageLength int
		want          bool
	}{
		{messageLength: 1024, want: false},
		{messageLength: bench.MaxPayloadLength, want: false},
		{messageLength: bench.MaxPayloadLength + 1, want: true},
		{messageLength: 4096, want: true},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.messageLength), func(t *testing.T) {
			require.Equal(t, tt.want, bench.Config{MessageLength: tt.messageLength}.IsFragmented())
		})
	}
}

func TestPinHotThreadWithoutCPUsDoesNothing(t *testing.T) {
	require.NoError(t, bench.Config{}.PinHotThread(0))
}

func TestPinHotThreadRejectsMissingCPU(t *testing.T) {
	require.Error(t, bench.Config{CPUs: []int{0}}.PinHotThread(1))
}
