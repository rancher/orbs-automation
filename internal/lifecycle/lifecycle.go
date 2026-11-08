// Package lifecycle reads Rancher's lifecycle dates from the SUSE lifecycle
// page (https://www.suse.com/lifecycle/). The page is the source of truth for
// the dates in rancher.lines.yaml; this package only parses it. It does no I/O
// of its own, so a test feeds it saved HTML.
package lifecycle

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"golang.org/x/net/html"
)

// Parse reads the lifecycle page from r and returns the dates of each Rancher
// line in the product section whose data-url is product (the page's own anchor,
// for example "suse-rancher-prime"), keyed by line id ("v2.15"). A cell left
// blank, or holding a placeholder such as TBD, is a date that is not set. It
// fails, naming what is wrong, when the section or its version table is
// missing or a version or date does not parse, so a change to the page is an
// error and never an empty result.
//
// On the page a product is a row marked data-url, followed by a row holding its
// detail: one table per lifecycle (Rancher's newer lines and its older ones are
// two tables with the same columns). Every table of the detail with a Version
// column is read, and the columns are found by their header, not by position.
func Parse(r io.Reader, product string) (map[string]v1alpha2.Dates, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("read the lifecycle page: %w", err)
	}
	row := findProductRow(doc, product)
	if row == nil {
		return nil, fmt.Errorf("the lifecycle page has no product section %q (no row with data-url=%q)", product, product)
	}
	detail := nextRow(row)
	if detail == nil {
		return nil, fmt.Errorf("section %q has no version table: nothing follows its row", product)
	}
	lines := map[string]v1alpha2.Dates{}
	var found bool
	for _, table := range descendants(detail, "table") {
		header, rows := tableRows(table)
		cols, ok := versionColumns(header)
		if !ok {
			continue
		}
		found = true
		if err := cols.complete(); err != nil {
			return nil, fmt.Errorf("section %q: %w", product, err)
		}
		for _, cells := range rows {
			id, dates, err := cols.read(cells)
			if err != nil {
				return nil, fmt.Errorf("section %q: %w", product, err)
			}
			if _, dup := lines[id]; dup {
				return nil, fmt.Errorf("section %q: line %s is listed twice", product, id)
			}
			lines[id] = dates
		}
	}
	if !found {
		return nil, fmt.Errorf("section %q has no version table (one with Version, GA, EOM and EOL columns)", product)
	}
	return lines, nil
}

// columns are the indexes of the columns of a version table, or -1 for one the
// table does not have.
type columns struct{ version, ga, eom, eol int }

// versionColumns finds the columns by header text. ok is false when there is no
// Version column, which is how a table that is not a version table shows.
func versionColumns(header []string) (c columns, ok bool) {
	c = columns{-1, -1, -1, -1}
	for i, h := range header {
		switch strings.ToLower(h) {
		case "version":
			c.version = i
		case "ga":
			c.ga = i
		case "eom":
			c.eom = i
		case "eol":
			c.eol = i
		}
	}
	return c, c.version >= 0
}

func (c columns) complete() error {
	for _, f := range []struct {
		name string
		idx  int
	}{{"GA", c.ga}, {"EOM", c.eom}, {"EOL", c.eol}} {
		if f.idx < 0 {
			return fmt.Errorf("the version table has no %s column", f.name)
		}
	}
	return nil
}

func (c columns) read(cells []string) (string, v1alpha2.Dates, error) {
	need := max(c.version, c.ga, c.eom, c.eol) + 1
	if len(cells) < need {
		return "", v1alpha2.Dates{}, fmt.Errorf("row %s has %d cells, need %d", cells[0], len(cells), need)
	}
	id, err := lineID(cells[c.version])
	if err != nil {
		return "", v1alpha2.Dates{}, err
	}
	var d v1alpha2.Dates
	for _, f := range []struct {
		name string
		cell string
		to   *time.Time
	}{{"GA", cells[c.ga], &d.GA}, {"EOM", cells[c.eom], &d.EOM}, {"EOL", cells[c.eol], &d.EOL}} {
		if *f.to, err = parseDate(f.cell); err != nil {
			return "", v1alpha2.Dates{}, fmt.Errorf("row %s: %s: %w", cells[c.version], f.name, err)
		}
	}
	return id, d, nil
}

var versionCell = regexp.MustCompile(`^(\d+\.\d+)\.x$`)

// lineID turns a version cell such as "2.15.x" into the line id "v2.15".
func lineID(cell string) (string, error) {
	m := versionCell.FindStringSubmatch(cell)
	if m == nil {
		return "", fmt.Errorf("%q is not a line version like \"2.15.x\"", cell)
	}
	return "v" + m[1], nil
}

var dateCell = regexp.MustCompile(`^(\d{1,2}) ([A-Za-z]+) (\d{4})$`)

var months = []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"}

// parseDate reads a date such as "27 Aug 2026" (also "22 Sept 2024" and
// "01 Mar 2023") as midnight UTC. A blank cell or a placeholder is not set.
func parseDate(cell string) (time.Time, error) {
	switch strings.ToLower(cell) {
	case "", "tbd", "n/a", "not applicable", "-", "–", "—":
		return time.Time{}, nil
	}
	m := dateCell.FindStringSubmatch(cell)
	if m == nil {
		return time.Time{}, fmt.Errorf("cannot read the date %q (want like \"27 Aug 2026\")", cell)
	}
	month := monthNumber(m[2])
	if month == 0 {
		return time.Time{}, fmt.Errorf("cannot read the date %q: unknown month %q", cell, m[2])
	}
	var day, year int
	_, _ = fmt.Sscanf(m[1], "%d", &day)
	_, _ = fmt.Sscanf(m[3], "%d", &year)
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if t.Day() != day {
		return time.Time{}, fmt.Errorf("cannot read the date %q: no such day", cell)
	}
	return t, nil
}

// monthNumber is the month a name or abbreviation of at least three letters
// stands for ("Aug", "Sept", "September"), or 0.
func monthNumber(word string) time.Month {
	word = strings.ToLower(word)
	if len(word) < 3 {
		return 0
	}
	for i, name := range months {
		if strings.HasPrefix(name, word) {
			return time.Month(i + 1)
		}
	}
	return 0
}

// findProductRow returns the first tr with data-url equal to product.
func findProductRow(n *html.Node, product string) *html.Node {
	if n.Type == html.ElementNode && n.Data == "tr" && attr(n, "data-url") == product {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findProductRow(c, product); found != nil {
			return found
		}
	}
	return nil
}

// nextRow is the tr that follows row among its siblings, or nil.
func nextRow(row *html.Node) *html.Node {
	for s := row.NextSibling; s != nil; s = s.NextSibling {
		if s.Type == html.ElementNode && s.Data == "tr" {
			return s
		}
	}
	return nil
}

// tableRows returns the header cells of a table (its first row of th cells) and
// the td cells of every other row, each as clean text.
func tableRows(table *html.Node) (header []string, rows [][]string) {
	for _, tr := range descendants(table, "tr") {
		if th := children(tr, "th"); len(th) > 0 {
			if header == nil {
				header = texts(th)
			}
			continue
		}
		if td := children(tr, "td"); len(td) > 0 {
			rows = append(rows, texts(td))
		}
	}
	return header, rows
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// descendants returns every element named name below n, in document order.
func descendants(n *html.Node, name string) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == name {
			out = append(out, c)
		}
		out = append(out, descendants(c, name)...)
	}
	return out
}

// children returns the direct children of n named name.
func children(n *html.Node, name string) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == name {
			out = append(out, c)
		}
	}
	return out
}

func texts(nodes []*html.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = text(n)
	}
	return out
}

// text is the text under n with every run of whitespace collapsed to one
// space. strings.Fields counts the non-breaking space the page puts inside
// dates as whitespace.
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
