// Package bench holds the code that the luas and aerongo binaries share:
// flag parsing, warmup, the run loop, the histogram, and thread pinning.
// The Java bench package mirrors it.
//
// The package also defines the result file and the run record. Package
// cells and the report commands use these types.
package bench
