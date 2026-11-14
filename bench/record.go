package bench

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
)

// RunRecord holds the host, driver and toolchain facts of one run directory.
// Each run directory holds one RunRecord as host.json.
type RunRecord struct {
	RunID string `json:"run_id"`

	// Commit holds the luas-benchmarks commit the run builds from.
	Commit string `json:"commit"`

	Host HostInfo `json:"host"`

	// HousekeepingCPUs holds the CPUs every process of the run starts on.
	// Each hot thread then moves itself to a CPU of its own.
	HousekeepingCPUs []int `json:"housekeeping_cpus"`

	// ServerCPUs holds the CPUs of the cluster member and archive threads:
	// four per cluster member in member order, and the first for the
	// archive of an archive cell. Empty when the run pins no server thread.
	ServerCPUs []int `json:"server_cpus"`

	Driver DriverInfo `json:"driver"`

	// ServerProperties holds each properties file of the Java archive, the
	// Java cluster members and the cluster member drivers, keyed by file
	// name. Each value maps a property name to its value.
	ServerProperties map[string]map[string]string `json:"server_properties"`

	Clients []ClientVersion `json:"clients"`
	Go      GoInfo          `json:"go"`
	JVM     JVMInfo         `json:"jvm"`
}

// HostInfo holds the kernel and CPU facts of the bench host.
type HostInfo struct {
	Kernel   string `json:"kernel"`
	CPUModel string `json:"cpu_model"`

	// Governor joins the distinct scaling governors of all CPUs with ",".
	// Empty when the host does not expose cpufreq.
	Governor string `json:"governor"`

	// SMT holds the kernel SMT control state, for example "on" or "off".
	SMT string `json:"smt"`

	// IsolatedCPUs holds the CPUs the kernel isolates. CollectHost returns
	// an empty list, not nil, so the JSON matches Config.CPUs.
	IsolatedCPUs []int `json:"isolated_cpus"`

	// THP holds the selected transparent huge page mode, for example
	// "madvise". Empty when the host does not expose the mode.
	THP string `json:"thp"`

	// ShmemTHP holds the selected huge page mode of the kernel's internal
	// shmem mount, for example "never". "deny" and "force" override every
	// tmpfs mount. Empty when the host does not expose the mode.
	ShmemTHP string `json:"shmem_thp"`

	// DevShmHuge holds the huge= option of the /dev/shm mount. The driver
	// keeps its term buffers there. "never" when the mount sets no huge=
	// option. Empty when /dev/shm is not a mount.
	DevShmHuge string `json:"dev_shm_huge"`
}

// DriverInfo holds the media driver version and its full config.
type DriverInfo struct {
	Version string `json:"version"`
	SHA     string `json:"sha"`

	// Config holds the context the driver prints on start, under the C
	// field names. Config also holds the properties the driver reads, under
	// the property names.
	Config map[string]string `json:"config"`
}

// ClientVersion names one client library and the commit it builds from.
type ClientVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	SHA     string `json:"sha"`
}

// GoInfo holds the Go toolchain and the runtime settings of the Go clients.
type GoInfo struct {
	Version    string `json:"version"`
	GOMAXPROCS int    `json:"gomaxprocs"`
	GOGC       string `json:"gogc"`
	GOMEMLIMIT string `json:"gomemlimit"`
	GODEBUG    string `json:"godebug"`
}

// JVMInfo holds the JDK version and the JVM flags of the Java client.
type JVMInfo struct {
	Version string   `json:"version"`
	Flags   []string `json:"flags"`
}

// CollectHost reads HostInfo from the proc and sys files under root. Pass
// os.DirFS("/") on a live host.
//
// Returns an error when:
//   - a kernel, cpuinfo or isolated CPU file does not read
//   - a governor, SMT, huge page or mounts file exists but does not read
//   - the isolated CPU list does not parse
//
// A missing cpufreq, SMT, huge page or mounts file leaves its field empty.
func CollectHost(root fs.FS) (HostInfo, error) {
	host, err := collectHost(root)
	if err != nil {
		return HostInfo{}, fmt.Errorf("collect host: %w", err)
	}
	return host, nil
}

func collectHost(root fs.FS) (HostInfo, error) {
	var host HostInfo
	var err error
	if host.Kernel, err = readTrimmed(root, "proc/sys/kernel/osrelease"); err != nil {
		return HostInfo{}, err
	}
	cpuinfo, err := fs.ReadFile(root, "proc/cpuinfo")
	if err != nil {
		return HostInfo{}, err
	}
	host.CPUModel = cpuModel(cpuinfo)

	governors, err := fs.Glob(root, "sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_governor")
	if err != nil {
		return HostInfo{}, err
	}
	names := make([]string, len(governors))
	for i, path := range governors {
		if names[i], err = readTrimmed(root, path); err != nil {
			return HostInfo{}, err
		}
	}
	slices.Sort(names)
	host.Governor = strings.Join(slices.Compact(names), ",")

	host.SMT, err = readTrimmed(root, "sys/devices/system/cpu/smt/control")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return HostInfo{}, err
	}

	isolated, err := fs.ReadFile(root, "sys/devices/system/cpu/isolated")
	if err != nil {
		return HostInfo{}, err
	}
	if host.IsolatedCPUs, err = ParseCPUList(string(isolated)); err != nil {
		return HostInfo{}, fmt.Errorf("isolated cpus: %w", err)
	}

	if host.THP, err = readSelected(root, "sys/kernel/mm/transparent_hugepage/enabled"); err != nil {
		return HostInfo{}, err
	}
	if host.ShmemTHP, err = readSelected(
		root,
		"sys/kernel/mm/transparent_hugepage/shmem_enabled",
	); err != nil {
		return HostInfo{}, err
	}
	mounts, err := fs.ReadFile(root, "proc/mounts")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return HostInfo{}, err
	}
	host.DevShmHuge = mountHuge(mounts, "/dev/shm")
	return host, nil
}

// readSelected returns the bracketed choice of the sysfs mode file at path,
// such as "madvise" from "always [madvise] never". A missing file returns
// "".
func readSelected(root fs.FS, path string) (string, error) {
	data, err := fs.ReadFile(root, path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for field := range strings.FieldsSeq(string(data)) {
		if choice, found := strings.CutPrefix(field, "["); found {
			return strings.TrimSuffix(choice, "]"), nil
		}
	}
	return "", nil
}

// mountHuge returns the huge= option of the mount at mountPoint in mounts,
// a /proc/mounts listing. A mount without the option returns "never", the
// tmpfs default. A missing mount returns "".
func mountHuge(mounts []byte, mountPoint string) string {
	huge := ""
	for line := range strings.Lines(string(mounts)) {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != mountPoint {
			continue
		}
		// A later mount on the same point hides the earlier one.
		huge = "never"
		for option := range strings.SplitSeq(fields[3], ",") {
			if value, found := strings.CutPrefix(option, "huge="); found {
				huge = value
			}
		}
	}
	return huge
}

// readTrimmed returns the file at path without leading and trailing white
// space.
func readTrimmed(root fs.FS, path string) (string, error) {
	data, err := fs.ReadFile(root, path)
	return string(bytes.TrimSpace(data)), err
}

// cpuModel returns the first "model name" value in cpuinfo. Empty when
// cpuinfo does not have one.
func cpuModel(cpuinfo []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(cpuinfo))
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), ":")
		if found && strings.TrimSpace(key) == "model name" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// CollectGo returns GoInfo for the calling process.
func CollectGo() GoInfo {
	return GoInfo{
		Version:    runtime.Version(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		GOGC:       os.Getenv("GOGC"),
		GOMEMLIMIT: os.Getenv("GOMEMLIMIT"),
		GODEBUG:    os.Getenv("GODEBUG"),
	}
}

var driverVersionPattern = regexp.MustCompile(`major (\d+) minor (\d+) patch (\d+) git (\S+)`)

// ParseDriverVersion parses the line that aeronmd -v prints into the
// version and the commit. For example, "aeronmd <aeron version=1.53.3
// commit=377943bcb8> major 1 minor 53 patch 3 git 377943bcb8" yields
// "1.53.3" and "377943bcb8".
//
// Returns an error when text does not hold a "major N minor N patch N git
// SHA" part.
func ParseDriverVersion(text string) (version, sha string, err error) {
	match := driverVersionPattern.FindStringSubmatch(text)
	if match == nil {
		return "", "", fmt.Errorf("parse driver version %q: no major, minor, patch and git fields", text)
	}
	return match[1] + "." + match[2] + "." + match[3], match[4], nil
}

// driverContextStart opens the context block that aeronmd prints on start.
const driverContextStart = "aeron_driver_context_t {"

// ParseDriverContext parses the context block that aeronmd prints on start
// when aeron.print.configuration is true. The line "aeron_driver_context_t {"
// opens the block. The line "}" closes it. Each line between holds one
// field as name=value. A later field of the same name replaces an earlier
// one.
//
// Returns an error when log does not hold a closed block, or a block line
// does not have "=" or has an empty name.
func ParseDriverContext(log []byte) (map[string]string, error) {
	fields := make(map[string]string)
	inBlock := false
	scanner := bufio.NewScanner(bytes.NewReader(log))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case !inBlock:
			inBlock = line == driverContextStart
		case line == "}":
			return fields, nil
		default:
			name, value, found := strings.Cut(line, "=")
			if !found || name == "" {
				return nil, fmt.Errorf("parse driver context: line %q: want name=value", line)
			}
			fields[name] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse driver context: %w", err)
	}
	return nil, fmt.Errorf("parse driver context: no closed %q block", driverContextStart)
}

// ParseProperties parses Aeron properties text with one name=value pair per
// line. Blank lines and lines that start with # or ! do not hold a property.
// ParseProperties trims white space around each name and value.
//
// Returns an error when a line does not have "=" or has an empty name.
func ParseProperties(text string) (map[string]string, error) {
	properties := make(map[string]string)
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return nil, fmt.Errorf("parse properties: line %q: want name=value", line)
		}
		properties[name] = strings.TrimSpace(value)
	}
	return properties, nil
}

var (
	aeronJavaVersionPattern = regexp.MustCompile(`\bVERSION = "([^"]+)"`)
	aeronJavaSHAPattern     = regexp.MustCompile(`\bGIT_SHA = "([^"]+)"`)
)

// ParseAeronJavaVersion parses the constants that "javap -constants
// io.aeron.AeronVersion" prints into the Aeron version and commit of the
// Java client jar.
//
// Returns an error when text does not hold the VERSION constant or does
// not hold the GIT_SHA constant.
func ParseAeronJavaVersion(text string) (version, sha string, err error) {
	versionMatch := aeronJavaVersionPattern.FindStringSubmatch(text)
	shaMatch := aeronJavaSHAPattern.FindStringSubmatch(text)
	if versionMatch == nil || shaMatch == nil {
		return "", "", errors.New("parse aeron java version: no VERSION or GIT_SHA constant")
	}
	return versionMatch[1], shaMatch[1], nil
}

// ParseJDKVersion parses the properties that "java -XshowSettings:properties
// -version" prints into one line of the runtime version, the VM name and
// the vendor, for example "26.0.1+9 OpenJDK 64-Bit Server VM (Eclipse
// Adoptium)".
//
// Returns an error when text does not hold a java.runtime.version
// property.
func ParseJDKVersion(text string) (string, error) {
	properties := make(map[string]string)
	for line := range strings.Lines(text) {
		name, value, found := strings.Cut(line, " = ")
		if found {
			properties[strings.TrimSpace(name)] = strings.TrimSpace(value)
		}
	}
	version := properties["java.runtime.version"]
	if version == "" {
		return "", errors.New("parse jdk version: no java.runtime.version property")
	}
	return fmt.Sprintf(
		"%s %s (%s)",
		version,
		properties["java.vm.name"],
		properties["java.vendor"],
	), nil
}

// ModuleVersion returns the version of the module at path that info lists,
// under the client name. SHA holds the commit of a pseudo-version. SHA is
// empty for a tagged version.
//
// Returns an error when:
//   - info does not list a module at path
//   - a replace directive swaps the module for another
//   - the version is no tag or pseudo-version, for example "(devel)" for a
//     go.work module
func ModuleVersion(info *debug.BuildInfo, name, path string) (ClientVersion, error) {
	for _, dep := range info.Deps {
		if dep.Path != path {
			continue
		}
		if dep.Replace != nil {
			return ClientVersion{}, fmt.Errorf(
				"module %s: replaced by %s %s",
				path,
				dep.Replace.Path,
				dep.Replace.Version,
			)
		}
		if !strings.HasPrefix(dep.Version, "v") {
			return ClientVersion{}, fmt.Errorf(
				"module %s: version %q: want a tag or pseudo-version",
				path,
				dep.Version,
			)
		}
		return ClientVersion{Name: name, Version: dep.Version, SHA: pseudoVersionCommit(dep.Version)}, nil
	}
	return ClientVersion{}, fmt.Errorf("module %s: not in build info of %s", path, info.Path)
}

// pseudoVersionCommit returns the 12-digit commit at the end of a Go
// pseudo-version, for example "a26d23547a96" for
// "v0.0.0-20261018064756-a26d23547a96". Empty for any other version.
func pseudoVersionCommit(version string) string {
	version = strings.TrimSuffix(version, "+incompatible")
	index := strings.LastIndexByte(version, '-')
	commit := version[index+1:]
	if index < 0 || len(commit) != 12 || strings.Trim(commit, "0123456789abcdef") != "" {
		return ""
	}
	return commit
}
