package cells

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// HostFile names the run record in a run directory.
const HostFile = "host.json"

// LuasClient names the Luas client in results.
const LuasClient = "luas"

// Client names one result client and its title.
type Client struct {
	Name  string
	Title string
}

// Clients lists the clients in run order.
var Clients = []Client{
	{Name: LuasClient, Title: "Luas"},
	{Name: "java", Title: "Java"},
	{Name: "aerongo", Title: "aeron-go"},
}

// LoadRun reads the run record and every result file of the run directory
// dir. Each JSON file other than host.json holds one result. The results of
// one scenario variant and client come in run order: the number at the end
// of the file name.
//
// Returns an error when:
//   - host.json or a result file does not read or decode
//   - dir does not hold a result file
//   - a result names an unknown client
//   - a result holds both or neither of a throughput and a latency
//   - a result holds fixed-rate fields without a rate, or the reverse
//   - a fixed-rate result holds no latency
func LoadRun(dir string) (bench.RunRecord, []bench.Result, error) {
	var record bench.RunRecord
	if err := bench.ReadJSON(filepath.Join(dir, HostFile), &record); err != nil {
		return bench.RunRecord{}, nil, fmt.Errorf("load run record: %w", err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return bench.RunRecord{}, nil, fmt.Errorf("list results: %w", err)
	}
	slices.SortFunc(paths, compareResultPaths)
	var results []bench.Result
	for _, path := range paths {
		if filepath.Base(path) == HostFile {
			continue
		}
		var result bench.Result
		if err := bench.ReadJSON(path, &result); err != nil {
			return bench.RunRecord{}, nil, fmt.Errorf("load result: %w", err)
		}
		if err := checkResult(result); err != nil {
			return bench.RunRecord{}, nil, fmt.Errorf("load result %s: %w", path, err)
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return bench.RunRecord{}, nil, fmt.Errorf("load results: no result file in %s", dir)
	}
	return record, results, nil
}

// compareResultPaths orders result paths by the name before the run number,
// then by the run number.
func compareResultPaths(a, b string) int {
	nameA, runA := splitRunNumber(a)
	nameB, runB := splitRunNumber(b)
	return cmp.Or(strings.Compare(nameA, nameB), cmp.Compare(runA, runB), strings.Compare(a, b))
}

// splitRunNumber splits the path of a result file at its last "-" into the
// name and the run number. A path without a run number yields run number 0.
func splitRunNumber(path string) (string, int) {
	stem := strings.TrimSuffix(path, ".json")
	index := strings.LastIndexByte(stem, '-')
	if index < 0 {
		return stem, 0
	}
	run, err := strconv.Atoi(stem[index+1:])
	if err != nil {
		return stem, 0
	}
	return stem[:index], run
}

// checkResult rejects a result that names an unknown client, or that holds
// both or neither of a throughput and a latency. A result of a config with
// a rate must hold a latency and the fixed-rate fields. Other results must
// not hold the fixed-rate fields.
func checkResult(result bench.Result) error {
	if !slices.ContainsFunc(Clients, func(c Client) bool { return c.Name == result.Client }) {
		return fmt.Errorf("unknown client %q", result.Client)
	}
	hasThroughput := result.Throughput != nil
	hasLatency := result.Latency != nil
	if hasThroughput == hasLatency {
		return errors.New("want exactly one of throughput and latency")
	}
	isFixedRate := result.Config.Rate > 0
	if isFixedRate != (result.FixedRate != nil) {
		return errors.New("want fixed_rate exactly when the config sets a rate")
	}
	if isFixedRate && !hasLatency {
		return errors.New("want latency with fixed_rate")
	}
	return nil
}
