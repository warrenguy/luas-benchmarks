package cells

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// LaunchFile names the launch record that scripts/ec2-matrix writes in an
// EC2 run directory.
const LaunchFile = "launch.txt"

// Section names one run directory of a report and its heading.
type Section struct {
	// Label names the instance of an EC2 run. Empty for a run directory.
	Label string

	Dir string

	// Arch names the instance directory of an EC2 run: "amd64" or "arm".
	// Empty for a run directory.
	Arch string

	// Commit names the commit of the run when its run record does not.
	Commit string

	// LaunchRunID names the run id from launch.txt. It wins over the run
	// record, which names the instance or shard run.
	LaunchRunID string

	// Launch holds the properties of launch.txt. Nil for a run directory.
	Launch map[string]string
}

// Heading returns the label of the section, or the CPU model of record, or
// else its run id.
func (s Section) Heading(record bench.RunRecord) string {
	return cmp.Or(s.Label, record.Host.CPUModel, record.RunID)
}

// RunID returns the launch run id of the section, or else the run id of
// record.
func (s Section) RunID(record bench.RunRecord) string {
	return cmp.Or(s.LaunchRunID, record.RunID)
}

// RunCommit returns the commit of record, or else the commit of the
// section.
func (s Section) RunCommit(record bench.RunRecord) string {
	return cmp.Or(record.Commit, s.Commit, "unknown")
}

// Sections returns the sections of dir. A directory with host.json is a
// run directory and yields one section. A directory with launch.txt is an
// EC2 run directory.
//
// Returns an error when dir holds neither file, or the EC2 run directory
// does not resolve.
func Sections(dir string) ([]Section, error) {
	switch {
	case isFile(filepath.Join(dir, HostFile)):
		return []Section{{Dir: dir}}, nil
	case isFile(filepath.Join(dir, LaunchFile)):
		return ec2Sections(dir)
	default:
		return nil, fmt.Errorf("%s: no %s or %s", dir, HostFile, LaunchFile)
	}
}

// ec2Arches lists the instance directories of an EC2 run directory in
// report order, with the heading prefix of each.
var ec2Arches = []struct {
	name  string
	title string
}{
	{name: "amd64", title: "amd64"},
	{name: "arm", title: "Arm"},
}

// ec2Sections returns one section per instance directory of the EC2 run
// directory dir. The heading names the instance type from launch.txt. dir
// may lack an instance directory. That instance then does not get a section.
// Each section takes its run id and commit from launch.txt.
//
// Returns an error when launch.txt does not parse, it does not name a
// run_id, or dir does not hold an instance directory.
func ec2Sections(dir string) ([]Section, error) {
	text, err := os.ReadFile(filepath.Join(dir, LaunchFile))
	if err != nil {
		return nil, err
	}
	launch, err := bench.ParseProperties(string(text))
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", dir, LaunchFile, err)
	}
	runID := launch["run_id"]
	if runID == "" {
		return nil, fmt.Errorf("%s: %s does not name a run_id", dir, LaunchFile)
	}
	var sections []Section
	for _, arch := range ec2Arches {
		archDir := filepath.Join(dir, arch.name)
		if !isDir(archDir) {
			continue
		}
		label := arch.title + " " + launch[arch.name+"_type"]
		sections = append(sections, Section{
			Label:       label,
			Dir:         filepath.Join(archDir, "results", runID),
			Arch:        arch.name,
			Commit:      launch["commit"],
			LaunchRunID: runID,
			Launch:      launch,
		})
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("%s: no amd64 or arm directory", dir)
	}
	return sections, nil
}

// isFile reports whether path names a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// isDir reports whether path names a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
