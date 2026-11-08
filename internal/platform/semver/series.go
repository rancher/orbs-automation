package semver

import (
	"strconv"
	"strings"
)

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
func ParseSeries(text string) (Series, bool) {
	fields := strings.Split(strings.TrimPrefix(text, "v"), ".")
	if len(fields) > 3 {
		return Series{}, false
	}
	var s Series
	for _, f := range fields {
		if f == "x" {
			s.parts = append(s.parts, part{any: true})
			continue
		}
		n, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return Series{}, false
		}
		s.parts = append(s.parts, part{n: n})
	}
	return s, true
}

// String writes the series as it is read, without a leading v.
func (s Series) String() string {
	fields := make([]string, len(s.parts))
	for i, p := range s.parts {
		fields[i] = "x"
		if !p.any {
			fields[i] = strconv.FormatUint(p.n, 10)
		}
	}
	return strings.Join(fields, ".")
}

// Contains reports whether the version is in the series.
func (s Series) Contains(v Version) bool {
	have := [3]uint64{v.Major, v.Minor, v.Patch}
	for i, p := range s.parts {
		if !p.any && p.n != have[i] {
			return false
		}
	}
	return true
}

// Overlaps reports whether some version is in both series: no position at which
// both name a number has two different ones.
func (s Series) Overlaps(o Series) bool {
	for i := 0; i < len(s.parts) && i < len(o.parts); i++ {
		a, b := s.parts[i], o.parts[i]
		if !a.any && !b.any && a.n != b.n {
			return false
		}
	}
	return true
}

// InSeries reports whether the tag name is a version in the series written as
// text. A tag that is not a version, or a series that is not numbers and x,
// matches nothing.
func InSeries(tag, series string) bool {
	v, ok := Parse(tag)
	if !ok {
		return false
	}
	s, ok := ParseSeries(series)
	return ok && s.Contains(v)
}
