package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriverProperties(t *testing.T) {
	tests := []struct {
		name           string
		propertiesText string
		defines        []string
		want           map[string]string
		wantErr        bool
	}{
		{
			name:           "distinct names",
			propertiesText: "aeron.threading.mode=DEDICATED\n",
			defines:        []string{"aeron.print.configuration=true"},
			want: map[string]string{
				"aeron.threading.mode":      "DEDICATED",
				"aeron.print.configuration": "true",
			},
		},
		{
			name:           "file wins over define",
			propertiesText: "aeron.threading.mode=DEDICATED\n",
			defines:        []string{"aeron.threading.mode=SHARED"},
			want:           map[string]string{"aeron.threading.mode": "DEDICATED"},
		},
		{
			name:    "malformed define",
			defines: []string{"aeron.threading.mode"},
			wantErr: true,
		},
		{
			name:           "malformed file line",
			propertiesText: "=DEDICATED\n",
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := driverProperties(tt.propertiesText, tt.defines)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCollectServerProperties(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(text), 0o644))
		return path
	}
	cluster := write("cluster.properties", "aeron.cluster.idle.strategy=busy\n")
	clusterDriver := write(
		"cluster-aeronmd.properties",
		"# member driver\naeron.threading.mode=SHARED\n",
	)
	malformed := write("malformed.properties", "=SHARED\n")
	otherCluster := filepath.Join(t.TempDir(), "cluster.properties")
	require.NoError(t, os.WriteFile(otherCluster, []byte("aeron.cluster.idle.strategy=noop\n"), 0o644))

	tests := []struct {
		name    string
		paths   []string
		want    map[string]map[string]string
		wantErr bool
	}{
		{name: "no files", want: map[string]map[string]string{}},
		{
			name:  "two files",
			paths: []string{cluster, clusterDriver},
			want: map[string]map[string]string{
				"cluster.properties":         {"aeron.cluster.idle.strategy": "busy"},
				"cluster-aeronmd.properties": {"aeron.threading.mode": "SHARED"},
			},
		},
		{name: "missing file", paths: []string{filepath.Join(dir, "missing.properties")}, wantErr: true},
		{name: "malformed file", paths: []string{malformed}, wantErr: true},
		{name: "shared file name", paths: []string{cluster, otherCluster}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectServerProperties(tt.paths)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
