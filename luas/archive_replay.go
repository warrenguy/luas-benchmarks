package main

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/warrenguy/luas/aeron"
	"github.com/warrenguy/luas/archive"
	"github.com/warrenguy/luas/atomicbuffer"
	"github.com/warrenguy/luas/idlestrategy"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runArchiveReplay runs archive-replay. The archive first records the warmup
// and measured messages of one IPC publication, outside the clock. Each pass
// then replays its part of the finished recording to one subscription on the
// calling goroutine. The clock starts before the replay request. The clock
// stops when the subscriber holds the last message.
func runArchiveReplay(ctx context.Context, args []string) (err error) {
	cfg, err := bench.ParseFlagsWithUnlock(bench.ArchiveReplay, args)
	if err != nil {
		return err
	}
	setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	client, err := connect(setupCtx, cfg)
	if err != nil {
		return err
	}
	defer closeClient(client, &err)
	archiveClient, err := connectArchive(setupCtx, client)
	if err != nil {
		return err
	}
	defer closeArchive(archiveClient, &err)
	positions, recordingID, err := recordReplaySource(setupCtx, ctx, cfg, client, archiveClient)
	if err != nil {
		return err
	}
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()
	sub, err := client.AddSubscription(bench.ReplayStreamID, cfg.Channel)
	if err != nil {
		return fmt.Errorf("add replay subscription: %w", err)
	}

	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	if err := cfg.PinHotThread(bench.SubscriberThread); err != nil {
		return err
	}

	checker := bench.NewSequenceChecker(cfg.MessageLength)
	handler := func(buffer atomicbuffer.AtomicBuffer, offset, length int, _ *aeron.Header) {
		checker.Check(int64(binary.LittleEndian.Uint64(buffer.Bytes()[offset:])), length)
	}
	if cfg.IsFragmented() {
		handler = aeron.NewFragmentAssembler(handler).Handler()
	}
	replayChannel := []byte(cfg.Channel)
	duty := newConductorDuty(client, cfg, bench.SubscriberThread, bench.HotRepin{})
	pass := func(count int64) error {
		received := checker.Received()
		from := positions[received]
		target := received + count
		if _, err := archiveClient.StartReplay(
			ctx, recordingID, from, positions[target]-from, bench.ReplayStreamID, replayChannel,
		); err != nil {
			return fmt.Errorf("start replay: %w", err)
		}
		for checker.Received() < target {
			if err := checker.Err(); err != nil {
				return err
			}
			fragments := sub.Poll(handler, cfg.FragmentLimit)
			duty.tick()
			idlestrategy.BusySpin.Idle(fragments)
		}
		return nil
	}

	measurement, err := bench.Measure(cfg, pass, pass)
	if err != nil {
		return err
	}
	throughput := bench.NewThroughput(cfg.Messages, cfg.MessageLength, measurement.ElapsedNs)
	return bench.WriteJSON(cfg.Output, bench.Result{
		Scenario:   bench.ArchiveReplay.Name,
		Client:     clientName,
		Variant:    cfg.Variant(),
		Config:     cfg,
		Throughput: &throughput,
		GoRuntime:  &measurement.GoRuntime,
	})
}

// recordReplaySource records the warmup and measured messages.
// recordReplaySource then stops the recording and closes the recorded
// publication. setupCtx bounds the start of the recording. ctx bounds the
// stop.
//
// Returns the recording id and the recorded position at each pass
// boundary, keyed by message count: 0, cfg.WarmupMessages and the total.
func recordReplaySource(
	setupCtx context.Context,
	ctx context.Context,
	cfg bench.Config,
	client *aeron.Client,
	archiveClient *archive.Client,
) (
	positions map[int64]int64,
	recordingID int64,
	err error,
) {
	source, err := startRecording(setupCtx, cfg, client, archiveClient)
	if err != nil {
		return nil, 0, err
	}
	defer source.close(ctx, &err)
	positions = map[int64]int64{0: source.startPosition}
	for _, count := range []int64{cfg.WarmupMessages, cfg.WarmupMessages + cfg.Messages} {
		position, recordErr := source.record(count - source.sent)
		if recordErr != nil {
			return nil, 0, recordErr
		}
		positions[count] = position
	}
	return positions, source.recordingID, nil
}
