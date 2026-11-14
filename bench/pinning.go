package bench

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

// PinThread locks the calling goroutine to its OS thread. PinThread then
// binds that thread to cpu.
//
// Lifetime: PinThread does not unlock the goroutine. The runtime ends the
// pinned thread when the goroutine exits. No other goroutine runs on that
// thread.
//
// Returns an error when cpu is outside 0 to MaxCPU or the kernel rejects the
// CPU set.
func PinThread(cpu int) error {
	if cpu < 0 || cpu > MaxCPU {
		return fmt.Errorf("pin thread: cpu %d outside 0-%d", cpu, MaxCPU)
	}
	runtime.LockOSThread()
	return setThreadCPU(cpu)
}

// setThreadCPU binds the calling OS thread to cpu. It does not lock the
// calling goroutine.
func setThreadCPU(cpu int) error {
	var set unix.CPUSet
	set.Set(cpu)
	if err := unix.SchedSetaffinity(0, &set); err != nil {
		return fmt.Errorf("pin thread to cpu %d: %w", cpu, err)
	}
	return nil
}

// ThreadCPUs returns the CPUs the calling OS thread may run on, in ascending
// order. Call it from a goroutine locked to its thread.
func ThreadCPUs() ([]int, error) {
	var set unix.CPUSet
	if err := unix.SchedGetaffinity(0, &set); err != nil {
		return nil, fmt.Errorf("read thread cpus: %w", err)
	}
	var cpus []int
	for cpu := range MaxCPU + 1 {
		if set.IsSet(cpu) {
			cpus = append(cpus, cpu)
		}
	}
	return cpus, nil
}
