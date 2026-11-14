package main

import (
	"context"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// runArchiveRecord runs archive-record: the archive records one IPC
// publication locally. The calling goroutine offers the messages. The clock
// stops when the recorded position reaches the publication position.
func runArchiveRecord(ctx context.Context, args []string) (err error) {
	cfg, err := bench.ParseFlags(bench.ArchiveRecord, args)
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
	source, err := startRecording(setupCtx, cfg, arch)
	if err != nil {
		return err
	}
	defer source.close(&err)
	// Stop the setup timer before the measured section. A timer that fires
	// there starts a goroutine, which allocates.
	cancel()

	// Pin after connect in every client. A thread the client starts takes
	// the CPU set of the thread that starts it.
	if err := cfg.PinHotThread(bench.PublisherThread); err != nil {
		return err
	}

	pass := func(count int64) error {
		_, err := source.record(count)
		return err
	}
	measurement, err := bench.Measure(cfg, pass, pass)
	if err != nil {
		return err
	}
	throughput := bench.NewThroughput(cfg.Messages, cfg.MessageLength, measurement.ElapsedNs)
	return bench.WriteJSON(cfg.Output, bench.Result{
		Scenario:   bench.ArchiveRecord.Name,
		Client:     clientName,
		Variant:    cfg.Variant(),
		Config:     cfg,
		Throughput: &throughput,
		GoRuntime:  &measurement.GoRuntime,
	})
}
