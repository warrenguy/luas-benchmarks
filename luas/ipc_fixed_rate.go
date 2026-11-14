package main

import (
	"context"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runIPCFixedRate runs ipc-fixed-rate: round trips at a fixed send rate over
// two IPC streams.
func runIPCFixedRate(ctx context.Context, args []string) error {
	return runFixedRate(ctx, bench.IPCFixedRate, args)
}
