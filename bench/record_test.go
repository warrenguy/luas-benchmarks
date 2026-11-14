package bench_test

import (
	"path/filepath"
	"runtime/debug"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

func TestCollectHost(t *testing.T) {
	cpuinfo := "processor\t: 0\nvendor_id\t: Example\nmodel name\t: Example CPU @ 3.00GHz\n\n" +
		"processor\t: 1\nmodel name\t: Example CPU @ 3.00GHz\n"
	tests := []struct {
		name    string
		files   fstest.MapFS
		want    bench.HostInfo
		wantErr bool
	}{
		{
			name: "full",
			files: fstest.MapFS{
				"proc/sys/kernel/osrelease": {Data: []byte("7.2.9-zen1-1-zen\n")},
				"proc/cpuinfo":              {Data: []byte(cpuinfo)},
				"sys/devices/system/cpu/cpu0/cpufreq/scaling_governor":  {Data: []byte("performance\n")},
				"sys/devices/system/cpu/cpu1/cpufreq/scaling_governor":  {Data: []byte("powersave\n")},
				"sys/devices/system/cpu/cpu10/cpufreq/scaling_governor": {Data: []byte("performance\n")},
				"sys/devices/system/cpu/smt/control":                    {Data: []byte("off\n")},
				"sys/devices/system/cpu/isolated":                       {Data: []byte("2-3,6\n")},
				"sys/kernel/mm/transparent_hugepage/enabled": {
					Data: []byte("always [madvise] never\n"),
				},
				"sys/kernel/mm/transparent_hugepage/shmem_enabled": {
					Data: []byte("always within_size advise [never] deny force\n"),
				},
				"proc/mounts": {Data: []byte("proc /proc proc rw,nosuid 0 0\n" +
					"tmpfs /dev/shm tmpfs rw,nosuid,nodev,huge=always,inode64 0 0\n")},
			},
			want: bench.HostInfo{
				Kernel:       "7.2.9-zen1-1-zen",
				CPUModel:     "Example CPU @ 3.00GHz",
				Governor:     "performance,powersave",
				SMT:          "off",
				IsolatedCPUs: []int{2, 3, 6},
				THP:          "madvise",
				ShmemTHP:     "never",
				DevShmHuge:   "always",
			},
		},
		{
			name: "dev shm without huge option",
			files: fstest.MapFS{
				"proc/sys/kernel/osrelease":       {Data: []byte("7.2.9\n")},
				"proc/cpuinfo":                    {Data: []byte(cpuinfo)},
				"sys/devices/system/cpu/isolated": {Data: []byte("\n")},
				"proc/mounts": {
					Data: []byte("tmpfs /dev/shm tmpfs rw,nosuid,nodev,inode64 0 0\n"),
				},
			},
			want: bench.HostInfo{
				Kernel:       "7.2.9",
				CPUModel:     "Example CPU @ 3.00GHz",
				IsolatedCPUs: []int{},
				DevShmHuge:   "never",
			},
		},
		{
			name: "no cpufreq, smt or isolated cpus",
			files: fstest.MapFS{
				"proc/sys/kernel/osrelease":       {Data: []byte("7.2.9\n")},
				"proc/cpuinfo":                    {Data: []byte(cpuinfo)},
				"sys/devices/system/cpu/isolated": {Data: []byte("\n")},
			},
			want: bench.HostInfo{Kernel: "7.2.9", CPUModel: "Example CPU @ 3.00GHz", IsolatedCPUs: []int{}},
		},
		{
			name: "bad isolated cpus",
			files: fstest.MapFS{
				"proc/sys/kernel/osrelease":       {Data: []byte("7.2.9\n")},
				"proc/cpuinfo":                    {Data: []byte(cpuinfo)},
				"sys/devices/system/cpu/isolated": {Data: []byte("3-1\n")},
			},
			wantErr: true,
		},
		{
			name: "no kernel release",
			files: fstest.MapFS{
				"proc/cpuinfo":                    {Data: []byte(cpuinfo)},
				"sys/devices/system/cpu/isolated": {Data: []byte("\n")},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bench.CollectHost(tt.files)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCollectGo(t *testing.T) {
	t.Setenv("GOGC", "off")
	t.Setenv("GOMEMLIMIT", "8GiB")
	t.Setenv("GODEBUG", "")

	info := bench.CollectGo()
	require.NotEmpty(t, info.Version)
	require.Positive(t, info.GOMAXPROCS)
	require.Equal(t, "off", info.GOGC)
	require.Equal(t, "8GiB", info.GOMEMLIMIT)
	require.Empty(t, info.GODEBUG)
}

func TestRunRecordJSONRoundTrip(t *testing.T) {
	record := bench.RunRecord{
		RunID:  "20261018-120000",
		Commit: "4f2a9c1e8b7d6053a1e2f3c4d5b6a7980e1f2d3c",
		Host: bench.HostInfo{
			Kernel:       "7.2.9",
			CPUModel:     "Example CPU @ 3.00GHz",
			Governor:     "performance",
			SMT:          "off",
			IsolatedCPUs: []int{2, 3, 4, 5},
		},
		HousekeepingCPUs: []int{0, 1},
		ServerCPUs:       []int{6, 7, 8, 9},
		Driver: bench.DriverInfo{
			Version: "1.53.3",
			SHA:     "377943bcb8",
			Config:  map[string]string{"aeron.threading.mode": "DEDICATED", "threading_mode": "DEDICATED"},
		},
		ServerProperties: map[string]map[string]string{
			"cluster.properties": {
				"aeron.cluster.idle.strategy": "org.agrona.concurrent.BusySpinIdleStrategy",
			},
			"cluster-aeronmd.properties": {"aeron.threading.mode": "SHARED"},
		},
		Clients: []bench.ClientVersion{
			{Name: "luas", Version: "v0.0.0-20261018064756-a26d23547a96", SHA: "a26d23547a96"},
			{Name: "java", Version: "1.53.3", SHA: "377943bcb8"},
			{Name: "aerongo", Version: "v0.0.0-20240606170339-8b05ad14e456", SHA: "8b05ad14e456"},
		},
		Go: bench.GoInfo{
			Version:    "go1.26.4",
			GOMAXPROCS: 4,
			GOGC:       "off",
			GOMEMLIMIT: "8GiB",
			GODEBUG:    "madvdontneed=1",
		},
		JVM: bench.JVMInfo{
			Version: "26",
			Flags: []string{
				"-XX:+UseParallelGC",
				"--add-opens",
				"java.base/jdk.internal.misc=ALL-UNNAMED",
			},
		},
	}
	path := filepath.Join(t.TempDir(), "host.json")
	require.NoError(t, bench.WriteJSON(path, record))

	var got bench.RunRecord
	require.NoError(t, bench.ReadJSON(path, &got))
	require.Equal(t, record, got)
}

func TestParseDriverVersion(t *testing.T) {
	version, sha, err := bench.ParseDriverVersion(
		"bin/aeronmd <aeron version=1.53.3 commit=377943bcb8> major 1 minor 53 patch 3 git 377943bcb8\n",
	)
	require.NoError(t, err)
	require.Equal(t, "1.53.3", version)
	require.Equal(t, "377943bcb8", sha)

	_, _, err = bench.ParseDriverVersion("Usage: aeronmd [-v][-Dname=value]\n")
	require.Error(t, err)
}

func TestParseDriverContext(t *testing.T) {
	tests := []struct {
		name    string
		log     string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "block between log lines",
			log: "INFO: starting\naeron_driver_context_t {\n    cnc_version=0.2.0\n" +
				"    threading_mode=DEDICATED\n" +
				"    udp_channel_transport_bindings.media=default,0x5562fa65d900(aeronmd:(null))\n" +
				"}\nShutting down driver...\n",
			want: map[string]string{
				"cnc_version":                          "0.2.0",
				"threading_mode":                       "DEDICATED",
				"udp_channel_transport_bindings.media": "default,0x5562fa65d900(aeronmd:(null))",
			},
		},
		{name: "no block", log: "INFO: starting\n", wantErr: true},
		{name: "unclosed block", log: "aeron_driver_context_t {\n    cnc_version=0.2.0\n", wantErr: true},
		{
			name:    "line without equals",
			log:     "aeron_driver_context_t {\n    cnc_version\n}\n",
			wantErr: true,
		},
		{
			name: "later field replaces earlier",
			log:  "aeron_driver_context_t {\n    a=1\n    a=2\n}\n",
			want: map[string]string{"a": "2"},
		},
		{name: "empty name", log: "aeron_driver_context_t {\n    =value\n}\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bench.ParseDriverContext([]byte(tt.log))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseProperties(t *testing.T) {
	got, err := bench.ParseProperties(
		"# comment\n! comment\n\naeron.threading.mode=DEDICATED\n aeron.dir = /dev/shm/x \n",
	)
	require.NoError(t, err)
	require.Equal(
		t,
		map[string]string{"aeron.threading.mode": "DEDICATED", "aeron.dir": "/dev/shm/x"},
		got,
	)

	_, err = bench.ParseProperties("aeron.threading.mode\n")
	require.Error(t, err)
	_, err = bench.ParseProperties("=DEDICATED\n")
	require.Error(t, err)
}

func TestParseAeronJavaVersion(t *testing.T) {
	constants := `Compiled from "AeronVersion.java"
public class io.aeron.AeronVersion implements io.aeron.version.Version {
  public static final java.lang.String VERSION = "1.53.3";
  public static final int MAJOR_VERSION = 1;
  public static final java.lang.String GIT_SHA = "377943bcb8";
}
`
	version, sha, err := bench.ParseAeronJavaVersion(constants)
	require.NoError(t, err)
	require.Equal(t, "1.53.3", version)
	require.Equal(t, "377943bcb8", sha)

	_, _, err = bench.ParseAeronJavaVersion("Error: class not found: io.aeron.AeronVersion\n")
	require.Error(t, err)
}

func TestParseJDKVersion(t *testing.T) {
	settings := "Property settings:\n    java.class.version = 70.0\n" +
		"    java.library.path = /usr/lib\n        /lib\n" +
		"    java.runtime.version = 26.0.1+9\n    java.vendor = Eclipse Adoptium\n" +
		"    java.vm.name = OpenJDK 64-Bit Server VM\n\nopenjdk version \"26.0.1\" 2026-01-20\n"
	got, err := bench.ParseJDKVersion(settings)
	require.NoError(t, err)
	require.Equal(t, "26.0.1+9 OpenJDK 64-Bit Server VM (Eclipse Adoptium)", got)

	_, err = bench.ParseJDKVersion("openjdk version \"26.0.1\" 2026-01-20\n")
	require.Error(t, err)
}

func TestModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{
		Path: "github.com/warrenguy/luas-benchmarks/luas",
		Deps: []*debug.Module{
			{Path: "github.com/warrenguy/luas", Version: "v0.0.0-20261018064756-a26d23547a96"},
			{Path: "github.com/HdrHistogram/hdrhistogram-go", Version: "v1.4.0"},
			{
				Path:    "github.com/lirm/aeron-go",
				Version: "v0.0.0-20240606170339-8b05ad14e456",
				Replace: &debug.Module{Path: "../aeron-go", Version: "(devel)"},
			},
			{Path: "github.com/example/workspace", Version: "(devel)"},
		},
	}
	tests := []struct {
		name    string
		path    string
		want    bench.ClientVersion
		wantErr bool
	}{
		{
			name: "pseudo-version",
			path: "github.com/warrenguy/luas",
			want: bench.ClientVersion{
				Name:    "client",
				Version: "v0.0.0-20261018064756-a26d23547a96",
				SHA:     "a26d23547a96",
			},
		},
		{
			name: "tagged version",
			path: "github.com/HdrHistogram/hdrhistogram-go",
			want: bench.ClientVersion{Name: "client", Version: "v1.4.0"},
		},
		{name: "replaced", path: "github.com/lirm/aeron-go", wantErr: true},
		{name: "workspace module", path: "github.com/example/workspace", wantErr: true},
		{name: "missing", path: "github.com/example/missing", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bench.ModuleVersion(info, "client", tt.path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
