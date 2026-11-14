package bench

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ParseCPUList parses a Linux CPU list such as "2-4,7". Each entry is one CPU
// or an ascending range. ParseCPUList keeps the input order. ParseCPUList
// trims leading and trailing white space. ParseCPUList returns an empty
// list, not nil, for blank text.
//
// Returns an error when:
//   - an entry is empty or a descending range
//   - a CPU is not an integer from 0 to MaxCPU
//   - a CPU appears twice
func ParseCPUList(text string) ([]int, error) {
	text = strings.TrimSpace(text)
	cpus := []int{}
	if text == "" {
		return cpus, nil
	}
	seen := make(map[int]bool)
	for entry := range strings.SplitSeq(text, ",") {
		first, last, err := parseCPURange(entry)
		if err != nil {
			return nil, fmt.Errorf("parse cpu list %q: %w", text, err)
		}
		for cpu := first; cpu <= last; cpu++ {
			if seen[cpu] {
				return nil, fmt.Errorf("parse cpu list %q: cpu %d appears twice", text, cpu)
			}
			seen[cpu] = true
			cpus = append(cpus, cpu)
		}
	}
	return cpus, nil
}

// MaxCPU is the highest CPU number that unix.CPUSet holds.
const MaxCPU = 1023

// parseCPURange parses one CPU list entry: "N" or "N-M" with N <= M.
func parseCPURange(entry string) (first, last int, err error) {
	low, high, isRange := strings.Cut(entry, "-")
	first, err = parseCPU(low)
	if err != nil {
		return 0, 0, err
	}
	if !isRange {
		return first, first, nil
	}
	last, err = parseCPU(high)
	if err != nil {
		return 0, 0, err
	}
	if last < first {
		return 0, 0, fmt.Errorf("descending range %q", entry)
	}
	return first, last, nil
}

func parseCPU(text string) (int, error) {
	if text == "" {
		return 0, errors.New("empty entry")
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("cpu %q: want a non-negative integer", text)
		}
	}
	cpu, err := strconv.Atoi(text)
	if err != nil || cpu > MaxCPU {
		return 0, fmt.Errorf("cpu %q: above %d", text, MaxCPU)
	}
	return cpu, nil
}

// FormatCPUList formats cpus as a Linux CPU list. FormatCPUList writes each
// ascending run of consecutive CPUs as a range. For distinct CPUs from 0 to
// MaxCPU, ParseCPUList(FormatCPUList(cpus)) returns cpus.
func FormatCPUList(cpus []int) string {
	var b strings.Builder
	for i := 0; i < len(cpus); {
		end := i
		for end+1 < len(cpus) && cpus[end+1] == cpus[end]+1 {
			end++
		}
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(cpus[i]))
		if end > i {
			b.WriteByte('-')
			b.WriteString(strconv.Itoa(cpus[end]))
		}
		i = end + 1
	}
	return b.String()
}
