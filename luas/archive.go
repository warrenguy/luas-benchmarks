package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/warrenguy/luas/aeron"
	"github.com/warrenguy/luas/aeron/cnc"
	"github.com/warrenguy/luas/archive"
	"github.com/warrenguy/luas/archive/codecs/archivewire"
	"github.com/warrenguy/luas/idlestrategy"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// recorder offers messages on a publication that the archive records
// locally. Each message holds its sequence number in the first 8 bytes. The
// sequence continues over every record call.
//
// Concurrency: invoker-only. The goroutine that polls client must call
// every method.
type recorder struct {
	archive     *archive.Client
	publication publication
	sender      *sender
	channel     []byte
	streamID    int32

	// recordingID is the archive recording id of the publication.
	recordingID int64

	// startPosition is the recorded position before the first message.
	startPosition int64

	position cnc.CounterSlot
	duty     conductorDuty
	message  []byte
	sent     int64
}

// connectArchive connects an archive control session on client. Every
// client uses the same control channels, a 10s control timeout and a
// busy-spin idle strategy.
func connectArchive(ctx context.Context, client *aeron.Client) (*archive.Client, error) {
	cfg := archive.Config{
		Aeron:                   client,
		ControlRequestChannel:   bench.ArchiveControlRequestChannel,
		ControlRequestStreamID:  bench.ArchiveControlRequestStreamID,
		ControlResponseChannel:  bench.ArchiveControlResponseChannel,
		ControlResponseStreamID: bench.ArchiveControlResponseStreamID,
		ConnectTimeout:          setupTimeout,
		ControlResponseTimeout:  setupTimeout,
		IdleStrategy:            idlestrategy.BusySpin,
	}.WithRemoteControl()
	archiveClient, err := archive.Connect(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect archive: %w", err)
	}
	return archiveClient, nil
}

// startRecording adds a publication on cfg.Channel and cfg.StreamID.
// startRecording then starts a local recording of the publication.
// startRecording waits for the recording position counter.
//
// Ownership: the caller closes client and archiveClient. The caller calls
// close after a nil error. On error, startRecording closes the publication.
// A step that fails after the recording starts also stops the recording.
func startRecording(
	ctx context.Context,
	cfg bench.Config,
	client *aeron.Client,
	archiveClient *archive.Client,
) (
	_ *recorder,
	err error,
) {
	pub, err := addPublication(client, cfg, cfg.StreamID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, pub.Close())
		}
	}()
	r := &recorder{
		archive:     archiveClient,
		publication: pub,
		channel:     []byte(cfg.Channel),
		streamID:    cfg.StreamID,
		duty:        newConductorDuty(client, cfg, bench.PublisherThread, bench.HotRepin{}),
		message:     make([]byte, cfg.MessageLength),
	}
	r.sender, err = newSender(pub, cfg, &bench.Worker{})
	if err != nil {
		return nil, err
	}
	tracker := archive.NewStartSignalTracker(archiveClient.ControlSessionID())
	r.recordingID, err = archiveClient.StartRecordingAwaitSignal(
		ctx, cfg.StreamID, archivewire.SourceLocation_LOCAL, false, r.channel, tracker,
	)
	if err != nil {
		return nil, fmt.Errorf("start recording: %w", err)
	}
	defer func() {
		if err != nil {
			r.stopRecording(context.WithoutCancel(ctx), &err)
		}
	}()
	counters, err := client.CountersReader()
	if err != nil {
		return nil, fmt.Errorf("counters reader: %w", err)
	}
	for {
		// cnc.NullValue matches any archive. One archive runs on the driver.
		slot, found := counters.FindRecordingPositionCounter(r.recordingID, cnc.NullValue)
		if found {
			r.position = slot
			break
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("wait for recording position counter: %w", err)
		}
		client.DoWork()
		spinIdle()
	}
	if err := waitUntilConnected(ctx, []publication{pub}, nil); err != nil {
		return nil, err
	}
	r.startPosition = r.position.Get()
	return r, nil
}

// record offers count messages. record then waits until the recorded
// position reaches the publication position. record does not allocate.
//
// Returns the recorded position after the last message.
func (r *recorder) record(count int64) (int64, error) {
	for range count {
		binary.LittleEndian.PutUint64(r.message, uint64(r.sent))
		if err := r.sender.send(r.message); err != nil {
			return 0, err
		}
		r.sent++
		r.duty.tick()
	}
	target := r.publication.Position()
	for r.position.Get() < target {
		r.duty.tick()
		spinIdle()
	}
	return target, nil
}

// close stops the recording, then closes the publication. close joins each
// error to err.
func (r *recorder) close(ctx context.Context, err *error) {
	r.stopRecording(ctx, err)
	*err = errors.Join(*err, r.publication.Close())
}

// stopRecording stops the recording and joins the error to err.
func (r *recorder) stopRecording(ctx context.Context, err *error) {
	if stopErr := r.archive.StopRecording(ctx, r.streamID, r.channel); stopErr != nil {
		*err = errors.Join(*err, fmt.Errorf("stop recording: %w", stopErr))
	}
}

// closeArchive closes the archive control session and joins its error to
// err.
func closeArchive(archiveClient *archive.Client, err *error) {
	*err = errors.Join(*err, archiveClient.Close())
}
