package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/warrenguy/luas-benchmarks/cells"
)

// shortfallNote explains the mark of a fixed-rate cell with a shortfall.
const shortfallNote = "`!` marks a fixed-rate cell where a run missed its send rate " +
	"or lost replies."

// comparisonCaption explains the comparison tables.
const comparisonCaption = `- Each cell shows the median.
- A dash marks a client with no run.
- A latency cell shows the percentile of the merged histogram of all runs.
- ` + shortfallNote + `

`

// scenarioTable holds the rows of one comparison table: one scenario, or
// one member count of a cluster scenario.
type scenarioTable struct {
	title string
	rows  []*variantRow
}

// kind returns the kind of the first row of t.
func (t scenarioTable) kind() cells.Kind {
	return t.rows[0].Kind
}

// heading returns the table heading: the scenario title and the unit.
func (t scenarioTable) heading() string {
	return t.title + ", " + metrics(t.kind())[0].unit
}

// scenarioTables splits rows into one table per scenario title, in order of
// first appearance.
func scenarioTables(rows []*variantRow) []*scenarioTable {
	var tables []*scenarioTable
	byTitle := make(map[string]*scenarioTable)
	for _, row := range rows {
		title := scenarioTitle(row)
		table, found := byTitle[title]
		if !found {
			table = &scenarioTable{title: title}
			byTitle[title] = table
			tables = append(tables, table)
		}
		table.rows = append(table.rows, row)
	}
	return tables
}

// writeComparison writes a line of links to the tables of rows, then one
// table per scenario. Each table line holds the cell value of one metric
// for each client. A row takes one line per metric in the comparison
// metrics of its kind. anchors counts the headings written before, so each
// link names its own table.
func writeComparison(b *strings.Builder, rows []*variantRow, anchors headingAnchors) {
	tables := scenarioTables(rows)
	links := make([]string, len(tables))
	for i, table := range tables {
		links[i] = "[" + table.title + "](#" + anchors.next(table.heading()) + ")"
	}
	b.WriteString("Scenarios: " + strings.Join(links, " | ") + "\n")
	for _, table := range tables {
		fmt.Fprintf(b, "\n### %s\n\n", table.heading())
		writeComparisonTable(b, table)
	}
}

// writeComparisonTable writes the table head and the lines of each row. A
// latency table adds a Percentile column. A fixed-rate table adds a Rate
// column in messages per second, then a Percentile column.
func writeComparisonTable(b *strings.Builder, table *scenarioTable) {
	b.WriteString("| Publication | Send | Bytes |")
	separator := "|---|---|---:|"
	kind := table.kind()
	if kind == cells.FixedRate {
		b.WriteString(" Rate |")
		separator += "---:|"
	}
	if kind.IsLatency() {
		b.WriteString(" Percentile |")
		separator += "---|"
	}
	for _, c := range cells.Clients {
		b.WriteString(" " + c.Title + " |")
	}
	b.WriteString("\n" + separator + strings.Repeat("---:|", len(cells.Clients)) + "\n")
	for _, row := range table.rows {
		for _, m := range comparisonMetrics(row.Kind) {
			writeComparisonLine(b, row, m)
		}
	}
}

// writeComparisonLine writes the line of metric m of row.
func writeComparisonLine(b *strings.Builder, row *variantRow, m metric) {
	fmt.Fprintf(
		b,
		"| %s | %s | %d |",
		row.Config.Publication,
		row.Config.SendMethod,
		row.Config.MessageLength,
	)
	if row.Kind == cells.FixedRate {
		fmt.Fprintf(b, " %d |", row.Config.Rate)
	}
	if row.Kind.IsLatency() {
		b.WriteString(" " + m.statistic + " |")
	}
	for _, c := range cells.Clients {
		if len(row.Runs[c.Name]) == 0 {
			b.WriteString(" - |")
			continue
		}
		value, _ := row.cellValue(c.Name, m)
		b.WriteString(" " + formatValue(value) + row.mark(c.Name) + " |")
	}
	b.WriteByte('\n')
}

// headingAnchors counts the link anchors of the headings of one document.
// Each anchor follows the GitHub rule: lower case, letters, digits, "-" and
// "_" kept, spaces turned to "-", other characters dropped. A repeated
// anchor takes the suffix "-1", then "-2", and so on.
type headingAnchors map[string]int

// next returns the anchor of the next heading with the text heading.
func (a headingAnchors) next(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	anchor := b.String()
	count := a[anchor]
	a[anchor]++
	if count > 0 {
		return anchor + "-" + strconv.Itoa(count)
	}
	return anchor
}

// scenarioTitle returns the scenario name of row, with the member count of
// a cluster row.
func scenarioTitle(row *variantRow) string {
	switch members := row.Config.ClusterMembers; members {
	case 0:
		return row.Scenario
	case 1:
		return row.Scenario + ", 1 member"
	default:
		return row.Scenario + ", " + strconv.Itoa(members) + " members"
	}
}
