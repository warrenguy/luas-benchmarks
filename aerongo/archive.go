package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/counters"
	"github.com/lirm/aeron-go/archive"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// recorder offers messages on a publication that the archive records
// locally. Each message holds its sequence number in the first 8 bytes. The
// sequence continues over every record call.
//
// Concurrency: one goroutine at a time.
type recorder struct {
	archive     *archive.Archive
	publication *aeron.Publication
	sender      *sender
	channel     string
	streamID    int32

	// recordingID is the archive recording id of the publication.
	recordingID int64

	// startPosition is the recorded position before the first message.
	startPosition int64

	counters  *counters.Reader
	counterID int32
	message   *atomic.Buffer
	sent      int64
}

// connectArchive connects an aeron-go client and an archive control session
// to the driver in cfg.AeronDir. Every client uses the same control channels
// and a busy-spin idle strategy.
//
// The archive connects through AuthConnectRequest with empty credentials.
// The default ConnectRequest no longer exists in the archive protocol of the
// pinned Aeron.
//
// Ownership: the caller closes the archive. Close also closes its client.
func connectArchive(cfg bench.Config) (*archive.Archive, error) {
	options := archive.DefaultOptions()
	options.RequestChannel = bench.ArchiveControlRequestChannel
	options.RequestStream = bench.ArchiveControlRequestStreamID
	options.ResponseChannel = bench.ArchiveControlResponseChannel
	options.ResponseStream = bench.ArchiveControlResponseStreamID
	options.Timeout = setupTimeout
	options.IdleStrategy = idle
	options.AuthEnabled = true
	arch, err := archive.NewArchive(
		options,
		aeron.NewContext().AeronDir(cfg.AeronDir).MediaDriverTimeout(setupTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("connect archive: %w", err)
	}
	return arch, nil
}

// startRecording adds a publication on cfg.Channel and cfg.StreamID.
// startRecording then starts a local recording of the publication.
// startRecording waits for the recording position counter.
//
// Ownership: the caller closes arch. The caller calls close after a nil
// error. On error, startRecording closes the publication. A step that fails
// after the recording starts also stops the recording.
func startRecording(
	ctx context.Context,
	cfg bench.Config,
	arch *archive.Archive,
) (
	_ *recorder,
	err error,
) {
	pub, err := addPublication(arch.Aeron(), cfg, cfg.StreamID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, pub.Close())
		}
	}()
	r := &recorder{
		archive:     arch,
		publication: pub,
		sender:      newSender(pub, cfg, &bench.Worker{}),
		channel:     cfg.Channel,
		streamID:    cfg.StreamID,
		counters:    arch.Aeron().CounterReader(),
		message:     atomic.MakeBuffer(make([]byte, cfg.MessageLength)),
	}
	if _, err := arch.StartRecording(cfg.Channel, cfg.StreamID, true, false); err != nil {
		return nil, fmt.Errorf("start recording: %w", err)
	}
	defer func() {
		if err != nil {
			r.stopRecording(&err)
		}
	}()
	if err := waitUntil(ctx, "recording position counter", func() bool {
		r.counterID = archive.FindCounterIdBySession(r.counters, pub.SessionID())
		return r.counterID != counters.NullCounterId
	}); err != nil {
		return nil, err
	}
	r.recordingID = archive.GetRecordingId(r.counters, r.counterID)
	if err := waitUntilConnected(ctx, []*aeron.Publication{pub}, nil); err != nil {
		return nil, err
	}
	r.startPosition = r.counters.GetCounterValue(r.counterID)
	return r, nil
}

// record offers count messages. record then waits until the recorded
// position reaches the publication position. record does not allocate.
//
// Returns the recorded position after the last message. Returns a TryClaim
// panic as an error.
func (r *recorder) record(count int64) (position int64, err error) {
	defer recoverPanic(&err)
	length := r.message.Capacity()
	for range count {
		r.message.PutInt64(0, r.sent)
		if err := r.sender.send(r.message, 0, length); err != nil {
			return 0, err
		}
		r.sent++
	}
	target := r.publication.Position()
	for r.counters.GetCounterValue(r.counterID) < target {
		spinIdle()
	}
	return target, nil
}

// close stops the recording, then closes the publication. close joins each
// error to err.
func (r *recorder) close(err *error) {
	r.stopRecording(err)
	*err = errors.Join(*err, r.publication.Close())
}

// stopRecording stops the recording and joins the error to err.
func (r *recorder) stopRecording(err *error) {
	if stopErr := r.archive.StopRecording(r.channel, r.streamID); stopErr != nil {
		*err = errors.Join(*err, fmt.Errorf("stop recording: %w", stopErr))
	}
}

// closeArchive closes the archive control session and its client. The
// error joins err.
func closeArchive(arch *archive.Archive, err *error) {
	*err = errors.Join(*err, arch.Close())
}
