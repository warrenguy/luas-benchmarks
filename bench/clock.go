package bench

import "time"

var clockStart = time.Now()

// NanoTime returns monotonic nanoseconds since the process started. NanoTime
// does not allocate. Values compare only within one process.
func NanoTime() int64 {
	return int64(time.Since(clockStart))
}
