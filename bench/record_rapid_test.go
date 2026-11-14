package bench_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/warrenguy/luas-benchmarks/bench"
)

// TestParsePropertiesMatchesModel draws a map of properties and writes it as
// name=value lines. The test pads each name and value with white space. It
// mixes in comment lines and blank lines. ParseProperties must return the
// drawn map.
func TestParsePropertiesMatchesModel(t *testing.T) {
	padding := rapid.StringMatching(`[ \t]{0,2}`)
	rapid.Check(t, func(rt *rapid.T) {
		want := rapid.MapOf(
			rapid.StringMatching(`[a-z][a-z0-9._#!-]{0,15}`),
			rapid.StringMatching(`([!-~]([ !-~]{0,14}[!-~])?)?`),
		).Draw(rt, "properties")
		names := rapid.Permutation(slices.Sorted(maps.Keys(want))).Draw(rt, "names")

		var text strings.Builder
		for _, name := range names {
			writeFillerLines(rt, &text, padding)
			text.WriteString(padding.Draw(rt, "before name") + name + padding.Draw(rt, "after name") + "=" +
				padding.Draw(rt, "before value") + want[name] + padding.Draw(rt, "after value"))
			text.WriteString(rapid.SampledFrom([]string{"\n", "\r\n"}).Draw(rt, "line end"))
		}
		writeFillerLines(rt, &text, padding)

		got, err := bench.ParseProperties(text.String())
		require.NoError(rt, err)
		require.Equal(rt, want, got)
	})
}

// writeFillerLines writes up to two comment or blank lines to text.
func writeFillerLines(rt *rapid.T, text *strings.Builder, padding *rapid.Generator[string]) {
	rt.Helper()
	for range rapid.IntRange(0, 2).Draw(rt, "filler lines") {
		text.WriteString(padding.Draw(rt, "filler padding"))
		if rapid.Bool().Draw(rt, "comment") {
			text.WriteString(rapid.SampledFrom([]string{"#", "!"}).Draw(rt, "comment mark"))
			text.WriteString(rapid.StringMatching(`[ -~]{0,16}`).Draw(rt, "comment text"))
		}
		text.WriteString("\n")
	}
}
