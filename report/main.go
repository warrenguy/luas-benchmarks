// Command report prints a markdown comparison of benchmark runs.
//
// Usage:
//
//	report results/<run-id>
//	report -comparison <dir>...
//	report -check <result.json>...
//
// The first form prints the full report of each section of a directory. The
// second prints the comparison tables of each section of each directory and
// generates RESULTS.md. A run directory yields one section. An EC2 run
// directory yields one section per instance. The third form fails when a
// result file holds a field that the result type does not name.
package main

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/warrenguy/luas-benchmarks/bench"
	"github.com/warrenguy/luas-benchmarks/cells"
)

const usage = `usage: report results/<run-id>
       report -comparison <dir>...
       report -check <result.json>...`

func main() {
	comparisonOnly := flag.Bool(
		"comparison",
		false,
		"print the comparison tables of each run or EC2 run directory",
	)
	checkOnly := flag.Bool("check", false, "reject result files that hold an unknown field")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	var err error
	switch {
	case *checkOnly && !*comparisonOnly && len(args) > 0:
		err = checkResults(args)
	case *checkOnly:
		flag.Usage()
		os.Exit(2)
	case *comparisonOnly && len(args) > 0:
		err = writeComparisonReport(os.Stdout, args)
	case !*comparisonOnly && len(args) == 1:
		err = writeReport(os.Stdout, args[0])
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "report: %v\n", err)
		os.Exit(1)
	}
}

// checkResults returns an error when a result file in paths does not
// decode, or holds a field that bench.Result does not name.
func checkResults(paths []string) error {
	for _, path := range paths {
		var result bench.Result
		if err := bench.ReadJSONStrict(path, &result); err != nil {
			return err
		}
	}
	return nil
}

// writeReport writes the report of each section of the run or EC2 run
// directory dir to w. A section title names the instance of an EC2 run.
//
// Returns an error when dir does not resolve, a run directory does not
// load, the results of one row mix throughput and latency, or the write
// fails.
func writeReport(w io.Writer, dir string) error {
	sections, err := cells.Sections(dir)
	if err != nil {
		return err
	}
	var b strings.Builder
	anchors := make(headingAnchors)
	for i, s := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		if err := writeSectionReport(&b, s, anchors); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// writeSectionReport writes the report of section s: the run record, the
// run settings, the comparison tables and the full table. anchors counts
// the headings written before.
//
// Returns an error when the run directory does not load or the results of
// one row mix throughput and latency.
func writeSectionReport(b *strings.Builder, s cells.Section, anchors headingAnchors) error {
	record, results, rows, err := loadRows(s.Dir)
	if err != nil {
		return fmt.Errorf("%s: %w", s.Dir, err)
	}
	expectedRuns := scenarioRuns(rows)

	b.WriteString("# Luas, Java and aeron-go comparison")
	if s.Label != "" {
		b.WriteString(" on " + s.Label)
	}
	b.WriteString("\n\n")
	fmt.Fprintf(
		b,
		"Run `%s` at commit `%s`. See the README for the method and the differences between clients.\n\n",
		s.RunID(record),
		s.RunCommit(record),
	)
	b.WriteString("## Host\n\n")
	writeHostFacts(b, record, results)
	writeRun(b, record, results, expectedRuns)
	b.WriteString("## Comparison\n\n")
	b.WriteString(comparisonCaption)
	writeComparison(b, rows, anchors)
	b.WriteString("\n## Results\n\n")
	b.WriteString(
		"Each cell shows the median, then the min-max spread. `n=` marks a cell with fewer runs. " +
			"A dash marks a cell with no run. A latency cell shows the percentile of the merged " +
			"histogram, then the min-max spread of the runs. " + shortfallNote + "\n\n",
	)
	writeTable(b, rows, expectedRuns)
	return nil
}

// writeComparisonReport writes the comparison tables of each section of
// each directory in dirs to w, after a line of links to each section.
//
// Returns an error when:
//   - a directory does not hold host.json or launch.txt
//   - an EC2 run directory does not resolve
//   - a run directory does not load
//   - the results of one row mix throughput and latency
//   - the write fails
func writeComparisonReport(w io.Writer, dirs []string) error {
	var sections []cells.Section
	for _, dir := range dirs {
		dirSections, err := cells.Sections(dir)
		if err != nil {
			return err
		}
		sections = append(sections, dirSections...)
	}

	type loadedSection struct {
		heading string
		runID   string
		commit  string
		record  bench.RunRecord
		results []bench.Result
		rows    []*variantRow
	}
	loaded := make([]loadedSection, len(sections))
	for i, s := range sections {
		record, results, rows, err := loadRows(s.Dir)
		if err != nil {
			return fmt.Errorf("%s: %w", s.Dir, err)
		}
		loaded[i] = loadedSection{
			heading: s.Heading(record),
			runID:   s.RunID(record),
			commit:  s.RunCommit(record),
			record:  record,
			results: results,
			rows:    rows,
		}
	}

	anchors := make(headingAnchors)
	links := make([]string, len(loaded))
	for i, s := range loaded {
		links[i] = "[" + s.heading + "](#" + anchors.next(s.heading) + ")"
	}
	var b strings.Builder
	b.WriteString("# Benchmark results\n\n")
	b.WriteString(
		"`go run ./report -comparison` generates this file. " +
			"See the README for the method and the differences between clients.\n\n",
	)
	b.WriteString("Instances: " + strings.Join(links, " | ") + "\n\n")
	b.WriteString(comparisonCaption)
	for i, s := range loaded {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "## %s\n\n", s.heading)
		fmt.Fprintf(&b, "- Run: `%s` at commit `%s`. Runs per cell: %s, interleaved.\n",
			s.runID, s.commit, formatRuns(scenarioRuns(s.rows)))
		writeHostFacts(&b, s.record, s.results)
		writeComparison(&b, s.rows, anchors)
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// loadRows loads the run directory dir and groups its results into rows.
//
// Returns an error when the run directory does not load or the results of
// one row mix throughput and latency.
func loadRows(dir string) (
	bench.RunRecord,
	[]bench.Result,
	[]*variantRow,
	error,
) {
	record, results, err := cells.LoadRun(dir)
	if err != nil {
		return bench.RunRecord{}, nil, nil, err
	}
	rows, err := groupRows(results)
	if err != nil {
		return bench.RunRecord{}, nil, nil, fmt.Errorf("group results: %w", err)
	}
	return record, results, rows, nil
}

// writeHostFacts writes the host, driver, client and toolchain facts of
// record as a list, then a blank line. When a Luas result unlocks its hot
// goroutines, a line names the Luas layout, the yield interval, and the
// GOMAXPROCS of the unlocked Luas results. When another Luas result stays
// locked, the line also names the unlocked scenarios.
func writeHostFacts(b *strings.Builder, record bench.RunRecord, results []bench.Result) {
	host := record.Host
	fmt.Fprintf(b, "- Kernel: %s\n", cmp.Or(host.Kernel, "unknown"))
	fmt.Fprintf(b, "- CPU: %s\n", cmp.Or(host.CPUModel, "unknown"))
	fmt.Fprintf(b, "- Governor: %s\n", cmp.Or(host.Governor, "unknown"))
	fmt.Fprintf(b, "- SMT: %s\n", cmp.Or(host.SMT, "unknown"))
	fmt.Fprintf(b, "- Isolated CPUs: %s\n", cmp.Or(bench.FormatCPUList(host.IsolatedCPUs), "none"))
	fmt.Fprintf(b, "- Driver: aeronmd %s, commit %s\n",
		cmp.Or(record.Driver.Version, "unknown"), cmp.Or(record.Driver.SHA, "unknown"))
	for _, c := range cells.Clients {
		version := clientVersion(record, c.Name)
		fmt.Fprintf(
			b,
			"- %s: %s, commit %s\n",
			c.Title,
			cmp.Or(version.Version, "unknown"),
			cmp.Or(version.SHA, "unknown"),
		)
	}
	goInfo := record.Go
	fmt.Fprintf(b, "- Go: %s, GOMAXPROCS %d, GOGC %s, GOMEMLIMIT %s, GODEBUG %s\n",
		cmp.Or(goInfo.Version, "unknown"), goInfo.GOMAXPROCS,
		cmp.Or(goInfo.GOGC, "unset"), cmp.Or(goInfo.GOMEMLIMIT, "unset"), cmp.Or(goInfo.GODEBUG, "unset"))
	if hasUnlockedLuas(results) {
		layout := "hot goroutines unlocked"
		if interval := luasYieldInterval(results); interval != "" {
			layout += ", yield every " + interval + " ms"
		}
		layout += ", GOMAXPROCS " + cmp.Or(luasGOMAXPROCS(results), "unknown")
		if hasLockedLuas(results) {
			layout += ", in " + unlockedLuasScenarios(results)
		}
		fmt.Fprintf(b, "- Luas layout: %s\n", layout)
	}
	fmt.Fprintf(
		b,
		"- JDK: %s, flags: %s\n\n",
		cmp.Or(record.JVM.Version, "unknown"),
		cmp.Or(strings.Join(record.JVM.Flags, " "), "none"),
	)
}

// writeRun writes the run settings that record and results hold.
func writeRun(
	b *strings.Builder,
	record bench.RunRecord,
	results []bench.Result,
	expectedRuns map[string]int,
) {
	var messages, warmups, clusterMessages, clusterWarmups []int64
	var fixedRateMessages, fixedRateWarmups, rates, warmupRates []int64
	var cpus []string
	for _, result := range results {
		messages = append(messages, result.Config.Messages)
		warmups = append(warmups, result.Config.WarmupMessages)
		if result.Config.ClusterMembers > 0 {
			clusterMessages = append(clusterMessages, result.Config.Messages)
			clusterWarmups = append(clusterWarmups, result.Config.WarmupMessages)
		}
		if result.Config.Rate > 0 {
			fixedRateMessages = append(fixedRateMessages, result.Config.Messages)
			fixedRateWarmups = append(fixedRateWarmups, result.Config.WarmupMessages)
			rates = append(rates, result.Config.Rate)
			warmupRates = append(warmupRates, result.Config.WarmupRate)
		}
		cpus = append(cpus, cmp.Or(bench.FormatCPUList(result.Config.CPUs), "none"))
	}
	b.WriteString("## Run\n\n")
	fmt.Fprintf(b, "- Runs per cell: %s, interleaved.\n", formatRuns(expectedRuns))
	fmt.Fprintf(
		b,
		"- Messages per run: %s. Warmup messages per run: %s.\n",
		distinct(messages),
		distinct(warmups),
	)
	if len(clusterMessages) > 0 {
		fmt.Fprintf(
			b,
			"- Cluster messages per run: %s. Cluster warmup messages per run: %s.\n",
			distinct(clusterMessages),
			distinct(clusterWarmups),
		)
	}
	if len(fixedRateMessages) > 0 {
		fmt.Fprintf(
			b,
			"- Fixed-rate messages per run: %s. Fixed-rate warmup messages per run: %s.\n",
			distinct(fixedRateMessages),
			distinct(fixedRateWarmups),
		)
		fmt.Fprintf(
			b,
			"- Fixed rates: %s msg/s. Fixed-rate warmup rates: %s msg/s.\n",
			distinct(rates),
			distinct(warmupRates),
		)
	}
	fmt.Fprintf(
		b,
		"- Housekeeping CPUs: %s.\n",
		cmp.Or(bench.FormatCPUList(record.HousekeepingCPUs), "unknown"),
	)
	fmt.Fprintf(b, "- Server CPUs: %s.\n", cmp.Or(bench.FormatCPUList(record.ServerCPUs), "none"))
	fmt.Fprintf(b, "- Client hot thread CPUs: %s.\n", distinctStrings(cpus, "; "))
	b.WriteString("\n")
}

// isUnlockedLuas reports whether result is a Luas result that ran with
// UnlockThreads.
func isUnlockedLuas(result bench.Result) bool {
	return result.Client == cells.LuasClient && result.Config.UnlockThreads
}

// hasUnlockedLuas reports whether a Luas result in results ran with
// UnlockThreads.
func hasUnlockedLuas(results []bench.Result) bool {
	return slices.ContainsFunc(results, isUnlockedLuas)
}

// hasLockedLuas reports whether a Luas result keeps its hot goroutines
// locked.
func hasLockedLuas(results []bench.Result) bool {
	return slices.ContainsFunc(results, func(result bench.Result) bool {
		return result.Client == cells.LuasClient && !result.Config.UnlockThreads
	})
}

// unlockedLuasScenarios returns the distinct scenarios of the unlocked Luas
// results in ascending order, joined with ", ".
func unlockedLuasScenarios(results []bench.Result) string {
	var scenarios []string
	for _, result := range results {
		if isUnlockedLuas(result) {
			scenarios = append(scenarios, result.Scenario)
		}
	}
	return distinctStrings(scenarios, ", ")
}

// luasGOMAXPROCS returns the distinct GOMAXPROCS values of the unlocked
// Luas results in ascending order, joined with ", ". Empty when no unlocked
// Luas result records one.
func luasGOMAXPROCS(results []bench.Result) string {
	var values []int64
	for _, result := range results {
		if isUnlockedLuas(result) && result.GoRuntime != nil && result.GoRuntime.GOMAXPROCS > 0 {
			values = append(values, int64(result.GoRuntime.GOMAXPROCS))
		}
	}
	return distinct(values)
}

// luasYieldInterval returns the distinct yield intervals of the Luas
// results in milliseconds, in ascending order, joined with ", ". Empty when
// no Luas result yields.
func luasYieldInterval(results []bench.Result) string {
	var values []int64
	for _, result := range results {
		if result.Client == cells.LuasClient && result.Config.YieldIntervalNs > 0 {
			values = append(values, result.Config.YieldIntervalNs/1_000_000)
		}
	}
	return distinct(values)
}

// clientVersion returns the version that record holds for the client name.
// Returns the zero ClientVersion when record does not hold one.
func clientVersion(record bench.RunRecord, name string) bench.ClientVersion {
	index := slices.IndexFunc(
		record.Clients,
		func(v bench.ClientVersion) bool { return v.Name == name },
	)
	if index < 0 {
		return bench.ClientVersion{}
	}
	return record.Clients[index]
}

// distinct returns the distinct values in ascending order, joined with ", ".
func distinct(values []int64) string {
	var texts []string
	for _, value := range slices.Compact(slices.Sorted(slices.Values(values))) {
		texts = append(texts, strconv.FormatInt(value, 10))
	}
	return strings.Join(texts, ", ")
}

// distinctStrings returns the distinct values in ascending order, joined
// with separator.
func distinctStrings(values []string, separator string) string {
	return strings.Join(slices.Compact(slices.Sorted(slices.Values(values))), separator)
}
