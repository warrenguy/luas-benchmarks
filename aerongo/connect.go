package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
)

const (
	connectChannel  = "aeron:ipc"
	connectStreamID = 1001
	connectTimeout  = 10 * time.Second
	connectIdle     = time.Millisecond
)

var connectPayload = []byte("luas-benchmarks connect check")

// runConnect connects to the driver in AERON_DIR. runConnect sends one message
// over IPC. runConnect then polls the same stream for that message. The
// connect check does not take flags.
func runConnect(ctx context.Context, args []string) (err error) {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q", args[0])
	}
	aeronDir := os.Getenv("AERON_DIR")
	if aeronDir == "" {
		return errors.New("AERON_DIR not set")
	}
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	client, err := aeron.Connect(
		aeron.NewContext().AeronDir(aeronDir).MediaDriverTimeout(connectTimeout),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { err = errors.Join(err, client.Close()) }()

	publication, err := client.AddPublication(connectChannel, connectStreamID)
	if err != nil {
		return fmt.Errorf("add publication: %w", err)
	}
	subscription, err := client.AddSubscription(connectChannel, connectStreamID)
	if err != nil {
		return fmt.Errorf("add subscription: %w", err)
	}
	if err := waitUntil(ctx, "connected publication and image", func() bool {
		return publication.IsConnected() && subscription.IsConnected()
	}); err != nil {
		return err
	}

	buffer := atomic.MakeBuffer(connectPayload)
	var lastResult int64
	if err := waitUntil(ctx, "offer", func() bool {
		lastResult = publication.Offer(buffer, 0, buffer.Capacity(), nil)
		return lastResult > 0
	}); err != nil {
		return fmt.Errorf("%w: last result %d", err, lastResult)
	}

	var received []byte
	handler := func(fragment *atomic.Buffer, offset, length int32, _ *logbuffer.Header) {
		received = fragment.GetBytesArray(offset, length)
	}
	if err := waitUntil(ctx, "poll", func() bool {
		return subscription.Poll(handler, 1) > 0
	}); err != nil {
		return err
	}
	if !bytes.Equal(received, connectPayload) {
		return fmt.Errorf("payload mismatch: got %q, want %q", received, connectPayload)
	}
	fmt.Printf("aerongo connect: round-tripped %d bytes\n", len(received))
	return nil
}

// waitUntil calls isDone until it returns true or ctx ends.
func waitUntil(ctx context.Context, what string, isDone func() bool) error {
	for !isDone() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for %s: %w", what, ctx.Err())
		case <-time.After(connectIdle):
		}
	}
	return nil
}
