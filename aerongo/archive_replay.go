package main

import (
	"context"
	"fmt"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	"github.com/lirm/aeron-go/archive"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runArchiveReplay runs archive-replay. The archive first records the warmup
// and measured messages of one IPC publication, outside the clock. Each pass
// then replays its part of the finished recording to one subscription on the
// calling goroutine. The clock starts before the replay request. The clock
// stops when the subscriber holds the last message.
func runArchiveReplay(ctx context.Context, args []string) (err error) {
	cfg, err := bench.ParseFlags(bench.ArchiveReplay, args)
	if err != nil {
		return err
	}
	setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	arch, err := connectArchive(cfg)
	if err != nil {
		return err
	}
	defer closeArchive(arch, &err)
	positions, recordingID, err := recordReplaySource(setupCtx, cfg, arch)
	if err != nil {
		return err
	}
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()
	sub, err := arch.Aeron().AddSubscription(cfg.Channel, bench.ReplayStreamID)
	if err != nil {
		return fmt.Errorf("add replay subscription: %w", err)
	}

	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	if err := cfg.PinHotThread(bench.SubscriberThread); err != nil {
		return err
	}

	checker := bench.NewSequenceChecker(cfg.MessageLength)
	handler := func(buffer *atomic.Buffer, offset, length int32, _ *logbuffer.Header) {
		checker.Check(buffer.GetInt64(offset), int(length))
	}
	if cfg.IsFragmented() {
		handler = aeron.NewFragmentAssembler(
			handler,
			aeron.DefaultFragmentAssemblyBufferLength,
		).OnFragment
	}
	pass := func(count int64) error {
		received := checker.Received()
		from := positions[received]
		target := received + count
		if _, err := arch.StartReplay(
			recordingID,
			from,
			positions[target]-from,
			cfg.Channel,
			bench.ReplayStreamID,
		); err != nil {
			return fmt.Errorf("start replay: %w", err)
		}
		for checker.Received() < target {
			if err := checker.Err(); err != nil {
				return err
			}
			idle.Idle(sub.Poll(handler, cfg.FragmentLimit))
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
// publication. ctx bounds the start of the recording.
//
// Returns the recording id and the recorded position at each pass
// boundary, keyed by message count: 0, cfg.WarmupMessages and the total.
func recordReplaySource(
	ctx context.Context,
	cfg bench.Config,
	arch *archive.Archive,
) (
	positions map[int64]int64,
	recordingID int64,
	err error,
) {
	source, err := startRecording(ctx, cfg, arch)
	if err != nil {
		return nil, 0, err
	}
	defer source.close(&err)
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
