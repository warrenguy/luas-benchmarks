// Command record writes the host.json run record of one run directory.
//
// Usage: record [flags] [-- JVM flags]
//
// The JVM flags after "--" name the flags the run passes to every Java
// client. Run record under the same CPU set and environment as the Go
// clients, so the Go settings match theirs. The Go version comes from the
// build info of the Go client binaries.
package main

import (
	"debug/buildinfo"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// options holds the record flags.
type options struct {
	runID            string
	commit           string
	output           string
	aeronmd          string
	driverLog        string
	driverProperties string
	driverDefines    []string
	serverProperties []string
	housekeepingCPUs string
	serverCPUs       string
	luasBinary       string
	aerongoBinary    string
	jar              string
	java             string
	javap            string
	jvmFlags         []string
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "record: %v\n", err)
		os.Exit(2)
	}
	if err := writeRecord(opts); err != nil {
		fmt.Fprintf(os.Stderr, "record: %v\n", err)
		os.Exit(1)
	}
}

// parseOptions parses args. Every flag except -driver-define,
// -server-properties, -java and -javap is required.
func parseOptions(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("record", flag.ContinueOnError)
	flags.StringVar(&opts.runID, "run-id", "", "run id")
	flags.StringVar(&opts.commit, "commit", "", "luas-benchmarks commit the run builds from")
	flags.StringVar(&opts.output, "output", "", "host.json path")
	flags.StringVar(&opts.aeronmd, "aeronmd", "", "aeronmd binary")
	flags.StringVar(
		&opts.driverLog,
		"driver-log",
		"",
		"driver log that holds the printed driver context",
	)
	flags.StringVar(
		&opts.driverProperties,
		"driver-properties",
		"",
		"properties file the driver reads",
	)
	flags.Func(
		"driver-define",
		"name=value the driver takes as -D (repeatable)",
		func(text string) error {
			opts.driverDefines = append(opts.driverDefines, text)
			return nil
		},
	)
	flags.Func(
		"server-properties",
		"properties file of a Java server or a server driver (repeatable)",
		func(path string) error {
			opts.serverProperties = append(opts.serverProperties, path)
			return nil
		},
	)
	flags.StringVar(
		&opts.housekeepingCPUs,
		"housekeeping-cpus",
		"",
		"CPU list every process of the run starts on",
	)
	flags.StringVar(
		&opts.serverCPUs,
		"server-cpus",
		"",
		"CPU list of the cluster member and archive threads",
	)
	flags.StringVar(&opts.luasBinary, "luas", "", "luas client binary")
	flags.StringVar(&opts.aerongoBinary, "aerongo", "", "aerongo client binary")
	flags.StringVar(&opts.jar, "jar", "", "Java client jar")
	flags.StringVar(&opts.java, "java", "java", "java command")
	flags.StringVar(&opts.javap, "javap", "javap", "javap command")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	opts.jvmFlags = append([]string{}, flags.Args()...)
	required := []struct{ name, value string }{
		{"run-id", opts.runID},
		{"commit", opts.commit},
		{"output", opts.output},
		{"aeronmd", opts.aeronmd},
		{"driver-log", opts.driverLog},
		{"driver-properties", opts.driverProperties},
		{"housekeeping-cpus", opts.housekeepingCPUs},
		{"luas", opts.luasBinary},
		{"aerongo", opts.aerongoBinary},
		{"jar", opts.jar},
	}
	var errs []error
	for _, option := range required {
		if option.value == "" {
			errs = append(errs, fmt.Errorf("-%s required", option.name))
		}
	}
	return opts, errors.Join(errs...)
}

// writeRecord collects the run record and writes it to opts.output.
func writeRecord(opts options) error {
	host, err := bench.CollectHost(os.DirFS("/"))
	if err != nil {
		return err
	}
	housekeepingCPUs, err := bench.ParseCPUList(opts.housekeepingCPUs)
	if err != nil {
		return fmt.Errorf("-housekeeping-cpus: %w", err)
	}
	serverCPUs, err := bench.ParseCPUList(opts.serverCPUs)
	if err != nil {
		return fmt.Errorf("-server-cpus: %w", err)
	}
	driver, err := collectDriver(opts)
	if err != nil {
		return err
	}
	serverProperties, err := collectServerProperties(opts.serverProperties)
	if err != nil {
		return err
	}
	clients, goVersion, err := collectClients(opts)
	if err != nil {
		return err
	}
	goInfo := bench.CollectGo()
	goInfo.Version = goVersion
	jdkVersion, err := run(opts.java, "-XshowSettings:properties", "-version")
	if err != nil {
		return err
	}
	jvm := bench.JVMInfo{Flags: opts.jvmFlags}
	if jvm.Version, err = bench.ParseJDKVersion(jdkVersion); err != nil {
		return err
	}
	return bench.WriteJSON(opts.output, bench.RunRecord{
		RunID:            opts.runID,
		Commit:           opts.commit,
		Host:             host,
		HousekeepingCPUs: housekeepingCPUs,
		ServerCPUs:       serverCPUs,
		Driver:           driver,
		ServerProperties: serverProperties,
		Clients:          clients,
		Go:               goInfo,
		JVM:              jvm,
	})
}

// collectDriver returns the driver version and config. The config holds the
// context the driver printed into its log, plus the -D defines and the
// properties file.
func collectDriver(opts options) (bench.DriverInfo, error) {
	versionText, err := run(opts.aeronmd, "-v")
	if err != nil {
		return bench.DriverInfo{}, err
	}
	var driver bench.DriverInfo
	if driver.Version, driver.SHA, err = bench.ParseDriverVersion(versionText); err != nil {
		return bench.DriverInfo{}, err
	}
	log, err := os.ReadFile(opts.driverLog)
	if err != nil {
		return bench.DriverInfo{}, fmt.Errorf("read driver log: %w", err)
	}
	if driver.Config, err = bench.ParseDriverContext(log); err != nil {
		return bench.DriverInfo{}, err
	}
	propertiesText, err := os.ReadFile(opts.driverProperties)
	if err != nil {
		return bench.DriverInfo{}, fmt.Errorf("read driver properties: %w", err)
	}
	properties, err := driverProperties(string(propertiesText), opts.driverDefines)
	if err != nil {
		return bench.DriverInfo{}, err
	}
	maps.Copy(driver.Config, properties)
	return driver, nil
}

// collectServerProperties reads each properties file in paths. Returns the
// properties of each file, keyed by file name.
//
// Returns an error when a file does not read or parse, or two paths share a
// file name.
func collectServerProperties(paths []string) (map[string]map[string]string, error) {
	files := make(map[string]map[string]string, len(paths))
	for _, path := range paths {
		name := filepath.Base(path)
		if _, ok := files[name]; ok {
			return nil, fmt.Errorf("server properties %s: file name %s already recorded", path, name)
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read server properties: %w", err)
		}
		properties, err := bench.ParseProperties(string(text))
		if err != nil {
			return nil, fmt.Errorf("server properties %s: %w", path, err)
		}
		files[name] = properties
	}
	return files, nil
}

// driverProperties merges the -D defines and the properties file text in the
// order aeronmd applies them. aeronmd sets every define first, then loads the
// properties file over them. So the file value wins over a define of the same
// name.
func driverProperties(propertiesText string, defines []string) (map[string]string, error) {
	properties, err := bench.ParseProperties(strings.Join(defines, "\n"))
	if err != nil {
		return nil, err
	}
	file, err := bench.ParseProperties(propertiesText)
	if err != nil {
		return nil, err
	}
	maps.Copy(properties, file)
	return properties, nil
}

// collectClients returns the Luas, Java and aeron-go client versions, under
// the client names of the results. collectClients also returns the Go
// toolchain that built both Go clients.
//
// Returns an error when a version does not read, or the two Go clients come
// from different Go toolchains.
func collectClients(opts options) (clients []bench.ClientVersion, goVersion string, err error) {
	luas, luasGo, err := binaryModule(opts.luasBinary, "luas", "github.com/warrenguy/luas")
	if err != nil {
		return nil, "", err
	}
	aerongo, aerongoGo, err := binaryModule(opts.aerongoBinary, "aerongo", "github.com/lirm/aeron-go")
	if err != nil {
		return nil, "", err
	}
	if luasGo != aerongoGo {
		return nil, "", fmt.Errorf(
			"go clients: luas built with %s, aerongo built with %s: rebuild both",
			luasGo,
			aerongoGo,
		)
	}
	constants, err := run(opts.javap, "-constants", "-cp", opts.jar, "io.aeron.AeronVersion")
	if err != nil {
		return nil, "", err
	}
	java := bench.ClientVersion{Name: "java"}
	if java.Version, java.SHA, err = bench.ParseAeronJavaVersion(constants); err != nil {
		return nil, "", err
	}
	return []bench.ClientVersion{luas, java, aerongo}, luasGo, nil
}

// binaryModule returns the version of the module at path that the Go binary
// at binary builds from. binaryModule also returns the Go toolchain that
// built the binary.
func binaryModule(binary, name, path string) (bench.ClientVersion, string, error) {
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		return bench.ClientVersion{}, "", fmt.Errorf("read build info of %s: %w", binary, err)
	}
	version, err := bench.ModuleVersion(info, name, path)
	if err != nil {
		return bench.ClientVersion{}, "", err
	}
	return version, info.GoVersion, nil
}

// run runs command with args. Returns stdout and stderr together.
func run(command string, args ...string) (string, error) {
	output, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s %s: %w: %s", command, strings.Join(args, " "), err, output)
	}
	return string(output), nil
}
