package bench

import "runtime"

// DefaultYieldIntervalNs is the YieldIntervalNs of an unlocked layout. It
// stays under the 10 ms after which the Go scheduler preempts a running
// goroutine.
const DefaultYieldIntervalNs int64 = 5_000_000

// HotYield yields one unlocked hot goroutine once per YieldIntervalNs.
//
// The Go scheduler preempts every goroutine that runs past 10 ms in one
// pass. Two hot goroutines then reach the global run queue together. One
// thread can take both and leave the other P idle until the next
// preemption, about 20 ms later. A goroutine that yields first never
// reaches the 10 ms limit. Each hot thread yields at its own offset within
// the interval, so two hot goroutines never yield together.
type HotYield struct {
	schedule hotSchedule
}

// HotYield returns the yield schedule of hot thread index. Hot thread i of
// n yields at offset i*YieldIntervalNs/n within each interval. The schedule
// never yields without UnlockThreads, CPUs or YieldIntervalNs: a locked
// goroutine hands off its thread on each yield.
func (c Config) HotYield(index int) HotYield {
	return HotYield{schedule: c.hotSchedule(index, c.YieldIntervalNs)}
}

// Tick calls runtime.Gosched when nowNs reaches the next yield offset. Tick
// reports whether it yielded. Tick does not allocate.
func (y *HotYield) Tick(nowNs int64) bool {
	if !y.schedule.isDue(nowNs) {
		return false
	}
	runtime.Gosched()
	return true
}

// hotSchedule fires once per interval at the offset of one unlocked hot
// thread. The zero hotSchedule never fires.
type hotSchedule struct {
	enabled    bool
	intervalNs int64
	phaseNs    int64
	nextNs     int64
}

// hotSchedule returns the schedule of hot thread index at intervalNs. Hot
// thread i of n fires at offset i*intervalNs/n within each interval. The
// schedule never fires without UnlockThreads, CPUs or intervalNs.
func (c Config) hotSchedule(index int, intervalNs int64) hotSchedule {
	if !c.UnlockThreads || len(c.CPUs) == 0 || intervalNs <= 0 {
		return hotSchedule{}
	}
	phaseNs := intervalNs * int64(index) / int64(len(c.CPUs))
	return hotSchedule{enabled: true, intervalNs: intervalNs, phaseNs: phaseNs, nextNs: phaseNs}
}

// isDue reports whether nowNs reaches the next offset. A due schedule moves
// to the first offset after nowNs.
func (s *hotSchedule) isDue(nowNs int64) bool {
	if !s.enabled || nowNs < s.nextNs {
		return false
	}
	s.nextNs = ((nowNs-s.phaseNs)/s.intervalNs+1)*s.intervalNs + s.phaseNs
	return true
}
