package semver

// Series is a dotted prefix of a version in which x stands for any number: "9.x"
// holds 9.0.0 and 9.4.1, "0.9.x" holds 0.9.0 and 0.9.7, and "v2.15" holds v2.15.0
// and v2.15.3-rc.1. A leading v is ignored.
type Series struct {
	parts []part
}

// part is one position of a series: a number, or any.
type part struct {
	n   uint64
	any bool
}

// ParseSeries reads a series of up to three positions, each a number or x. It is
// false for anything else.
func ParseSeries(string) (Series, bool) { return Series{}, false }

// String writes the series as it is read, without a leading v.
func (s Series) String() string {
	for _, p := range s.parts {
		_, _ = p.n, p.any
	}
	return ""
}

// Contains reports whether the version is in the series.
func (s Series) Contains(Version) bool { return false }

// Overlaps reports whether some version is in both series.
func (s Series) Overlaps(Series) bool { return false }

// InSeries reports whether the tag name is a version in the series written as
// text. A tag that is not a version, or a series that is not numbers and x,
// matches nothing.
func InSeries(string, string) bool { return false }
