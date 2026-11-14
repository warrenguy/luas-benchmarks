package main

import (
	"context"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runUDPPingPong runs udp-ping-pong: round trips over two UDP loopback streams.
func runUDPPingPong(ctx context.Context, args []string) error {
	return runPingPong(ctx, bench.UDPPingPong, args)
}
