package main

import (
	"context"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runUDPFixedRate runs udp-fixed-rate: round trips at a fixed send rate over
// two UDP loopback streams.
func runUDPFixedRate(ctx context.Context, args []string) error {
	return runFixedRate(ctx, bench.UDPFixedRate, args)
}
