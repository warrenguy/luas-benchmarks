package bench

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// pinSample is one goroutine's CPU mask, read while that goroutine is still
// locked. tid is the OS thread. The two hot goroutines must not share it.
type pinSample struct {
	cpus       []int
	tid        int
	readErr    error
	restoreErr error
}

func TestRunPinnedPairPinsDistinctCPUs(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var original unix.CPUSet
	require.NoError(t, unix.SchedGetaffinity(0, &original))
	// Restore while this goroutine is still locked. unlockHot drops only the
	// PinThread lock, so the defer runs before the thread returns to the pool.
	defer func() { require.NoError(t, unix.SchedSetaffinity(0, &original)) }()

	allowed, err := ThreadCPUs()
	require.NoError(t, err)
	if len(allowed) < 2 {
		t.Skip("process CPU mask has fewer than 2 CPUs")
	}
	cfg := Config{
		CPUs:          []int{allowed[0], allowed[1]},
		UnlockThreads: true,
	}
	var mu sync.Mutex
	var samples []pinSample
	before := func() {
		cpus, readErr := ThreadCPUs()
		tid := unix.Gettid()
		restoreErr := unix.SchedSetaffinity(0, &original)
		mu.Lock()
		samples = append(samples, pinSample{cpus, tid, readErr, restoreErr})
		mu.Unlock()
	}
	ran := make(chan struct{})
	require.NoError(t, cfg.runPinnedPair(0, 1, before, func() { close(ran) }))
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("fn did not run")
	}

	require.Len(t, samples, 2)
	got := map[int]struct{}{}
	tids := map[int]struct{}{}
	for _, sample := range samples {
		require.NoError(t, sample.readErr)
		require.NoError(t, sample.restoreErr)
		require.Len(t, sample.cpus, 1)
		got[sample.cpus[0]] = struct{}{}
		tids[sample.tid] = struct{}{}
	}
	require.Len(t, tids, 2)
	_, hasFirst := got[allowed[0]]
	_, hasSecond := got[allowed[1]]
	require.True(t, hasFirst)
	require.True(t, hasSecond)
}

func TestRunPinnedPairEmptyCPUsStartsFn(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Config{}.runPinnedPair(0, 1, nil, func() {
			close(started)
			<-release
		})
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("runPinnedPair blocked in fn")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("fn did not start")
	}
	close(release)
}

func TestRunPinnedPairMissingIndex(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var original unix.CPUSet
	require.NoError(t, unix.SchedGetaffinity(0, &original))
	defer func() { require.NoError(t, unix.SchedSetaffinity(0, &original)) }()

	allowed, err := ThreadCPUs()
	require.NoError(t, err)
	require.NotEmpty(t, allowed)

	// Stay on this goroutine. t.Run starts another one, and unlockHot would
	// then return that goroutine's thread to the pool with the pin still set.
	ran := make(chan struct{}, 1)
	cfg := Config{CPUs: []int{allowed[0]}, UnlockThreads: true}
	err = cfg.runPinnedPair(1, 0, nil, func() { ran <- struct{}{} })
	require.ErrorContains(t, err, "pin hot thread 1")
	select {
	case <-ran:
		t.Fatal("fn ran")
	default:
	}
	got, err := ThreadCPUs()
	require.NoError(t, err)
	require.Equal(t, allowed, got)

	err = cfg.runPinnedPair(0, 1, nil, func() { ran <- struct{}{} })
	require.ErrorContains(t, err, "pin hot thread 1")
	select {
	case <-ran:
		t.Fatal("fn ran")
	default:
	}
}
