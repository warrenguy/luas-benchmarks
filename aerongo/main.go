// Command aerongo runs one benchmark scenario on the aeron-go client.
//
// Usage: aerongo <scenario> [flags]
package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
)

// clientName names the client in each result.
const clientName = "aerongo"

var scenarios = map[string]func(context.Context, []string) error{
	"connect":           runConnect,
	"ipc-throughput":    runIPCThroughput,
	"udp-throughput":    runUDPThroughput,
	"ipc-ping-pong":     runIPCPingPong,
	"udp-ping-pong":     runUDPPingPong,
	"ipc-fixed-rate":    runIPCFixedRate,
	"udp-fixed-rate":    runUDPFixedRate,
	"archive-record":    runArchiveRecord,
	"archive-replay":    runArchiveReplay,
	"cluster-ping-pong": runClusterPingPong,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(
			os.Stderr,
			"usage: aerongo <scenario> [flags]\nscenarios: %s\n",
			strings.Join(slices.Sorted(maps.Keys(scenarios)), ", "),
		)
		os.Exit(2)
	}
	name := os.Args[1]
	run, ok := scenarios[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "aerongo: unknown scenario %q\n", name)
		os.Exit(2)
	}
	if err := run(context.Background(), os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "aerongo %s: %v\n", name, err)
		os.Exit(1)
	}
}
