package bench_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// fakeThread reads its CPU from cpu and records each pin.
type fakeThread struct {
	cpu  int
	pins []int
}

func (f *fakeThread) CPU() int {
	return f.cpu
}

func (f *fakeThread) Pin(cpu int) error {
	f.pins = append(f.pins, cpu)
	f.cpu = cpu
	return nil
}

func TestHotRepinTick(t *testing.T) {
	const interval = bench.RepinIntervalNs
	unlocked := bench.Config{CPUs: []int{4, 5}, UnlockThreads: true}
	tests := []struct {
		name  string
		cfg   bench.Config
		index int
		ticks []int64
		cpus  []int
		want  []bool
		// wantPins lists the CPU of each re-pin.
		wantPins []int
	}{
		{
			name:  "locked threads never check",
			cfg:   bench.Config{CPUs: []int{4, 5}},
			index: bench.PingThread,
			ticks: []int64{0, interval},
			cpus:  []int{7, 7},
			want:  []bool{false, false},
		},
		{
			name:  "unpinned threads never check",
			cfg:   bench.Config{UnlockThreads: true},
			index: bench.PingThread,
			ticks: []int64{0, interval},
			cpus:  []int{7, 7},
			want:  []bool{false, false},
		},
		{
			name:  "own CPU keeps the pin",
			cfg:   unlocked,
			index: bench.PingThread,
			ticks: []int64{0, 1, interval, 2 * interval},
			cpus:  []int{4, 4, 4, 4},
			want:  []bool{true, false, true, true},
		},
		{
			name:     "other CPUs re-pin",
			cfg:      unlocked,
			index:    bench.PongThread,
			ticks:    []int64{interval / 2, 3 * interval / 2, 5 * interval / 2, 7 * interval / 2},
			cpus:     []int{5, 0, 23, 5},
			want:     []bool{true, true, true, true},
			wantPins: []int{5, 5},
		},
		{
			name:     "CPU of another hot thread re-pins",
			cfg:      unlocked,
			index:    bench.PingThread,
			ticks:    []int64{0, interval},
			cpus:     []int{5, 4},
			want:     []bool{true, true},
			wantPins: []int{4},
		},
		{
			name:  "second thread checks half an interval later",
			cfg:   unlocked,
			index: bench.PongThread,
			ticks: []int64{0, interval/2 - 1, interval / 2},
			cpus:  []int{5, 5, 5},
			want:  []bool{false, false, true},
		},
		{
			name:  "failed CPU read pins nothing",
			cfg:   unlocked,
			index: bench.PingThread,
			ticks: []int64{0, interval},
			cpus:  []int{-1, bench.MaxCPU + 1},
			want:  []bool{true, true},
		},
		{
			name:  "index past the CPU list never checks",
			cfg:   unlocked,
			index: 2,
			ticks: []int64{0, interval},
			cpus:  []int{7, 7},
			want:  []bool{false, false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repin := tt.cfg.HotRepin(tt.index)
			thread := &fakeThread{}
			got := make([]bool, len(tt.ticks))
			for i, nowNs := range tt.ticks {
				thread.cpu = tt.cpus[i]
				got[i] = repin.Tick(nowNs, thread)
			}
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.wantPins, thread.pins)
		})
	}
}

// lockOnAllowedCPUs locks the test goroutine to its thread and restores the
// thread's CPU set at cleanup. It returns the CPUs the thread may run on.
func lockOnAllowedCPUs(t *testing.T) []int {
	t.Helper()
	runtime.LockOSThread()
	var original unix.CPUSet
	require.NoError(t, unix.SchedGetaffinity(0, &original))
	t.Cleanup(func() {
		require.NoError(t, unix.SchedSetaffinity(0, &original))
		runtime.UnlockOSThread()
	})
	allowed, err := bench.ThreadCPUs()
	require.NoError(t, err)
	return allowed
}

func TestHotRepinTickOnOSThreadDoesNotAllocate(t *testing.T) {
	allowed := lockOnAllowedCPUs(t)
	cpu := allowed[len(allowed)-1]
	require.NoError(t, bench.PinThread(cpu))
	repin := bench.Config{CPUs: []int{cpu}, UnlockThreads: true}.HotRepin(bench.PingThread)
	var nowNs int64
	allocations := testing.AllocsPerRun(100, func() {
		repin.Tick(nowNs, bench.OSThread{})
		nowNs += bench.RepinIntervalNs
	})
	require.Zero(t, allocations)
	require.Equal(t, cpu, bench.CurrentCPU())
}

func TestHotRepinTickRepinsOSThread(t *testing.T) {
	allowed := lockOnAllowedCPUs(t)
	if len(allowed) < 2 {
		t.Skip("needs 2 allowed CPUs")
	}
	from, to := allowed[0], allowed[len(allowed)-1]
	require.NoError(t, bench.PinThread(from))
	repin := bench.Config{CPUs: []int{to}, UnlockThreads: true}.HotRepin(bench.PingThread)

	require.True(t, repin.Tick(0, bench.OSThread{}))
	got, err := bench.ThreadCPUs()
	require.NoError(t, err)
	require.Equal(t, []int{to}, got)
	require.Equal(t, to, bench.CurrentCPU())
}

func TestCurrentCPUReadsPinnedCPU(t *testing.T) {
	allowed := lockOnAllowedCPUs(t)
	cpu := allowed[len(allowed)-1]
	require.NoError(t, bench.PinThread(cpu))
	require.Equal(t, cpu, bench.CurrentCPU())
}
