package main

import (
	"context"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runIPCThroughput runs ipc-throughput: one publisher and one subscriber on one
// IPC stream.
func runIPCThroughput(ctx context.Context, args []string) error {
	return runThroughput(ctx, bench.IPCThroughput, args)
}
