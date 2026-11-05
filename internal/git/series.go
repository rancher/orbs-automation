package git

import (
	"strconv"
	"strings"

	"github.com/rancher/orbs-automation/internal/semver"
)

// InSeries reports whether the tag name is a version in the series, a dotted
// prefix of the version in which x stands for any number: "v2.15" holds
// v2.15.0 and v2.15.3-rc.1, and "9.x" holds 9.0.0 and 9.4.1. A leading v is
// ignored on both sides. A tag that is not a version, or a series that is not
// numbers and x, matches nothing.
func InSeries(tag, series string) bool {
	v, ok := semver.Parse(tag)
	if !ok {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(series, "v"), ".")
	if len(parts) > 3 {
		return false
	}
	have := [3]uint64{v.Major, v.Minor, v.Patch}
	for i, part := range parts {
		if part == "x" {
			continue
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil || n != have[i] {
			return false
		}
	}
	return true
}
