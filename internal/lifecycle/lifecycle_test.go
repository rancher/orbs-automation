package lifecycle

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestParseTheRealPageFixture(t *testing.T) {
	f, err := os.Open("testdata/lifecycle.html")
	require.NoError(t, err)
	defer f.Close()

	got, err := Parse(f, "suse-rancher-prime")

	require.NoError(t, err)
	// Both tables of the section are read (2.9 and later, then 2.8 and earlier),
	// and the other product's table is not.
	assert.Len(t, got, 17)
	assert.NotContains(t, got, "v9.9")
	assert.Equal(t, config.Dates{GA: day(2026, time.August, 27), EOM: day(2027, time.February, 27), EOL: day(2028, time.February, 27)}, got["v2.15"])
	// A non-breaking space between the month and the year.
	assert.Equal(t, config.Dates{GA: day(2026, time.April, 30), EOM: day(2026, time.October, 30), EOL: day(2027, time.October, 10)}, got["v2.14"])
	// A four-letter month, and a zero-padded day.
	assert.Equal(t, config.Dates{GA: day(2024, time.January, 22), EOM: day(2024, time.September, 22), EOL: day(2025, time.July, 22)}, got["v2.8"])
	assert.Equal(t, config.Dates{GA: day(2021, time.August, 31), EOM: day(2023, time.March, 1), EOL: day(2024, time.April, 30)}, got["v2.6"])
	assert.Equal(t, config.Dates{GA: day(2017, time.June, 8), EOM: day(2019, time.December, 31), EOL: day(2020, time.June, 30)}, got["v1.6"])
}

func TestParsePicksTheNamedProduct(t *testing.T) {
	f, err := os.Open("testdata/lifecycle.html")
	require.NoError(t, err)
	defer f.Close()

	got, err := Parse(f, "suse-other-product")

	require.NoError(t, err)
	assert.Equal(t, map[string]config.Dates{
		"v9.9": {GA: day(2030, time.January, 1), EOM: day(2030, time.July, 1), EOL: day(2031, time.January, 1)},
	}, got)
}

// page wraps one version table in a product section the way the real page does.
func page(table string) string {
	return fmt.Sprintf(`<table><tbody>
<tr class="row" data-url="p"><td>Product</td></tr>
<tr class="sub_row"><td colspan="10">%s</td></tr>
</tbody></table>`, table)
}

func versionTable(header string, rows ...string) string {
	return "<table><thead><tr>" + header + "</tr></thead><tbody>" + strings.Join(rows, "") + "</tbody></table>"
}

const stdHeader = "<th>Version</th><th>GA</th><th>EOM</th><th>EOL</th>"

func row(cells ...string) string {
	return "<tr><td>" + strings.Join(cells, "</td><td>") + "</td></tr>"
}

func TestParseReadsColumnsByHeader(t *testing.T) {
	html := page(versionTable("<th>EOL</th><th>Version</th><th>EOM</th><th>GA</th>",
		row("3 Mar 2028", "2.15.x", "2 Feb 2027", "1 Jan 2026")))

	got, err := Parse(strings.NewReader(html), "p")

	require.NoError(t, err)
	assert.Equal(t, map[string]config.Dates{
		"v2.15": {GA: day(2026, time.January, 1), EOM: day(2027, time.February, 2), EOL: day(2028, time.March, 3)},
	}, got)
}

func TestParseHeaderTextIsCaseAndSpaceInsensitive(t *testing.T) {
	html := page(versionTable("<th> version </th><th>Ga</th><th>eom</th><th>EOL </th>",
		row("2.15.x", "1 Jan 2026", "1 Jul 2026", "1 Jan 2027")))

	got, err := Parse(strings.NewReader(html), "p")

	require.NoError(t, err)
	assert.Contains(t, got, "v2.15")
}

func TestParseABlankDateIsNotSet(t *testing.T) {
	html := page(versionTable(stdHeader, row("2.16.x", "1 Jan 2027", "", "&nbsp;")))

	got, err := Parse(strings.NewReader(html), "p")

	require.NoError(t, err)
	assert.Equal(t, config.Dates{GA: day(2027, time.January, 1)}, got["v2.16"])
}

func TestParseSkipsRowsWithNoCells(t *testing.T) {
	html := page(versionTable(stdHeader,
		"<tr></tr>",
		row("2.15.x", "1 Jan 2026", "1 Jul 2026", "1 Jan 2027")))

	got, err := Parse(strings.NewReader(html), "p")

	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Contains(t, got, "v2.15")
}

func TestParsePlaceholdersAreNotSet(t *testing.T) {
	for _, placeholder := range []string{"TBD", "tbd", "N/A", "Not Applicable", "-", "—", "–"} {
		t.Run(placeholder, func(t *testing.T) {
			html := page(versionTable(stdHeader, row("2.16.x", placeholder, placeholder, placeholder)))

			got, err := Parse(strings.NewReader(html), "p")

			require.NoError(t, err)
			assert.Equal(t, map[string]config.Dates{"v2.16": {}}, got)
		})
	}
}

func TestParseFailsLoudly(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		product string
		want    string
	}{
		{"product not on the page", page(versionTable(stdHeader)), "other", `"other"`},
		{"empty page", "", "p", `"p"`},
		{
			"section without a table",
			`<table><tr class="row" data-url="p"><td>x</td></tr><tr class="sub_row"><td>no table</td></tr></table>`,
			"p", "version table",
		},
		{
			"product row with no sub row after it",
			`<table><tr class="row" data-url="p"><td>x</td></tr></table>`,
			"p", "version table",
		},
		{"missing EOM column", page(versionTable("<th>Version</th><th>GA</th><th>EOL</th>", row("2.15.x", "1 Jan 2026", "1 Jan 2027"))), "p", "EOM"},
		{"missing GA column", page(versionTable("<th>Version</th><th>EOM</th><th>EOL</th>", row("2.15.x", "1 Jul 2026", "1 Jan 2027"))), "p", "GA"},
		{"missing EOL column", page(versionTable("<th>Version</th><th>GA</th><th>EOM</th>", row("2.15.x", "1 Jan 2026", "1 Jul 2026"))), "p", "EOL"},
		{"missing version column", page(versionTable("<th>GA</th><th>EOM</th><th>EOL</th>")), "p", "Version"},
		{"table with unrelated headers", page(versionTable("<th>Name</th><th>Price</th>")), "p", "version table"},
		{"version not a line", page(versionTable(stdHeader, row("latest", "1 Jan 2026", "1 Jul 2026", "1 Jan 2027"))), "p", `"latest"`},
		{"version without the x", page(versionTable(stdHeader, row("2.15", "1 Jan 2026", "1 Jul 2026", "1 Jan 2027"))), "p", `"2.15"`},
		{"date that is not a date", page(versionTable(stdHeader, row("2.15.x", "soon", "1 Jul 2026", "1 Jan 2027"))), "p", `"soon"`},
		{"impossible date", page(versionTable(stdHeader, row("2.15.x", "31 Feb 2026", "1 Jul 2026", "1 Jan 2027"))), "p", `"31 Feb 2026"`},
		{"unknown month", page(versionTable(stdHeader, row("2.15.x", "1 Foo 2026", "1 Jul 2026", "1 Jan 2027"))), "p", `"1 Foo 2026"`},
		{
			"same line twice",
			page(versionTable(stdHeader, row("2.15.x", "1 Jan 2026", "1 Jul 2026", "1 Jan 2027"), row("2.15.x", "1 Jan 2026", "1 Jul 2026", "1 Jan 2027"))),
			"p", "v2.15",
		},
		{"row with too few cells", page(versionTable(stdHeader, "<tr><td>2.15.x</td><td>1 Jan 2026</td></tr>")), "p", "2.15"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tt.html), tt.product)

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.want)
			assert.Nil(t, got)
		})
	}
}

func TestParseNamesTheRowThatFailed(t *testing.T) {
	html := page(versionTable(stdHeader,
		row("2.15.x", "1 Jan 2026", "1 Jul 2026", "1 Jan 2027"),
		row("2.14.x", "oops", "1 Jul 2026", "1 Jan 2027")))

	_, err := Parse(strings.NewReader(html), "p")

	require.Error(t, err)
	assert.ErrorContains(t, err, "2.14.x")
	assert.ErrorContains(t, err, "GA")
}
