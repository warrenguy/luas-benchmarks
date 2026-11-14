package bench_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warrenguy/luas-benchmarks/bench"
)

func TestParseCPUList(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    []int
		wantErr bool
	}{
		{name: "empty", text: "", want: []int{}},
		{name: "newline only", text: "\n", want: []int{}},
		{name: "single", text: "3", want: []int{3}},
		{name: "ranges keep order", text: "6-7,2-3,0\n", want: []int{6, 7, 2, 3, 0}},
		{name: "one cpu range", text: "4-4", want: []int{4}},
		{name: "max cpu", text: "1023", want: []int{1023}},
		{name: "above max cpu", text: "1024", wantErr: true},
		{name: "empty entry", text: "1,,2", wantErr: true},
		{name: "trailing comma", text: "1,", wantErr: true},
		{name: "negative", text: "-1", wantErr: true},
		{name: "open range", text: "2-", wantErr: true},
		{name: "descending range", text: "3-1", wantErr: true},
		{name: "duplicate", text: "1-3,2", wantErr: true},
		{name: "sign", text: "+1", wantErr: true},
		{name: "word", text: "all", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bench.ParseCPUList(tt.text)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFormatCPUList(t *testing.T) {
	require.Empty(t, bench.FormatCPUList(nil))
	require.Equal(t, "2-4,7,6,0-1", bench.FormatCPUList([]int{2, 3, 4, 7, 6, 0, 1}))
}
