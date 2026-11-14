package bench

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// RepinIntervalNs is the time between two CPU checks of one unlocked hot
// goroutine. Hot thread i of n checks at offset i*RepinIntervalNs/n, the
// offset of its yield at DefaultYieldIntervalNs.
const RepinIntervalNs int64 = DefaultYieldIntervalNs

// HotRepin keeps one unlocked hot goroutine on its CPU. An unlocked
// goroutine can leave its pinned thread. A check off its own CPU shows
// that it runs on another thread. HotRepin then pins that thread to the
// goroutine's CPU.
type HotRepin struct {
	schedule hotSchedule
	cpu      int
}

// HotRepin returns the CPU check of hot thread index. The check never runs
// without UnlockThreads or CPUs: a locked goroutine keeps its thread.
func (c Config) HotRepin(index int) HotRepin {
	if index < 0 || index >= len(c.CPUs) {
		return HotRepin{}
	}
	return HotRepin{schedule: c.hotSchedule(index, RepinIntervalNs), cpu: c.CPUs[index]}
}

// Thread is the OS thread that runs a hot goroutine. OSThread is the
// production Thread.
type Thread interface {
	// CPU returns the CPU the thread runs on, or -1 when the read fails.
	CPU() int

	// Pin binds the thread to cpu. The goroutine stays unlocked.
	Pin(cpu int) error
}

// OSThread is the OS thread of the calling goroutine. Its methods do not
// allocate.
type OSThread struct{}

// CPU returns CurrentCPU.
func (OSThread) CPU() int {
	return CurrentCPU()
}

// Pin binds the calling OS thread to cpu.
func (OSThread) Pin(cpu int) error {
	return setThreadCPU(cpu)
}

// Tick reads the CPU of thread when nowNs reaches the next check offset.
// Off the hot thread's own CPU, Tick pins thread to it. A failed pin leaves
// the thread as it was. The next check retries. A CPU outside 0 to MaxCPU
// pins nothing. Tick reports whether it checked. Tick does not allocate.
func (r *HotRepin) Tick(nowNs int64, thread Thread) bool {
	if !r.schedule.isDue(nowNs) {
		return false
	}
	cpu := thread.CPU()
	if cpu >= 0 && cpu <= MaxCPU && cpu != r.cpu {
		_ = thread.Pin(r.cpu)
	}
	return true
}

// CurrentCPU returns the CPU the calling thread runs on. It returns -1
// when getcpu fails. CurrentCPU makes a raw getcpu system call, so the
// scheduler does not see it. CurrentCPU does not allocate.
func CurrentCPU() int {
	var cpu uint32
	_, _, errno := unix.RawSyscall(unix.SYS_GETCPU, uintptr(unsafe.Pointer(&cpu)), 0, 0)
	if errno != 0 {
		return -1
	}
	return int(cpu)
}
