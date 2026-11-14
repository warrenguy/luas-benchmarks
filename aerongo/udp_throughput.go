package main

import (
	"context"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runUDPThroughput runs udp-throughput: one publisher and one subscriber on one
// UDP loopback stream.
func runUDPThroughput(ctx context.Context, args []string) error {
	return runThroughput(ctx, bench.UDPThroughput, args)
}
