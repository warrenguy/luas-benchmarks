package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
	"github.com/warrenguy/luas-benchmarks/cells"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// TestWriteReportMatchesGolden renders the fixture run directory. The
// fixture covers an odd and an even run count and a client with no run.
func TestWriteReportMatchesGolden(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, writeReport(&out, fixtureRun))
	requireGolden(t, "report.golden.md", out.String())
}

// TestWriteComparisonReportMatchesGolden renders the fixture run directory
// under its CPU model.
func TestWriteComparisonReportMatchesGolden(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, writeComparisonReport(&out, []string{fixtureRun}))
	requireGolden(t, "comparison.golden.md", out.String())
}

// requireGolden compares got with the golden file name in testdata. The
// -update flag rewrites the file first.
func requireGolden(t *testing.T, name, got string) {
	t.Helper()
	golden := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.WriteFile(golden, []byte(got), 0o644))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	require.Equal(t, string(want), got)
}

func TestWriteComparisonReportRejects(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "empty directory", args: []string{t.TempDir()}, wantErr: "no host.json or launch.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, writeComparisonReport(io.Discard, tt.args), tt.wantErr)
		})
	}
}

// writeLaunch returns a new EC2 run directory whose launch.txt holds launch.
func writeLaunch(t *testing.T, launch string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, cells.LaunchFile), []byte(launch), 0o644))
	return dir
}

// ec2RunID is the run id of the EC2 run directories that the tests lay out.
const ec2RunID = "20261018T120000Z"

// ec2Launch is the launch.txt of the EC2 run directories that the tests lay
// out. Its commit differs from the commit of the fixture run record.
const ec2Launch = "run_id=" + ec2RunID + "\ncommit=1111111\n" +
	"amd64_type=m8azn.6xlarge\narm_type=c9g.8xlarge\n"

// fixtureRun is the run directory of the fixture run.
var fixtureRun = filepath.Join("testdata", "run")

// writeEC2Run returns a new EC2 run directory whose launch.txt holds
// ec2Launch. The run of each instance in arches holds a copy of the fixture
// run directory.
func writeEC2Run(t *testing.T, arches []string) string {
	t.Helper()
	dir := writeLaunch(t, ec2Launch)
	for _, arch := range arches {
		require.NoError(t, os.CopyFS(filepath.Join(dir, arch, "results", ec2RunID), os.DirFS(fixtureRun)))
	}
	return dir
}

// ec2RunTests lists EC2 run directory layouts and the section headings
// that each layout yields.
var ec2RunTests = []struct {
	name      string
	arches    []string
	headings  []string
	instances string
}{
	{
		name:     "both instances",
		arches:   []string{"arm", "amd64"},
		headings: []string{"amd64 m8azn.6xlarge", "Arm c9g.8xlarge"},
		instances: "Instances: [amd64 m8azn.6xlarge](#amd64-m8azn6xlarge) | " +
			"[Arm c9g.8xlarge](#arm-c9g8xlarge)",
	},
	{
		name:      "no arm directory",
		arches:    []string{"amd64"},
		headings:  []string{"amd64 m8azn.6xlarge"},
		instances: "Instances: [amd64 m8azn.6xlarge](#amd64-m8azn6xlarge)",
	},
}

// TestWriteComparisonReportReadsEC2Run lays out the fixture run as an EC2
// run directory. Each instance section must match the section of the plain
// run directory under the instance heading.
func TestWriteComparisonReportReadsEC2Run(t *testing.T) {
	for _, tt := range ec2RunTests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeEC2Run(t, tt.arches)
			// The oracle passes one plain run per instance, so its
			// scenario anchors count up the same way.
			plainDirs := make([]string, len(tt.headings))
			for i := range tt.headings {
				plainDirs[i] = fixtureRun
			}
			var plain bytes.Buffer
			require.NoError(t, writeComparisonReport(&plain, plainDirs))
			_, want := cutLine(t, plain.String(), "Instances: ")
			for _, heading := range tt.headings {
				want = strings.Replace(want, "## Example CPU @ 3.00GHz\n", "## "+heading+"\n", 1)
			}

			var got bytes.Buffer
			require.NoError(t, writeComparisonReport(&got, []string{dir}))
			instances, rest := cutLine(t, got.String(), "Instances: ")
			require.Equal(t, tt.instances, instances)
			require.Equal(t, want, rest)
		})
	}
}

// anchorLink matches a link to a heading anchor.
var anchorLink = regexp.MustCompile(`\(#([^)]+)\)`)

// TestWriteReportReadsEC2Run lays out the fixture run as an EC2 run
// directory. Each section must match the report of the plain run directory
// under a title that names the instance. Its anchors count up after the
// anchors of the sections before it.
func TestWriteReportReadsEC2Run(t *testing.T) {
	const title = "# Luas, Java and aeron-go comparison\n"
	var plain bytes.Buffer
	require.NoError(t, writeReport(&plain, fixtureRun))
	body, found := strings.CutPrefix(plain.String(), title)
	require.True(t, found)
	for _, tt := range ec2RunTests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeEC2Run(t, tt.arches)
			sections := make([]string, len(tt.headings))
			for i, heading := range tt.headings {
				section := "# Luas, Java and aeron-go comparison on " + heading + "\n" + body
				if i > 0 {
					section = anchorLink.ReplaceAllString(section, "(#${1}-"+strconv.Itoa(i)+")")
				}
				sections[i] = section
			}

			var got bytes.Buffer
			require.NoError(t, writeReport(&got, dir))
			require.Equal(t, strings.Join(sections, "\n"), got.String())
		})
	}
}

// TestRunLineReadsLaunch removes the commit from the run record of an EC2
// run directory, and gives the record a shard run id. The run lines must
// name the run id and commit in launch.txt.
func TestRunLineReadsLaunch(t *testing.T) {
	dir := writeEC2Run(t, []string{"amd64"})
	path := filepath.Join(dir, "amd64", "results", ec2RunID, cells.HostFile)
	var record bench.RunRecord
	require.NoError(t, bench.ReadJSON(path, &record))
	record.Commit = ""
	record.RunID = ec2RunID + "-amd64-main"
	require.NoError(t, bench.WriteJSON(path, record))
	tests := []struct {
		name   string
		write  func(io.Writer, string) error
		prefix string
		want   []string
	}{
		{
			name:   "report",
			write:  writeReport,
			prefix: "Run `",
			want: []string{
				"Run `" + ec2RunID + "` at commit `1111111`. " +
					"See the README for the method and the differences between clients.",
			},
		},
		{
			name:   "comparison",
			write:  func(w io.Writer, dir string) error { return writeComparisonReport(w, []string{dir}) },
			prefix: "- Run: ",
			want: []string{
				"- Run: `" + ec2RunID + "` at commit `1111111`. Runs per cell: 3, interleaved.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, tt.write(&out, dir))
			var got []string
			for line := range strings.Lines(out.String()) {
				if strings.HasPrefix(line, tt.prefix) {
					got = append(got, strings.TrimSuffix(line, "\n"))
				}
			}
			require.Equal(t, tt.want, got)
		})
	}
}

// cutLine returns the first line of text that starts with prefix, and text
// without that line.
func cutLine(t *testing.T, text, prefix string) (line, rest string) {
	t.Helper()
	start := strings.Index(text, "\n"+prefix)
	require.GreaterOrEqual(t, start, 0, "no line starts with %q", prefix)
	end := strings.IndexByte(text[start+1:], '\n')
	require.GreaterOrEqual(t, end, 0)
	return text[start+1 : start+1+end], text[:start+1] + text[start+1+end+1:]
}

func TestWriteComparisonReportRejectsEC2Run(t *testing.T) {
	tests := []struct {
		name    string
		launch  string
		wantErr string
	}{
		{name: "bad line", launch: "run_id\n", wantErr: "want name=value"},
		{name: "no run_id", launch: "amd64_type=m8azn.6xlarge\n", wantErr: "does not name a run_id"},
		{
			name:    "no instance directory",
			launch:  "run_id=20261018T120000Z\n",
			wantErr: "no amd64 or arm directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeLaunch(t, tt.launch)
			require.ErrorContains(t, writeComparisonReport(io.Discard, []string{dir}), tt.wantErr)
		})
	}
}

func TestScenarioTitle(t *testing.T) {
	row := &variantRow{Row: &cells.Row{Scenario: "cluster-ping-pong"}}
	require.Equal(t, "cluster-ping-pong", scenarioTitle(row))
	row.Config.ClusterMembers = 1
	require.Equal(t, "cluster-ping-pong, 1 member", scenarioTitle(row))
	row.Config.ClusterMembers = 3
	require.Equal(t, "cluster-ping-pong, 3 members", scenarioTitle(row))
}

// TestWriteComparisonLatency renders one latency row. The table holds one
// line per percentile and leaves out the max. Each Luas value is the value
// of the merged histogram, not the median of the run values: the median p50
// is 2 us.
func TestWriteComparisonLatency(t *testing.T) {
	config := bench.Config{Publication: "concurrent", SendMethod: "offer", MessageLength: 32}
	pingPong := func(client string, values map[int64]int) bench.Result {
		return bench.Result{
			Scenario: "ipc-ping-pong",
			Client:   client,
			Variant:  "concurrent-offer-32",
			Config:   config,
			Latency:  &bench.Latency{Histogram: encodeHistogram(t, values)},
		}
	}
	rows, err := groupRows([]bench.Result{
		pingPong("luas", map[int64]int{1000: 3000}),
		pingPong("luas", map[int64]int{2000: 1}),
		pingPong("luas", map[int64]int{3000: 1}),
		pingPong("java", map[int64]int{1000: 1, 3000: 1}),
	})
	require.NoError(t, err)
	var b strings.Builder
	writeComparison(&b, rows, make(headingAnchors))
	require.Equal(t, `Scenarios: [ipc-ping-pong](#ipc-ping-pong-us)

### ipc-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Java | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 32 | p50 | 1.00 | 1.00 | - |
| concurrent | offer | 32 | p90 | 1.00 | 3.00 | - |
| concurrent | offer | 32 | p99 | 1.00 | 3.00 | - |
| concurrent | offer | 32 | p99.9 | 1.00 | 3.00 | - |
| concurrent | offer | 32 | p99.99 | 3.00 | 3.00 | - |
| concurrent | offer | 32 | p99.999 | 3.00 | 3.00 | - |
`, b.String())
}

// encodeHistogram returns the encoded histogram of values: each key in
// nanoseconds, recorded as often as its count.
func encodeHistogram(t *testing.T, values map[int64]int) string {
	t.Helper()
	histogram := bench.NewHistogram()
	for value, count := range values {
		for range count {
			require.NoError(t, histogram.Record(value))
		}
	}
	encoded, err := histogram.Encode()
	require.NoError(t, err)
	return encoded
}

func TestHeadingAnchors(t *testing.T) {
	anchors := make(headingAnchors)
	tests := []struct {
		heading string
		want    string
	}{
		{heading: "amd64 m8azn.6xlarge", want: "amd64-m8azn6xlarge"},
		{heading: "Example CPU @ 3.00GHz", want: "example-cpu--300ghz"},
		{heading: "ipc-throughput, Mmsg/s", want: "ipc-throughput-mmsgs"},
		{heading: "cluster-ping-pong, 3 members, us", want: "cluster-ping-pong-3-members-us"},
		{heading: "ipc-throughput, Mmsg/s", want: "ipc-throughput-mmsgs-1"},
		{heading: "IPC-throughput, Mmsg/s", want: "ipc-throughput-mmsgs-2"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, anchors.next(tt.heading), tt.heading)
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{value: 0, want: "0"},
		{value: 0.0523, want: "0.0523"},
		{value: 0.05184, want: "0.0518"},
		{value: 0.001234, want: "0.00123"},
		{value: 0.0999, want: "0.0999"},
		{value: 0.09996, want: "0.100"},
		{value: 0.305, want: "0.305"},
		{value: 9.996, want: "10.0"},
		{value: 9.92, want: "9.92"},
		{value: 21.5, want: "21.5"},
		{value: 99.96, want: "100"},
		{value: 143, want: "143"},
		{value: 12345.6, want: "12346"},
		{value: -0.0523, want: "-0.0523"},
	}
	for _, tt := range tests {
		t.Run(strconv.FormatFloat(tt.value, 'g', -1, 64), func(t *testing.T) {
			require.Equal(t, tt.want, formatValue(tt.value))
		})
	}
}

func TestScenarioRuns(t *testing.T) {
	rows := []*variantRow{
		{
			Row: &cells.Row{
				Scenario: "ipc-throughput",
				Runs: map[string][]bench.Result{
					"luas": make([]bench.Result, 15),
					"java": make([]bench.Result, 14),
				},
			},
		},
		{
			Row: &cells.Row{
				Scenario: "ipc-throughput",
				Runs:     map[string][]bench.Result{"luas": make([]bench.Result, 12)},
			},
		},
		{
			Row: &cells.Row{
				Scenario: "ipc-ping-pong",
				Runs: map[string][]bench.Result{
					"luas": make([]bench.Result, 5),
					"java": make([]bench.Result, 4),
				},
			},
		},
	}
	require.Equal(t, map[string]int{"ipc-throughput": 15, "ipc-ping-pong": 5}, scenarioRuns(rows))
}

func TestFormatRuns(t *testing.T) {
	tests := []struct {
		name string
		runs map[string]int
		want string
	}{
		{name: "one count", runs: map[string]int{"ipc-throughput": 3, "ipc-ping-pong": 3}, want: "3"},
		{
			name: "two counts",
			runs: map[string]int{
				"archive-record": 5,
				"ipc-ping-pong":  5,
				"udp-throughput": 15,
				"ipc-throughput": 15,
			},
			want: "15 in ipc-throughput, udp-throughput; 5 in ipc-ping-pong, archive-record",
		},
		{name: "none", runs: map[string]int{}, want: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatRuns(tt.runs))
		})
	}
}

func TestWriteHostFactsNamesLuasLayout(t *testing.T) {
	unlocked := bench.Config{UnlockThreads: true}
	yielding := bench.Config{UnlockThreads: true, YieldIntervalNs: 5_000_000}
	tests := []struct {
		name    string
		results []bench.Result
		want    string
	}{
		{
			name: "locked luas",
			results: []bench.Result{
				{Client: "luas", GoRuntime: &bench.GoRuntime{GOMAXPROCS: 8}},
				{Client: "aerongo", GoRuntime: &bench.GoRuntime{GOMAXPROCS: 8}},
			},
		},
		{
			name: "unlocked luas",
			results: []bench.Result{
				{Client: "luas", Config: unlocked, GoRuntime: &bench.GoRuntime{GOMAXPROCS: 2}},
				{Client: "luas", Config: unlocked, GoRuntime: &bench.GoRuntime{GOMAXPROCS: 1}},
				{Client: "luas", Config: unlocked, GoRuntime: &bench.GoRuntime{GOMAXPROCS: 2}},
				{Client: "aerongo", GoRuntime: &bench.GoRuntime{GOMAXPROCS: 8}},
				{Client: "java"},
			},
			want: "- Luas layout: hot goroutines unlocked, GOMAXPROCS 1, 2\n",
		},
		{
			name: "unlocked luas with yields",
			results: []bench.Result{
				{Client: "luas", Config: yielding, GoRuntime: &bench.GoRuntime{GOMAXPROCS: 2}},
				{Client: "luas", Config: yielding, GoRuntime: &bench.GoRuntime{GOMAXPROCS: 1}},
				{Client: "aerongo", GoRuntime: &bench.GoRuntime{GOMAXPROCS: 8}},
			},
			want: "- Luas layout: hot goroutines unlocked, yield every 5 ms, GOMAXPROCS 1, 2\n",
		},
		{
			name: "luas unlocked in some scenarios",
			results: []bench.Result{
				{Scenario: "ipc-throughput", Client: "luas", GoRuntime: &bench.GoRuntime{GOMAXPROCS: 8}},
				{
					Scenario:  "udp-ping-pong",
					Client:    "luas",
					Config:    yielding,
					GoRuntime: &bench.GoRuntime{GOMAXPROCS: 2},
				},
				{
					Scenario:  "ipc-fixed-rate",
					Client:    "luas",
					Config:    yielding,
					GoRuntime: &bench.GoRuntime{GOMAXPROCS: 2},
				},
				{
					Scenario:  "ipc-fixed-rate",
					Client:    "luas",
					Config:    yielding,
					GoRuntime: &bench.GoRuntime{GOMAXPROCS: 2},
				},
				{Scenario: "ipc-throughput", Client: "aerongo", GoRuntime: &bench.GoRuntime{GOMAXPROCS: 8}},
			},
			want: "- Luas layout: hot goroutines unlocked, yield every 5 ms, GOMAXPROCS 2, " +
				"in ipc-fixed-rate, udp-ping-pong\n",
		},
		{
			name:    "results without GOMAXPROCS",
			results: []bench.Result{{Client: "luas", Config: unlocked, GoRuntime: &bench.GoRuntime{}}},
			want:    "- Luas layout: hot goroutines unlocked, GOMAXPROCS unknown\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			writeHostFacts(&b, bench.RunRecord{}, tt.results)
			var got string
			for line := range strings.Lines(b.String()) {
				if strings.HasPrefix(line, "- Luas layout:") {
					got = line
				}
			}
			require.Equal(t, tt.want, got)
		})
	}
}
