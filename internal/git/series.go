package git

// InSeries reports whether the tag name is a version in the series, a dotted
// prefix of the version in which x stands for any number: "v2.15" holds
// v2.15.0 and v2.15.3-rc.1, and "9.x" holds 9.0.0 and 9.4.1. A leading v is
// ignored on both sides. A tag that is not a version, or a series that is not
// numbers and x, matches nothing.
func InSeries(tag, series string) bool {
	return false
}
