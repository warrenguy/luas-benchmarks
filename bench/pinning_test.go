package bench_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/warrenguy/luas-benchmarks/bench"
)

func TestPinThreadSetsAndReadsBackCPU(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var original unix.CPUSet
	require.NoError(t, unix.SchedGetaffinity(0, &original))
	defer func() { require.NoError(t, unix.SchedSetaffinity(0, &original)) }()

	allowed, err := bench.ThreadCPUs()
	require.NoError(t, err)
	require.Len(t, allowed, original.Count())
	cpu := allowed[len(allowed)-1]

	require.NoError(t, bench.PinThread(cpu))
	got, err := bench.ThreadCPUs()
	require.NoError(t, err)
	require.Equal(t, []int{cpu}, got)
}

func TestPinThreadRejectsOutOfRangeCPU(t *testing.T) {
	require.Error(t, bench.PinThread(-1))
	require.Error(t, bench.PinThread(bench.MaxCPU+1))
}
