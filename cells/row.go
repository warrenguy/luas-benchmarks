package cells

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// Kind names what the results of a row measure.
type Kind int

const (
	// Throughput rows hold a message rate per run.
	Throughput Kind = iota

	// Latency rows hold the round trip histogram of a ping-pong or cluster
	// scenario.
	Latency

	// FixedRate rows hold the round trip histogram and the message counts of
	// a fixed-rate scenario.
	FixedRate
)

// String returns the name of k.
func (k Kind) String() string {
	switch k {
	case Throughput:
		return "throughput"
	case Latency:
		return "latency"
	case FixedRate:
		return "fixed-rate"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

// IsLatency reports whether rows of kind k hold a latency cell.
func (k Kind) IsLatency() bool {
	return k == Latency || k == FixedRate
}

// resultKind returns the kind of result.
func resultKind(result bench.Result) Kind {
	switch {
	case result.FixedRate != nil:
		return FixedRate
	case result.Latency != nil:
		return Latency
	default:
		return Throughput
	}
}

// ScenarioOrder lists the scenarios in row order. Other scenarios follow in
// name order.
var ScenarioOrder = []string{
	"ipc-throughput",
	"udp-throughput",
	"ipc-ping-pong",
	"udp-ping-pong",
	"ipc-fixed-rate",
	"udp-fixed-rate",
	"archive-record",
	"archive-replay",
	"cluster-ping-pong",
}

// ScenarioRank returns the index of scenario in ScenarioOrder, or the list
// length for a scenario outside it.
func ScenarioRank(scenario string) int {
	if index := slices.Index(ScenarioOrder, scenario); index >= 0 {
		return index
	}
	return len(ScenarioOrder)
}

// Row holds every run of one scenario variant, by client name.
type Row struct {
	Scenario string
	Variant  string
	Config   bench.Config
	Kind     Kind

	// Runs holds the results of each client in run order.
	Runs map[string][]bench.Result

	// Latency holds the cell of each client of a latency or fixed-rate row.
	// Nil in a throughput row.
	Latency map[string]*LatencyCell
}

// Group groups results into one row per scenario and variant, in row order.
// Group merges the histograms of each client of a latency or fixed-rate
// row. Each row keeps the results of a client in their order in results.
//
// Returns an error when the results of one row differ in kind, or a
// histogram is missing or does not decode.
func Group(results []bench.Result) ([]*Row, error) {
	byKey := make(map[[2]string]*Row)
	var rows []*Row
	for _, result := range results {
		kind := resultKind(result)
		key := [2]string{result.Scenario, result.Variant}
		row, found := byKey[key]
		if !found {
			row = &Row{
				Scenario: result.Scenario,
				Variant:  result.Variant,
				Config:   result.Config,
				Kind:     kind,
				Runs:     make(map[string][]bench.Result),
			}
			byKey[key] = row
			rows = append(rows, row)
		}
		if kind != row.Kind {
			return nil, fmt.Errorf(
				"%s %s: %s mixes %s and %s results",
				result.Scenario,
				result.Variant,
				result.Client,
				row.Kind,
				kind,
			)
		}
		row.Runs[result.Client] = append(row.Runs[result.Client], result)
	}
	for _, row := range rows {
		if !row.Kind.IsLatency() {
			continue
		}
		row.Latency = make(map[string]*LatencyCell)
		for name, runs := range row.Runs {
			cell, err := newLatencyCell(runs)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %s: %w", row.Scenario, row.Variant, name, err)
			}
			row.Latency[name] = cell
		}
	}
	slices.SortFunc(rows, compareRows)
	return rows, nil
}

// compareRows orders rows by scenario, publication, send method, cluster
// member count, message length and rate.
func compareRows(a, b *Row) int {
	return cmp.Or(
		cmp.Compare(ScenarioRank(a.Scenario), ScenarioRank(b.Scenario)),
		strings.Compare(a.Scenario, b.Scenario),
		strings.Compare(a.Config.Publication, b.Config.Publication),
		strings.Compare(a.Config.SendMethod, b.Config.SendMethod),
		cmp.Compare(a.Config.ClusterMembers, b.Config.ClusterMembers),
		cmp.Compare(a.Config.MessageLength, b.Config.MessageLength),
		cmp.Compare(a.Config.Rate, b.Config.Rate),
		strings.Compare(a.Variant, b.Variant),
	)
}
