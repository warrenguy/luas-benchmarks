// Package cells loads benchmark run directories and groups their results
// into rows and cells. A row holds one scenario variant. A cell holds the
// runs of one client in one row.
//
// The package aggregates values. It does not format output. Command report
// builds its tables from these rows.
//
// A latency cell reads each percentile from the merged histogram of its
// runs, not from the percentiles of each run.
package cells
