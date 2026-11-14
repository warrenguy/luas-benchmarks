package main

import (
	"context"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runIPCPingPong runs ipc-ping-pong: round trips over two IPC streams.
func runIPCPingPong(ctx context.Context, args []string) error {
	return runPingPong(ctx, bench.IPCPingPong, args)
}
