package bench_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

func TestResultJSONRoundTrip(t *testing.T) {
	config := bench.Config{
		AeronDir:       "/dev/shm/luas-benchmarks",
		Channel:        "aeron:udp?endpoint=localhost:20121",
		StreamID:       1001,
		PongStreamID:   1002,
		MessageLength:  256,
		Messages:       1000,
		WarmupMessages: 100,
		FragmentLimit:  10,
		Publication:    "exclusive",
		SendMethod:     "offer",
		CPUs:           []int{2, 3},
	}
	unlockedConfig := config
	unlockedConfig.UnlockThreads = true
	fixedRateConfig := config
	fixedRateConfig.Rate = 1000
	fixedRateConfig.WarmupRate = 100
	tests := []struct {
		name   string
		result bench.Result
	}{
		{
			name: "go throughput",
			result: bench.Result{
				Scenario: "ipc-throughput",
				Client:   "luas",
				Variant:  "exclusive-offer-256",
				Config:   unlockedConfig,
				Throughput: &bench.Throughput{
					Messages:           1000,
					Bytes:              256000,
					ElapsedNs:          5000,
					MessagesPerSecond:  2e8,
					MegabytesPerSecond: 51200,
				},
				GoRuntime: &bench.GoRuntime{
					GOMAXPROCS:            2,
					Allocations:           3,
					AllocationsPerMessage: 0.003,
					GCCycles:              1,
					GCPauseNs:             1200,
				},
			},
		},
		{
			name: "java latency",
			result: bench.Result{
				Scenario: "ipc-ping-pong",
				Client:   "java",
				Variant:  "exclusive-offer-256",
				Config:   config,
				Latency: &bench.Latency{
					Count:     1000,
					P50Ns:     1000,
					P90Ns:     2000,
					P99Ns:     3000,
					P999Ns:    4000,
					P9999Ns:   4500,
					P99999Ns:  4800,
					MaxNs:     5000,
					Histogram: "HISTFAAAACB4nJNpmSzMwMDAwQABzFCaEUzOmDFjBgAeTAJ6",
				},
				JVMRuntime: &bench.JVMRuntime{GCCount: 0, GCTimeMs: 0},
			},
		},
		{
			name: "go fixed rate",
			result: bench.Result{
				Scenario: "ipc-fixed-rate",
				Client:   "aerongo",
				Variant:  "exclusive-offer-256-rate-1000",
				Config:   fixedRateConfig,
				Latency: &bench.Latency{
					Count:    990,
					P50Ns:    1000,
					P90Ns:    2000,
					P99Ns:    3000,
					P999Ns:   4000,
					P9999Ns:  4500,
					P99999Ns: 4800,
					MaxNs:    5000,
				},
				FixedRate: &bench.FixedRate{
					Expected:  1000,
					Sent:      995,
					Received:  990,
					Histogram: "HISTFAAAACB4nJNpmSzMwMDAwQABzFCaEUzOmDFjBgAeTAJ6",
				},
				GoRuntime: &bench.GoRuntime{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.json")
			require.NoError(t, bench.WriteJSON(path, tt.result))

			var got bench.Result
			require.NoError(t, bench.ReadJSON(path, &got))
			require.Equal(t, tt.result, got)
		})
	}
}

func TestReadJSONRejects(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		isStrictOnly bool
	}{
		{name: "unknown field", data: `{"scenario": "ipc-throughput", "unknown": 1}`, isStrictOnly: true},
		{name: "trailing data", data: `{"scenario": "ipc-throughput"} {"scenario": "ipc-throughput"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.json")
			require.NoError(t, os.WriteFile(path, []byte(tt.data), 0o644))

			var got bench.Result
			require.Error(t, bench.ReadJSONStrict(path, &got))
			err := bench.ReadJSON(path, &got)
			if tt.isStrictOnly {
				require.NoError(t, err)
				require.Equal(t, "ipc-throughput", got.Scenario)
				return
			}
			require.Error(t, err)
		})
	}
}
