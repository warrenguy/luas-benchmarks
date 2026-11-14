package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/warrenguy/luas/aeron"
)

const (
	connectChannel  = "aeron:ipc"
	connectStreamID = 1001
	connectTimeout  = 10 * time.Second
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

	client, err := aeron.Connect(ctx, aeron.Config{AeronDir: aeronDir})
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { err = errors.Join(err, client.Close()) }()

	publication, err := client.AddPublication(connectStreamID, connectChannel)
	if err != nil {
		return fmt.Errorf("add publication: %w", err)
	}
	subscription, err := client.AddSubscription(connectStreamID, connectChannel)
	if err != nil {
		return fmt.Errorf("add subscription: %w", err)
	}
	if err := publication.WaitUntilConnected(ctx); err != nil {
		return fmt.Errorf("wait for publication: %w", err)
	}
	if err := subscription.WaitUntilImageCount(ctx, 1); err != nil {
		return fmt.Errorf("wait for image: %w", err)
	}
	if _, err := publication.OfferWithRetry(ctx, connectPayload); err != nil {
		return fmt.Errorf("offer: %w", err)
	}
	received, err := subscription.PollOneMessage(ctx)
	if err != nil {
		return fmt.Errorf("poll: %w", err)
	}
	if !bytes.Equal(received, connectPayload) {
		return fmt.Errorf("payload mismatch: got %q, want %q", received, connectPayload)
	}
	fmt.Printf("luas connect: round-tripped %d bytes\n", len(received))
	return nil
}
